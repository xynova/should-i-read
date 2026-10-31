CREATE TABLE mail_shapes (
  fingerprint TEXT PRIMARY KEY,
  sender_domain TEXT NOT NULL,
  subject_template TEXT NOT NULL,
  kind TEXT NOT NULL,
  term_id TEXT NOT NULL,
  canonical_term_id TEXT NOT NULL DEFAULT '',
  last_slot TEXT NOT NULL,
  prompt_hash TEXT NOT NULL,
  essence_model TEXT NOT NULL DEFAULT '',
  seed_object_hash TEXT NOT NULL DEFAULT '',
  hit_count INTEGER NOT NULL DEFAULT 0,
  confirm_count INTEGER NOT NULL DEFAULT 0,
  reject_count INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL CHECK (status IN ('accumulating', 'active', 'disabled')),
  updated_at TEXT NOT NULL
);

CREATE INDEX mail_shapes_status ON mail_shapes (status);
