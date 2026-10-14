ALTER TABLE clusters DROP COLUMN attention_scored;
ALTER TABLE clusters DROP COLUMN involves_decision;
ALTER TABLE clusters DROP COLUMN needs_reply;
ALTER TABLE clusters DROP COLUMN urgency;

ALTER TABLE classify_state DROP COLUMN attention_error;
ALTER TABLE classify_state DROP COLUMN attention_scored;
ALTER TABLE classify_state DROP COLUMN involves_decision_noul;
ALTER TABLE classify_state DROP COLUMN needs_reply_noul;
ALTER TABLE classify_state DROP COLUMN urgency;
