package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/muozez/ephem-centralized-access-broker/internal/auth"
	"github.com/muozez/ephem-centralized-access-broker/internal/db"
	"github.com/muozez/ephem-centralized-access-broker/internal/policy"
	"github.com/muozez/ephem-centralized-access-broker/internal/provider"
)

// generateUUID creates a cryptographically secure RFC 4122 v4 compliant UUID
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// HandleRequestSession authenticates the JWT, checks policies, triggers the provider to issue a role,
// saves the session, and returns temporary access credentials.
func HandleRequestSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// 1. Authenticate JWT from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized: Missing Authorization header", http.StatusUnauthorized)
		return
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := auth.VerifyToken(tokenStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unauthorized: Invalid token: %v", err), http.StatusUnauthorized)
		return
	}

	// 2. Parse request body
	type RequestBody struct {
		ResourceName string `json:"resource_name"`
		Duration     string `json:"duration"`
	}
	var body RequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Bad Request: Invalid JSON body", http.StatusBadRequest)
		return
	}
	if body.ResourceName == "" {
		http.Error(w, "Bad Request: resource_name is required", http.StatusBadRequest)
		return
	}

	requestedDuration := 30 * time.Minute // default
	if body.Duration != "" {
		d, err := time.ParseDuration(body.Duration)
		if err != nil {
			http.Error(w, "Bad Request: Invalid duration format (e.g. 30m, 1h)", http.StatusBadRequest)
			return
		}
		requestedDuration = d
	}

	// 3. Connect to database
	conn, err := db.Connect()
	if err != nil {
		http.Error(w, fmt.Sprintf("Internal Error: Database connection failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// 4. Retrieve Resource and check if enabled
	type Resource struct {
		ID          string
		Provider    string
		Name        string
		Enabled     bool
		ConfigExtra []byte
	}
	var res Resource
	query := `
		SELECT r.id, r.provider, r.name, r.enabled, c.config_extra
		FROM resources r
		JOIN provider_configs c ON c.id = r.config_id
		WHERE r.name = $1
	`
	err = conn.QueryRowContext(r.Context(), query, body.ResourceName).Scan(&res.ID, &res.Provider, &res.Name, &res.Enabled, &res.ConfigExtra)
	if err == sql.ErrNoRows {
		http.Error(w, fmt.Sprintf("Not Found: Resource '%s' not found", body.ResourceName), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, fmt.Sprintf("Internal Error: Failed to fetch resource: %v", err), http.StatusInternalServerError)
		return
	}

	if !res.Enabled {
		http.Error(w, "Forbidden: Resource is disabled", http.StatusForbidden)
		return
	}

	// 5. Evaluate policy engine
	pe := policy.NewEngine(conn)
	allowed, err := pe.Evaluate(r.Context(), claims.Roles, res.Name, requestedDuration)
	if err != nil {
		http.Error(w, fmt.Sprintf("Forbidden: Policy evaluation error: %v", err), http.StatusForbidden)
		return
	}
	if !allowed {
		// Log access denied in audit logs
		_, _ = conn.ExecContext(r.Context(), `
			INSERT INTO audit_logs (user_id, action, resource_name, result, reason, created_at)
			VALUES ($1, 'request_session', $2, 'DENIED', 'Policy engine evaluation failed', NOW())
		`, claims.Subject, res.Name)

		http.Error(w, "Forbidden: Access denied by policy engine", http.StatusForbidden)
		return
	}

	// 6. Get Provider
	reg := provider.GetRegistry()
	p, err := reg.Get(res.Provider)
	if err != nil {
		http.Error(w, fmt.Sprintf("Internal Error: Unsupported provider '%s'", res.Provider), http.StatusInternalServerError)
		return
	}

	// 7. Issue Session via Provider
	sessionID := generateUUID()
	tempUsername := fmt.Sprintf("ephem_u_%s", strings.ReplaceAll(sessionID[:8], "-", ""))

	provReq := provider.IssueRequest{
		SessionID: sessionID,
		Username:  tempUsername,
		Duration:  requestedDuration,
	}

	resp, err := p.IssueSession(r.Context(), provReq, res.ConfigExtra)
	if err != nil {
		// Log failure in audit logs
		_, _ = conn.ExecContext(r.Context(), `
			INSERT INTO audit_logs (user_id, action, resource_name, result, reason, created_at)
			VALUES ($1, 'request_session', $2, 'ERROR', $3, NOW())
		`, claims.Subject, res.Name, err.Error())

		http.Error(w, fmt.Sprintf("Internal Error: Provider failed to issue session: %v", err), http.StatusInternalServerError)
		return
	}

	// 8. Save Session to DB
	metaBytes, _ := json.Marshal(map[string]string{"username": tempUsername})
	_, err = conn.ExecContext(r.Context(), `
		INSERT INTO sessions (id, user_id, resource_id, provider, issued_at, expires_at, status, metadata)
		VALUES ($1, $2, $3, $4, NOW(), $5, 'ACTIVE', $6)
	`, sessionID, claims.Subject, res.ID, res.Provider, resp.ExpiresAt, metaBytes)
	if err != nil {
		http.Error(w, fmt.Sprintf("Internal Error: Failed to save session state: %v", err), http.StatusInternalServerError)
		return
	}

	// 9. Log successful audit log
	_, _ = conn.ExecContext(r.Context(), `
		INSERT INTO audit_logs (user_id, session_id, action, resource_name, result, created_at)
		VALUES ($1, $2, 'request_session', $3, 'SUCCESS', NOW())
	`, claims.Subject, sessionID, res.Name)

	// 10. Return Response
	type JSONResponse struct {
		SessionID string               `json:"session_id"`
		ExpiresAt string               `json:"expires_at"`
		Type      provider.SessionType `json:"type"`
		Payload   json.RawMessage      `json:"payload"`
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(JSONResponse{
		SessionID: sessionID,
		ExpiresAt: resp.ExpiresAt.Format(time.RFC3339),
		Type:      resp.Type,
		Payload:   resp.Payload,
	})
}

// HandleRevokeSession authenticates the JWT, checks ownership, and drops the temporary database credentials.
func HandleRevokeSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// 1. Authenticate JWT
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized: Missing Authorization header", http.StatusUnauthorized)
		return
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := auth.VerifyToken(tokenStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unauthorized: Invalid token: %v", err), http.StatusUnauthorized)
		return
	}

	// 2. Extract Session ID
	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		type RequestBody struct {
			SessionID string `json:"session_id"`
		}
		var body RequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			sessionID = body.SessionID
		}
	}

	if sessionID == "" {
		http.Error(w, "Bad Request: session_id is required", http.StatusBadRequest)
		return
	}

	// 3. Connect to database
	conn, err := db.Connect()
	if err != nil {
		http.Error(w, fmt.Sprintf("Internal Error: Database connection failed: %v", err), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// 4. Retrieve Session details, resource, config
	type Session struct {
		ID          string
		UserID      string
		Provider    string
		Status      string
		Metadata    []byte
		ConfigExtra []byte
		ResName     string
	}
	var sess Session
	query := `
		SELECT s.id, s.user_id, s.provider, s.status, s.metadata, c.config_extra, r.name
		FROM sessions s
		JOIN resources r ON r.id = s.resource_id
		JOIN provider_configs c ON c.id = r.config_id
		WHERE s.id = $1
	`
	err = conn.QueryRowContext(r.Context(), query, sessionID).Scan(
		&sess.ID, &sess.UserID, &sess.Provider, &sess.Status, &sess.Metadata, &sess.ConfigExtra, &sess.ResName,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "Not Found: Session not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, fmt.Sprintf("Internal Error: Database query failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Authorization check: only admin or session owner can revoke
	isAdmin := false
	for _, role := range claims.Roles {
		if role == "admin" {
			isAdmin = true
			break
		}
	}
	if sess.UserID != claims.Subject && !isAdmin {
		http.Error(w, "Forbidden: Cannot revoke other users' sessions", http.StatusForbidden)
		return
	}

	// Check if already revoked
	if sess.Status == "REVOKED" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "already_revoked"}`))
		return
	}

	// 5. Call Provider to Revoke Session
	reg := provider.GetRegistry()
	p, err := reg.Get(sess.Provider)
	if err != nil {
		http.Error(w, fmt.Sprintf("Internal Error: Unsupported provider '%s'", sess.Provider), http.StatusInternalServerError)
		return
	}

	err = p.RevokeSession(r.Context(), sess.Metadata, sess.ConfigExtra)
	if err != nil {
		// Log failure
		_, _ = conn.ExecContext(r.Context(), `
			INSERT INTO audit_logs (user_id, session_id, action, resource_name, result, reason, created_at)
			VALUES ($1, $2, 'revoke_session', $3, 'ERROR', $4, NOW())
		`, claims.Subject, sess.ID, sess.ResName, err.Error())

		http.Error(w, fmt.Sprintf("Internal Error: Provider failed to revoke: %v", err), http.StatusInternalServerError)
		return
	}

	// 6. Update Session Status
	_, err = conn.ExecContext(r.Context(), `
		UPDATE sessions SET status = 'REVOKED', revoked_at = NOW() WHERE id = $1
	`, sess.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Internal Error: Failed to update session status: %v", err), http.StatusInternalServerError)
		return
	}

	// 7. Log successful revoke
	_, _ = conn.ExecContext(r.Context(), `
		INSERT INTO audit_logs (user_id, session_id, action, resource_name, result, created_at)
		VALUES ($1, $2, 'revoke_session', $3, 'SUCCESS', NOW())
	`, claims.Subject, sess.ID, sess.ResName)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status": "revoked"}`))
}
