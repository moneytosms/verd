CREATE TABLE contests (
  id         INTEGER PRIMARY KEY,
  name       TEXT    NOT NULL,
  phase      TEXT    NOT NULL,
  start_time INTEGER NOT NULL DEFAULT 0,
  duration   INTEGER NOT NULL DEFAULT 0
);
