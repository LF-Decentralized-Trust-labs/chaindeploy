package http

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/chainlaunch/chainlaunch/pkg/logger"
	"github.com/go-chi/chi/v5"
)

// Handler exposes lightweight host-introspection endpoints used by the UI
// to validate user-supplied configuration before submission. All endpoints
// are read-only and side-effect free.
type Handler struct {
	logger *logger.Logger
}

// NewHandler creates a new system handler.
func NewHandler(logger *logger.Logger) *Handler {
	return &Handler{logger: logger}
}

// RegisterRoutes registers the system routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/system", func(r chi.Router) {
		r.Get("/ports/free", h.GetPortsFree)
	})
}

// PortsFreeResponse reports which of the requested host TCP ports are
// currently bindable from the chainlaunch process. A port is reported as
// "free" only if net.Listen on 0.0.0.0:<port> succeeds — matching how the
// node container later requests the port via docker's port-publish
// (docker also fails the bind if anything else owns the port).
type PortsFreeResponse struct {
	Free []int `json:"free"`
	Busy []int `json:"busy"`
}

const maxPortsPerProbe = 256

// GetPortsFree probes the requested host ports and reports their availability.
//
// Query: ?ports=17000,17001,17010
// Each port must be in [1, 65535]; duplicates are coalesced; a comma-separated
// list larger than maxPortsPerProbe is rejected to keep the request bounded.
//
// The probe binds 0.0.0.0:<port> with SO_REUSEADDR off so the result reflects
// what docker would see when publishing the port. Any IPv6-only listener on
// the same port also blocks the bind on most kernels (dual-stack lookup), so
// this catches the realistic collision cases for the FabricX quickstart's
// docker-published ports.
func (h *Handler) GetPortsFree(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("ports"))
	if raw == "" {
		writeJSONError(w, http.StatusBadRequest, "missing required query parameter 'ports'")
		return
	}
	parts := strings.Split(raw, ",")
	if len(parts) > maxPortsPerProbe {
		writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("too many ports (max %d)", maxPortsPerProbe))
		return
	}
	seen := make(map[int]bool, len(parts))
	ports := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid port %q", p))
			return
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		ports = append(ports, n)
	}

	resp := PortsFreeResponse{Free: []int{}, Busy: []int{}}
	for _, port := range ports {
		if probePortFree(port) {
			resp.Free = append(resp.Free, port)
		} else {
			resp.Busy = append(resp.Busy, port)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Warnf("encode ports/free response: %v", err)
	}
}

// probePortFree reports whether 0.0.0.0:port is currently bindable. The
// listener is closed immediately; the only goal is to learn whether the
// kernel would accept a fresh bind (which is the same check docker performs
// when publishing a container port).
func probePortFree(port int) bool {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	l, err := net.Listen("tcp4", addr)
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// writeJSONError writes a small JSON error envelope; we don't pull in the
// project-wide response middleware here because system endpoints are
// intentionally minimal.
func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
