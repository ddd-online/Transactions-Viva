---
name: Transactions
description: 本地优先的个人财务桌面工作台——安静的灰阶外壳里，数字是唯一的主角
colors:
  primary: "#1d4ed8"
  primary-dark: "#2f5ce0"
  primary-foreground: "#ffffff"
  canvas: "#f4f4f5"
  canvas-dark: "#27272a"
  rail: "#fafafa"
  rail-dark: "#1f1f23"
  paper: "#ffffff"
  paper-dark: "#18181b"
  ink: "#18181b"
  ink-dark: "#fafafa"
  ink-muted: "#646b76"
  ink-muted-dark: "#a1a1aa"
  hairline: "#e4e4e7"
  hairline-dark: "rgba(250, 250, 250, 0.10)"
  field: "#d4d4d8"
  field-dark: "rgba(250, 250, 250, 0.14)"
  selection: "#c7d7fe"
  selection-dark: "#2b4a7d"
  highlight: "#e4e4e7"
  highlight-dark: "#2a2a2d"
  income: "#0f6b34"
  income-dark: "#4ade80"
  expense: "#b91c1c"
  expense-dark: "#f87171"
  transfer: "#0369a1"
  transfer-dark: "#38bdf8"
  outlier: "#8a5a06"
  outlier-dark: "#d9a04a"
  series-violet: "#7c3aed"
  series-violet-dark: "#b08bd8"
typography:
  title:
    fontSize: "26px"
    fontWeight: 700
    lineHeight: 1.3
  subtitle:
    fontSize: "19px"
    fontWeight: 700
    lineHeight: 1.4
  heading:
    fontSize: "14px"
    fontWeight: 700
    lineHeight: 1.5
  body:
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.6
  data:
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.6
  label:
    fontSize: "11px"
    fontWeight: 400
    lineHeight: 1.5
  amount:
    fontFamily: "Viva mono face"
    fontSize: "13px"
    fontWeight: 500
    lineHeight: 1.6
    letterSpacing: "-0.01em"
rounded:
  item: "5.5px"
  control: "7.5px"
  popup: "10px"
  surface: "11px"
  full: "9999px"
spacing:
  "2xs": "2px"
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "24px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.primary-foreground}"
    rounded: "{rounded.control}"
    height: "32px"
    padding: "0 12px"
  button-outline:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    height: "32px"
    padding: "0 12px"
  button-ghost:
    textColor: "{colors.ink-muted}"
    rounded: "{rounded.control}"
    height: "32px"
    padding: "0 12px"
  input:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    height: "32px"
    padding: "0 12px"
  nav-item:
    textColor: "{colors.ink}"
    rounded: "{rounded.item}"
    height: "32px"
    padding: "0 8px"
  nav-item-active:
    backgroundColor: "{colors.highlight}"
    textColor: "{colors.ink}"
    rounded: "{rounded.item}"
    height: "32px"
    padding: "0 8px"
  workspace-card:
    backgroundColor: "{colors.paper}"
    rounded: "{rounded.surface}"
    padding: "0"
  mark-chip:
    backgroundColor: "{colors.outlier}"
    textColor: "{colors.outlier}"
    rounded: "{rounded.item}"
    padding: "1px 6px"
---

# Design System: Transactions

## Overview

**Creative North Star: "The Quiet Ledger"**

Transactions 是本地优先的个人财务桌面工作台。窗口是一张安静的工作面：灰阶外壳、纸白卡片、一条蓝，其余留给数字与排布。它不表演，只把「这一笔账」和「此刻的状态」说清楚。

界面是一台桌面仪器，不是一张海报：精确比表达重要。密度来自 32px 的控件、48px 的记录行和 11px 的列表标签；克制来自「只有一个强调色、四个语义色，且语义色只出现在数据上」。层次由 1px 发丝线与色差完成，阴影只出现在真正浮起来的东西下面。

这套世界已经落地：`internal/desktop/theme.go` 里是两套完整主题——浅色 **Cool Graphite**（白纸、冷灰外壳、石墨字）与深色 **Cool Ink**（石墨纸、更亮一档的外壳）。导航列永远夹在外壳与纸面之间，深浅两套只是把这把尺子掉了个头：浅色里纸面最亮，深色里外壳最亮。页面只读 token；深色不是反相，每一对颜色都在自己的底色上单独挑过，并由 `theme_test.go` 盯着对比度（正文与四个数据色 ≥4.5:1，数据色压在自己的 12% 芯片底上也要 ≥4.5:1）。

**Key Characteristics:**

