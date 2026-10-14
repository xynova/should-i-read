CREATE TABLE clusters (
  cluster_id TEXT PRIMARY KEY,
  fingerprint TEXT NOT NULL UNIQUE,
  term_id TEXT NOT NULL DEFAULT '',
  sender_domain TEXT NOT NULL DEFAULT '',
  size INTEGER NOT NULL,
  tldr TEXT NOT NULL DEFAULT '',
  suggested_action TEXT NOT NULL DEFAULT 'review'
    CHECK (suggested_action IN ('review', 'quarantine_candidate')),
  tldr_model TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL
);

CREATE INDEX clusters_fingerprint ON clusters (fingerprint);

CREATE TABLE cluster_members (
  cluster_id TEXT NOT NULL,
  object_hash TEXT NOT NULL,
  PRIMARY KEY (cluster_id, object_hash),
  FOREIGN KEY (cluster_id) REFERENCES clusters(cluster_id) ON DELETE CASCADE
);

CREATE INDEX cluster_members_hash ON cluster_members (object_hash);
