package audit

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventOutcome represents the result of an audited event
type EventOutcome string

const (
	EventOutcomeSuccess EventOutcome = "SUCCESS"
	EventOutcomeFailure EventOutcome = "FAILURE"
	EventOutcomePending EventOutcome = "PENDING"
)

// Severity represents the importance level of an audit event
type Severity string

const (
	SeverityDebug    Severity = "DEBUG"
	SeverityInfo     Severity = "INFO"
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
)

// Event represents an audit event to be logged
type Event struct {
	ID               int64                  `json:"id"`
	Timestamp        time.Time              `json:"timestamp"`
	EventSource      string                 `json:"eventSource"`
	UserIdentity     int64                  `json:"userIdentity"`
	SourceIP         string                 `json:"sourceIp"`
	EventType        string                 `json:"eventType"`
	EventOutcome     EventOutcome           `json:"eventOutcome"`
	AffectedResource string                 `json:"affectedResource"`
	RequestID        uuid.UUID              `json:"requestId"`
	Severity         Severity               `json:"severity"`
	Details          map[string]interface{} `json:"details"`
	SessionID        string                 `json:"sessionId"`
}

// Config holds the configuration for the audit service
type Config struct {
	// AsyncBufferSize is the size of the buffer for async logging
	AsyncBufferSize int
	// WorkerCount is the number of workers for async logging
	WorkerCount int
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		AsyncBufferSize: 1000,
		WorkerCount:     5,
	}
}

// NewEvent creates a new audit event with default values
func NewEvent() Event {
	return Event{
		Timestamp:    time.Now().UTC(),
		EventOutcome: EventOutcomeSuccess,
		Severity:     SeverityInfo,
		Details:      make(map[string]interface{}),
	}
}

// WithDetails adds details to the event
func (e Event) WithDetails(details map[string]interface{}) Event {
	e.Details = details
	return e
}

// WithSeverity sets the severity of the event
func (e Event) WithSeverity(severity Severity) Event {
	e.Severity = severity
	return e
}

// WithOutcome sets the outcome of the event
func (e Event) WithOutcome(outcome EventOutcome) Event {
	e.EventOutcome = outcome
	return e
}

// ToJSON converts the event to a JSON string
func (e Event) ToJSON() (string, error) {
	data, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
