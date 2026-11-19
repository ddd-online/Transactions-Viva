# Transactions

本地优先的个人记账桌面应用：一个工作空间内管理多个账本，覆盖记账、股票、待办、事件与日记。
所有业务数据只保存在用户自己选择的本地 SQLite 工作空间里。

## 当前状态

项目正在从 Electron + React 迁移到 **Viva + Go**。外壳与记账-记录页的前端已经搭起：

- 入口是根目录 `main.go`，已可运行窗口外壳（自绘标题栏、Viva Sidebar 导航、底栏、设置页、
  浅色 / 深色主题）。
- 记账-记录页有工具栏、时间范围、排序、表格、分页和收支统计；「记一笔」打开
  新增记录弹窗，右键「编辑记录」复用同一弹窗（模板 / 日期 / 类型 / 分类 / 标签 / 标记 /
  关联事件日期 / 描述 / 金额；日期带日历选择，标签可多选）。记录行右键还提供
  同步到其他账本、删除。分页栏组合 Viva 分页与每页 15 / 30 / 50 的 Select。行数据、模板与
  新增 / 编辑都只在内存里。
- 其余功能页仍是 Sidebar 导航 + 占位空态；子功能作为 Sidebar 二级项（点击一级项展开、
  其他收起）。
- `internal/domain` 已放入核心数据形状与金额换算；SQLite 存储、配置持久化还没开始。
- `viva init` 的计数示例已移除。

![外壳：记账页（浅色）](docs/scaffold-light.png)

![新增记录弹窗](docs/record-composer.png)

![编辑记录弹窗](docs/record-composer-edit.png)

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
