CREATE TABLE problems (
  contest_id   INTEGER NOT NULL,
  idx          TEXT    NOT NULL,
  name         TEXT    NOT NULL,
  rating       INTEGER NOT NULL DEFAULT 0,
  tags         TEXT    NOT NULL DEFAULT '[]',
  solved_count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (contest_id, idx)
);
CREATE TABLE meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
