# transactions

## 项目目标

Transactions 是一款本地优先的个人记账桌面应用：一个工作空间内管理多个账本，覆盖记账
（记录 / 分析 / 标签 / 模板）、股票（账户 / 持仓 / 记录 / 心得 / 统计 / 天梯 / 设置）、
待办、事件与日记。核心财务数据只保存在用户自己选择的本地 SQLite 工作空间里。

本轮重构的目标，是把旧工程 `D:\github\Transactions`（Electron 44 + React 19 + TypeScript）
整体迁移到 **Viva + Go**：没有 Electron、没有 Node、没有内嵌浏览器，窗口与控件由 GPU
直接绘制。**行为、文案、交互、数据契约一律以旧工程为准，先追平再改进。**

当前阶段：**外壳、配置与通用设置、记账四页、日记与 SQLite 数据层已落地；记账四页与日记读写工作空间；旧数据可导入；股票、待办、事件仍是占位。**

- 入口是根目录 `main.go`：只做 `log.Fatal(desktop.Run())`，`viva.toml` 的 `package = "."`。
  开发时 `viva dev .`，一次性运行 `go run .`。注意：`viva dev` 的 reload host 不转发原生
  文本事件，所有 `ui.TextField` 在它下面都只有光标、不能输入；它也不刷新宿主侧的悬停，
  右键菜单项 / 列表行的 hover 在 dev 下看不出来（`go run .`、打包版和驱动测试都正常，
  见 `docs/architecture.md` 的「框架缺口」）。需要测表单输入或悬停时用
  `go run .` / `make run` 或打包版。
- 外壳（标题栏 / 左侧导航 / 设置页）、页面骨架、领域类型与文档已就位。
- 记账-记录页已迁移前端：工具栏、时间范围、表头排序、表格、分页、收支统计与
  新增 / 编辑记录弹窗都在。弹窗照旧工程 `record-composer.tsx` 排列模板、日期、类型、分类、
  标签、标记、关联事件日期、描述与金额；字段用 `ui.FormField`，日期与关联事件日期用共享的
  `datepicker.Picker`（同一个组件，参数关掉粒度分段与前后周期，见
  `internal/components/datepicker`），
  分类是单选下拉（候选取当前类型下的分类），
  标签用 Viva 的 `components.PaintMultiSelect`（`MultiSelectState`），
  下拉一律用本工程的 `selectbox.Select`（`internal/components/selectbox`：Viva v0.6.5
  的 Select 不收下拉动画的组，弹窗里会把下面的字段带得抖一下，见 `docs/architecture.md`
  的「框架缺口」）。
  记录行右键打开 ContextMenu（编辑 / 同步到其他账本 / 删除）。行数据来自工作空间
  `transactions.db`：账本 + 日期范围 + 多列排序都在 SQL 里；统计与分页在页面内存里；
  编辑保留记录 ID，同步到其他账本会复制一条新记录（不带关联事件日期）。工具栏最左侧是
  「账本」Select，列表来自数据库；切换后表格、统计与分页按账本过滤，新记录落到当前账本；
  这个选择在记账的记录 / 标签 / 模板三页之间共享。
  日期与金额两个表头各带一个方形排序按钮（文字在
  按钮外）：未选 → 降序 → 升序，未选 `arrow-up-down`、降序 `arrow-up-wide-narrow`、
  升序 `arrow-down-narrow-wide`，选中时图标变主色；金额按有符号值比较（支出为负），两列可同时
  生效、按点击顺序决定优先级（先点的先排）。工具栏的时间范围用同一个
  `datepicker.Picker`
  （打开 `Grains` / `Periods`：日 / 月 / 年
  粒度、前后周期、快捷区间，日粒度的面板标题两侧还有 `chevrons-left` / `chevrons-right`
  的整年箭头，浮层面板不占布局）；分页栏在 `internal/components/pagination`：
  每页 15 / 30 / 50 的 Select 在 Viva 分页左侧，「共 N 条」在右侧，三者是一个自定义组件。
- 记账-标签页已迁移前端：工具栏最左侧是同一个「账本」选择器，后面是 支出 / 收入 / 转账 类型卡片；
  主体是 240px 分类栏 + 标签栏；切账本会清空已选分类；
  分类与标签都能新增、删除（带确认弹窗）、拖拽排序，重名在弹窗里提示；数据与记录数来自
  工作空间；新建账本时播种旧工程的 19 个分类 / 57 个标签；删除分类连带其标签，删除标签
  连带记录上的标签关联。
- 记账-分析页已迁移前端：工具栏是共享账本、分析时间范围与「新增图表」；左侧是图表列表
  （选中 / 删除 / 拖拽排序），右侧是折线图或饼图画布、图下方的曲线 / 分组明细表，
  以及底部一条合计带（收入 / 支出 / 转账；饼图是合计与组数）。内置「月度消费
  趋势 / 年度消费趋势」两张预设（首次进分析页按账本播种，删掉后不再回来），可新增自定义
  折线图 / 饼图、重命名、改粒度、改饼图分组、增删曲线；曲线条件支持交易类型、分类、标签
  多选（任意 / 全部）、描述与离群值。折线图由 `components.PaintAreaChart` 绘制（轴、网格、
  点、悬停提示框都是组件的），指针经 `Config.OnPointer` → `accounting.Pointer` →
  `analysis.Pointer` 进入页面；饼图仍是自绘（扇区、圆环、中心合计与悬停提示都由
  `drawPie` / `pieTooltip` 画，提示框是同一个 `components.PaintChartTooltip`，指针命中靠上一帧
  的圆心与半径换算成扇区）。明细表最多 5 行、以「栏头 + 两行」为下限：窗口矮到画不下时先缩表，
  否则它的最小高度会把合计带和整个外壳顶出窗口。图表存 `chart` 表，按账本与 `sort_order` 排序；
  时间范围默认本年，月 / 年粒度由图表自己的 granularity 决定。
- 记账-模板页已迁移前端：工具栏最左侧是同一个「账本」选择器，右侧是「新建模板」；主体是通到
  版心边缘的表格（拖拽把手 / 模板名称 / 交易类型 / 分类 / 标签 / 标记 / 描述 / 操作），
  行可拖拽排序、删除前确认；模板按账本存放，切账本换一整套列表；「新建模板」弹窗按旧工程校验模板名称与分类，
  标签取所选分类下的多选器（Viva `PaintMultiSelect`，与记一笔共用同一套键盘 / 滚轮 / 输入路由）。
  模板存在 `transaction_template` 表，`internal/pages/accounting/template` 只是同步绘制用的
  缓存；记录页「记一笔」的模板下拉与模板页读同一份缓存，新建的模板马上能在弹窗里选到。
- 日记页已迁移前端：工具栏是**日记本**（不是账本——日记与账本无关，一本日记本是一本独立的
  笔记本，工作空间里可以有多本）、今天、日期字段、保存状态与「删除日记」；左栏是
  年 → 月 → 日 的日期树（年 / 月行带篇数，日行带心情 emoji 与字数，点年份或月份展开收起），
  右栏是当天的标题（日期 + 星期 + 字数 + 心情分段）与正文卡。正文是纯文本，心情是五个 emoji
  加一个清除档；**自动保存**：最后一次改动后停 600ms 落库，工具栏依次显示 已保存 / 未保存 /
  保存失败，切日期、切日记本、离开页面与删除前先把草稿写完。字数按 Unicode 码点算。日记按日记本
  隔离（`(book_id, date)` 唯一），旧数据里的中文心情读出时映射到同一套 emoji。文字区域用
  `ui.Field`（`ui.TextArea(c, &m.body, …)`）——字符串绑定要到字段声明时才同步，字数那一行在它
  之前声明，只有直接读字段才能在同一个 paint 里数到刚敲的字。
- 设置 ·「日记配置」是两组：**日记本**（照账本管理：列名称 + 描述，标出日记页正在读的那本，
  每行有编辑 / 删除，底部「新建日记本」；最后一本不可删）与 **导入导出**（按目录批量导入，
  递归扫 `YYYY-MM-DD.txt`，编码回退链 UTF-8 → UTF-16(BOM) → GBK → 非法字节换 `?`；按全部 /
  年 / 月导出 `<date>.txt`，UTF-8）。导入导出都作用在日记页当前那本上，行说明里写着本名。
  文件与目录由 `internal/diary` 处理，弹目录选择器与回调在 `internal/desktop`，页面只显示结果；
  日记本的增删改走 `diary.Model`（与账本的增删改走 `accounting.Model` 对称），导入后日记页重载。
  两组用的是设置页统一的设置组组件 `controls.Section`（组名 + 组内容，8px；组与组 24px），
  与「关于软件」的版本信息 / 更新 / 项目同一套节奏。
- 用户配置已落地：`internal/config` 读写 `~/.transactions.json`（`viva dev` 期间是
  `~/.transactions.dev.json`），逐字段归一化，写回保留未知顶层键，写入走 `.tmp` + rename。
  窗口尺寸 / 位置（关闭时保存）、工作空间、外观、关闭行为、功能开关与账本级覆盖都在里面；
  `desktop.Run` 启动时加载，设置页的改动立即写回。
