# 架构

Transactions 的目标形态是 **Viva + Go 的原生桌面应用**：一个进程、一个事件循环，窗口和控件由
GPU 直接绘制，没有 Electron、Node 或内嵌浏览器。旧工程
`D:\github\Transactions`（Electron + React + TypeScript）是行为与数据的唯一权威，本仓库只负责
用 Go 重新表达它。

## 分层

```text
main.go                    入口：只做 log.Fatal(desktop.Run())
  └─ internal/desktop            外壳：窗口、标题栏、导航、设置页、主题
       ├─ internal/pages/*       功能页：accounting / stock / todo / events / diary / settings
       │    ├─ internal/pages/*/controls   只属于该页面的组件
       │    ├─ internal/components/*      可复用的自定义组件（datepicker / pagination / selectbox）
       │    └─ internal/domain       纯领域规则与数据形状
       ├─ internal/repository   页面使用的数据契约（接口 / 查询 / 错误）
       ├─ internal/config       应用配置形状、默认值、读写与归一化
       └─ internal/store        SQLite 工作空间、schema / 迁移 / 仓储 / 旧库导入
```

依赖方向固定为：

```text
main → desktop → pages → components
                    ↘
              repository ← store
                    ↘
                      domain
```

- `domain` 不 import `ui`、`gfx`、`store` 或任何操作系统 / 数据库包；它是可以用普通 Go 单测
  直接验证的规则层。
- `repository` 只依赖 `domain`，是页面与数据层之间的接口；页面不直接写 SQL。
- `store` 只依赖 `domain`、`repository` 和 SQLite 驱动，实现工作空间与仓储。
- `pages` 只依赖 `ui`、`components`、`domain`、`repository`，不依赖 `desktop` 或 `store`
  （避免外壳与页面互相引用，也避免页面碰 SQLite）。
- `desktop` 是唯一知道窗口、标题栏和页面切换的地方；页面模型由它长期持有，切页不丢状态。
- `config` 不依赖 UI；设置页只通过 `Spec` 回调用外壳读写配置，自己不碰文件。

**自定义组件不设样式。** 自定义组件（`internal/components` 与各页面的 `controls` 子包）只用原生外观：`ui` 控件本来的样子、
`components` 的 recipe（`BeginButton` / `Calendar` / `PopupRecipe` / `Select` …）和
`c.Theme()` 的 `Metrics` / 颜色。不写颜色、圆角、阴影常量，也不给控件固定像素尺寸——
尺寸和留白从主题度量与原生组件的布局里来；需要在画布上画浮层时，同样只调用 recipe 的
`Paint` 或取它的度量。**文字与字重只有一个来源**：`internal/components/typography` 的
五个角色（12 / 14 / 16 / 20 / 24）。组件和页面都不写 `FontSize(数字)`——组件的
`typography_test.go` 会扫描 `internal/` 并让这类改动构建失败。

## 包与旧工程对照

| 新包 / 文件 | 职责 | 旧工程来源 |
|---|---|---|
| `main.go` | 入口 | `src/main.tsx` + `electron/main.ts` 的启动部分 |
| `internal/desktop` | 窗口、标题栏、导航、设置页 | `src/App.tsx`、`src/app-shell.tsx`、`src/app-view.ts` |
| `internal/desktop/theme.go` | 两套自建主题（Cool Graphite / Cool Ink）：全部 token、对比度契约与唯一改色点 | `gfx.LightTheme` / `gfx.DarkTheme` 的几何与 API；配色见 `DESIGN.md`，旧工程 `src/theme` |
| `internal/pages/accounting` | 记账分区的四个子功能（记录 / 分析 / 标签 / 模板） | `src/pages/records*`、`charts*`、`category-tag*`、`template*` |
| `internal/pages/accounting/records` | 记录页：工具栏、表格、分页、统计、空态、行操作 | `src/pages/records.tsx`、`records-table.tsx` |
| `internal/pages/accounting/records/composer` | 新增 / 编辑记录弹窗（草稿、标签多选、日期选择器） | `src/pages/record-composer.tsx` |
| `internal/pages/accounting/ledger` | 记账三页共享的账本列表与当前账本（含工具栏选择器） | `src/app-shell.tsx` 的账本菜单、`electron/services/ledger.ts` |
| `internal/pages/accounting/tags` | 标签页：账本选择、类型卡片、分类 / 标签两栏、新增 / 删除 / 拖拽排序 | `src/pages/category-tag.tsx`、`category-column.tsx`、`tag-column.tsx` |
| `internal/pages/accounting/template` | 模板缓存与 domain 互转：按账本存放，记录弹窗与模板页共用 | `electron/services/transaction-template.ts` |
| `internal/pages/accounting/templates` | 模板页：表格、新建 / 删除 / 拖拽排序 | `src/pages/templates.tsx`、`template-list.tsx`、`template-form.tsx` |
| `internal/pages/accounting/analysis` | 分析页：图表列表、折线 / 饼图、曲线条件、明细表与合计带 | `src/pages/charts.tsx`、`chart-*`、`antv-*-chart.tsx` |
| `internal/pages/stock` | 账户 / 持仓 / 记录 / 心得 / 统计 / 天梯 / 设置；账户是工作空间级、多账户，工具栏共享「账户」选择器 | `src/pages/stock*` |
| `internal/stockquote` | 腾讯行情（GBK、3 秒），`Fetcher` 接口，失败返回空 | `electron/services/stock-quote.ts` |
| `internal/stocklimit` | 东财涨停池（8 秒）与天梯缓存策略，`Service.Ladder` | `electron/services/stock-limit*.ts`、`shared/domain/limit-ladder.ts` |
| `internal/backup` | 备份命名 / 保留裁剪、VACUUM 快照、手动 / 自动备份、恢复前副本与备份列表 | `shared/domain/backup.ts`、`electron/services/backup.ts` |
| `internal/proxy` | 代理三态、手动地址归一化、环境变量 / Windows 系统代理探测与共享 HTTP client | `shared/domain/proxy.ts`、`electron/ipc/handlers/settings.ts` 的代理部分 |
| `internal/update` | GitHub release 检查、安装包下载 / SHA256 校验、进度回调与拉起安装程序 | `shared/domain/update.ts`、`electron/services/updater.ts` |
| `internal/pages/todo` | 待办：共享账本选择、主题看板 / 四象限、事项与进度、历史、旧库导入 | `src/pages/todo*`、`electron/services/todo.ts` |
| `internal/pages/events` | 事件页：年份列表、身份色、图片画廊与灯箱、跨账本关联交易（事件是工作空间级数据） | `src/pages/events*`、`event-*`、`electron/services/key-event.ts` |
| `internal/assets` | 工作空间图片资产：路径校验、落盘 / 删除 / 另存、解码、EXIF 方向与缩略图 | `electron/services/assets.ts`、`image-import.ts`、`image-resize.ts`、`heic-decode.ts` |
| `internal/events` | 事件图片的导入 / 删除编排：实况照片配对、逐张报告、清文件 | `electron/services/key-event.ts` 的 `addImage` / `deleteImage` |
| `internal/pages/diary` | 日记页：日记本、日期树、正文与心情、自动保存、删除 | `src/pages/diary.tsx`、`src/components/diary-tree.tsx`、`mood-picker.tsx`、`shared/domain/diary-tree.ts`（日记本是与旧工程不同的地方，见下） |
| `internal/diary` | 日记文件：递归扫描、编码回退链、写 `<date>.txt`（不依赖 UI，不直接碰 SQLite） | `electron/services/diary.ts` |
| `internal/pages/settings` | 设置页六个分区：通用设置 / 账本管理 / 股票设置 / 日记配置 / 数据备份 / 关于软件（通用设置页内再分「常规」与「功能开关」两组；股票账户与账户设置同在「股票设置」） | `src/pages/settings*`、`src/app-menu.tsx` 的「创建账本」 |
| `internal/components` | 可复用的片段：占位空态（后续的 page header、table cell 等） | `src/components` |
| `internal/components/datepicker` | 日期 / 时间范围选择器（工具栏与表单同一组件） | `src/components/time-range-picker.tsx`、antd DatePicker |
| `internal/components/pagination` | 分页栏：页大小 Select + Viva 分页 + 总数 | `src/components/pagination.tsx` |
| `internal/components/selectbox` | `ui.Select` 的包装，补上框架漏收的动画组 | — |
| `internal/buildinfo` | 版本、构建提交、运行平台与运行时；关于软件读它，`viva.toml` 的版本由测试对齐 | 旧工程 `settings-about.tsx` 的版本面板 |
| `internal/components/typography` | 字号 / 字重的五个角色（Caption 12 · Body 14 · Section 16 · Title 20 · Metric 24） | `src/styles/tokens.css` 的 `--transactions-size-text-*`，收掉 12/13/14 的 1px 档 |
| `internal/pages/<page>/controls` | 只属于某一个页面的组件：目前是设置页的滚动页 / 分组卡片 / 设置行 | 旧工程同页目录下的组件 |
| `internal/domain` | 金额、模型、常量、股票费用、统计、图表规则 | `shared/domain` |
| `internal/config` | `AppConfig`、功能开关、默认值、读写 / 归一化 / 未知键保留 | `shared/types/config.ts`、`electron/config.ts`、`electron/services/config.ts` |
| `internal/repository` | 页面数据契约：账本 / 分类标签 / 记录 / 模板的查询与写入 | `electron/ipc/handlers` 的命令边界 |
| `internal/store` | SQLite 工作空间、schema v1 / 迁移、账本 / 分类标签 / 记录 / 模板仓储、旧库导入 | `electron/db`、`electron/services` 的持久化部分 |
| `cmd/tr-import` | 旧工作空间导入命令 | `electron/services/*` 的导入 / 备份入口 |

