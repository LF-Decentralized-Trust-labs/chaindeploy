package response

import (
	"encoding/json"
	"fmt"
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

// DetailedErrorResponse represents an error response with dynamic details
// Use this for errors that need to return extra fields (e.g., model, tokenCount, etc.)
type DetailedErrorResponse struct {
	Error   string                 `json:"error"`
	Message string                 `json:"message,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ProblemDetail represents an RFC 7807 Problem Details error response
// See: https://datatracker.ietf.org/doc/html/rfc7807
type ProblemDetail struct {
	Type     string                 `json:"type"`               // URI reference identifying the problem type
	Title    string                 `json:"title"`              // Short, human-readable summary
	Status   int                    `json:"status"`             // HTTP status code
	Detail   string                 `json:"detail,omitempty"`   // Human-readable explanation specific to this occurrence
	Instance string                 `json:"instance,omitempty"` // URI reference identifying the specific occurrence
	Extra    map[string]interface{} `json:"-"`                  // Extension members
}

// MarshalJSON implements custom JSON marshaling for ProblemDetail to include extension members
func (p ProblemDetail) MarshalJSON() ([]byte, error) {
	type Alias ProblemDetail
	base := struct {
		Alias
	}{
		Alias: (Alias)(p),
	}

	// Create a map with the base fields
	m := make(map[string]interface{})
	data, err := json.Marshal(base)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	// Add extension members
	for k, v := range p.Extra {
		m[k] = v
	}

	return json.Marshal(m)
}

// DetailedResponse represents a flexible response with dynamic fields
// Use this for cases where the response structure is not known at compile time
// or when you want to return arbitrary key-value pairs.
type DetailedResponse map[string]interface{}

// NodeCreationErrorDetails represents structured error details for node creation failures
// This is used when a node creation partially succeeds (e.g., node created in DB but failed during startup)
type NodeCreationErrorDetails struct {
	// NodeCreated indicates whether the node was created in the database
	NodeCreated bool `json:"node_created"`

	// NodeID is the database ID of the node (only set if NodeCreated is true)
	NodeID int64 `json:"node_id,omitempty"`

	// Stage indicates at which stage the failure occurred (validation, db_insert, initialization, deployment_config, startup, connectivity)
	Stage string `json:"stage,omitempty"`

	// Node contains the partial node data (only set if NodeCreated is true)
	// This can be of type interface{} to accept any node type (FabricPeer, FabricOrderer, BesuNode)
	Node interface{} `json:"node,omitempty"`
}

// NodeCreationErrorResponse represents the complete error response for node creation failures
type NodeCreationErrorResponse struct {
	Error   string                   `json:"error"`
	Message string                   `json:"message"`
	Details NodeCreationErrorDetails `json:"details"`
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

// ProblemDetailError sends an RFC 7807 Problem Details error response
func ProblemDetailError(w http.ResponseWriter, problemType, title string, status int, detail, instance string, extra map[string]interface{}) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ProblemDetail{
		Type:     problemType,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
		Extra:    extra,
	})
}

// SimpleProblemDetailError sends a simple RFC 7807 Problem Details error response with just type, title, and status
func SimpleProblemDetailError(w http.ResponseWriter, problemType, title string, status int) {
	ProblemDetailError(w, problemType, title, status, "", "", nil)
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

// MultiValidationErrorResponse represents the response for multiple validation errors
type MultiValidationErrorResponse struct {
	Error   string                        `json:"error"`
	Message string                        `json:"message"`
	Errors  []errors.ValidationFieldError `json:"errors"`
}

// WriteError writes an error response
func WriteError(w http.ResponseWriter, err error) {
	var statusCode int

	// Check for MultiValidationError first (multiple field errors)
	if mve, ok := errors.GetMultiValidationError(err); ok {
		resp := MultiValidationErrorResponse{
			Error:   "validation_failed",
			Message: mve.Message,
			Errors:  mve.Errors,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(resp)
		return
	}

	var response Response

	switch e := err.(type) {
	case *errors.AppError:
		response = Response{
			Message: e.Message,
			Data:    e.Details,
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
			Message: fmt.Sprintf("An unexpected error occurred: %v", e.Error()),
		}
		statusCode = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// WriteMultiValidationError writes a multi-validation error response with 400 status
func WriteMultiValidationError(w http.ResponseWriter, mve *errors.MultiValidationError) error {
	resp := MultiValidationErrorResponse{
		Error:   "validation_failed",
		Message: mve.Message,
		Errors:  mve.Errors,
	}
	return WriteJSON(w, http.StatusBadRequest, resp)
}

// WriteNodeCreationError writes a properly typed node creation error response
// This is used for failures during node creation to provide structured error details
func WriteNodeCreationError(w http.ResponseWriter, errorCode, message string, details NodeCreationErrorDetails) error {
	resp := NodeCreationErrorResponse{
		Error:   errorCode,
		Message: message,
		Details: details,
	}
	return WriteJSON(w, http.StatusInternalServerError, resp)
}