- SQLite 工作空间已落地：目录里的 `transactions.db`，schema v4 含 `app_meta` /
  `ledger` / `transaction_record` / `transaction_record_tag` / `category` / `tag` /
  `transaction_template` / `chart` / `diary_book` / `diary_entry`（列与旧库兼容，不建股票等表）。打开时 `quick_check`、版本迁移
  （升级前 `VACUUM INTO <db>.bak`）、拒绝损坏 / 陌生 / 旧版 / 更高版本库。空工作空间
  自动创建「默认账本」并播种默认分类标签；设置里切换工作空间先打开新库，失败保留旧库。
  没有工作空间时内容区显示「打开工作空间」门禁，设置仍可进入。
- 通用设置（除网络代理）已可用：工作空间（目录选择器，记入最近 5 条）、外观（浅色 / 深色 /
  跟随系统，标题栏的明暗开关写同一份配置，系统换色时自动重绘）、关闭行为（直接退出 /
  最小化到托盘 / 每次询问；托盘用 Viva 的 StatusItem，没有托盘时回退为退出）。功能开关会
  立即从左侧导航移除对应分区；网络代理与数据备份仍是占位；关于软件已可用（版本 / 构建 / 平台 / 运行时、
复制版本信息、项目与问题反馈链接）。
- 运行入口按模式分叉：`viva dev`（有 `VIVA_RELOAD_ADDRESS`）必须走 `app.RunCanvas`——开发
  重载 worker 只在 `RunCanvas` 里连接宿主，换成 `RunApplication` 会打开真窗口并让宿主等
  连接直到超时；发布版走 `app.RunApplication` 才能拿到窗口 / Application 句柄，建托盘、
  隐藏窗口、处理原生关闭。开发宿主只按自己的绘制请求重绘，所以开发模式关闭窗口直接退出，
  不弹「每次询问」。
- 原来的 `viva init` 示例（计数页）已移除；根目录 `main.go` 现在是真实入口。
- 旧数据导入工具：`go run ./cmd/tr-import -from <旧工作空间或 transactions.db> -to <新目录>`
  只读旧库，复制账本 / 分类 / 标签 / 记录 / 记录标签 / 模板，保留 ID、时间戳、排序与空值，
  报告孤儿行（`-strict` 中止）；旧库的股票、事件、日记、待办不导入，`chart` 表有则一并复制。
- 其余页面的真实内容还没迁移（见下面的迁移顺序）。

## 参考对象

| 路径 | 角色 | 纪律 |
|---|---|---|
| `D:\github\viva` | Viva 框架源码 | 只读。通用桌面能力先查这里；本工程依赖的是 `go.mod` 里的版本（当前 v0.6.5），本地源码可能更新，**以 `go doc` 为准，不要拿源码里的新 API 写进本工程**。 |
| `D:\github\motty` | 参考应用（大型、可运行的 Viva 应用） | 只读。目录划分、入口、构建、窗口组织、测试与 headless 驱动的样板；不要整段复制它的业务代码。 |
| `D:\github\Transactions` | 旧工程（Electron / React） | 只读，行为权威。页面在 `src/pages`，服务在 `electron/services`，领域规则在 `shared/domain`，SQLite schema 与迁移在 `electron/db`，视觉规范在 `DESIGN.md`，产品定位在 `PRODUCT.md`，E2E 在 `tests/e2e`。 |
| `D:\github\viva\examples` | 框架示例 | 写页面前先找最接近的例子（`guides/`、`recipes/`）；`go doc github.com/vivaui/viva/ui` 列全部元素。 |

**禁止修改这三个参考目录里的任何文件。** 需要新能力时，写在本仓库；属于框架缺口的，
先记录在 `docs/architecture.md`，不要在应用里绕过去。

## 工程结构

```text
main.go                  真实入口：只做 log.Fatal(desktop.Run())
internal/desktop/        应用外壳：窗口、自绘标题栏、Sidebar 导航、设置页、主题
internal/icons/          由 SVG 编译出的矢量图标（viva icons）
internal/pages/          各功能页：accounting、stock、todo、events、diary、settings
  accounting/records/    记录页本身（表格 / 工具栏 / 分页 / 空态 / 行操作）
  accounting/records/composer/  新增 / 编辑记录弹窗
  accounting/ledger/     记账各页共享的账本列表与当前账本
  accounting/tags/       标签页（类型卡片 / 分类与标签两栏 / 新增与删除弹窗）
  accounting/template/   模板缓存与 domain 互转（按账本存放，记录弹窗与模板页共用）
  accounting/templates/  模板页（表格 / 新建与删除弹窗 / 拖拽排序）
  accounting/analysis/   分析页（图表列表 / 折线图 / 饼图 / 曲线条件 / 合计）
  diary/                 日记页（日记本 / 日期树 / 正文 / 心情 / 自动保存 / 删除）
internal/diary/          日记文件的导入导出（扫描、编码回退链、写文件），只依赖 domain 与 repository
internal/components/     可复用的 UI：占位空态，datepicker / pagination / selectbox / semantic 子包
internal/pages/settings/controls/  只属于设置页的分组卡片 / 设置行
internal/domain/         纯领域规则与数据形状：金额、记录、账本、事件、日记、待办、股票
internal/repository/     页面使用的数据契约（接口 / 查询 / 错误），不依赖 SQLite
internal/config/         应用配置：形状、默认值、读写与归一化（~/.transactions.json）
internal/store/          SQLite 工作空间、schema / 迁移 / 仓储 / 旧库导入
  storetest/             测试用的临时工作空间与种子数据
  schema/                schema SQL（0001_init.sql … 0004_diary_book.sql）
cmd/tr-import/           旧工作空间导入命令
docs/architecture.md     分层规则、旧模块到新包的映射、迁移顺序
```

依赖方向是单向的：

```text
main → desktop → pages → components
                   ↓
             repository ← store
                   ↓
                 domain
config 不依赖 UI；domain 不依赖 UI、数据库或操作系统；repository 只依赖 domain。
```

规则：页面只 import `ui`（以及本工程的 `components` / `domain` / `icons` / `repository`）；
页面不直接 import `internal/store`，也不持有 `ui.C`、`ui.Box` 或 `gfx.Canvas`；后台工作走
`ui.Effect` 或 `Invalidator`，不从页面函数里起 goroutine；坐标与像素只交给布局，不手算
`gfx.Rect`。

导航是 Sidebar 的两级结构：一级功能项展开自己的子功能，同时收起其他一级项；没有子功能的
功能（事件、日记）就是单独一项。页面用 `Tabs() []string` / `Tab() int` / `SelectTab(int)`
暴露子功能，内容里不再画 `ui.Tabs`。

## 迁移顺序

1. **外壳**（已完成）：窗口、标题栏、导航、主题切换、设置页、页面骨架。
2. **领域 + 存储**（已完成）：`internal/domain` 核心形状；`internal/store` 用
   `modernc.org/sqlite` 打开工作空间，schema v1→v2 与迁移、账本 / 记录 / 分类标签 / 模板 /
   图表仓储。
3. **记账**：记录 / 分析 / 标签 / 模板已接工作空间（记录：SQL 范围过滤 / 排序 + 内存分页；
   分析：图表持久化、折线 / 饼图 SQL 聚合、曲线条件与合计；标签：分类标签 CRUD / 计数 /
   排序；模板：CRUD / 排序 / 记录弹窗下拉）；记录的条件筛选（标签 / 描述）还没接入列表。
4. **待办**：卡片、四象限、六档紧急度 / 重要度、进度记录、历史。
5. **事件**：事件画廊、图片资产、实况照片。（日记已完成：树、心情、正文、导入导出。）
6. **股票**：账户、持仓、成交记录、轮次、费用算法、统计、天梯、设置。
7. **设置与数据**：配置持久化、通用设置与五个功能开关、工作空间打开与旧数据导入已完成；
   网络代理、数据备份 / 恢复、更新检查还在后面。
8. **打包**：`viva package`、图标、单实例、托盘、发布流程。

每一阶段先对照旧工程的 E2E / 单测确认行为，再动下一步。**不要在没有行为依据时猜功能。**

## 数据契约（不可漂移）

- 金额恒为整数「分」（字段以 `Cents` 结尾）；股票价格恒为整数「厘」（1/1000 元，字段以
  `Milli` 结尾）；时间恒为 Unix 秒（`At`）；日期恒为本地 `YYYY-MM-DD`（`Date`）。
- 配置是用户数据：生产 `~/.transactions.json`，开发 `~/.transactions.dev.json`；升级 / 降级都必须保留未知键。
- 每个工作空间一个独立 SQLite 数据库；图片资产在工作空间 `data/assets` 下，引用相对路径。
- 新库 schema 版本在 `app_meta.schema_version`，从 1 连续递增；迁移升级前写
  `transactions.db.bak`。v2 增加 `chart` 表（图表配置与排序），v3 增加 `diary_entry`
  （最初每账本每天一篇），v4 把日记从账本上解绑：新增 `diary_book`（日记本：`book_id` / 名称 /
  描述 / 排序），`diary_entry` 换成 `book_id`、`(book_id, date)` 唯一——v3 里每个有日记的账本
  迁移成一本同名日记本，没有日记的工作空间得到一本「默认日记本」。（旧库识别不看 `diary_entry`，
  它从 v3 起是自己的表；只看只有旧工程才有的股票 / 事件表。）新库不建股票表，也不建立
  股票与账本的关联；旧库通过导入工具复制记账表与 chart，不直接打开。
- 打开工作空间只做 `PRAGMA quick_check`；全量 `integrity_check` 只用于备份与恢复两个显式动作。
  库坏了就报告并引导从备份恢复，不自动修复、不静默建空库。
