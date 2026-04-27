// Package http exposes REST endpoints for node_groups. See ADR 0001
// for why node groups are a first-class resource distinct from nodes.
//
// Routes mounted under /node-groups:
//   GET    /node-groups                       list
//   POST   /node-groups                       create
//   GET    /node-groups/{id}                  get
//   DELETE /node-groups/{id}                  delete
//   POST   /node-groups/{id}/start            start all children
//   POST   /node-groups/{id}/stop             stop all children
//   POST   /node-groups/{id}/restart          restart (stop+start)
//   GET    /node-groups/{id}/services         list attached services
//   POST   /node-groups/{id}/services/postgres attach managed postgres (legacy)
//   PUT    /node-groups/{id}/postgres-service  point group at an existing postgres service
//   DELETE /node-groups/{id}/postgres-service  clear the pointer (does not stop the service)
package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	ngservice "github.com/chainlaunch/chainlaunch/pkg/nodegroups/service"
	ngtypes "github.com/chainlaunch/chainlaunch/pkg/nodegroups/types"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

type Handler struct {
	service  *ngservice.Service
	validate *validator.Validate
}

func NewHandler(svc *ngservice.Service) *Handler {
	return &Handler{service: svc, validate: validator.New()}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/node-groups", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/", h.Create)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.Get)
			r.Delete("/", h.Delete)
			r.Post("/init", h.Init)
			r.Post("/start", h.Start)
			r.Post("/stop", h.Stop)
			r.Post("/restart", h.Restart)
			r.Get("/children", h.ListChildren)
			r.Route("/services", func(r chi.Router) {
				r.Get("/", h.ListServices)
				r.Post("/postgres", h.CreatePostgresService)
			})
			r.Route("/postgres-service", func(r chi.Router) {
				r.Put("/", h.AttachPostgresService)
				r.Delete("/", h.DetachPostgresService)
			})
		})
	})
}

// CreateRequest is the JSON body for POST /node-groups.
type CreateRequest struct {
	Name           string            `json:"name" validate:"required"`
	Platform       string            `json:"platform" validate:"required"`
	GroupType      ngtypes.GroupType `json:"groupType" validate:"required"`
	MSPID          string            `json:"mspId,omitempty"`
	OrganizationID *int64            `json:"organizationId,omitempty"`
	PartyID        *int64            `json:"partyId,omitempty"`
	Version        string            `json:"version,omitempty"`
	ExternalIP     string            `json:"externalIp,omitempty"`
	DomainNames    []string          `json:"domainNames,omitempty"`
	Config         json.RawMessage   `json:"config,omitempty"`
}

// @Summary List node groups
// @Tags NodeGroups
// @Produce json
// @Success 200 {array} types.NodeGroup
// @Router /node-groups [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit := parseInt32(r.URL.Query().Get("limit"), 50)
	offset := parseInt32(r.URL.Query().Get("offset"), 0)
	groups, err := h.service.List(r.Context(), limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, groups)
}

// @Summary Create a node group
// @Tags NodeGroups
// @Accept json
// @Produce json
// @Param request body CreateRequest true "Group creation request"
// @Success 201 {object} types.NodeGroup
// @Router /node-groups [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	grp, err := h.service.Create(r.Context(), ngservice.CreateInput{
		Name:           req.Name,
		Platform:       req.Platform,
		GroupType:      req.GroupType,
		MSPID:          req.MSPID,
		OrganizationID: req.OrganizationID,
		PartyID:        req.PartyID,
		Version:        req.Version,
		ExternalIP:     req.ExternalIP,
		DomainNames:    req.DomainNames,
		Config:         req.Config,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, grp)
}

// @Summary Get a node group
// @Tags NodeGroups
// @Produce json
// @Param id path int true "Group ID"
// @Success 200 {object} types.NodeGroup
// @Router /node-groups/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	grp, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, grp)
}

// @Summary Delete a node group
// @Tags NodeGroups
// @Param id path int true "Group ID"
// @Success 204
// @Router /node-groups/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// InitOrdererRequest is the JSON body for POST /node-groups/{id}/init
// when the group is a FABRICX_ORDERER_GROUP. Ports are required; the
// rest is derived from the persisted group row.
type InitOrdererRequest struct {
	RouterPort    int    `json:"routerPort" validate:"required,gt=0"`
	BatcherPort   int    `json:"batcherPort" validate:"required,gt=0"`
	ConsenterPort int    `json:"consenterPort" validate:"required,gt=0"`
	AssemblerPort int    `json:"assemblerPort" validate:"required,gt=0"`
	ConsenterType string `json:"consenterType,omitempty"`
}

