package stock

import (
	"context"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/repository"
)

// ledgerDraft is the editable text of one ledger. The numbers are derived from
// the round's trades and are never stored.
type ledgerDraft struct {
	whyBuy         string
	stopLoss       string
	takeProfit     string
	followedSystem string
	summary        string
}

func (d ledgerDraft) trimmed() ledgerDraft {
	return ledgerDraft{
		whyBuy:         strings.TrimSpace(d.whyBuy),
		stopLoss:       strings.TrimSpace(d.stopLoss),
		takeProfit:     strings.TrimSpace(d.takeProfit),
		followedSystem: strings.TrimSpace(d.followedSystem),
		summary:        strings.TrimSpace(d.summary),
	}
}

func (d ledgerDraft) empty() bool {
	return d.whyBuy == "" && d.stopLoss == "" && d.takeProfit == "" &&
		d.followedSystem == "" && d.summary == ""
}

func ledgerDraftOf(entry *domain.StockLedgerEntry) ledgerDraft {
	if entry == nil {
		return ledgerDraft{}
	}
	return ledgerDraft{
		whyBuy:         entry.WhyBuy,
		stopLoss:       entry.StopLoss,
		takeProfit:     entry.TakeProfit,
		followedSystem: entry.FollowedSystem,
		summary:        entry.Summary,
	}
}

// ledgerSheet is one ledger as the page shows it: the entry's own text plus the
// numbers the round's trades produce. Both the open position and a closed round
// fill the same shape, so the two pages paint the same sheet.
type ledgerSheet struct {
	Entry    *domain.StockLedgerEntry
	BuyPrice string
	BuyCost  string

	// Closed says the round has been sold out; sell numbers are shown only then.
	Closed    bool
	SellPrice string
	SellNet   string
	NetPnl    string
	NetCents  int64
	HasNet    bool
}

func (s ledgerSheet) levels() domain.PlanLevels {
	if s.Entry == nil {
		return domain.PlanLevels{RatioKind: domain.PlanRatioMissing}
	}
	return domain.PlanLevelsOf(s.BuyPrice, s.Entry.StopLoss, s.Entry.TakeProfit)
}

func ledgerSheetOfView(view *domain.StockLedgerView) ledgerSheet {
	if view == nil {
		return ledgerSheet{}
	}
	sheet := ledgerSheet{
		Entry:    &view.Entry,
		BuyPrice: domain.MilliToYuan(view.BuyPriceMilli),
		BuyCost:  money(view.BuyCostCents),
		Closed:   true,
	}
	if view.SellPriceMilli != nil {
		sheet.SellPrice = domain.MilliToYuan(*view.SellPriceMilli)
	}
	if view.SellNetCents != nil {
		sheet.SellNet = money(*view.SellNetCents)
	}
	if view.NetPnlCents != nil {
		sheet.NetPnl = money(*view.NetPnlCents)
		sheet.NetCents = *view.NetPnlCents
		sheet.HasNet = true
	}
	return sheet
}

// ledgerSection is the 交易台账 block both pages paint: its head, the empty
// state, and the grouped sheet. The records page paints the same body under
// its own section head, so the head and the body are separate.
func ledgerSection(c ui.C, sheet ledgerSheet, errorText string, onEdit func()) {
	ui.Column(c).FillWidth().Gap(layout.Gap).Children(func() {
		ledgerSectionHead(c, sheet, onEdit)
		ledgerSectionBody(c, sheet, errorText, onEdit)
	})
}

func ledgerSectionHead(c ui.C, sheet ledgerSheet, onEdit func()) {
	ui.Row(c).FillWidth().Height(layout.SectionHeaderHeight).Items(ui.Center).Children(func() {
		typography.Section(c, "交易台账")
		ui.Spacer(c)
		if ui.Button(c, ledgerActionLabel(sheet)).Outline().Small().Clicked() && onEdit != nil {
			onEdit()
		}
	})
}