- 一个强调色（Signal Blue）承担全部动作、焦点与选中；没有第二强调色。
- 灰阶分三层：外壳灰、导航灰、纸白。它们之间只隔 1px 发丝线。
- 金额右对齐、列宽固定；等宽字体是已确认的规则，尚未实现。
- 32px 控件、7.5px 控件圆角、11px 内容卡圆角：小而一致。
- 语义色（收入 / 支出 / 转账 / 离群值）只标数据与状态，外壳与导航永不上色。
- 明暗双主题共用同一套 token 名，深色单独取值，不靠反色推导。

## Colors

中性色撑起界面，Signal Blue 是唯一的强调，四个语义色只服务于交易与状态。

### Primary

- **Signal Blue**（`#1d4ed8`，深色 `#2f5ce0`）：主按钮、活动排序按钮、选中的页签与账本、链接、焦点强调。前景是纯白（6.7:1）。深色下它反而提亮一档，因为深蓝会沉进石墨外壳。
- 选中底用主色的 14% alpha（`Primary.WithAlpha(0.14)`），hover 加一档到 22%，按下 30%。同一套透明度出现在排序按钮、活动筛选与图表曲线选中上。

### Neutral

- **Shell**（`#f4f4f5`，深色 `#27272a`）：窗口底、标题栏、状态行背后那层。它永远是最外的一层，卡片浮在它上面。
- **Rail**（`#fafafa`，深色 `#1f1f23`）：左侧导航列，永远夹在外壳与纸面之间——浅色下比外壳浅，深色下比外壳深。靠色差而不是边框与内容分开。
- **Paper**（`#ffffff`，深色 `#18181b`）：内容卡片、弹窗、输入框。整个窗口只有一块纸；浅色下它最亮，深色下它比外壳暗一档。
- **Ink**（`#18181b`，深色 `#fafafa`）：正文与图标。次要文字 `#646b76`（深色 `#a1a1aa`）承担标签、表头、占位说明——在纸面上 5.4:1 / 6.9:1，在 Muted 底上 4.9:1 / 5.8:1。
- **Hairline**（`#e4e4e7`，深色 `rgba(250,250,250,0.10)`）：所有分隔——表头底线、行分隔、卡片描边。输入框的描边深一档（`#d4d4d8` / `rgba(250,250,250,0.14)`）。
- **Highlight**（`#e4e4e7`，深色 `#2a2a2d`）：导航与菜单行的悬停 / 选中底。

### Semantic (data and state only)

- **Income**（`#0f6b34`，深色 `#4ade80`）：收入金额、收入类型字、完成的勾。主题的 `Success`。
- **Expense**（`#b91c1c`，深色 `#f87171`）：支出金额、删除确认、危险动作。主题的 `Danger`。
- **Transfer**（`#0369a1`，深色 `#38bdf8`）：转账金额与类型。图表色板的第三位，是一支偏青的天蓝——刻意离开动作蓝，交易类型永远不该读成一个按钮。
- **Outlier**（`#8a5a06`，深色 `#d9a04a`）：离群值标记、关联事件日期星标、警告状态。图表色板的第四位。
- 四个色都比旧工程的取色深一档：`#16a34a` 在纸上是 3.3:1、`#dc2626` 是 4.0:1，做 13px 金额不够。色相没动，只是压到正文对比度以内。
- 每个语义色压在自己 12% alpha 的胶囊底上，并且仍要 ≥4.5:1——页面通过 `internal/components/semantic` 读，不写死。

### The Two Appearances

浅色是 **Cool Graphite**：白纸、冷灰外壳、石墨正文，动作是 Signal Blue。深色是 **Cool Ink**：石墨纸（`#18181b`）夹在更亮的外壳（`#27272a`）与更暗的导航列（`#1f1f23`）之间，动作蓝提亮，危险色换成亮红——所以深色里的实心危险按钮压的是墨色字，而不是白字。

两套共用同一组 token 名，深色值单独挑、不由浅色反相。`internal/desktop/theme_test.go` 是对比度闸门：它把两套主题的正文、次要文字、四个数据色（含 12% 芯片底）、主按钮字色、焦点环、图表线逐条量一遍，任何一条掉出阈值都会让测试失败。

### Named Rules

**The One Accent Rule.** 全系统只有一个强调色。强调靠位置、字重与留白，不靠第二个颜色。

**The Semantic Silo Rule.** 收入 / 支出 / 转账 / 离群值只出现在数据与状态上——金额、类型字、标记、连通性；外壳、导航、标题栏、品牌面永不上色。

**The Theme-Only Colour Rule.** 颜色只在 `internal/desktop/theme.go` 定义一次，页面读 `c.Theme()`。数据色（收入 / 支出 / 转账 / 离群值）经 `internal/components/semantic` 从主题的 `Success`、`Danger` 与图表色板取；新页面里不出现一个十六进制。

## Typography

