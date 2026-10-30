package memory

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotConfigured = errors.New("memory service not configured")
	ErrDisabled      = errors.New("memory service disabled")
	ErrUnavailable   = errors.New("memory service unavailable")
)

// Message represents a conversation message for memory storage
type Message struct {
	Role      string                 `json:"role"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at,omitempty"`
}

// Memory represents retrieved memory context
type Memory struct {
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Relevance float64                `json:"relevance,omitempty"`
	CreatedAt time.Time              `json:"created_at,omitempty"`
}

// SessionSummary represents conversation summary
type SessionSummary struct {
	Summary   string    `json:"summary"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Client defines the interface for memory service operations
type Client interface {
	// EnsureSession creates the remote session for a user when missing, so
	// memories stored under the session are associated with that user.
	EnsureSession(ctx context.Context, sessionID, userID string) error

	// AddMessage adds a message to a conversation session
	AddMessage(ctx context.Context, sessionID string, message Message) error

	// GetSessionSummary retrieves the summary of a conversation
	GetSessionSummary(ctx context.Context, sessionID string) (*SessionSummary, error)

	// SearchMemories searches for relevant memories for a user
	SearchMemories(ctx context.Context, userID, query string, limit int) ([]Memory, error)

	// IsAvailable checks if the memory service is reachable
	IsAvailable(ctx context.Context) error
}

// NullClient implements Client but does nothing (when memory is disabled)
type NullClient struct{}

func (n *NullClient) EnsureSession(ctx context.Context, sessionID, userID string) error {
	return nil // silently succeed
}

func (n *NullClient) AddMessage(ctx context.Context, sessionID string, message Message) error {
	return nil // silently succeed
}

func (n *NullClient) GetSessionSummary(ctx context.Context, sessionID string) (*SessionSummary, error) {
	return nil, nil // no summary available
}

func (n *NullClient) SearchMemories(ctx context.Context, userID, query string, limit int) ([]Memory, error) {
	return nil, nil // no memories available
}

func (n *NullClient) IsAvailable(ctx context.Context) error {
	return ErrDisabled
}

// NewNullClient creates a no-op client for when memory is disabled
func NewNullClient() Client {
	return &NullClient{}
}
