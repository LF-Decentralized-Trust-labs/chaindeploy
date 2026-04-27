package http

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/networks/service/fabricx"
)

// CreateFabricXNamespaceRequest is the HTTP payload for creating or updating a
// namespace inside a FabricX channel. Namespaces are logical partitions written
// to the reserved _meta namespace; each has its own endorsement policy keyed
// by the name string.
type CreateFabricXNamespaceRequest struct {
	Name            string `json:"name" validate:"required"`
	Version         int    `json:"version"` // -1 (default) creates a new ns; >=0 updates an existing one
	SubmitterOrgID  int64  `json:"submitterOrgId" validate:"required"`
	WaitForFinality bool   `json:"waitForFinality"`
	FinalityTimeoutSeconds int `json:"finalityTimeoutSeconds,omitempty"`
	// EndorsementPublicKeyPEM is an optional PEM-encoded public key to store as
	// the namespace's endorsement policy. If omitted, the submitter org's admin
	// pubkey is used. Typical use-case: token-sdk-x FSC endorsers that sign
	// token txs with a peer identity rather than the org admin identity.
	EndorsementPublicKeyPEM string `json:"endorsementPublicKeyPEM,omitempty"`
}

// FabricXNamespaceResponse is the HTTP-serializable view of a namespace row.
// It merges chain state (authoritative: name, version, onChain) with local DB
// submission metadata (id, submitter, txId, createdAt) when a row exists.
//
// source: "chain" — only on-chain (created outside ChainLaunch or DB wiped)
// source: "db"    — only locally (PENDING/FAILED submission that never committed)
// source: "both"  — on-chain and tracked locally
type FabricXNamespaceResponse struct {
	ID             *int64  `json:"id,omitempty"`
	NetworkID      *int64  `json:"networkId,omitempty"`
	Name           string  `json:"name"`
	Version        uint64  `json:"version"`
	OnChain        bool    `json:"onChain"`
	Source         string  `json:"source"`
	SubmitterMspID string  `json:"submitterMspId,omitempty"`
	SubmitterOrgID *int64  `json:"submitterOrgId,omitempty"`
	TxID           *string `json:"txId,omitempty"`
	Status         string  `json:"status"`
	Error          *string `json:"error,omitempty"`
	CreatedAt      *string `json:"createdAt,omitempty"`
	UpdatedAt      *string `json:"updatedAt,omitempty"`
}

// FabricXNamespaceListResponse wraps the merged list with a non-fatal chain
// error so the UI can show a "committer unreachable, DB-only view" banner.
type FabricXNamespaceListResponse struct {
	Namespaces []FabricXNamespaceResponse `json:"namespaces"`
	ChainError string                     `json:"chainError,omitempty"`
}

// @Summary Create a FabricX namespace
// @Description Creates (or updates) a namespace within a FabricX channel by broadcasting a signed applicationpb.Tx to the channel's orderer router. Requires an existing network whose orderer group is running.
// @Tags FabricX Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Param request body CreateFabricXNamespaceRequest true "Namespace creation request"
// @Success 201 {object} FabricXNamespaceResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabricx/{id}/namespaces [post]
func (h *Handler) FabricXNamespaceCreate(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	var req CreateFabricXNamespaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "name is required")
		return
	}
	if req.SubmitterOrgID == 0 {
		writeError(w, http.StatusBadRequest, "missing_submitter", "submitterOrgId is required")
		return
	}
	// Default version of 0 is ambiguous — callers should pass -1 for new. We
	// let NamespaceCreateOptions normalize it.
	version := req.Version
	if version == 0 {
		version = -1
	}

	opts := fabricx.NamespaceCreateOptions{
		Name:            req.Name,
		Version:         version,
		SubmitterOrgID:  req.SubmitterOrgID,
		WaitForFinality: req.WaitForFinality,
	}
	if req.FinalityTimeoutSeconds > 0 {
		opts.FinalityTimeout = time.Duration(req.FinalityTimeoutSeconds) * time.Second
	}
	if strings.TrimSpace(req.EndorsementPublicKeyPEM) != "" {
		opts.EndorsementPublicKeyPEM = []byte(req.EndorsementPublicKeyPEM)
	}

	result, err := h.networkService.CreateFabricXNamespace(r.Context(), networkID, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "namespace_create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, mapFabricXNamespaceToResponse(result.Row))
}

