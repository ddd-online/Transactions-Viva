-- 待办：卡片（主题）/ 事项 / 进度记录。All three are scoped by ledger_id,
-- matching the old application's tables and import.
CREATE TABLE IF NOT EXISTS todo_card (
  id TEXT PRIMARY KEY,
  ledger_id TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_todo_card_ledger ON todo_card(ledger_id, sort_order);

CREATE TABLE IF NOT EXISTS todo_item (
  id TEXT PRIMARY KEY,
  ledger_id TEXT NOT NULL DEFAULT '',
  card_id TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  start_date TEXT NOT NULL DEFAULT '',
  due_date TEXT NOT NULL DEFAULT '',
  urgency INTEGER NOT NULL DEFAULT 0,
  importance INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'doing',
  completed_at INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_todo_item_card ON todo_item(ledger_id, card_id, status);
CREATE INDEX IF NOT EXISTS idx_todo_item_status ON todo_item(ledger_id, status, completed_at);

CREATE TABLE IF NOT EXISTS todo_progress (
  id TEXT PRIMARY KEY,
  ledger_id TEXT NOT NULL DEFAULT '',
  item_id TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  done INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_todo_progress_item ON todo_progress(ledger_id, item_id, created_at);
