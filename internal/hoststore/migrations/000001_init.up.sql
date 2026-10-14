CREATE TABLE classify_state (
  object_hash TEXT PRIMARY KEY,
  status TEXT NOT NULL CHECK (status IN ('complete', 'error', 'running')),
  term_id TEXT NOT NULL DEFAULT '',
  path_json TEXT NOT NULL DEFAULT '[]',
  label TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  strategy TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL DEFAULT '',
  about TEXT NOT NULL DEFAULT '',
  shape TEXT NOT NULL DEFAULT '',
  cosine REAL NOT NULL DEFAULT 0,
  canonical_term_id TEXT NOT NULL DEFAULT '',
  reinforced INTEGER NOT NULL DEFAULT 0,
  judge_score REAL NOT NULL DEFAULT 0,
  draft_kind TEXT NOT NULL DEFAULT '',
  catalog_applied INTEGER NOT NULL DEFAULT 0,
  sender_term_id TEXT NOT NULL DEFAULT '',
  sender_label TEXT NOT NULL DEFAULT '',
  sender_maps_to TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  catalog_id TEXT NOT NULL DEFAULT '',
  classify_model TEXT NOT NULL DEFAULT '',
  embed_model TEXT NOT NULL DEFAULT '',
  claim_token TEXT NOT NULL DEFAULT '',
  lease_until TEXT NOT NULL DEFAULT '',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);

CREATE INDEX classify_state_status ON classify_state (status);

CREATE TABLE embeddings (
  object_hash TEXT NOT NULL,
  model_id TEXT NOT NULL,
  dim INTEGER NOT NULL,
  vector BLOB NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (object_hash, model_id)
);
