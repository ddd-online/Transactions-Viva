# Transactions

本地优先的个人财务桌面应用。一个工作空间里放多个账本，记账、股票、待办、事件和日记共用
同一份数据；所有业务数据只写进用户自己选的本地 SQLite 文件。

## 功能

- **记账**：记录、分析、标签、模板四页共用一个账本。记录页有时间范围、排序、分页和右键编辑 /
  删除；分析页是折线图、饼图加曲线表，支持预设和自建图表；标签、模板可以增删改和拖拽排序。
- **股票**：工作空间级多账户，账户、持仓、记录、心得、统计、天梯六页。持仓和记录共用「交易
  台账 / 每日复盘 / 本轮复盘 / 成交记录」四段布局；统计是指标卡、曲线和明细表；天梯按连板
  高度分层。
- **待办**：整个工作空间一份，列表和四象限两种看法。事项有六档紧急与重要度、开始与截止日期
  和进度记录；历史里可以恢复、删除、订正进度。
- **事件**：一天一条，左边事件卡，中间图片画廊和正文，右边是跨账本关联的交易。图片支持
  JPEG、PNG、GIF、WebP、HEIC，缩略图在本地生成。
- **日记**：多个日记本，按年、月、日往下点，正文停止输入 600ms 后自动保存，心情分五个档。
- **设置**：通用、账本管理、股票设置、日记配置、数据备份、关于软件。备份能立即备份、自动
  备份、恢复，也能删掉单份；最近工作空间可以切换或移除。

## 运行

需要 Go 1.26（以 go.mod 为准），目前只发布 Windows 10 / 11 桌面版。窗口和控件由 Viva 直接
绘制，没有浏览器或内嵌运行时。

```sh
go run . -dev               # 开发配置 ~/.transactions.dev.json
go run .                    # 生产配置 ~/.transactions.json
go test ./... -count=1      # 无窗口驱动测试和领域单测
go vet ./...                # 静态检查
go tool viva vet            # 页面级检查
```

`viva dev .` 能热重载，但它的 reload host 不转发原生文本事件：`ui.TextField` 在下面有光标
却打不出字。要测表单输入，用 `go run . -dev` 或者装好的桌面版。

安装包在 [GitHub Releases](https://github.com/ddd-online/Transactions-Viva/releases) 上，
构建和发布流程见 docs/release.md。

## 数据

- 一个工作空间就是一个目录：`transactions.db`（SQLite）和 `data/assets` 下的图片。
- 金额一律存整数分，股票价格存整数厘（1/1000 元），时间存 Unix 秒，日期存本地 `YYYY-MM-DD`。
- 配置在 `~/.transactions.json`，开发时用 `~/.transactions.dev.json`；写回会保留不认识的键。
- 打开工作空间先做 `quick_check`，库坏了只报告并引导从备份恢复，不会自动修复，也不会悄悄
  建一个空库。
- 联网只用在股票行情、涨停池和更新检查上；只有更新检查与下载走你在「关于软件 › 更新代理」里配的
  HTTP 代理，行情与涨停池直连；其余功能离线可用。

## 文档

- [docs/architecture.md](docs/architecture.md)：分层、依赖方向和页面约定
- [docs/release.md](docs/release.md)：版本号、构建产物和发布步骤
- [AGENTS.md](AGENTS.md)：工程约定、验证命令和提交规范
- [PRODUCT.md](PRODUCT.md)、[DESIGN.md](DESIGN.md)：产品定位与视觉规范
