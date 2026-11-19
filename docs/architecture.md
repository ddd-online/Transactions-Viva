# 架构

Transactions 的目标形态是 **Viva + Go 的原生桌面应用**：一个进程、一个事件循环，窗口和控件由
GPU 直接绘制，没有 Electron、Node 或内嵌浏览器。旧工程
`D:\github\Transactions`（Electron + React + TypeScript）是行为与数据的唯一权威，本仓库只负责
用 Go 重新表达它。

## 分层

```text
main.go                    入口：只做 log.Fatal(desktop.Run())
  └─ internal/desktop            外壳：窗口、标题栏、导航、底栏、设置页、主题
       ├─ internal/pages/*       功能页：accounting / stock / todo / events / diary / settings
       │    ├─ internal/components   跨页 UI 片段
       │    └─ internal/domain       纯领域规则与数据形状
       ├─ internal/config        应用配置形状与默认值
       └─ internal/store         SQLite 工作空间与 DAO（尚未实现）
```

依赖方向固定为：

```text
main → desktop → pages → components
                    ↘
                      domain ← store
```

- `domain` 不 import `ui`、`gfx`、`store` 或任何操作系统 / 数据库包；它是可以用普通 Go 单测
  直接验证的规则层。
- `store` 只依赖 `domain` 和 SQLite 驱动；页面不直接写 SQL。
- `pages` 只依赖 `ui`、`components`、`domain`，不依赖 `desktop`（避免外壳与页面互相引用）。
- `desktop` 是唯一知道窗口、标题栏和页面切换的地方；页面模型由它长期持有，切页不丢状态。
- `config` 不依赖 UI；设置页通过依赖注入拿到读写配置的能力（落地时补）。

## 包与旧工程对照

| 新包 / 文件 | 职责 | 旧工程来源 |
|---|---|---|
| `main.go` | 入口 | `src/main.tsx` + `electron/main.ts` 的启动部分 |
| `internal/desktop` | 窗口、标题栏、导航、底栏 | `src/App.tsx`、`src/app-shell.tsx`、`src/app-view.ts` |
| `internal/desktop/theme.go` | 主题选择：Viva 原生浅色 / 深色（旧配色暂不迁移） | `gfx.LightTheme` / `gfx.DarkTheme`；旧配色见 `DESIGN.md`、`src/theme` |
| `internal/pages/accounting` | 记录（mock 前端）/ 分析 / 标签 / 模板 | `src/pages/records*`、`charts*`、`category-tag*`、`template*` |
| `internal/pages/stock` | 账户 / 持仓 / 记录 / 心得 / 统计 / 天梯 / 设置 | `src/pages/stock*` |
| `internal/pages/todo` | 待办 / 历史 | `src/pages/todo*` |
| `internal/pages/events` | 事件画廊、图片、实况照片 | `src/pages/event*` |
| `internal/pages/diary` | 日记树、正文、导入导出 | `src/pages/diary*` |
| `internal/pages/settings` | 设置页五个分区 | `src/pages/settings*` |
| `internal/components` | 空态，以及设置页的分组卡片 / 设置行（后续的 page header、table cell 等） | `src/components` |
| `internal/domain` | 金额、模型、常量、股票费用、统计、图表规则 | `shared/domain` |
| `internal/config` | `AppConfig`、`FeatureFlags`、默认值 | `shared/types/config.ts`、`electron/config.ts` |
| `internal/store` | SQLite 打开 / 迁移 / DAO | `electron/db`、`electron/services` 的持久化部分 |

非 UI 的能力按职责拆到 `internal/<capability>`（对齐 motty 的做法），例如后续会出现的
`backup`、`update`、`stockquote`、`proxy`。不要把所有东西都塞进 `desktop`。

## 页面约定

- 外壳的中栏固定为「左侧 Sidebar + 右侧内容」：Sidebar 使用 Viva 的 `components.Sidebar`
  （204px，与 motty 的 `navigationWidth` 对齐；图标 + 文字，`应用设置` 固定在底部；选中状态由
  `desktop.Model.section` 持有）。页面只负责右侧内容，不再自己画导航。
- 右侧内容是 motty 式的工作区卡片：`RadiusSurface` 圆角、1px 边框、`theme.Background`
  底色，浮在 `theme.Muted` 的窗口壳上（标题栏、底栏、导航同壳）。卡片顶部贴标题栏、
  左侧贴导航，右侧与底栏上方各留 7px；Sidebar 自带的右分隔线用其自身底色覆盖，由卡片的
  圆角边框充当边界。
- 设置是一个普通分区页，不是弹窗：左栏是分区列表，右栏是分区标题 + 分组卡片；卡片里的
  「设置行」是标题 / 说明在左、控件在右的 64px 行。尺寸与分组方式对齐 motty 的设置页
  （208px 分区列、760px 阅读列、32px 页边距），进入设置时左侧主导航保持可见，选中状态由
  `desktop.Model.section` 持有。
- 记账-记录页是当前唯一有真实前端的页面：工具栏 / 类型筛选 / 时间范围 / 排序 / 表格 /
  收支统计都在 `internal/pages/accounting/records.go`，行数据来自 `records_mock.go`，
  `records_composer.go` 的「记一笔」只写内存。接入 store 时替换 mock，不改这套布局。
- 导航是两级：一级功能项展开自己的子功能，同时收起其他一级项；没有子功能的功能（事件、日记）
  就是单独一项。页面通过 `Tabs() []string`、`Tab() int`、`SelectTab(int)` 暴露子功能，
  内容里不画 `ui.Tabs`。
- 一个页面是一个持有自己状态的类型：`type Model struct{...}`、`New() *Model`、`View(c ui.C)`。
  状态由外壳长期持有；切换导航只是换 `View`，不重建模型。
- `View` 只读模型、不修改状态；点击在控件声明处改状态（Viva 的 immediate-mode 规则）。
- 子功能用 `ui.Tabs`，值存在页面模型里；页签内容按当前值声明，不预先创建全部页面。
- 页面不持有 `ui.C`、`ui.Box`、`gfx.Canvas`；不手算坐标；不启动 goroutine。耗时的读取用
  `ui.Effect`，结果通过 `Take` 在窗口线程落到模型。
- 真实窗口只用于人工验收（IME、焦点丢失、窗口管理器关闭、高分屏）。日常验证用
  `app.NewDriver`，见 `internal/desktop/desktop_test.go`。

## 存储设计（下一步）

旧工程的 schema 与迁移在 `electron/db/schema/` 和 `electron/db/migrate.ts`，**整体移植**，不要
只抄 `0001_init.sql`：`key_event_image` 的 `original_path` / `motion_path` 就在后续迁移里。

计划：

```text
internal/store/
  store.go        打开工作空间：连接串、PRAGMA、quick_check、关闭
  migrate.go      按 app_meta.schema_version 顺序执行迁移
  schema/         嵌入式 SQL（//go:embed）
  ledger.go       ledger / app_meta
  record.go       transaction_record / transaction_record_tag
  ...
```

- 驱动：`modernc.org/sqlite`（纯 Go，无 cgo；motty 也用它）。
- 打开时只做 `PRAGMA quick_check`；`integrity_check` 只属于备份 / 恢复两个显式动作。
- 迁移失败不静默建新库；库损坏时报告并引导「设置 → 数据备份 → 从备份恢复」。
- 写入用事务；金额、价格、时间、日期的单位见 `domain` 的包文档。

## 待记录的决策

第一次出现需要留档的取舍时，在 `docs/adr/` 下建一条 ADR（旧工程的 `docs/agents/domain.md`
对何时建 ADR 有说明）。目前尚未建立 ADR 目录。
