package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chainlaunch/chainlaunch/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ProblemDetail tests ---

func TestProblemDetail_MarshalJSON_BasicFields(t *testing.T) {
	pd := ProblemDetail{
		Type:   "about:blank",
		Title:  "Not Found",
		Status: 404,
		Detail: "The requested resource was not found",
	}
	data, err := json.Marshal(pd)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Equal(t, "about:blank", m["type"])
	assert.Equal(t, "Not Found", m["title"])
	assert.Equal(t, float64(404), m["status"])
	assert.Equal(t, "The requested resource was not found", m["detail"])
}

func TestProblemDetail_MarshalJSON_WithExtensionMembers(t *testing.T) {
	pd := ProblemDetail{
		Type:   "urn:problem:validation",
		Title:  "Validation Error",
		Status: 400,
		Extra: map[string]interface{}{
			"invalid_fields": []string{"name", "port"},
			"request_id":     "abc-123",
		},
	}
	data, err := json.Marshal(pd)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	assert.Equal(t, "urn:problem:validation", m["type"])
	assert.Equal(t, float64(400), m["status"])
	assert.Contains(t, m, "invalid_fields")
	assert.Equal(t, "abc-123", m["request_id"])
}

func TestProblemDetail_MarshalJSON_EmptyExtra(t *testing.T) {
	pd := ProblemDetail{
		Type:   "about:blank",
		Title:  "OK",
		Status: 200,
	}
	data, err := json.Marshal(pd)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))
	// Extra should not appear in output
	assert.NotContains(t, string(data), `"Extra"`)
}

func TestProblemDetail_MarshalJSON_OmitsEmptyOptionalFields(t *testing.T) {
	pd := ProblemDetail{
		Type:   "about:blank",
		Title:  "Error",
		Status: 500,
	}
	data, err := json.Marshal(pd)
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &m))

	// detail and instance should be omitted when empty
	_, hasDetail := m["detail"]
	_, hasInstance := m["instance"]
	assert.False(t, hasDetail, "empty detail should be omitted")
	assert.False(t, hasInstance, "empty instance should be omitted")
}

// --- WriteJSON tests ---

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	err := WriteJSON(w, http.StatusCreated, map[string]string{"id": "123"})
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "123", body["id"])
}

func TestWriteJSON_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	err := WriteJSON(w, http.StatusOK, nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "null")
}

// --- WriteError tests ---

func TestWriteError_AppError_ValidationError(t *testing.T) {
	w := httptest.NewRecorder()
	appErr := errors.NewValidationError("name is required", map[string]interface{}{"field": "name"})
	WriteError(w, appErr)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "name is required", body.Message)
}

func TestWriteError_AppError_NotFoundError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, errors.NewNotFoundError("node not found", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWriteError_AppError_AuthorizationError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, errors.NewAuthorizationError("not authorized", nil))
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWriteError_AppError_ConflictError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, errors.NewConflictError("already exists", nil))
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestWriteError_AppError_DatabaseError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, errors.NewDatabaseError("db fail", fmt.Errorf("locked"), nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWriteError_AppError_NetworkError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, errors.NewNetworkError("unreachable", fmt.Errorf("timeout"), nil))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestWriteError_GenericError_IncludesMessage(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, fmt.Errorf("something broke unexpectedly"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	// BF fix: should include the actual error message, not generic text
	assert.Contains(t, body.Message, "something broke unexpectedly")
	assert.Contains(t, body.Message, "An unexpected error occurred")
}

func TestWriteError_MultiValidationError(t *testing.T) {
	w := httptest.NewRecorder()

	mve := errors.NewMultiValidationError("Node validation failed")
	mve.Add("name", "name is required")
	mve.AddWithValue("port", "port out of range", "99999")

	WriteError(w, mve)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body MultiValidationErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	assert.Equal(t, "validation_failed", body.Error)
	assert.Equal(t, "Node validation failed", body.Message)
	require.Len(t, body.Errors, 2)
	assert.Equal(t, "name", body.Errors[0].Field)
	assert.Equal(t, "name is required", body.Errors[0].Message)
	assert.Equal(t, "port", body.Errors[1].Field)
	assert.Equal(t, "99999", body.Errors[1].Value)
}

func TestWriteError_MultiValidationError_Empty(t *testing.T) {
	w := httptest.NewRecorder()

	mve := errors.NewMultiValidationError("Validation failed")
	// No errors added — still a MVE
	WriteError(w, mve)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body MultiValidationErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "validation_failed", body.Error)
	assert.Empty(t, body.Errors)
}

