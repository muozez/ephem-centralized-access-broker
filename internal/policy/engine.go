package policy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

type Policy struct {
	ID           string
	RoleID       string
	ResourceGlob string
	Effect       string // ALLOW | DENY
	Conditions   string // JSON: {"max_duration": "1h"}
}

type Conditions struct {
	MaxDuration string `json:"max_duration"`
}

type Engine struct {
	db *sql.DB
}

func NewEngine(db *sql.DB) *Engine {
	return &Engine{db: db}
}

// Evaluate checks if the user with the given roles is allowed to access the resource
// for the requested duration. Returns allowed (bool) and error if any.
func (e *Engine) Evaluate(ctx context.Context, userRoles []string, resourceName string, requestedDuration time.Duration) (bool, error) {
	if len(userRoles) == 0 {
		return false, nil
	}

	// Fetch all policies matching the user's roles
	// If a user has multiple roles, we combine their policies.
	query := `
		SELECT p.id, p.role_id, p.resource_glob, p.effect, p.conditions
		FROM policies p
		JOIN roles r ON r.id = p.role_id
		WHERE r.name = ANY($1)
	`
	rows, err := e.db.QueryContext(ctx, query, userRoles)
	if err != nil {
		return false, fmt.Errorf("failed to query policies: %w", err)
	}
	defer rows.Close()

	var policies []Policy
	for rows.Next() {
		var p Policy
		var conds sql.NullString
		if err := rows.Scan(&p.ID, &p.RoleID, &p.ResourceGlob, &p.Effect, &conds); err != nil {
			return false, fmt.Errorf("failed to scan policy: %w", err)
		}
		if conds.Valid {
			p.Conditions = conds.String
		}
		policies = append(policies, p)
	}

	// Match logic:
	// 1. Any matching DENY rule denies access immediately (DENY takes precedence).
	// 2. An ALLOW rule allows access, provided the conditions (e.g. max duration) are met.
	hasAllow := false

	for _, p := range policies {
		matched, err := filepath.Match(p.ResourceGlob, resourceName)
		if err != nil {
			// Malformed glob in policy, skip or log warning
			continue
		}

		if matched {
			if p.Effect == "DENY" {
				return false, nil // DENY rule matched, immediate rejection
			}
			if p.Effect == "ALLOW" {
				// Verify conditions
				if p.Conditions != "" {
					var conds Conditions
					if err := json.Unmarshal([]byte(p.Conditions), &conds); err == nil && conds.MaxDuration != "" {
						maxDur, err := time.ParseDuration(conds.MaxDuration)
						if err == nil && requestedDuration > maxDur {
							// Requested duration exceeds policy limit
							return false, fmt.Errorf("requested duration %v exceeds maximum allowed duration %v", requestedDuration, maxDur)
						}
					}
				}
				hasAllow = true
			}
		}
	}

	return hasAllow, nil
}
