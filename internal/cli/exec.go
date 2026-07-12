package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	durationStr string
)

var execCmd = &cobra.Command{
	Use:   "exec [resource] -- [command]",
	Short: "Execute a command with temporary credentials",
	Long:  `Requests ephemeral credentials for a resource and executes the specified command with those credentials in its environment.`,
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		resourceName := args[0]
		commandArgs := args[1:]

		// 1. Load Token
		token, err := LoadToken()
		if err != nil || token == "" {
			fmt.Println("Error: Not logged in. Please run 'ephem login' first.")
			os.Exit(1)
		}

		// 2. Request Session Credentials from API
		fmt.Printf("Requesting temporary access to resource '%s'...\n", resourceName)
		type ReqBody struct {
			ResourceName string `json:"resource_name"`
			Duration     string `json:"duration"`
		}
		reqBodyBytes, _ := json.Marshal(ReqBody{
			ResourceName: resourceName,
			Duration:     durationStr,
		})

		req, err := http.NewRequest(http.MethodPost, ApiURL+"/v1/sessions/request", bytes.NewBuffer(reqBodyBytes))
		if err != nil {
			fmt.Printf("Error creating request: %v\n", err)
			os.Exit(1)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("API request failed: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("Access Denied: API returned status %d: %s\n", resp.StatusCode, string(body))
			os.Exit(1)
		}

		type JSONResponse struct {
			SessionID string          `json:"session_id"`
			ExpiresAt string          `json:"expires_at"`
			Type      string          `json:"type"`
			Payload   json.RawMessage `json:"payload"`
		}
		var sResp JSONResponse
		if err := json.NewDecoder(resp.Body).Decode(&sResp); err != nil {
			fmt.Printf("Error parsing response: %v\n", err)
			os.Exit(1)
		}

		// 3. Parse credentials payload and prepare environment variables
		var envVars []string
		if sResp.Type == "db_credentials" {
			type DBCredentials struct {
				Host     string `json:"host"`
				Port     int    `json:"port"`
				Database string `json:"database"`
				Username string `json:"username"`
				Password string `json:"password"`
			}
			var creds DBCredentials
			if err := json.Unmarshal(sResp.Payload, &creds); err == nil {
				envVars = append(envVars,
					fmt.Sprintf("PGHOST=%s", creds.Host),
					fmt.Sprintf("PGPORT=%d", creds.Port),
					fmt.Sprintf("PGUSER=%s", creds.Username),
					fmt.Sprintf("PGPASSWORD=%s", creds.Password),
					fmt.Sprintf("PGDATABASE=%s", creds.Database),
				)
			}
		}

		// 4. Setup child process execution
		child := exec.Command(commandArgs[0], commandArgs[1:]...)
		child.Stdin = os.Stdin
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr

		// Copy parent env and append temporary database credentials
		child.Env = os.Environ()
		child.Env = append(child.Env, envVars...)

		// 5. Setup signal forwarding & immediate revocation on exit
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Start child process
		if err := child.Start(); err != nil {
			fmt.Printf("Failed to start child process: %v\n", err)
			revokeSession(sResp.SessionID, token)
			os.Exit(1)
		}

		// Forward signals to child process
		go func() {
			for sig := range sigChan {
				_ = child.Process.Signal(sig)
			}
		}()

		// Wait for child process exit
		exitErr := child.Wait()

		// 6. Revoke session immediately
		fmt.Println("\nRevoking ephemeral database credentials...")
		revokeSession(sResp.SessionID, token)

		if exitErr != nil {
			if exitCode, ok := exitErr.(*exec.ExitError); ok {
				os.Exit(exitCode.ExitCode())
			}
			os.Exit(1)
		}
	},
}

func revokeSession(sessionID string, token string) {
	type RevokeReq struct {
		SessionID string `json:"session_id"`
	}
	reqBytes, _ := json.Marshal(RevokeReq{SessionID: sessionID})

	req, err := http.NewRequest(http.MethodPost, ApiURL+"/v1/sessions/revoke", bytes.NewBuffer(reqBytes))
	if err != nil {
		fmt.Printf("Warning: Failed to create revoke request: %v\n", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Warning: Revocation call failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Println("Ephemeral credentials revoked successfully.")
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Warning: Failed to revoke credentials (API code %d): %s\n", resp.StatusCode, string(body))
	}
}

func init() {
	execCmd.Flags().StringVarP(&durationStr, "duration", "d", "30m", "Requested duration for the session (e.g. 15m, 1h)")
	RootCmd.AddCommand(execCmd)
}