非 UI 的能力按职责拆到 `internal/<capability>`（对齐 motty 的做法），例如
`backup`、`stockquote`、`stocklimit`、`diary`。不要把所有东西都塞进 `desktop`。

## SQLite 工作空间与数据层

- 工作空间是一个目录，库固定为 `transactions.db`。`store.Open` 要求目录已存在，打开后设置
  `journal_mode=WAL` / `synchronous=NORMAL` / `busy_timeout=5000` / `foreign_keys=OFF`，
  并保持单连接。已有库先跑 `PRAGMA quick_check`；新建 / 空库直接建表。打开序：目录 →
  `quick_check` → 识别 `app_meta` / 旧库 → 迁移。损坏、陌生、旧版（含股票表的 v12）与更高
  版本库都返回明确错误，不静默建空库。
- schema v1 建记账需要的表：`app_meta`、`ledger`、`transaction_record`、
  `transaction_record_tag`、`category`、`tag`、`transaction_template`；v2 增加 `chart`
  （图表配置、类型、分组、预设标记与排序）；v3 增加 `diary_entry`（最初每账本每天一篇）；
  v4 把日记从账本上解绑：新增 `diary_book`（`book_id` / 名称 / 描述 / 排序），`diary_entry`
  换成 `book_id`、`(book_id, date)` 唯一，`content` 为纯文本、`mood` 存 emoji。v5 增加 `key_event`
  与 `key_event_image`：**事件属于工作空间，不按账本隔离**，`key_event.date` 唯一，两张表都没有
  `ledger_id`，图片路径仍是相对 `data/assets`。列名 / 类型与旧库逐列一致（金额 `_cents`、时间
  `_at`、日期文本、无外键），便于导入工具直接复制；v6 增加股票表；v7 增加 `todo_card` /
  `todo_item` / `todo_progress`，三张表都带 `ledger_id`，状态 `doing` / `done`，完成不搬行。
  v4 的迁移把 v3 里每个「有日记的账本」变成一本同名日记本（`book_id = 'book-' || ledger_id`，
  纯 SQL 就能跑），没有日记的工作空间得到一本「默认日记本」；写库时 `SaveDiary` 自己检查日记本
  是否存在——schema 没有外键，这条规则写在仓储里。v6 增加股票账户与股票数据：
  `stock_account`（名称唯一、描述、本金、排序）与 12 张 `account_id` 子表（费用 / 资金 / 持仓 /
  成交 / 历史 / 轮次 / 标签 / 复盘 / 操作 / 每日复盘 / 心得 / 台账），市场数据
  `stock_limit_day / pool / quote` 不带账户；同时写 `app_meta('app_id','transactions-viva')`。
  v7 的待办三表按 `ledger_id` 隔离：卡片有 `sort_order`，事项存六档紧急 / 重要度与
  `completed_at`，进度行存 `done`；完成只改 `status` / `completed_at`，不搬数据。
  旧库识别改用身份键与旧形状：有 `app_id` 或新 `stock_account`（无 `ledger_id`）不算旧库；
  只有旧形状的 `stock_account`（带 `ledger_id`）或 `todo_item` 才算。`diary_entry` / `key_event`
  从 v3 / v5 起已经是本工程自己的表，不能再用它们判断「这是旧库」。
- 版本：`app_meta.schema_version` 从 1 连续递增，迁移清单在 `internal/store/migrate.go`，
  SQL 内嵌在 `internal/store/schema/`。升级已有库前 `VACUUM INTO <db>.bak`，失败不升级；
  每条迁移与版本更新同事务。新增结构时只追加迁移，不改已发布的 SQL。
