// Package http exposes REST endpoints for managed services.
//
// Services are a first-class, standalone resource — the node_groups
// coordinator references the service it needs via postgres_service_id;
// it does not own the service. That reversal is the reason this handler
// lives under /services, not /node-groups/{id}/services.
//
// Routes mounted under /services:
//   GET    /services                 list (filters: ?serviceType= ?status=)
//   POST   /services/postgres        create a POSTGRES service
//   GET    /services/{id}            get
//   PUT    /services/{id}            update (rejected while RUNNING/STARTING)
//   DELETE /services/{id}            delete (rejected while RUNNING)
//   POST   /services/{id}/start      start on a caller-chosen docker network
//   POST   /services/{id}/stop       stop the underlying container
package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	svcservice "github.com/chainlaunch/chainlaunch/pkg/services/service"
	svctypes "github.com/chainlaunch/chainlaunch/pkg/services/types"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

type Handler struct {
	service  *svcservice.Service
	validate *validator.Validate
}

func NewHandler(svc *svcservice.Service) *Handler {
	return &Handler{service: svc, validate: validator.New()}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/services", func(r chi.Router) {
		r.Get("/", h.List)
		r.Post("/postgres", h.CreatePostgres)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.Get)
			r.Put("/", h.Update)
			r.Delete("/", h.Delete)
			r.Post("/start", h.Start)
			r.Post("/stop", h.Stop)
			r.Get("/consumers", h.Consumers)
			r.Get("/logs", h.Logs)
			r.Route("/postgres", func(r chi.Router) {
				r.Post("/databases", h.CreatePostgresDatabases)
			})
		})
	})
}

// CreatePostgresRequest is the JSON body for POST /services/postgres.
type CreatePostgresRequest struct {
	Name     string `json:"name" validate:"required"`
	Version  string `json:"version,omitempty"`
	DB       string `json:"db" validate:"required"`
	User     string `json:"user" validate:"required"`
	Password string `json:"password" validate:"required"`
	HostPort int    `json:"hostPort,omitempty"`
}

// UpdateServiceRequest is the JSON body for PUT /services/{id}.
// All fields are optional — omitted fields keep their current value.
type UpdateServiceRequest struct {
	Name     *string `json:"name,omitempty"`
	Version  *string `json:"version,omitempty"`
	Password *string `json:"password,omitempty"`
	HostPort *int    `json:"hostPort,omitempty"`
}

// StartServiceRequest is the JSON body for POST /services/{id}/start.
// NetworkName is mandatory — the service runs on the caller-chosen
// docker network so siblings can dial it by container name.
type StartServiceRequest struct {
	NetworkName string `json:"networkName" validate:"required"`
}

// @Summary List services
// @Tags Services
// @Produce json
// @Param serviceType query string false "Filter by service type (e.g. POSTGRES)"
// @Param status query string false "Filter by status (e.g. RUNNING, STOPPED)"
// @Param limit query int false "Page size (default 100)"
// @Param offset query int false "Offset"
// @Success 200 {array} svctypes.Service
// @Router /services [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	f := svcservice.ListFilter{
		ServiceType: svctypes.ServiceType(r.URL.Query().Get("serviceType")),
		Status:      r.URL.Query().Get("status"),
		Limit:       int64(parseInt32(r.URL.Query().Get("limit"), 100)),
		Offset:      int64(parseInt32(r.URL.Query().Get("offset"), 0)),
	}
	services, err := h.service.List(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, services)
}

