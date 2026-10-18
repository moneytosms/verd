CREATE TABLE contest_materials (
  contest_id INTEGER PRIMARY KEY,
  data       TEXT    NOT NULL,
  fetched_at INTEGER NOT NULL
);
CREATE TABLE blog_entries (
  id         INTEGER PRIMARY KEY,
  title      TEXT    NOT NULL,
  locale     TEXT    NOT NULL,
  content    TEXT    NOT NULL,
  fetched_at INTEGER NOT NULL
);
