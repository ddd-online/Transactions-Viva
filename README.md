# Transactions

本地优先的个人财务桌面应用：一个工作空间内管理多个账本，覆盖记账、股票、待办、事件与日记。
所有业务数据只保存在用户自己选择的本地 SQLite 工作空间里。

## 功能

- **记账**：记录、分析、标签、模板四页共享账本。记录支持时间范围、排序、分页、右键编辑 /
  删除；分析支持折线图、饼图、预设与自定义图表；标签与模板都可增删改、拖拽排序。
- **股票**：工作空间级多账户，账户 / 持仓 / 记录 / 心得 / 统计 / 天梯六页。持仓与记录共用
  「交易台账 / 每日复盘 / 本轮复盘 / 成交记录」四段布局；统计有指标卡、自适应曲线与明细分页；
  天梯按连板分层并带缓存状态。
- **待办**：工作空间一份，列表 / 四象限切换。列表按主题分组，表头在主题展开区内，事项行紧凑，
  支持六档紧急 / 重要度、开始与截止日期、进度记录；历史可恢复、删除与订正进度。
- **事件**：工作空间级、一天一条。左侧事件卡，中间图片画廊与正文，右侧跨账本关联交易；支持
  JPEG / PNG / GIF / WebP / HEIC，缩略图与预览本地生成。
- **日记**：工作空间级多日记本，年 → 月 → 日日期树，正文停 600ms 自动保存，带五个心情档。
- **设置**：通用设置、账本管理、股票设置、日记配置、数据备份、关于软件。数据备份可立即备份、
  自动备份、恢复或删除单份备份；最近工作空间可切换或移除。关于软件可检查更新并下载安装包。

![外壳：记账页（浅色）](docs/scaffold-light.png)

![设置：通用设置](docs/settings-general.png)

![分析：折线图与曲线表](docs/analysis.png)

![分析：悬停读数](docs/analysis-hover.png)

![日记：日期树与正文](docs/diary.png)

![事件：年份列表、图片与关联交易](docs/events.png)

![股票：账户页](docs/stock-account.png)

![股票：持仓与本轮复盘](docs/stock-positions.png)

![股票：已清仓轮次](docs/stock-records.png)

![股票：心得](docs/stock-lessons.png)

![股票：结算统计](docs/stock-statistics.png)

![股票：连板天梯](docs/stock-ladder.png)

![待办：事项列表](docs/todo-board.png)

![待办：四象限](docs/todo-quadrant.png)

![待办：历史](docs/todo-history.png)

![设置：股票设置](docs/stock-settings.png)

![设置：日记配置](docs/diary-settings.png)

![设置：数据备份](docs/data-backup.png)

![设置：关于软件](docs/settings-update.png)

![设置：账本管理](docs/ledger-management.png)

![新增记录弹窗](docs/record-composer.png)

## 开发

```sh
go run . -dev               # 原生窗口 + 开发配置 ~/.transactions.dev.json（文本输入可用）
go run .                    # 原生窗口 + 生产配置 ~/.transactions.json
make run                    # 同 go run . -dev
viva dev .                  # 热重载开发；reload host 不转发原生文本事件
go test ./... -count=1      # 无窗口驱动测试 + 领域单测
go vet ./... && go tool viva vet  # 静态检查（CLI 版本由 go.mod 钉住）
viva build                  # 发布构建
```

> `viva dev .` 的热重载宿主只转发画面、指针与按键，不携带原生文本事件：所有 `ui.TextField`
> 在它下面都有光标却打不出字。需要测表单输入时，用 `go run . -dev` 或打包后的原生窗口。

## 数据

- 一个工作空间是一个目录：`transactions.db`（SQLite）加 `data/assets` 图片资产。
- 金额是整数分，股票价格是整数厘（1/1000 元），时间是 Unix 秒，日期是本地 `YYYY-MM-DD`。
- 配置在 `~/.transactions.json`，开发配置在 `~/.transactions.dev.json`；写入保留未知键。
- 核心数据只在本地；联网仅用于股票行情、涨停池与更新检查，并支持 HTTP 代理。

更完整的结构、分层与约定见 [docs/architecture.md](docs/architecture.md) 和
[AGENTS.md](AGENTS.md)。
