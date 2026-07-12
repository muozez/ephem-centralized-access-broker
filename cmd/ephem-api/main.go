package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/muozez/ephem-centralized-access-broker/internal/api"
	"github.com/muozez/ephem-centralized-access-broker/internal/provider"
	"github.com/muozez/ephem-centralized-access-broker/internal/scheduler"
)

func main() {
	fmt.Println("Starting ephem API Server...")

	// Start background polling scheduler (runs every 10 seconds)
	sched := scheduler.NewScheduler()
	sched.Start(10 * time.Second)
	defer sched.Stop()

	// Pre-initialize providers to warm cache and generate dev key assets (like SSH CA keys)
	_ = provider.GetRegistry()

	// Register Routes
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	http.HandleFunc("/v1/auth/login", api.HandleLogin)
	http.HandleFunc("/v1/auth/callback", api.HandleCallback)
	http.HandleFunc("/.well-known/jwks.json", api.HandleJWKS)
	http.HandleFunc("/v1/sessions/request", api.HandleRequestSession)
	http.HandleFunc("/v1/sessions/revoke", api.HandleRevokeSession)

	// Admin Console Endpoints
	http.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(api.AdminConsoleHTML))
	})
	http.HandleFunc("/v1/admin/sessions", api.HandleAdminSessions)
	http.HandleFunc("/v1/admin/resources", api.HandleAdminResources)
	http.HandleFunc("/v1/admin/policies", api.HandleAdminPolicies)
	http.HandleFunc("/v1/admin/audit_logs", api.HandleAdminAuditLogs)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Listening on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		os.Exit(1)
	}
}
