-- Enable pgcrypto for gen_random_uuid() just in case, though it is standard in modern PG
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Table: users
CREATE TABLE IF NOT EXISTS users (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email       TEXT NOT NULL UNIQUE,
  name        TEXT,
  provider    TEXT NOT NULL,  -- github | google | oidc
  external_id TEXT NOT NULL,
  created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- Table: roles
CREATE TABLE IF NOT EXISTS roles (
  id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE  -- admin, developer, dba, devops
);

-- Table: user_roles
CREATE TABLE IF NOT EXISTS user_roles (
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, role_id)
);

-- Table: provider_configs
CREATE TABLE IF NOT EXISTS provider_configs (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider              TEXT NOT NULL,               -- e.g., 'postgres', 'mysql', 'aws'
  host                  TEXT,
  port                  INTEGER,
  database              TEXT,
  authentication_method TEXT NOT NULL,               -- 'password' | 'iam' | 'tls' | 'agent' | 'vault'
  config_extra          JSONB,                       -- Ek metod verileri (örn. AWS role ARN)
  created_at            TIMESTAMPTZ DEFAULT NOW(),
  updated_at            TIMESTAMPTZ DEFAULT NOW()
);

-- Table: resources (Resource Registry)
CREATE TABLE IF NOT EXISTS resources (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  provider     TEXT NOT NULL,
  name         TEXT NOT NULL UNIQUE,       -- e.g., 'postgres-production'
  display_name TEXT,
  config_id    UUID REFERENCES provider_configs(id) ON DELETE RESTRICT,
  environment  TEXT NOT NULL,               -- e.g., 'production', 'staging'
  owner_team   TEXT,                        -- e.g., 'data-team'
  labels       JSONB,                       -- e.g., {"pci": "true", "critical": "true", "region": "eu-central"}
  enabled      BOOLEAN DEFAULT TRUE,
  created_at   TIMESTAMPTZ DEFAULT NOW()
);

-- Table: policies
CREATE TABLE IF NOT EXISTS policies (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  role_id       UUID REFERENCES roles(id) ON DELETE CASCADE,
  resource_glob TEXT NOT NULL,              -- e.g., 'postgres-staging-*', 'postgres-production'
  effect        TEXT NOT NULL,              -- ALLOW | DENY
  conditions    JSONB                       -- { "max_duration": "1h", "require_approval": true }
);

-- Table: sessions
CREATE TABLE IF NOT EXISTS sessions (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID REFERENCES users(id) ON DELETE RESTRICT,
  resource_id  UUID REFERENCES resources(id) ON DELETE RESTRICT,
  provider     TEXT NOT NULL,
  issued_at    TIMESTAMPTZ,
  expires_at   TIMESTAMPTZ,
  revoked_at   TIMESTAMPTZ,
  status       TEXT NOT NULL DEFAULT 'REQUESTED', -- REQUESTED, PENDING_APPROVAL, APPROVED, ISSUING, ACTIVE, FAILED, EXPIRED, REVOKED
  approved_by  UUID REFERENCES users(id),
  approved_at  TIMESTAMPTZ,
  metadata     JSONB                       -- Oturumun geçici kullanıcı adı gibi hassas olmayan meta-verileri (örn: {"db_role": "ephem_u_123"})
);

-- Table: audit_logs
CREATE TABLE IF NOT EXISTS audit_logs (
  id            BIGSERIAL PRIMARY KEY,
  user_id       UUID REFERENCES users(id),
  session_id    UUID,
  action        TEXT NOT NULL,              -- login | request_session | approve_session | revoke_session | deny
  resource_name TEXT,                       -- Denormalize resource ismi (hızlı arama için)
  result        TEXT NOT NULL,              -- SUCCESS | DENIED | ERROR
  reason        TEXT,
  ip_address    INET,
  created_at    TIMESTAMPTZ DEFAULT NOW()
);

-- Seed Data: Roles
INSERT INTO roles (name) VALUES
  ('admin'),
  ('developer'),
  ('dba'),
  ('devops')
ON CONFLICT (name) DO NOTHING;

-- Seed Data: Pre-registered Users
INSERT INTO users (email, name, provider, external_id) VALUES
  ('admin@company.com', 'Admin User', 'oidc', 'ext-admin@company.com'),
  ('developer@company.com', 'Developer User', 'oidc', 'ext-developer@company.com')
ON CONFLICT (email) DO NOTHING;

-- Seed Data: User Roles Mapping
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r
WHERE u.email = 'admin@company.com' AND r.name = 'admin'
ON CONFLICT (user_id, role_id) DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r
WHERE u.email = 'developer@company.com' AND r.name = 'developer'
ON CONFLICT (user_id, role_id) DO NOTHING;