- `internal/repository` 是页面看到的数据契约：账本、`Labels` 快照（分类 + 标签 + 计数）、
  记录（按账本 + 本地日期范围）、模板、日记本与日记、事件与事件图片、待办，以及各自的写入 / 删除 /
  排序。待办另有一条 `TodoRepository`：卡片 / 事项 / 进度都按账本隔离，删卡片或事项时同一事务清掉
  子行。事件另有一条 `KeyEventRecords(date)`：工作空间里 `key_event_date` 等于该日期的记录，
  跨账本、带标签，事件页的右栏用它。页面只调用接口，不写 SQL；`internal/store` 用编译期断言
  实现该接口。记录写入与标签关联、删除与关联清理、分类删除与其标签、标签删除与记录关联、
  删事件与它的图片行都在单事务里；删账本只删账本自己的数据，不碰工作空间级的事件。
- 新建账本在同一个事务里写入旧工程的 19 个分类 / 57 个标签。空工作空间由
  `accounting.SetRepository` 创建「默认账本」；导入的旧库已有账本时不重复播种。
- 记录范围查询把本地 `YYYY-MM-DD` 两端换算成当天 00:00:00 / 23:59:59（含结束日），SQL 只
  做账本 + 时间过滤 + 多列排序并批量取标签；统计 / 分页留在页面内存。记录编辑用 UPDATE 保留 ID
  （旧工程是新建后删旧），同步到其他账本复制一条新记录并把 `key_event_date` 置空。
- 旧库导入：`internal/store/import_old.go` 用只读连接打开旧 `transactions.db`，校验六张表，
  `-strict` 检查孤儿行，然后在新目录创建当前 schema 并单事务复制
  `ledger / category / tag / transaction_record / transaction_record_tag /
  transaction_template`，旧库有 `chart` / `key_event` / 股票表 / `todo_card` 等表时也复制；
  保留 ID、时间戳、排序、JSON 标签与可空字段。事件按日期合并，待办的 11 档级别就近归到六档。
  CLI 是 `cmd/tr-import`；旧库不直接由应用打开。

## 页面约定

- 外壳的中栏固定为「左侧 Sidebar + 右侧内容」：Sidebar 是 `internal/desktop/sidebar.go` 里
  用 `ui` 声明的导航列（204px，与 motty 的 `navigationWidth` 对齐；图标 + 文字，`设置`
  固定在底部；选中状态由 `desktop.Model.section` 持有）。一级项展开自己的子功能时，子功能行
  只缩进前缘，底色右边缘与一级项对齐（理由见「框架缺口」）。页面只负责右侧内容，
  不再自己画导航。
- 右侧内容是 motty 式的工作区卡片：`RadiusSurface` 圆角、1px 边框、`theme.Background`
  底色，浮在 `theme.Muted` 的窗口壳上（标题栏、导航同壳）。卡片顶部贴标题栏、
  左侧贴导航，右侧与下方各留 7px；导航自身不画右分隔线，由卡片的圆角边框充当边界。
- 设置是一个普通分区页，不是弹窗：左栏是分区列表，右栏是分区标题 + 分组卡片；卡片里的
  「设置行」是标题 / 说明在左、控件在右的 64px 行。尺寸与分组方式对齐 motty 的设置页
  （208px 分区列、760px 阅读列、32px 页边距），进入设置时左侧主导航保持可见，选中状态由
  `desktop.Model.section` 持有。
- 配置与设置：`internal/config` 读写 `~/.transactions.json`（`viva dev` 期间是
  `~/.transactions.dev.json`）。读取时每个字段独立回退（缺字段、类型不对、文件损坏都只影响
  那一项），写回把已知字段合并进文件原有的顶层对象再写 `.tmp` + rename，所以升级 / 降级
  不会抹掉未知键。窗口关闭时把尺寸 / 位置写回 `width` / `height` / `x` / `y`，下次启动
  按它开窗。设置页的「通用设置」是工作空间（`app.OpenDirectory`：先 `store.Open` 新库，
  成功后才关闭旧库并写入 `workspaceDir` 和最近 5 条 `recentWorkspaces`；失败保留旧工作空间）、
  外观（浅色 / 深色 / 跟随系统；标题栏开关写同一个值，`app.WatchSystemAppearance` 让跟随
  系统时会重绘）、关闭行为（`OnCloseRequest`：直接退出 / 隐藏到托盘 / 弹窗询问；托盘用
  Viva 的 `StatusItem`，没有状态区时回退为退出）。同一页的「网络代理」是不使用 / 自动探测 /
  手动三态：自动探测先读环境变量（`ALL_PROXY` → `HTTPS_PROXY` → `HTTP_PROXY`，含小写与
  `NO_PROXY`），再读 Windows Internet Options 的静态代理（不解析 PAC）；手动只接受
  `http://host:port`，由 `internal/proxy` 归一化并给出文案；保存后立即重建行情与涨停池的
  HTTP client，「检测」通过同一 client 请求一次并显示结果。之后是
  「功能开关」（不再单独占一个导航小节），写 `features` 的全局默认；`ledgerFeatures` 的账本
  覆盖规则已在 `config` 里，但还没有接到真实账本上。关掉的分区立即从 Sidebar 消失，五个全关时落到设置。
- 关于软件是成立的页面（对齐 motty 的关于页）：身份卡（标志、名称、一句话定位、「复制版本信息」）→「版本信息」2×2 事实网格 →「更新」→「项目」两张外链卡 →「界面由 Viva 绘制」尾注。事实来自 `internal/buildinfo`（`debug.ReadBuildInfo` 的提交与日期 + `runtime`，一次读取）；「复制」与「打开链接」走 `Spec` 回调外壳（`app.SetClipboardText` / `app.OpenURL`，在驱动下记录而不碰真实系统）。
- 设置 ·「更新」由 `internal/update` 负责：检查 `github.com/ddd-online/Transactions-Viva` 的 latest release（走同一份代理配置），只认非 prerelease 且带 `.exe` asset 的版本；release body 在「查看更新内容」弹窗里按段落 / 标题渲染。下载到系统临时目录，边下边报进度（页面用 `ui.Progress`），完成后按 asset 的 SHA256 校验；「开始更新」用独立进程拉起安装程序，600ms 后调用 `Application.Quit` 让窗口、数据库和托盘按正常退出路径关闭。
- 账本管理是设置自己的一个分区（排在通用设置之后）：列出工作空间的账本（名称 + 描述），
  可以新增、编辑（名称与描述一次写入）与删除，标出记账页正在读的那一本。三命令落在
  `accounting.Model`（`CreateLedger` / `UpdateLedger` / `DeleteLedger`），因为它持有四个记账页
  共享的 `ledger.Store`：写入后重读列表并重置 store，新建的账本成为当前账本（与旧工程
  `ledger_create` 一致），删除后 `Reset` 回退到第一本。设置页自己不碰 SQLite，只通过
  `Spec` 回调外壳。数据库里 `ledger.description` 从 v1 就在；`store.UpdateLedger` 是新的，
  删除时会连带 `chart` 表（旧工程的级联清单里有，v1 的 `DeleteLedger` 漏了）。
