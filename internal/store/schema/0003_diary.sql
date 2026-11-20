-- schema v3: diary entries.
--
-- Column for column the old project's diary_entry table, so the importer can copy
-- a ledger's diary without translation. One entry per ledger per local date: the
-- unique index is what makes saving an existing day an update (the upsert keys on
-- it). Content is plain text; mood is one emoji (old rows may hold a Chinese mood
-- name, which the domain normalizes on read).
CREATE TABLE IF NOT EXISTS diary_entry (
  id TEXT PRIMARY KEY,
  date TEXT NOT NULL,
  content TEXT,
  word_count INTEGER NOT NULL DEFAULT 0,
  mood TEXT DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  ledger_id TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_diary_entry_ledger_date ON diary_entry(ledger_id, date);