**Display Font:** 无自选字体。界面用 Viva 自带的字形与度量，三平台一致。
**Body Font:** 同上。
**Money Font:** 等宽面（Viva 的 `Mono()`），已确认、尚未实现。

**Character:** 一套字，靠字号与留白分层，不靠字体家族的变化。中文只有 400 与 700 两档真实字重，所以层级永远由字号与颜色承担；500 只给拉丁字母、数字与等宽面。

### Hierarchy

- **Title**（700, 26px, 1.3）：页面里的顶级区块标题，例如设置页右侧的「通用设置」。一屏最多一处。
- **Subtitle**（700, 19px, 1.4）：面板名与页名，例如设置面板左上角的「设置」。
- **Heading**（700, 14px, 1.5）：分组标题、卡片标题、设置行标题。
- **Body**（400, 13px, 1.6）：记录描述、表单值、对话框正文、空态的一句话。
- **Data**（400 / 500, 12px, 1.6）：表格里的分类与标签、类型字、模板单元格。
- **Label**（400, 11px, 1.5）：表头、小标题、离群值标记、状态行。表头用次要灰，不放大写中文。
- **Amount**（等宽, 500, 13px, 1.6, −0.01em）：所有金额。右对齐到列边。

### Named Rules

**The Mono Money Rule.**（已确认，尚未实现）每一笔金额、每一个价格都用等宽面，列与列对齐。正文里出现的钱也必须换面——数字不是散文。

**The Size Carries the Step Rule.** 中文没有 500 与 600 的区别：层级用字号和颜色拉，不要指望加粗。11 → 12 → 13 → 14 → 19 → 26 是全部档位，不新增中间档。

**The Label-Not-Annotation Rule.** 灰字只在控件自己说不清那个事实时才出现。设置页是一列「标题 + 控件」，能一句话说清的留在行尾，机制解释进 `Tooltip`，不进常驻正文。

## Layout

窗口是三层：36px 标题栏、中间一行（204px 导航列 + 内容卡）、卡片外的 7px 边距。

- **Title bar** 36px：左侧产品名 12px 700，右侧外观切换 32px 图标键；窗口按钮（最小化 / 最大化 / 关闭）由 Viva 自绘，标题栏在右侧为它们留出 3 倍键宽的内边距。整条可拖动窗口。
- **Navigation rail** 204px：8px 内边距、4px 行距，每个一级项 32px 高、子项 28px，子项缩进 26px 对齐标题文字；「应用设置」钉在底部。
- **Workspace card**：距右、下各 7px，11px 圆角、1px 发丝线边框、纸白底、裁剪。整窗只有这一张卡。
- **Content inside the card**：工具栏 56px 高、左右 18px；表头行 34px；记录行 48px、左右 18px、列距 12px。列宽固定（日期 78 / 类型 64 / 分类 112 / 标签 180 / 标记 92 / 金额 120），只有描述列吃剩余宽度。
- **Pagination** 贴在卡片底部，左侧每页条数、右侧「共 N 条」与翻页键；再下面是 11px 的状态行（左：数据位置说明；右：工作空间与版本）。
- **Settings** 用 208px 的左侧小节列 + 760px 的阅读列（左右各 32px、顶部 34px 起）；小节标题 26px，设置行 36px 高、行距 4px。
- **Window** 最小 960 × 620；更窄的窗口不折行金额、不压缩数字，表格改为横向滚动。

### Named Rules

**The One Card Rule.** 一个窗口只有一张卡（工作区）。卡里的一切靠发丝线与留白分组，不做「卡片里再套卡片」。

**The Fixed Column Rule.** 表格列宽固定、金额右对齐、描述列承担唯一的弹性。数字列随内容变宽会让每一页的数字都跳一次位。

## Elevation & Depth

发丝线分层：表面之间用色差加 1px 边框分开，卡片本身不带阴影。阴影只出现在真正浮起来的层——菜单、下拉、弹窗、对话框；它们由 Viva 的组件画，页面不自己加 `Shadow`。

内容是平的，焦点是立体的：键盘焦点用 2px `theme.Ring` 环（导航行 3px；浅色 `#52525b` 7.7:1、深色 `#a1a1aa` 6.9:1），画在控件外侧，不改变布局。环是石墨色而不是蓝：蓝环会和「选中」的强调色抢同一个意思。

### Shadow Vocabulary

- **popup**：菜单、Select 下拉、日期选择面板——Viva 组件自带，页面不覆盖。
- **dialog**：弹窗与确认框——同上，靠遮罩（scrim）压住下面的内容，不靠更重的阴影。

### Named Rules

**The Hairline Layering Rule.** 先用色差和 1px 发丝线分层；只有在东西真的离开纸面（悬停浮层、菜单、弹窗）时才出现阴影。

