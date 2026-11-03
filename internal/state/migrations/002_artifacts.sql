CREATE TABLE artifacts (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(id),
  source_id TEXT NOT NULL,
  database_scope TEXT NOT NULL,
  scope_kind TEXT NOT NULL CHECK(scope_kind IN ('database','globals')),
  engine TEXT NOT NULL CHECK(engine IN ('postgres','mongo')),
  tool_version TEXT NOT NULL,
  consistency TEXT NOT NULL CHECK(consistency IN ('postgres_per_database','mongo_live_not_guaranteed')),
  key_id TEXT NOT NULL,
  captured_from INTEGER NOT NULL,
  captured_to INTEGER,
  state TEXT NOT NULL CHECK(state IN ('writing','staged','failed','interrupted')),
  relative_path TEXT,
  ciphertext_size INTEGER CHECK(ciphertext_size>=0),
  ciphertext_sha256 TEXT,
  manifest_ref TEXT,
  manifest_size INTEGER CHECK(manifest_size>=0),
  manifest_sha256 TEXT,
  UNIQUE(run_id,source_id,scope_kind,database_scope)
);
CREATE INDEX artifacts_by_run ON artifacts(run_id,id);
