package errors

import (
	"fmt"

	"github.com/pkg/errors"
)

type ErrorType string

const (
	ValidationError     ErrorType = "VALIDATION_ERROR"
	NotFoundError       ErrorType = "NOT_FOUND"
	AuthenticationError ErrorType = "AUTHENTICATION_ERROR"
	AuthorizationError  ErrorType = "AUTHORIZATION_ERROR"
	DatabaseError       ErrorType = "DATABASE_ERROR"
	NetworkError        ErrorType = "NETWORK_ERROR"
	ConflictError       ErrorType = "CONFLICT_ERROR"
	InternalError       ErrorType = "INTERNAL_ERROR"
)

// AppError represents a custom application error
type AppError struct {
	Type    ErrorType              `json:"type"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
	Err     error                  `json:"-"` // Internal error, not exposed in JSON
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Helper functions to create specific error types
func NewValidationError(msg string, details map[string]interface{}) *AppError {
	return &AppError{
		Type:    ValidationError,
		Message: msg,
		Details: details,
	}
}

func NewNotFoundError(msg string, details map[string]interface{}) *AppError {
	return &AppError{
		Type:    NotFoundError,
		Message: msg,
		Details: details,
	}
}

func NewAuthenticationError(msg string, details map[string]interface{}) *AppError {
	return &AppError{
		Type:    AuthenticationError,
		Message: msg,
		Details: details,
	}
}

func NewAuthorizationError(msg string, details map[string]interface{}) *AppError {
	return &AppError{
		Type:    AuthorizationError,
		Message: msg,
		Details: details,
	}
}

func NewDatabaseError(msg string, err error, details map[string]interface{}) *AppError {
	return &AppError{
		Type:    DatabaseError,
		Message: msg,
		Details: details,
		Err:     err,
	}
}

func NewNetworkError(msg string, err error, details map[string]interface{}) *AppError {
	return &AppError{
		Type:    NetworkError,
		Message: msg,
		Details: details,
		Err:     err,
	}
}

func NewConflictError(msg string, details map[string]interface{}) *AppError {
	return &AppError{
		Type:    ConflictError,
		Message: msg,
		Details: details,
	}
}

func NewInternalError(msg string, err error, details map[string]interface{}) *AppError {
	return &AppError{
		Type:    InternalError,
		Message: errors.Wrap(err, msg).Error(),
		Details: details,
		Err:     err,
	}
}

func IsType(err error, target ErrorType) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == target
	}
	return false
}

// ValidationFieldError represents a single field validation error
type ValidationFieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"` // The invalid value (if safe to expose)
}

// MultiValidationError represents multiple validation errors
// This is used when a request has multiple invalid fields
type MultiValidationError struct {
	Message string                 `json:"message"`
	Errors  []ValidationFieldError `json:"errors"`
}

func (e *MultiValidationError) Error() string {
	if len(e.Errors) == 0 {
		return e.Message
	}
	return fmt.Sprintf("%s: %d validation error(s)", e.Message, len(e.Errors))
}

// Add adds a new validation error
func (e *MultiValidationError) Add(field, message string) {
	e.Errors = append(e.Errors, ValidationFieldError{
		Field:   field,
		Message: message,
	})
}

// AddWithValue adds a new validation error with the invalid value
func (e *MultiValidationError) AddWithValue(field, message, value string) {
	e.Errors = append(e.Errors, ValidationFieldError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// HasErrors returns true if there are any validation errors
func (e *MultiValidationError) HasErrors() bool {
	return len(e.Errors) > 0
}

// NewMultiValidationError creates a new MultiValidationError
func NewMultiValidationError(message string) *MultiValidationError {
	return &MultiValidationError{
		Message: message,
		Errors:  []ValidationFieldError{},
	}
}

// IsMultiValidationError checks if an error is a MultiValidationError
func IsMultiValidationError(err error) bool {
	_, ok := err.(*MultiValidationError)
	return ok
}

// GetMultiValidationError extracts MultiValidationError from an error if it exists
func GetMultiValidationError(err error) (*MultiValidationError, bool) {
	if mve, ok := err.(*MultiValidationError); ok {
		return mve, true
	}
	return nil, false
}
