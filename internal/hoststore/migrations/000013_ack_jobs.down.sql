DROP INDEX IF EXISTS ack_job_items_status;
DROP TABLE IF EXISTS ack_job_items;
DROP TABLE IF EXISTS ack_jobs;
DROP TABLE IF EXISTS ack_cart;

CREATE TABLE message_outcomes_old (
  object_hash TEXT PRIMARY KEY,
  outcome TEXT NOT NULL CHECK (outcome IN ('trashed_at_source', 'already_in_trash', 'kept')),
  source TEXT NOT NULL CHECK (source IN ('delete_job', 'replica_folder', 'overview_keep')),
  job_id TEXT NOT NULL DEFAULT '',
  sender TEXT NOT NULL DEFAULT '',
  decided_at TEXT NOT NULL
);

INSERT INTO message_outcomes_old (object_hash, outcome, source, job_id, sender, decided_at)
SELECT object_hash, outcome, source, job_id, sender, decided_at
FROM message_outcomes
WHERE outcome IN ('trashed_at_source', 'already_in_trash', 'kept')
  AND source IN ('delete_job', 'replica_folder', 'overview_keep');

DROP TABLE message_outcomes;
ALTER TABLE message_outcomes_old RENAME TO message_outcomes;
CREATE INDEX message_outcomes_outcome ON message_outcomes (outcome);
