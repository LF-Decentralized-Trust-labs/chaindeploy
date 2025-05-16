package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"github.com/chainlaunch/chainlaunch/pkg/logger"
)

// NodeEventType represents the type of node event
type NodeEventType string

const (
	NodeEventStarting             NodeEventType = "STARTING"
	NodeEventStarted              NodeEventType = "STARTED"
	NodeEventStopping             NodeEventType = "STOPPING"
	NodeEventStopped              NodeEventType = "STOPPED"
	NodeEventError                NodeEventType = "ERROR"
	NodeEventRenewingCertificates NodeEventType = "RENEWING_CERTIFICATES"
	NodeEventRenewedCertificates  NodeEventType = "RENEWED_CERTIFICATES"
)

// NodeEvent represents a node event in the service layer
type NodeEvent struct {
	ID        int64         `json:"id"`
	NodeID    int64         `json:"node_id"`
	Type      NodeEventType `json:"type"`
	Data      interface{}   `json:"data,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

// NodeEventService handles business logic for node events
type NodeEventService struct {
	db     *db.Queries
	logger *logger.Logger
}

// NewNodeEventService creates a new NodeEventService instance
func NewNodeEventService(db *db.Queries, logger *logger.Logger) *NodeEventService {
	return &NodeEventService{
		db:     db,
		logger: logger,
	}
}

// CreateEvent creates a new node event in the database
func (s *NodeEventService) CreateEvent(ctx context.Context, nodeID int64, eventType NodeEventType, data interface{}) error {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	_, err = s.db.CreateNodeEvent(ctx, &db.CreateNodeEventParams{
		NodeID:      nodeID,
		EventType:   string(eventType),
		Data:        sql.NullString{String: string(dataJSON), Valid: true},
		Description: "created",
		Status:      "created",
	})
	if err != nil {
		return fmt.Errorf("failed to create node event: %w", err)
	}

	return nil
}

// GetEvents retrieves a paginated list of node events
func (s *NodeEventService) GetEvents(ctx context.Context, nodeID int64, page, limit int) ([]NodeEvent, error) {
	offset := (page - 1) * limit
	dbEvents, err := s.db.ListNodeEvents(ctx, &db.ListNodeEventsParams{
		NodeID: nodeID,
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list node events: %w", err)
	}

	return s.mapDBEventsToNodeEvents(dbEvents), nil
}

// GetLatestEvent retrieves the latest event for a node
func (s *NodeEventService) GetLatestEvent(ctx context.Context, nodeID int64) (*NodeEvent, error) {
	dbEvent, err := s.db.GetLatestNodeEvent(ctx, nodeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest node event: %w", err)
	}

	events := s.mapDBEventsToNodeEvents([]*db.NodeEvent{dbEvent})
	if len(events) == 0 {
		return nil, nil
	}
	return &events[0], nil
}

// GetEventsByType retrieves events of a specific type for a node
func (s *NodeEventService) GetEventsByType(ctx context.Context, nodeID int64, eventType NodeEventType, page, limit int) ([]NodeEvent, error) {
	offset := (page - 1) * limit
	dbEvents, err := s.db.ListNodeEventsByType(ctx, &db.ListNodeEventsByTypeParams{
		NodeID:    nodeID,
		EventType: string(eventType),
		Limit:     int64(limit),
		Offset:    int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list node events by type: %w", err)
	}

	return s.mapDBEventsToNodeEvents(dbEvents), nil
}

// mapDBEventsToNodeEvents converts database events to service layer events
func (s *NodeEventService) mapDBEventsToNodeEvents(dbEvents []*db.NodeEvent) []NodeEvent {
	events := make([]NodeEvent, len(dbEvents))
	for i, dbEvent := range dbEvents {
		// var data interface{}
		// if err := json.Unmarshal([]byte(), &data); err != nil {
		// 	s.logger.Error("Failed to unmarshal event data", "error", err)
		// }

		events[i] = NodeEvent{
			ID:        dbEvent.ID,
			NodeID:    dbEvent.NodeID,
			Type:      NodeEventType(dbEvent.EventType),
			Data:      dbEvent.Data,
			CreatedAt: dbEvent.CreatedAt,
		}
	}
	return events
}
