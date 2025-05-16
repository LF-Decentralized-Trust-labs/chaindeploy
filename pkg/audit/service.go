package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/google/uuid"
)

// AuditService implements the Service interface
type AuditService struct {
	db       *db.Queries
	queue    chan Event
	workers  int
	wg       sync.WaitGroup
	stopChan chan struct{}
}

// NewService creates a new audit service
func NewService(db *db.Queries, workers int) *AuditService {
	if workers <= 0 {
		workers = 5 // Default number of workers
	}

	s := &AuditService{
		db:       db,
		queue:    make(chan Event, 1000), // Buffer size of 1000 events
		workers:  workers,
		stopChan: make(chan struct{}),
	}

	s.startWorkers()
	return s
}

func (s *AuditService) startWorkers() {
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}
}

// LogEvent implements the Service interface
func (s *AuditService) LogEvent(ctx context.Context, event Event) error {
	details, err := json.Marshal(event.Details)
	if err != nil {
		return err
	}

	// Create audit log using generated query
	_, err = s.db.CreateAuditLog(ctx, &db.CreateAuditLogParams{
		Timestamp:        event.Timestamp,
		EventSource:      event.EventSource,
		UserIdentity:     event.UserIdentity,
		SourceIp:         sql.NullString{String: event.SourceIP, Valid: true},
		EventType:        event.EventType,
		EventOutcome:     string(event.EventOutcome),
		AffectedResource: sql.NullString{String: event.AffectedResource, Valid: true},
		RequestID:        sql.NullString{String: event.RequestID.String(), Valid: true},
		Severity:         sql.NullString{String: string(event.Severity), Valid: true},
		Details:          sql.NullString{String: string(details), Valid: true},
		SessionID:        sql.NullString{String: event.SessionID, Valid: true},
	})

	return err
}

// LogEventAsync implements the Service interface
func (s *AuditService) LogEventAsync(event Event) {
	select {
	case s.queue <- event:
		// Event queued successfully
	default:
		// Queue is full, log error but don't block
	}
}

// worker processes events from the channel
func (s *AuditService) worker() {
	defer s.wg.Done()

	for {
		select {
		case event := <-s.queue:
			// Convert event to database model
			details, err := json.Marshal(event.Details)
			if err != nil {
				// Log error but continue processing
				continue
			}

			// Create audit log using generated query
			_, err = s.db.CreateAuditLog(context.Background(), &db.CreateAuditLogParams{
				Timestamp:        event.Timestamp,
				EventSource:      event.EventSource,
				UserIdentity:     event.UserIdentity,
				SourceIp:         sql.NullString{String: event.SourceIP, Valid: true},
				EventType:        event.EventType,
				EventOutcome:     string(event.EventOutcome),
				AffectedResource: sql.NullString{String: event.AffectedResource, Valid: true},
				RequestID:        sql.NullString{String: event.RequestID.String(), Valid: true},
				Severity:         sql.NullString{String: string(event.Severity), Valid: true},
				Details:          sql.NullString{String: string(details), Valid: true},
			})

			if err != nil {
				// Log error but continue processing
				continue
			}

		case <-s.stopChan:
			return
		}
	}
}

// Close stops the service and waits for all workers to finish
func (s *AuditService) Close() {
	close(s.stopChan)
	s.wg.Wait()
	close(s.queue)
}

// ListLogs implements the Service interface
func (s *AuditService) ListLogs(ctx context.Context, page, pageSize int, start, end *time.Time, eventType string, userID int64) (*ListLogsResponse, error) {
	offset := (page - 1) * pageSize

	// Convert time pointers to sql.NullTime
	var startTime, endTime sql.NullTime
	if start != nil {
		startTime.Time = *start
		startTime.Valid = true
	}
	if end != nil {
		endTime.Time = *end
		endTime.Valid = true
	}

	// Get logs using generated query
	logs, err := s.db.ListAuditLogs(ctx, &db.ListAuditLogsParams{
		Column1:      start,
		Timestamp:    startTime.Time,
		Column3:      end,
		Timestamp_2:  endTime.Time,
		Column5:      eventType,
		EventType:    eventType,
		Column7:      userID,
		UserIdentity: userID,
		Limit:        int64(pageSize),
		Offset:       int64(offset),
	})
	if err != nil {
		return nil, err
	}

	// Get total count using generated query
	total, err := s.db.CountAuditLogs(ctx, &db.CountAuditLogsParams{
		Column1:      start,
		Timestamp:    startTime.Time,
		Column3:      end,
		Timestamp_2:  endTime.Time,
		Column5:      eventType,
		EventType:    eventType,
		Column7:      userID,
		UserIdentity: userID,
	})
	if err != nil {
		return nil, err
	}

	// Convert database models to response models
	events := make([]Event, len(logs))
	for i, log := range logs {
		var details map[string]interface{}
		if err := json.Unmarshal([]byte(log.Details.String), &details); err != nil {
			return nil, err
		}

		requestID, err := uuid.Parse(log.RequestID.String)
		if err != nil {
			return nil, err
		}

		events[i] = Event{
			ID:               log.ID,
			Timestamp:        log.Timestamp,
			EventSource:      log.EventSource,
			UserIdentity:     log.UserIdentity,
			SourceIP:         log.SourceIp.String,
			EventType:        log.EventType,
			EventOutcome:     EventOutcome(log.EventOutcome),
			AffectedResource: log.AffectedResource.String,
			RequestID:        requestID,
			Severity:         Severity(log.Severity.String),
			Details:          details,
		}
	}

	return &ListLogsResponse{
		Items:      events,
		TotalCount: int(total),
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// GetLog implements the Service interface
func (s *AuditService) GetLog(ctx context.Context, id int64) (*Event, error) {
	log, err := s.db.GetAuditLog(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	var details map[string]interface{}
	if err := json.Unmarshal([]byte(log.Details.String), &details); err != nil {
		return nil, err
	}

	requestID, err := uuid.Parse(log.RequestID.String)
	if err != nil {
		return nil, err
	}

	return &Event{
		ID:               log.ID,
		Timestamp:        log.Timestamp,
		EventSource:      log.EventSource,
		UserIdentity:     log.UserIdentity,
		SourceIP:         log.SourceIp.String,
		EventType:        log.EventType,
		EventOutcome:     EventOutcome(log.EventOutcome),
		AffectedResource: log.AffectedResource.String,
		RequestID:        requestID,
		Severity:         Severity(log.Severity.String),
		Details:          details,
		SessionID:        log.SessionID.String,
	}, nil
}
