-- schema v5: key events and their image assets.
--
-- Events belong to the workspace, not to a ledger: the old application stored
-- one event per (ledger, date) and the page switched with the ledger, but a
-- ledger was never what an event was about. One workspace therefore has one
-- set of events and one event per local date.
--
-- The columns follow the old project's key_event and key_event_image tables
-- (including original_path / motion_path, which the old project added in its
-- migration 6) minus ledger_id, so the importer only has to merge by date.
-- Image paths stay relative to <workspace>/data/assets.
--
-- No foreign keys: deletes are explicit transactions in the store layer, as in
-- every other table here.

CREATE TABLE IF NOT EXISTS key_event (
  id TEXT PRIMARY KEY,
  date TEXT NOT NULL,
  title TEXT,
  content TEXT,
  color TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_key_event_date ON key_event(date);

CREATE TABLE IF NOT EXISTS key_event_image (
  id TEXT PRIMARY KEY,
  event_date TEXT NOT NULL,
  file_path TEXT NOT NULL DEFAULT '',
  thumb_path TEXT NOT NULL DEFAULT '',
  original_path TEXT NOT NULL DEFAULT '',
  motion_path TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_key_event_image_date ON key_event_image(event_date);