- 恢复必然重启；恢复前的当前数据先另存为 `pre-restore-*`；手动备份是「库 + assets」目录，
  自动备份是只含库的单文件，两者各自只裁剪自己产生的最近 7 份。
- 图表预设（月度 / 年度）只可删除，不可改名 / 改粒度；自建图表整行回写。
- 联网能力仅限股票行情、涨停池和更新检查，且都可由用户配置 HTTP 代理；其余功能离线可用。
- 完整条目见旧工程 `AGENTS.md` 的「数据契约」一节；迁移时逐条对照，不要凭记忆简化。

## 验证

```sh
gofmt -l .                          # 必须没有输出
go vet ./...                        # 必须通过
viva vet                            # 页面级错误（未加 Key 的循环按钮等）
go test ./... -count=1              # 无窗口驱动测试 + 领域单测
```

页面行为用 `app.NewDriver` 驱动真实 painter / 输入 / 焦点来验证（见
`internal/desktop/desktop_test.go`），不要为了看一眼界面就开真实窗口；需要截图或
布局检查时用 `building-with-viva` skill 里的 headless 协议。

## 提交

**每完成一次修改就提交一次。** 不要把多组改动留在工作区里等下一轮，也不要把不相关的
改动塞进同一个提交。

```sh
gofmt -l . && go vet ./... && viva vet && go test ./... -count=1
git status --short                  # 只应有本次要提交的文件
git add -A && git commit -m "feat(scope): 中文摘要"
```

- 提交信息沿用旧工程的 `type(scope): 中文摘要`：`feat`、`fix`、`style`、`docs`、
  `refactor`、`test`、`chore`。
- 一个提交只做一件事；初始化、迁移阶段、布局调整各算一件。
- 截图、文档等产物与对应改动一起提交，不单独留下未跟踪文件。
- `build/`、`*.exe` 已在 `.gitignore`；参考目录 `D:\github\{viva,motty,Transactions}`
  在本仓库之外，绝不 `git add` 它们。
- 每轮工作结束前 `git status --short` 必须为空。

## Viva 速查

下面是 Viva 的写作方式。框架版本以 `go.mod` 为准（v0.6.5）；页面代码如果不确定某个
API，先 `go doc github.com/vivaui/viva/ui`，不要凭名字猜。命令：

```sh
viva dev .          # run with hot reload: save a file, the window reloads and keeps its state
go test ./...      # tests drive the real application with no window (see Testing)
viva vet            # the page mistakes that compile but do the wrong thing (see Mistakes)
viva build          # a release program in build/
viva help           # every command
```

## How a window works

The window's page is a function, `page(c ui.C)` in `internal/desktop/desktop.go`, which the
shell's `Run` paints (`ui.NewPage` + `app.RunCanvas`).
It is called again after every event that could change the picture. It draws the whole
window from your state, and it may be called many times, so it must be cheap and must not
change state, with one exception: a click is handled where its control is declared (below).
Nothing draws unless an event, `Invalidate`, or a running animation asks for a frame; there
is no frame loop to write.

State lives in a struct (`Model` in `internal/desktop`). The page reads it. Never keep a `ui.Box`,
a `ui.C`, a `gfx.Canvas` or a closure given to `Children` after the page returns; `Box.Valid()`
says whether one still names an element of the current paint.

## Pages: ui.C

Every element is a function of `ui` that takes the page's `c` first: layout, text, controls,
overlays, lists, and your own components. What follows the call is the same chain as below
(`.Pad`, `.Gap`, `.Grow`, `.Children(func() { ... })`). A page imports `ui` alone: colors, geometry and the
theme are there too (`ui.RGB`, `ui.Color`, `ui.Rect`, `ui.Theme`, `ui.LightTheme()`). Package `gfx` is for
what is below a page: painting, input events, keys and identities. Controls take the value they change:

```go
func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
		ui.TextField(c, &m.name, "Name", "Your name") // m.name is a string; typing needs nothing more
		ui.Switch(c, &m.notify, "Notify me")          // a click flips m.notify
		ui.Choice(c, &m.theme, themes, nil)           // m.theme is one of themes ([]string here; else pass a label func)
		ui.Select(c, &m.size, sizes, nil)             // a drop-down; its open state is its own
		ui.Scroll(c).Grow(1).Children(func() {
			for i := range m.hosts {
				h := &m.hosts[i]
				ui.Row(c.KeyInt(h.ID)).Gap(8).Children(func() { // Key a row by its item: it keeps focus as rows move
					ui.Checkbox(c, &h.Done, h.Name)
					if ui.Button(c, "Delete").Ghost().Clicked() {
						c.After(func() { m.remove(h.ID) }) // changes the list: after the paint
					}
				})
			}
		})
		if ui.Button(c, "Save").Clicked() { m.save() }
	})
	ui.Modal(c, &m.confirm, "Delete host?", func() { /* body */ }, func() { /* footer buttons */ })
	ui.Popover(c, anchorBox, &m.menuOpen, func() { /* the panel */ })
}
```

- `c.Key("text")` / `c.KeyInt(id)` name the next element (a row); `c.Part("word")` names a part of a
  component. Everything else is named by its place in its parent: no names to declare.
- Bind fields of the model (`&h.Done`). A copy (`v := m.viewed[id]; ui.Checkbox(c, &v, ...)`) works too:
  write it back on `.Changed()`.
- `Changed()` reports the paint in which a bound control changed its value; `Clicked()` on a `Button`.
  `Popover` and `Modal` set their `*bool` to false when dismissed (Escape, a press outside, the close
  button) and give the focus back.
- Long lists: `ui.List(c, &m.list, len(items), func(row ui.ListRow) { row.Element.PadX(8).Children(...) })` declares only the rows
  on screen; style the row with `row.Element`, declare what is in it with `row.C`, name the list with `.Label("Files")`. A click
  chooses a row (`m.list.Selected()`, `Changed()`), the arrows, Home, End and Page Up/Down move, Enter or a double click is
  `m.list.Submitted()`, and `m.list.Label = func(i int) string` makes typing a row's first letters move to it (`m.list.Name = func(i int) string`
  is the name assistive technology reads when a row speaks more than its text, and `m.list.NoTypeahead` leaves typed keys to the page).
  `m.list.HasFocus()` says the keyboard is on its rows. Set `m.list.Selection =
  &m.chosen` (a `ui.Selection`, held by `m.list.Key`) to let the user choose several: Primary-click toggles, Shift extends, Primary+A
  chooses all. `m.list.ScrollTo(row, ui.ListCenter)`, `ScrollIntoView`, `ScrollToEnd`, `Visible()` and `AtEnd()` move and read it; a
  row it has not measured is placed from the rows it has, so a scroll request settles over the few paints it asks for itself.
  `box.ClickModifiers()` and `box.DoubleClicked()` read how a click on any element came. Do not number rows or keep a selected
  index of your own. `row.Element` is a Row, so an icon, a name and a size lie side by side. `m.list.Header = func(i int) bool`
  makes some rows section headings: the heading of the section at the top stays there until the next pushes it away (give it a
  Background; it is not chosen, focused or typed to, and the keys pass over it). `m.list.Reorder = func(rows []int, to int)` lets
  the user drag rows (the chosen ones when the row dragged is chosen) and calls it with the rows and the row they go before; move
  them there, in the model. `m.list.RowWidth = 900` makes the rows at least that wide and the list scrolls sideways (the
  horizontal wheel, or the wheel with Shift); a column header above it follows with
  `ui.Row(c).Clip().Children(func() { ui.Row(c).Width(900).ScrollWith(&m.list).Children(titles) })`.
- `.Tooltip("Delete")` on a control is the tip shown while the pointer rests on it (and its description).
- A menu of actions off a button: `ui.Menu(c, more, &m.menu, func(menu *ui.MenuItems) { if menu.Item("Rename").Clicked() { ... } })`;
  `menu.Separator()`, `menu.Label("Group")`, `.Destructive()`, `.Disabled(x)`. Choosing an item closes it.
  Typing a letter goes to the item it begins. `menu.Sub("Move to", func(to *ui.MenuItems) { ... })` is a submenu
  (opens as the pointer rests on it, on a click, or on Right/Enter/Space; Left goes back), in a ContextMenu too.
- A right-click menu: `ui.ContextMenu(c, row, func(menu *ui.MenuItems) { ... })` (Shift+F10 opens it too); items
  take `.Icon(icon)`, `.Keys(ui.Primary, "c")` (shown, not bound), `.Checked(on)` and `.Chosen(on)`.
- A search field: `ui.TextField(c, &m.query, "Search", "").Icon(search).Action(cross, "Clear")`, and
  `.ActionClicked()` for the button.
- A side panel: `ui.Drawer(c, &m.open, "Title", ui.DrawerSpec{Side: ui.DrawerLeft, Size: 320}, body, footer)`;
  an empty title gives the body the whole panel, `NonModal` keeps the keyboard on the page, `Swipe` adds a
  handle that a drag toward the edge closes it by (past half its size, or thrown).
