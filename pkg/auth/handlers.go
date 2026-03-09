package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/errors"
	"github.com/chainlaunch/chainlaunch/pkg/http/response"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	authService *AuthService
}

func NewHandler(authService *AuthService) *Handler {
	return &Handler{
		authService: authService,
	}
}

// RegisterRoutes registers all authentication and user management routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Auth routes
	r.Route("/auth", func(r chi.Router) {
		r.Post("/logout", response.Middleware(h.LogoutHandler))
		r.Get("/me", response.Middleware(h.GetCurrentUserHandler))
		r.Post("/change-password", response.Middleware(h.ChangePasswordHandler))
	})

	// User management routes
	r.Route("/users", func(r chi.Router) {
		r.Get("/", response.Middleware(h.ListUsersHandler))
		r.Post("/", response.Middleware(h.CreateUserHandler))
		r.Get("/{id}", response.Middleware(h.GetUserHandler))
		r.Put("/{id}", response.Middleware(h.UpdateUserHandler))
		r.Delete("/{id}", response.Middleware(h.DeleteUserHandler))
		r.Put("/{id}/password", response.Middleware(h.UpdateUserPasswordHandler))
		r.Put("/{id}/role", response.Middleware(h.UpdateUserRoleHandler))
	})
}

// @Summary Login user
// @Description Authenticates a user and returns a session cookie
// @Tags Authentication
// @Accept json
// @Produce json
// @Param credentials body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse "Login successful"
// @Failure 400 {object} response.Response "Invalid request body"
// @Failure 401 {object} response.Response "Invalid credentials"
// @Failure 405 {object} response.Response "Method not allowed"
// @Router /auth/login [post]
// @BasePath /api/v1
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return errors.NewValidationError("method not allowed", nil)
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", nil)
	}

	ctx := r.Context()
	session, err := h.authService.Login(ctx, req.Username, req.Password)
	if err != nil {
		log.Printf("Error logging in: %v", err)
		return errors.NewValidationError("invalid credentials", nil)
	}

	// Sign the session ID
	signature, err := signSessionID(session.ID)
	if err != nil {
		log.Printf("Error signing session ID: %v", err)
		return errors.NewInternalError("failed to sign session", err, nil)
	}
	signedSessionID := session.ID + "." + signature

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    signedSessionID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})

	return response.WriteJSON(w, http.StatusOK, LoginResponse{
		Message: "Login successful",
	})
}

// @Summary Logout user
// @Description Invalidates the current session and clears the session cookie
// @Tags Authentication
// @Produce json
// @Security CookieAuth
// @Success 200 {object} LogoutResponse "Logout successful"
// @Failure 405 {object} response.Response "Method not allowed"
// @Router /auth/logout [post]
// @BasePath /api/v1
func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return errors.NewValidationError("method not allowed", nil)
	}

	cookie, err := r.Cookie(SessionCookieName)
	if err == nil {
		h.authService.Logout(r.Context(), cookie.Value)
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})

	return response.WriteJSON(w, http.StatusOK, LogoutResponse{
		Message: "Logout successful",
	})
}

// @Summary Get current user
// @Description Returns information about the currently authenticated user
// @Tags Authentication
// @Produce json
// @Security CookieAuth
// @Success 200 {object} UserResponse "User information"
// @Failure 401 {object} response.Response "Unauthorized"
// @Router /auth/me [get]
// @BasePath /api/v1
func (h *Handler) GetCurrentUserHandler(w http.ResponseWriter, r *http.Request) error {
	session, ok := SessionFromContext(r.Context())
	if !ok {
		return errors.NewValidationError("unauthorized", nil)
	}

	user, err := h.authService.GetUserByUsername(session.Username)
	if err != nil {
		return errors.NewValidationError("user not found", nil)
	}

	userResponse := UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		Role:        user.Role,
		CreatedAt:   user.CreatedAt,
		LastLoginAt: user.LastLoginAt,
	}

	return response.WriteJSON(w, http.StatusOK, userResponse)
}

// @Summary Create new user
// @Description Creates a new user with specified role (admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param user body CreateUserRequest true "User to create"
// @Success 201 {object} UserResponse "User created"
// @Failure 400 {object} response.Response "Invalid request body"
// @Failure 401 {object} response.Response "Unauthorized"
// @Failure 403 {object} response.Response "Forbidden - Requires admin role"
// @Router /users [post]
// @BasePath /api/v1
func (h *Handler) CreateUserHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return errors.NewValidationError("method not allowed", nil)
	}

	session, ok := SessionFromContext(r.Context())
	if !ok {
		return errors.NewValidationError("unauthorized", nil)
	}

	if session.Role != RoleAdmin {
		return errors.NewValidationError("forbidden - requires admin role", nil)
	}

	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", nil)
	}

	user, err := h.authService.CreateUser(r.Context(), &req)
	if err != nil {
		return err
	}

	userResponse := UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		Role:        user.Role,
		CreatedAt:   user.CreatedAt,
		LastLoginAt: user.LastLoginAt,
	}

	return response.WriteJSON(w, http.StatusCreated, userResponse)
}

