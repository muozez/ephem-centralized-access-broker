package provider

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	_ "github.com/lib/pq"
)

type PostgresProvider struct{}

type PostgresConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Database   string `json:"database"`
	User       string `json:"user"`
	Password   string `json:"password"`
	SSLMode    string `json:"sslmode"`
	ClientHost string `json:"client_host"`
	ClientPort int    `json:"client_port"`
}

type PostgresCredentialPayload struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewPostgresProvider() *PostgresProvider {
	return &PostgresProvider{}
}

func (p *PostgresProvider) Name() string {
	return "postgres"
}

// Generate secure random alphanumeric password
func generateRandomPassword(length int) (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		result[i] = chars[num.Int64()]
	}
	return string(result), nil
}

func (p *PostgresProvider) IssueSession(ctx context.Context, req IssueRequest, config []byte) (*IssueResponse, error) {
	var cfg PostgresConfig
	if err := json.Unmarshal(config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid postgres configuration: %w", err)
	}

	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}

	// Connect to target database as admin
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode)
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer dbConn.Close()

	if err := dbConn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Generate secure password for temp user
	tempPassword, err := generateRandomPassword(18)
	if err != nil {
		return nil, fmt.Errorf("failed to generate password: %w", err)
	}

	expiresAt := time.Now().Add(req.Duration)
	// Format time for Postgres: YYYY-MM-DD HH:MM:SS TZ
	validUntilStr := expiresAt.Format("2006-01-02 15:04:05-07")

	// Execute role creation
	createRoleQuery := fmt.Sprintf("CREATE ROLE %s WITH LOGIN PASSWORD %s VALID UNTIL %s",
		req.Username, fmt.Sprintf("'%s'", tempPassword), fmt.Sprintf("'%s'", validUntilStr))
	if _, err := dbConn.ExecContext(ctx, createRoleQuery); err != nil {
		return nil, fmt.Errorf("failed to create temporary role: %w", err)
	}

	// Grant connect & basic readonly permissions
	grantConnectQuery := fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", cfg.Database, req.Username)
	if _, err := dbConn.ExecContext(ctx, grantConnectQuery); err != nil {
		// Clean up on failure
		_, _ = dbConn.ExecContext(ctx, fmt.Sprintf("DROP ROLE IF EXISTS %s", req.Username))
		return nil, fmt.Errorf("failed to grant CONNECT privilege: %w", err)
	}

	grantUsageQuery := fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s", req.Username)
	if _, err := dbConn.ExecContext(ctx, grantUsageQuery); err != nil {
		_, _ = dbConn.ExecContext(ctx, fmt.Sprintf("DROP ROLE IF EXISTS %s", req.Username))
		return nil, fmt.Errorf("failed to grant schema USAGE privilege: %w", err)
	}

	grantSelectQuery := fmt.Sprintf("GRANT SELECT ON ALL TABLES IN SCHEMA public TO %s", req.Username)
	if _, err := dbConn.ExecContext(ctx, grantSelectQuery); err != nil {
		_, _ = dbConn.ExecContext(ctx, fmt.Sprintf("DROP ROLE IF EXISTS %s", req.Username))
		return nil, fmt.Errorf("failed to grant SELECT privilege: %w", err)
	}

	clientHost := cfg.ClientHost
	if clientHost == "" {
		clientHost = cfg.Host
	}
	clientPort := cfg.ClientPort
	if clientPort == 0 {
		clientPort = cfg.Port
	}

	// Package response payload
	credPayload := PostgresCredentialPayload{
		Host:     clientHost,
		Port:     clientPort,
		Database: cfg.Database,
		Username: req.Username,
		Password: tempPassword,
	}

	payloadBytes, err := json.Marshal(credPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credential payload: %w", err)
	}

	return &IssueResponse{
		ExpiresAt: expiresAt,
		Type:      SessionTypeDBCredentials,
		Payload:   payloadBytes,
	}, nil
}

type RevokeMetadata struct {
	Username string `json:"username"`
}

func (p *PostgresProvider) RevokeSession(ctx context.Context, metadata []byte, config []byte) error {
	var meta RevokeMetadata
	if err := json.Unmarshal(metadata, &meta); err != nil {
		return fmt.Errorf("invalid session metadata for revoke: %w", err)
	}

	var cfg PostgresConfig
	if err := json.Unmarshal(config, &cfg); err != nil {
		return fmt.Errorf("invalid postgres configuration for revoke: %w", err)
	}

	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}

	// Connect as admin
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode)
	dbConn, err := sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection for revoke: %w", err)
	}
	defer dbConn.Close()

	// 1. Terminate any active sessions of the temporary user
	terminateQuery := `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE usename = $1`
	_, _ = dbConn.ExecContext(ctx, terminateQuery, meta.Username)

	// 2. Drop owned objects & privileges
	dropOwnedQuery := fmt.Sprintf("DROP OWNED BY %s", meta.Username)
	_, _ = dbConn.ExecContext(ctx, dropOwnedQuery)

	// 3. Drop the role itself
	dropRoleQuery := fmt.Sprintf("DROP ROLE IF EXISTS %s", meta.Username)
	if _, err := dbConn.ExecContext(ctx, dropRoleQuery); err != nil {
		return fmt.Errorf("failed to drop temporary role: %w", err)
	}

	return nil
}
