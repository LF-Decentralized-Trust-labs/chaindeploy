package http

import (
	"context"
	"io"
	"net/http"
	"strings"
)

// ResourceKey is the context key for the resource
type ResourceKey string

const (
	// ResourceContextKey is the key used to store the resource in the context
	ResourceContextKey ResourceKey = "resource"
)

// Resource represents an API resource
type Resource struct {
	// Type is the type of resource (e.g., "user", "project", "deployment")
	Type string
	// ID is the identifier of the resource (if applicable)
	ID string
	// Action is the action being performed on the resource (e.g., "create", "update", "delete")
	Action string
	// Body is the request body (if available)
	Body []byte
}

// WithResource adds a resource annotation to the request context
func WithResource(r *http.Request, resource Resource) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ResourceContextKey, resource))
}

// ResourceFromContext retrieves the resource from the request context
func ResourceFromContext(r *http.Request) (Resource, bool) {
	resource, ok := r.Context().Value(ResourceContextKey).(Resource)
	return resource, ok
}

// ResourceMiddleware creates a middleware that adds resource information to the request context
func ResourceMiddleware(resourceType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract resource ID from path if it exists
			// Assuming path format: /api/v1/{resourceType}/{id}
			parts := strings.Split(r.URL.Path, "/")
			var resourceID string
			if len(parts) >= 5 {
				resourceID = parts[4]
			}

			// Determine action based on HTTP method
			action := "view"
			switch r.Method {
			case http.MethodPost:
				action = "create"
			case http.MethodPut, http.MethodPatch:
				action = "update"
			case http.MethodDelete:
				action = "delete"
			}

			// Get request body from Chi's context if available
			var body []byte
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				if r.Body != nil {
					// Read the body
					body, _ = io.ReadAll(r.Body)
					// Restore the body for the actual handler
					r.Body = io.NopCloser(strings.NewReader(string(body)))
				}
			}

			resource := Resource{
				Type:   resourceType,
				ID:     resourceID,
				Action: action,
				Body:   body,
			}

			// Add resource to context
			r = WithResource(r, resource)
			next.ServeHTTP(w, r)
		})
	}
}