// @Summary Initialize a FabricX orderer node group
// @Description Generates crypto, writes on-disk config, persists deployment_config,
// @Description and creates 4 child role rows (router, batcher, consenter, assembler).
// @Description Only valid for FABRICX_ORDERER_GROUP groups. Idempotency: fails if
// @Description already initialized.
// @Description
// @Description FABRICX_COMMITTER groups have no init step — children are added one
// @Description at a time via POST /nodes with fabricXCommitter.nodeGroupId pointing
// @Description at the parent group. Each committer child owns its own MSP identity.
// @Tags NodeGroups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Param request body InitOrdererRequest true "Port allocations"
// @Success 200 {object} types.NodeGroup
// @Router /node-groups/{id}/init [post]
func (h *Handler) Init(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	grp, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}

	if grp.GroupType != ngtypes.GroupTypeFabricXOrderer {
		writeErr(w, http.StatusBadRequest,
			fmt.Sprintf("init is only supported for FABRICX_ORDERER_GROUP; this group is %q. "+
				"For FABRICX_COMMITTER, add children via POST /nodes with fabricXCommitter.nodeGroupId",
				grp.GroupType))
		return
	}

	var req InitOrdererRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.InitOrdererGroup(r.Context(), id, ngservice.OrdererInitInput{
		RouterPort:    req.RouterPort,
		BatcherPort:   req.BatcherPort,
		ConsenterPort: req.ConsenterPort,
		AssemblerPort: req.AssemblerPort,
		ConsenterType: req.ConsenterType,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// @Summary Start all children in a node group
// @Tags NodeGroups
// @Param id path int true "Group ID"
// @Success 202
// @Router /node-groups/{id}/start [post]
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.service.StartGroup(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// @Summary Stop all children in a node group
// @Tags NodeGroups
// @Param id path int true "Group ID"
// @Success 202
// @Router /node-groups/{id}/stop [post]
func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.service.StopGroup(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// @Summary Restart a node group
// @Tags NodeGroups
// @Param id path int true "Group ID"
// @Success 202
// @Router /node-groups/{id}/restart [post]
func (h *Handler) Restart(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.service.RestartGroup(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// CreatePostgresRequest is the JSON body for POST /node-groups/{id}/services/postgres.
type CreatePostgresRequest struct {
	Name     string `json:"name" validate:"required"`
	Version  string `json:"version,omitempty"`
	DB       string `json:"db" validate:"required"`
	User     string `json:"user" validate:"required"`
	Password string `json:"password" validate:"required"`
	HostPort int    `json:"hostPort,omitempty"`
}

// @Summary List child nodes of a group
// @Description Returns the nodes owned by this group in canonical role order
// @Description (the same order StartGroup uses). For a committer that's
// @Description sidecar → coordinator → validator → verifier → query; for an
// @Description orderer group it's router → batcher → consenter → assembler.
// @Tags NodeGroups
// @Produce json
// @Param id path int true "Group ID"
// @Success 200 {array} ngservice.Child
// @Router /node-groups/{id}/children [get]
func (h *Handler) ListChildren(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	children, err := h.service.ListChildren(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, children)
}

// @Summary List services attached to a node group
// @Tags NodeGroups
// @Produce json
// @Param id path int true "Group ID"
// @Success 200 {array} types.NodeGroupService
// @Router /node-groups/{id}/services [get]
func (h *Handler) ListServices(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	services, err := h.service.ListServicesForGroup(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, services)
}

// @Summary Attach a managed PostgreSQL service to a node group
// @Tags NodeGroups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Param request body CreatePostgresRequest true "Postgres service config"
// @Success 201 {object} types.NodeGroupService
// @Router /node-groups/{id}/services/postgres [post]
func (h *Handler) CreatePostgresService(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req CreatePostgresRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	svc, err := h.service.CreatePostgresService(r.Context(), id, ngservice.CreatePostgresInput{
		Name:     req.Name,
		Version:  req.Version,
		DB:       req.DB,
		User:     req.User,
		Password: req.Password,
		HostPort: req.HostPort,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, svc)
}

// AttachPostgresServiceRequest is the JSON body for PUT /node-groups/{id}/postgres-service.
type AttachPostgresServiceRequest struct {
	ServiceID int64 `json:"serviceId" validate:"required"`
}

// @Summary Attach an existing postgres service to a node group
// @Description Sets node_groups.postgres_service_id. The service must already exist
// @Description (create it via POST /services/postgres). Only FABRICX_COMMITTER groups
// @Description accept postgres services.
// @Tags NodeGroups
// @Accept json
// @Produce json
// @Param id path int true "Group ID"
// @Param request body AttachPostgresServiceRequest true "Service to attach"
// @Success 200 {object} types.NodeGroup
// @Router /node-groups/{id}/postgres-service [put]
func (h *Handler) AttachPostgresService(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req AttachPostgresServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	grp, err := h.service.AttachPostgresService(r.Context(), id, req.ServiceID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, grp)
}

// @Summary Detach the postgres service pointer from a node group
// @Description Clears node_groups.postgres_service_id. The underlying service is
// @Description NOT stopped — use POST /services/{id}/stop for that.
// @Tags NodeGroups
// @Produce json
// @Param id path int true "Group ID"
// @Success 200 {object} types.NodeGroup
// @Router /node-groups/{id}/postgres-service [delete]
func (h *Handler) DetachPostgresService(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	grp, err := h.service.DetachPostgresService(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, grp)
}

// --- helpers ---

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	s := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid id %q", s))
		return 0, false
	}
	return id, true
}

func parseInt32(s string, def int32) int32 {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return def
	}
	return int32(v)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
