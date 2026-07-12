# ephem — Centralized Ephemeral Access Broker

[![Go Version](https://img.shields.io/github/go-mod/go-version/muozez/ephem-centralized-access-broker?filename=go.mod)](https://golang.org)
[![License](https://img.shields.io/github/license/muozez/ephem-centralized-access-broker)](LICENSE)

> **"Infrastructure access, issued on demand."**
> 
> *ephem* (from *ephemeral*) is an enterprise-grade, centralized access broker that implements the **Zero Standing Privileges (ZSP)** philosophy. Instead of distributing permanent credentials or standing access to critical resources, ephem authenticates users via OpenID Connect (OIDC) and provisions short-lived, automated credentials and signed SSH certificates on target systems on-demand.

---

## Table of Contents

1. [Key Features](#key-features)
2. [Architectural Overview](#architectural-overview)
3. [Database Schema](#database-schema)
4. [Supported Providers](#supported-providers)
   - [PostgreSQL Provider (Role Provisioning)](#postgresql-provider-role-provisioning)
   - [SSH Provider (CA Signed Certificates)](#ssh-provider-ca-signed-certificates)
5. [Getting Started (Dockerized Sandbox)](#getting-started-dockerized-sandbox)
6. [Usage Guide](#usage-guide)
7. [Security Hardening & Policies](#security-hardening--policies)
8. [Testing](#testing)

---

## Key Features

- **Zero Standing Privileges (ZSP):** No persistent passwords, API tokens, or permanent admin roles exist on target systems. All access is temporary, audited, and automatically revoked.
- **Asymmetric Cryptography (RS256 & JWKS):** All authorization tokens (JWTs) are signed using a 2048-bit RSA private key. The API exposes a standard JSON Web Key Set (JWKS) endpoint (`/.well-known/jwks.json`) for downstream validation.
- **SSH Certificate Authority (CA):** Implements dynamic, on-the-fly user key pair generation and SSH certificate signing. The target SSH host disables password authentication and trusts the broker's CA public key for secure access without pre-configured keys on the host.
- **CSRF-Hardened Authentication:** Features state verification at both the CLI loopback listener level and the OAuth2 redirection flow to prevent request injection.
- **Glob-Based Policy Engine:** Access control rules support wildcard/glob syntax for resource matching combined with maximum session duration limits.
- **Provider-Agnostic Plugin Registry:** Providers (such as Postgres and SSH) implement a clean, standard interface to issue and revoke credentials.
- **Comprehensive Audit Trail:** All access requests, policy evaluations, and revocations are saved to structured audit logs.

---

## Architectural Overview

```
                      +-------------------+
                      |     ephem-cli     |
                      +---------+---------+
                                |
                   1. REST Request (Signed JWT)
                                |
                                v
                      +-------------------+
                      |     ephem-api     | (Central Broker)
                      +---------+---------+
                                |
                     2. Policy Engine Check
                                |
                                v
                      +-------------------+
                      | Provider Registry |
                      +----+-----------+--+
                           |           |
        3a. Create Temp    |           | 3b. Sign Client Key
            Postgres Role  |           |     With CA Private Key
                           v           v
                    +----------+   +----------+
                    | Postgres |   | Target   |
                    | DB Host  |   | SSH Host |
                    +----------+   +----------+
```

### Authentication Lifecycle

1. **Initiate Login:** The user runs `ephem login`. The CLI starts a local HTTP loopback server, generates a random CSRF state, and opens the user's browser to the OIDC provider's login URL.
2. **Identity Verification:** The OIDC provider (or Mock Developer UI in dev environments) validates the identity.
3. **Security Check & Enrollment:** The API checks configured domain restrictions and pre-registration policies. If JIT (Just-in-Time) provisioning is enabled, the user is created and assigned a default role. If JIT is disabled, the login is rejected unless the user was pre-registered by an admin.
4. **Token Generation:** The API fetches the user's roles from the database and returns a signed RS256 JWT containing the user ID, email, name, and database-managed roles.
5. **Credential Storage:** The CLI validates the state parameter, receives the JWT, and saves it locally in `~/.ephem/token`.

---

## Database Schema

The persistence layer is structured for compliance and strict access tracking:

- **`users`:** Holds user metadata (email, name, external ID, provider).
- **`roles`:** Stores permission roles (`admin`, `developer`, `dba`, `devops`).
- **`user_roles`:** Junction table mapping users to roles.
- **`provider_configs`:** Houses targeted resource connection details (e.g. host, port, database, credentials) with optional custom configurations.
- **`resources`:** The Resource Registry containing target infrastructure objects, environments, ownership teams, and labels.
- **`policies`:** Stores access policies mapping roles to resource name globs (e.g. `postgres-staging-*`) with permission effects (`ALLOW`/`DENY`) and maximum session duration conditions.
- **`sessions`:** Tracks requested and active ephemeral credential lifecycles. **Hassas şifreler (passwords) veritabanına asla kaydedilmez;** they are only returned to the CLI in memory.
- **`audit_logs`:** Secure JSON-structured logs of all system events.

---

## Supported Providers

### PostgreSQL Provider (Role Provisioning)
Provisions short-lived PostgreSQL database roles on-demand.
- **Issuance:** Generates a temporary username (`ephem_u_<session_id>`) and a secure password. Creates the role with `LOGIN` and grants standard permissions.
- **Revocation:** Terminates all active connections for the temporary role, revokes its grants, and drops the role.

### SSH Provider (CA Signed Certificates)
Enables passwordless, secure SSH access using short-lived SSH user certificates.
- **Issuance:** Generates a temporary RSA keypair. Signs the public key using the broker's CA private key, setting the principal to the target username and the certificate expiry to the session duration.
- **Revocation:** Since the certificate naturally expires based on the duration enforced by the target SSH daemon, no active revocation is required. The CLI deletes key assets from disk immediately on exit.

---

## Getting Started (Dockerized Sandbox)

We provide a complete multi-container sandbox environment in `docker-compose.yml` which includes:
1. **`postgres`:** Central database containing schemas, configs, resources, and policy seeds.
2. **`ephem-api`:** The central access broker compiling and running Go code.
3. **`target-ssh`:** An Alpine-based SSH server configured with `PasswordAuthentication no` and trusting the broker's SSH CA public key.

### Prerequisites
- Go (1.22+)
- Docker & Docker Compose

### Starting the Sandbox

1. Clone the repository:
   ```bash
   git clone https://github.com/muozez/ephem-centralized-access-broker.git
   cd ephem-centralized-access-broker
   ```

2. Spin up the containers (this compiles the API and configures SSH CA Trust automatically):
   ```bash
   docker-compose up --build -d
   ```

3. Build the CLI binary:
   ```bash
   make build
   ```

---

## Usage Guide

### 1. Log in with the CLI
Run the login sequence:
```bash
./bin/ephem login
```
Open the printed link in your browser. Enter `developer@company.com` (which is pre-registered in the seed data) to log in.

### 2. Verify Identity and Health
Check the current session and run self-diagnostics:
```bash
./bin/ephem whoami
./bin/ephem doctor
```

### 3. Connect to Ephemeral PostgreSQL
Use the convenience command to connect to the database via `psql` (the database credentials are dynamically generated, mapped to standard env variables, passed to `psql`, and revoked immediately when you exit `psql`):
```bash
./bin/ephem postgres staging
```

### 4. Connect to Target SSH Host
Run an interactive SSH shell on the `target-ssh` host using short-lived SSH certificates (no passwords or manual keys required):
```bash
./bin/ephem ssh staging
```

You can also execute non-interactive remote commands securely:
```bash
./bin/ephem exec ssh-staging -- ssh id
```

### 5. Log out
Delete the local session token:
```bash
./bin/ephem logout
```

---

## Distributed Agent Architecture (Bring Your Own Agent)

For secure infrastructure access across private networks or isolated VPCs, `ephem` provides a **Distributed Agent Architecture**. The central `ephem-api` server does not require direct line-of-sight to target databases or machines. Instead, it delegates credential provisioning to a lightweight gRPC agent (`ephem-agent`) running locally inside the target network.

### How the Agent Works

1. **Mutual TLS (mTLS) Security:** The connection between `ephem-api` and `ephem-agent` is secured using Mutual TLS. Both endpoints present X.509 certificates signed by a shared Certificate Authority (CA) and verify each other before transmitting payloads.
2. **gRPC Protocol:** Sessions are requested and revoked via protobuf-defined gRPC methods over HTTP/2.
3. **Local Provisioning:** When the Central API receives an access request for a remote resource:
   - It checks authorization and policy rules locally.
   - It forwards the request to the target `ephem-agent` over gRPC.
   - The Agent runs the target provider logic locally (e.g. creating a PostgreSQL user inside its private subnet) and returns the credentials to the API.
4. **Asynchronous Revocation:** Revocations are scheduled via a Redis-backed queue (`Sorted Set`) in the Central API. When a session expires, the scheduler pops it and requests the agent to remove the database role/credentials immediately.

```
       +-----------------------+
       |   Developer/Client    |
       +-----------+-----------+
                   |
            1. Request access
                   |
                   v
       +-----------------------+
       |       ephem-api       | (Central Control Plane)
       +-----------+-----------+
                   |
     2. gRPC Call over mTLS (VPC Border)
                   |
                   v
       +-----------------------+
       |      ephem-agent      | (Lightweight Agent inside subnet)
       +-----------+-----------+
                   |
        3. Local provisioning (e.g., PostgreSQL / Redis)
                   |
                   v
       +-----------------------+
       |    Target Resource    |
       +-----------------------+
```

### Bring Your Own Agent (BYOA) Setup

Connecting a new agent to your central `ephem` control plane takes 3 simple steps:

#### 1. Generate Certificates
Generate server and client certificates signed by your shared internal CA. Place them on the agent machine (e.g., inside `/etc/ephem/certs`):
- `ca.crt` (CA Root Certificate)
- `agent.crt` (Agent Server Certificate)
- `agent.key` (Agent Private Key)

#### 2. Start the Agent
Launch the `ephem-agent` binary in the target network:
```bash
./bin/ephem-agent \
  --port 50051 \
  --cert-dir /etc/ephem/certs \
  --config /etc/ephem/agent_config.json
```
*(The agent automatically registers supported local providers like postgres or redis)*

#### 3. Register the Remote Resource in the Central API
Add the agent provider configuration and target resource in the `ephem` PostgreSQL registry:

```sql
-- 1. Insert remote configuration pointing to the agent endpoint
INSERT INTO provider_configs (id, provider, host, port, authentication_method, config_extra)
VALUES (
  '8795ea33-88cf-4824-954f-123456789abc',
  'remote',
  'agent-hostname-or-ip', -- Target agent host
  50051,                  -- Target agent port
  'agent',
  '{
     "endpoint": "ephem-agent:50051", 
     "delegate_provider": "postgres",
     "delegate_config": {
       "host": "postgres-internal-db",
       "port": 5432,
       "user": "root_user",
       "password": "root_password",
       "dbname": "target_db"
     }
  }'::jsonb
);

-- 2. Map target resource to the provider configuration
INSERT INTO resources (provider, name, display_name, config_id, environment, owner_team, enabled)
VALUES (
  'remote',
  'remote-postgres',
  'Remote Production Database',
  '8795ea33-88cf-4824-954f-123456789abc',
  'production',
  'data-team',
  true
);
```

Once registered, client commands like `ephem exec remote-postgres -- psql` will automatically route through your agent seamlessly.

---

## Security Hardening & Policies

To ensure production-grade security, the access broker can be configured using environment variables on the API server:

| Variable | Description | Default |
| --- | --- | --- |
| `OIDC_ENABLED` | Set to `true` to disable Mock login and enable real OIDC discovery flow. | `false` |
| `OIDC_ALLOW_JIT_PROVISIONING` | Allow new users logging in via OIDC to automatically register. | `false` |
| `OIDC_DEFAULT_ROLE` | The default role assigned to new JIT-provisioned users. | `developer` |
| `OIDC_ALLOWED_DOMAINS` | Comma-separated list of domains allowed to log in (e.g. `company.com`). | (no restriction) |
| `RSA_PRIVATE_KEY_PATH` | Path to custom PEM-encoded RSA Private Key for JWT signing. | (auto-generates dynamic key) |
| `SSH_CA_PRIVATE_KEY_PATH` | Path to custom SSH CA Private Key for signing SSH host certificates. | `/shared/ssh_ca.key` |
| `SSH_CA_PUBLIC_KEY_PATH` | Path to custom SSH CA Public Key. | `/shared/ssh_ca.pub` |

---

## Testing

### Unit Tests
Verify the cryptographic operations and JWKS publishing:
```bash
make test
```

### Integration Tests
Run sandbox-specific testing scripts to verify logins, policies, and DB/SSH lifecycles:
```bash
# Verify OIDC security policies (CSRF, domain blocks, enrollment)
./scratch/test_login.sh

# Verify PostgreSQL Provider & Policy Engine
./scratch/test_postgres_provider.sh
```