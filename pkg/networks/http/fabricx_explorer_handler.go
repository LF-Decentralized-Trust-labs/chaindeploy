package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// FabricXChainInfoResponse is the response body for GET /networks/fabricx/{id}/chain-info.
type FabricXChainInfoResponse struct {
	Height uint64 `json:"height"`
}

// FabricXBlockListResponse wraps a page of blocks with the chain height as total.
type FabricXBlockListResponse struct {
	Blocks []FabricXBlockSummary `json:"blocks"`
	Total  uint64                `json:"total"`
}

// FabricXBlockSummary is the UI-safe view of a block without transaction bodies.
type FabricXBlockSummary struct {
	Number       uint64 `json:"number"`
	DataHash     string `json:"dataHash,omitempty"`
	PreviousHash string `json:"previousHash,omitempty"`
	TxCount      int    `json:"txCount"`
}

// FabricXNamespacePolicyResponse is one channel-namespace policy.
type FabricXNamespacePolicyResponse struct {
	Namespace string `json:"namespace"`
	Version   uint64 `json:"version"`
}

// FabricXStateRowResponse is one key/value row from the committer query service.
type FabricXStateRowResponse struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Version uint64 `json:"version"`
}

// @Summary Get FabricX chain info
// @Description Returns ledger height for a FabricX network by calling the committer sidecar's BlockQueryService.GetBlockchainInfo.
// @Tags FabricX Networks
// @Produce json
// @Param id path int true "Network ID"
// @Success 200 {object} FabricXChainInfoResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabricx/{id}/chain-info [get]
func (h *Handler) FabricXGetChainInfo(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	info, err := h.networkService.GetFabricXChainInfo(r.Context(), networkID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chain_info_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, FabricXChainInfoResponse{Height: info.Height})
}

// @Summary List FabricX blocks
// @Description Returns a paginated list of blocks by calling BlockQueryService.GetBlockByNumber against the committer sidecar. Latest-first when reverse=true.
// @Tags FabricX Networks
// @Produce json
// @Param id path int true "Network ID"
// @Param limit query int false "Number of blocks to return (default: 10)"
// @Param offset query int false "Number of blocks to skip from the chain tip (default: 0)"
// @Param reverse query bool false "Return newest first when true (default: true)"
// @Success 200 {object} FabricXBlockListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabricx/{id}/blocks [get]
func (h *Handler) FabricXGetBlocks(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	limit := int32(10)
	offset := int32(0)
	reverse := true
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			limit = int32(n)
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			offset = int32(n)
		}
	}
	if v := r.URL.Query().Get("reverse"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			reverse = b
		}
	}

	blocks, height, err := h.networkService.GetFabricXBlocks(r.Context(), networkID, limit, offset, reverse)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_blocks_failed", err.Error())
		return
	}
	summaries := make([]FabricXBlockSummary, 0, len(blocks))
	for _, b := range blocks {
		summaries = append(summaries, FabricXBlockSummary{
			Number:       b.Number,
			DataHash:     b.DataHash,
			PreviousHash: b.PreviousHash,
			TxCount:      b.TxCount,
		})
	}
	writeJSON(w, http.StatusOK, FabricXBlockListResponse{Blocks: summaries, Total: height})
}

// @Summary Get a FabricX block
// @Description Returns a fully decoded block (with tx headers) by calling BlockQueryService.GetBlockByNumber.
// @Tags FabricX Networks
// @Produce json
// @Param id path int true "Network ID"
// @Param blockNum path int true "Block number"
// @Success 200 {object} object
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabricx/{id}/blocks/{blockNum} [get]
func (h *Handler) FabricXGetBlock(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	num, err := strconv.ParseUint(chi.URLParam(r, "blockNum"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_block_number", "Invalid block number")
		return
	}
	block, err := h.networkService.GetFabricXBlock(r.Context(), networkID, num)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_block_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, block)
}

// @Summary Get a FabricX transaction
// @Description Returns a decoded transaction envelope (type, timestamp, namespaces, endorsers) by calling BlockQueryService.GetTxByID against the committer sidecar.
// @Tags FabricX Networks
// @Produce json
// @Param id path int true "Network ID"
// @Param txId path string true "Transaction ID"
// @Success 200 {object} object
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabricx/{id}/transactions/{txId} [get]
func (h *Handler) FabricXGetTransaction(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	txID := chi.URLParam(r, "txId")
	if txID == "" {
		writeError(w, http.StatusBadRequest, "invalid_tx_id", "Invalid transaction ID")
		return
	}
	tx, err := h.networkService.GetFabricXTransaction(r.Context(), networkID, txID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_transaction_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tx)
}

// @Summary List FabricX namespace policies
// @Description Returns the on-chain namespace policies from the committer query-service. These are the authoritative namespaces on the channel (distinct from the local namespace creation records).
// @Tags FabricX Networks
// @Produce json
// @Param id path int true "Network ID"
// @Success 200 {array} FabricXNamespacePolicyResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabricx/{id}/namespace-policies [get]
func (h *Handler) FabricXGetNamespacePolicies(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	policies, err := h.networkService.GetFabricXNamespacePolicies(r.Context(), networkID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_namespace_policies_failed", err.Error())
		return
	}
	out := make([]FabricXNamespacePolicyResponse, 0, len(policies))
	for _, p := range policies {
		out = append(out, FabricXNamespacePolicyResponse{Namespace: p.Namespace, Version: p.Version})
	}
	writeJSON(w, http.StatusOK, out)
}

// @Summary Query FabricX namespace state
// @Description Queries specific keys within a namespace via the committer query-service GetRows. FabricX does not expose a full scan, so the keys parameter is required and comma-separated.
// @Tags FabricX Networks
// @Produce json
// @Param id path int true "Network ID"
// @Param namespace path string true "Namespace ID"
// @Param keys query string true "Comma-separated list of keys to look up"
// @Success 200 {array} FabricXStateRowResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /networks/fabricx/{id}/state/{namespace} [get]
func (h *Handler) FabricXGetNamespaceState(w http.ResponseWriter, r *http.Request) {
	networkID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_network_id", "Invalid network ID")
		return
	}
	namespace := chi.URLParam(r, "namespace")
	if namespace == "" {
		writeError(w, http.StatusBadRequest, "invalid_namespace", "Invalid namespace")
		return
	}
	keysParam := r.URL.Query().Get("keys")
	var keys []string
	if keysParam != "" {
		for _, k := range strings.Split(keysParam, ",") {
			k = strings.TrimSpace(k)
			if k != "" {
				keys = append(keys, k)
			}
		}
	}
	rows, err := h.networkService.GetFabricXNamespaceState(r.Context(), networkID, namespace, keys)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_namespace_state_failed", err.Error())
		return
	}
	out := make([]FabricXStateRowResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, FabricXStateRowResponse{Key: row.Key, Value: row.Value, Version: row.Version})
	}
	writeJSON(w, http.StatusOK, out)
}
