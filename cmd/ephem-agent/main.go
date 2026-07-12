package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	pb "github.com/muozez/ephem-centralized-access-broker/internal/agent/proto"
	"github.com/muozez/ephem-centralized-access-broker/internal/mtls"
	"github.com/muozez/ephem-centralized-access-broker/internal/provider"
)

type AgentConfig struct {
	Resources map[string]ResourceConfig `json:"resources"`
}

type ResourceConfig struct {
	Provider string          `json:"provider"`
	Config   json.RawMessage `json:"config"`
}

type agentServer struct {
	pb.UnimplementedAgentServiceServer
	config AgentConfig
}

func main() {
	log.Println("Starting ephem Agent...")

	// 1. Ensure mTLS certificates are generated
	certDir := os.Getenv("MTLS_CERT_DIR")
	if certDir == "" {
		certDir = "certs"
	}
	if err := mtls.GenerateKeysAndCerts(certDir); err != nil {
		log.Fatalf("Failed to generate/check mTLS certs: %v", err)
	}

	// 2. Load Local Config (keeping secrets local)
	configPath := os.Getenv("AGENT_CONFIG_PATH")
	if configPath == "" {
		configPath = "agent_config.json"
	}

	// If config file doesn't exist, write a default seed one
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Printf("Agent config not found at %s. Creating a default config...", configPath)
		defaultConfig := AgentConfig{
			Resources: map[string]ResourceConfig{
				"postgres-staging": {
					Provider: "postgres",
					Config:   json.RawMessage(`{"host": "postgres", "port": 5432, "database": "ephem", "user": "ephem", "password": "ephem_password", "sslmode": "disable", "client_host": "localhost", "client_port": 5432}`),
				},
				"redis-staging": {
					Provider: "redis",
					Config:   json.RawMessage(`{"host": "redis", "port": 6379, "password": "redis_password", "db": 0, "client_host": "localhost", "client_port": 6379}`),
				},
			},
		}
		data, _ := json.MarshalIndent(defaultConfig, "", "  ")
		_ = ioutil.WriteFile(configPath, data, 0600)
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatalf("Failed to read agent config: %v", err)
	}

	var agentConf AgentConfig
	if err := json.Unmarshal(data, &agentConf); err != nil {
		log.Fatalf("Failed to parse agent config: %v", err)
	}

	// 3. Setup mTLS gRPC Server
	tlsConfig, err := mtls.GetServerTLSConfig(certDir)
	if err != nil {
		log.Fatalf("Failed to get mTLS server config: %v", err)
	}

	port := os.Getenv("AGENT_PORT")
	if port == "" {
		port = "50051"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	creds := credentials.NewTLS(tlsConfig)
	s := grpc.NewServer(grpc.Creds(creds))

	serverInstance := &agentServer{config: agentConf}
	pb.RegisterAgentServiceServer(s, serverInstance)

	log.Printf("ephem Agent listening on port %s (mTLS enforced)\n", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC Server failed: %v", err)
	}
}

func (s *agentServer) IssueSession(ctx context.Context, req *pb.IssueRequest) (*pb.IssueResponse, error) {
	log.Printf("[Agent] IssueSession request for provider %s, session %s\n", req.Provider, req.SessionId)

	// In the agent, we lookup config by provider or resource name. Let's find resource config for this provider.
	var resConfig []byte
	found := false
	for _, res := range s.config.Resources {
		if res.Provider == req.Provider {
			resConfig = res.Config
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("provider '%s' credentials not configured on this agent", req.Provider)
	}

	reg := provider.GetRegistry()
	p, err := reg.Get(req.Provider)
	if err != nil {
		return nil, fmt.Errorf("local provider error: %w", err)
	}

	issueReq := provider.IssueRequest{
		SessionID: req.SessionId,
		Username:  req.Username,
		Duration:  time.Duration(req.DurationSeconds) * time.Second,
	}

	resp, err := p.IssueSession(ctx, issueReq, resConfig)
	if err != nil {
		return nil, fmt.Errorf("local provider failed to issue session: %w", err)
	}

	return &pb.IssueResponse{
		ExpiresAtUnix:     resp.ExpiresAt.Unix(),
		SessionType:       string(resp.Type),
		CredentialPayload: resp.Payload,
	}, nil
}

func (s *agentServer) RevokeSession(ctx context.Context, req *pb.RevokeRequest) (*pb.RevokeResponse, error) {
	log.Printf("[Agent] RevokeSession request for provider %s\n", req.Provider)

	var resConfig []byte
	found := false
	for _, res := range s.config.Resources {
		if res.Provider == req.Provider {
			resConfig = res.Config
			found = true
			break
		}
	}

	if !found {
		return &pb.RevokeResponse{Success: false, ErrorMessage: fmt.Sprintf("provider '%s' credentials not configured on this agent", req.Provider)}, nil
	}

	reg := provider.GetRegistry()
	p, err := reg.Get(req.Provider)
	if err != nil {
		return &pb.RevokeResponse{Success: false, ErrorMessage: fmt.Sprintf("local provider error: %v", err)}, nil
	}

	// We pass raw metadata from the request to the local provider revoke function
	err = p.RevokeSession(ctx, req.Metadata, resConfig)
	if err != nil {
		return &pb.RevokeResponse{Success: false, ErrorMessage: err.Error()}, nil
	}

	return &pb.RevokeResponse{Success: true}, nil
}
