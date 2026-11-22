-- schema v6: 股票账户与股票数据。
--
-- 旧工程把股票挂在账本上（每本账一个 stock_account，所有股票表都带 ledger_id）。
-- 这里改成工作空间级的多账户：一个工作空间可以有多个股票账户，每只账户有自己的
-- 本金、资金链、持仓、成交、轮次与设置，账本的选择与股票无关。
--
-- 所有股票子表用 account_id；只有天梯的涨停池 / 行情快照是市场数据，按交易日存，
-- 不带账户键。金额是整数分（_cents），价格是整数厘（_milli），时间是 Unix 秒（_at），
-- 日期是本地 YYYY-MM-DD（_date）。没有外键：账户删除由 store 在一个事务里手工级联。
CREATE TABLE IF NOT EXISTS stock_account (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  principal_cents INTEGER NOT NULL DEFAULT 0,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_account_name ON stock_account(name);
CREATE INDEX IF NOT EXISTS idx_stock_account_order ON stock_account(sort_order, created_at);

CREATE TABLE IF NOT EXISTS stock_fee_setting (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  commission_rate REAL NOT NULL DEFAULT 0.0002354,
  min_commission_cents INTEGER NOT NULL DEFAULT 500,
  stamp_duty_rate REAL NOT NULL DEFAULT 0.0005,
  transfer_fee_rate REAL NOT NULL DEFAULT 0.00001,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_fee_setting_account ON stock_fee_setting(account_id);

CREATE TABLE IF NOT EXISTS stock_fund_record (
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
CREATE INDEX IF NOT EXISTS idx_stock_fund_account_date ON stock_fund_record(account_id, record_date);

CREATE TABLE IF NOT EXISTS stock_position (
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_position_account_code ON stock_position(account_id, stock_code);

CREATE TABLE IF NOT EXISTS stock_trade (
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
CREATE INDEX IF NOT EXISTS idx_stock_trade_order ON stock_trade(order_id);
CREATE INDEX IF NOT EXISTS idx_stock_trade_round ON stock_trade(round_id);
CREATE INDEX IF NOT EXISTS idx_stock_trade_account_code ON stock_trade(account_id, stock_code);
CREATE INDEX IF NOT EXISTS idx_stock_trade_account_time ON stock_trade(account_id, trade_time);

CREATE TABLE IF NOT EXISTS stock_trade_history (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  stock_code TEXT NOT NULL,
  stock_name TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_trade_history_account_code
  ON stock_trade_history(account_id, stock_code);

CREATE TABLE IF NOT EXISTS stock_trade_round (
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_trade_round_history_no
  ON stock_trade_round(history_id, round_no);
CREATE INDEX IF NOT EXISTS idx_stock_trade_round_account_code
  ON stock_trade_round(account_id, stock_code);

CREATE TABLE IF NOT EXISTS stock_trade_tag_setting (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  tags TEXT NOT NULL DEFAULT '[]',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_trade_tag_setting_account
  ON stock_trade_tag_setting(account_id);

CREATE TABLE IF NOT EXISTS stock_operation (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL DEFAULT '',
  detail TEXT NOT NULL DEFAULT '',
  target_id TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_stock_operation_account
  ON stock_operation(account_id, created_at);

CREATE TABLE IF NOT EXISTS stock_review_setting (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  template TEXT NOT NULL DEFAULT '',
  daily_template TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_review_setting_account
  ON stock_review_setting(account_id);

CREATE TABLE IF NOT EXISTS stock_day_review (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL,
  stock_code TEXT NOT NULL,
  round_id TEXT NOT NULL DEFAULT '',
  review_date TEXT NOT NULL,
  review TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_day_review_round_date
  ON stock_day_review(account_id, stock_code, round_id, review_date);

CREATE TABLE IF NOT EXISTS stock_lesson (
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
CREATE INDEX IF NOT EXISTS idx_stock_lesson_account ON stock_lesson(account_id, created_at);
CREATE INDEX IF NOT EXISTS idx_stock_lesson_round ON stock_lesson(round_id);

CREATE TABLE IF NOT EXISTS stock_ledger (
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
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_ledger_open
  ON stock_ledger(account_id, stock_code) WHERE round_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_stock_ledger_round
  ON stock_ledger(account_id, round_id) WHERE round_id IS NOT NULL;

-- 市场数据：按交易日存，工作空间级，不带 account_id。
CREATE TABLE IF NOT EXISTS stock_limit_day (
  trade_date TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  count INTEGER NOT NULL DEFAULT 0,
  fetched_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS stock_limit_pool (
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
CREATE INDEX IF NOT EXISTS idx_stock_limit_pool_date ON stock_limit_pool(trade_date);

CREATE TABLE IF NOT EXISTS stock_limit_quote (
  trade_date TEXT NOT NULL,
  stock_code TEXT NOT NULL,
  open_pct REAL,
  close_pct REAL,
  close_price_milli INTEGER,
  fetched_at INTEGER NOT NULL,
  PRIMARY KEY (trade_date, stock_code)
);

-- 新库身份键：旧库没有它，旧库识别不再依赖表名。
INSERT INTO app_meta (key, value) VALUES ('app_id', 'transactions-viva')
  ON CONFLICT(key) DO UPDATE SET value = excluded.value;
