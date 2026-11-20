# Transactions

本地优先的个人记账桌面应用：一个工作空间内管理多个账本，覆盖记账、股票、待办、事件与日记。
所有业务数据只保存在用户自己选择的本地 SQLite 工作空间里。

## 当前状态

项目正在从 Electron + React 迁移到 **Viva + Go**。外壳、配置与记账三页已经可用：

- 入口是根目录 `main.go`，已可运行窗口外壳（自绘标题栏、Viva Sidebar 导航、设置页、
  浅色 / 深色主题）。
- 记账-记录页有工具栏、时间范围、排序、表格、分页和收支统计；「记一笔」打开
  新增记录弹窗，右键「编辑记录」复用同一弹窗（模板 / 日期 / 类型 / 分类 / 标签 / 标记 /
  关联事件日期 / 描述 / 金额；日期带日历选择，标签可多选）。记录行右键还提供
  同步到其他账本、删除。分页栏组合 Viva 分页与每页 15 / 30 / 50 的 Select。
- 记账四页读写工作空间里的 `transactions.db`：账本、记录、分类标签、模板、分析图表都会持久化；
  空工作空间自动创建「默认账本」并播种旧工程的 19 个分类 / 57 个标签。没有工作空间时
  内容区显示「打开工作空间」门禁，设置里可切换目录。
- SQLite schema v1 / v2 带版本迁移（升级前写 `transactions.db.bak`）与损坏 / 版本检查；
  股票、事件、日记、待办表还未迁移，股票也不与账本关联。
- 记账-分析页有图表列表、折线图 / 饼图、时间范围、预设与自定义图表；图下方是曲线 / 分组的明细表，底部一条合计带。
- 旧工程的账本 / 分类标签 / 记录 / 模板 / 图表可用 `cmd/tr-import` 导入（见下）。
- 其余功能页仍是 Sidebar 导航 + 占位空态；子功能作为 Sidebar 二级项（点击一级项展开、
  其他收起）。
- 配置持久化、通用设置、外观与关闭行为已可用；设置里的「账本管理」可以新增 / 重命名 /
  删除账本并写账本描述（最后一本不可删）；「关于软件」显示版本 / 构建 / 平台 / 运行时，
  可复制版本信息并打开项目与问题反馈；`viva init` 的计数示例已移除。

![外壳：记账页（浅色）](docs/scaffold-light.png)

![分析：折线图与曲线表](docs/analysis.png)

![分析：深色](docs/analysis-dark.png)

![设置：账本管理](docs/ledger-management.png)

![设置：关于软件](docs/scaffold-about.png)

![新增记录弹窗](docs/record-composer.png)

![编辑记录弹窗](docs/record-composer-edit.png)

## 开发

```sh
go run .                    # 打开原生窗口（文本输入可用）
viva dev .                  # 热重载开发；reload host 不转发原生文本事件
go test ./... -count=1      # 无窗口驱动测试 + 领域单测
go vet ./... && viva vet    # 静态检查
viva build                  # 发布构建
```

> `viva dev .` 的热重载宿主只负责画面与指针 / 按键，不携带原生文本事件；所有 `ui.TextField`
> 在它下面都会出现光标却打不出字。需要测试图表名称、记录描述等表单输入时用 `go run .`
> 或打包后的原生窗口。

## 导入旧数据

```sh
go run ./cmd/tr-import -from <旧工作空间目录或 transactions.db> -to <新工作空间目录>
```

目标目录里不能已有 `transactions.db`，导入前请关闭旧应用。工具只读旧库，复制账本、分类、标签、记录、记录标签
与模板、图表；股票、事件、日记、待办不导入。默认报告孤儿行（例如记录指向不存在的账本），
加 `-strict` 则在有孤儿行时中止、不写入目标库。

## 参考

- 旧工程（行为与数据权威）：`D:\github\Transactions`，视觉规范见其中的 `DESIGN.md`。
- 框架：`D:\github\viva`，版本以 `go.mod` 为准；示例在 `examples/`。
- 参考应用：`D:\github\motty`，大型 Viva 应用的目录与工程实践样板。

更完整的结构、分层规则与迁移顺序见 [docs/architecture.md](docs/architecture.md) 和
[AGENTS.md](AGENTS.md)。
