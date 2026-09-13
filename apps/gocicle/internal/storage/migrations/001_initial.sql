CREATE TABLE users (
 id text PRIMARY KEY, name text NOT NULL, enabled boolean NOT NULL DEFAULT false,
 administrator boolean NOT NULL DEFAULT false, preferences jsonb NOT NULL DEFAULT '{}',
 version bigint NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE provider_instances (
 id text PRIMARY KEY, name text NOT NULL UNIQUE, kind text NOT NULL,
 config jsonb NOT NULL DEFAULT '{}', version bigint NOT NULL DEFAULT 1
);
CREATE TABLE identities (
 id text PRIMARY KEY, user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 provider_id text NOT NULL REFERENCES provider_instances(id), subject text NOT NULL,
 profile jsonb NOT NULL DEFAULT '{}', UNIQUE(provider_id, subject)
);
CREATE TABLE sessions (
 hash text PRIMARY KEY, user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 csrf_hash text NOT NULL, expires_at timestamptz NOT NULL
);
CREATE TABLE api_tokens (
 id text PRIMARY KEY, user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 hash text NOT NULL UNIQUE, scopes jsonb NOT NULL, expires_at timestamptz NOT NULL
);
CREATE TABLE invitations (
 hash text PRIMARY KEY, administrator boolean NOT NULL DEFAULT false,
 expires_at timestamptz NOT NULL, used_at timestamptz
);
CREATE TABLE oauth_states (
 hash text PRIMARY KEY, provider_id text NOT NULL REFERENCES provider_instances(id),
 user_id text REFERENCES users(id), invitation_hash text,
 encrypted jsonb NOT NULL, expires_at timestamptz NOT NULL
);
CREATE TABLE projects (
 id text PRIMARY KEY, name text NOT NULL UNIQUE, owner_id text NOT NULL REFERENCES users(id),
 source jsonb NOT NULL, preferences jsonb NOT NULL DEFAULT '{}', version bigint NOT NULL DEFAULT 1
);
CREATE TABLE memberships (
 project_id text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 role text NOT NULL CHECK(role IN ('owner','operator','viewer')), PRIMARY KEY(project_id,user_id)
);
CREATE TABLE secrets (
 id text PRIMARY KEY, owner_id text NOT NULL REFERENCES users(id), name text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('environment','https','ssh','notification','oauth')),
 encrypted jsonb NOT NULL, version bigint NOT NULL DEFAULT 1
);
CREATE TABLE credential_grants (
 secret_id text NOT NULL REFERENCES secrets(id) ON DELETE CASCADE,
 project_id text NOT NULL REFERENCES projects(id) ON DELETE CASCADE, PRIMARY KEY(secret_id,project_id)
);
CREATE TABLE connections (
 id text PRIMARY KEY, owner_id text NOT NULL REFERENCES users(id), name text NOT NULL,
 provider_id text NOT NULL REFERENCES provider_instances(id), secret_id text REFERENCES secrets(id),
 config jsonb NOT NULL DEFAULT '{}', version bigint NOT NULL DEFAULT 1
);
CREATE TABLE ssh_keys (
 id text PRIMARY KEY, owner_id text NOT NULL REFERENCES users(id), name text NOT NULL,
 secret_id text NOT NULL REFERENCES secrets(id), config jsonb NOT NULL DEFAULT '{}', version bigint NOT NULL DEFAULT 1
);
CREATE TABLE notification_destinations (
 id text PRIMARY KEY, owner_id text NOT NULL REFERENCES users(id), name text NOT NULL,
 secret_id text NOT NULL REFERENCES secrets(id), config jsonb NOT NULL DEFAULT '{}', version bigint NOT NULL DEFAULT 1
);
CREATE TABLE runners (
 id text PRIMARY KEY, name text NOT NULL UNIQUE, token_hash text NOT NULL UNIQUE,
 labels jsonb NOT NULL DEFAULT '[]', approved_roots jsonb NOT NULL DEFAULT '[]',
 enabled boolean NOT NULL DEFAULT false, last_seen timestamptz, version bigint NOT NULL DEFAULT 1
);
CREATE TABLE runner_grants (
 runner_id text NOT NULL REFERENCES runners(id) ON DELETE CASCADE,
 user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE, PRIMARY KEY(runner_id,user_id)
);
CREATE TABLE enrollments (
 hash text PRIMARY KEY, name text NOT NULL, labels jsonb NOT NULL, approved_roots jsonb NOT NULL,
 expires_at timestamptz NOT NULL
);
CREATE TABLE jobs (
 id text PRIMARY KEY, project_id text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 name text NOT NULL, revision_id text, version bigint NOT NULL DEFAULT 1,
 enabled boolean NOT NULL DEFAULT false, UNIQUE(project_id,name)
);
CREATE TABLE job_revisions (
 id text PRIMARY KEY, job_id text NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
 spec jsonb NOT NULL, source jsonb NOT NULL, created_by text NOT NULL REFERENCES users(id),
 created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE jobs ADD CONSTRAINT jobs_revision_fk FOREIGN KEY(revision_id) REFERENCES job_revisions(id) DEFERRABLE INITIALLY DEFERRED;
CREATE TABLE schedules (
 job_id text PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
 revision_id text NOT NULL REFERENCES job_revisions(id), expression text NOT NULL,
 timezone text NOT NULL, next_at timestamptz NOT NULL
);
CREATE TABLE runs (
 id text PRIMARY KEY, job_id text NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
 revision_id text NOT NULL REFERENCES job_revisions(id), project_id text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 preferences jsonb NOT NULL, status text NOT NULL CHECK(status IN ('queued','running','success','failure','timeout','cancelled','lost','skipped','missed','push_failure')),
 scheduled_at timestamptz, created_at timestamptz NOT NULL DEFAULT now(), started_at timestamptz, finished_at timestamptz,
 runner_id text REFERENCES runners(id), source_commit text NOT NULL DEFAULT '', image_identity text NOT NULL DEFAULT '',
 result jsonb NOT NULL DEFAULT '{}', push_result text NOT NULL DEFAULT '', cancel_requested boolean NOT NULL DEFAULT false,
 log_bytes bigint NOT NULL DEFAULT 0, log_truncated boolean NOT NULL DEFAULT false,
 UNIQUE(revision_id,scheduled_at)
);
CREATE UNIQUE INDEX one_active_run ON runs(job_id) WHERE status IN ('queued','running');
CREATE INDEX runs_project_history ON runs(project_id,created_at DESC);
CREATE TABLE runner_leases (
 run_id text PRIMARY KEY REFERENCES runs(id) ON DELETE CASCADE,
 runner_id text NOT NULL REFERENCES runners(id), token_hash text NOT NULL, expires_at timestamptz NOT NULL,
 deadline timestamptz NOT NULL
);
CREATE TABLE run_locks (
 run_id text NOT NULL REFERENCES runs(id) ON DELETE CASCADE, key text NOT NULL UNIQUE,
 PRIMARY KEY(run_id,key)
);
CREATE TABLE run_events (
 run_id text NOT NULL REFERENCES runs(id) ON DELETE CASCADE, sequence bigint NOT NULL CHECK(sequence>0),
 kind text NOT NULL, payload_hash text NOT NULL, data jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(run_id,sequence)
);
CREATE TABLE log_chunks (
 run_id text NOT NULL REFERENCES runs(id) ON DELETE CASCADE, sequence bigint NOT NULL,
 content text NOT NULL, PRIMARY KEY(run_id,sequence)
);
CREATE TABLE metric_samples (
 run_id text NOT NULL REFERENCES runs(id) ON DELETE CASCADE, sequence bigint NOT NULL,
 data jsonb NOT NULL, PRIMARY KEY(run_id,sequence)
);
CREATE TABLE notification_deliveries (
 id text PRIMARY KEY, run_id text NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
 destination_id text NOT NULL REFERENCES notification_destinations(id) ON DELETE CASCADE,
 event text NOT NULL, attempts integer NOT NULL DEFAULT 0, next_at timestamptz NOT NULL DEFAULT now(),
 delivered_at timestamptz, last_error text NOT NULL DEFAULT '', UNIQUE(run_id,destination_id,event)
);
CREATE TABLE audit_events (
 id text PRIMARY KEY, actor_id text REFERENCES users(id), action text NOT NULL,
 target text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE idempotency_keys (
 actor_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE, key text NOT NULL,
 request_hash text NOT NULL, response jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(actor_id,key)
);
CREATE FUNCTION prevent_revision_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Job revisions are immutable'; END;
$$;
CREATE TRIGGER immutable_revision BEFORE UPDATE ON job_revisions FOR EACH ROW EXECUTE FUNCTION prevent_revision_update();
CREATE TABLE inspections (
 id text PRIMARY KEY, project_id text NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
 runner_id text NOT NULL REFERENCES runners(id), source jsonb NOT NULL,
 status text NOT NULL DEFAULT 'queued', token_hash text NOT NULL DEFAULT '',
 expires_at timestamptz, deadline timestamptz, document jsonb, error text NOT NULL DEFAULT '',
 created_at timestamptz NOT NULL DEFAULT now()
);
