package provider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHProvider struct {
	caPrivateKey []byte
}

type SSHConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	CAPrivate  string `json:"ca_private_key"` // PEM encoded
	ClientHost string `json:"client_host"`
	ClientPort int    `json:"client_port"`
}

type SSHCredentialPayload struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	PrivateKey  string `json:"private_key"`
	Certificate string `json:"certificate"`
}

func NewSSHProvider() *SSHProvider {
	p := &SSHProvider{}
	go func() {
		time.Sleep(100 * time.Millisecond)
		var cfg SSHConfig
		_, _ = p.ensureCAKey(&cfg)
	}()
	return p
}

func (p *SSHProvider) Name() string {
	return "ssh"
}

func (p *SSHProvider) ensureCAKey(cfg *SSHConfig) ([]byte, error) {
	// If CA key is specified in the resource config, use it
	if cfg.CAPrivate != "" {
		return []byte(cfg.CAPrivate), nil
	}

	// Fallback to local files for local dev testing
	privPath := os.Getenv("SSH_CA_PRIVATE_KEY_PATH")
	if privPath == "" {
		privPath = "ssh_ca.key"
	}
	pubPath := os.Getenv("SSH_CA_PUBLIC_KEY_PATH")
	if pubPath == "" {
		pubPath = "ssh_ca.pub"
	}

	if _, err := os.Stat(privPath); err == nil {
		return os.ReadFile(privPath)
	}

	// Generate a 2048-bit RSA key for CA
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key for SSH CA: %w", err)
	}

	// Private key in PEM format
	privDER := x509.MarshalPKCS1PrivateKey(key)
	privBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privDER,
	}
	privPEM := pem.EncodeToMemory(privBlock)

	// Save private key
	if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
		return nil, fmt.Errorf("failed to save private key: %w", err)
	}

	// Save public key formatted for SSH CA (TrustedUserCAKeys)
	sshPubKey, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert public key: %w", err)
	}
	pubBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	if err := os.WriteFile(pubPath, pubBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to save public key: %w", err)
	}

	return privPEM, nil
}

func (p *SSHProvider) IssueSession(ctx context.Context, req IssueRequest, config []byte) (*IssueResponse, error) {
	var cfg SSHConfig
	if err := json.Unmarshal(config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid ssh configuration: %w", err)
	}

	if cfg.Port == 0 {
		cfg.Port = 22
	}

	caKeyPEM, err := p.ensureCAKey(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve SSH CA key: %w", err)
	}

	// 1. Generate client keypair (RSA)
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client SSH key: %w", err)
	}

	clientPrivDER := x509.MarshalPKCS1PrivateKey(clientKey)
	clientPrivPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: clientPrivDER,
	})

	clientPubKey, err := ssh.NewPublicKey(&clientKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get client public key: %w", err)
	}

	// 2. Parse CA signer
	caSigner, err := ssh.ParsePrivateKey(caKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA private key: %w", err)
	}

	// 3. Create SSH certificate
	validAfter := time.Now().Add(-2 * time.Minute) // buffer for clock skew
	validBefore := time.Now().Add(req.Duration)

	cert := &ssh.Certificate{
		Key:             clientPubKey,
		CertType:        ssh.UserCert,
		KeyId:           req.SessionID,
		ValidPrincipals: []string{cfg.Username},
		ValidAfter:      uint64(validAfter.Unix()),
		ValidBefore:     uint64(validBefore.Unix()),
		Permissions: ssh.Permissions{
			Extensions: map[string]string{
				"permit-pty":              "",
				"permit-port-forwarding":  "",
				"permit-agent-forwarding": "",
				"permit-user-rc":          "",
			},
		},
	}

	// Sign the certificate
	if err := cert.SignCert(rand.Reader, caSigner); err != nil {
		return nil, fmt.Errorf("failed to sign SSH certificate: %w", err)
	}

	certBytes := ssh.MarshalAuthorizedKey(cert)

	clientHost := cfg.ClientHost
	if clientHost == "" {
		clientHost = cfg.Host
	}
	clientPort := cfg.ClientPort
	if clientPort == 0 {
		clientPort = cfg.Port
	}

	// 4. Return credentials payload
	payload := SSHCredentialPayload{
		Host:        clientHost,
		Port:        clientPort,
		Username:    cfg.Username,
		PrivateKey:  string(clientPrivPEM),
		Certificate: string(certBytes),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}

	return &IssueResponse{
		ExpiresAt: validBefore,
		Type:      "ssh_cert",
		Payload:   payloadBytes,
	}, nil
}

func (p *SSHProvider) RevokeSession(ctx context.Context, metadata []byte, config []byte) error {
	// SSH Certificates use short validity terms enforced on the server.
	// No active revocation is needed.
	return nil
}
