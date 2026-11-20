-- schema v1 for the new application.
--
-- The five accounting tables (and the record-tag relation) are column-for-column
-- compatible with the old project's electron/db/schema/0001_init.sql, so the
-- old-data importer can copy rows without translating them. The old project's
-- stock, chart, event, diary and todo tables are deliberately not created: the
-- new application only needs ledgers, records, categories, tags and templates.
--
-- Units follow the domain contract:
--   amounts = integer cents (columns ending _cents)
--   times   = Unix seconds (columns ending _at)
--   dates   = local "YYYY-MM-DD" (columns ending _date)
--
-- No foreign keys: old workspaces may contain orphan rows, and v1 keeps the old
-- table shapes exactly. Deletes are explicit transactions in the store layer.

CREATE TABLE IF NOT EXISTS app_meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS ledger (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS transaction_record (
  transaction_id TEXT PRIMARY KEY,
  ledger_id TEXT NOT NULL,
  amount_cents INTEGER NOT NULL,
  transaction_type TEXT NOT NULL,
  category TEXT NOT NULL,
  description TEXT,
  flags TEXT,
  key_event_date TEXT NOT NULL DEFAULT '',
  transaction_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tr_key_event_date ON transaction_record(key_event_date);
CREATE INDEX IF NOT EXISTS idx_tr_ledger_at ON transaction_record(ledger_id, transaction_at);

CREATE TABLE IF NOT EXISTS transaction_record_tag (
  ledger_id TEXT NOT NULL,
  transaction_id TEXT NOT NULL,
  tag TEXT NOT NULL,
  PRIMARY KEY (ledger_id, transaction_id, tag)
);
CREATE INDEX IF NOT EXISTS idx_tr_tag_transaction ON transaction_record_tag(transaction_id);
CREATE INDEX IF NOT EXISTS idx_tr_tag_ledger ON transaction_record_tag(ledger_id);

CREATE TABLE IF NOT EXISTS category (
  ledger_id TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  transaction_type TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_category_ledger_name_type
  ON category(ledger_id, name, transaction_type);

CREATE TABLE IF NOT EXISTS tag (
  ledger_id TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  category_transaction_type TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tag_ledger_name_cattype
  ON tag(ledger_id, name, category_transaction_type);

CREATE TABLE IF NOT EXISTS transaction_template (
  template_id TEXT PRIMARY KEY,
  ledger_id TEXT NOT NULL,
  template_name TEXT NOT NULL,
  transaction_type TEXT NOT NULL,
  category TEXT NOT NULL,
  tags TEXT,
  flags TEXT,
  description TEXT,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
