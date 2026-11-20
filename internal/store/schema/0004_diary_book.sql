-- schema v4: diaries belong to a diary book, not to a ledger.
--
-- The old application kept one diary per ledger. This application keeps one
-- diary per diary book: a workspace has as many books as the writer wants, and
-- the ledger choice has nothing to do with them. The table keeps the entry shape
-- of v3 (plain-text content, one emoji mood, one entry per local date) and swaps
-- the ledger key for the book key.
--
-- The data migration keeps what v3 held: every ledger that had entries becomes a
-- diary book named after it, so a workspace that was writing two ledgers' diaries
-- still has two books rather than one merged one. A workspace with no entries at
-- all gets one 默认日记本. Book ids are derived from the ledger id here, once, so
-- the migration can run in SQL without asking the application for new ids.
CREATE TABLE IF NOT EXISTS diary_book (
  book_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

INSERT INTO diary_book (book_id, name, description, sort_order, created_at, updated_at)
SELECT 'book-' || d.ledger_id, COALESCE(NULLIF(l.name, ''), '默认日记本'), '', 0,
       CAST(strftime('%s', 'now') AS INTEGER), CAST(strftime('%s', 'now') AS INTEGER)
FROM (SELECT DISTINCT ledger_id FROM diary_entry WHERE ledger_id <> '') AS d
LEFT JOIN ledger AS l ON l.id = d.ledger_id
WHERE NOT EXISTS (SELECT 1 FROM diary_book);

INSERT INTO diary_book (book_id, name, description, sort_order, created_at, updated_at)
SELECT 'book-default', '默认日记本', '', 0,
       CAST(strftime('%s', 'now') AS INTEGER), CAST(strftime('%s', 'now') AS INTEGER)
WHERE NOT EXISTS (SELECT 1 FROM diary_book);

ALTER TABLE diary_entry ADD COLUMN book_id TEXT NOT NULL DEFAULT '';
UPDATE diary_entry SET book_id = 'book-' || ledger_id;
DROP INDEX IF EXISTS idx_diary_entry_ledger_date;
CREATE UNIQUE INDEX IF NOT EXISTS idx_diary_entry_book_date ON diary_entry(book_id, date);
ALTER TABLE diary_entry DROP COLUMN ledger_id;
