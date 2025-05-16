package response

import (
	"encoding/json"
	"net/http"

	"github.com/chainlaunch/chainlaunch/pkg/errors"
)

// Response represents a standard API response
type Response struct {
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// JSON sends a JSON response
func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Error sends an error response
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, ErrorResponse{
		Error: message,
	})
}

// Handler is a custom type for http handlers that can return errors
type Handler func(w http.ResponseWriter, r *http.Request) error

// Middleware converts our custom handler to standard http.HandlerFunc
func Middleware(h Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err != nil {
			WriteError(w, err)
			return
		}
	}
}

// WriteJSON writes a JSON response
func WriteJSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

// WriteError writes an error response
func WriteError(w http.ResponseWriter, err error) {
	var response Response
	var statusCode int

	switch e := err.(type) {
	case *errors.AppError:
		response = Response{
			Message: e.Message,
		}

		// Map error types to HTTP status codes
		switch e.Type {
		case errors.ValidationError:
			statusCode = http.StatusBadRequest
		case errors.NotFoundError:
			statusCode = http.StatusNotFound
		case errors.AuthorizationError:
			statusCode = http.StatusUnauthorized
		case errors.ConflictError:
			statusCode = http.StatusConflict
		case errors.DatabaseError:
			statusCode = http.StatusInternalServerError
		case errors.NetworkError:
			statusCode = http.StatusServiceUnavailable
		default:
			statusCode = http.StatusInternalServerError
		}
	default:
		response = Response{
			Message: "An unexpected error occurred",
		}
		statusCode = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