- To read or move a page's scroll area: `area := ui.Scroll(c).Grow(1)` then `area.ScrollState()` has `Offset`, `Content`,
  `Viewport`, `Max()`, `Pinned()`, `Dragging()`, `ScrollTo(px)` and `ScrollToEnd()` (also for the area a `ui.List` or `ui.Tree` returns); ask for it
  in each paint and do not keep the pointer. `.Sideways()` scrolls left and right too (`OffsetX`, `ContentWidth`, `ViewportWidth`,
  `MaxX()` and `ScrollToX(px)`), and `.Scrollbars()` draws draggable bars over the overflowing edges. The bars are
  the hover style: they appear while the pointer is over the area, and `.ScrollbarSize(px)` sets their thickness
  (a right `Pad` leaves the gutter they sit in instead of covering the content). `PageConfig.ScrollbarSize` sets the
  page's default thickness for every bar that does not set its own.
- `ui.Choice` (a segmented single choice) and `ui.Tabs` are one tab stop each; the arrow keys, Home and End move to an
  option and choose it, so do not wire `components.SegmentedControlKeyDown` or `TabsKeyDown` yourself.
- A tree: keep a `ui.TreeState` with `Node` describing each visible row, `ui.Tree(c, &m.tree, n, label, nil)`,
  then `m.tree.Toggled()` (open or close that branch, give the tree its new rows) and `m.tree.Chosen()`.
  Typing the start of a name moves to that row. To drag rows, set `m.tree.CanDrop = func(from, to int,
  where ui.TreeDrop) bool` (`TreeDropBefore`, `TreeDropInside`, `TreeDropAfter`) and read `m.tree.Dropped()`;
  change the rows in `c.After`, since the paint that reports the drop is declaring them.
  `m.tree.RowID(i)` (`DisclosureID(i)` for the chevron; `ListState` has `RowID`) is a row's identity, to focus it or find its hit.
- Any other box drags and drops with a `ui.DragState` in the model: declare a `Hit` box with
  `.DragSource(&m.drag)` and another with `.DropTarget(&m.drag, accept, drop)`. The page routes the pointer;
  `m.drag.Dragging()`, `Source()`, `Target()` and `At()` paint the gesture, `accept(source, target)` decides,
  `drop(source, target)` runs on release, and Escape lets go. A press that does not move is still a click.
  Save the callbacks in the model and do not declare the boxes in a `Memo`. Inside a
  `ui.Control`'s `Draw`, a part (a card of a custom grid) is a source or target of the same state with
  `d.DragSource(&m.drag, part, r)` and `d.DropTarget(&m.drag, part, r, accept, drop)`, declared beside
  `d.Canvas.Hit(part, r)`; a target part needs no `Hit`.
- A long list to choose from by typing: `ui.Combobox(c, &m.zone, zones, Zone.String, "Time zone")` (Select's
  arguments plus a label); it filters as the user types. `.Clearable()` adds a clear button, `.Empty("No zone")`
  is what an empty list says, `.Creatable("")` offers what was typed as a new choice, read with `.Created()`.
- A button with an icon: `ui.Button(c, "New host").Icon(plus)`; `.Icon(trash).IconOnly()` for the icon alone.
- Toasts: keep a `ui.Toaster` in the model, `m.toaster.Show("Saved", "", ui.ToastSuccess)`, and declare
  `ui.Toasts(c, &m.toaster)` once, last in the page. `ShowAction(title, "", kind, "Undo")` adds a button,
  `m.toaster.Acted()` (after `Toasts`) reports its press; a `ui.ToastLoading` toast stays until
  `m.toaster.Done(id, title, "", ui.ToastSuccess)` replaces it; `Toaster.Position` picks the corner.
- `ui.TextField(...).AutoFocus()` focuses a field as it appears; `.Submitted()` is Enter pressed in it.
  `ui.TextField(...).Secure()` is a password field. Focus that lives in the model is
  `ui.FocusBind(ui.TextField(...), &m.focus, FocusSearch)`: setting `m.focus` moves the focus, a user's
  focus writes back, and `ui.FocusedValue(c, &m.focus)` reads the committed one. The element needs a
  control or a Hit, and the binding is not for a Memo.
- Keyboard shortcuts are declared where they act: `if ui.Shortcut(c, ui.Primary, "s") { m.save() }` (Primary is
  Command on macOS, Control elsewhere); `list.Shortcut(0, "delete")` on a container or control acts only while the
  focus is in it, the innermost one winning. A chord without Primary, Ctrl or Alt is left to a field being typed in.
  `ui.TextArea(c, &m.notes, "Notes", "").Height(120)` is several rows.
