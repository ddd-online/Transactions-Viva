-- schema v2: 笔记.
--
-- 笔记是工作空间的一棵树：文件夹可以装文件夹和笔记，笔记永远是叶子，根级只放
-- 文件夹。一篇笔记有一个标题（存在节点行上）和一串从上到下的块。
--
-- 单位沿用 domain 契约：times = Unix 秒 (columns ending _at)。
-- 图片尺寸按原图像素记录，image_scale 是用户拖出的显示比例；显示宽度以正文
-- 宽度为上限，由页面换算，不写回数据库。

-- table note_node
CREATE TABLE note_node (
  id TEXT PRIMARY KEY,
  parent_id TEXT NOT NULL DEFAULT '',
  kind TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0,
  expanded INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- index idx_note_node_parent
CREATE INDEX idx_note_node_parent ON note_node(parent_id, sort_order);

-- table note_block
CREATE TABLE note_block (
  id TEXT PRIMARY KEY,
  note_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  level INTEGER NOT NULL DEFAULT 0,
  text TEXT NOT NULL DEFAULT '',
  image_path TEXT NOT NULL DEFAULT '',
  image_width INTEGER NOT NULL DEFAULT 0,
  image_height INTEGER NOT NULL DEFAULT 0,
  image_scale REAL NOT NULL DEFAULT 1,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- index idx_note_block_note
CREATE INDEX idx_note_block_note ON note_block(note_id, sort_order);
