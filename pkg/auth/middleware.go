package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"os"
	"strings"
)

const (
	SessionCookieName = "session_id"
)

// getEncryptionKey returns the session encryption key from environment variable
func getEncryptionKey() []byte {
	key := os.Getenv("SESSION_ENCRYPTION_KEY")
	if key == "" {
		// Fallback to a default key if not set (not recommended for production)
		key = "default-encryption-key-12345"
	}
	return []byte(key)
}

// signSessionID creates a signed version of the session ID
func signSessionID(sessionID string) string {
	mac := hmac.New(sha256.New, getEncryptionKey())
	mac.Write([]byte(sessionID))
	signature := mac.Sum(nil)
	return base64.URLEncoding.EncodeToString(signature)
}

// verifySessionID verifies the signature of a signed session ID
func verifySessionID(sessionID, signature string) bool {
	expectedSignature := signSessionID(sessionID)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// parseBasicAuth extracts credentials from Basic Auth header
func parseBasicAuth(r *http.Request) (username, password string, ok bool) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Basic ") {
		return "", "", false
	}

	payload, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		return "", "", false
	}

	pair := strings.SplitN(string(payload), ":", 2)
	if len(pair) != 2 {
		return "", "", false
	}

	return pair[0], pair[1], true
}

// GetSessionID extracts and validates the session ID from the request
// Returns the session ID if valid, empty string otherwise
func GetSessionID(r *http.Request) string {
	// Try Bearer token auth first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	// Try cookie auth
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}

	// Split cookie value into session ID and signature
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return ""
	}

	sessionID := parts[0]
	signature := parts[1]

	// Verify signature
	if !verifySessionID(sessionID, signature) {
		return ""
	}

	return sessionID
}

// AuthMiddleware validates the session token and adds the user to the context
func AuthMiddleware(authService *AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var user *User
			var err error

			// First try Basic Auth
			if username, password, ok := parseBasicAuth(r); ok {
				session, err := authService.Login(r.Context(), username, password)
				if err != nil {
					http.Error(w, "Invalid credentials", http.StatusUnauthorized)
					return
				}
				user, err = authService.GetUser(r.Context(), session.UserID)
				if err != nil {
					http.Error(w, "Invalid session", http.StatusUnauthorized)
					return
				}
			} else {
				// Try Bearer token auth
				var token string
				var sessionID string
				authHeader := r.Header.Get("Authorization")
				if authHeader != "" {
					parts := strings.Split(authHeader, " ")
					if len(parts) != 2 || parts[0] != "Bearer" {
						http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
						return
					}
					token = parts[1]
				} else {
					// Finally, try cookie auth
					cookie, err := r.Cookie(SessionCookieName)
					if err != nil {
						http.Error(w, "No authentication credentials found", http.StatusUnauthorized)
						return
					}

					// Split cookie value into session ID and signature
					parts := strings.Split(cookie.Value, ".")
					if len(parts) != 2 {
						http.Error(w, "Invalid session format", http.StatusUnauthorized)
						return
					}

					sessionID = parts[0]
					signature := parts[1]

					// Verify signature
					if !verifySessionID(sessionID, signature) {
						http.Error(w, "Invalid session signature", http.StatusUnauthorized)
						return
					}
				}

				// Validate session for both Bearer token and Cookie auth
				if token != "" {
					user, err = authService.ValidateSessionByToken(r.Context(), token)
					if err != nil {
						http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
						return
					}
				}
				if sessionID != "" {
					user, err = authService.ValidateSessionByID(r.Context(), sessionID)
					if err != nil {
						http.Error(w, "Invalid or expired session", http.StatusUnauthorized)
						return
					}
				}
			}

			if user == nil {
				http.Error(w, "Authentication failed", http.StatusUnauthorized)
				return
			}

			// Add user to context
			ctx := ContextWithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
