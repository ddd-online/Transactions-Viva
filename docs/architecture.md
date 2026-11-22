# 架构

Transactions 是一个 Viva + Go 的本地优先桌面应用。窗口、控件与绘制都由 Viva 提供；业务数据
只在用户选择的工作空间 SQLite 数据库里。

## 分层

```text
main → desktop → pages → components
                   ↓
             repository ← store
                   ↓
                 domain
config 不依赖 UI；domain 不依赖 UI、数据库或操作系统；repository 只依赖 domain。
```

- `main.go`：入口，只解析 `-dev` 并调用 `desktop.Run`。
- `internal/desktop`：窗口、自绘标题栏、Sidebar、主题、设置页、工作空间与备份 / 更新编排。
- `internal/pages`：各功能页；页面只 import `ui` 与本工程的 components / domain / icons / repository。
- `internal/components`：共享 UI，包括 `layout` 页面壳与表头、`datepicker`、`pagination`、
  `selectbox`、`semantic`、`chartanim`。
- `internal/domain`：纯领域规则与数据形状（金额、记录、账本、事件、日记、待办、股票）。
- `internal/repository`：页面使用的数据契约；不依赖 SQLite。
- `internal/store`：工作空间、schema、仓储与备份 / 恢复所需的数据库操作。
- `internal/config`：配置形状、默认值、读写与归一化。
- `internal/assets` / `internal/backup` / `internal/events` / `internal/diary` /
  `internal/proxy` / `internal/update` / `internal/stockquote` / `internal/stocklimit`：
  各自独立的文件、备份、图片、网络与市场数据能力。

## 页面

功能页统一使用共享壳：

```go
layout.Page(c, func(c ui.C) { /* toolbar */ }, func(c ui.C) { /* body */ })
```

表头由 `layout.TableHeader` 绘制内缩圆角表头带；数据表不画表头与首行、表项之间的分隔线。
页面状态在 `Model`，`View(c)` 只根据状态声明界面；点击处理在控件声明处完成，改变列表 / 打开
原生对话框放到 `c.After`。

工作空间、股票账户、日记本、账本的选择器属于外壳 / 页面共享状态，不在页面里直接打开数据库。

## 数据

- 一个工作空间一个目录：`transactions.db` + `data/assets`。
- schema v1 在 `internal/store/schema/0001_init.sql` 一次性建全；打开时做 `quick_check` 与版本检查。
- 金额整数分、股票价格整数厘、时间 Unix 秒、日期本地 `YYYY-MM-DD`。
- 配置是用户数据：生产 `~/.transactions.json`，开发 `~/.transactions.dev.json`；写回保留未知键。
- 手动备份是库 + assets 目录；自动备份是单库快照；恢复前先另存 `pre-restore-*`。

## 验证

```sh
gofmt -l .
go vet ./...
go tool viva vet
go test ./... -count=1
```

页面行为用 `app.NewDriver` 驱动真实绘制与输入；需要截图或布局检查时使用 headless 协议。
提交采用 `type(scope): 中文摘要`，一次一个逻辑改动。
