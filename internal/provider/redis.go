package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisProvider struct{}

type RedisConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Password   string `json:"password"`
	DB         int    `json:"db"`
	ClientHost string `json:"client_host"`
	ClientPort int    `json:"client_port"`
	Rules      string `json:"rules"` // e.g., "~* +@all"
}

type RedisCredentialPayload struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

func NewRedisProvider() *RedisProvider {
	return &RedisProvider{}
}

func (p *RedisProvider) Name() string {
	return "redis"
}

func (p *RedisProvider) IssueSession(ctx context.Context, req IssueRequest, config []byte) (*IssueResponse, error) {
	var cfg RedisConfig
	if err := json.Unmarshal(config, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse Redis config: %w", err)
	}

	// Generate temp username and password
	username := fmt.Sprintf("ephem_u_%s", req.SessionID[:8])
	passwordBytes := make([]byte, 16)
	if _, err := rand.Read(passwordBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random password: %w", err)
	}
	tempPassword := hex.EncodeToString(passwordBytes)

	// Set default rules if empty
	rules := cfg.Rules
	if rules == "" {
		rules = "~* +@all"
	}

	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	defer rdb.Close()

	// Ping connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	// Provision temporary ACL user
	// command format: ACL SETUSER username on >password rules
	cmd := []interface{}{"ACL", "SETUSER", username, "on", ">" + tempPassword}
	// Split and append rules
	for _, rule := range strings.Fields(rules) {
		cmd = append(cmd, rule)
	}

	// In newer redis-go, we run a custom command
	if err := rdb.Do(ctx, cmd...).Err(); err != nil {
		return nil, fmt.Errorf("failed to set Redis ACL: %w", err)
	}

	expiresAt := time.Now().Add(req.Duration)

	// Construct metadata for revocation
	metadata := RevokeMetadata{
		Username: username,
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		// Clean up
		_ = rdb.Do(ctx, "ACL", "DELUSER", username).Err()
		return nil, fmt.Errorf("failed to marshal revoke metadata: %w", err)
	}

	clientHost := cfg.ClientHost
	if clientHost == "" {
		clientHost = cfg.Host
	}
	clientPort := cfg.ClientPort
	if clientPort == 0 {
		clientPort = cfg.Port
	}

	credPayload := RedisCredentialPayload{
		Host:     clientHost,
		Port:     clientPort,
		Username: username,
		Password: tempPassword,
		DB:       cfg.DB,
	}

	payloadBytes, err := json.Marshal(credPayload)
	if err != nil {
		_ = rdb.Do(ctx, "ACL", "DELUSER", username).Err()
		return nil, fmt.Errorf("failed to marshal credential payload: %w", err)
	}

	return &IssueResponse{
		ExpiresAt: expiresAt,
		Type:      SessionTypeDBCredentials,
		Payload:   payloadBytes,
		Metadata:  metadataBytes,
	}, nil
}

func (p *RedisProvider) RevokeSession(ctx context.Context, metadata []byte, config []byte) error {
	var cfg RedisConfig
	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("failed to parse Redis config: %w", err)
	}

	var meta RevokeMetadata
	if err := json.Unmarshal(metadata, &meta); err != nil {
		return fmt.Errorf("failed to parse revoke metadata: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	defer rdb.Close()

	// Revoke user by deleting ACL record
	if err := rdb.Do(ctx, "ACL", "DELUSER", meta.Username).Err(); err != nil {
		return fmt.Errorf("failed to delete Redis ACL user: %w", err)
	}

	return nil
}