func ledgerActionLabel(sheet ledgerSheet) string {
	if sheet.Entry != nil {
		return "编辑"
	}
	return "填写台账"
}

func ledgerSectionBody(c ui.C, sheet ledgerSheet, errorText string, onEdit func()) {
	theme := c.Theme()
	if errorText != "" {
		typography.Caption(c, errorText).Color(theme.Danger)
	}
	if sheet.Entry == nil {
		ledgerEmpty(c, theme, onEdit)
		return
	}
	ledgerSheetView(c, sheet)
}

func ledgerEmpty(c ui.C, theme *ui.Theme, onEdit func()) {
	row := ui.Row(c.Key("ledger-empty")).FillWidth().Height(layout.RowHeight).
		PadX(4).Gap(layout.Gap).Items(ui.Center).
		HitWith(ui.ID(c), "填写交易台账", gfx.HitOptions{Cursor: gfx.PointerCursorHand}).
		Hover(ui.Look{}.Background(theme.Hover)).
		Radius(theme.Metrics.RadiusControl)
	row.Children(func() {
		typography.Caption(c, "还没有填写台账，记下买入理由、止损与止盈。").
			Color(theme.MutedForeground)
	})
	if row.Clicked() && onEdit != nil {
		onEdit()
	}
}

func ledgerSheetView(c ui.C, sheet ledgerSheet) {
	theme := c.Theme()
	levels := sheet.levels()
	entry := sheet.Entry
	ui.Column(c).Gap(layout.GroupGap).Children(func() {
		ledgerGroup(c, "买入计划", func() {
			ledgerProse(c, "为什么买", entry.WhyBuy)
			ui.Flow(c).Gap(layout.GroupGap).LineGap(layout.BlockGap).Children(func() {
				ledgerValue(c, "买入价", ledgerOrDash(sheet.BuyPrice), true,
					theme.Foreground, "", theme.MutedForeground)
				ledgerValue(c, "买入成本", ledgerOrDash(sheet.BuyCost), true,
					theme.Foreground, "", theme.MutedForeground)
				ledgerValue(c, "止损", ledgerOrDash(entry.StopLoss), false,
					theme.Foreground, ledgerDeltaNote(levels.Downside), ledgerDeltaTone(theme, levels.Downside))
				ledgerValue(c, "止盈", ledgerOrDash(entry.TakeProfit), false,
					theme.Foreground, ledgerDeltaNote(levels.Upside), ledgerDeltaTone(theme, levels.Upside))
				ledgerValue(c, "盈亏比", ledgerRatioValue(levels), true,
					theme.Foreground, "", theme.MutedForeground)
			})
		})
		ledgerGroup(c, "清仓结果", func() {
			ui.Flow(c).Gap(layout.GroupGap).LineGap(layout.BlockGap).Children(func() {
				if !sheet.Closed {
					ledgerValueInline(c, "卖出价", "清仓后回填", false, theme.MutedForeground)
					ledgerValueInline(c, "卖出净额", "清仓后回填", false, theme.MutedForeground)
					ledgerValueInline(c, "净盈亏", "清仓后回填", false, theme.MutedForeground)
					return
				}
				tone := theme.Foreground
				if sheet.HasNet {
					tone = pnlColor(*theme, sheet.NetCents)
				}
				ledgerValueInline(c, "卖出价", ledgerOrDash(sheet.SellPrice), true, theme.Foreground)
				ledgerValueInline(c, "卖出净额", ledgerOrDash(sheet.SellNet), true, theme.Foreground)
				ledgerValueInline(c, "净盈亏", ledgerOrDash(sheet.NetPnl), true, tone)
			})
		})
		ledgerGroup(c, "执行复盘", func() {
			ui.Row(c).FillWidth().Gap(layout.GroupGap).Items(ui.Start).Children(func() {
				ledgerProseWidth(c, "是否按系统执行", entry.FollowedSystem, 160)
				ledgerProseWidth(c, "一句话", entry.Summary, 0)
			})
		})
	})
}

