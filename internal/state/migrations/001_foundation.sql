CREATE TABLE schema_migrations (
  version INTEGER PRIMARY KEY, checksum TEXT NOT NULL, applied_at INTEGER NOT NULL
);
CREATE TABLE config_revisions (
  id TEXT PRIMARY KEY, snapshot BLOB NOT NULL, created_at INTEGER NOT NULL
);
CREATE TABLE active_config (
  singleton INTEGER PRIMARY KEY CHECK(singleton=1),
  config_id TEXT NOT NULL REFERENCES config_revisions(id)
);
CREATE TABLE job_revisions (
  id TEXT PRIMARY KEY, job_id TEXT NOT NULL, source_id TEXT NOT NULL,
  config_id TEXT NOT NULL REFERENCES config_revisions(id), snapshot BLOB NOT NULL,
  UNIQUE(job_id,config_id)
);
CREATE TABLE runs (
  id TEXT PRIMARY KEY, job_revision_id TEXT NOT NULL REFERENCES job_revisions(id),
  request_id TEXT NOT NULL UNIQUE, payload_hash TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('queued','preflight','dumping','delivering','succeeded','degraded','failed','cancelled','interrupted')),
  version INTEGER NOT NULL CHECK(version>=1), started_at INTEGER NOT NULL,
  finished_at INTEGER, error_code TEXT NOT NULL DEFAULT ''
);
CREATE INDEX runs_history ON runs(started_at DESC,id DESC);
CREATE TABLE run_commands (
  request_id TEXT PRIMARY KEY, run_id TEXT NOT NULL REFERENCES runs(id),
  kind TEXT NOT NULL CHECK(kind='cancel'), payload_hash TEXT NOT NULL,
  result_version INTEGER NOT NULL
);
CREATE TABLE events (
  id INTEGER PRIMARY KEY, run_id TEXT NOT NULL REFERENCES runs(id),
  sequence INTEGER NOT NULL, event_type TEXT NOT NULL, payload BLOB NOT NULL,
  occurred_at INTEGER NOT NULL, UNIQUE(run_id,sequence)
);
CREATE TABLE channel_revisions (
  id TEXT PRIMARY KEY, config_id TEXT NOT NULL REFERENCES config_revisions(id),
  channel_id TEXT NOT NULL, snapshot BLOB NOT NULL, UNIQUE(config_id,channel_id)
);
CREATE TABLE deliveries (
  event_id INTEGER NOT NULL REFERENCES events(id),
  channel_revision_id TEXT NOT NULL REFERENCES channel_revisions(id),
  state TEXT NOT NULL CHECK(state IN ('pending','cancelled')),
  PRIMARY KEY(event_id,channel_revision_id)
);
CREATE TABLE health_probe (singleton INTEGER PRIMARY KEY CHECK(singleton=1), tick INTEGER NOT NULL);
INSERT INTO health_probe VALUES(1,0);
