CREATE TABLE problem_state (
  contest_id  INTEGER NOT NULL,
  idx         TEXT    NOT NULL,
  lang        TEXT    NOT NULL DEFAULT '',
  mode        TEXT    NOT NULL DEFAULT '',
  last_opened INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (contest_id, idx)
);