// @Summary List users
// @Description Returns a list of all users (admin only)
// @Tags Users
// @Produce json
// @Security CookieAuth
// @Success 200 {array} UserResponse "List of users"
// @Failure 401 {object} response.Response "Unauthorized"
// @Failure 403 {object} response.Response "Forbidden - Requires admin role"
// @Router /users [get]
// @BasePath /api/v1
func (h *Handler) ListUsersHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return errors.NewValidationError("method not allowed", nil)
	}

	session, ok := SessionFromContext(r.Context())
	if !ok {
		return errors.NewValidationError("unauthorized", nil)
	}

	if session.Role != RoleAdmin {
		return errors.NewValidationError("forbidden - requires admin role", nil)
	}

	users, err := h.authService.ListUsers(r.Context())
	if err != nil {
		return err
	}

	responses := make([]UserResponse, len(users))
	for i, user := range users {
		responses[i] = UserResponse{
			ID:          user.ID,
			Username:    user.Username,
			Role:        user.Role,
			CreatedAt:   user.CreatedAt,
			LastLoginAt: user.LastLoginAt,
		}
	}

	return response.WriteJSON(w, http.StatusOK, responses)
}

// @Summary Get user by ID
// @Description Get a user's details by ID (admin only)
// @Tags Users
// @Produce json
// @Security CookieAuth
// @Param id path int true "User ID"
// @Success 200 {object} UserResponse "User details"
// @Failure 401 {object} response.Response "Unauthorized"
// @Failure 403 {object} response.Response "Forbidden - Requires admin role"
// @Failure 404 {object} response.Response "User not found"
// @Router /users/{id} [get]
// @BasePath /api/v1
func (h *Handler) GetUserHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return errors.NewValidationError("method not allowed", nil)
	}

	session, ok := SessionFromContext(r.Context())
	if !ok {
		return errors.NewValidationError("unauthorized", nil)
	}

	if session.Role != RoleAdmin {
		return errors.NewValidationError("forbidden - requires admin role", nil)
	}

	userID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid user ID", nil)
	}

	user, err := h.authService.GetUserByID(r.Context(), userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewValidationError("user not found", nil)
		}
		return err
	}

	userResponse := UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		Role:        user.Role,
		CreatedAt:   user.CreatedAt,
		LastLoginAt: user.LastLoginAt,
	}

	return response.WriteJSON(w, http.StatusOK, userResponse)
}

// @Summary Update user
// @Description Updates an existing user (admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "User ID"
// @Param user body UpdateUserRequest true "User updates"
// @Success 200 {object} UserResponse "User updated"
// @Failure 400 {object} response.Response "Invalid request body"
// @Failure 401 {object} response.Response "Unauthorized"
// @Failure 403 {object} response.Response "Forbidden - Requires admin role"
// @Failure 404 {object} response.Response "User not found"
// @Router /users/{id} [put]
// @BasePath /api/v1
func (h *Handler) UpdateUserHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPut {
		return errors.NewValidationError("method not allowed", nil)
	}

	session, ok := SessionFromContext(r.Context())
	if !ok {
		return errors.NewValidationError("unauthorized", nil)
	}

	if session.Role != RoleAdmin {
		return errors.NewValidationError("forbidden - requires admin role", nil)
	}

	userID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid user ID", nil)
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", nil)
	}

	user, err := h.authService.UpdateUser(r.Context(), userID, &req)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewValidationError("user not found", nil)
		}
		return err
	}

	userResponse := UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		Role:        user.Role,
		CreatedAt:   user.CreatedAt,
		LastLoginAt: user.LastLoginAt,
	}

	return response.WriteJSON(w, http.StatusOK, userResponse)
}

