package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run diagnostic checks on ephem system health",
	Long:  `Performs self-tests on CLI token, API connectivity, authentication endpoints, and local client dependency tools.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running ephem system diagnostic check...")
		fmt.Println("=========================================")

		allPassed := true

		// 1. Token Check
		token, err := LoadToken()
		if err != nil || token == "" {
			fmt.Println("[✗] Local authentication token: Not found or unreadable. Please run 'ephem login'.")
			allPassed = false
		} else {
			parts := strings.Split(token, ".")
			if len(parts) != 3 {
				fmt.Println("[✗] Local authentication token: Token format is invalid. Please run 'ephem login' to refresh.")
				allPassed = false
			} else {
				fmt.Println("[✓] Local authentication token: Valid format and readable.")
			}
		}

		// 2. API Server Connection Check
		client := http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(ApiURL + "/health")
		if err != nil {
			fmt.Printf("[✗] API server connection: Failed to reach API at %s. Error: %v\n", ApiURL, err)
			allPassed = false
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("[✗] API server connection: API returned non-OK status: %d\n", resp.StatusCode)
				allPassed = false
			} else {
				type HealthResp struct {
					Status string `json:"status"`
				}
				var hr HealthResp
				if err := json.NewDecoder(resp.Body).Decode(&hr); err == nil && hr.Status == "ok" {
					fmt.Printf("[✓] API server connection: Connected successfully to %s\n", ApiURL)
				} else {
					fmt.Printf("[✗] API server connection: API returned invalid health response payload.\n")
					allPassed = false
				}
			}
		}

		// 3. JWKS Endpoint Check
		respJWKS, err := client.Get(ApiURL + "/.well-known/jwks.json")
		if err != nil {
			fmt.Printf("[✗] JWKS endpoint: Failed to reach key provider endpoint. Error: %v\n", err)
			allPassed = false
		} else {
			defer respJWKS.Body.Close()
			if respJWKS.StatusCode != http.StatusOK {
				fmt.Printf("[✗] JWKS endpoint: Returned status code %d\n", respJWKS.StatusCode)
				allPassed = false
			} else {
				fmt.Println("[✓] JWKS endpoint: RSA Public Keys are publishable and healthy.")
			}
		}

		// 4. Local client tools checking (psql)
		psqlPath, err := exec.LookPath("psql")
		if err != nil {
			fmt.Println("[✗] Local command tools: 'psql' client is NOT installed or not in system PATH.")
			fmt.Println("    To use 'ephem postgres', please install the postgresql-client CLI tool.")
			allPassed = false
		} else {
			fmt.Printf("[✓] Local command tools: 'psql' client found at %s\n", psqlPath)
		}

		fmt.Println("=========================================")
		if allPassed {
			fmt.Println("Result: All diagnostic checks PASSED! System is fully operational.")
		} else {
			fmt.Println("Result: Diagnostic checks FAILED. Please resolve the [✗] items listed above.")
		}
	},
}

func init() {
	RootCmd.AddCommand(doctorCmd)
}
