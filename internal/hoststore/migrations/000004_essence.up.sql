CREATE TABLE essence_cache (
  object_hash TEXT PRIMARY KEY,
  kind TEXT NOT NULL DEFAULT '',
  about TEXT NOT NULL DEFAULT '',
  shape TEXT NOT NULL DEFAULT '',
  why TEXT NOT NULL DEFAULT '',
  essence_model TEXT NOT NULL,
  escalate_model TEXT NOT NULL DEFAULT '',
  prompt_hash TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