func ledgerGroup(c ui.C, label string, body func()) {
	ui.Column(c).FillWidth().Gap(layout.Gap).Children(func() {
		ui.Text(c, label).FontSize(typography.SizeBody).Bold()
		body()
	})
}

// ledgerProse is a full-width text field of the sheet. Empty text shows a dash.
func ledgerProse(c ui.C, label, text string) {
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		typography.Caption(c, label)
		ledgerProseText(c, text, 0)
	})
}

func ledgerProseWidth(c ui.C, label, text string, width float32) {
	row := ui.Row(c).Gap(8).Items(ui.Center)
	if width > 0 {
		row = row.Width(width)
	} else {
		row = row.Grow(1)
	}
	row.Children(func() {
		typography.Caption(c, label).Shrink(0)
		text = strings.TrimSpace(text)
		if text == "" {
			typography.Caption(c, "—").Color(c.Theme().MutedForeground)
			return
		}
		ui.Text(c, text).FontSize(typography.SizeBody).
			Grow(1).MinWidth(0).Ellipsis()
	})
}

func ledgerProseText(c ui.C, text string, maxWidth float32) {
	text = strings.TrimSpace(text)
	if text == "" {
		typography.Caption(c, "—").Color(c.Theme().MutedForeground)
		return
	}
	body := ui.Wrap(c, text).FontSize(typography.SizeBody)
	if maxWidth > 0 {
		body.MaxWidth(maxWidth)
	}
}

// ledgerValue is one read-only number (or a short free-text level) of the sheet.
func ledgerValue(c ui.C, label, value string, mono bool, tone gfx.Color, note string, noteTone gfx.Color) {
	ui.Column(c).Width(160).Gap(4).Children(func() {
		typography.Caption(c, label)
		if mono {
			ui.Text(c, value).FontSize(typography.SizeBody).Mono().Color(tone).Ellipsis()
		} else {
			ui.Paragraph(c, value, 2).FontSize(typography.SizeBody).Color(tone)
		}
		if note != "" {
			ui.Text(c, note).FontSize(typography.SizeCaption).Mono().Color(noteTone)
		}
	})
}

// ledgerValueInline is a read-only value with its label on the same line. The
// clear-result and review fields use it so an unfilled value does not make the
// field two lines tall.
func ledgerValueInline(c ui.C, label, value string, mono bool, tone gfx.Color) {
	ui.Row(c).Width(160).Gap(6).Items(ui.Center).Children(func() {
		typography.Caption(c, label).Width(56).Shrink(0).Ellipsis()
		cell := ui.Text(c, value).FontSize(typography.SizeBody).
			Grow(1).MinWidth(0).Ellipsis().Color(tone)
		if mono {
			cell.Mono()
		}
	})
}

func ledgerOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func ledgerDeltaNote(delta string) string {
	if delta == "" {
		return ""
	}
	return "较买入价 " + delta
}

func ledgerDeltaTone(theme *ui.Theme, delta string) gfx.Color {
	switch {
	case strings.HasPrefix(delta, "+"):
		return pnlColor(*theme, 1)
	case strings.HasPrefix(delta, "-"):
		return pnlColor(*theme, -1)
	}
	return theme.MutedForeground
}

func ledgerRatioValue(levels domain.PlanLevels) string {
	if levels.RatioKind != domain.PlanRatioValue {
		return "—"
	}
	return levels.RatioText
}

// ledgerPrompt is why the ratio has no number yet, in the dialog.
func ledgerPrompt(kind domain.PlanRatioKind) string {
	switch kind {
	case domain.PlanRatioMissing:
		return "填好止损和止盈后自动计算"
	case domain.PlanRatioUnreadable:
		return "止损和止盈写成价格数字，才能算盈亏比"
	case domain.PlanRatioBlocked:
		return "止盈高于买入价、止损低于买入价，才有盈亏比"
	}
	return ""
}

