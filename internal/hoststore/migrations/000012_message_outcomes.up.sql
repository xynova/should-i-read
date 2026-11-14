CREATE TABLE message_outcomes (
  object_hash TEXT PRIMARY KEY,
  outcome TEXT NOT NULL CHECK (outcome IN ('trashed_at_source', 'already_in_trash', 'kept')),
  source TEXT NOT NULL CHECK (source IN ('delete_job', 'replica_folder', 'overview_keep')),
  job_id TEXT NOT NULL DEFAULT '',
  sender TEXT NOT NULL DEFAULT '',
  decided_at TEXT NOT NULL
);

CREATE INDEX message_outcomes_outcome ON message_outcomes (outcome);