## Shapes

一致的圆角阶梯，越大的表面越圆：标签与胶囊 5.5px，控件 7.5px，浮层 10px，卡片 11px，开关与药丸 9999px。边框永远是 1px，不用 2px 描边做强调——强调交给颜色与字号。

标记、标签、账本选择器共用同一套「小胶囊」形状：`PadX(6~8) + PadY(1~2)`，圆角 5.5px 或 4px。不要用大圆角把数据做成气泡。

## Components

### Buttons

- **Shape:** 32px 高，7.5px 圆角，左右 12px；小号 28px，图标 16px。
- **Primary:** Signal Blue 底、纯白字，用于每页唯一的主动作（「记一笔」「新建模板」）。
- **Outline:** 纸白底、1px 发丝线、墨色字；工具栏的筛选与时间范围用它。
- **Ghost / Icon:** 无底无边框，悬停才有 Highlight 底；图标键 24–32px 见方。
- **States:** 悬停加深底色，按下再深一档；没有位移、缩放或涟漪。焦点是 2px 环。

### Inputs / Fields

- **Style:** 纸白底、1px `field` 描边、7.5px 圆角、32px 高、左右 12px 内边距。
- **Focus:** 描边换主色并加 2px 环，不改变尺寸。
- **Error:** 12px 危险色一行，落在字段下方；标签与控件始终成对出现，占位符不替代标签。

### Navigation

- **Style:** 204px 灰列，行 32px（子项 28px）、5.5px 圆角、8px 左右内边距，图标 16px 与文字相距 8px。
- **States:** 悬停与选中用同一块 Highlight 底；选中行字重 500、字色加深。没有蓝色选中条——强调色在这条列里不出现。
- **Two levels:** 一级项展开自己的子功能，同时收起其他一级项；没有子功能的分区（事件、日记）就是一行。

### Tables

- **Character:** 一张表就是这一页的内容，不是嵌在卡片里的卡片。
- **Header:** 11px 次要灰，行高 34px，底部一条发丝线。
- **Rows:** 48px，底部发丝线；悬停用 `theme.Hover`（几乎察觉不到的一档）。
- **Cells:** 日期 13px、类型 12px 500 且带语义色、分类与标签 12px、描述 13px 吃剩余宽度、金额 13px 500 右对齐。
- **Marks:** 离群值是一枚 11px 语义色胶囊（12% 底），关联事件日期是一枚琥珀星。

### Chips / Marks

- **Style:** 语义色字 + 同色 12% 底，无描边，5.5px 圆角，`PadX(6~8) PadY(1~2)`。
- **Muted tag:** 中性胶囊用 `Muted` 底 + 次要灰字，和语义胶囊区分开（前者是分类，后者是状态）。

### Dialogs

- **Style:** 11px 圆角、纸白、标题 14px 700；宽度只有几档——380 确认删除、400 单字段、440 关闭询问、520–540 短表单、640 记录编辑器。
- **Behavior:** 遮罩压住页面，Esc 与点遮罩关闭；弹窗里不出现第二个主按钮。

### Pagination

贴在卡片底部的一行：每页 15 / 30 / 50 的选择器在左，「共 N 条」与翻页键在右。空表不显示分页行。

## Do's and Don'ts

### Do:

- **Do** 从 `c.Theme()` 取颜色、从 `theme.Metrics` 取尺寸；页面里不出现十六进制字面量，数据色经 `internal/components/semantic` 取。
- **Do** 用 32px 控件、7.5px 控件圆角、11px 卡片圆角这三档把界面钉住。
- **Do** 金额右对齐、列宽固定；实现等宽规则时，整列一起换，不留半列等宽。
- **Do** 把语义色限制在数据与状态上。
- **Do** 先用色差与 1px 发丝线分层，阴影只给浮层。
- **Do** 给每个可交互元素 2px 焦点环（导航行 3px）与 32px 以内的点击面，键盘路径与指针路径同样可用。
- **Do** 中文不加 `text-transform: uppercase`，也不给标签加字距——那套是拉丁字母的排版习惯。

### Don't:

- **Don't** 引入第二个强调色；强调靠位置与字号。
- **Don't** 用语义色涂导航、标题栏或任何 chrome。
- **Don't** 在页面里硬编码颜色——每套主题只在 `internal/desktop/theme.go` 定义一次。
- **Don't** 用阴影搭层级（卡片静止无阴影），也不做「卡片套卡片」。
- **Don't** 给按下态加位移、缩放或涟漪；Viva 的控件用颜色回应，整个应用保持一致。
- **Don't** 让数字列随内容变宽或折行；窄窗优先收内边距与横向滚动。