// @Summary Delete user
// @Description Deletes a user (admin only)
// @Tags Users
// @Security CookieAuth
// @Param id path int true "User ID"
// @Success 204 "User deleted"
// @Failure 401 {object} response.Response "Unauthorized"
// @Failure 403 {object} response.Response "Forbidden - Requires admin role"
// @Failure 404 {object} response.Response "User not found"
// @Router /users/{id} [delete]
// @BasePath /api/v1
func (h *Handler) DeleteUserHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodDelete {
		return errors.NewValidationError("method not allowed", nil)
	}

	session, ok := SessionFromContext(r.Context())
	if !ok {
		return errors.NewValidationError("unauthorized", nil)
	}

	if session.Role != RoleAdmin {
		return errors.NewValidationError("forbidden - requires admin role", nil)
	}

	userID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid user ID", nil)
	}

	if userID == session.UserID {
		return errors.NewValidationError("cannot delete your own account", nil)
	}

	if err := h.authService.DeleteUser(r.Context(), userID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewValidationError("user not found", nil)
		}
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// @Summary Update user password
// @Description Update a user's password (admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "User ID"
// @Param request body ChangePasswordRequest true "New password"
// @Success 200 {object} map[string]string "Password updated successfully"
// @Failure 400 {object} response.Response "Invalid request body"
// @Failure 401 {object} response.Response "Unauthorized"
// @Failure 403 {object} response.Response "Forbidden - Requires admin role or self-modification not allowed"
// @Failure 404 {object} response.Response "User not found"
// @Router /users/{id}/password [put]
// @BasePath /api/v1
func (h *Handler) UpdateUserPasswordHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPut {
		return errors.NewValidationError("method not allowed", nil)
	}

	session, ok := SessionFromContext(r.Context())
	if !ok {
		return errors.NewValidationError("unauthorized", nil)
	}

	if session.Role != RoleAdmin {
		return errors.NewValidationError("forbidden - requires admin role", nil)
	}

	userID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid user ID", nil)
	}

	if userID == session.UserID {
		return errors.NewValidationError("cannot modify your own password through this endpoint", nil)
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", nil)
	}

	user, err := h.authService.GetUserByID(r.Context(), userID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewValidationError("user not found", nil)
		}
		return err
	}

	if err := h.authService.UpdateUserPassword(r.Context(), user.Username, req.NewPassword); err != nil {
		return err
	}

	return response.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Password updated successfully",
	})
}

// @Summary Update user role
// @Description Update a user's role (admin only)
// @Tags Users
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param id path int true "User ID"
// @Param request body UpdateUserRequest true "New role"
// @Success 200 {object} UserResponse "User role updated"
// @Failure 400 {object} response.Response "Invalid request body"
// @Failure 401 {object} response.Response "Unauthorized"
// @Failure 403 {object} response.Response "Forbidden - Requires admin role or self-modification not allowed"
// @Failure 404 {object} response.Response "User not found"
// @Router /users/{id}/role [put]
// @BasePath /api/v1
func (h *Handler) UpdateUserRoleHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPut {
		return errors.NewValidationError("method not allowed", nil)
	}

	session, ok := SessionFromContext(r.Context())
	if !ok {
		return errors.NewValidationError("unauthorized", nil)
	}

	if session.Role != RoleAdmin {
		return errors.NewValidationError("forbidden - requires admin role", nil)
	}

	userID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return errors.NewValidationError("invalid user ID", nil)
	}

	if userID == session.UserID {
		return errors.NewValidationError("cannot modify your own role", nil)
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", nil)
	}

	if req.Role == "" {
		return errors.NewValidationError("role is required", nil)
	}

	updatedUser, err := h.authService.UpdateUser(r.Context(), userID, &UpdateUserRequest{
		Role: req.Role,
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return errors.NewValidationError("user not found", nil)
		}
		return err
	}

	userResponse := UserResponse{
		ID:          updatedUser.ID,
		Username:    updatedUser.Username,
		Role:        updatedUser.Role,
		CreatedAt:   updatedUser.CreatedAt,
		LastLoginAt: updatedUser.LastLoginAt,
	}

	return response.WriteJSON(w, http.StatusOK, userResponse)
}

// ChangePasswordRequest represents the request to change a user's password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// @Summary Change own password
// @Description Allows a user to change their own password
// @Tags Authentication
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param request body ChangePasswordRequest true "Password change request"
// @Success 200 {object} map[string]string "Password changed successfully"
// @Failure 400 {object} response.Response "Invalid request body"
// @Failure 401 {object} response.Response "Unauthorized"
// @Failure 403 {object} response.Response "Invalid current password"
// @Router /auth/change-password [post]
// @BasePath /api/v1
func (h *Handler) ChangePasswordHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return errors.NewValidationError("method not allowed", nil)
	}

	// Get current user from context
	session, ok := SessionFromContext(r.Context())
	if !ok {
		return errors.NewValidationError("unauthorized", nil)
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return errors.NewValidationError("invalid request body", nil)
	}

	// Get user from database to verify current password
	user, err := h.authService.GetUserByUsername(session.Username)
	if err != nil {
		return errors.NewValidationError("user not found", nil)
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		return errors.NewValidationError("invalid current password", nil)
	}

	// Update password
	if err := h.authService.UpdateUserPassword(r.Context(), session.Username, req.NewPassword); err != nil {
		return errors.NewInternalError("failed to update password", err, nil)
	}

	return response.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Password changed successfully",
	})
}