- 运行入口按模式分叉：`viva dev`（`VIVA_RELOAD_ADDRESS` 存在）必须用 `app.RunCanvas`，开发
  重载 worker 在它内部连接宿主；发布版用 `app.RunApplication`，才能拿到 `*app.Window` /
  `*app.Application` 建 `StatusItem`、隐藏到托盘并处理 `OnCloseRequest`。开发宿主只在自身的
  绘制请求里重绘，所以开发模式的关闭直接退出，不弹「每次询问」。
- 工作空间：`desktop.Model` 持有 `*store.Workspace`。启动时读配置里的 `workspaceDir` 并
  `store.Open`；没有配置 / 打不开时内容区显示「打开工作空间」门禁（设置仍可进入），门禁与
  设置里的目录选择都走同一个流程。账本为空的新工作空间由 `accounting.SetRepository` 自动
  创建「默认账本」并播种 19 个分类 / 57 个标签。切换工作空间时先打开新库、注入页面，成功
  后才关闭旧库；页面读不到 SQLite 以外的状态。退出时关闭连接。
- 记账的三个页面共用 `internal/pages/accounting/ledger.Store`（运行时列表，由数据层填充）：
  每个工具栏最左侧都是同一个「账本」Select，运行期间保持选中；切换后记录页重过滤并从第一页开始、
  标签页清空已选分类、模板页换成本账本自己的列表。切换由任一页触发后，`accounting` 统一重载
  三页，避免别的页面留在旧账本的数据上；切子页时目标页也会重载。
- 记账-记录页：工具栏 / 时间范围 / 表头排序 / 表格 / 分页 /
  收支统计都在 `internal/pages/accounting/records`。行数据来自 `repository.Records`：
  SQL 按账本 + 本地日期范围过滤、按表头多列排序并带出标签，页面保留统计 / 分页；工具栏最左侧的
  「账本」`selectbox.Select` 来自共享的 `ledger.Store`：表格、统计与分页随当前账本过滤，
  新记录落到当前账本，右键的「同步到其他账本」列出其余账本并复制一条新记录（不带关联事件
  日期）。编辑用 UPDATE 保留记录 ID；删除在一个事务里删记录与标签关联。
  日期与金额两个表头各带一个方形排序按钮（文字在按钮外）：每次点击在「未选 → 降序 → 升序」
  之间循环；未选用 `arrow-up-down`，降序用 `arrow-up-wide-narrow`，升序用
  `arrow-down-narrow-wide`，选中时图标取主色、底色加一层主色激活态。金额按行上显示的有符号值
  比较（支出记负、收入 / 转账记正），所以降序里 -4 排在 -32 前面。两列可同时生效，
  `sorts` 的顺序就是点击顺序，先点的列是主排序。工具栏原来的「排序」菜单已移除。
  子包 `records/composer` 的「新增记录 / 编辑记录」弹窗写 `transaction_record` 与
  `transaction_record_tag`：模板、日期、类型、分类、标签、标记、关联事件日期、描述与金额按
  旧工程 `record-composer.tsx` 排列；模板来自 `internal/pages/accounting/template` 的缓存，
  和模板页共用同一份，分类 / 标签候选来自当前账本的 `category` / `tag` 表；
  每个字段用 `ui.FormField`（标题 + 控件的标准表单节奏），日期/事件日期用共享的
  `components.DateField`（可输入的日期框 + 浮在弹窗上的月历面板，见下面的「时间选择器」），
  分类是单选下拉（`selectbox.Select`，候选取当前「类型」下的分类；换类型时已选分类留在
  候选末尾，旧工程的 AutoComplete 也是把值留在框里），
  标签用 Viva 的 `components.PaintMultiSelect`，候选取所选分类下的标签。MultiSelect 需要宿主路由：`Config.OnKey` /
  `OnScroll` 交给 `MultiSelectState.Key/Wheel`，`Config.TextInput` 把聚焦的输入接到
  `MultiSelectState.Query`。
  没有记录时用 Viva 的 `components.Empty`（图标 + 标题 + 说明，居中在表格区），
  下面挂旧工程空态的三个按钮：上月 / 今年 / 记一笔。
  工具栏的时间范围是共享的 `datepicker.Picker`（旧工程 `TimeRangePicker`）：日 / 月 / 年
  三个粒度、前后翻一个周期、浮层里的月历 / 月份格 / 年份格与快捷区间（今天 / 近 7 天 / 本周 /
  本月 / 上月；月粒度每月 / 近 6 个月 / 今年；年粒度今年 / 去年），左端「今天」把粒度收回日、
  区间收回今天。范围两端都是本地 `YYYY-MM-DD`，按日包含地过滤记录。
  记录行的右键 ContextMenu 取代原「操作」列：
  编辑、同步到其他账本、删除。底部分页是 `internal/components/pagination`：
  每页 15 / 30 / 50 的 `selectbox.Select` 在 Viva 分页左侧，「共 N 条」在右侧，三者同属
  一个自定义分页组件；分页栏上方不再画分割线。
- 记账-标签页（`internal/pages/accounting/tags`，旧工程 `category-tag.tsx`）：工具栏最左侧是
  共享的账本 Select，后面是支出 / 收入 / 转账三张类型卡片（选中时用该类型的语义色 + 8% 同色底）；
  切账本会清空已选分类；主体是 240px 的
  分类栏、一条竖线、剩下的标签栏。分类栏列出当前类型的分类（名称、记录数徽标、拖拽手柄、
  删除），栏头是计数与「新增分类」；标签栏标题是所选分类名（未选时「标签」），栏头是计数与
  「新增标签」（未选分类时禁用），列出该分类下的标签。两张列表都用 `ui.List` 的
  `Reorder` 拖拽排序；行内删除打开确认弹窗，新增分类 / 新增标签用 `ui.ModalWith`，重名分别
  提示「该分类已存在 / 该标签已存在」，成功后用 `ui.Toaster`。数据来自 `category` / `tag` /
  `transaction_record` / `transaction_record_tag` 表，`repository.Labels` 一次带回分类、标签
  与两类计数；新增 / 删除 / 排序都直接写库，成功后重载；删除分类连带它的标签，删除标签连带
  记录上的标签关联（与旧服务一致）。