// ledgerEditor is the shared 交易台账 dialog state, held by both pages.
type ledgerEditor struct {
	open      bool
	stockCode string
	roundID   string
	sheet     ledgerSheet
	draft     ledgerDraft
	err       string
	onSaved   func(domain.StockLedgerEntry)
}

func (e *ledgerEditor) begin(sheet ledgerSheet, stockCode, roundID string,
	onSaved func(domain.StockLedgerEntry)) {
	e.open = true
	e.sheet = sheet
	e.stockCode, e.roundID = stockCode, roundID
	e.draft = ledgerDraftOf(sheet.Entry)
	e.err = ""
	e.onSaved = onSaved
}

func (e *ledgerEditor) close() {
	e.open = false
	e.err = ""
}

func (e *ledgerEditor) dialog(c ui.C, repo repository.StockRepository, accountID string) {
	if !e.open {
		return
	}
	theme := c.Theme()
	entry := e.sheet.Entry
	title := "填写台账"
	if entry != nil {
		title = "编辑台账"
	}
	levels := domain.PlanLevelsOf(e.sheet.BuyPrice, e.draft.stopLoss, e.draft.takeProfit)
	// Keep the footer on screen at the 620 px minimum window; the body scrolls
	// when the whole form does not fit.
	bodyHeight := c.Bounds().Height - 300
	if bodyHeight < 300 {
		bodyHeight = 300
	}
	ui.ModalWith(c, &e.open, title, ui.ModalSpec{Width: 680}, func() {
		ui.Scroll(c).MaxHeight(bodyHeight).Scrollbars().Children(func() {
			ui.Column(c).FillWidth().Gap(layout.GroupGap).Children(func() {
				ledgerGroup(c, "买入计划", func() {
					ledgerTextArea(c, "为什么买", &e.draft.whyBuy,
						"当时为什么决定买这一轮")
					ui.Row(c).FillWidth().Gap(layout.BlockGap).Items(ui.Start).Children(func() {
						ledgerComputed(c, "买入价", ledgerOrDash(e.sheet.BuyPrice), theme.Foreground)
						ledgerComputed(c, "买入成本", ledgerOrDash(e.sheet.BuyCost), theme.Foreground)
						ledgerInput(c, "止损", func() {
							ui.TextField(c, &e.draft.stopLoss, "止损", "例如：9.00").
								Mono().Grow(1)
						}, ledgerDeltaNote(levels.Downside), ledgerDeltaTone(theme, levels.Downside))
						ledgerInput(c, "止盈", func() {
							ui.TextField(c, &e.draft.takeProfit, "止盈", "例如：12.00").
								Mono().Grow(1)
						}, ledgerDeltaNote(levels.Upside), ledgerDeltaTone(theme, levels.Upside))
					})
					ledgerRatioLine(c, levels)
				})
				ledgerGroup(c, "清仓结果", func() {
					ui.Row(c).FillWidth().Gap(layout.BlockGap).Items(ui.Start).Children(func() {
						if !e.sheet.Closed {
							ledgerComputed(c, "卖出价", "清仓后回填", theme.MutedForeground)
							ledgerComputed(c, "卖出净额", "清仓后回填", theme.MutedForeground)
							ledgerComputed(c, "净盈亏", "清仓后回填", theme.MutedForeground)
							return
						}
						tone := theme.Foreground
						if e.sheet.HasNet {
							tone = pnlColor(*theme, e.sheet.NetCents)
						}
						ledgerComputed(c, "卖出价", ledgerOrDash(e.sheet.SellPrice), theme.Foreground)
						ledgerComputed(c, "卖出净额", ledgerOrDash(e.sheet.SellNet), theme.Foreground)
						ledgerComputed(c, "净盈亏", ledgerOrDash(e.sheet.NetPnl), tone)
					})
				})
				ledgerGroup(c, "执行复盘", func() {
					ledgerRadioGroup(c, "是否按系统执行", &e.draft.followedSystem)
					ledgerInput(c, "一句话", func() {
						ui.TextField(c, &e.draft.summary, "一句话", "一句话总结这一轮").Grow(1)
					}, "", theme.MutedForeground)
				})
				if e.err != "" {
					typography.Caption(c, e.err).Color(theme.Danger)
				}
			})
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			e.close()
		}
		if ui.Button(c, "保存").Clicked() {
			e.save(repo, accountID)
		}
	})
}

// ledgerFollowedOptions are the answers 是否按系统执行 accepts. The empty value
// is 未填写, so an optional field can be cleared again after it was answered.
var ledgerFollowedOptions = [...]ledgerOption{
	{value: "是", label: "是"},
	{value: "否", label: "否"},
	{value: "部分", label: "部分"},
	{value: "", label: "未填写"},
}

type ledgerOption struct {
	value string
	label string
}

func ledgerRadioGroup(c ui.C, label string, value *string) {
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		typography.Caption(c, label)
		ui.Row(c).Gap(layout.GroupGap).Items(ui.Center).
			FocusGroup(ui.FocusHorizontal).Children(func() {
			for _, option := range ledgerFollowedOptions {
				ui.Radio(c.Key("followed-"+option.label), value, option.value, option.label)
			}
		})
	})
}

