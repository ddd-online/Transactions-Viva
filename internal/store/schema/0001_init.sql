-- schema v1 for Transactions 0.1.0.
--
-- This is the application's only schema version: every feature the 0.1.0 build
-- ships is created here in one pass, so a fresh workspace needs no migration
-- steps. The table shapes stay column-for-column compatible with the old
-- Electron project's accounting schema, which is what internal/store/import_old.go
-- copies from; the old project's database is never opened directly.
--
-- Units follow the domain contract:
--   amounts = integer cents (columns ending _cents)
--   prices  = integer milli-yuan (columns ending _milli)
--   times   = Unix seconds (columns ending _at)
--   dates   = local "YYYY-MM-DD" (columns ending _date)
--
-- No foreign keys: old workspaces may contain orphan rows, and deletes are
-- explicit transactions in the store layer.

-- table app_meta
CREATE TABLE app_meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

-- table ledger
CREATE TABLE ledger (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- table transaction_record
CREATE TABLE transaction_record (
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

-- index idx_tr_key_event_date
CREATE INDEX idx_tr_key_event_date ON transaction_record(key_event_date);

-- index idx_tr_ledger_at
CREATE INDEX idx_tr_ledger_at ON transaction_record(ledger_id, transaction_at);

-- table transaction_record_tag
CREATE TABLE transaction_record_tag (
  ledger_id TEXT NOT NULL,
  transaction_id TEXT NOT NULL,
  tag TEXT NOT NULL,
  PRIMARY KEY (ledger_id, transaction_id, tag)
);

-- index idx_tr_tag_transaction
CREATE INDEX idx_tr_tag_transaction ON transaction_record_tag(transaction_id);

-- index idx_tr_tag_ledger
CREATE INDEX idx_tr_tag_ledger ON transaction_record_tag(ledger_id);

-- table category
CREATE TABLE category (
  ledger_id TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  transaction_type TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);

-- index idx_category_ledger_name_type
CREATE UNIQUE INDEX idx_category_ledger_name_type
  ON category(ledger_id, name, transaction_type);

-- table tag
CREATE TABLE tag (
  ledger_id TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  category_transaction_type TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);

-- index idx_tag_ledger_name_cattype
CREATE UNIQUE INDEX idx_tag_ledger_name_cattype
  ON tag(ledger_id, name, category_transaction_type);

-- table transaction_template
CREATE TABLE transaction_template (
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

-- table chart
CREATE TABLE chart (
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

-- index idx_chart_ledger
CREATE INDEX idx_chart_ledger ON chart(ledger_id);

-- table diary_entry
CREATE TABLE diary_entry (
  id TEXT PRIMARY KEY,
  date TEXT NOT NULL,
  content TEXT,
  word_count INTEGER NOT NULL DEFAULT 0,
  mood TEXT DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  book_id TEXT NOT NULL DEFAULT '');

-- table diary_book
CREATE TABLE diary_book (
  book_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- index idx_diary_entry_book_date
CREATE UNIQUE INDEX idx_diary_entry_book_date ON diary_entry(book_id, date);

-- table key_event
CREATE TABLE key_event (
  id TEXT PRIMARY KEY,
  date TEXT NOT NULL,
  title TEXT,
  content TEXT,
  color TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- index idx_key_event_date
CREATE UNIQUE INDEX idx_key_event_date ON key_event(date);

-- table key_event_image
CREATE TABLE key_event_image (
  id TEXT PRIMARY KEY,
  event_date TEXT NOT NULL,
  file_path TEXT NOT NULL DEFAULT '',
  thumb_path TEXT NOT NULL DEFAULT '',
  original_path TEXT NOT NULL DEFAULT '',
  motion_path TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL
);

-- index idx_key_event_image_date
CREATE INDEX idx_key_event_image_date ON key_event_image(event_date);

-- table stock_account
CREATE TABLE stock_account (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  principal_cents INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- index idx_stock_account_name
CREATE UNIQUE INDEX idx_stock_account_name ON stock_account(name);

-- index idx_stock_account_order
CREATE INDEX idx_stock_account_order ON stock_account(sort_order, created_at);

-- table stock_fee_setting
CREATE TABLE stock_fee_setting (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  commission_rate REAL NOT NULL DEFAULT 0.0002354,
  min_commission_cents INTEGER NOT NULL DEFAULT 500,
  stamp_duty_rate REAL NOT NULL DEFAULT 0.0005,
  transfer_fee_rate REAL NOT NULL DEFAULT 0.00001,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- index idx_stock_fee_setting_account
CREATE UNIQUE INDEX idx_stock_fee_setting_account ON stock_fee_setting(account_id);

-- table stock_fund_record
CREATE TABLE stock_fund_record (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  record_date TEXT NOT NULL,
  event_type TEXT NOT NULL DEFAULT '',
  event_text TEXT NOT NULL DEFAULT '',
  amount_change_cents INTEGER NOT NULL DEFAULT 0,
  cash_balance_cents INTEGER NOT NULL DEFAULT 0,
  net_pnl_cents INTEGER,
  remark TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);

-- index idx_stock_fund_account_date
CREATE INDEX idx_stock_fund_account_date ON stock_fund_record(account_id, record_date);

-- table stock_position
CREATE TABLE stock_position (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  stock_code TEXT NOT NULL,
  stock_name TEXT NOT NULL DEFAULT '',
  quantity INTEGER NOT NULL DEFAULT 0,
  total_cost_cents INTEGER NOT NULL DEFAULT 0,
  realized_pnl_cents INTEGER NOT NULL DEFAULT 0,
  review TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  tag TEXT NOT NULL DEFAULT ''
);

-- index idx_stock_position_account_code
CREATE UNIQUE INDEX idx_stock_position_account_code ON stock_position(account_id, stock_code);

-- table stock_trade
CREATE TABLE stock_trade (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  stock_code TEXT NOT NULL,
  stock_name TEXT NOT NULL DEFAULT '',
  trade_type TEXT NOT NULL DEFAULT '',
  round_id TEXT NOT NULL DEFAULT '',
  order_id TEXT NOT NULL DEFAULT '',
  order_seq INTEGER NOT NULL DEFAULT 1,
  price_milli INTEGER NOT NULL DEFAULT 0,
  lots INTEGER NOT NULL DEFAULT 0,
  shares INTEGER NOT NULL DEFAULT 0,
  amount_cents INTEGER NOT NULL DEFAULT 0,
  fee_cents INTEGER NOT NULL DEFAULT 0,
  commission_cents INTEGER NOT NULL DEFAULT 0,
  stamp_duty_cents INTEGER NOT NULL DEFAULT 0,
  transfer_fee_cents INTEGER NOT NULL DEFAULT 0,
  realized_pnl_cents INTEGER,
  trade_time INTEGER NOT NULL DEFAULT 0,
  remark TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  round_commission_rate REAL,
  round_min_commission_cents INTEGER,
  round_stamp_duty_rate REAL,
  round_transfer_fee_rate REAL
);

-- index idx_stock_trade_order
CREATE INDEX idx_stock_trade_order ON stock_trade(order_id);

-- index idx_stock_trade_round
CREATE INDEX idx_stock_trade_round ON stock_trade(round_id);

-- index idx_stock_trade_account_code
CREATE INDEX idx_stock_trade_account_code ON stock_trade(account_id, stock_code);

-- index idx_stock_trade_account_time
CREATE INDEX idx_stock_trade_account_time ON stock_trade(account_id, trade_time);

-- table stock_trade_history
CREATE TABLE stock_trade_history (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  stock_code TEXT NOT NULL,
  stock_name TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- index idx_stock_trade_history_account_code
CREATE UNIQUE INDEX idx_stock_trade_history_account_code
  ON stock_trade_history(account_id, stock_code);

-- table stock_trade_round
CREATE TABLE stock_trade_round (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  stock_code TEXT NOT NULL,
  history_id TEXT NOT NULL,
  round_no INTEGER NOT NULL,
  opened_at INTEGER NOT NULL DEFAULT 0,
  closed_at INTEGER NOT NULL DEFAULT 0,
  tag TEXT NOT NULL DEFAULT '分析',
  review TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);

-- index idx_stock_trade_round_history_no
CREATE UNIQUE INDEX idx_stock_trade_round_history_no
  ON stock_trade_round(history_id, round_no);

-- index idx_stock_trade_round_account_code
CREATE INDEX idx_stock_trade_round_account_code
  ON stock_trade_round(account_id, stock_code);

-- table stock_trade_tag_setting
CREATE TABLE stock_trade_tag_setting (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  tags TEXT NOT NULL DEFAULT '[]',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- index idx_stock_trade_tag_setting_account
CREATE UNIQUE INDEX idx_stock_trade_tag_setting_account
  ON stock_trade_tag_setting(account_id);

-- table stock_operation
CREATE TABLE stock_operation (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL DEFAULT '',
  detail TEXT NOT NULL DEFAULT '',
  target_id TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);

-- index idx_stock_operation_account
CREATE INDEX idx_stock_operation_account
  ON stock_operation(account_id, created_at);

-- table stock_review_setting
CREATE TABLE stock_review_setting (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  template TEXT NOT NULL DEFAULT '',
  daily_template TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- index idx_stock_review_setting_account
CREATE UNIQUE INDEX idx_stock_review_setting_account
  ON stock_review_setting(account_id);

-- table stock_day_review
CREATE TABLE stock_day_review (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  stock_code TEXT NOT NULL,
  round_id TEXT NOT NULL DEFAULT '',
  review_date TEXT NOT NULL,
  review TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- index idx_stock_day_review_round_date
CREATE UNIQUE INDEX idx_stock_day_review_round_date
  ON stock_day_review(account_id, stock_code, round_id, review_date);

-- table stock_lesson
CREATE TABLE stock_lesson (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  text TEXT NOT NULL,
  round_id TEXT,
  stock_code TEXT NOT NULL DEFAULT '',
  stock_name TEXT NOT NULL DEFAULT '',
  round_no INTEGER NOT NULL DEFAULT 0,
  opened_at INTEGER NOT NULL DEFAULT 0,
  closed_at INTEGER NOT NULL DEFAULT 0,
  pnl_cents INTEGER NOT NULL DEFAULT 0,
  pnl_rate REAL NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- index idx_stock_lesson_account
CREATE INDEX idx_stock_lesson_account ON stock_lesson(account_id, created_at);

-- index idx_stock_lesson_round
CREATE INDEX idx_stock_lesson_round ON stock_lesson(round_id);

-- table stock_ledger
CREATE TABLE stock_ledger (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  stock_code TEXT NOT NULL DEFAULT '',
  round_id TEXT,
  why_buy TEXT NOT NULL DEFAULT '',
  stop_loss TEXT NOT NULL DEFAULT '',
  take_profit TEXT NOT NULL DEFAULT '',
  followed_system TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- index idx_stock_ledger_open
CREATE UNIQUE INDEX idx_stock_ledger_open
  ON stock_ledger(account_id, stock_code) WHERE round_id IS NULL;

-- index idx_stock_ledger_round
CREATE UNIQUE INDEX idx_stock_ledger_round
  ON stock_ledger(account_id, round_id) WHERE round_id IS NOT NULL;

-- table stock_limit_day
CREATE TABLE stock_limit_day (
  trade_date TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  count INTEGER NOT NULL DEFAULT 0,
  fetched_at INTEGER NOT NULL
);

-- table stock_limit_pool
CREATE TABLE stock_limit_pool (
  trade_date TEXT NOT NULL,
  stock_code TEXT NOT NULL,
  stock_name TEXT NOT NULL DEFAULT '',
  market INTEGER NOT NULL DEFAULT 0,
  price_milli INTEGER NOT NULL DEFAULT 0,
  pct REAL NOT NULL DEFAULT 0,
  amount_cents INTEGER NOT NULL DEFAULT 0,
  float_mv_cents INTEGER NOT NULL DEFAULT 0,
  turnover REAL NOT NULL DEFAULT 0,
  level INTEGER NOT NULL DEFAULT 1,
  first_seal INTEGER NOT NULL DEFAULT 0,
  last_seal INTEGER NOT NULL DEFAULT 0,
  broken_count INTEGER NOT NULL DEFAULT 0,
  seal_fund_cents INTEGER NOT NULL DEFAULT 0,
  industry TEXT NOT NULL DEFAULT '',
  stat_days INTEGER NOT NULL DEFAULT 0,
  stat_count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (trade_date, stock_code)
);

-- index idx_stock_limit_pool_date
CREATE INDEX idx_stock_limit_pool_date ON stock_limit_pool(trade_date);

-- table stock_limit_quote
CREATE TABLE stock_limit_quote (
  trade_date TEXT NOT NULL,
  stock_code TEXT NOT NULL,
  open_pct REAL,
  close_pct REAL,
  close_price_milli INTEGER,
  fetched_at INTEGER NOT NULL,
  PRIMARY KEY (trade_date, stock_code)
);

-- table todo_card
CREATE TABLE todo_card (
  id TEXT PRIMARY KEY,
  ledger_id TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);

-- index idx_todo_card_ledger
CREATE INDEX idx_todo_card_ledger ON todo_card(ledger_id, sort_order);

-- table todo_item
CREATE TABLE todo_item (
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

-- index idx_todo_item_card
CREATE INDEX idx_todo_item_card ON todo_item(ledger_id, card_id, status);

-- index idx_todo_item_status
CREATE INDEX idx_todo_item_status ON todo_item(ledger_id, status, completed_at);

-- table todo_progress
CREATE TABLE todo_progress (
  id TEXT PRIMARY KEY,
  ledger_id TEXT NOT NULL DEFAULT '',
  item_id TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  done INTEGER NOT NULL DEFAULT 0
);

-- index idx_todo_progress_item
CREATE INDEX idx_todo_progress_item ON todo_progress(ledger_id, item_id, created_at);

-- Identity key: the import tool uses it to tell a current workspace from the
-- old Electron database.
INSERT OR IGNORE INTO app_meta (key, value) VALUES ('app_id', 'transactions-viva');

-- Every workspace opens with one diary book; the diary page no longer depends
-- on the ledger list.
INSERT INTO diary_book (book_id, name, description, sort_order, created_at, updated_at)
SELECT 'book-default', '默认日记本', '', 0,
       CAST(strftime('%s', 'now') AS INTEGER), CAST(strftime('%s', 'now') AS INTEGER)
WHERE NOT EXISTS (SELECT 1 FROM diary_book);
