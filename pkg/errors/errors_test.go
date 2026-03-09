package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- AppError tests ---

func TestAppError_Error_WithWrappedError(t *testing.T) {
	inner := fmt.Errorf("connection refused")
	err := &AppError{
		Type:    DatabaseError,
		Message: "failed to query",
		Err:     inner,
	}
	assert.Contains(t, err.Error(), "DATABASE_ERROR")
	assert.Contains(t, err.Error(), "failed to query")
	assert.Contains(t, err.Error(), "connection refused")
}

func TestAppError_Error_WithoutWrappedError(t *testing.T) {
	err := &AppError{
		Type:    ValidationError,
		Message: "name is required",
	}
	assert.Equal(t, "VALIDATION_ERROR: name is required", err.Error())
}

func TestNewValidationError(t *testing.T) {
	details := map[string]interface{}{"field": "name"}
	err := NewValidationError("bad input", details)
	assert.Equal(t, ValidationError, err.Type)
	assert.Equal(t, "bad input", err.Message)
	assert.Equal(t, details, err.Details)
	assert.Nil(t, err.Err)
}

func TestNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("node not found", nil)
	assert.Equal(t, NotFoundError, err.Type)
	assert.Equal(t, "node not found", err.Message)
}

func TestNewAuthenticationError(t *testing.T) {
	err := NewAuthenticationError("bad credentials", nil)
	assert.Equal(t, AuthenticationError, err.Type)
}

func TestNewAuthorizationError(t *testing.T) {
	err := NewAuthorizationError("not allowed", nil)
	assert.Equal(t, AuthorizationError, err.Type)
}

func TestNewDatabaseError(t *testing.T) {
	inner := fmt.Errorf("sqlite: locked")
	err := NewDatabaseError("query failed", inner, nil)
	assert.Equal(t, DatabaseError, err.Type)
	assert.Equal(t, inner, err.Err)
}

func TestNewNetworkError(t *testing.T) {
	inner := fmt.Errorf("timeout")
	err := NewNetworkError("unreachable", inner, nil)
	assert.Equal(t, NetworkError, err.Type)
	assert.Equal(t, inner, err.Err)
}

func TestNewConflictError(t *testing.T) {
	err := NewConflictError("already exists", nil)
	assert.Equal(t, ConflictError, err.Type)
}

func TestNewInternalError(t *testing.T) {
	inner := fmt.Errorf("nil pointer")
	err := NewInternalError("something broke", inner, nil)
	assert.Equal(t, InternalError, err.Type)
	assert.Equal(t, inner, err.Err)
	assert.Contains(t, err.Message, "something broke")
	assert.Contains(t, err.Message, "nil pointer")
}

func TestIsType(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		target   ErrorType
		expected bool
	}{
		{"matching type", NewValidationError("x", nil), ValidationError, true},
		{"non-matching type", NewValidationError("x", nil), NotFoundError, false},
		{"non-AppError", fmt.Errorf("plain"), ValidationError, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsType(tt.err, tt.target))
		})
	}
}

// --- MultiValidationError tests ---

func TestNewMultiValidationError(t *testing.T) {
	mve := NewMultiValidationError("Validation failed")
	require.NotNil(t, mve)
	assert.Equal(t, "Validation failed", mve.Message)
	assert.Empty(t, mve.Errors)
	assert.False(t, mve.HasErrors())
}

func TestMultiValidationError_Add(t *testing.T) {
	mve := NewMultiValidationError("Validation failed")
	mve.Add("name", "name is required")

	assert.True(t, mve.HasErrors())
	require.Len(t, mve.Errors, 1)
	assert.Equal(t, "name", mve.Errors[0].Field)
	assert.Equal(t, "name is required", mve.Errors[0].Message)
	assert.Empty(t, mve.Errors[0].Value)
}

func TestMultiValidationError_AddWithValue(t *testing.T) {
	mve := NewMultiValidationError("Validation failed")
	mve.AddWithValue("platform", "unsupported platform", "unknown")

	require.Len(t, mve.Errors, 1)
	assert.Equal(t, "platform", mve.Errors[0].Field)
	assert.Equal(t, "unsupported platform", mve.Errors[0].Message)
	assert.Equal(t, "unknown", mve.Errors[0].Value)
}

func TestMultiValidationError_MultipleErrors(t *testing.T) {
	mve := NewMultiValidationError("Node validation failed")
	mve.Add("name", "name is required")
	mve.Add("platform", "platform is required")
	mve.AddWithValue("port", "port out of range", "99999")

	assert.True(t, mve.HasErrors())
	assert.Len(t, mve.Errors, 3)
}

func TestMultiValidationError_Error_NoErrors(t *testing.T) {
	mve := NewMultiValidationError("Validation failed")
	assert.Equal(t, "Validation failed", mve.Error())
}

func TestMultiValidationError_Error_WithErrors(t *testing.T) {
	mve := NewMultiValidationError("Validation failed")
	mve.Add("a", "err1")
	mve.Add("b", "err2")

	assert.Equal(t, "Validation failed: 2 validation error(s)", mve.Error())
}

func TestMultiValidationError_HasErrors_EmptyAfterCreation(t *testing.T) {
	mve := NewMultiValidationError("test")
	assert.False(t, mve.HasErrors())

	mve.Add("field", "msg")
	assert.True(t, mve.HasErrors())
}

func TestIsMultiValidationError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"is MultiValidationError", NewMultiValidationError("test"), true},
		{"is AppError", NewValidationError("test", nil), false},
		{"is plain error", fmt.Errorf("plain"), false},
		{"is nil interface value", (*MultiValidationError)(nil), true}, // typed nil pointer
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsMultiValidationError(tt.err))
		})
	}
}

func TestGetMultiValidationError(t *testing.T) {
	t.Run("returns MVE when present", func(t *testing.T) {
		mve := NewMultiValidationError("test")
		mve.Add("f", "m")
		got, ok := GetMultiValidationError(mve)
		assert.True(t, ok)
		assert.Equal(t, mve, got)
	})

	t.Run("returns false for AppError", func(t *testing.T) {
		got, ok := GetMultiValidationError(NewValidationError("x", nil))
		assert.False(t, ok)
		assert.Nil(t, got)
	})

	t.Run("returns false for plain error", func(t *testing.T) {
		got, ok := GetMultiValidationError(fmt.Errorf("plain"))
		assert.False(t, ok)
		assert.Nil(t, got)
	})
}

func TestMultiValidationError_ImplementsErrorInterface(t *testing.T) {
	var err error = NewMultiValidationError("test")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "test")
}

func TestValidationFieldError_ZeroValue(t *testing.T) {
	var vfe ValidationFieldError
	assert.Empty(t, vfe.Field)
	assert.Empty(t, vfe.Message)
	assert.Empty(t, vfe.Value)
}
