# transactions

## 项目目标

Transactions 是一款本地优先的个人财务桌面应用：一个工作空间内管理多个账本，覆盖记账
（记录 / 分析 / 标签 / 模板）、股票（账户 / 持仓 / 记录 / 心得 / 统计 / 天梯 / 设置）、
待办、事件与日记。核心数据只保存在用户选择的本地 SQLite 工作空间里。

技术栈是 **Viva + Go**：窗口与控件由 GPU 直接绘制。
产品按独立的全新项目维护。

当前功能：

- 外壳、配置、通用设置、记账四页、待办、事件、日记、股票六页与设置页均已落地。
- 工作空间包含账本、股票账户、日记本、事件、待办与图表；配置在
  `~/.transactions.json`（开发期为 `~/.transactions.dev.json`）。
- 数据备份支持立即备份、自动备份、恢复与删除单份备份；最近工作空间可切换或移除。
- 发布由 `build/build.ps1` / `build/release.ps1` 完成。

## 工程结构

```text
main.go                  入口：解析 -dev 后调用 desktop.Run
internal/desktop/        应用外壳：窗口、自绘标题栏、Sidebar、主题、设置页、工作空间与更新
internal/icons/          由 SVG 编译出的矢量图标
internal/pages/          功能页：accounting、stock、todo、events、diary、settings
  accounting/records/    记录页（表格 / 工具栏 / 分页 / 空态 / 行操作）
  accounting/records/composer/  记录弹窗
  accounting/ledger/     记账各页共享的账本列表与当前账本
  accounting/tags/       标签页
  accounting/template/   模板缓存与 domain 互转
  accounting/templates/  模板页
  accounting/analysis/   分析页
  diary/                 日记页
  todo/                  待办页
internal/diary/          日记文件导入导出
internal/assets/         图片资产：路径校验、落盘、删除、解码、缩略图
internal/events/         事件图片导入 / 删除编排
internal/backup/         备份命名、保留、快照 / 恢复与列表
internal/proxy/          代理三态、地址归一化与 HTTP client
internal/update/         GitHub release 检查、下载 / 校验与安装
internal/stockquote/     腾讯行情
internal/stocklimit/     东财涨停池与天梯缓存
internal/components/     可复用 UI：layout / datepicker / pagination / selectbox / semantic / chartanim
internal/pages/settings/controls/  设置页的滚动页 / 分组卡片 / 设置行
internal/domain/         纯领域规则与数据形状
internal/repository/     页面使用的数据契约
internal/config/         配置形状、默认值、读写与归一化
internal/store/          SQLite 工作空间、schema、仓储
docs/                    架构、发布说明与截图
```

依赖方向：

```text
main → desktop → pages → components
                   ↓
             repository ← store
                   ↓
                 domain
config 不依赖 UI；domain 不依赖 UI、数据库或操作系统；repository 只依赖 domain。
```

规则：页面只 import `ui`（以及本工程的 `components` / `domain` / `icons` / `repository`）；页面
不直接 import `internal/store`，也不持有 `ui.C`、`ui.Box` 或 `gfx.Canvas`；后台工作走 `ui.Effect`
或 `Invalidator`，不从页面函数里起 goroutine；坐标与像素只交给布局，不手算 `gfx.Rect`。打开系统
文件 / 目录对话框的点击处理一律放进 `c.After`：原生对话框的模态循环会给窗口派发消息，在绘制中
调用会让同一次点击被嵌套重绘再投递一次，甚至结束窗口。

导航是 Sidebar 的两级结构：一级功能项展开自己的子功能，同时收起其他一级项；没有子功能的功能
（事件、日记）就是单独一项。页面用 `Tabs() []string` / `Tab() int` / `SelectTab(int)` 暴露子功能，
内容里不再画 `ui.Tabs`。

## 数据契约

- 金额恒为整数「分」（字段以 `Cents` 结尾）；股票价格恒为整数「厘」（字段以 `Milli` 结尾）；
  时间恒为 Unix 秒（`At`）；日期恒为本地 `YYYY-MM-DD`（`Date`）。
- 配置是用户数据；生产 `~/.transactions.json`，开发 `~/.transactions.dev.json`；写入保留未知键。
- 每个工作空间一个独立 SQLite 数据库；图片资产在工作空间 `data/assets` 下，引用相对路径。
- 待办属于工作空间，不属于账本。
- 事件属于工作空间，不属于账本：`key_event.date` 唯一，图片在 `data/assets/key_events/<date>/`。
- 新库 schema 版本在 `app_meta.schema_version`；打开时检查版本与 `quick_check`，拒绝损坏、陌生或
  更高版本的库。恢复前先把当前数据另存为 `pre-restore-*`。
- 图表预设（月度 / 年度）只可删除，不可改名 / 改粒度；自建图表整行回写。
- 联网能力仅限股票行情、涨停池和更新检查，且都可由用户配置 HTTP 代理；其余功能离线可用。

## 验证

```sh
gofmt -l .                          # 必须没有输出
go vet ./...                        # 必须通过
go tool viva vet                    # 页面级错误
go test ./... -count=1              # 无窗口驱动测试 + 领域单测
```

页面行为用 `app.NewDriver` 驱动真实 painter / 输入 / 焦点来验证，不要为了看一眼界面就开真实窗口；
需要截图或布局检查时用 `building-with-viva` skill 里的 headless 协议。

## 提交

**每完成一次修改就提交一次。** 不要把多组改动留在工作区里等下一轮，也不要把不相关的改动塞进
同一个提交。

```sh
gofmt -l . && go vet ./... && go tool viva vet && go test ./... -count=1
git status --short                  # 只应有本次要提交的文件
git add -A && git commit -m "feat(scope): 中文摘要"
```

提交信息用 `type(scope): 中文摘要`：`feat`、`fix`、`style`、`docs`、`refactor`、`test`、`chore`。
一个提交只做一件事；截图、文档等产物与对应改动一起提交。

### 推送与发布

**平时的修改只 commit，不 push、不打标签、不创建 GitHub Release。** 只有用户在当次对话里明确
要求发布时才执行远端操作，且发布前必须完成全量验证：

```sh
gofmt -l .
go vet ./...
go tool viva vet
go test ./... -count=1
```

验证通过后按 `docs/release.md` 构建与发布。

## 参考

- Viva 框架：`D:\github\viva`，版本以 `go.mod` 为准，只读。
- 参考应用：`D:\github\motty`，Viva 应用的目录与工程实践样板，只读。
