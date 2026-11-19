CREATE TABLE ack_cart (
    object_hash TEXT PRIMARY KEY,
    sender TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    mailed_at TEXT NOT NULL DEFAULT '',
    added_at TEXT NOT NULL
);

CREATE TABLE ack_jobs (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  confirm_token TEXT NOT NULL,
  created_at TEXT NOT NULL,
  confirmed_at TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT ''
);

CREATE TABLE ack_job_items (
  job_id TEXT NOT NULL,
  object_hash TEXT NOT NULL,
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
  FOREIGN KEY (job_id) REFERENCES ack_jobs(id)
);

CREATE INDEX ack_job_items_status ON ack_job_items (job_id, status);

CREATE TABLE message_outcomes_acked (
  object_hash TEXT PRIMARY KEY,
  outcome TEXT NOT NULL CHECK (outcome IN ('trashed_at_source', 'already_in_trash', 'kept', 'acked')),
  source TEXT NOT NULL CHECK (source IN ('delete_job', 'replica_folder', 'overview_keep', 'ack_job')),
  job_id TEXT NOT NULL DEFAULT '',
  sender TEXT NOT NULL DEFAULT '',
  decided_at TEXT NOT NULL
);

INSERT INTO message_outcomes_acked (object_hash, outcome, source, job_id, sender, decided_at)
SELECT object_hash, outcome, source, job_id, sender, decided_at FROM message_outcomes;

DROP TABLE message_outcomes;
ALTER TABLE message_outcomes_acked RENAME TO message_outcomes;
CREATE INDEX message_outcomes_outcome ON message_outcomes (outcome);
