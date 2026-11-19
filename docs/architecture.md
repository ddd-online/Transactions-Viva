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
       │    ├─ internal/pages/*/controls   只属于该页面的组件
       │    ├─ internal/components/*      可复用的自定义组件（datepicker / pagination / selectbox）
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

**自定义组件不设样式。** 自定义组件（`internal/components` 与各页面的 `controls` 子包）只用原生外观：`ui` 控件本来的样子、
`components` 的 recipe（`BeginButton` / `Calendar` / `PopupRecipe` / `Select` …）和
`c.Theme()` 的 `Metrics` / 颜色。不写字号、颜色、圆角、阴影常量，也不给控件固定像素尺寸——
尺寸和留白从主题度量与原生组件的布局里来；需要在画布上画浮层时，同样只调用 recipe 的
`Paint` 或取它的度量。

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
| `internal/components` | 可复用的片段：占位空态（后续的 page header、table cell 等） | `src/components` |
| `internal/components/datepicker` | 日期 / 时间范围选择器（工具栏与表单同一组件） | `src/components/time-range-picker.tsx`、antd DatePicker |
| `internal/components/pagination` | 分页栏：页大小 Select + Viva 分页 + 总数 | `src/components/pagination.tsx` |
| `internal/components/selectbox` | `ui.Select` 的包装，补上框架漏收的动画组 | — |
| `internal/pages/<page>/controls` | 只属于某一个页面的组件：目前是设置页的分组卡片 / 设置行 | 旧工程同页目录下的组件 |
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
- 记账-记录页是当前唯一有真实前端的页面：工具栏 / 时间范围 / 排序 / 表格 / 分页 /
  收支统计都在 `internal/pages/accounting/records.go`，行数据来自 `records_mock.go`，
  `records_composer.go` 的「新增记录 / 编辑记录」弹窗只写内存：模板、日期、类型、分类、
  标签、标记、关联事件日期、描述与金额按旧工程 `record-composer.tsx` 排列，模板也是 mock；
  每个字段用 `ui.FormField`（标题 + 控件的标准表单节奏），日期/事件日期用共享的
  `components.DateField`（可输入的日期框 + 浮在弹窗上的月历面板，见下面的「时间选择器」），
  分类是单选下拉（`selectbox.Select`，候选取当前「类型」下的分类；换类型时已选分类留在
  候选末尾，旧工程的 AutoComplete 也是把值留在框里），
  标签用 Viva 的 `components.PaintMultiSelect`。MultiSelect 需要宿主路由：`Config.OnKey` /
  `OnScroll` 交给 `MultiSelectState.Key/Wheel`，`Config.TextInput` 把聚焦的输入接到
  `MultiSelectState.Query`。
  工具栏的时间范围是共享的 `components.DateRangePicker`（旧工程 `TimeRangePicker`）：日 / 月 / 年
  三个粒度、前后翻一个周期、浮层里的月历 / 月份格 / 年份格与快捷区间（今天 / 近 7 天 / 本周 /
  本月 / 上月；月粒度每月 / 近 6 个月 / 今年；年粒度今年 / 去年），左端「今天」把粒度收回日、
  区间收回今天。范围两端都是本地 `YYYY-MM-DD`，按日包含地过滤记录。
  记录行的右键 ContextMenu 取代原「操作」列：
  编辑、同步到其他账本、删除。底部分页是 `internal/components/pagination`：
  每页 15 / 30 / 50 的 `selectbox.Select` 在 Viva 分页左侧，「共 N 条」在右侧，三者同属
  一个自定义分页组件；分页栏上方不再画分割线。接入 store 时替换 mock，不改这套布局。
- 导航是两级：一级功能项展开自己的子功能，同时收起其他一级项；没有子功能的功能（事件、日记）
  就是单独一项。页面通过 `Tabs() []string`、`Tab() int`、`SelectTab(int)` 暴露子功能，
  内容里不画 `ui.Tabs`。
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
- **只用原生样式**：触发器统一是 `ui.Button(值).Outline().Icon(Clock)`，表单里加 `.FillWidth()`；
  工具栏不给宽度靠内容，而是给一个固定宽度——按三种粒度最长的范围文本
  （`YYYY-MM-DD ~ YYYY-MM-DD` / `YYYY-MM ~ YYYY-MM` / `YYYY ~ YYYY`）量出来的那个宽度，
  所以换粒度、拉范围时字段不会变宽变窄。
- 三种粒度共用一套格子：日、月、年都自己排成网格，每格交给 `components.BeginButton` 画——
  端点 `Default`（实心主色）、区间内 `Secondary`、今天 `Outline`（描边）、其余 `Ghost`；
  「日」的状态因此与「月 / 年」是同一套外观。标题、星期行、前后箭头也走同一套原生控件
  （`Icon` 变体的 `Ghost` 按钮 + `LabelCenter` 的标题样式），浮层表面取
  `components.PopupRecipe` 的填充、描边、圆角与阴影。这里不写颜色、字号或固定像素尺寸：
  尺寸从 `c.Theme().Metrics` 与 `components.BeginButton` 的度量推出来。
- 面板仍画在画布上（`gfx.Canvas` + `HitWithOptions`）而不是 `ui.Popover`：Viva v0.6.5 里
  Popover 在 `ui.Modal` 内能画出来却收不到点击（hit 层被判给弹窗），见下面的「框架缺口」。
  `PushLayer(20)`，与标签多选同一层：弹窗有自己的合成层，面板必须在其上才看得见、点得着。
  外部点击由一个覆盖页面的背景 hit 关闭（`PassPrimary/PassSecondary`，与旧工程的文档级
  pointerdown 一致：关闭的同时那次点击照常落到下面的控件上）。
- 面板不占布局：它画在触发控件的 `DrawOver` 里，打开、翻页、选日期都不会让页面或弹窗重新排版。

## 框架缺口（Viva v0.6.5）

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
- **`ui.Modal` 里的 `ui.Popover` 收不到点击。** 弹窗给自己的内容开了焦点陷阱，Popover 的层
  虽然画在上面（`HitTest` 也能找到它的 hit），普通点击仍被判给弹窗里的控件，面板里什么都
  点不动（右键菜单、Select 的下拉不受影响：它们不走 Popover 这条路径）。所以日期面板是
  用画布自己画的（见「时间选择器」），与标签多选同一套做法。

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
