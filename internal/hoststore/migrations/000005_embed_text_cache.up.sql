CREATE TABLE embed_text_cache (
  text_sha256 TEXT NOT NULL,
  model_id TEXT NOT NULL,
  dim INTEGER NOT NULL,
  vector BLOB NOT NULL,
  created_at TEXT NOT NULL,
  PRIMARY KEY (text_sha256, model_id)
);