// @Summary Create a POSTGRES service
// @Tags Services
// @Accept json
// @Produce json
// @Param request body CreatePostgresRequest true "Postgres creation request"
// @Success 201 {object} svctypes.Service
// @Router /services/postgres [post]
func (h *Handler) CreatePostgres(w http.ResponseWriter, r *http.Request) {
	var req CreatePostgresRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	svc, err := h.service.CreatePostgres(r.Context(), svcservice.CreatePostgresInput{
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

// @Summary Get a service
// @Tags Services
// @Produce json
// @Param id path int true "Service ID"
// @Success 200 {object} svctypes.Service
// @Router /services/{id} [get]
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	svc, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

// @Summary Update a service
// @Tags Services
// @Accept json
// @Produce json
// @Param id path int true "Service ID"
// @Param request body UpdateServiceRequest true "Mutable fields"
// @Success 200 {object} svctypes.Service
// @Router /services/{id} [put]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req UpdateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	svc, err := h.service.UpdatePostgres(r.Context(), id, svcservice.UpdatePostgresInput{
		Name:     req.Name,
		Version:  req.Version,
		Password: req.Password,
		HostPort: req.HostPort,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

// @Summary Delete a service
// @Tags Services
// @Param id path int true "Service ID"
// @Success 204
// @Router /services/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary Start a service on a docker network
// @Tags Services
// @Accept json
// @Produce json
// @Param id path int true "Service ID"
// @Param request body StartServiceRequest true "Network to run on"
// @Success 200 {object} svctypes.Service
// @Router /services/{id}/start [post]
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req StartServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	svc, err := h.service.StartPostgres(r.Context(), id, req.NetworkName)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, svc)
}

// @Summary Stop a service
// @Tags Services
// @Param id path int true "Service ID"
// @Success 204
// @Router /services/{id}/stop [post]
func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.service.Stop(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CreatePostgresDatabasesRequest is the JSON body for POST /services/{id}/postgres/databases.
type CreatePostgresDatabasesRequest struct {
	Databases []DatabaseSpec `json:"databases" validate:"required,min=1,dive"`
}

// DatabaseSpec is a single database+role to provision inside a running POSTGRES service.
type DatabaseSpec struct {
	DB       string `json:"db" validate:"required"`
	User     string `json:"user" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// @Summary Provision databases inside a running POSTGRES service
// @Description Creates (or updates) one role + database per entry. Idempotent: re-runs safely update the password.
// @Tags Services
// @Accept json
// @Produce json
// @Param id path int true "Service ID"
// @Param request body CreatePostgresDatabasesRequest true "Databases to provision"
// @Success 204
// @Router /services/{id}/postgres/databases [post]
func (h *Handler) CreatePostgresDatabases(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req CreatePostgresDatabasesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	specs := make([]svcservice.DatabaseSpec, 0, len(req.Databases))
	for _, d := range req.Databases {
		specs = append(specs, svcservice.DatabaseSpec{DB: d.DB, User: d.User, Password: d.Password})
	}
	if err := h.service.CreatePostgresDatabases(r.Context(), id, specs); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary List consumers of a service
// @Description Returns node_groups that reference this service via postgres_service_id.
// @Description Empty response means the service has no consumers and can be safely deleted.
// @Tags Services
// @Produce json
// @Param id path int true "Service ID"
// @Success 200 {array} svcservice.Consumer
// @Router /services/{id}/consumers [get]
func (h *Handler) Consumers(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	out, err := h.service.ListConsumers(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// LogsResponse is the JSON body of GET /services/{id}/logs.
type LogsResponse struct {
	Logs string `json:"logs"`
	Tail int    `json:"tail"`
}

// @Summary Get container logs for a service
// @Description Returns the last N lines of the service's container logs. Tail capped server-side.
// @Description Returns an empty logs string when the service has never been started.
// @Tags Services
// @Produce json
// @Param id path int true "Service ID"
// @Param tail query int false "Max lines to return (default 200, max 2000)"
// @Success 200 {object} LogsResponse
// @Router /services/{id}/logs [get]
func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	tail := parseInt32(r.URL.Query().Get("tail"), 200)
	if tail < 0 {
		tail = 200
	}
	tail = min(tail, 2000)
	out, err := h.service.GetLogs(r.Context(), id, int(tail))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, LogsResponse{Logs: out, Tail: int(tail)})
}

// --- helpers ---------------------------------------------------------

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
