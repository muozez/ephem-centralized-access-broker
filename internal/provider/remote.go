package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/muozez/ephem-centralized-access-broker/internal/agent/proto"
	"github.com/muozez/ephem-centralized-access-broker/internal/mtls"
)

type RemoteProvider struct {
	certDir string
}

type RemoteConfig struct {
	AgentAddress string `json:"agent_address"`
	Provider     string `json:"provider"`
}

func NewRemoteProvider(certDir string) *RemoteProvider {
	return &RemoteProvider{certDir: certDir}
}

func (p *RemoteProvider) Name() string {
	return "remote"
}

func (p *RemoteProvider) IssueSession(ctx context.Context, req IssueRequest, config []byte) (*IssueResponse, error) {
	var cfg RemoteConfig
	if err := json.Unmarshal(config, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse Remote config: %w", err)
	}

	if cfg.AgentAddress == "" || cfg.Provider == "" {
		return nil, fmt.Errorf("agent_address and provider are required in remote provider configuration")
	}

	// Setup mTLS client connection
	tlsConfig, err := mtls.GetClientTLSConfig(p.certDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load client mTLS config: %w", err)
	}

	creds := credentials.NewTLS(tlsConfig)
	conn, err := grpc.DialContext(ctx, cfg.AgentAddress, grpc.WithTransportCredentials(creds), grpc.WithBlock())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to remote agent at %s: %w", cfg.AgentAddress, err)
	}
	defer conn.Close()

	client := pb.NewAgentServiceClient(conn)

	// Call agent gRPC
	gReq := &pb.IssueRequest{
		SessionId:       req.SessionID,
		Username:        req.Username,
		DurationSeconds: int64(req.Duration.Seconds()),
		Provider:        cfg.Provider,
	}

	gResp, err := client.IssueSession(ctx, gReq)
	if err != nil {
		return nil, fmt.Errorf("remote agent error: %w", err)
	}

	// We wrap the agent's return response metadata so we know how to revoke it
	meta := map[string]interface{}{
		"agent_address": cfg.AgentAddress,
		"provider":      cfg.Provider,
		"metadata":      req.Username, // Default metadata format
	}
	metaBytes, _ := json.Marshal(meta)

	return &IssueResponse{
		ExpiresAt: time.Unix(gResp.ExpiresAtUnix, 0),
		Type:      SessionType(gResp.SessionType),
		Payload:   gResp.CredentialPayload,
		Metadata:  metaBytes,
	}, nil
}

func (p *RemoteProvider) RevokeSession(ctx context.Context, metadata []byte, config []byte) error {
	var cfg RemoteConfig
	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("failed to parse Remote config: %w", err)
	}

	// We parse the wrap metadata
	var meta map[string]interface{}
	if err := json.Unmarshal(metadata, &meta); err != nil {
		return fmt.Errorf("failed to parse remote revoke metadata: %w", err)
	}

	agentAddr, ok1 := meta["agent_address"].(string)
	targetProvider, ok2 := meta["provider"].(string)
	if !ok1 || !ok2 {
		return fmt.Errorf("invalid remote revoke metadata format")
	}

	// Connect to agent
	tlsConfig, err := mtls.GetClientTLSConfig(p.certDir)
	if err != nil {
		return fmt.Errorf("failed to load client mTLS config: %w", err)
	}

	creds := credentials.NewTLS(tlsConfig)
	conn, err := grpc.DialContext(ctx, agentAddr, grpc.WithTransportCredentials(creds), grpc.WithBlock())
	if err != nil {
		return fmt.Errorf("failed to connect to remote agent at %s for revocation: %w", agentAddr, err)
	}
	defer conn.Close()

	client := pb.NewAgentServiceClient(conn)

	// Since Postgres/Redis providers use username inside metadata, we pass the username
	usernameMeta := map[string]string{"username": fmt.Sprintf("%v", meta["metadata"])}
	metaBytes, _ := json.Marshal(usernameMeta)

	gReq := &pb.RevokeRequest{
		Provider: targetProvider,
		Metadata: metaBytes,
	}

	gResp, err := client.RevokeSession(ctx, gReq)
	if err != nil {
		return fmt.Errorf("remote agent revocation failure: %w", err)
	}

	if !gResp.Success {
		return fmt.Errorf("remote agent failed to revoke session: %s", gResp.ErrorMessage)
	}

	return nil
}
