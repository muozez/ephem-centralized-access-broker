package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/muozez/ephem-centralized-access-broker/internal/db"
	"github.com/muozez/ephem-centralized-access-broker/internal/provider"
)

type Scheduler struct {
	ticker *time.Ticker
	done   chan bool
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		done: make(chan bool),
	}
}

func (s *Scheduler) Start(interval time.Duration) {
	s.ticker = time.NewTicker(interval)
	log.Printf("[Scheduler] Polling scheduler started (interval: %v)\n", interval)

	go func() {
		for {
			select {
			case <-s.done:
				return
			case <-s.ticker.C:
				s.pollAndRevokeExpired()
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.done <- true
	log.Println("[Scheduler] Polling scheduler stopped")
}

func (s *Scheduler) pollAndRevokeExpired() {
	conn, err := db.Connect()
	if err != nil {
		log.Printf("[Scheduler] Database connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Query expired active sessions
	query := `
		SELECT s.id, s.provider, s.metadata, c.config_extra, r.name, s.user_id
		FROM sessions s
		JOIN resources r ON r.id = s.resource_id
		JOIN provider_configs c ON c.id = r.config_id
		WHERE s.status = 'ACTIVE' AND s.expires_at <= NOW()
	`
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		log.Printf("[Scheduler] Failed to query expired sessions: %v\n", err)
		return
	}
	defer rows.Close()

	type ExpiredSession struct {
		ID           string
		ProviderName string
		Metadata     []byte
		ConfigExtra  []byte
		ResourceName string
		UserID       string
	}

	var expired []ExpiredSession
	for rows.Next() {
		var es ExpiredSession
		err := rows.Scan(&es.ID, &es.ProviderName, &es.Metadata, &es.ConfigExtra, &es.ResourceName, &es.UserID)
		if err != nil {
			log.Printf("[Scheduler] Failed to scan expired session row: %v\n", err)
			continue
		}
		expired = append(expired, es)
	}

	if len(expired) == 0 {
		return
	}

	log.Printf("[Scheduler] Found %d expired sessions to revoke\n", len(expired))

	reg := provider.GetRegistry()

	for _, es := range expired {
		log.Printf("[Scheduler] Revoking expired session %s (resource: %s)\n", es.ID, es.ResourceName)

		p, err := reg.Get(es.ProviderName)
		if err != nil {
			log.Printf("[Scheduler] Unsupported provider '%s' for session %s: %v\n", es.ProviderName, es.ID, err)
			s.updateSessionStatus(conn, es.ID, "FAILED")
			continue
		}

		err = p.RevokeSession(ctx, es.Metadata, es.ConfigExtra)
		if err != nil {
			log.Printf("[Scheduler] Provider failed to revoke session %s: %v\n", es.ID, err)
			// Log error in audit log
			_, _ = conn.ExecContext(ctx, `
				INSERT INTO audit_logs (user_id, session_id, action, resource_name, result, reason, created_at)
				VALUES ($1, $2, 'revoke_session', $3, 'ERROR', $4, NOW())
			`, es.UserID, es.ID, es.ResourceName, fmt.Sprintf("Auto-revoke failed: %v", err))

			s.updateSessionStatus(conn, es.ID, "FAILED")
			continue
		}

		// Update database status
		err = s.updateSessionStatus(conn, es.ID, "EXPIRED")
		if err != nil {
			log.Printf("[Scheduler] Failed to update status of session %s: %v\n", es.ID, err)
			continue
		}

		// Log successful audit log
		_, _ = conn.ExecContext(ctx, `
			INSERT INTO audit_logs (user_id, session_id, action, resource_name, result, reason, created_at)
			VALUES ($1, $2, 'revoke_session', $3, 'SUCCESS', 'Auto-revoked by scheduler TTL expiry', NOW())
		`, es.UserID, es.ID, es.ResourceName)

		log.Printf("[Scheduler] Successfully auto-revoked session %s\n", es.ID)
	}
}

func (s *Scheduler) updateSessionStatus(conn *sql.DB, sessionID string, status string) error {
	var query string
	if status == "EXPIRED" {
		query = `UPDATE sessions SET status = 'EXPIRED', revoked_at = NOW() WHERE id = $1`
	} else {
		query = `UPDATE sessions SET status = $2 WHERE id = $1`
	}
	_, err := conn.Exec(query, sessionID, status)
	return err
}
