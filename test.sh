#!/bin/bash
set -e

echo "Starting Postgres Provider Integration Test..."

# Ensure we clean up on exit
cleanup() {
  echo "Cleaning up..."
  kill $API_PID 2>/dev/null || true
  kill $CLI_PID 2>/dev/null || true
  fuser -k 8080/tcp || true
  rm -f api.log cli.log session_resp.json revoke_resp.json
}
trap cleanup EXIT

# Kill any existing server on 8080
fuser -k 8080/tcp || true

# -------------------------------------------------------------
# 1. Seed Postgres Registry, Configs, Resources and Policies
# -------------------------------------------------------------
echo "Seeding Postgres database config, resources, and policies..."
docker exec -i ephem-postgres psql -U ephem -d ephem -c "
-- Clean up old test configs/resources/policies to prevent foreign key issues
DELETE FROM policies WHERE resource_glob = 'postgres-*';
DELETE FROM sessions WHERE provider = 'postgres';
DELETE FROM resources WHERE name = 'postgres-staging';
DELETE FROM provider_configs WHERE provider = 'postgres';

-- Insert config
INSERT INTO provider_configs (id, provider, host, port, database, authentication_method, config_extra)
VALUES (
    '10000000-1000-1000-1000-100000000000',
    'postgres',
    'localhost',
    5432,
    'ephem',
    'password',
    '{\"host\": \"localhost\", \"port\": 5432, \"database\": \"ephem\", \"user\": \"ephem\", \"password\": \"ephem_password\", \"sslmode\": \"disable\"}'
);

-- Insert resource
INSERT INTO resources (id, provider, name, display_name, config_id, environment, owner_team, labels, enabled)
VALUES (
    '20000000-2000-2000-2000-200000000000',
    'postgres',
    'postgres-staging',
    'Staging Postgres Database',
    '10000000-1000-1000-1000-100000000000',
    'staging',
    'data-team',
    '{\"env\": \"staging\"}',
    true
);

-- Insert policy
INSERT INTO policies (id, role_id, resource_glob, effect, conditions)
VALUES (
    '30000000-3000-3000-3000-300000000000',
    (SELECT id FROM roles WHERE name = 'developer'),
    'postgres-*',
    'ALLOW',
    '{\"max_duration\": \"1h\"}'
);
"

# -------------------------------------------------------------
# 2. Start API Server
# -------------------------------------------------------------
echo "Starting API Server..."
PORT=8080 go run ./cmd/ephem-api > api.log 2>&1 &
API_PID=$!
sleep 2

if ! kill -0 $API_PID 2>/dev/null; then
  echo "API Server failed to start."
  cat api.log
  exit 1
fi

# -------------------------------------------------------------
# 3. Perform login as developer@company.com to save token
# -------------------------------------------------------------
echo "Starting CLI login flow..."
go run ./cmd/ephem login > cli.log 2>&1 &
CLI_PID=$!
sleep 2

PORT_VAL=$(grep -o "cli_port=[0-9]*" cli.log | head -n 1 | cut -d'=' -f2)
STATE_VAL=$(grep -o "state=[a-fA-F0-9]*" cli.log | head -n 1 | cut -d'=' -f2)

echo "Simulating callback for developer@company.com..."
curl -s -L -o /dev/null -X POST http://localhost:8080/v1/auth/callback \
  -d "email=developer@company.com" \
  -d "name=Developer User" \
  -d "cli_port=$PORT_VAL" \
  -d "state=$STATE_VAL"

sleep 1
kill $CLI_PID 2>/dev/null || true

# Read token
TOKEN=$(cat ~/.ephem/token)
if [ -z "$TOKEN" ]; then
  echo "Failed to load authentication token."
  exit 1
fi
echo "Loaded developer JWT token."

# -------------------------------------------------------------
# 4. Request a Temporary Postgres Session
# -------------------------------------------------------------
echo "Requesting temporary Postgres session..."
HTTP_CODE=$(curl -s -o session_resp.json -w "%{http_code}" -X POST http://localhost:8080/v1/sessions/request \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"resource_name": "postgres-staging", "duration": "30m"}')

echo "Session Request HTTP Code: $HTTP_CODE"
if [ "$HTTP_CODE" != "200" ]; then
  echo "Failed to request session!"
  cat session_resp.json
  exit 1
fi

cat session_resp.json

# Parse credentials using grep/sed
SESS_ID=$(grep -o '"session_id":"[^"]*' session_resp.json | cut -d'"' -f4)
TEMP_USER=$(grep -o '"username":"[^"]*' session_resp.json | cut -d'"' -f4)
TEMP_PASS=$(grep -o '"password":"[^"]*' session_resp.json | cut -d'"' -f4)

if [ -z "$SESS_ID" ] || [ -z "$TEMP_USER" ] || [ -z "$TEMP_PASS" ]; then
  echo "Failed to parse session credentials."
  exit 1
fi

echo "Successfully issued temporary Postgres credentials:"
echo "  Session ID: $SESS_ID"
echo "  Temp User:  $TEMP_USER"
echo "  Temp Pass:  $TEMP_PASS"

# -------------------------------------------------------------
# 5. Connect to target database as temporary user
# -------------------------------------------------------------
echo "Testing connectivity with temporary user..."
PGPASSWORD="$TEMP_PASS" docker exec -i ephem-postgres psql -U "$TEMP_USER" -d ephem -c "SELECT current_user, session_user;"

echo "Connectivity test succeeded!"

# -------------------------------------------------------------
# 6. Revoke temporary session
# -------------------------------------------------------------
echo "Revoking temporary session..."
REVOKE_CODE=$(curl -s -o revoke_resp.json -w "%{http_code}" -X POST http://localhost:8080/v1/sessions/revoke \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"session_id\": \"$SESS_ID\"}")

echo "Revoke Request HTTP Code: $REVOKE_CODE"
if [ "$REVOKE_CODE" != "200" ]; then
  echo "Failed to revoke session!"
  cat revoke_resp.json
  exit 1
fi
cat revoke_resp.json

# -------------------------------------------------------------
# 7. Verify temporary user no longer has access
# -------------------------------------------------------------
echo "Verifying temporary user access is revoked..."
set +e
PGPASSWORD="$TEMP_PASS" docker exec -i ephem-postgres psql -U "$TEMP_USER" -d ephem -c "SELECT 1;" > /dev/null 2>&1
STATUS=$?
set -e

if [ $STATUS -eq 0 ]; then
  echo "CRITICAL SECURITY ERROR: Temp user still has access after revoke!"
  exit 1
fi

echo "Hardening check passed: Temp user was successfully dropped from database."
echo "All Postgres Provider integration tests completed successfully!"

