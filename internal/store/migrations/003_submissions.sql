CREATE TABLE submissions (
  id           INTEGER PRIMARY KEY,
  contest_id   INTEGER NOT NULL,
  idx          TEXT    NOT NULL,
  language     TEXT    NOT NULL,
  verdict      TEXT    NOT NULL DEFAULT '',
  passed_tests INTEGER NOT NULL DEFAULT 0,
  time_ms      INTEGER NOT NULL DEFAULT 0,
  memory_bytes INTEGER NOT NULL DEFAULT 0,
  created_at   INTEGER NOT NULL
);
CREATE INDEX submissions_problem ON submissions(contest_id, idx);
