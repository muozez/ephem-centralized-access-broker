# ephem — Centralized Ephemeral Access Broker

[![Go Version](https://img.shields.io/github/go-mod/go-version/muozez/ephem-centralized-access-broker?filename=go.mod)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> **"Infrastructure access, issued on demand."**
> 
> *ephem* (from *ephemeral*) is a centralized access broker implementing the **Zero Standing Privileges (ZSP)** security philosophy. Instead of distributing permanent passwords or SSH keys, ephem authenticates users via OpenID Connect (OIDC) and provisions short-lived, automated credentials on target systems on-demand, revoking them immediately after use.

---

## Table of Contents

1. [Key Features](#key-features)
2. [Architectural Overview](#architectural-overview)
3. [Supported Providers](#supported-providers)
4. [Distributed Agent Architecture (BYOA)](#distributed-agent-architecture-byoa)
5. [Plug-and-Play Agent Bootstrapping](#plug-and-play-agent-bootstrapping)
6. [How to Write a Custom Provider](#how-to-write-a-custom-provider)
7. [Getting Started (Dockerized Sandbox)](#getting-started-dockerized-sandbox)
8. [Usage Guide](#usage-guide)
9. [Database Schema](#database-schema)
10. [License](#license)

---

## Key Features

- **Zero Standing Privileges (ZSP):** No persistent database users or permanent root access exist. All authorization is temporary, audited, and automatically cleaned up.
- **Asymmetric Cryptography (RS256 & JWKS):** Access tokens are signed using a 2048-bit RSA private key. The API exposes a standard JWKS endpoint (`/.well-known/jwks.json`) for downstream verification.
- **SSH Certificate Authority (CA):** Dynamic, on-the-fly client key generation and SSH certificate signing. Remote hosts trust the broker's CA public key for secure access without pre-configured keys.
- **Mutual TLS (mTLS) Nodes:** Secure mTLS connection between the central broker and distributed remote agents.
- **Docker Fallback Client:** The CLI automatically detects if CLI utilities (like `redis-cli` or `psql`) are missing on the host and runs them securely inside temporary local Docker containers.
- **Glob-Based Policy Engine:** Access rules support glob patterns (`postgres-staging-*`) combined with maximum session duration limits.

---

## Architectural Overview

The system consists of three main components:

1. **`ephem-cli` (Client):** The command-line utility used by developers to authenticate, request sessions, and invoke terminal clients (`psql`, `redis-cli`, `ssh`).
2. **`ephem-api` (Central Control Plane / Broker):** Authenticates users via OIDC, evaluates access policies against a database, schedules credential lifetimes, and coordinates with local or remote agents.
3. **`ephem-agent` (Remote Subnet Agent):** A lightweight gRPC daemon deployed inside isolated subnets/VPCs. It receives mTLS requests from the broker to provision/revoke database roles locally.

```
+---------------------------------------+
|             Developer CLI             |
+-------------------+-------------------+
                    |
          1. Request access (OIDC JWT)
                    |
                    v
+-------------------+-------------------+
|             ephem-api                 | (Central Control Plane)
+---------+-------------------+---------+
          |                   |
    (Local Subnet)      (Remote Subnet / VPC)
          |                   |
 2a. Direct Access            | 2b. gRPC Call over mTLS
          |                   |
          v                   v
   +--------------+    +--------------+
   | PostgreSQL / |    | ephem-agent  | (Subnet Agent)
   | SSH Target   |    +------+-------+
   +--------------+           |
                              | 3. Local Provisioning
                              v
                       +--------------+
                       | Redis / DB / |
                       | SSH Targets  |
                       +--------------+
```

---

## Supported Providers

### 1. PostgreSQL Provider
Provisions temporary database roles on-demand.
- **Issuance:** Generates a unique role `ephem_u_<session_id>` with `LOGIN` privileges and standard grants.
- **Revocation:** Forcefully terminates active client backends linked to the temporary role, revokes grants, and drops the role.

### 2. Redis Provider
Provisions short-lived Redis ACL users.
- **Issuance:** Creates a temporary ACL user `ephem_u_<session_id>` with rules mapped to specific keys/commands (`~* +@all`).
- **Revocation:** Executes `ACL DELUSER` command to immediately revoke credentials and sever active client sessions.

### 3. SSH Provider
Enables secure passwordless SSH access using signed certificates.
- **Issuance:** Generates a temporary RSA keypair and signs it using the broker's SSH CA private key.
- **Revocation:** Certs contain short-lived validities and naturally expire. The CLI deletes key assets from the user's disk on exit.

---

## Distributed Agent Architecture (BYOA)

For target servers in isolated networks or VPCs without public ingress, `ephem` provides a **Bring Your Own Agent (BYOA)** architecture. The central `ephem-api` server routes calls over an mTLS-secured gRPC connection (`port 50051`) to a local `ephem-agent` running within the private network. The agent performs local queries against target servers and returns the ephemeral credential payload.

---

## Plug-and-Play Agent Bootstrapping

Setting up a remote agent on a new target server (VDS/VPS) manually can be complex due to the requirements for mTLS certificate generation, directory setups, configuration, and service daemons.

We solved this with a single, automated bootstrap tool built directly into the CLI:

```bash
./bin/ephem agent bootstrap <target-host> [flags]
```

### What it does:
1. **Compiles** the `ephem-agent` binary locally for the remote architecture (supports cross-compiling for `amd64` and `arm64`).
2. **Connects** to the target remote machine over SSH (supports SSH keys, SSH agents, and password fallback).
3. **Sets up** `/etc/ephem` directory structure and uploads the generated mTLS certificates (`ca.crt`, `server.crt`, `server.key`).
4. **Generates** a customized `/etc/ephem/agent_config.json` with the host's public IP address mapped as `client_host`.
5. **Registers** the agent as a persistent Linux `systemd` service and starts it.

### Example usage:
```bash
# Bootstrap remote VDS using SSH key auth
./bin/ephem agent bootstrap root@5.10.220.56 --key ~/.ssh/id_rsa --arch amd64

# Bootstrap using password auth
./bin/ephem agent bootstrap root@5.10.220.56 --password my_secret_root_pass
```

---

## How to Write a Custom Provider

Adding new resource types (e.g. MySQL, MongoDB, AWS IAM) to `ephem` is modular. You only need to implement a standard Go interface and register it.

### 1. Implement the `Provider` Interface

Every provider must implement the `Provider` interface defined in `internal/provider/provider.go`:

```go
package provider

import (
	"context"
	"time"
)

type Provider interface {
	// Name returns the unique identifier for the provider (e.g. "mysql", "aws")
	Name() string

	// IssueSession provisions a temporary credential on the target system
	IssueSession(ctx context.Context, req IssueRequest, config []byte) (*IssueResponse, error)

	// RevokeSession cleans up the temporary credential after expiration or manual logout
	RevokeSession(ctx context.Context, metadata []byte, config []byte) error
}
```

### 2. Example Structure for a Custom Provider

Create a new file `internal/provider/my_provider.go`:

```go
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type MyProvider struct{}

type MyConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	ClientHost string `json:"client_host"`
	ClientPort int    `json:"client_port"`
}

type MyCredentials struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewMyProvider() *MyProvider {
	return &MyProvider{}
}

func (p *MyProvider) Name() string {
	return "my-custom-provider"
}

func (p *MyProvider) IssueSession(ctx context.Context, req IssueRequest, config []byte) (*IssueResponse, error) {
	var cfg MyConfig
	if err := json.Unmarshal(config, &cfg); err != nil {
		return nil, err
	}

	// 1. Logic to provision temporary user on the target system
	tempUsername := fmt.Sprintf("ephem_%s", req.SessionID[:8])
	tempPassword := "generate_secure_random_string"

	// 2. Prepare credential payload for the developer CLI
	payload, _ := json.Marshal(MyCredentials{
		Host:     cfg.ClientHost,
		Port:     cfg.ClientPort,
		Username: tempUsername,
		Password: tempPassword,
	})

	// 3. Prepare metadata to remember how to revoke this user later
	metadata, _ := json.Marshal(map[string]string{
		"username": tempUsername,
	})

	return &IssueResponse{
		ExpiresAt: time.Now().Add(req.Duration),
		Type:      SessionTypeDBCredentials,
		Payload:   payload,
		Metadata:  metadata,
	}, nil
}

func (p *MyProvider) RevokeSession(ctx context.Context, metadata []byte, config []byte) error {
	var cfg MyConfig
	_ = json.Unmarshal(config, &cfg)

	var meta map[string]string
	_ = json.Unmarshal(metadata, &meta)

	// Logic to remove the user (meta["username"]) from the target system
	fmt.Printf("Dropping user %s\n", meta["username"])
	return nil
}
```

### 3. Register the Provider

Open `internal/provider/registry.go` and register your new provider inside the `GetRegistry` function:

```go
func GetRegistry() *Registry {
	registryOnce.Do(func() {
		globalRegistry = NewRegistry()
		globalRegistry.Register(NewPostgresProvider())
		globalRegistry.Register(NewSSHProvider())
		globalRegistry.Register(NewRedisProvider())
		
		// Register your new provider:
		globalRegistry.Register(NewMyProvider())

		certDir := os.Getenv("MTLS_CERT_DIR")
		if certDir == "" {
			certDir = "certs"
		}
		_ = mtls.GenerateKeysAndCerts(certDir)
		globalRegistry.Register(NewRemoteProvider(certDir))
	})
	return globalRegistry
}
```

---

## Getting Started (Dockerized Sandbox)

We provide a complete multi-container sandbox in `docker-compose.yml` to test everything locally.

### Start the Sandbox
```bash
# Build and run the containers
docker-compose up --build -d

# Compile the ephem CLI
make build
```

---

## Usage Guide

### 1. Login
```bash
./bin/ephem login
```
Open the login link in your browser and log in as `developer@company.com`.

### 2. Verify Session
```bash
./bin/ephem whoami
./bin/ephem doctor
```

### 3. Request Access to Redis (Interactive)
The CLI will automatically request credentials, configure them, start your local `redis-cli` (or run it via a fallback Docker container if not installed), and revoke the credentials as soon as you exit:
```bash
./bin/ephem redis staging
```

### 4. Run SSH Commands
```bash
./bin/ephem ssh staging
```

---

## Database Schema

- **`users`:** Holds user accounts.
- **`roles` / `user_roles`:** Enforces role-based permissions.
- **`provider_configs`:** Houses targeted resource connection configurations.
- **`resources`:** Holds the Resource Registry.
- **`policies`:** Stores access policies matching roles to resource name globs.
- **`sessions`:** Tracks requested credentials. **Passwords are never saved in the database.**
- **`audit_logs`:** Secure trail of all authorization events.

---

## License

This project is licensed under the **MIT License**. Feel free to use, modify, and distribute it. See the [LICENSE](LICENSE) file for details.