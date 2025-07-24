package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/models"
	"github.com/chainlaunch/chainlaunch/pkg/keymanagement/service"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/sirupsen/logrus"
)

type KeyManagementHandler struct {
	service *service.KeyManagementService
}

func NewKeyManagementHandler(service *service.KeyManagementService) *KeyManagementHandler {
	return &KeyManagementHandler{
		service: service,
	}
}

// @Summary Get all keys
// @Description Get all keys with their certificates and metadata
// @Tags Keys
// @Accept json
// @Produce json
// @Success 200 {array} models.KeyResponse
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /keys/all [get]
// @BasePath /api/v1
func (h *KeyManagementHandler) GetAllKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.service.GetKeys(r.Context(), 1, 100)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.JSON(w, r, keys)
}

// @Summary Create a new key
// @Description Create a new key pair with specified algorithm and parameters
// @Tags Keys
// @Accept json
// @Produce json
// @Param request body models.CreateKeyRequest true "Key creation request"
// @Success 201 {object} models.KeyResponse
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security ApiKeyAuth
// @Router /keys [post]
// @BasePath /api/v1
func (h *KeyManagementHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	var req models.CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid request body"})
		return
	}

	// Get user from context (implement your auth middleware)
	userID := 1
	// userID := r.Context().Value("userID").(int)

	key, err := h.service.CreateKey(r.Context(), req, userID)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, key)
}

// Register routes
func (h *KeyManagementHandler) RegisterRoutes(r chi.Router) {
	r.Route("/keys", func(r chi.Router) {
		r.Get("/all", h.GetAllKeys)
		r.Post("/", h.CreateKey)
		r.Get("/", h.GetKeys)
		r.Get("/{id}", h.GetKey)
		r.Delete("/{id}", h.DeleteKey)
		r.Post("/{keyID}/sign", h.SignCertificate)
		r.Post("/{keyID}/sign-data", h.SignData)
		r.Get("/filter", h.FilterKeys)
	})

	r.Route("/key-providers", func(r chi.Router) {
		r.Post("/", h.CreateProvider)
		r.Get("/", h.ListProviders)
		r.Get("/{id}", h.GetProvider)
		r.Delete("/{id}", h.DeleteProvider)
	})
}

// @Summary Get paginated keys
// @Description Get a paginated list of keys
// @Tags Keys
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(10)
// @Success 200 {object} models.PaginatedResponse
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /keys [get]
// @BasePath /api/v1
func (h *KeyManagementHandler) GetKeys(w http.ResponseWriter, r *http.Request) {
	// Parse pagination parameters
	page := 1
	pageSize := 10

	// Get page from query params
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	// Get pageSize from query params
	if pageSizeStr := r.URL.Query().Get("pageSize"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	resp, err := h.service.GetKeys(r.Context(), page, pageSize)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.JSON(w, r, resp)
}

// @Summary Get a specific key by ID
// @Description Get detailed information about a specific key
// @Tags Keys
// @Accept json
// @Produce json
// @Param id path int true "Key ID"
// @Success 200 {object} models.KeyResponse
// @Failure 400 {object} map[string]string "Invalid ID"
// @Failure 404 {object} map[string]string "Key not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /keys/{id} [get]
// @BasePath /api/v1
func (h *KeyManagementHandler) GetKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid ID"})
		return
	}

	key, err := h.service.GetKey(r.Context(), id)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.JSON(w, r, key)
}

// @Summary Delete a key
// @Description Delete a specific key by ID
// @Tags Keys
// @Accept json
// @Produce json
// @Param id path int true "Key ID"
// @Success 204 "No Content"
// @Failure 400 {object} map[string]string "Invalid ID"
// @Failure 404 {object} map[string]string "Key not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /keys/{id} [delete]
// @BasePath /api/v1
func (h *KeyManagementHandler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid ID"})
		return
	}

	if err := h.service.DeleteKey(r.Context(), id); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.Status(r, http.StatusNoContent)
}

// @Summary Create a new key provider
// @Description Create a new provider for key management
// @Tags Providers
// @Accept json
// @Produce json
// @Param request body models.CreateProviderRequest true "Provider creation request"
// @Success 201 {object} models.ProviderResponse
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /key-providers [post]
// @BasePath /api/v1
func (h *KeyManagementHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var req models.CreateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid request body"})
		return
	}

	provider, err := h.service.CreateProvider(r.Context(), req)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, provider)
}

