-- Seed Data: Provider Configs
INSERT INTO provider_configs (id, provider, host, port, database, authentication_method, config_extra)
VALUES
  (
    '10000000-1000-1000-1000-100000000000',
    'postgres',
    'postgres',
    5432,
    'ephem',
    'password',
    '{"host": "postgres", "port": 5432, "database": "ephem", "user": "ephem", "password": "ephem_password", "sslmode": "disable", "client_host": "localhost", "client_port": 5432}'
  ),
  (
    '10000000-1000-1000-1000-111111111111',
    'ssh',
    'target-ssh',
    22,
    '',
    'ssh_ca',
    '{"host": "target-ssh", "port": 22, "username": "ubuntu", "client_host": "localhost", "client_port": 2222}'
  )
ON CONFLICT (id) DO NOTHING;

-- Seed Data: Resources
INSERT INTO resources (id, provider, name, display_name, config_id, environment, owner_team, labels, enabled)
VALUES
  (
    '20000000-2000-2000-2000-200000000000',
    'postgres',
    'postgres-staging',
    'Staging Postgres Database',
    '10000000-1000-1000-1000-100000000000',
    'staging',
    'data-team',
    '{"env": "staging"}',
    true
  ),
  (
    '20000000-2000-2000-2000-111111111111',
    'ssh',
    'ssh-staging',
    'Staging SSH Server',
    '10000000-1000-1000-1000-111111111111',
    'staging',
    'infra-team',
    '{"env": "staging"}',
    true
  )
ON CONFLICT (name) DO NOTHING;

-- Seed Data: Policies
INSERT INTO policies (id, role_id, resource_glob, effect, conditions)
VALUES
  (
    '30000000-3000-3000-3000-200000000000',
    (SELECT id FROM roles WHERE name = 'developer'),
    'postgres-*',
    'ALLOW',
    '{"max_duration": "1h"}'
  ),
  (
    '30000000-3000-3000-3000-111111111111',
    (SELECT id FROM roles WHERE name = 'developer'),
    'ssh-*',
    'ALLOW',
    '{"max_duration": "1h"}'
  )
ON CONFLICT (id) DO NOTHING;
