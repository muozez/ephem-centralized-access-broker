package cli

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var (
	sshUser     string
	sshPort     int
	sshKey      string
	sshPassword string
	agentArch   string
	certsDir    string
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage ephem agents",
}

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap [host]",
	Short: "Bootstrap and install ephem-agent on a remote host over SSH",
	Long:  `Compiles ephem-agent locally, generates/transfers necessary mTLS certs and configuration, and sets it up as a systemd service on the remote machine.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		host := args[0]
		user := sshUser
		if strings.Contains(host, "@") {
			parts := strings.SplitN(host, "@", 2)
			user = parts[0]
			host = parts[1]
		}

		fmt.Printf("Building ephem-agent locally for GOOS=linux GOARCH=%s...\n", agentArch)
		binPath := filepath.Join("bin", fmt.Sprintf("ephem-agent-linux-%s", agentArch))
		_ = os.MkdirAll("bin", 0755)

		buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/ephem-agent")
		buildCmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+agentArch)
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("failed to build agent: %w", err)
		}
		defer os.Remove(binPath)

		caPath := filepath.Join(certsDir, "ca.crt")
		srvCertPath := filepath.Join(certsDir, "server.crt")
		srvKeyPath := filepath.Join(certsDir, "server.key")

		caData, err := os.ReadFile(caPath)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate at %s: %w", caPath, err)
		}
		srvCertData, err := os.ReadFile(srvCertPath)
		if err != nil {
			return fmt.Errorf("failed to read server certificate at %s: %w", srvCertPath, err)
		}
		srvKeyData, err := os.ReadFile(srvKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read server key at %s: %w", srvKeyPath, err)
		}

		authMethods, err := getSSHAuthMethods(sshKey, sshPassword)
		if err != nil {
			return err
		}
		if len(authMethods) == 0 {
			fmt.Print("Enter SSH Password: ")
			var pswd string
			_, _ = fmt.Scanln(&pswd)
			authMethods = append(authMethods, ssh.Password(pswd))
		}

		sshConfig := &ssh.ClientConfig{
			User:            user,
			Auth:            authMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         10 * time.Second,
		}

		addr := fmt.Sprintf("%s:%d", host, sshPort)
		fmt.Printf("Connecting to remote host %s...\n", addr)
		client, err := ssh.Dial("tcp", addr, sshConfig)
		if err != nil {
			return fmt.Errorf("ssh connection failed: %w", err)
		}
		defer client.Close()

		fmt.Println("Creating remote directory /etc/ephem/certs...")
		if err := runRemoteCmd(client, "mkdir -p /etc/ephem/certs"); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		fmt.Println("Uploading CA certificate...")
		if err := writeRemoteFile(client, "/etc/ephem/certs/ca.crt", caData, "0644"); err != nil {
			return fmt.Errorf("failed to upload CA cert: %w", err)
		}
		fmt.Println("Uploading server certificate...")
		if err := writeRemoteFile(client, "/etc/ephem/certs/server.crt", srvCertData, "0644"); err != nil {
			return fmt.Errorf("failed to upload server cert: %w", err)
		}
		fmt.Println("Uploading server key...")
		if err := writeRemoteFile(client, "/etc/ephem/certs/server.key", srvKeyData, "0600"); err != nil {
			return fmt.Errorf("failed to upload server key: %w", err)
		}

		fmt.Println("Uploading ephem-agent binary...")
		agentBinData, err := os.ReadFile(binPath)
		if err != nil {
			return err
		}
		if err := writeRemoteFile(client, "/etc/ephem/ephem-agent", agentBinData, "0755"); err != nil {
			return fmt.Errorf("failed to upload agent binary: %w", err)
		}

		fmt.Println("Creating remote default agent_config.json...")
		defaultConfig := fmt.Sprintf(`{
  "resources": {
    "redis": {
      "provider": "redis",
      "config": {
        "host": "localhost",
        "port": 6379,
        "password": "redis_password",
        "db": 0,
        "client_host": "%s",
        "client_port": 6379
      }
    }
  }
}`, host)
		if err := writeRemoteFile(client, "/etc/ephem/agent_config.json", []byte(defaultConfig), "0600"); err != nil {
			return fmt.Errorf("failed to upload agent_config.json: %w", err)
		}

		fmt.Println("Writing systemd service file...")
		serviceFile := `[Unit]