// @Summary List all key providers
// @Description Get a list of all configured key providers
// @Tags Providers
// @Accept json
// @Produce json
// @Success 200 {array} models.ProviderResponse
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /key-providers [get]
// @BasePath /api/v1
func (h *KeyManagementHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.service.ListProviders(r.Context())
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}
	logrus.Infof("providers: %+v", providers)
	render.JSON(w, r, providers)
}

// @Summary Get a specific provider
// @Description Get detailed information about a specific key provider
// @Tags Providers
// @Accept json
// @Produce json
// @Param id path int true "Provider ID"
// @Success 200 {object} models.ProviderResponse
// @Failure 400 {object} map[string]string "Invalid ID"
// @Failure 404 {object} map[string]string "Provider not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /key-providers/{id} [get]
// @BasePath /api/v1
func (h *KeyManagementHandler) GetProvider(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid ID"})
		return
	}

	provider, err := h.service.GetProviderByID(r.Context(), id)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.JSON(w, r, provider)
}

// @Summary Delete a provider
// @Description Delete a specific key provider
// @Tags Providers
// @Accept json
// @Produce json
// @Param id path int true "Provider ID"
// @Success 204 "No Content"
// @Failure 400 {object} map[string]string "Invalid ID"
// @Failure 404 {object} map[string]string "Provider not found"
// @Failure 409 {object} map[string]string "Provider has existing keys"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /key-providers/{id} [delete]
// @BasePath /api/v1
func (h *KeyManagementHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid ID"})
		return
	}

	if err := h.service.DeleteProvider(r.Context(), id); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.Status(r, http.StatusNoContent)
}

// @Summary Sign a certificate
// @Description Sign a certificate for a key using a CA key
// @Tags Keys
// @Accept json
// @Produce json
// @Param keyID path int true "Key ID to sign"
// @Param request body object true "Certificate signing request" SchemaExample({"caKeyId":1,"certificate":{"commonName":"example.com","organization":["Example Org"],"validFor":"8760h"}})
// @Success 200 {object} models.KeyResponse
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 404 {object} map[string]string "Key not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /keys/{keyID}/sign [post]
// @BasePath /api/v1
func (h *KeyManagementHandler) SignCertificate(w http.ResponseWriter, r *http.Request) {
	// Get key ID from URL
	keyIDStr := chi.URLParam(r, "keyID")
	keyID, err := strconv.Atoi(keyIDStr)
	if err != nil {
		http.Error(w, "Invalid key ID", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req struct {
		CAKeyID int                       `json:"caKeyId"`
		Cert    models.CertificateRequest `json:"certificate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Sign certificate
	key, err := h.service.SignCertificate(r.Context(), keyID, req.CAKeyID, req.Cert)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(key)
}

// @Summary Filter keys by algorithm and curve
// @Description Get keys filtered by algorithm type and/or curve type
// @Tags Keys
// @Accept json
// @Produce json
// @Param algorithm query string false "Algorithm type (e.g., RSA, ECDSA)"
// @Param curve query string false "Curve type (e.g., P256, P384, P521)"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(10)
// @Success 200 {object} models.PaginatedResponse
// @Failure 400 {object} map[string]string "Invalid parameters"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /keys/filter [get]
// @BasePath /api/v1
func (h *KeyManagementHandler) FilterKeys(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	algorithm := r.URL.Query().Get("algorithm")
	curve := r.URL.Query().Get("curve")

	// Parse pagination parameters with defaults
	page := 1
	pageSize := 10

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if sizeStr := r.URL.Query().Get("pageSize"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 {
			pageSize = s
		}
	}

	// Call service method with filters
	resp, err := h.service.FilterKeys(r.Context(), algorithm, curve, page, pageSize)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.JSON(w, r, resp)
}

// @Summary Sign data using a key
// @Description Sign data using a specific key with configurable parameters
// @Tags Keys
// @Accept json
// @Produce json
// @Param keyID path int true "Key ID to use for signing"
// @Param request body models.SignRequest true "Signing request"
// @Success 200 {object} models.SignResponse
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 404 {object} map[string]string "Key not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /keys/{keyID}/sign-data [post]
// @BasePath /api/v1
func (h *KeyManagementHandler) SignData(w http.ResponseWriter, r *http.Request) {
	// Get key ID from URL
	keyIDStr := chi.URLParam(r, "keyID")
	keyID, err := strconv.Atoi(keyIDStr)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid key ID"})
		return
	}

	// Parse request body
	var req models.SignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "Invalid request body"})
		return
	}

	// Sign the data
	resp, err := h.service.SignData(r.Context(), keyID, req)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}

	render.JSON(w, r, resp)
}
