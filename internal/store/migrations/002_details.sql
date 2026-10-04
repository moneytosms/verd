CREATE TABLE problem_details (
  contest_id      INTEGER NOT NULL,
  idx             TEXT    NOT NULL,
  statement       TEXT    NOT NULL,
  time_limit_ms   INTEGER NOT NULL,
  memory_limit_mb INTEGER NOT NULL,
  interactive     INTEGER NOT NULL,
  hint            TEXT    NOT NULL DEFAULT '',
  fetched_at      INTEGER NOT NULL,
  PRIMARY KEY (contest_id, idx)
);
CREATE TABLE sample_tests (
  contest_id INTEGER NOT NULL,
  idx        TEXT    NOT NULL,
  n          INTEGER NOT NULL,
  input      TEXT    NOT NULL,
  output     TEXT    NOT NULL,
  PRIMARY KEY (contest_id, idx, n)
);
