# Transactions

本地优先的个人记账桌面应用：一个工作空间内管理多个账本，覆盖记账、股票、待办、事件与日记。
所有业务数据只保存在用户自己选择的本地 SQLite 工作空间里。

## 当前状态

项目正在从 Electron + React 迁移到 **Viva + Go**。这一轮只搭结构：

- 入口是根目录 `main.go`，已可运行窗口外壳（自绘标题栏、Viva Sidebar 导航、底栏、设置页、
  浅色 / 深色主题）。
- 六个功能页已有 Sidebar 导航和占位空态；子功能作为 Sidebar 二级项（点击一级项展开、其他收起），真实内容尚未迁移。
- `internal/domain` 已放入核心数据形状与金额换算；SQLite 存储、配置持久化还没开始。
- `viva init` 的计数示例已移除。

![外壳：记账页（浅色）](docs/scaffold-light.png)

## 开发

```sh
go run .                    # 打开应用窗口
viva dev .                  # 热重载开发
go test ./... -count=1      # 无窗口驱动测试 + 领域单测
go vet ./... && viva vet    # 静态检查
viva build                  # 发布构建
```

## 参考

- 旧工程（行为与数据权威）：`D:\github\Transactions`，视觉规范见其中的 `DESIGN.md`。
- 框架：`D:\github\viva`，版本以 `go.mod` 为准；示例在 `examples/`。
- 参考应用：`D:\github\motty`，大型 Viva 应用的目录与工程实践样板。

更完整的结构、分层规则与迁移顺序见 [docs/architecture.md](docs/architecture.md) 和
[AGENTS.md](AGENTS.md)。
