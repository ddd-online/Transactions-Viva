-- schema v2: saved analysis charts.
--
-- The columns are identical to the old project's chart table so the importer can
-- copy a ledger's saved charts without translation. chart_lines is the old JSON
-- array of line configurations; pie charts keep an empty array and describe
-- their composition in group_field / group_transaction_type / group_category.
CREATE TABLE IF NOT EXISTS chart (
  chart_id TEXT PRIMARY KEY,
  ledger_id TEXT NOT NULL,
  title TEXT NOT NULL,
  granularity TEXT NOT NULL,
  chart_lines TEXT NOT NULL,
  chart_type TEXT NOT NULL DEFAULT 'line',
  group_field TEXT NOT NULL DEFAULT '',
  group_transaction_type TEXT NOT NULL DEFAULT '',
  group_category TEXT NOT NULL DEFAULT '',
  is_preset INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chart_ledger ON chart(ledger_id);
