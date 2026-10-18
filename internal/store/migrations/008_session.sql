CREATE TABLE session (
  id         INTEGER PRIMARY KEY CHECK (id = 1),
  contest_id INTEGER NOT NULL,
  name       TEXT    NOT NULL,
  mode       TEXT    NOT NULL,
  start_at   INTEGER NOT NULL,
  end_at     INTEGER NOT NULL
);
