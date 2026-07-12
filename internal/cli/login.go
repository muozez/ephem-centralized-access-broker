package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to ephem using OIDC",
	Run: func(cmd *cobra.Command, args []string) {
		// 1. Start local listener
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			fmt.Printf("Error starting local server: %v\n", err)
			return
		}
		defer listener.Close()

		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			fmt.Printf("Error parsing port: %v\n", err)
			return
		}

		tokenChan := make(chan string, 1)
		errChan := make(chan error, 1)

		mux := http.NewServeMux()
		server := &http.Server{Handler: mux}

		mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("token")
			if token == "" {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Missing token parameter"))
				errChan <- fmt.Errorf("missing token parameter")
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			// Premium visual success page
			w.Write([]byte(`
				<!DOCTYPE html>
				<html>
				<head>
					<title>ephem - Login Successful</title>
					<style>
						body {
							font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
							background-color: #0b0f19;
							color: #f3f4f6;
							display: flex;
							justify-content: center;
							align-items: center;
							height: 100vh;
							margin: 0;
						}
						.card {
							background: rgba(255, 255, 255, 0.05);
							backdrop-filter: blur(10px);
							border: 1px solid rgba(255, 255, 255, 0.1);
							padding: 40px;
							border-radius: 12px;
							text-align: center;
							box-shadow: 0 4px 30px rgba(0, 0, 0, 0.5);
							max-width: 400px;
						}
						h1 {
							color: #10b981;
							margin-top: 0;
						}
						p {
							color: #9ca3af;
							line-height: 1.5;
						}
					</style>
				</head>
				<body>
					<div class="card">
						<h1>Login Successful!</h1>
						<p>Authentication complete. You can close this tab and return to the terminal.</p>
					</div>
				</body>
				</html>
			`))

			tokenChan <- token
		})

		go func() {
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				errChan <- err
			}
		}()

		loginURL := fmt.Sprintf("%s/v1/auth/login?cli_port=%s", ApiURL, port)
		fmt.Printf("Opening browser to OIDC provider:\n%s\n\n", loginURL)
		
		_ = openBrowser(loginURL)

		// Wait for token or timeout
		select {
		case token := <-tokenChan:
			err := SaveToken(token)
			if err != nil {
				fmt.Printf("Error saving token: %v\n", err)
				return
			}
			fmt.Println("Successfully logged in.")
		case err := <-errChan:
			fmt.Printf("Authentication failed: %v\n", err)
		case <-time.After(3 * time.Minute):
			fmt.Println("Authentication timed out after 3 minutes.")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	},
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // "linux"
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func init() {
	RootCmd.AddCommand(loginCmd)
}