Description=ephem Distributed Access Agent
After=network.target

[Service]
Type=simple
WorkingDirectory=/etc/ephem
Environment=AGENT_PORT=50051
Environment=MTLS_CERT_DIR=/etc/ephem/certs
Environment=AGENT_CONFIG_PATH=/etc/ephem/agent_config.json
ExecStart=/etc/ephem/ephem-agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`
		if err := writeRemoteFile(client, "/etc/systemd/system/ephem-agent.service", []byte(serviceFile), "0644"); err != nil {
			return fmt.Errorf("failed to write systemd service file: %w", err)
		}

		fmt.Println("Starting and enabling ephem-agent service...")
		commands := []string{
			"systemctl daemon-reload",
			"systemctl enable ephem-agent",
			"systemctl restart ephem-agent",
		}
		for _, cmdStr := range commands {
			if err := runRemoteCmd(client, cmdStr); err != nil {
				return fmt.Errorf("failed to execute command '%s': %w", cmdStr, err)
			}
		}

		fmt.Printf("\nSUCCESS: ephem-agent successfully bootstrapped on %s!\n", host)
		fmt.Println("The agent is now running and listening on port 50051.")
		return nil
	},
}

func getSSHAuthMethods(keyPath string, password string) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if socket := os.Getenv("SSH_AUTH_SOCK"); socket != "" {
		if conn, err := net.Dial("unix", socket); err == nil {
			agentClient := agent.NewClient(conn)
			if signers, err := agentClient.Signers(); err == nil && len(signers) > 0 {
				methods = append(methods, ssh.PublicKeys(signers...))
			}
		}
	}

	var keys []string
	if keyPath != "" {
		keys = []string{keyPath}
	} else {
		home, _ := os.UserHomeDir()
		if home != "" {
			keys = []string{
				filepath.Join(home, ".ssh", "id_rsa"),
				filepath.Join(home, ".ssh", "id_ed25519"),
				filepath.Join(home, ".ssh", "id_ecdsa"),
			}
		}
	}

	for _, k := range keys {
		if data, err := os.ReadFile(k); err == nil {
			signer, err := ssh.ParsePrivateKey(data)
			if err == nil {
				methods = append(methods, ssh.PublicKeys(signer))
			}
		}
	}

	if password != "" {
		methods = append(methods, ssh.Password(password))
	}

	return methods, nil
}

func runRemoteCmd(client *ssh.Client, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	return session.Run(cmd)
}

func writeRemoteFile(client *ssh.Client, remotePath string, content []byte, perm string) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdin = bytes.NewReader(content)
	cmd := fmt.Sprintf("cat > %s && chmod %s %s", remotePath, perm, remotePath)
	return session.Run(cmd)
}

func init() {
	bootstrapCmd.Flags().StringVarP(&sshUser, "user", "U", "root", "SSH user for remote host")
	bootstrapCmd.Flags().IntVarP(&sshPort, "port", "p", 22, "SSH port of remote host")
	bootstrapCmd.Flags().StringVarP(&sshKey, "key", "i", "", "SSH private key path")
	bootstrapCmd.Flags().StringVarP(&sshPassword, "password", "P", "", "SSH password (fallback)")
	bootstrapCmd.Flags().StringVarP(&agentArch, "arch", "a", "amd64", "Target architecture (amd64, arm64)")
	bootstrapCmd.Flags().StringVarP(&certsDir, "certs-dir", "c", "certs", "Local path to directory containing mTLS certificates")

	agentCmd.AddCommand(bootstrapCmd)
	RootCmd.AddCommand(agentCmd)
}
