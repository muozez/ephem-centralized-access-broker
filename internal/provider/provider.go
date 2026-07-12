package provider

import (
	"context"
	"time"
)

type SessionType string

const (
	SessionTypeDBCredentials SessionType = "db_credentials"
)

type IssueResponse struct {
	ExpiresAt time.Time   `json:"expires_at"`
	Type      SessionType `json:"type"`
	Payload   []byte      `json:"payload"`
}

type IssueRequest struct {
	SessionID string
	Username  string
	Duration  time.Duration
}

type Provider interface {
	Name() string
	IssueSession(ctx context.Context, req IssueRequest, config []byte) (*IssueResponse, error)
	RevokeSession(ctx context.Context, metadata []byte, config []byte) error
}
