package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/muozez/ephem-centralized-access-broker/internal/auth"
	"github.com/muozez/ephem-centralized-access-broker/internal/db"
)

// checkAdminAuth verifies the JWT and ensures the user is an admin
func checkAdminAuth(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized: Missing Authorization header", http.StatusUnauthorized)
		return nil, false
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := auth.VerifyToken(tokenStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unauthorized: Invalid token: %v", err), http.StatusUnauthorized)
		return nil, false
	}

	isAdmin := false
	for _, role := range claims.Roles {
		if role == "admin" {
			isAdmin = true
			break
		}
	}

	if !isAdmin {
		http.Error(w, "Forbidden: Admin role required", http.StatusForbidden)
		return nil, false
	}

	return claims, true
}

// HandleAdminSessions lists all active and past sessions
func HandleAdminSessions(w http.ResponseWriter, r *http.Request) {
	if _, ok := checkAdminAuth(w, r); !ok {
		return
	}

	conn, err := db.Connect()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	rows, err := conn.QueryContext(r.Context(), `
		SELECT s.id, s.user_id, s.provider, r.name, s.issued_at, s.expires_at, s.status
		FROM sessions s
		JOIN resources r ON r.id = s.resource_id
		ORDER BY s.issued_at DESC
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type SessionItem struct {
		ID           string `json:"id"`
		UserID       string `json:"user_id"`
		ProviderName string `json:"provider"`
		ResourceName string `json:"resource_name"`
		IssuedAt     string `json:"issued_at"`
		ExpiresAt    string `json:"expires_at"`
		Status       string `json:"status"`
	}

	list := []SessionItem{}
	for rows.Next() {
		var item SessionItem
		var issued, expires string
		if err := rows.Scan(&item.ID, &item.UserID, &item.ProviderName, &item.ResourceName, &issued, &expires, &item.Status); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		item.IssuedAt = issued
		item.ExpiresAt = expires
		list = append(list, item)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

// HandleAdminResources lists all targets/resources
func HandleAdminResources(w http.ResponseWriter, r *http.Request) {
	if _, ok := checkAdminAuth(w, r); !ok {
		return
	}

	conn, err := db.Connect()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	rows, err := conn.QueryContext(r.Context(), `
		SELECT id, provider, name, display_name, environment, owner_team, enabled
		FROM resources
		ORDER BY name ASC
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type ResourceItem struct {
		ID          string `json:"id"`
		Provider    string `json:"provider"`
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		Environment string `json:"environment"`
		OwnerTeam   string `json:"owner_team"`
		Enabled     bool   `json:"enabled"`
	}

	list := []ResourceItem{}
	for rows.Next() {
		var item ResourceItem
		if err := rows.Scan(&item.ID, &item.Provider, &item.Name, &item.DisplayName, &item.Environment, &item.OwnerTeam, &item.Enabled); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		list = append(list, item)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

// HandleAdminPolicies lists all rules/policies
func HandleAdminPolicies(w http.ResponseWriter, r *http.Request) {
	if _, ok := checkAdminAuth(w, r); !ok {
		return
	}

	conn, err := db.Connect()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	rows, err := conn.QueryContext(r.Context(), `
		SELECT p.id, r.name, p.resource_glob, p.effect, p.conditions
		FROM policies p
		JOIN roles r ON r.id = p.role_id
		ORDER BY r.name ASC
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type PolicyItem struct {
		ID           string          `json:"id"`
		RoleName     string          `json:"role_name"`
		ResourceGlob string          `json:"resource_glob"`
		Effect       string          `json:"effect"`
		Conditions   json.RawMessage `json:"conditions"`
	}

	list := []PolicyItem{}
	for rows.Next() {
		var item PolicyItem
		if err := rows.Scan(&item.ID, &item.RoleName, &item.ResourceGlob, &item.Effect, &item.Conditions); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		list = append(list, item)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

// HandleAdminAuditLogs lists write-only audit history
func HandleAdminAuditLogs(w http.ResponseWriter, r *http.Request) {
	if _, ok := checkAdminAuth(w, r); !ok {
		return
	}

	conn, err := db.Connect()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	rows, err := conn.QueryContext(r.Context(), `
		SELECT id, user_id, session_id, action, resource_name, result, reason, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type AuditItem struct {
		ID           int    `json:"id"`
		UserID       string `json:"user_id"`
		SessionID    *string `json:"session_id"`
		Action       string `json:"action"`
		ResourceName string `json:"resource_name"`
		Result       string `json:"result"`
		Reason       *string `json:"reason"`
		CreatedAt    string `json:"created_at"`
	}

	list := []AuditItem{}
	for rows.Next() {
		var item AuditItem
		var created string
		if err := rows.Scan(&item.ID, &item.UserID, &item.SessionID, &item.Action, &item.ResourceName, &item.Result, &item.Reason, &created); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		item.CreatedAt = created
		list = append(list, item)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}