// @Summary List FabricX namespaces (merged chain + DB view)
// @Description Returns the merged view of namespaces for a FabricX network. The chain is the source of truth for existence and version (queried live from the committer query-service via GetNamespacePolicies); the local DB adds submission metadata (submitter, txId, pending/failed attempts). Rows carry source ∈ {chain, db, both}. If the committer is unreachable, chainError is set and only DB rows are returned.
// @Tags FabricX Networks
// @Produce json
// @Param id path int true "Network ID"
// @Success 200 {object} FabricXNamespaceListResponse
// @Router /networks/fabricx/{id}/namespaces [get]
func (h *Handler) FabricXNamespaceList(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	views, chainErr, err := h.networkService.ListFabricXNamespacesMerged(r.Context(), networkID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "namespace_list_failed", err.Error())
		return
	}
	out := FabricXNamespaceListResponse{
		Namespaces: make([]FabricXNamespaceResponse, 0, len(views)),
	}
	for _, v := range views {
		out.Namespaces = append(out.Namespaces, mapFabricXNamespaceViewToResponse(v))
	}
	if chainErr != nil {
		out.ChainError = chainErr.Error()
	}
	writeJSON(w, http.StatusOK, out)
}

// @Summary Delete a FabricX namespace record (local only)
// @Description Removes the local database record for the namespace. On-chain the namespace is not removed — FabricX supports only namespace updates, not deletion.
// @Tags FabricX Networks
// @Param id path int true "Network ID"
// @Param namespaceId path int true "Namespace record ID"
// @Success 204 "Deleted"
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabricx/{id}/namespaces/{namespaceId} [delete]
func (h *Handler) FabricXNamespaceDelete(w http.ResponseWriter, r *http.Request) {
	nsID, err := strconv.ParseInt(chi.URLParam(r, "namespaceId"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_namespace_id", "Invalid namespace ID")
		return
	}
	if err := h.networkService.DeleteFabricXNamespaceRecord(r.Context(), nsID); err != nil {
		writeError(w, http.StatusInternalServerError, "namespace_delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// mapFabricXNamespaceToResponse maps the DB row shape returned by
// CreateFabricXNamespace into the merged response. Create happens before the
// chain is queried, so OnChain reflects the DB status (COMMITTED only once
// finality wait succeeded) and source is "db".
func mapFabricXNamespaceToResponse(row *db.FabricxNamespace) FabricXNamespaceResponse {
	id := row.ID
	nid := row.NetworkID
	createdAt := row.CreatedAt.Format(time.RFC3339)
	version := uint64(0)
	if row.Version >= 0 {
		version = uint64(row.Version)
	}
	resp := FabricXNamespaceResponse{
		ID:             &id,
		NetworkID:      &nid,
		Name:           row.Name,
		Version:        version,
		OnChain:        row.Status == "COMMITTED",
		Source:         "db",
		SubmitterMspID: row.SubmitterMspID,
		Status:         row.Status,
		CreatedAt:      &createdAt,
	}
	if row.SubmitterOrgID.Valid {
		v := row.SubmitterOrgID.Int64
		resp.SubmitterOrgID = &v
	}
	if row.TxID.Valid {
		v := row.TxID.String
		resp.TxID = &v
	}
	if row.Error.Valid {
		v := row.Error.String
		resp.Error = &v
	}
	if row.UpdatedAt.Valid {
		v := row.UpdatedAt.Time.Format(time.RFC3339)
		resp.UpdatedAt = &v
	}
	return resp
}

func mapFabricXNamespaceViewToResponse(v fabricx.NamespaceView) FabricXNamespaceResponse {
	resp := FabricXNamespaceResponse{
		Name:           v.Name,
		Version:        v.Version,
		OnChain:        v.OnChain,
		Source:         v.Source,
		SubmitterMspID: v.SubmitterMspID,
		SubmitterOrgID: v.SubmitterOrgID,
		TxID:           v.TxID,
		Status:         v.Status,
		Error:          v.Error,
	}
	if v.ID != nil {
		resp.ID = v.ID
	}
	if v.NetworkID != nil {
		resp.NetworkID = v.NetworkID
	}
	if v.CreatedAt != nil {
		s := v.CreatedAt.Format(time.RFC3339)
		resp.CreatedAt = &s
	}
	if v.UpdatedAt != nil {
		s := v.UpdatedAt.Format(time.RFC3339)
		resp.UpdatedAt = &s
	}
	return resp
}

// PublishPublicParamsRequest is the HTTP payload for publishing token-sdk ZK
// public parameters into a FabricX namespace.
type PublishPublicParamsRequest struct {
	// SubmitterOrgID is the fabric_organizations.id whose admin key signs the tx.
	SubmitterOrgID int64 `json:"submitterOrgId" validate:"required"`
	// PublicParametersB64 is the base64-encoded raw PP bytes
	// (e.g. the contents of zkatdlognoghv1_pp.json).
	PublicParametersB64 string `json:"publicParametersB64" validate:"required"`
	// WaitForFinality, when true, blocks until the tx is committed on-chain.
	WaitForFinality bool `json:"waitForFinality"`
	// FinalityTimeoutSeconds overrides the default 120-second finality window.
	FinalityTimeoutSeconds int `json:"finalityTimeoutSeconds,omitempty"`
}

// PublishPublicParamsResponse is returned by FabricXNamespacePublishPublicParams.
type PublishPublicParamsResponse struct {
	// TxID is the fabric transaction identifier that was broadcast.
	TxID string `json:"txId"`
	// Status is "broadcast" or "committed".
	Status string `json:"status"`
	// Namespace echoes the namespace name from the URL path.
	Namespace string `json:"namespace"`
}

// @Summary Publish token-sdk public parameters into a FabricX namespace
// @Description Writes the ZK public parameters (PPs) for fabric-token-sdk into
// the named FabricX namespace by broadcasting a signed applicationpb.Tx. The PP
// tx uses BlindWrites for the setup key ("\x00se\x00") and setup hash key
// ("\x00seh\x00"), and a ReadsOnly guard on "initialized". Without published PPs
// every token-sdk issue/transfer call fails with "no public params found".
// @Tags FabricX Networks
// @Accept json
// @Produce json
// @Param id path int true "Network ID"
// @Param name path string true "Namespace name (e.g. token_namespace)"
// @Param request body PublishPublicParamsRequest true "Publish public parameters request"
// @Success 200 {object} PublishPublicParamsResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabricx/{id}/namespaces/{name}/public-params [post]
func (h *Handler) FabricXNamespacePublishPublicParams(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "namespace name is required")
		return
	}

	var req PublishPublicParamsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	if req.SubmitterOrgID == 0 {
		writeError(w, http.StatusBadRequest, "missing_submitter", "submitterOrgId is required")
		return
	}
	if req.PublicParametersB64 == "" {
		writeError(w, http.StatusBadRequest, "missing_public_params", "publicParametersB64 is required")
		return
	}

	ppRaw, err := base64.StdEncoding.DecodeString(req.PublicParametersB64)
	if err != nil {
		// Try URL-safe base64 as a fallback — callers may encode with either variant.
		ppRaw, err = base64.URLEncoding.DecodeString(req.PublicParametersB64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_public_params", "publicParametersB64 is not valid base64")
			return
		}
	}
	if len(ppRaw) == 0 {
		writeError(w, http.StatusBadRequest, "empty_public_params", "publicParametersB64 decoded to empty bytes")
		return
	}

	opts := fabricx.PublishPublicParamsOptions{
		Name:             name,
		SubmitterOrgID:   req.SubmitterOrgID,
		PublicParameters: ppRaw,
		WaitForFinality:  req.WaitForFinality,
	}
	if req.FinalityTimeoutSeconds > 0 {
		opts.FinalityTimeout = time.Duration(req.FinalityTimeoutSeconds) * time.Second
	}

	result, err := h.networkService.PublishFabricXPublicParams(r.Context(), networkID, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "publish_public_params_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, PublishPublicParamsResponse{
		TxID:      result.TxID,
		Status:    result.Status,
		Namespace: name,
	})
}

// silences unused import warning when the file is edited in isolation.
var _ sql.NullString