- 记账-模板页（`internal/pages/accounting/templates`，旧工程 `templates.tsx`）：工具栏最左侧是
  共享的账本 Select，右侧是「新建模板」；模板按账本存放在 `transaction_template` 表里，
  `internal/pages/accounting/template` 只是绘制用的缓存，切账本换一整套列表；主体是一张通到版心边缘的表
  （软灰底、固定的表头，行间发丝线），列是
  拖拽把手、模板名称、交易类型、分类、标签、标记、描述、操作；行可拖拽排序，操作列的删除
  按钮打开确认弹窗。`新建模板` 弹窗按旧工程的字段顺序：模板名称、交易类型（支出 / 收入 /
  转账）、分类、标签（该分类下的 `components.PaintMultiSelect`；没有分类或分类没有标签时
  字段禁用并显示原因，键盘 / 滚轮 / 输入和记录弹窗一样经 `accounting.Key/Scroll/TagsEditor`
  路由）、描述、标记（离群值）；保存时依次校验「模板名称不能为空」「分类不能为空」，成功后
  toast「保存模板成功」，删除后「删除模板成功」。模板存在 `transaction_template` 表，记录页的
  「记一笔」和模板页共用同一个缓存：在模板页新建的模板马上出现在记录弹窗的模板下拉里，
  但只在它所属的账本下出现；排序写 `sort_order`。
- 记账-分析页（`internal/pages/accounting/analysis`）：左侧图表列表（预设 / 自定义，选中、
  删除、拖拽排序），右侧是图表头、画布、图下方的曲线 / 分组明细表与底部合计带。折线图按账本的 `chart.granularity`
  分月 / 年聚合，曲线条件在 SQL 里按交易类型、分类、标签多选（任意 / 全部）、描述与离群值
  过滤；
  饼图按交易类型 / 分类 / 标签分组，标签分组会重复计入多标签记录。预设「月度消费趋势 /
  年度消费趋势」首次进入分析页时按账本播种，`app_meta` 留标记，删掉后不再回来；预设只能
  删除 / 排序，自定义图表可重命名、改粒度 / 分组、增删曲线。图表配置存 `chart` 表；折线图
  直接交给 `components.PaintAreaChart`（轴、网格、点、悬停竖线与提示框都由组件画，页面只
  把 `lineData` 换成元与 X 轴标签），指针从 `Config.OnPointer` 进外壳、`accounting.Pointer`
  转到 `analysis.Pointer`，再用 `ChartPlot.IndexAt` 换算成悬停点；饼图没有对应组件，仍是
  自绘（扇区、圆环与中心合计由 `drawPie` 画，指针同样走 `analysis.Pointer`，页面用上一帧的
  圆心 / 半径把落点换成扇区，提示框借 `components.PaintChartTooltip`，所以两张图的提示框是
  同一个壳）。
- 日记页（`internal/pages/diary`，旧工程 `diary.tsx` / `diary-tree.tsx` / `services/diary.ts`）：
  **日记不再跟账本走**——这是本工程对旧工程的一处有意改动（旧工程一个账本一本日记）。日记属于
  「日记本」：工作空间里可以有很多本，工具栏最左的选择器选的是日记本，与记账页共享的账本
  选择器没有关系。`diary.Model` 自己拿着 `Books()` / `ActiveBook()` / `CreateBook` / `UpdateBook` /
  `DeleteBook`，设置页的「日记本」卡与日记页读的是同一份；切换日记本时先把草稿写完再换
  （`SetBook` 里先 `flush`），空工作空间第一次打开会自动建一本「默认日记本」。
  工具栏是日记本、今天、`datepicker` 的日字段，右侧是安静的保存状态与 `Outline` 的「删除日记」；左栏 220px 是 `ui.Tree` 的年 → 月 → 日
  日期树（年 / 月行右侧篇数，日行是日号 + 心情 emoji + 字数，三档共用一个右边缘），右栏是
  日期 + 星期 + 字数 + 心情分段（`ui.Choice`，5 个 emoji 加「—」清除）与正文卡。数据是
  `diary_entry` 表，按 `(ledger_id, date)` 唯一，正文纯文本，心情存 emoji——旧库里的中文心情
  由 `domain.NormalizeDiaryMood` 读出时映射，写回一律 emoji；字数按 Unicode 码点
  （`domain.DiaryWordCount`）。**自动保存**：正文绑在 `ui.Field` 上（`ui.TextArea(c, &m.body, …)`），
  每帧开头读字段文本、与已知草稿比较，变了就记下时间并 `RequestFrameIn(600ms)`；到点的帧先写库
  再声明页面，所以写与「已保存」在同一帧里出现。用 `Changed()` 不行：字符串绑定在字段声明时
  才同步，而字数那一行在字段之前声明。切日期 / 切账本 / 离开分区 / 删除前都先 flush；新的一天
  没正文也没心情就不写，打开一个日期不会凭空建一条空日记。
- 事件页（`internal/pages/events`，旧工程 `events.tsx` / `event-*.tsx` / `services/key-event.ts`）：
  **事件不再跟账本走**，这是本工程对旧工程的第二处有意改动（旧工程一条事件属于一个账本）。事件属于
  整个工作空间，每个日期一条（`key_event.date` 唯一），所以工具栏没有账本选择器；页面的数据源是
  `repository.Repository`，图片的选取 / 落盘 / 删除经 `Commands` 交给 `internal/desktop`，页面自己不碰
  文件系统。布局是工具栏（窄年份 `datepicker` + 关联交易开关 + 新增事件）+ 事件列表（3px 身份色条、标题 /
  日期 / 30 字摘要，右上删除）+ 详情（44px 标题 / 日期 / 图片数栏头、主图与缩略图、与图片区 2:1 分配、空的一边保持空态高度的纯文本正文井、
  底部「颜色」+ 20 档色点、添加图片 / 编辑描述）+ 关联交易；无图时主图位是 120px 的「暂无图片 + 添加图片」，
  两侧栏宽按窗口 `clamp` 在 200–280 / 180–280。中栏用 `Basis(0)` 让出剩余宽度，否则色板的固有宽度会在
  960px 窗口把右栏推出窗口。右侧关联交易是 `KeyEventRecords(date)`——工作空间里
  `key_event_date` 等于该日期的记录，跨账本，行里标账本名。图片解码与缩略图在 `internal/assets`：
  导入时生成 300px 缩略图与 1600px 展示副本（`*.view.jpg`），原图留给「保存图片」；旧库在首次显示时
  补齐副本。页面用 `LoadView` 在 `ui.Effect` 里后台加载展示副本，未到达时先画缩略图，所以切事件 / 切图
  不阻塞；`LoadImage` 只给缩略图与旧库兜底。主图 contain、模糊底由 96px 缩略图生成再放大垫底，
  缩略图 160×90 cover；删除在选中主图的「删除图片」，成功后自动选中下一张（最后一张退到上一张）；
  灯箱是 `ui.Control`，滚轮缩放、拖动、左右键切图。图片缓存只保留当前日
  的缩略图、当前选中图的展示副本与一张模糊底：切日 / 切图时释放上一张（`CachedImageBytes` 是回归预算），
  `Draw` 只读缓存、不再解码，避免旧帧把刚切走的整图又加回缓存。删事件 / 删图先删行
  再删文件（`internal/events`），删账本不会碰事件——事件不属于账本。
