package mtls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// GenerateKeysAndCerts creates CA, server, and client certs/keys in the given directory.
func GenerateKeysAndCerts(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	caKeyPath := filepath.Join(dir, "ca.key")
	caCertPath := filepath.Join(dir, "ca.crt")
	serverKeyPath := filepath.Join(dir, "server.key")
	serverCertPath := filepath.Join(dir, "server.crt")
	clientKeyPath := filepath.Join(dir, "client.key")
	clientCertPath := filepath.Join(dir, "client.crt")

	// Check if already exists
	if fileExists(caCertPath) && fileExists(serverCertPath) && fileExists(clientCertPath) {
		return nil
	}

	// 1. Generate CA
	caPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate CA private key: %w", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"ephem Distributed Access CA"},
			CommonName:   "ephem-root-ca",
		},
		NotBefore:             time.Now().Add(-10 * time.Minute),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	if err := writePEM(caKeyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(caPrivateKey), 0600); err != nil {
		return err
	}
	if err := writePEM(caCertPath, "CERTIFICATE", caBytes, 0644); err != nil {
		return err
	}

	// 2. Generate Server Certificate
	serverPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate server private key: %w", err)
	}

	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"ephem Distributed Access Agent"},
			CommonName:   "ephem-agent",
		},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0")},
		DNSNames:    []string{"localhost", "ephem-agent"},
		NotBefore:   time.Now().Add(-10 * time.Minute),
		NotAfter:    time.Now().AddDate(5, 0, 0), // 5 years
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	serverBytes, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, &serverPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to create server certificate: %w", err)
	}

	if err := writePEM(serverKeyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(serverPrivateKey), 0600); err != nil {
		return err
	}
	if err := writePEM(serverCertPath, "CERTIFICATE", serverBytes, 0644); err != nil {
		return err
	}

	// 3. Generate Client Certificate
	clientPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate client private key: %w", err)
	}

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			Organization: []string{"ephem Centralized Access Broker"},
			CommonName:   "ephem-api",
		},
		NotBefore:   time.Now().Add(-10 * time.Minute),
		NotAfter:    time.Now().AddDate(5, 0, 0), // 5 years
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}

	clientBytes, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, &clientPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to create client certificate: %w", err)
	}

	if err := writePEM(clientKeyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(clientPrivateKey), 0600); err != nil {
		return err
	}
	if err := writePEM(clientCertPath, "CERTIFICATE", clientBytes, 0644); err != nil {
		return err
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writePEM(path string, pemType string, bytes []byte, perm os.FileMode) error {
	block := &pem.Block{
		Type:  pemType,
		Bytes: bytes,
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", path, err)
	}
	defer f.Close()
	if err := pem.Encode(f, block); err != nil {
		return fmt.Errorf("failed to encode PEM to %s: %w", path, err)
	}
	return nil
}

// GetServerTLSConfig loads mTLS configuration for gRPC server.
func GetServerTLSConfig(dir string) (*tls.Config, error) {
	caCertPath := filepath.Join(dir, "ca.crt")
	serverCertPath := filepath.Join(dir, "server.crt")
	serverKeyPath := filepath.Join(dir, "server.key")

	cert, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load server keypair: %w", err)
	}

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate to pool")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// GetClientTLSConfig loads mTLS configuration for gRPC client.
func GetClientTLSConfig(dir string) (*tls.Config, error) {
	caCertPath := filepath.Join(dir, "ca.crt")
	clientCertPath := filepath.Join(dir, "client.crt")
	clientKeyPath := filepath.Join(dir, "client.key")

	cert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client keypair: %w", err)
	}

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate to pool")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
		ServerName:   "ephem-agent",
		MinVersion:   tls.VersionTLS13,
	}, nil
}
