CREATE TABLE delete_jobs (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  confirm_token TEXT NOT NULL,
  created_at TEXT NOT NULL,
  confirmed_at TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT ''
);

CREATE TABLE delete_job_items (
  job_id TEXT NOT NULL,
  object_hash TEXT NOT NULL,
  group_id TEXT NOT NULL DEFAULT '',
  group_label TEXT NOT NULL DEFAULT '',
  sender TEXT NOT NULL DEFAULT '',
  subject TEXT NOT NULL DEFAULT '',
  collection TEXT NOT NULL DEFAULT '',
  seq INTEGER NOT NULL DEFAULT 0,
  link_id TEXT NOT NULL DEFAULT '',
  envelope_id TEXT NOT NULL DEFAULT '',
  folder TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (job_id, object_hash),
  FOREIGN KEY (job_id) REFERENCES delete_jobs(id)
);

CREATE INDEX delete_job_items_status ON delete_job_items (job_id, status);