func TestWriteError_MultiValidationError_TakesPrecedenceOverAppError(t *testing.T) {
	// MultiValidationError should be checked before AppError type assertion
	w := httptest.NewRecorder()
	mve := errors.NewMultiValidationError("test")
	mve.Add("f", "m")
	WriteError(w, mve)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- WriteMultiValidationError tests ---

func TestWriteMultiValidationError(t *testing.T) {
	w := httptest.NewRecorder()

	mve := errors.NewMultiValidationError("Form errors")
	mve.Add("email", "invalid email format")
	mve.AddWithValue("age", "must be positive", "-5")

	err := WriteMultiValidationError(w, mve)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body MultiValidationErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "validation_failed", body.Error)
	assert.Len(t, body.Errors, 2)
}

// --- WriteNodeCreationError tests ---

func TestWriteNodeCreationError(t *testing.T) {
	w := httptest.NewRecorder()

	details := NodeCreationErrorDetails{
		NodeCreated: true,
		NodeID:      42,
		Stage:       "startup",
	}

	err := WriteNodeCreationError(w, "node_startup_failed", "Node started but connectivity check failed", details)
	require.NoError(t, err)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body NodeCreationErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "node_startup_failed", body.Error)
	assert.Equal(t, "Node started but connectivity check failed", body.Message)
	assert.True(t, body.Details.NodeCreated)
	assert.Equal(t, int64(42), body.Details.NodeID)
	assert.Equal(t, "startup", body.Details.Stage)
}

func TestWriteNodeCreationError_NodeNotCreated(t *testing.T) {
	w := httptest.NewRecorder()

	details := NodeCreationErrorDetails{
		NodeCreated: false,
		Stage:       "validation",
	}

	err := WriteNodeCreationError(w, "validation_error", "Invalid config", details)
	require.NoError(t, err)

	var body NodeCreationErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.False(t, body.Details.NodeCreated)
	assert.Equal(t, int64(0), body.Details.NodeID)
}

// --- ProblemDetailError HTTP tests ---

func TestProblemDetailError_HTTP(t *testing.T) {
	w := httptest.NewRecorder()
	ProblemDetailError(w, "urn:problem:rate-limited", "Rate Limited", 429, "Too many requests", "/api/v1/nodes", map[string]interface{}{
		"retry_after": 30,
	})

	assert.Equal(t, 429, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "urn:problem:rate-limited", body["type"])
	assert.Equal(t, "Rate Limited", body["title"])
	assert.Equal(t, float64(429), body["status"])
	assert.Equal(t, "Too many requests", body["detail"])
	assert.Equal(t, "/api/v1/nodes", body["instance"])
	assert.Equal(t, float64(30), body["retry_after"])
}

func TestSimpleProblemDetailError_HTTP(t *testing.T) {
	w := httptest.NewRecorder()
	SimpleProblemDetailError(w, "about:blank", "Internal Server Error", 500)

	assert.Equal(t, 500, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))
}

// --- Middleware tests ---

func TestMiddleware_NoError(t *testing.T) {
	handler := Middleware(func(w http.ResponseWriter, r *http.Request) error {
		return WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMiddleware_WithAppError(t *testing.T) {
	handler := Middleware(func(w http.ResponseWriter, r *http.Request) error {
		return errors.NewNotFoundError("not found", nil)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMiddleware_WithMultiValidationError(t *testing.T) {
	handler := Middleware(func(w http.ResponseWriter, r *http.Request) error {
		mve := errors.NewMultiValidationError("bad request")
		mve.Add("field1", "error1")
		return mve
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMiddleware_WithGenericError(t *testing.T) {
	handler := Middleware(func(w http.ResponseWriter, r *http.Request) error {
		return fmt.Errorf("unexpected failure")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body.Message, "unexpected failure")
}

// --- JSON and Error helper tests ---

func TestJSON_Helper(t *testing.T) {
	w := httptest.NewRecorder()
	JSON(w, http.StatusAccepted, map[string]int{"count": 5})

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestError_Helper(t *testing.T) {
	w := httptest.NewRecorder()
	Error(w, http.StatusForbidden, "access denied")

	assert.Equal(t, http.StatusForbidden, w.Code)

	var body ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "access denied", body.Error)
}