// ledgerTextArea is a labelled multi-line editor.
func ledgerTextArea(c ui.C, label string, value *string, placeholder string) {
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		typography.Caption(c, label)
		ui.TextArea(c, value, label, placeholder).Height(96).Grow(1)
	})
}

// ledgerInput is one labelled editable field with an optional note under it.
func ledgerInput(c ui.C, label string, control func(), note string, noteTone gfx.Color) {
	ui.Column(c).Grow(1).Basis(0).Gap(4).Children(func() {
		typography.Caption(c, label)
		control()
		if note != "" {
			ui.Text(c, note).FontSize(typography.SizeCaption).Mono().Color(noteTone)
		}
	})
}

// ledgerComputed is a derived number drawn like a disabled field: the dialog
// never lets the user edit a figure that comes from the trades.
func ledgerComputed(c ui.C, label, value string, tone gfx.Color) {
	theme := c.Theme()
	ui.Column(c).Grow(1).Basis(0).Gap(4).Children(func() {
		typography.Caption(c, label)
		ui.Row(c).Height(theme.Metrics.ControlHeight).FillWidth().
			PadX(theme.Metrics.PadControlX).Radius(theme.Metrics.RadiusControl).
			Background(theme.Muted).Items(ui.Center).Children(func() {
			ui.Text(c, value).FontSize(typography.SizeBody).Mono().
				Color(tone).Ellipsis()
		})
	})
}

func ledgerRatioLine(c ui.C, levels domain.PlanLevels) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Gap(6).Items(ui.Center).Children(func() {
		typography.Caption(c, "盈亏比")
		if levels.RatioKind == domain.PlanRatioValue {
			ui.Text(c, levels.RatioText).FontSize(typography.SizeBody).
				Bold().Mono().Color(theme.Foreground)
			return
		}
		typography.Caption(c, ledgerPrompt(levels.RatioKind)).
			Color(theme.MutedForeground)
	})
}

func (e *ledgerEditor) save(repo repository.StockRepository, accountID string) {
	if repo == nil {
		return
	}
	draft := e.draft.trimmed()
	if draft.empty() {
		e.err = "台账至少填写一项"
		return
	}
	saved, err := repo.SaveStockLedger(context.Background(), domain.StockLedgerEntry{
		AccountID:      accountID,
		StockCode:      e.stockCode,
		RoundID:        e.roundID,
		WhyBuy:         draft.whyBuy,
		StopLoss:       draft.stopLoss,
		TakeProfit:     draft.takeProfit,
		FollowedSystem: draft.followedSystem,
		Summary:        draft.summary,
	})
	if err != nil {
		e.err = positionsErrorText(err)
		return
	}
	e.open = false
	e.err = ""
	if e.onSaved != nil {
		e.onSaved(saved)
	}
}
