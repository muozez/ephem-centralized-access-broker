package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/muozez/ephem-centralized-access-broker/internal/api"
)

func main() {
	fmt.Println("Starting ephem API Server...")

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
