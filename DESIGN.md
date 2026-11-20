---
name: Transactions
description: 本地优先的个人财务桌面工作台——安静的灰阶外壳里，数字是唯一的主角
colors:
  primary: "#1447e6"
  primary-dark: "#193cb8"
  primary-foreground: "#eff6ff"
  canvas: "#f5f5f5"
  canvas-dark: "#262626"
  rail: "#f7f7f7"
  rail-dark: "#1c1c1c"
  paper: "#ffffff"
  paper-dark: "#0a0a0a"
  ink: "#0a0a0a"
  ink-dark: "#fafafa"
  ink-muted: "#737373"
  ink-muted-dark: "#a1a1a1"
  hairline: "#e5e5e5"
  hairline-dark: "rgba(255, 255, 255, 0.10)"
  field: "#e5e5e5"
  field-dark: "rgba(255, 255, 255, 0.15)"
  selection: "#b5d3ff"
  selection-dark: "#264f78"
  highlight: "#e6e6e6"
  highlight-dark: "#343434"
  income: "#16a34a"
  income-dark: "#4ade80"
  expense: "#dc2626"
  expense-dark: "#f87171"
  transfer: "#3b82f6"
  transfer-dark: "#60a5fa"
  outlier: "#d97706"
  outlier-dark: "#fbbf24"
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

界面是一座桌面工作台，不是一张海报。密度来自 32px 的控件、48px 的记录行和 11px 的列表标签；克制来自「只有一个强调色、四个语义色，且语义色只出现在数据上」。层次由 1px 发丝线与色差完成，阴影只出现在真正浮起来的东西下面。

有一件事今天还看不见：中性色要往暖里走（暖灰纸面），而现在的壳仍是 Viva 默认的冷灰。方向已确认、代码未实现；所以暖色只从 `internal/desktop/theme.go` 一处落地，页面里不要先散着改。

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

- **Signal Blue**（`#1447e6`，深色 `#193cb8`）：主按钮、活动排序按钮、选中的页签与账本、链接、焦点强调。前景 `#eff6ff` 是它的字色——蓝面上永远压近白色，不压纯白以外的灰度。
- 选中底用主色的 14% alpha（`Primary.WithAlpha(0.14)`），hover 加一档到 22%，按下 30%。同一套透明度出现在排序按钮、活动筛选与图表曲线选中上。

### Neutral

- **Shell Gray**（`#f5f5f5`，深色 `#262626`）：窗口底、标题栏、状态行背后的那层。它永远是最外的一层，卡片浮在它上面。
- **Rail Gray**（`#f7f7f7`，深色 `#1c1c1c`）：左侧导航列。比外壳浅一档，靠色差而不是边框与内容分开。
- **Paper**（`#ffffff`，深色 `#0a0a0a`）：内容卡片、弹窗、输入框。整个窗口只有一块纸。
- **Ink**（`#0a0a0a`，深色 `#fafafa`）：正文与图标。次要文字 `#737373`（深色 `#a1a1a1`）承担标签、表头、占位说明。
- **Hairline**（`#e5e5e5`，深色 `rgba(255,255,255,0.10)`）：所有分隔——表头底线、行分隔、卡片描边。输入框描边用同一值。
- **Highlight**（`#e6e6e6`，深色 `#343434`）：导航与菜单行的悬停 / 选中底。它由 Viva 的 `Accent` 改写而来，比库里的默认值深，因为默认值在纸白上几乎读不出悬停。

### Semantic (data and state only)

- **Income**（`#16a34a`，深色 `#4ade80`）：收入金额、收入类型字、完成的勾。
- **Expense**（`#dc2626`，深色 `#f87171`）：支出金额、删除确认、危险动作。
- **Transfer**（`#3b82f6`，深色 `#60a5fa`）：转账金额与类型。
- **Outlier**（`#d97706`，深色 `#fbbf24`）：离群值标记、关联事件日期星标、警告状态。
- 每个语义色的胶囊底是它自己的 12% alpha，不是另取一个米色。

### Direction: Warm Paper（已确认，尚未实现）

中性色往暖里走：外壳从冷灰换成暖灰纸面，纸白微微偏米，墨色偏暖黑。强调蓝与四个语义色不动——温度只由中性色提供，不能靠加第二个彩色。

提案值（迁移时的起点，不是现在的代码）：外壳 `#f4f1ec`、导航 `#f7f4ef`、纸面 `#fffdfa`、发丝线 `#e7e2d9`、墨色 `#1a1815`、次要文字 `#6f675c`。落地时必须成对给出深色值，并按对比度复核，而不是把浅色反相。

### Named Rules

**The One Accent Rule.** 全系统只有一个强调色。强调靠位置、字重与留白，不靠第二个颜色。

**The Semantic Silo Rule.** 收入 / 支出 / 转账 / 离群值只出现在数据与状态上——金额、类型字、标记、连通性；外壳、导航、标题栏、品牌面永不上色。

**The Theme-Only Colour Rule.** 颜色只在 `theme.go` 定义一次。页面读 `c.Theme()`，不写字面量十六进制；暖色迁移只改主题层，页面一行都不用动。

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

内容是平的，焦点是立体的：键盘焦点用 2px `theme.Ring` 环（导航行 3px；浅色 `#a1a1a1`、深色 `#737373`），画在控件外侧，不改变布局。今天的焦点环是中性灰而不是蓝——这是现状，不是目标；新页面先用 `theme.Ring`，等主题层统一换色。

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
- **Primary:** Signal Blue 底、`#eff6ff` 字，用于每页唯一的主动作（「记一笔」「新建模板」）。
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

- **Do** 从 `c.Theme()` 取颜色、从 `theme.Metrics` 取尺寸；页面里不出现十六进制字面量（语义色的明暗对由共享 helper 提供，例如 `kindColor`）。
- **Do** 用 32px 控件、7.5px 控件圆角、11px 卡片圆角这三档把界面钉住。
- **Do** 金额右对齐、列宽固定；实现等宽规则时，整列一起换，不留半列等宽。
- **Do** 把语义色限制在数据与状态上。
- **Do** 先用色差与 1px 发丝线分层，阴影只给浮层。
- **Do** 给每个可交互元素 2px 焦点环（导航行 3px）与 32px 以内的点击面，键盘路径与指针路径同样可用。
- **Do** 中文不加 `text-transform: uppercase`，也不给标签加字距——那套是拉丁字母的排版习惯。

### Don't:

- **Don't** 引入第二个强调色；强调靠位置与字号。
- **Don't** 用语义色涂导航、标题栏或任何 chrome。
- **Don't** 在页面里硬编码新的灰或暖色——暖色迁移只发生在 `internal/desktop/theme.go`。
- **Don't** 用阴影搭层级（卡片静止无阴影），也不做「卡片套卡片」。
- **Don't** 给按下态加位移、缩放或涟漪；Viva 的控件用颜色回应，整个应用保持一致。
- **Don't** 让数字列随内容变宽或折行；窄窗优先收内边距与横向滚动。