- 股票页（`internal/pages/stock`，旧工程 `src/pages/stock*` / `electron/services/stock-*`）：
  **股票账户属于工作空间，不属于账本**——这是本工程对旧工程的第三处有意改动（旧工程每账本一个
  股票账户）。一个工作空间有多个账户；`internal/pages/stock/account_store.go` 保存列表与当前
  账户，账户 / 持仓 / 记录 / 心得 / 统计 / 天梯六页工具栏最左共享同一个「账户」`selectbox.Select`，
  切换后 `stock.Model.dataChanged` 重载六页；天梯是市场数据，没有账户选择器。设置 ·
  「股票设置」用与账本相同的行式管理账户（最后一户不可删，删除单事务级联 12 张子表），
  同一小节的「交易与复盘」与「数据管理」组放标签、复盘模板、费用、最近操作 / 回滚、归档与重置。
  `desktop.settingsSpec` 把命令转给 `stock.Model`。工作空间打开且没有账户时股票页建
  「默认股票账户」。
  数据层：`internal/domain/stock_*.go` 负责费用 / 成本结转 / 当前轮 / 现金链 / 统计 / 天梯纯规则；
  `internal/store/stock_trade.go` 与 `stock_replay.go` 负责下单、编辑、删除委托、整体重放与回滚
  （一次委托一个事务；同秒买单排在卖单前；轮次按「股票 + 轮次号」复用 ID）；`stock_read.go` /
  `stock_lessons.go` / `stock_settings.go` / `stock_stats.go` / `stock_limit.go` 提供页面读模型；
  `repository.StockRepository` 是页面唯一依赖。费用口径：佣金按整笔委托收一次并套最低佣金，
  印花税 / 过户费逐笔取整再相加；0 手清仓标记保留，减仓到 0 不归档。
  页面：账户页（总资产、8 指标、资金记录 10/20/50 分页、追加 / 支取 / 利息归本）；持仓页
  （240px 持仓栏、详情、行情 `ui.Effect`、本轮费率、台账、每日复盘、本轮复盘、当前轮成交、
  建仓 / 加减仓 / 清仓）；记录页（按轮次折叠、底部汇总、编辑 / 删除成交、标签与复盘）；
  心得页（≤80 字、关联轮次、跳转）；统计页（8 卡、`PaintAreaChart`、13 列明细分页、指针经
  `stock.Pointer`）；天梯页（日期 / 状态 / 刷新 / 自包含 HTML 导出、分层与窄窗 Flow）；
  设置页 ·「股票设置」是账户设置页签搬到设置页后的样子：账户列表（当前 / 设为当前 / 编辑 / 删除 /
  新增）之后是「交易与复盘」组（标签、两份复盘模板、费用）与「数据管理」组（最近操作 / 回滚、归档、重置）。
  行情 `internal/stockquote`（腾讯，GBK 3 秒）失败返回空，市值按成本、字段显示 `—`；
  涨停池 `internal/stocklimit`（东财，8 秒）按旧策略缓存：09:30 前 `preopen`、周末 `offday`、
  盘中只渲染不落库、45 天外 `unavailable` 不写空标记、`qdate` 不符返回空。两者都在
  `ui.Effect` 里执行，不阻塞绘制；代理仍按配置文件的 `proxy` 构造 HTTP client。
- 待办页（`internal/pages/todo`，旧工程 `src/pages/todo*` / `electron/services/todo.ts`）：
  **属于账本**，与旧库的 `ledger_id` 一致；工具栏的「账本」是 `accounting/ledger.Store` 的同一个
  选择，记账 / 日记 / 待办三处切换互相同步。看板是横向滚动的主题列（每列自己的事项滚动），
  列头可折叠、通过「主题操作」菜单前移 / 后移 / 删除；事项行有完成圆圈、标题、六档紧急 /
  重要度、截止日（逾期 / 今天高亮）与进度计数，点开内联进度，弹窗编辑事项（日期与两档）。
  历史按 `completed_at` 倒序列出，可恢复、删除、查看并订正进度；四象限按紧急 × 重要分组，
  点事项直接编辑。六档与「就近取档」在 `internal/domain`，页面只依赖
  `repository.TodoRepository`；`store.TodoCards` / `TodoHistory` 一次取卡片、事项与进度再分组，
  删卡片 / 事项在同一事务里清子行。
- 设置 ·「日记配置」（`internal/pages/settings` 的 `diaryConfig`）分两张卡：**日记本**（照账本管理
  的行：名称 + 描述、当前那本的「当前」、行尾编辑 / 删除图标、底部「新建日记本」；最后一本不可删，
  增删改都走 `diary.Model`，与账本走 `accounting.Model` 对称）与 **导入导出**。导入按目录递归扫
  `YYYY-MM-DD.txt`（大小写敏感的 `.txt`，日期必须是真实的一天），导出写 `<date>.txt`。
  编码回退链在 `internal/diary/decode.go`：合法 UTF-8 原样（保留 BOM）→ 带 BOM 的 UTF-16 →
  GBK → 非法字节逐段替换成 `?`；GBK 结果含 U+FFFD 说明不是 GBK，继续走最后一步（Go 的 GBK
  解码器不会像旧工程的 `fatal:true` 那样报错）。目录选择器与文件读写由 `internal/desktop`
  的 `importDiary` / `exportDiary` 调 `internal/diary`，读写的是日记页当前那本（说明里写着本名），
  页面只拿结果（"导入 1/1 篇" 与失败清单），导入后调 `diary.Reload()`，切回日记页就能看到。
- v0.6.7 起窗口配置由 `ui.Configure(page, app.Config)` 生成（`page.Config` 已移除）；`go.mod`
  的 `tool` 行钉住同版本 CLI，验证用 `go tool viva vet`，PATH 上的独立 `viva` 不随模块升级。
