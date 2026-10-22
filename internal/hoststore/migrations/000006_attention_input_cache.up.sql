CREATE TABLE attention_input_cache (
  input_hash TEXT NOT NULL,
  judge_model TEXT NOT NULL,
  prompt_hash TEXT NOT NULL,
  urgency REAL NOT NULL,
  needs_reply_noul REAL NOT NULL,
  involves_decision_noul REAL NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (input_hash, judge_model, prompt_hash)
);
