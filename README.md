# ephem — Centralized Ephemeral Access Broker

[![Go Version](https://img.shields.io/github/go-mod/go-version/muozez/ephem-centralized-access-broker?filename=go.mod)](https://golang.org)
[![License](https://img.shields.io/github/license/muozez/ephem-centralized-access-broker)](LICENSE)

> **"Infrastructure access, issued on demand."**
> 
> *ephem* (from *ephemeral*) is an enterprise-grade, centralized access broker that implements the **Zero Standing Privileges (ZSP)** philosophy. Instead of distributing permanent credentials or standing access to critical resources, ephem authenticates users via OpenID Connect (OIDC) and provisions short-lived, automated credentials on the target systems on-demand.

---

## Table of Contents

1. [Key Features](#key-features)
2. [Architectural Overview](#architectural-overview)
3. [Database Schema](#database-schema)
4. [Getting Started](#getting-started)
5. [Usage Guide](#usage-guide)
6. [Security Hardening & Policies](#security-hardening--policies)
7. [Testing](#testing)

---

## Key Features

- **Zero Standing Privileges (ZSP):** No persistent passwords, API tokens, or permanent admin roles exist on target systems. All access is temporary and automatically revoked.
- **Asymmetric Cryptography (RS256 & JWKS):** All authorization tokens (JWTs) are signed using a 2048-bit RSA private key. The API exposes a standard JSON Web Key Set (JWKS) endpoint (`/.well-known/jwks.json`) for downstream validation.
- **CSRF-Hardened Authentication:** Features state verification at both the CLI loopback listener level and the OAuth2 redirection flow to prevent request injection.
- **Glob-Based Policy Engine:** Access control rules support wildcard/glob syntax for resource matching combined with maximum session duration limits.
- **Provider-Agnostic Plugin Registry:** Providers (such as the local PostgreSQL Provider) implement a clean, standard interface to issue and revoke credentials.
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
                      |     ephem-api     |
                      +---------+---------+
                                |
                    2. Policy Engine Check
                                |
                                v
                      +-------------------+
                      | Provider Registry |
                      +---------+---------+
                                |
                 3. Issue Temporary Postgres Role
                                |
                                v
                      +-------------------+
                      |  Target Resource  |
                      +-------------------+
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

## Getting Started

### Prerequisites

- Go (1.22+)
- Docker & Docker Compose
- `psql` (optional, for manual verification)

### Setting Up the Environment

1. Clone the repository:
   ```bash
   git clone https://github.com/muozez/ephem-centralized-access-broker.git
   cd ephem-centralized-access-broker
   ```

2. Start the PostgreSQL database container (contains the schema and initial seed data):
   ```bash
   docker-compose up -d
   ```

3. Build the CLI and API binaries:
   ```bash
   make build
   ```
   This compiles `ephem` and `ephem-api` into the `bin/` directory.

---

## Usage Guide

### 1. Start the API Server
Run the API server (it defaults to port `8080`):
```bash
./bin/ephem-api
```

### 2. Log in with the CLI
In a new terminal window, execute:
```bash
./bin/ephem login
```
Open the printed link in your browser. If OIDC is disabled (default dev mode), you will see the developer mock login interface. Enter `developer@company.com` (which is pre-registered in the seed data) to log in.

### 3. Verify Identity
Check your currently logged-in user profile and roles:
```bash
./bin/ephem whoami
```

### 4. Log out
To delete the local session token:
```bash
./bin/ephem logout
```

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

---

## Testing

### Unit Tests
Verify the JWT signing logic and verify key sets:
```bash
make test
```

### Integration Tests
To test the user enrollment policies and PostgreSQL Provider end-to-end (role provisioning, connectivity verification, and dropping roles from the real container):

```bash
# Verify OIDC security policies (CSRF, pre-registration block, whitelists)
./scratch/test_login.sh

# Verify PostgreSQL Provider & Policy Engine (issuing and revoking temporary roles)
./scratch/test_postgres_provider.sh
```