- 页面自己浮起来的层（选择器面板、标签弹层）在切走分区 / 子功能时由外壳收起：页面实现
  `CloseOverlays()`，`internal/desktop` 的 `selectNav` 在离开前调用它，浮层不会跟着用户
  跑到另一个页面上。
- 导航是两级：一级功能项展开自己的子功能，同时收起其他一级项；没有子功能的功能（事件、日记）
  就是单独一项。页面通过 `Tabs() []string`、`Tab() int`、`SelectTab(int)` 暴露子功能，
  内容里不画 `ui.Tabs`。
- 设置页的版式是「设置组」：一个 16px 700 的组名加它下面的内容，组名与内容间距 8px，
  组与组 24px。右栏只有一处实现——`internal/pages/settings/controls.Page`：它自己撑满
  右栏、把 760px 阅读列居中、固定小节标题，并让设置组在标题下方的独立视口里滚动，按 `GroupGap` 排列传入的 `GroupSpec`。
  每个设置小节只填 `GroupSpec`（`Title` + 行内容；`Raw` 给身份头 / 尾注这类不套卡的块），
  不再自己包 `Column` 或手写组间距。`Section` / `Group` / `Card` 仍是组本身的实现，
  关于软件的「版本信息 / 更新 / 项目」、日记配置的「日记本 / 导入导出」与股票设置都用同一套。
- 设置 ·「数据备份」由 `internal/backup` 负责：手动备份用 `VACUUM INTO` 写「库 + assets」
  目录，自动备份每天第一次打开工作空间时写单库快照；两者各自只保留最近 2 份。恢复前先把
  当前库写成 `pre-restore-*.db`，恢复后由外壳重新 `store.Open` 同一目录并 `adoptWorkspace`，
  不重启进程。备份位置、自动备份开关与最近工作空间写现有配置。
- 组件按归属放：可复用的组件放在 `internal/components` 自己的子包里（`datepicker`、
  `pagination`、`selectbox`，包名即组件名，导出名不再重复包名）；只有某一个页面用的组件
  才放在该页面的 `controls` 子包里（例如 `internal/pages/settings/controls` 的 `Group` /
  `Row`）。
- 一个页面是一个持有自己状态的类型：`type Model struct{...}`、`New() *Model`、`View(c ui.C)`。
  状态由外壳长期持有；切换导航只是换 `View`，不重建模型。
- `View` 只读模型、不修改状态；点击在控件声明处改状态（Viva 的 immediate-mode 规则）。
- 子功能用 `ui.Tabs`，值存在页面模型里；页签内容按当前值声明，不预先创建全部页面。
- 页面不持有 `ui.C`、`ui.Box`、`gfx.Canvas`；不手算坐标；不启动 goroutine。耗时的读取用
  `ui.Effect`，结果通过 `Take` 在窗口线程落到模型。
- 真实窗口只用于人工验收（IME、焦点丢失、窗口管理器关闭、高分屏）。日常验证用
  `app.NewDriver`，见 `internal/desktop/desktop_test.go`。

## 时间选择器（internal/components/datepicker）

- 只有一个组件 `DatePicker(c, state, spec)`，外观由参数决定：
  - 工具栏的时间范围：`Grains: true`（日 / 月 / 年分段）+ `Periods: true`（前后一个周期的箭头）。
  - 表单的日期 / 关联事件日期：固定 `Grain: DateByDay`，`Grains` / `Periods` 都不开，
    `Fill: true` 撑满字段列，`CloseOnPick: true` 选完即收起。
  日期算术（月长、整周、按粒度对齐 / 翻页）与快捷区间都在这个文件里，纯函数、用 UTC 上的
  `time.Time` 做公历运算，不碰时区与夏令时。
- **只用原生样式**：触发器统一是 `ui.Button(值).Outline().Icon(Calendar)`，表单里加 `.FillWidth()`；
  工具栏不给宽度靠内容，而是给一个固定宽度——按三种粒度最长的范围文本
  （`YYYY-MM-DD ~ YYYY-MM-DD` / `YYYY-MM ~ YYYY-MM` / `YYYY ~ YYYY`）量出来的那个宽度，
  所以换粒度、拉范围时字段不会变宽变窄。
- 三种粒度共用一套格子：日、月、年都自己排成网格，每格交给 `components.BeginButton` 画——
  端点 `Default`（实心主色）、区间内 `Secondary`、今天 `Outline`（描边）、其余 `Ghost`；
  「日」的状态因此与「月 / 年」是同一套外观。标题、星期行、前后箭头也走同一套原生控件
  （`Icon` 变体的 `Ghost` 按钮 + `LabelCenter` 的标题样式），浮层表面取
  `components.PopupRecipe` 的填充、描边、圆角与阴影。这里不写颜色、字号或固定像素尺寸：
  尺寸从 `c.Theme().Metrics` 与 `components.BeginButton` 的度量推出来。
  日粒度的标题两侧在最外侧多一对 `chevrons-left` / `chevrons-right` 的整年箭头，单箭头
  仍是翻月；月 / 年粒度只有单箭头（翻年 / 翻十年）。
- 面板仍画在画布上（`gfx.Canvas` + `HitWithOptions`）而不是 `ui.Popover`：Viva v0.6.7 里
  Popover 在 `ui.Modal` 内能画出来却收不到点击（hit 层被判给弹窗），见下面的「框架缺口」。
  `PushLayer(20)`，与标签多选同一层：弹窗有自己的合成层，面板必须在其上才看得见、点得着。
  外部点击由一个覆盖页面的背景 hit 关闭（`PassPrimary/PassSecondary`，与旧工程的文档级
  pointerdown 一致：关闭的同时那次点击照常落到下面的控件上）。
- 面板不占布局：它画在触发控件的 `DrawOver` 里，打开、翻页、选日期都不会让页面或弹窗重新排版。

## 框架缺口（Viva v0.6.7）

- **`viva dev` 的 reload host 不转发原生文本事件。** worker 协议只带画面、指针、按键和
  滚轮；`Config.TextInput` 和页面的 TextField 提供者不会到达宿主，所以 `viva dev` 下的
  `ui.TextField` 有光标却无法输入（记录描述、图表名称、模板名称都一样），不是某个弹窗的
  问题。需要输入文本时用 `go run .` / `make run` 或打包后的原生窗口；不要用 dev host 判断
  表单输入是否可用。Viva 的 conformance 也明确记录了这一点（Native text events in
  `viva dev`：worker protocol does not carry them）。