- A control of your own (terminal, editor, drawing canvas): `ui.Control(c, "Label", &m.handlers).Grow(1).Draw(m.draw)`;
  `ui.Handlers` has Key, Pointer, Scroll and, with Role RoleTerminal or RoleTextField, Text for typing
  and input methods. Keep the Handlers and the draw function in the model; declare no identities.
  A control with several interactive parts (the stars of a rating, a graph's nodes) declares them as it draws:
  `d.Canvas.HitWithOptions(m.handlers.Parts().At(i), rect, "Star 3", opts)`; a key, press or wheel on a part reaches
  the same Handlers with the part as the event's Target (`event.Target.In(m.handlers.Parts())` gives i). A printable key
  typed while the control or a part has the focus reaches Key with its Text (Key zero), so typeahead needs no extra wiring.
- A toolbar, or any row or column of controls that is one Tab stop with arrow keys between them:
  `ui.Row(c).FocusGroup(ui.FocusHorizontal).Children(...)` (`FocusVertical`, `FocusBoth`); Tab reaches the member that had the
  focus last, the arrows and Home/End move among the members (wrapping), and a text field, combobox or slider inside keeps its own.
- A control with a look of your own: `ui.ButtonBase`, `CheckboxBase`, `SwitchBase`, `ToggleBase`, `RadioBase`, `TabBase`,
  `SliderBase`, `SelectBase` and `TextFieldBase` are the controls without their look, bound like their dressed forms; give them
  Background, Border, Radius and Draw (and Children, for the row-shaped ones). Do not copy a control to restyle it.
- A part that rarely changes: `ui.Memo(c.KeyInt(id), *item, func() { ... })` is copied from the last
  paint while that value compares equal: pass the row (or a comparable key of what the function
  reads), not a counter you must remember to bump.
- Your own component is a function of the same shape: `func Card(c ui.C, title string, body func()) ui.Box`.
  Its own small state (whether it is open) is `ui.State[bool](c, "open")`; the application's state stays in the model.
- A settings form: tag the struct's fields (`form:"Label"`, `choices:"options,nameFunc"`, `placeholder:"..."`)
  and run `go tool viva form` (or `//go:generate go tool viva form`); it writes `Form(c ui.C) ui.Box`.
- An independent window is a page: `ui.Run` or `ui.NewPage(...).Config(base)` wires its input. A pane of a window is a keyed container in the same page, not a second `Page`; do not draw a page into a slot in application code.
- `ui.ID(c)` names a custom control of your own (see Identities and clicks); `c.Theme()` is the theme; `c.Bounds()` and `c.Canvas()` are for measuring.
- In a test, find controls by their name (`Hit.Label`), as `internal/desktop/desktop_test.go` does.

## Mistakes that compile (and how each is reported)

Run `go vet ./... && viva vet && go test ./...` before you finish. `viva vet` finds these without
running anything; a test under `app.NewDriver` panics on them where they happen, with the fix.

| You wrote | What goes wrong | Write instead |
| --- | --- | --- |
| `for ... { ui.Button(c, "Delete") }` with no Key | a row added above moves the click to another row | `ui.Row(c.KeyInt(item.ID)).Children(func() { ui.Button(c, "Delete") })` |
| a `ui.C` in a struct field or a global | it is valid only inside the page function | keep the `*ui.Page`, pass `c` down |
| deleting from the list inside its loop on `Clicked` | the loop still runs over it | `c.After(func() { m.remove(id) })` |
| `ui.Modal(c, &open, "", ...)` | panics: a dialog needs a title | give it one; it names the dialog for screen readers |
| two rows with `c.Key("x")` in one parent | panics | key rows by something unique (an ID) |
| a `ui.Memo` whose function reads something its value does not cover | the part shows stale content | pass the row (or a comparable key of what the function reads); `ui.MemoCheck(true)` in a test panics on it |

A line, or the line above it, with `// viva:ignore unkeyed-button` (or `kept-context`, `memo-value`) silences
one finding you have checked.

## Laying a page out (package github.com/vivaui/viva/ui)

Describe the page; do not compute coordinates.

```go
func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
		ui.Text(c, "Title").FontSize(20).Bold()
		ui.Paragraph(c, description, 3).Muted()          // wraps; at most 3 lines
		ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
			ui.Text(c, "Notify me")
			ui.Spacer(c)                               // pushes what follows to the far edge
			ui.Switch(c, &m.notify, "").Small()
		})
		ui.Scroll(c).Grow(1).Gap(4).Children(func() { /* long content */ })
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c, "Cancel").Outline()
			if ui.Button(c, "Save").Clicked() { m.save() }
		})
	})
}
```

- The window's `*ui.Page` is made by `ui.Run` or `ui.NewPage` and lives with the
  window; the page function only declares with `c`. A `ui.C` and the `Box`es of one
  paint are valid only while that paint runs.
- Containers: `Row` (left to right), `Column` (top to bottom), `Stack` (on top of each
  other), `Grid(minCell)` (as many equal columns as fit), `Flow` (a row that wraps onto
  new lines; `LineGap` sets the space between lines, else it is `Gap`), `Scroll` (a clipped
  column taller than itself). Leaves: `Text`, `Paragraph`, `Wrap`, `Spacer`, `Divider`, `Icon`,
  `Image(bitmap)` (a `gfx.Bitmap` such as `app.FileIcon` returns, 16 px square unless sized), `Button`, `Checkbox`, `Switch`, `Slot`, `Progress(fraction)` (a bar; `.Indeterminate()` for unknown length),
  `Spinner()`, `Splitter(&state)` (see Keys, drag and the mouse), `Slider(&state)` (a number picker, see below).
- Size: nothing = as big as its content; `Width`, `Height`, `Size` fix it; `FillWidth`,
  `FillHeight`, `Fill` take the room; `Grow(n)` shares free room along the container;
  `Shrink(n)`; `Basis(px)` is what an element starts from before free room is shared;
  `WidthPercent`, `HeightPercent` take a share of the container; `MinWidth`, `MaxWidth`,
  `MinHeight`, `MaxHeight` bound it (room a capped element cannot take goes to the others,
  as in CSS: `Grow(1).MaxWidth(470)` beside a `Spacer()` works); `MaxWidthPercent(70)` and `MaxHeightPercent` cap at a share of the container (a chat bubble).
- Space: `Pad`, `PadX`, `PadY`, `PadTop`, `PadRight`, `PadBottom`, `PadLeft`, `PadSides(top,right,bottom,left)`;
  `Margin`, `MarginX`, `MarginY`, `MarginTop`, `MarginRight`, `MarginBottom`, `MarginLeft`, `MarginSides`;
  `Gap` between children. `Pad`, `Border` and `Background` work on a `Text` or `Paragraph` too
  (a chip, a badge, a chat bubble). Every length is a `float32`: `Width(float32(n))`, never an `int`.
- Place: across a container children stretch by default; `Items(ui.Center)` or
  `ui.Start`/`ui.End`/`ui.Stretch` changes that; `ui.Baseline` lines a Row's children up
  on their first line of text; `Self(...)` for one child;
  `Justify(ui.Center|End|Between)` places what is left along it; `Center()` does both.
- Look: `Background`, `Radius`, `Border(width, color)`, `Shadow(gfx.ShadowMD)` (a drop shadow under the fill; `ShadowXS` to `ShadowLG`), `Clip`. A circle (an avatar, a dot) is a
  square box with `Radius(size/2)` and `Center()`: `ui.Row(c).Size(32, 32).Radius(16).Background(theme.Primary).Center().Children(func() { ui.Text(c, "AB") })`.
- Effects, all typed and checked by the compiler (there is no class string): `Ring(3, c)` draws a line around the
  outside without taking room (a focus ring); `Opacity(0.5)` fades the element and everything in it (it is still laid out
  and clickable); `Gradient(ui.ToBottomRight, from, to)` and `GradientStops(angle, gfx.Stop(0, a), gfx.Stop(1, b), ...)`
  fill the element, corners included, over `Background` and under `Border`; `Dashed()` breaks the `Border` line into
  equal dashes and gaps (a drop target's frame), and `Look{}` has `Dashed(bool)` too.
- States: an element with an identity (`Hit`, which also makes it focusable) changes with `Hover(ui.Look{}.Background(theme.Hover))`, `Pressed(...)` and
  `Focus(ui.Look{}.Ring(3, theme.Ring))` (keyboard focus, as `:focus-visible`), plus `Focused(...)` for any focus (a click in a text field focuses it
  without making it visible; a group box that frames its field follows the field with `StateFrom(field)`). A `Look` lists only what differs from rest and has `Background`, `Border`,
  `Color`, `Ring`, `Shadow` and `Opacity`; looks combine field by field (Focus, then Hover, then Pressed win).
  `Transition()` fades between them over the theme's motion and schedules nothing while at rest. `StateFrom(row)` makes a
  child follow its row's state: a pencil that shows on hover is
  `ui.Icon(c, edit).Opacity(0).StateFrom(row).Hover(ui.Look{}.Opacity(1))`.
- Finished, themed examples to copy (settings row, list row with a hover action, avatar, badge, stat card, gradient hero, icon button, empty state): <https://github.com/vivaui/viva/tree/main/examples/recipes>, with guidance in <https://github.com/vivaui/viva/blob/main/docs/skills/building-with-viva/recipes.md>.
- Icons move inside their box, not the box: `ui.Icon(c, glyph).IconTransform(gfx.IconTransform{Rotate: radians, Scale: 0.9, Offset: gfx.Point{X: 2, Y: 0}})`
  turns and scales the glyph about the center of its box and moves it after that, while the row keeps its place, its
  size and its hit region. The zero `gfx.IconTransform` is the still icon, and a paint that passes one costs nothing.
- A glyph's parts move on their own: `glyph.Contours()` splits the glyph into one icon per contour, and
  `IconTransform.Parts` takes one `gfx.IconPart` per contour that moves (`Contour: n`, `Offset: gfx.Point{Y: 2}`), so
  that contour moves while the others stay. Contours are numbered in drawing order, so a regenerated glyph can renumber
  them; pin the contours you address with a test that paints the part and checks where it lands. Filled `Layers` are
  not contours: they paint once, unchanged, while parts move.
- A hover gesture has a small caller-owned driver: keep a `components.IconHover` and declare
  `id := ui.ID(c); ui.Icon(c, glyph).Hit(id, "Action").BindIconHover(ui.BindIconHover(&m.hover, id))`. It waits for the pointer
  to rest, plays once, and finishes even if the pointer leaves; `IconHover.Progress` exposes the linear progress
  when a glyph needs its own motion. `BindIconHover` only moves the glyph inside its box.
- Window: in a `WindowCustomTitlebar` window nothing moves the window unless you say where: `ui.Row(c).Height(36).MovesWindow().Children(...)` on the top bar
  (controls inside it still take their own clicks; a menu or dialog open over it is dismissed by a click there and the window moves once the pointer drags).
- Text: `FontSize`, `Weight`, `Bold`, `Color`, `Muted`, `Mono`, `MonoFace`, `TextCenter`, `TextEnd`, `TextDirection`, `LineHeight`;
  `Ellipsis()` keeps a Text to one line and ends it with … when it does not fit; `MaxLines(n)` lets a Text
  break into at most n lines (the last ends with …), `ui.Wrap(c, text)` into as many as the width needs. Lines
  break where Unicode (UAX #14) allows: at spaces, after hyphens and slashes, between CJK characters (kinsoku
  rules apply), between the syllables of Thai, Lao, Khmer and Myanmar, at the spaces of Korean; a word wider than
  the line breaks after a `/ - _ .` or between characters, never inside an emoji, a conjunct or a letter with its
  marks. Hebrew, Arabic and other right-to-left text is reordered (UAX #9), aligned to the start of its paragraph
  (the right edge; `TextEnd()` picks the other) and given the paragraph's direction on every line; `TextDirection`
  overrides it. The height follows the width. Viva measures text with its own metrics,
  the same on every platform, and draws the platform's glyphs at that width: a button as wide as its label, a tab, a
  table column sized to content and every line break are the same on macOS, Windows and Linux, so do not size by a
  platform's font (or special-case a platform) to make text fit.
- Buttons (the plain one is the primary; there is no `Primary()`): `Secondary`, `Outline`, `Ghost`, `Link`, `Destructive`, `DestructiveOutline`,
  `XSmall`, `Small`, `Large`, `IconOnly`, `Disabled(cond bool)` (so `ui.Button(c, "-").Disabled(m.n <= 0)`, no `if`).
- A control call returns a `ui.Box` (there is no `ui.Box` constructor); keep it when you need it later: `filter := ui.Button(c, "Filter")`.
  A bound Checkbox, Switch, Choice or Tabs writes its value where it is declared, so a click you do not read still changes it;
  read `.Changed()` for work that follows the change: `if ui.Switch(c, &m.auto, "Auto-scroll").Changed() { m.persist() }`.
  `Label` is the name a screen reader or a test finds the control by, so give one to every control that has no text of
  its own, and put the visible caption in a `ui.Text` beside it. A header row of named controls is a `Row` of `Button`s
  (`.Ghost()`) or `Hit` boxes; each is found by its label.
- Text fields that just work: `ui.TextField(c, &m.name, "Name", "placeholder")` binds a string of the model, and
  `ui.TextField(c, &m.query, "Search", "")` binds a `ui.Field` when the program reads the caret or selection too
  (`m.query.Secure = true` for that field). Typing, caret, selection, clipboard and input methods work; the page's
  `Config` gives the platform the editor, so no `Config.TextInput` is needed for them. Read a string as `m.name`;
  read a field with `m.query.Text()` and set it with `SetText("x")`. Do not build `text.Editor`s or snapshots by hand.
  In a `Row` a field has no width of its own: give it `Grow(1)` or `Width`, or it is invisible.
  Enter in a field: `if ui.TextField(c, &m.draft, "Message", "Type").Submitted() { m.send(); m.draft = "" }` (true only in the
  repaint that delivers Enter, not for a click; `OnKey` is not needed). The label (first string) is never empty.
- Several rows: `ui.TextArea(c, &m.notes, "Notes", "").Height(120)` (Enter is a new line). Arrow keys and Page Up/Down
  move by row while the platform edits it, and the page routes them; no `OnKey` is needed.
- Other fields: `ui.TextField(...).Icon(plus)` puts an icon before the text, `.Action(cross, "Clear")` a button at its end
  (`.ActionClicked()` reads it), `.AutoFocus()` takes the focus as it appears and `.Secure()` makes it a password field.
  A search box is a TextField with `.Icon(search)` and `.Action(cross, "Clear")`. `ui.Choice(c, &m.theme, themes, nil)` is a
  segmented choice and `ui.Tabs(c, &m.tab, tabs, nil)` a tab strip; each writes the chosen value itself. For a control
  that only `components` paints, declare a `ui.Slot(c)` and paint it in the slot's `Draw` callback (see Components).
- Controls with your own look: every control with the theme's look has a Base, the same control without one, and the look is then
  the element's own. `ui.ButtonBase(c, "name")` (give it `Children`), `ui.TextFieldBase(c, &m.cell, "Cell", "")` for a
  single-line field without a look, and `ui.TextArea` for the multi-row field, `ui.CheckboxBase(c, &m.on, "name")`, `ui.SwitchBase` and `ui.ToggleBase(c, &m.bold, "Bold")` the
  two-state controls, `ui.SliderBase(c, &m.volume)` a slider whose thumb you draw where `state.Fraction()` says, and
  `ui.TabBase(c, &m.tab, tab, label)`, `ui.RadioBase(c, &m.mode, option, label)` and `ui.SelectBase(c, &m.size, sizes, nil)` the items of
  composites. A select with your own look: `sel := ui.SelectBase(c, &m.size, sizes, nil)`, draw the trigger in its `Draw`, and
  when `list, ok := sel.Popup()` draw the open list too (`sel.Part(i)` is option i's rectangle); its state, `Picked`, popup, focus
  scope, Escape and type-ahead are the select's.
  Dress any element with the calls it takes (`Pad`, `Border`, `Radius`, `Background`, `Shadow`, `Hover`, `Focus`; on a text field also
  `FontSize`, `Weight`, `Color`, `Mono`/`MonoFace` and `LineHeight`), and draw what those cannot say with
  `.Draw(func(d *ui.DrawContext) { ... })` (after the background, before the children) or `.DrawOver(...)` (after the children);
  `d.Canvas`, `d.Rect` (laid out, so no `Slot` and no `Rect()` after the paint), `d.State` (a `gfx.State`: `Hovered`, `Pressed`,
  `Focused`, `FocusVisible`, `Disabled`, `Checked`; `Box.State()` is the same while declaring), `d.Label` and `d.Fade(color)` are yours.
  A Draw callback only paints and may run more than once a paint. The theme's look is a pure function you can start from:
  `components.TextEntryRecipe`, `CheckboxRecipe`, `SwitchRecipe`, `ButtonRecipe`. `ui.TextField` and `ui.TextArea` take `Radius`,
  `Border`, `Background`, `Shadow` and `Pad` too: what you set replaces the recipe's, the rest stays, so `ui.TextField(c, &m.x, "X", "").Radius(0)` is a square shadcn field.
- Interactive rows and cards: `ui.Row(c).Pad(8).Hit(ui.ID(c), name).Children(func() { ui.Text(c, name) }).Clicked()`.
  `ui.ID(c)` names the next element of the page (a `Hit` box, a `Control`, a recipe); a `c.Key`/`c.KeyInt` before it names
  a row, and `c.Part("word")` names a part of a component. `HitWith(id, label, gfx.HitOptions)` adds `Cursor`
  (`gfx.PointerCursorResizeHorizontal`, `gfx.PointerCursorHand`, ...), `Selected`, `Disabled` and `Role`. `Hit` only makes a
  pointer target and an accessibility name; it paints nothing, so a container with `Hit` and no `Children` (and no size or
  background) panics at the first paint. A part of a row a component owns, such as a Tree row's name that carries the
  row's `ContextMenu`, is `ui.Text(c, name).LabelFor(m.tree.RowID(i))`: a label of the row, which a click, the hover
  and a right click reach as the row, where a `Hit` would be a control of its own that takes the row's clicks.
- A segmented choice among labels: `if ui.Choice(c, &m.theme, themes, nil).Changed() { m.saveTheme() }`; the choice writes
  the option itself, so `m.theme` is the selected value (`Theme.String` names options that are not strings).
  `ChosenIndex()` reports the option index a click chose on a `ui.Choice` strip (the call the bound form uses itself), for
  a caller that needs the index. `ui.Tabs(c, &m.tab, tabs, nil)` is a tab strip with the same write; declare the active
  tab's content after it, for the value it wrote, and `TabClicked()` reports the tab index a click chose when you need the
  index rather than the value. A vertical divider is just `ui.Divider(c)` inside a `Row` (there is no `.Vertical()` on a Divider).
- A table: the data is a `ui.TableRows` in the model (`[][]string`, passed as `&m.data`) or your own pointer type with
  `Len() int` and `Cell(row, col int) string`; not a plain slice of structs. `ui.Table` makes its row and header scopes
  from the state when the spec leaves them zero:

  ```go
  columns := []components.TableColumn{
      {Width: 160},
      {}, // no width: the last column takes the rest
  }
  t := ui.Table(c, &m.table, ui.TableSpec{Headers: []string{"Name", "Age"},
      Sortable: []bool{true, true}, Source: &m.data, Striped: true, Columns: columns})
  if i, ok := t.RowClicked(); ok { m.table.Select(i) }       // Selected() is the row or -1, ClearSort() unsorts
  if col, ok := t.HeaderClicked(); ok { m.table.SortBy(col); m.sortData() } // Sort() returns (column, descending, ok)
  ```

  A header cell is clickable only where `Sortable[c]` is true; `Headers` alone draws a title nobody can click. In a
  `ui.Scroll` the table is as tall as its rows and only visible rows are read; with `Height`, `Fill` or `Grow` it scrolls
  its rows under a fixed header and draws its own hover bars: `Scrollbar` names the bar's hit, `ScrollbarLabel` names it
  for assistive technology, `ScrollbarSize` sets its thickness (else the page's `PageConfig.ScrollbarSize`) and
  `ScrollbarGutter` keeps a strip beside the columns for it. An application painting its own vertical bar sets
  `HideHoverScrollbars: true` to omit the anonymous vertical hover thumb only; the horizontal overlay, gutter, wheel,
  and an explicitly identified draggable `Scrollbar` are unchanged. The page gives the wheel to a self-scrolling table under the pointer; give the `app.Config` passed to
  `page.Config` `OnKey: m.table.Key` for Up/Down/Page/Home/End selection (a host that routes its own scroll gives
  `m.table.Wheel` the wheel).
- Tabs: `if ui.Tabs(c, &m.tab, tabs, nil).Line().Changed() { ... }`, then declare the active tab's content under it with an
  ordinary `switch m.tab`. `Line()` is the underlined variant (it draws 5 px under the strip), `Vertical()` stacks the tabs,
  `Disabled(true)` makes them inert.
- A drop-down select: keep the value in the model; whether the list is open is the Select's own state:
  `if ui.Select(c, &m.size, sizes, nil).Placeholder("Size").Changed() { m.load(m.size) }`. The list is placed under the
  trigger (above it when there is no room), painted over everything, and scrolls when it is taller than the window; the page
  routes the wheel to it while it is open. `ui.Combobox` takes the same arguments plus a label and filters as the user types.
- A modal dialog: declare it while it is open, `ui.Modal(c, &m.confirm, "Delete host?", body, footer)`; the title is
  required because it names the dialog for screen readers, and `body`/`footer` may be nil. It is centered over a scrim,
  keeps the keyboard while open and gives it back as it closes; a press on the scrim, the close button and Escape set
  `m.confirm` to false. `ui.ModalWith(c, &m.confirm, "Delete host?", ui.ModalSpec{Description: "...", Width: 400}, body, footer)`
  adds a description, a size, `NoClose` or `InitialFocus`. For a dialog whose identities the application names itself, use
  `ui.Dialog(c, ui.DialogSpec{...})`: `.Header(fn)`, `.Body(fn)` and `.Footer(fn)` declare it and `.Dismissed()` reports
  the closing click.
- Menus, tooltips and select lists: a menu of actions is `ui.Menu(c, anchor, &m.menuOpen, func(menu *ui.MenuItems) { ... })`
  off a button's `Box`; its items, submenus, keys and dismissal are the menu's. `.Tooltip("text")` on the
  control it describes is the tip shown while the pointer rests on it. `ui.Select`, `ui.Combobox` and `ui.Drawer` place their own lists and panels. Your own panel is
  `ui.Popover(c, anchor, &m.open, body)` or `ui.PopoverWith(c, anchor, ui.PopoverSpec{Width: 220, Align: ui.Start}, &m.open, body)`;
  it opens off its anchor, flips when there is no room, closes on Escape or an outside press, and gives the focus back.
  A floating region that is none of these is `ui.Overlay(c, ui.OverlaySpec{Band: ui.LayerPaneChrome, Bounds: r})` (or
  `ui.LayerDrag` for a drag ghost): it takes no room, paints after the page and below Viva's menus and dialogs, and its
  children are page elements. Do not build a popup from a `Slot` and a `Hit`; the page's overlays keep the pointer, focus
  and dismissal a popup needs. A component author who declares the underlying popup places it with `Beside(rect, gap, align)`
  or `BesideBox(box, gap, align)` to the right of an anchor (a submenu), `BesideLeft`/`BesideLeftBox` to its left, and
  `Within(rect)` to keep it inside a region instead of the window; `ui.PopoverWith`'s `Side` is the application-level form of
  the same placement. `Below`/`BelowBox` and `Above`/`AboveBox` place below and above.
- A page of the system's web engine inside the window (WebKitGTK on Linux, WKWebView on macOS, WebView2 on Windows):
  put an `app.WebHost` in `Config.Web`, open the page in its `OnReady` (`host.Open(app.WebViewConfig{URL: ...})`, or `HTML`
  with a `Scheme` that serves your own resources), keep the `*app.WebView`, and in the page give it the room of a slot:
  `ui.Slot(c).Grow(1).NativeView(m.view.ID())`. It fills the slot inside its clips and scrolls with it; menus, popovers
  and dialogs are painted above it automatically. The page talks to the app with `window.viva.postMessage(text)` /
  `viva.onmessage`, and the app with `OnMessage` and `view.PostMessage(text)`. Calls on the host and the page run on the
  window thread. `Open` fails with `app.ErrUnsupported` when the platform or the installed engine cannot host pages: show
  something else then.
- Grid children can cover several cells: `.ColSpan(n)` and `.RowSpan(n)` (columns are equal; the next child goes in
  the first free cell after the previous one, skipping cells a span has taken).
- Take a child out of the flow with `.Absolute()` and place it by edges in its container: `.Top(8).Right(8)`
  (`Bottom`, `Left`; a negative edge reaches outside), `.Inset(0)` for all four, with `Width/Height/Size` or both
  edges of an axis to stretch between them. It takes no room, does not move its siblings, is painted and clicked
  above them, and edges do nothing without `.Absolute()`. A badge on an avatar, a close button on a card, an overlay
  (`ui.Stack(c).Children(...)` holding it), a toast stack at the bottom right of the page
  (`ui.Column(c).Absolute().Right(16).Bottom(16).Gap(8)`).
- Your own painting inside a laid-out page: `ui.Slot(c)` reserves a rectangle, and `Box.Rect()` says where it ended up
  once the page is laid out. Paint it in a `Draw` callback (`Box.Draw(func(d *ui.DrawContext) { ... })`), which is given
  the element's `Rect` in its place, or use the rectangle in the same event pass after the page function returns; do not
  keep the `Box` for a later paint. `c.Canvas()` is the canvas for measuring or reading pointer and focus state, and
  `c.Bounds()` is the rectangle the whole page was laid out in, known before the page returns.
- A slider (volume, zoom, a threshold): a `ui.SliderState` in the model holds the number in `Value` (set it before the first paint), and
  `if ui.Slider(c, &m.volume).Range(0, 1).Step(0.05).Label("Volume").Changed() { m.player.SetVolume(m.volume.Value) }` draws it
  (`Range` defaults to 0..100, `Step` is optional, `Vertical()` makes it tall, `Disabled(cond)` dims it, 16 high and as wide as its column
  or 160 in a row unless given a `Width`). A drag, a click on the track and the arrow keys all keep `Value` inside the range; the page
  routes the focused slider's keys, so no `Config.OnKey` is needed. `Changed()` is true in the one paint that delivers a user's change,
  so call it last in the chain. Do not draw a track from a `Slot` and `Progress` by hand.
- Scrolling needs nothing in the model: `area := ui.Scroll(c).Grow(1)`, then `area.ScrollState()` has `Offset`, `Content`,
  `Viewport`, `Max()`, `AtEnd()`, `Pinned()`, `ScrollTo(px)` and `ScrollToEnd()` (also for the area a `ui.List` or `ui.Tree` returns);
  ask for it in each paint and do not keep the pointer. The page gives the wheel to the area under the pointer (the innermost, or the
  next one out when that is at its end), so side-by-side columns and nested lists each scroll alone. A log or chat that grows at the
  bottom: `.Follow(true)` keeps the area at its end until the user scrolls up (and again when they scroll back down; `.Follow(m.autoScroll)`
  makes it a switch); `Pinned()` says the next layout puts it at its end before `Offset` has caught up, and `ScrollToEnd()` after adding
  the item holds in the paint that grows the content. `.Sideways()` adds horizontal scrolling (`OffsetX`, `ContentWidth`,
  `ViewportWidth`, `MaxX()`, `ScrollToX(px)`), and `.Scrollbars()` draws the draggable bars (hover-revealed; `.ScrollbarSize(px)`
  sets their thickness). An area a paint does not declare forgets
  its offset (a closed dialog, another tab).
- Do not write `gfx.Rect{X: ..., Y: ...}` arithmetic for ordinary parts of a page. It breaks
  when a label gets longer or the window gets narrower; a page laid out with ui does not.

## Component authoring (page functions)

A component is a function of the page, declared where it is used: `func Card(c ui.C, title string, body func()) ui.Box`.
It composes the same `ui` elements as the application, keeps its own small state with `ui.State[T](c, "open")`, and never
declares a `gfx.Tree`, opens a window or holds a `C` past the paint. A hit or a test names it with `ui.ID(c)` (the next
element's identity) or `c.Part("word")` when the name must be stable.

- A control you paint yourself is `ui.Control(c, "Label", &m.handlers)`; `ui.Handlers` receives its keys, pointer, wheel
  and, with `RoleTextField` or `RoleTerminal`, the platform's typing and input methods through `Text`. Give it a size or
  `Grow(1)`, paint it with `.Draw(m.draw)`, and declare its interactive parts inside that Draw with `d.Canvas.Hit(part, r)`:
  an event on a part reaches the same `Handlers` with the part as its target, and `event.Target.In(m.handlers.Parts())`
  gives its index.
- A component that places its children by its own rule uses `ui.CustomLayout(c, &m.layout)` and implements `ui.Layout`
  (`MeasureWidth`, `PlaceWidth`, `MeasureHeight`, `PlaceHeight`). The page keeps identity, focus, hits and clipping; the
  layout owns its children's sizes and positions. Wrap it in `Scroll`, `List` or `Tree` to scroll it.
- Chrome that floats over the page without being a menu or dialog is
  `ui.Overlay(c, ui.OverlaySpec{Band: ui.LayerPaneChrome, Bounds: r})` (or `ui.LayerDrag`); its children are page elements,
  and a `Canvas.Block` over its empty surface stops presses to the page behind it.
- A component from `github.com/vivaui/viva/components` with no `ui` wrapper is painted in a slot: declare `ui.Slot(c)` and
  paint the component's `Paint...` function in the slot's `Draw` callback (`components.PaintInput(d.Canvas, *d.Theme, d.Rect, snapshot)`).
  For a control whose input the page must route, declare its `ui` element or its Base (`ui.ButtonBase`, `ui.TextFieldBase`, ...).
  Do not copy a control to restyle it.

## Keys, drag and the mouse

The page routes the window's keys, pointer, wheel and text input to the controls it declared, so an application writes
no handler for the ordinary cases. `Config.OnKey func(gfx.KeyEvent) gfx.EventResult` still receives the keys the page did
not take, for the control that has keyboard focus (a table's `TableState.Key`, a terminal of your own), so a key handler
on a page where nothing is focused never runs. For "Escape closes this", declare a `Modal`, `Popover`, `Menu` or
`Drawer`; they dismiss themselves, and `OnKey` never sees Escape for an unfocused page. `gfx.EventResult{Repaint: true}`
asks for a repaint, `Handled: true` stops the default handling; it is a struct, there is no `gfx.Handled` constant.

`Config.OnPointer func(gfx.PointerEvent) gfx.EventResult` receives pointer input the page does not model: `e.Kind` is
`gfx.PointerDown`, `PointerMove`, `PointerUp` or `PointerExit`, `e.X`/`e.Y` are window pixels, `e.Target` is the identity
pressed (it stays the pressed one while a drag runs). Two panes with a draggable divider need none of it: keep a
`ui.SplitterState` in the model, give the first pane its `Pos` and declare `ui.Splitter(c, &m.split)` between them (the
pointer is captured, the resize cursor shows, `Mins` bounds both panes, and `SplitterState.Key` given to the config's
`OnKey` adds the arrow keys; `m.split.Dragging()` says a drag is running):

```go
m.split = ui.SplitterState{Pos: 240} // once, in newModel
ui.Row(c).Fill().Children(func() {
	ui.Column(c).Width(m.split.Pos).Children(func() { /* left pane */ })
	ui.Splitter(c, &m.split).Mins(120, 200)
	ui.Column(c).Grow(1).Children(func() { /* right pane */ })
})
```

(`Splitter` in a `Column` divides top and bottom; the first pane then gets `Height(m.split.Pos)`.)

A drag of your own is a `ui.DragState` in the model, declared with `.DragSource` and `.DropTarget` on `Hit` boxes, or with
`d.DragSource` and `d.DropTarget` inside a `Control`'s `Draw` (see Pages); the page follows the press and the release.

## A modal dialog

Use `ui.Modal(c, &m.open, title, body, footer)` or `ui.ModalWith` (see Laying a page out): it dims the page and keeps the
keyboard while `m.open` is true, and Escape, a press on the dim and the close button all set it to false, so there is no
Escape handler to write. For a dialog whose identities the application names, `ui.Dialog(c, ui.DialogSpec{...})` gives you
`Scope`, `DismissID`, `CloseID` and `InitialFocusID`, with `.Header`, `.Body`, `.Footer` and `.Dismissed()`.

## Animation and time

Nothing runs by itself. A paint that must be followed by another (a progress bar, a spinner) calls
`c.Canvas().RequestFrame()` (or `RequestFrameIn(d)`) and keeps asking while it is still moving; when it stops asking the
window goes idle again. `c.Canvas().Now()` is the animation clock (a `time.Duration`): store `began := c.Canvas().Now()`
when the thing starts and derive the state from the difference in the page function, never from a ticker or goroutine.
`c.Canvas().Animate(id, slot, target, gfx.DefaultMotion())` eases a value toward a target for you (hover fades, sliding
indicators). Ask it for `gfx.EaseLinear` and it becomes a clock for a motion with keyframes of your own, each segment
eased by `gfx.CubicBezier(x1, y1, x2, y2)` (the CSS curve): a press that turns an icon and settles it back is one click,
one slot and one map of stop times.

```go
canvas := c.Canvas()
if m.running {
	fraction = min(float32(canvas.Now()-m.began)/float32(3*time.Second), 1)
	if fraction < 1 { canvas.RequestFrame() } else { m.running, m.done = false, true }
}
ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
	if ui.Button(c, "Start").Clicked() { m.running, m.began = true, canvas.Now(); canvas.RequestFrame() }
	ui.Row(c).Gap(12).Items(ui.Center).Children(func() {
		ui.Progress(c, fraction).Grow(1).Height(8)   // ui.Progress(c, 0).Indeterminate() when the length is unknown
		if m.running { ui.Spinner(c) }               // turns by itself; asks for frames only while on screen
	})
})
```

`Progress` and `Spinner` need no `RequestFrame` of yours; the animation you drive from your own state (the fraction above) does.

## Several windows

`app.RunCanvas` runs one window. For more, `app.RunApplication(func(a *app.Application) error {...})` calls its
function once on the event-loop thread; open windows there or from a click, each with its own page, config and canvas.
Make every window a `ui.Page` and give the window the config `page.Config(...)` returns: `w, err :=
a.OpenWindow(page.Config(config), new(gfx.Canvas))`. `w.Close()` closes one (and `w.Closed()` says whether it is gone;
it is true for a nil window); `a.Quit()` ends everything. Keep `newConfig(m)` for the main window so tests still drive
it. To repaint another window after the shared model changed, give it an `app.Invalidator` (`config.Invalidator =
inv`, declared as `&app.Invalidator{}`, the zero value works) and call `inv.Invalidate()`; `Page.Config` makes one when
neither the page nor the base config has it.

```go
err := app.RunApplication(func(a *app.Application) error {
	m := newModel()
	var inspector *app.Window
	m.openInspector = func() { // set before the window opens; called from a click on the event-loop thread
		if inspector.Closed() {
			page := ui.NewPage(ui.PageConfig{View: m.inspector}) // func (m *model) inspector(c ui.C)
			inspector, _ = a.OpenWindow(page.Config(app.Config{Title: "Inspector", Width: 420, Height: 320}), new(gfx.Canvas))
		}
	}
	m.closeInspector = func() { inspector.Close() }
	config := newConfig(m) // ui.NewPage(ui.PageConfig{View: m.page}).Config(...)
	config.OnClosed = a.Quit // closing the main window ends the program
	_, err := a.OpenWindow(config, new(gfx.Canvas))
	return err
})
```

## The theme (gfx.Theme; `gfx.LightTheme()`, `gfx.DarkTheme()`)

Colors, all `gfx.Color`: `Background`, `Foreground`, `Card`, `CardForeground`, `CardElevated`, `Popover`,
`PopoverForeground`, `Muted`, `MutedForeground`, `Border`, `Hover`, `Input`, `Primary`, `PrimaryForeground`,
`Selection`, `SelectionForeground`, `Secondary`, `SecondaryForeground`, `Accent`, `AccentForeground`, `Ring`, `Success`, `Danger`, `DangerForeground`, and
`Chart` (an array of five). There is no `Surface`, `Text` or `Error`: use `Card`, `Foreground`, `Danger`.
A `gfx.Color` is `{Red, Green, Blue, Alpha float32}` with each in 0..1 (so `gfx.Color{Alpha: 0.5}` is a black scrim);
it has `Mix(other, t)`, `Over(backdrop)` and `WithAlpha(a)`, and no `R` or `A`. To write a color of your own, `gfx.RGB24(0x0a84ff)`
(an integer constant, no string to parse), `gfx.RGB(10, 132, 255)` or `gfx.RGBA(10, 132, 255, 0.5)`; take theme colors where you can. A dark window is `gfx.DarkTheme()` in place
of `gfx.LightTheme()`: keep the theme in the model (`m.theme`) and give it to the page with
`ui.NewPage(ui.PageConfig{View: m.page, Theme: &m.theme})`; the page reads it through `c.Theme()`, and a change to the
value is the next paint's theme. `PageConfig.Background` fills the window with a color other than the theme's
`Background`.
`theme.Metrics` has `ControlHeight`, `RadiusControl`, `RadiusSurface`, `FontSizeS`, `FontSizeM`, `FontSizeL`,
`PadControlX`, `PadSurface`, `GapItem`. Take colors and sizes from the theme rather than writing literals.

## Components (package github.com/vivaui/viva/components)

Use them from `github.com/vivaui/viva/components` directly: Button, Checkbox, Switch, Input, Table, Tabs,
Dialog, Popover, Command, Calendar, Pagination, Progress, Tooltip and more; importing the
package costs nothing until you call one. To change a component beyond its options, copy it
into your project with `viva add <name>` (`viva list` shows every name; what it calls into is
copied with it). They take a
`gfx.Theme` (`gfx.LightTheme()`, `gfx.DarkTheme()`) and a rectangle. In a page, put one in a
`ui.Slot(c)` and paint it from the slot's `Draw` callback, where `d.Rect` is the slot's rectangle.

Painting your own thing in a slot: `ui.Slot(c).Grow(1).Draw(func(d *ui.DrawContext) { ... })`, with `d.Canvas.Fill(rect, color)`, `Rounded(rect, radius, color)`, `Border`, `RoundedBorder`, `Elevate(rect, radius, gfx.ShadowMD)` (a drop shadow: call it before the surface),
`DrawPath` and `d.Canvas.Label(text, rect, gfx.TextStyle{Size: 14, Color: c})` (`LabelCenter` is the same, centered) are all of
it; there is no arc, circle or line call (a circle is a `Rounded` square, or a `Box` with `Radius`). `d.Fade(color)` applies the
element's opacity to a color you supply, and `d.DashedBorder(rect, radius, 1, d.Fade(color))` paints a dashed frame the
element fades with.
For tabs, tables and text fields use `ui.Tabs`, `ui.Table` and `ui.TextField` above, not the components by hand.

## Background work

A goroutine never touches the window. Work that belongs to the page — a directory listing, a
request for the selected host — is a `ui.Effect`, not a goroutine of your own:

```go
var load = ui.NewEffect[dirListing, string](func(ctx context.Context) (dirListing, error) { ... })

func (m *model) page(c ui.C) {
	ui.Effect(c, m.load, m.dir) // the key: a new directory cancels the old run
	if listing, err, ok := m.load.Take(); ok {
		m.apply(listing, err)
	}
}
```

The run gets a context, must return when it is done, and must not touch a `C`, `Box` or `Page`.
The same key does not start again; a new key cancels the old run and starts the new one at once, so a
run that is slow to return can overlap it. `.Sequential()` on the state waits for the old run to
return before starting the next, and only the latest key declared while it waits runs.
`ui.NewEffectKeyed` passes the key a run started for to the run function. A paint that stops declaring
it cancels it; closing the window cancels all of them. `Take` reads the result once, on the window's
thread, after the effect is declared, and the page wakes itself when the result lands. One state per
page, and not inside a Memo. A page you draw into a window of your own takes that window's
`PageConfig.Invalidator` and calls `page.Close()` when it goes away.

A goroutine of your own, for something with no page key (a long-lived watcher), stores its result
under a lock and calls `invalidator.Invalidate()` (`Config.Invalidator`); `Config.OnWake` copies
the result into the model on the window's thread before the repaint.

## Testing

`app.NewDriver(page.Config(config()), nil)` runs the real application with no window
(`ui.NewPage(ui.PageConfig{View: m.page})` makes the page):
`driver.Click(x, y)`, `Type`, `Key`, `Move`, `Scroll`, `Resize`, then `driver.Snapshot()` for the
labels, hits and rectangles painted. `internal/desktop/desktop_test.go` clicks a control by its
name and reads the labels; copy it. Test behaviour a user sees (the page changed, the dialog
opened), not pixel positions.

## Rules

- Allocation-conscious: no `fmt.Sprintf` per frame for text that has not changed if it is in a
  hot path; keep the strings in the model.
- No reflection, no `map[string]any` for UI state, no goroutines started from a page function by
  hand: page work is a `ui.Effect`.

## Limits to know

- Controls and containers laid out by package ui are painted when the page function returns
  (`Page.Draw`), before whatever a `Slot` paints in its `Draw` callback. Hit order follows paint order, so
  declare overlapping interactive things in the order you want them to win.
- A `Border` is inside the box (as CSS `border-box`): it takes its width out of the padding area.
  `Stroke(width, color)` draws the same line but takes no room. `HitOutset(top, right, bottom, left)` makes the
  click region reach beyond the painted rectangle (an inset rounded row that clicks across its whole width).
