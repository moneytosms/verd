CREATE TABLE rating_changes (
  contest_id INTEGER PRIMARY KEY,
  old_rating INTEGER NOT NULL,
  new_rating INTEGER NOT NULL,
  at         INTEGER NOT NULL
);