- **`viva dev` 下悬停不刷新。** 开发宿主的画面是 worker 的绘制快照，悬停身份由宿主随
  `PaintRequest` 带回 worker（`Canvas.SetInteraction`）；而 worker 收到一个普通指针移动时，
  只把它交给应用的 `Config.Pointer`，页面自己有指针通道（`Canvas.ListensForPointer`，例如
  `ui.Control` 的 Handlers）时才会走指针 pass 重绘。结果是右键菜单项、列表行这类「只靠
  `Canvas.Hovered()` 变样式」的悬停，在 `viva dev` 里看不到变化；`go run .` / 打包版走真实
  窗口，悬停正常。判断悬停或悬停底色是否成立要用真实窗口（驱动测试也算），别用 dev host。
- **`viva dev` 的 reload host 不接收窗口按钮的样式回调。** worker 协议只把 `ClearColor`
  的四个分量送给宿主，`Config.WindowControls` 是函数，过不去；宿主只在 worker 启动时按
  `ClearColor` 的明度选一次按钮墨色（亮底黑字、暗底白字）。所以在开发宿主里用标题栏的
  明暗开关切外观，页面变了、三个窗口按钮不变——浅色下白字几乎看不见。发布版走
  `app.RunApplication`，Viva 每帧调用 `WindowControls`，按钮跟着主题走（`internal/desktop`
  的 `TestWindowControlsFollowTheTheme` 守着这条）。`viva dev` 里切完外观保存一次文件让
  worker 重启，或直接用 `go run .` / `make run`，都能拿到正确墨色。
- **Select 的下拉动画没收尾。** `ui` 画一个打开的下拉时用自己的 `gfx.Canvas.BeginPresence`
  起了入场的组（`slide-in-from-*-2`，8px），却没有结束它；同一帧里之后第一个
  `EndPresence`——也就是弹窗在自己 `End` 里做的那一次——会把这个组收掉，并把下拉的
  8px 位移应用到「下拉之后画的所有东西」上。表现是：在弹窗里点开任意 Select，弹窗中
  该字段以下的标题、控件、底部按钮会先整体跳 8px，再在 100ms 里弹回。
  本仓库的 `internal/components/selectbox` 提供了 `selectbox.Select`：它在
  `DrawOver` 里（外壳把 Select 自己画完之后）补上那次 `EndPresence`，位移只留给下拉
  本身，弹窗不再抖。`ui.Select` 在本工程里一律走这个包装；等框架自己收尾后删掉即可
  （回归测试：`internal/desktop` 的 `TestRecordMarkSelectKeepsTheDialogStill`）。
- 同类问题：`ui` 里凡是「用 `BeginPresence` 开、由调用方 `End`」的组件，放进弹窗都要
  检查是否漏了收尾；漏了就会由弹窗的 `EndPresence` 误收，位移落到弹窗内容上。
- **`ui.Scroll` 不要设 `FillHeight`。** 纵向的滚动区本来就会被所在的行 / 列分到剩余高度；
  一旦加 `FillHeight`，`measureContainer` 只对 `dimAuto` 的滚动区把首选高度归零，于是它把**内容高度**
  报成自己的首选高度，长列表会把所在 Row 连同整页撑高。事件页的缩略图列就曾因此把主图和底栏顶出
  窗口（回归测试：`internal/desktop` 的 `TestEventGalleryWithManyImagesStaysInsideTheWindow`）。
  滚动区只给宽度或 `Grow(1)`，高度交给容器。
- **表单字段的标签字号与颜色改不了。** `ui.FormField` 画的是 `components.BeginField` 的
  recipe：标题固定 `Metrics.FontSizeM`（14）+ `WeightMedium` + `Foreground`，错误行同号
  危险色，`field.Spec` 只收文本、不收样式。所以记录弹窗里的八个字段标题和正文同号，
  层级靠位置与字段边框；页面不要再自己画一行标签去「补」字号，否则同一张表单会有两套
  标签。对话框标题是 `Metrics.FontSizeM + 2`（16），正好等于 Section 档。
- **`ui.Modal` 里的 `ui.Popover` 收不到点击。** 弹窗给自己的内容开了焦点陷阱，Popover 的层
  虽然画在上面（`HitTest` 也能找到它的 hit），普通点击仍被判给弹窗里的控件，面板里什么都
  点不动（右键菜单、Select 的下拉不受影响：它们不走 Popover 这条路径）。所以日期面板是
  用画布自己画的（见「时间选择器」），与标签多选同一套做法。
- **图表提示框的数值贴住右边框。** `components.PaintChartTooltip` 用 `Canvas.TextWidth`
  量数值宽度（常规字重），绘制时却用 `WeightMedium`；字重更宽，右侧 10px 内边距被吃掉，
  最后一位数字贴着边框（数值本身不裁切）。框架已经提供 `TextWidthWeight`，等它用上就
  自动消失；应用里不要给数值补空格之类的补偿。
- **没有内建的图片解码与缩放 API。** `ui.Slot` / `gfx.Bitmap` 只负责画，解码、EXIF 方向、缩放都归应用。
  `internal/assets` 用标准库 `image/jpeg|png|gif`、`golang.org/x/image/webp`、`x/image/draw` 与
  `github.com/gen2brain/h265/heic`（纯 Go HEVC/HEIC 解码器，MIT，无 CGo、无系统编解码器依赖；其 README
  提醒 HEVC 专利授权由使用方自行考虑）解码成
  `gfx.Bitmap`（预乘 BGRA）；缩略图 300px 宽 JPEG q75；主图解码后最长边缩到 2048 再画，且只在选中变化时
  解码，`Draw` 里不读文件。GIF 只画第一帧（灯箱与画廊都不播动画）。HEIC 解码较贵，所以只在导入时跑一次：
  展示图转成 JPEG q92，`<id>.heic` 原图保留在 `original_path`，之后每次 paint 读的都是 JPEG。
- **没有视频控件，实况照片只存不播。** 照片与同名视频一起导入、`motion_path` 落库，界面只显示「实况」
  角标与说明。Viva 有内嵌浏览器（`app.WebHost`），但本项目明确不要内嵌浏览器，所以不用它绕。
- **`components.Sidebar` 的子菜单两侧都缩进。** `AddSubMenu` 照搬 shadcn 的 `mx-3.5` +
  `px-2.5`：子项在右侧也比一级项再收进 24px，而接口没有宽度 / 缩进的开关；缩进本应只在
  前缘，否则选中的子项底色比一级项短一截，右侧看起来是断的。主内容区又用卡片边框当分隔
  线（见「页面约定」），子菜单左侧那根引导线被压成底色，两边的缩进没有视觉依据。所以导航
  不用这个组件，改在 `internal/desktop/sidebar.go` 用 `ui` 的行声明：子项只加
  `MarginLeft`，底色与一级项共用同一条右边缘（回归测试：`internal/desktop` 的
  `TestSidebarSubItemsShareTheirSectionRowTrailingEdge`）。

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
