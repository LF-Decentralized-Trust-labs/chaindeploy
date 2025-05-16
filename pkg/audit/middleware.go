package audit

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/auth"
	httputil "github.com/chainlaunch/chainlaunch/pkg/http"
	"github.com/google/uuid"
)

const (
	// maxBodySize is the maximum size of request/response body to log (1MB)
	maxBodySize = 1 * 1024 * 1024

	// SessionCookieName is the name of the session cookie
	SessionCookieName = "session_id"
)

// isStaticFile checks if the path is a static file
func isStaticFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm", ".css", ".js", ".json", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".woff", ".woff2", ".ttf", ".eot":
		return true
	}
	return false
}

// isAPIPath checks if the path is an API endpoint
func isAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/")
}

// isSecurityEvent checks if the request is a security-relevant event
func isSecurityEvent(path string, method string) bool {
	// Authentication endpoints
	if strings.Contains(path, "/auth") || strings.Contains(path, "/login") {
		return true
	}
	// Authorization changes
	if strings.Contains(path, "/permissions") || strings.Contains(path, "/roles") {
		return true
	}
	// System configuration changes
	if strings.Contains(path, "/config") || strings.Contains(path, "/settings") {
		return true
	}
	// Security-related operations
	if method == http.MethodDelete || method == http.MethodPut {
		return true
	}
	return false
}

// HTTPMiddleware creates a middleware that logs HTTP requests and responses
func HTTPMiddleware(service *AuditService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auditing for static files and non-API paths
			if isStaticFile(r.URL.Path) || !isAPIPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Skip auditing for audit log endpoints to prevent infinite loops
			if strings.Contains(r.URL.Path, "/api/v1/audit") {
				next.ServeHTTP(w, r)
				return
			}

			// Skip auditing for log streaming endpoints
			if strings.Contains(r.URL.Path, "/api/v1/nodes/") && strings.HasSuffix(r.URL.Path, "/logs") {
				next.ServeHTTP(w, r)
				return
			}

			// Generate a unique request ID and get session ID
			requestID := uuid.New()
			sessionID := auth.GetSessionID(r)
			if sessionID == "" {
				sessionID = uuid.New().String()
			}

			// Create a response writer that captures the status code and body
			rw := newResponseWriter(w)

			// Start timing the request with UTC timestamp
			start := time.Now().UTC()

			// Process the request
			next.ServeHTTP(rw, r)

			// Calculate request duration
			duration := time.Since(start)

			// Create base event details
			details := map[string]interface{}{
				"method":            r.Method,
				"path":              r.URL.Path,
				"query":             r.URL.RawQuery,
				"user_agent":        r.UserAgent(),
				"duration":          duration.String(),
				"status":            rw.statusCode,
				"session_id":        sessionID,
				"correlation_id":    r.Header.Get("X-Correlation-ID"),
				"timestamp_utc":     start.Format(time.RFC3339Nano),
				"client_ip":         r.RemoteAddr,
				"forwarded_for":     r.Header.Get("X-Forwarded-For"),
				"is_security_event": isSecurityEvent(r.URL.Path, r.Method),
			}

			// Get request body from resource context if available
			if resource, ok := httputil.ResourceFromContext(r); ok {
				if len(resource.Body) > 0 && len(resource.Body) <= maxBodySize {
					details["request_body"] = string(resource.Body)
				}
			}

			// Add response body for non-GET requests or error responses
			if (r.Method != http.MethodGet || rw.statusCode >= 400) && len(rw.body) > 0 && len(rw.body) <= maxBodySize {
				details["response_body"] = string(rw.body)
			}

			// Create audit event
			event := NewEvent().WithDetails(details)

			// Set event fields
			event.EventSource = "http"
			event.EventType = "http_request"
			event.RequestID = requestID
			event.SourceIP = r.RemoteAddr

			// Set resource information if available
			if resource, ok := httputil.ResourceFromContext(r); ok {
				event.AffectedResource = resource.Type
				if resource.ID != "" {
					event.AffectedResource += ":" + resource.ID
				}
				// Add resource action to details
				details["resource_action"] = resource.Action
			}

			// Set user identity and authentication method if available
			if user, ok := auth.UserFromContext(r.Context()); ok {
				event.UserIdentity = user.ID
				details["auth_method"] = r.Header.Get("X-Auth-Method")
				details["auth_provider"] = r.Header.Get("X-Auth-Provider")
			}

			// Set outcome based on status code
			if rw.statusCode >= 200 && rw.statusCode < 400 {
				event.EventOutcome = EventOutcomeSuccess
			} else {
				event.EventOutcome = EventOutcomeFailure
				// Add failure reason for security events
				if isSecurityEvent(r.URL.Path, r.Method) {
					details["failure_reason"] = http.StatusText(rw.statusCode)
				}
			}

			// Set severity based on status code and event type
			switch {
			case rw.statusCode >= 500:
				event.Severity = SeverityCritical
			case rw.statusCode >= 400:
				event.Severity = SeverityWarning
			case isSecurityEvent(r.URL.Path, r.Method):
				event.Severity = SeverityInfo
			default:
				event.Severity = SeverityDebug
			}

			// Log the event asynchronously
			service.LogEventAsync(event)
		})
	}
}

// responseWriter is a wrapper around http.ResponseWriter that captures the status code and body
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

// newResponseWriter creates a new responseWriter
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK, nil}
}

// WriteHeader captures the status code before writing it
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response body before writing it
func (rw *responseWriter) Write(b []byte) (int, error) {
	// Only capture body if it's not too large
	if len(rw.body)+len(b) <= maxBodySize {
		rw.body = append(rw.body, b...)
	}
	return rw.ResponseWriter.Write(b)
}
