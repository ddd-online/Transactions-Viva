package stock

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/emptystate"
	"transactions/internal/components/layout"
	"transactions/internal/components/selectbox"
	"transactions/internal/components/semantic"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/repository"
	"transactions/internal/stockquote"
)

// Column widths shared by the trade table header and its rows. The fixed
// columns keep the table readable at the 960 px minimum window; the price,
// amount and fee columns share what is left, so the table widens with the
// detail instead of leaving a blank strip before 操作.
const (
	tradeTimeWidth   = float32(96)
	tradeTypeWidth   = float32(56)
	tradeLotsWidth   = float32(48)
	tradePriceMin    = float32(56)
	tradeAmountMin   = float32(64)
	tradeFeeMin      = float32(56)
	tradeActionWidth = float32(56)

	// Below this window width the round header gives up its single line: the
	// round, its stats, the P&L and the two controls no longer fit together.
	recordsRoundHeaderSingleRowWidth float32 = 1200
)

// recordsPanel is 股票 · 记录: the closed stocks on the left and the chosen
// stock's rounds on the right. It reads the active account from the shared
// accountStore and never changes it itself.
type recordsPanel struct {
	accounts *accountStore
	repo     repository.StockRepository
	quotes   stockquote.Fetcher
	onChange func()
	toaster  *ui.Toaster
	// openRound is the shell's "记心得" jump: it opens the lessons composer for
	// a closed round (normally selecting the 心得 tab and calling
	// lessonsPanel.OpenForRound). A nil callback hides the button.
	openRound func(code, roundID string)

	histories   []domain.StockHistoryItem
	historyView []int
	historyFind ui.Field
	details     map[string]*domain.StockHistoryDetail
	summary     domain.StockHistorySummary
	selected    string
	// accountID is the account the loaded data belongs to; a reload after a
	// switch elsewhere in the section drops the old selection and overlays.
	accountID string
	// defaultedFor names the stock whose default expansion (the largest round)
	// has already been chosen, so a reload does not reopen a round the user
	// closed.
	defaultedFor string
	expanded     map[string]bool
	arrivedRound string
	scrollRound  string
	scrollPaints int

	tags           []string
	roundTagValues map[string]*string
	reviewTemplate string

	errorMessage string

	historyList ui.ListState
	roundList   ui.ListState

	quoteEffect *ui.EffectState[quoteResult, quoteRequest]
	quoteKey    string
	quoteSerial int
	quoteMap    map[string]domain.StockQuote

	ledgerEdit ledgerEditor

	reviewOpen    bool
	reviewRoundID string
	reviewDraft   string
	reviewError   string

	readOpen  bool
	readTitle string
	readText  string

	editOpen   bool
	editTrade  domain.StockTrade
	editPrice  string
	editLots   string
	editError  string
	deleteOpen bool
	deleteID   string
	deleteNote string

	focusCode  string
	focusRound string
}

// recordsPanel is one of the section's screens.
var _ panel = (*recordsPanel)(nil)

// newRecordsPanel returns the 记录 screen over the section's shared accounts.
// stock.Model fills onChange, toaster, quotes and openRound once the screen is
// wired.
func newRecordsPanel(accounts *accountStore) *recordsPanel {
	p := &recordsPanel{
		accounts:       accounts,
		expanded:       map[string]bool{},
		roundTagValues: map[string]*string{},
		quoteMap:       map[string]domain.StockQuote{},
	}
	p.historyList.Label = p.historyLabel
	p.roundList.Label = p.roundLabel
	p.quoteEffect = ui.NewEffectKeyed(func(ctx context.Context, request quoteRequest) (quoteResult, error) {
		result := quoteResult{Quotes: map[string]domain.StockQuote{}}
		if p.quotes == nil {
			return result, nil
		}
		codes := strings.Split(request.Codes, ",")
		if len(codes) == 0 || (len(codes) == 1 && codes[0] == "") {
			return result, nil
		}
		quotes, err := p.quotes.Quotes(ctx, codes)
		if quotes != nil {
			result.Quotes = quotes
		}
		result.Failed = err != nil
		return result, nil
	}).Sequential()
	return p
}

// setRepository connects the panel to the workspace and reads the active
// account. stock.Model calls it next to accountPanel.setRepository.
func (p *recordsPanel) setRepository(repo repository.StockRepository) {
	p.repo = repo
	if repo == nil {
		p.clear()
		return
	}
	p.Reload()
}

// setQuotes gives the panel the section's quote source; without one the left
// cards show 现价 as「—」.
func (p *recordsPanel) setQuotes(quotes stockquote.Fetcher) { p.quotes = quotes }

func (p *recordsPanel) active() string {
	if p.accounts == nil {
		return ""
	}
	return p.accounts.activeID()
}

// Reload reads the closed histories, every trade, round, day review and ledger
// row of the active account, then assembles the left cards, the right detail
// and the footer summary. A pending FocusRound is applied after the read.
func (p *recordsPanel) Reload() {
	p.errorMessage = ""
	p.quoteKey = ""
	p.quoteSerial++
	active := p.active()
	if p.accountID != active {
		p.accountID = active
		p.CloseOverlays()
		p.selected, p.defaultedFor, p.arrivedRound, p.scrollRound = "", "", "", ""
		p.expanded = map[string]bool{}
		p.roundTagValues = map[string]*string{}
		p.quoteMap = map[string]domain.StockQuote{}
		p.historyFind.SetText("")
		p.historyView = nil
		p.tags = nil
		p.reviewTemplate = ""
	}
	if p.repo == nil || active == "" {
		p.clear()
		return
	}
	ctx := context.Background()
	histories, err := p.repo.StockHistories(ctx, active)
	if err != nil {
		p.fail("读取交易历史失败：" + err.Error())
		return
	}
	trades, err := p.repo.StockTrades(ctx, active)
	if err != nil {
		p.fail("读取成交失败：" + err.Error())
		return
	}
	rounds, err := p.repo.StockRounds(ctx, active)
	if err != nil {
		p.fail("读取轮次失败：" + err.Error())
		return
	}
	days, err := p.repo.StockDayReviews(ctx, active)
	if err != nil {
		p.fail("读取每日复盘失败：" + err.Error())
		return
	}
	ledgers, err := p.repo.StockLedgerEntries(ctx, active)
	if err != nil {
		p.fail("读取交易台账失败：" + err.Error())
		return
	}
	p.histories, p.details, p.summary = assembleRecordHistory(histories, trades, rounds, days, ledgers)
	p.rebuildHistoryView(p.historyFind.Text())
	if tags, err := p.repo.StockTradeTags(ctx, active); err == nil {
		p.tags = tags
	} else {
		p.errorMessage = "读取交易标签失败：" + err.Error()
	}
	if review, err := p.repo.StockReviewTemplates(ctx, active); err == nil {
		p.reviewTemplate = review.Template
	}
	p.applyFocus()
	p.ensureSelection()
}

// fail clears the read state and keeps the message above the content.
func (p *recordsPanel) fail(message string) {
	p.errorMessage = message
	p.histories, p.details = nil, nil
	p.historyView = nil
	p.summary = domain.StockHistorySummary{}
	p.selected, p.defaultedFor, p.arrivedRound, p.scrollRound = "", "", "", ""
	p.expanded = map[string]bool{}
}

func (p *recordsPanel) clear() {
	p.histories, p.details = nil, nil
	p.historyView = nil
	p.summary = domain.StockHistorySummary{}
	p.selected, p.defaultedFor, p.arrivedRound, p.scrollRound = "", "", "", ""
	p.expanded = map[string]bool{}
	p.tags = nil
	p.reviewTemplate = ""
	p.roundTagValues = map[string]*string{}
	p.quoteMap = map[string]domain.StockQuote{}
	p.quoteKey = ""
	p.errorMessage = ""
	p.CloseOverlays()
}

// applyFocus selects and expands the round a jump asked for, and lays the
// groundwork for the next few paints to scroll it into view.
func (p *recordsPanel) applyFocus() {
	if p.focusCode == "" {
		return
	}
	code, roundID := p.focusCode, p.focusRound
	p.focusCode, p.focusRound = "", ""
	detail := p.details[code]
	if detail == nil {
		return
	}
	p.selected, p.defaultedFor = code, code
	p.expanded = map[string]bool{}
	found := false
	for _, round := range detail.Rounds {
		if round.Round.ID == roundID {
			found = true
			break
		}
	}
	if found {
		p.expanded[roundID] = true
		p.arrivedRound = roundID
		p.scrollRound = roundID
		p.scrollPaints = 3
	} else {
		p.resetExpanded(detail)
	}
	p.syncHistorySelection()
}

// ensureSelection keeps the chosen stock when it survives a reload and falls
// back to the first card otherwise.
func (p *recordsPanel) ensureSelection() {
	if len(p.histories) == 0 {
		p.selected, p.defaultedFor, p.arrivedRound, p.scrollRound = "", "", "", ""
		return
	}
	if _, ok := p.details[p.selected]; !ok {
		p.selected = p.histories[0].History.StockCode
		p.defaultedFor = ""
	}
	if p.defaultedFor != p.selected {
		p.resetExpanded(p.details[p.selected])
		p.defaultedFor = p.selected
	}
	p.syncHistorySelection()
}

// syncHistorySelection mirrors the panel's choice into the rail list so its
// keyboard path starts from the same row.
func (p *recordsPanel) syncHistorySelection() {
	if p.selected == "" {
		p.historyList.Select(-1)
		return
	}
	for index, full := range p.historyView {
		if p.histories[full].History.StockCode == p.selected {
			if p.historyList.Selected() != index {
				p.historyList.Select(index)
			}
			return
		}
	}
	p.historyList.Select(-1)
}

// rebuildHistoryView applies the rail's search box to the full history. The
// query is matched as a substring of the stock name or code, so typing 600 or
// 浦发 narrows the rail on the next paint.
func (p *recordsPanel) rebuildHistoryView(query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	p.historyView = p.historyView[:0]
	for index := range p.histories {
		history := p.histories[index].History
		if query == "" ||
			strings.Contains(strings.ToLower(history.StockName), query) ||
			strings.Contains(strings.ToLower(history.StockCode), query) {
			p.historyView = append(p.historyView, index)
		}
	}
}

func (p *recordsPanel) detail() *domain.StockHistoryDetail {
	return p.details[p.selected]
}

// resetExpanded folds every round except the largest round number.
func (p *recordsPanel) resetExpanded(detail *domain.StockHistoryDetail) {
	p.expanded = map[string]bool{}
	if detail == nil || len(detail.Rounds) == 0 {
		return
	}
	latest := detail.Rounds[0]
	for _, round := range detail.Rounds[1:] {
		if round.Round.RoundNo > latest.Round.RoundNo {
			latest = round
		}
	}
	p.expanded[latest.Round.ID] = true
}

// FocusRound is the 心得 / 统计 jump target: it selects the stock, expands the
// round, highlights it and asks the next paints to scroll it into view. It
// reloads the account when a workspace is open; the shell only has to select
// the 记录 tab (and may call Reload again).
func (p *recordsPanel) FocusRound(code, roundID string) {
	p.focusCode, p.focusRound = code, roundID
	if p.repo != nil && p.active() != "" {
		p.Reload()
	}
}

// chooseHistory switches the right column to one stock's card.
func (p *recordsPanel) chooseHistory(index int) {
	if index < 0 || index >= len(p.historyView) {
		return
	}
	code := p.histories[p.historyView[index]].History.StockCode
	if code == p.selected {
		return
	}
	p.selected = code
	p.defaultedFor = code
	p.arrivedRound, p.scrollRound = "", ""
	p.resetExpanded(p.details[code])
}

// CloseOverlays dismisses the panel's dialogs when the shell leaves the screen.
func (p *recordsPanel) CloseOverlays() {
	p.ledgerEdit.close()
	p.reviewOpen = false
	p.reviewError = ""
	p.readOpen = false
	p.editOpen = false
	p.editError = ""
	p.deleteOpen = false
}

// View declares the screen: toolbar, the 240 px rail, the round column and
// the 40 px summary band.
func (p *recordsPanel) View(c ui.C) {
	if p.repo == nil || p.active() == "" {
		emptyAccountPage(c, "还没有股票账户", "工作空间里至少会有一个股票账户。")
		return
	}
	p.ensureQuotes(c)
	theme := c.Theme()
	ui.Column(c).Fill().Children(func() {
		p.toolbar(c)
		if p.errorMessage != "" {
			typography.Caption(c, p.errorMessage).Color(theme.Danger).
				PadX(layout.Inset).PadY(6)
		}
		ui.Row(c).Grow(1).FillWidth().Children(func() {
			p.historyRail(c)
			ui.Divider(c)
			p.detailColumn(c)
		})
		ui.Divider(c)
		p.footer(c)
	})
	p.ledgerEdit.dialog(c, p.repo, p.active())
	p.reviewModal(c)
	p.readModal(c)
	p.editModal(c)
	p.deleteModal(c)
}

func (p *recordsPanel) toolbar(c ui.C) {
	ui.Row(c).Height(layout.ToolbarHeight).FillWidth().PadX(layout.Inset).
		Gap(layout.Gap).Items(ui.Center).Children(func() {
		if p.accounts.picker(c) {
			p.CloseOverlays()
			p.selected, p.defaultedFor, p.arrivedRound = "", "", ""
			p.expanded = map[string]bool{}
			p.quoteMap = map[string]domain.StockQuote{}
			if p.onChange != nil {
				p.onChange()
			} else {
				p.Reload()
			}
		}
		ui.Divider(c).Height(24)
		ui.Spacer(c)
	})
}

// historyRail is the 240 px list of closed stocks. Its cards carry the current
// price and accumulated profit, then the round count, last close and rate.
func (p *recordsPanel) historyRail(c ui.C) {
	p.rebuildHistoryView(p.historyFind.Text())
	p.syncHistorySelection()
	ui.Column(c).Width(layout.RailWidth).MinWidth(layout.RailWidth).Shrink(0).
		FillHeight().Children(func() {
		ui.Row(c).Height(layout.SectionHeaderHeight).FillWidth().
			PadX(layout.Inset).Gap(layout.Gap).Items(ui.Center).Children(func() {
			typography.Section(c, "已清仓")
			typography.Caption(c, strconv.Itoa(len(p.histories))+" 只")
			ui.Spacer(c)
		})
		ui.Divider(c)
		if len(p.histories) == 0 {
			if p.errorMessage == "" {
				emptystate.Notice(c, "还没有已清仓的交易历史")
			} else {
				ui.Spacer(c).Grow(1)
			}
			return
		}
		if len(p.historyView) == 0 {
			typography.Caption(c, "没有匹配的股票").PadX(12).PadY(8).
				Color(c.Theme().MutedForeground)
		} else {
			// Keep the same Text node in both states: removing it would shift
			// the search field's identity and could drop its focus.
			ui.Text(c, "").Height(0)
		}
		// Keep the list in the tree even when the filter matches nothing: its
		// removal would shift the search field's identity and drop its focus.
		ui.List(c, &p.historyList, len(p.historyView), p.historyRow).
			Label("已清仓股票").Grow(1)
		if p.historyList.Changed() {
			p.chooseHistory(p.historyList.Selected())
		}
		ui.Divider(c)
		ui.Column(c).FillWidth().PadX(layout.Inset).PadY(8).Children(func() {
			ui.TextFieldID(c, ui.ID(c), &p.historyFind, "搜索股票", "股票名称或代码").
				FillWidth()
		})
	})
}

func (p *recordsPanel) historyRow(row ui.ListRow) {
	if row.Index < 0 || row.Index >= len(p.historyView) {
		return
	}
	item := p.histories[p.historyView[row.Index]]
	history := item.History
	theme := row.C.Theme()
	active := history.StockCode == p.selected
	name := history.StockName
	if name == "" || name == history.StockCode {
		name = history.StockCode
	}
	background := theme.Card
	if active {
		background = theme.Accent
	}
	labelColor := theme.MutedForeground
	if active {
		labelColor = theme.Foreground
	}
	row.Element.MinHeight(78).FillWidth().PadX(12).PadY(8).Gap(4).
		MarginSides(4, 8, 0, 8).
		Radius(theme.Metrics.RadiusControl).Background(background).
		Border(1, theme.Border).
		Hover(ui.Look{}.Background(theme.Hover)).
		Focus(ui.Look{}.Ring(2, theme.Ring))
	ui.Column(row.C).Grow(1).Gap(2).Children(func() {
		ui.Row(row.C).FillWidth().Gap(6).Items(ui.Center).Children(func() {
			typography.Body(row.C, name).Grow(1).Ellipsis()
			typography.Caption(row.C, history.StockCode).Mono().Ellipsis()
		})
		ui.Row(row.C).FillWidth().Gap(6).Items(ui.Center).Children(func() {
			typography.Caption(row.C, "现价").Color(labelColor)
			ui.Text(row.C, p.latestPrice(history.StockCode)).
				FontSize(typography.SizeCaption).Mono().Color(labelColor)
			ui.Spacer(row.C)
			label := "盈"
			if item.TotalPnl < 0 {
				label = "亏"
			}
			ui.Text(row.C, label+" "+money(historyAbsInt64(item.TotalPnl))).
				FontSize(typography.SizeCaption).Mono().
				Color(pnlColor(*theme, item.TotalPnl))
		})
		ui.Row(row.C).FillWidth().Gap(6).Items(ui.Center).Children(func() {
			typography.Caption(row.C,
				strconv.Itoa(item.RoundCount)+" 轮 · 最近 "+historyDate(item.LastClosedAt)).
				Color(labelColor).Grow(1).Ellipsis()
			ui.Text(row.C, historyPercent(item.TotalPnlRate)).
				FontSize(typography.SizeCaption).Mono().
				Color(historyRateColor(*theme, item.TotalPnlRate))
		})
	})
}

func (p *recordsPanel) historyLabel(index int) string {
	if index < 0 || index >= len(p.historyView) {
		return ""
	}
	history := p.histories[p.historyView[index]].History
	return historyStockTitle(history.StockName, history.StockCode)
}

func (p *recordsPanel) roundLabel(index int) string {
	detail := p.detail()
	if detail == nil || index < 0 || index >= len(detail.Rounds) {
		return ""
	}
	return "第 " + strconv.Itoa(detail.Rounds[index].Round.RoundNo) + " 轮"
}

// detailColumn is the right side: the stock's header, then its rounds.
func (p *recordsPanel) detailColumn(c ui.C) {
	detail := p.detail()
	ui.Column(c).Grow(1).FillHeight().MinWidth(0).Children(func() {
		if detail == nil {
			if p.errorMessage == "" {
				emptystate.Notice(c, "选择一只股票查看交易记录")
			} else {
				ui.Spacer(c).Grow(1)
			}
			return
		}
		p.detailHead(c, detail)
		ui.Divider(c)
		ui.List(c, &p.roundList, len(detail.Rounds),
			func(row ui.ListRow) { p.roundRow(row, detail) }).
			Label("交易轮次").Grow(1)
		if p.scrollPaints > 0 && p.scrollRound != "" {
			for index := range detail.Rounds {
				if detail.Rounds[index].Round.ID == p.scrollRound {
					p.roundList.ScrollIntoView(index)
					p.scrollPaints--
					break
				}
			}
		}
	})
}

func (p *recordsPanel) detailHead(c ui.C, detail *domain.StockHistoryDetail) {
	theme := c.Theme()
	name := detail.History.StockName
	if name == "" || name == detail.History.StockCode {
		name = detail.History.StockCode
	}
	ui.Row(c).FillWidth().Height(layout.SectionHeaderHeight).
		PadX(layout.Inset).Gap(8).Items(ui.Center).Children(func() {
		typography.Section(c, name).Ellipsis()
		typography.Caption(c, detail.History.StockCode).Mono()
		ui.Spacer(c)
		ui.Row(c).Shrink(0).Gap(10).Items(ui.Center).Children(func() {
			typography.Caption(c, "累计盈亏")
			ui.Text(c, money(detail.TotalPnl)).FontSize(typography.SizeBody).
				Mono().Color(pnlColor(*theme, detail.TotalPnl))
			ui.Text(c, historyPercent(detail.TotalPnlRate)).
				FontSize(typography.SizeCaption).Mono().
				Color(historyRateColor(*theme, detail.TotalPnlRate))
			typography.Caption(c, fmt.Sprintf("%d 胜 %d 负 · %d 轮",
				detail.WinCount, detail.LossCount, len(detail.Rounds)))
		})
	})
}

// roundRow is one expandable round block. Rounds are separated by hairlines,
// not nested cards.
func (p *recordsPanel) roundRow(row ui.ListRow, detail *domain.StockHistoryDetail) {
	if row.Index < 0 || row.Index >= len(detail.Rounds) {
		return
	}
	round := detail.Rounds[row.Index]
	theme := row.C.Theme()
	block := ui.Column(row.C).FillWidth().Children(func() {
		p.roundHeader(row.C, round)
		if p.expanded[round.Round.ID] {
			ui.Divider(row.C)
			p.roundBody(row.C, round)
		}
		ui.Divider(row.C)
	})
	if p.arrivedRound == round.Round.ID {
		block.Background(theme.Accent)
	}
}

func (p *recordsPanel) roundHeader(c ui.C, round domain.StockRoundDetail) {
	theme := c.Theme()
	expanded := p.expanded[round.Round.ID]
	action := "展开"
	if expanded {
		action = "收起"
	}
	head := ui.Column(c).FillWidth().PadX(12).PadY(8).Gap(4).
		Background(theme.Muted).
		HitWith(ui.ID(c), fmt.Sprintf("%s第 %d 轮", action, round.Round.RoundNo),
			gfx.HitOptions{Role: gfx.RoleButton, Selected: expanded}).
		Hover(ui.Look{}.Background(theme.Hover)).
		Focus(ui.Look{}.Ring(2, theme.Ring))
	if p.arrivedRound == round.Round.ID {
		head.Background(theme.Accent)
	}
	head.Children(func() {
		if c.Bounds().Width >= recordsRoundHeaderSingleRowWidth {
			ui.Row(c).FillWidth().Gap(layout.Gap).Items(ui.Center).
				Justify(ui.Between).Children(func() {
				recordRoundTitle(c, round, expanded, *theme)
				recordRoundStats(c, round)
				recordRoundPnl(c, round, *theme)
				p.roundControls(c, round)
			})
			return
		}
		ui.Column(c).FillWidth().Gap(6).Children(func() {
			ui.Row(c).FillWidth().Gap(layout.Gap).Items(ui.Center).Children(func() {
				recordRoundTitle(c, round, expanded, *theme)
				ui.Spacer(c)
				recordRoundStats(c, round)
			})
			ui.Row(c).FillWidth().Gap(layout.Gap).Items(ui.Center).Children(func() {
				recordRoundPnl(c, round, *theme)
				ui.Spacer(c)
				p.roundControls(c, round)
			})
		})
	})
	if head.Clicked() {
		p.toggleRound(round.Round.ID)
	}
}

// recordRoundTitle is the round's name and date range, with the disclosure mark.
func recordRoundTitle(c ui.C, round domain.StockRoundDetail, expanded bool, theme gfx.Theme) {
	ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
		glyph := icons.ChevronRight
		if expanded {
			glyph = icons.ChevronDown
		}
		ui.Icon(c, glyph).Size(13, 13).Color(theme.MutedForeground)
		ui.Text(c, "第 "+strconv.Itoa(round.Round.RoundNo)+" 轮").
			FontSize(typography.SizeBody).Weight(gfx.WeightMedium)
		typography.Caption(c, recordRoundRange(round.Round))
	})
}

// recordRoundStats is how much the round traded and what it cost.
func recordRoundStats(c ui.C, round domain.StockRoundDetail) {
	ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
		typography.Caption(c, strconv.Itoa(len(round.Trades))+" 笔成交")
		typography.Caption(c, "费用")
		ui.Text(c, money(round.FeeCents)).FontSize(typography.SizeCaption).Mono()
	})
}

// recordRoundPnl is the round's settled profit and its rate.
func recordRoundPnl(c ui.C, round domain.StockRoundDetail, theme gfx.Theme) {
	ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
		ui.Text(c, money(round.PnlCents)).FontSize(typography.SizeBody).
			Mono().Color(pnlColor(theme, round.PnlCents))
		ui.Text(c, historyPercent(round.PnlRate)).
			FontSize(typography.SizeCaption).Mono().
			Color(historyRateColor(theme, round.PnlRate))
	})
}

// roundControls are the round's editable tag and the 记心得 entry.
func (p *recordsPanel) roundControls(c ui.C, round domain.StockRoundDetail) {
	ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
		tag := p.roundTagValue(round.Round)
		if selectbox.Select(c, tag, p.roundTagChoices(*tag),
			func(value string) string { return value }).
			Small().Width(88).Label("标签").Changed() {
			p.saveRoundTag(round.Round.ID, *tag)
		}
		if p.openRound != nil {
			if ui.Button(c, "记心得").Small().Secondary().Clicked() {
				p.openRound(round.Round.StockCode, round.Round.ID)
			}
		}
	})
}

// roundTagValue keeps one stable string per round: a Select's state is keyed by
// the pointer it binds, and a local inside a list row would be shared by every
// row at the same stack address.
func (p *recordsPanel) roundTagValue(round domain.StockTradeRound) *string {
	if p.roundTagValues == nil {
		p.roundTagValues = map[string]*string{}
	}
	value := p.roundTagValues[round.ID]
	if value == nil {
		tag := domain.EffectivePositionTag(round.Tag)
		value = &tag
		p.roundTagValues[round.ID] = value
	}
	*value = domain.EffectivePositionTag(round.Tag)
	return value
}

func (p *recordsPanel) roundTagChoices(current string) []string {
	if current == "" {
		current = domain.DefaultStockTradeTag
	}
	choices := make([]string, 0, len(p.tags)+1)
	choices = append(choices, p.tags...)
	if !historyContains(choices, current) {
		choices = append(choices, current)
	}
	return choices
}

func (p *recordsPanel) toggleRound(roundID string) {
	p.expanded[roundID] = !p.expanded[roundID]
	if p.arrivedRound == roundID {
		p.arrivedRound = ""
	}
}

func (p *recordsPanel) roundBody(c ui.C, round domain.StockRoundDetail) {
	ui.Column(c).FillWidth().Children(func() {
		p.ledgerSection(c, round)
		ui.Divider(c)
		p.dailyReviews(c, round)
		ui.Divider(c)
		p.roundReview(c, round)
		ui.Divider(c)
		// 交易记录 is last: its table already draws the row separators, and the
		// round's closing divider supplies the bottom line without doubling it.
		p.recordSection(c, "交易记录", nil, func() {
			p.tradeTable(c, round)
		})
	})
}

// recordSection is one item of an expanded round: its name in the section
// header, an optional action, and the content under it. The four sections of a
// round share this head so their name size, header height and gap match.
func (p *recordsPanel) recordSection(c ui.C, name string, action func(), body func()) {
	ui.Column(c).FillWidth().Gap(4).PadBottom(8).Children(func() {
		ui.Row(c).FillWidth().PadX(12).PadTop(8).
			Items(ui.Center).Children(func() {
			typography.Section(c, name)
			ui.Spacer(c)
			if action != nil {
				action()
			}
		})
		body()
	})
}

// tradeTable is the read-only trade table of one round. Editing and deleting
// are the optional actions; a close fill cannot be edited, as in the old page.
func (p *recordsPanel) tradeTable(c ui.C, round domain.StockRoundDetail) {
	theme := c.Theme()
	ui.Column(c).FillWidth().Children(func() {
		ui.Row(c).Height(30).FillWidth().PadX(12).Gap(4).Items(ui.Center).
			Background(theme.Muted).Children(func() {
			typography.Caption(c, "时间").Width(tradeTimeWidth).TextCenter()
			typography.Caption(c, "类型").Width(tradeTypeWidth).TextCenter()
			typography.Caption(c, "成交价").Grow(1).Basis(tradePriceMin).
				MinWidth(tradePriceMin).TextEnd()
			typography.Caption(c, "手数").Width(tradeLotsWidth).TextEnd()
			typography.Caption(c, "金额").Grow(1).Basis(tradeAmountMin).
				MinWidth(tradeAmountMin).TextEnd()
			typography.Caption(c, "费用").Grow(1).Basis(tradeFeeMin).
				MinWidth(tradeFeeMin).TextEnd()
			ui.Spacer(c)
			typography.Caption(c, "操作").Width(tradeActionWidth).TextCenter()
		})
		for index, trade := range round.Trades {
			p.tradeRow(c, trade)
			if index < len(round.Trades)-1 {
				ui.Divider(c)
			}
		}
		if len(round.Trades) == 0 {
			typography.Caption(c, "本轮没有成交").PadX(12).PadY(8)
		}
	})
}

func (p *recordsPanel) tradeRow(c ui.C, trade domain.StockTrade) {
	theme := c.Theme()
	c.Key(trade.ID)
	ui.Row(c).Height(36).FillWidth().PadX(12).Gap(4).Items(ui.Center).
		Children(func() {
			ui.Text(c, historyShortDateTime(trade.TradeTime)).
				FontSize(typography.SizeBody).Mono().
				Width(tradeTimeWidth).TextCenter().Ellipsis()
			ui.Text(c, domain.TradeTypeLabel(trade.TradeType)).
				FontSize(typography.SizeBody).Mono().
				Color(positionsTradeTypeColor(*theme, trade.TradeType)).
				Width(tradeTypeWidth).TextCenter()
			ui.Text(c, domain.MilliToYuan(trade.PriceMilli)).
				FontSize(typography.SizeBody).Mono().
				Grow(1).Basis(tradePriceMin).MinWidth(tradePriceMin).TextEnd()
			ui.Text(c, strconv.Itoa(trade.Lots)).
				FontSize(typography.SizeBody).Mono().
				Width(tradeLotsWidth).TextEnd()
			ui.Text(c, domain.CentsToYuan(trade.AmountCents)).
				FontSize(typography.SizeBody).Mono().
				Grow(1).Basis(tradeAmountMin).MinWidth(tradeAmountMin).TextEnd()
			ui.Text(c, money(trade.FeeCents)).
				FontSize(typography.SizeBody).Mono().
				Grow(1).Basis(tradeFeeMin).MinWidth(tradeFeeMin).TextEnd()
			ui.Spacer(c)
			ui.Row(c).Width(tradeActionWidth).Gap(2).Items(ui.Center).
				Justify(ui.Center).Children(func() {
				if trade.TradeType != domain.TradeClose {
					edit := ui.Button(c, "编辑成交").
						Ghost().Icon(icons.Pencil).XSmall().IconOnly().Tooltip("编辑")
					if edit.Clicked() {
						p.openTradeEdit(trade)
					}
				}
				remove := ui.Button(c, "删除整笔委托").
					Ghost().Icon(icons.Trash2).XSmall().IconOnly().Tooltip("删除")
				if remove.Clicked() {
					p.askDeleteOrder(trade)
				}
			})
		})
}

// ledgerSection is the round's trade ledger, read-only here.
func (p *recordsPanel) ledgerSection(c ui.C, round domain.StockRoundDetail) {
	sheet := ledgerSheetOfView(round.Ledger)
	open := func() { p.openLedger(round) }
	p.recordSection(c, "交易台账", func() {
		if ui.Button(c, ledgerActionLabel(sheet)).Outline().Small().Clicked() {
			open()
		}
	}, func() {
		ui.Column(c).FillWidth().PadX(12).Children(func() {
			ledgerSectionBody(c, sheet, "", open)
		})
	})
}

func (p *recordsPanel) openLedger(round domain.StockRoundDetail) {
	p.ledgerEdit.begin(ledgerSheetOfView(round.Ledger), round.Round.StockCode, round.Round.ID,
		func(domain.StockLedgerEntry) {
			p.notify("台账已保存", ui.ToastSuccess)
			p.Reload()
		})
}

func (p *recordsPanel) dailyReviews(c ui.C, round domain.StockRoundDetail) {
	theme := c.Theme()
	p.recordSection(c, "每日复盘", nil, func() {
		ui.Column(c).FillWidth().PadX(12).Gap(4).Children(func() {
			if len(round.DayReviews) == 0 {
				typography.Caption(c, "本轮没有每日复盘")
				return
			}
			for _, day := range round.DayReviews {
				c.Key(day.ID)
				row := ui.Row(c).FillWidth().MinHeight(30).Gap(8).Items(ui.Center).
					Radius(theme.Metrics.RadiusControl).
					HitWith(ui.ID(c), "查看 "+day.ReviewDate+" 的每日复盘",
						gfx.HitOptions{Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand}).
					Hover(ui.Look{}.Background(theme.Hover)).
					Focus(ui.Look{}.Ring(2, theme.Ring))
				row.Children(func() {
					typography.Caption(c, day.ReviewDate).Mono().Width(84)
					typography.Body(c, day.Review).Grow(1).Ellipsis()
				})
				if row.Clicked() {
					p.openRead("每日复盘 · "+day.ReviewDate, day.Review)
				}
			}
		})
	})
}

func (p *recordsPanel) roundReview(c ui.C, round domain.StockRoundDetail) {
	theme := c.Theme()
	review := round.Round.Review
	filled := strings.TrimSpace(review) != ""
	p.recordSection(c, "本轮复盘", func() {
		if filled {
			if ui.Button(c, "查看全文").Ghost().Small().Clicked() {
				p.openRead("本轮复盘", review)
			}
		}
		if ui.Button(c, "编辑").Secondary().Small().Clicked() {
			p.openReviewEditor(round.Round)
		}
	}, func() {
		ui.Column(c).FillWidth().PadX(12).Children(func() {
			if filled {
				ui.Paragraph(c, review, 3).FontSize(typography.SizeBody).
					Color(theme.Foreground)
			} else {
				typography.Caption(c, "还没有写本轮复盘，点「编辑」记录买入与卖出的理由。")
			}
		})
	})
}

func (p *recordsPanel) openReviewEditor(round domain.StockTradeRound) {
	p.reviewRoundID = round.ID
	p.reviewDraft = round.Review
	if strings.TrimSpace(p.reviewDraft) == "" {
		p.reviewDraft = p.reviewTemplate
	}
	p.reviewError = ""
	p.reviewOpen = true
}

func (p *recordsPanel) openRead(title, text string) {
	p.readTitle, p.readText, p.readOpen = title, text, true
}

func (p *recordsPanel) saveRoundReview() {
	if p.repo == nil {
		p.reviewError = "工作空间尚未打开"
		return
	}
	if err := p.repo.UpdateStockRoundReview(context.Background(), p.active(),
		p.reviewRoundID, p.reviewDraft); err != nil {
		p.reviewError = err.Error()
		return
	}
	p.reviewOpen = false
	p.notify("复盘已保存", ui.ToastSuccess)
	p.Reload()
	p.changed()
}

func (p *recordsPanel) saveRoundTag(roundID, tag string) {
	if p.repo == nil {
		return
	}
	if err := p.repo.UpdateStockRoundTag(context.Background(), p.active(), roundID, tag); err != nil {
		p.notify("保存标签失败："+err.Error(), ui.ToastError)
		return
	}
	p.Reload()
	p.changed()
}

func (p *recordsPanel) openTradeEdit(trade domain.StockTrade) {
	p.editTrade = trade
	p.editPrice = domain.MilliToYuan(trade.PriceMilli)
	p.editLots = strconv.Itoa(trade.Lots)
	p.editError = ""
	p.editOpen = true
}

func (p *recordsPanel) saveTradeEdit() {
	if p.repo == nil {
		p.editError = "工作空间尚未打开"
		return
	}
	price, err := strconv.ParseFloat(strings.TrimSpace(p.editPrice), 64)
	if err != nil || price <= 0 {
		p.editError = "请输入有效的成交价"
		return
	}
	lots, err := strconv.Atoi(strings.TrimSpace(p.editLots))
	if err != nil || lots <= 0 {
		p.editError = "请输入有效的手数"
		return
	}
	request := repository.StockTradeUpdate{
		AccountID:  p.active(),
		TradeID:    p.editTrade.ID,
		PriceMilli: domain.PriceYuanToMilli(price),
		Lots:       lots,
		TradeTime:  p.editTrade.TradeTime,
	}
	if err := p.repo.UpdateStockTrade(context.Background(), request); err != nil {
		p.editError = err.Error()
		return
	}
	p.editOpen = false
	p.notify("成交已更新", ui.ToastSuccess)
	p.Reload()
	p.changed()
}

func (p *recordsPanel) askDeleteOrder(trade domain.StockTrade) {
	p.deleteID = trade.OrderID
	p.deleteNote = fmt.Sprintf("%s %s %d 手",
		domain.TradeTypeLabel(trade.TradeType), domain.MilliToYuan(trade.PriceMilli), trade.Lots)
	p.deleteOpen = true
}

func (p *recordsPanel) confirmDeleteOrder() {
	if p.repo == nil {
		p.deleteOpen = false
		return
	}
	if err := p.repo.DeleteStockOrder(context.Background(), p.active(), p.deleteID); err != nil {
		p.deleteOpen = false
		p.notify("删除委托失败："+err.Error(), ui.ToastError)
		return
	}
	p.deleteOpen = false
	p.Reload()
	p.changed()
}

// reviewModal edits one archived round's review.
func (p *recordsPanel) reviewModal(c ui.C) {
	if !p.reviewOpen {
		return
	}
	theme := c.Theme()
	ui.ModalWith(c, &p.reviewOpen, "本轮复盘", ui.ModalSpec{Width: 560}, func() {
		ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
			typography.Caption(c, "复盘")
			ui.TextArea(c, &p.reviewDraft, "复盘",
				"买入理由 / 卖出理由 / 买点审视 / 卖点审视 / 交易心得").
				FillWidth().Height(160)
			if p.reviewError != "" {
				typography.Caption(c, p.reviewError).Color(theme.Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			p.reviewOpen = false
		}
		if ui.Button(c, "保存复盘").Clicked() {
			p.saveRoundReview()
		}
	})
}

// readModal shows a day review or a saved round review in full, read-only.
func (p *recordsPanel) readModal(c ui.C) {
	if !p.readOpen {
		return
	}
	ui.ModalWith(c, &p.readOpen, p.readTitle, ui.ModalSpec{Width: 560}, func() {
		ui.Scroll(c).MaxHeight(320).Children(func() {
			ui.Wrap(c, p.readText).FontSize(typography.SizeBody)
		})
	}, func() {
		if ui.Button(c, "关闭").Outline().Clicked() {
			p.readOpen = false
		}
	})
}

func (p *recordsPanel) editModal(c ui.C) {
	if !p.editOpen {
		return
	}
	theme := c.Theme()
	ui.ModalWith(c, &p.editOpen, "编辑成交", ui.ModalSpec{
		Width:       400,
		Description: "改动后会按成交重算这一笔的手续费、持仓与资金。",
	}, func() {
		ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
			typography.Caption(c, "成交价（元）")
			ui.TextField(c, &p.editPrice, "成交价（元）", "0.00").Mono()
			typography.Caption(c, "手数")
			ui.TextField(c, &p.editLots, "手数", "0")
			if p.editError != "" {
				typography.Caption(c, p.editError).Color(theme.Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			p.editOpen = false
		}
		if ui.Button(c, "保存").Clicked() {
			p.saveTradeEdit()
		}
	})
}

func (p *recordsPanel) deleteModal(c ui.C) {
	if !p.deleteOpen {
		return
	}
	ui.ModalWith(c, &p.deleteOpen, "删除整笔委托", ui.ModalSpec{
		Width:       380,
		Description: "删除后持仓与资金会按重放结果回滚，确定继续？",
	}, func() {
		typography.Caption(c, "将删除 "+p.deleteNote+" 所在的整笔委托。")
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			p.deleteOpen = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
			p.confirmDeleteOrder()
		}
	})
}

// footer is the 40 px summary band: realized profit, its rate, the win / loss
// round count and the total number of rounds.
func (p *recordsPanel) footer(c ui.C) {
	theme := c.Theme()
	ui.Row(c).Height(layout.FooterHeight).FillWidth().PadX(layout.Inset).
		Gap(layout.GroupGap).Items(ui.Center).Justify(ui.Between).Children(func() {
		historyFooterMetric(c, "已实现盈亏", money(p.summary.TotalPnl),
			pnlColor(*theme, p.summary.TotalPnl))
		historyFooterMetric(c, "累计盈亏率", historyPercent(p.summary.TotalPnlRate),
			historyRateColor(*theme, p.summary.TotalPnlRate))
		historyFooterMetric(c, "胜负轮次",
			fmt.Sprintf("%d 胜 %d 负", p.summary.WinCount, p.summary.LossCount), theme.Foreground)
		historyFooterMetric(c, "总轮次", strconv.Itoa(p.summary.RoundCount)+" 轮", theme.Foreground)
	})
}

func historyFooterMetric(c ui.C, label, value string, color gfx.Color) {
	ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
		typography.Caption(c, label)
		ui.Text(c, value).FontSize(typography.SizeBody).Mono().Color(color)
	})
}

// ensureQuotes asks the shared quote source for the closed stocks' latest
// prices. The result lands on the next paint through the effect's Take.
func (p *recordsPanel) ensureQuotes(c ui.C) {
	if p.quotes == nil || p.quoteEffect == nil {
		return
	}
	codes := make([]string, 0, len(p.histories))
	for _, item := range p.histories {
		codes = append(codes, item.History.StockCode)
	}
	sort.Strings(codes)
	if len(codes) == 0 {
		p.quoteMap = map[string]domain.StockQuote{}
		p.quoteKey = ""
		return
	}
	key := p.active() + "|" + strings.Join(codes, ",") + "|" + strconv.Itoa(p.quoteSerial)
	if key == p.quoteKey {
		return
	}
	ui.Effect(c, p.quoteEffect, quoteRequest{Key: key, Codes: strings.Join(codes, ",")})
	if result, _, ok := p.quoteEffect.Take(); ok {
		p.quoteKey = key
		p.quoteMap = result.Quotes
	}
}

func (p *recordsPanel) latestPrice(code string) string {
	quote, ok := p.quoteMap[code]
	if !ok || quote.LatestPrice <= 0 {
		return "—"
	}
	return domain.MilliToYuan(quote.LatestPrice)
}

func (p *recordsPanel) notify(text string, kind ui.ToastKind) {
	if p.toaster != nil {
		p.toaster.Show(text, "", kind)
	}
}

func (p *recordsPanel) changed() {
	if p.onChange != nil {
		p.onChange()
	}
}

// assembleRecordHistory folds the store's flat rows into the left cards, one
// detail per closed stock and the footer summary. The list and summary use every
// trade of a round (the 0-share close marker is amount-free), while the expanded
// detail shows and counts only the fills that really traded, as the old page
// does.
func assembleRecordHistory(
	histories []domain.StockTradeHistory,
	trades []domain.StockTrade,
	rounds []domain.StockTradeRound,
	days []domain.StockDayReview,
	ledgers []domain.StockLedgerEntry,
) ([]domain.StockHistoryItem, map[string]*domain.StockHistoryDetail, domain.StockHistorySummary) {
	tradesByRound := map[string][]domain.StockTrade{}
	for _, trade := range trades {
		tradesByRound[trade.RoundID] = append(tradesByRound[trade.RoundID], trade)
	}
	daysByRound := map[string][]domain.StockDayReview{}
	for _, day := range days {
		if day.RoundID == "" {
			continue
		}
		daysByRound[day.RoundID] = append(daysByRound[day.RoundID], day)
	}
	ledgerByRound := map[string]domain.StockLedgerEntry{}
	for _, entry := range ledgers {
		if entry.RoundID != "" {
			ledgerByRound[entry.RoundID] = entry
		}
	}
	roundsByCode := map[string][]domain.StockTradeRound{}
	for _, round := range rounds {
		roundsByCode[round.StockCode] = append(roundsByCode[round.StockCode], round)
	}
	for code := range roundsByCode {
		codeRounds := roundsByCode[code]
		sort.SliceStable(codeRounds, func(i, j int) bool {
			return codeRounds[i].RoundNo < codeRounds[j].RoundNo
		})
	}

	items := make([]domain.StockHistoryItem, 0, len(histories))
	details := make(map[string]*domain.StockHistoryDetail, len(histories))
	var summary domain.StockHistorySummary
	var summaryBuyCost int64
	for _, history := range histories {
		codeRounds := roundsByCode[history.StockCode]
		detail := &domain.StockHistoryDetail{History: history}
		item := domain.StockHistoryItem{History: history, RoundCount: len(codeRounds)}
		var itemPnl, itemCost, detailRoundCost int64
		for _, round := range codeRounds {
			all := tradesByRound[round.ID]
			allBuy, _, allPnl, _ := domain.RoundPnl(all)
			itemPnl += allPnl
			itemCost += allBuy
			summary.RoundCount++
			summary.TotalPnl += allPnl
			summaryBuyCost += allBuy
			if allPnl > 0 {
				summary.WinCount++
			} else if allPnl < 0 {
				summary.LossCount++
			}
			if round.ClosedAt > item.LastClosedAt {
				item.LastClosedAt = round.ClosedAt
			}

			visible := make([]domain.StockTrade, 0, len(all))
			for _, trade := range all {
				if trade.Shares > 0 {
					visible = append(visible, trade)
				}
			}
			buyCost, _, pnl, rate := domain.RoundPnl(visible)
			roundDetail := domain.StockRoundDetail{
				Round:      round,
				Trades:     visible,
				DayReviews: daysByRound[round.ID],
				FeeCents:   domain.StockRoundFee(visible),
				PnlCents:   pnl,
				PnlRate:    rate,
			}
			if entry, ok := ledgerByRound[round.ID]; ok {
				view := ledgerViewOf(entry, visible)
				roundDetail.Ledger = &view
			}
			detail.Rounds = append(detail.Rounds, roundDetail)
			detail.TotalPnl += pnl
			detailRoundCost += buyCost
			if pnl > 0 {
				detail.WinCount++
			} else if pnl < 0 {
				detail.LossCount++
			}
		}
		detail.TotalPnlRate = domain.PercentOf(detail.TotalPnl, detailRoundCost)
		item.TotalPnl = itemPnl
		item.TotalPnlRate = domain.PercentOf(itemPnl, itemCost)
		details[history.StockCode] = detail
		items = append(items, item)
	}
	summary.StockCount = len(histories)
	summary.TotalPnlRate = domain.PercentOf(summary.TotalPnl, summaryBuyCost)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].LastClosedAt > items[j].LastClosedAt
	})
	return items, details, summary
}

// ledgerViewOf derives the read-only ledger numbers from a round's fills.
func ledgerViewOf(entry domain.StockLedgerEntry, trades []domain.StockTrade) domain.StockLedgerView {
	view := domain.StockLedgerView{Entry: entry}
	var buyAmount, buyShares, sellAmount, sellShares, buyCost, sellNet int64
	for _, trade := range trades {
		if trade.Shares <= 0 {
			continue
		}
		switch {
		case domain.IsBuy(trade.TradeType):
			buyAmount += trade.AmountCents
			buyShares += int64(trade.Shares)
			buyCost += trade.AmountCents + trade.FeeCents
		case domain.IsSell(trade.TradeType):
			sellAmount += trade.AmountCents
			sellShares += int64(trade.Shares)
			sellNet += trade.AmountCents - trade.FeeCents
		}
	}
	if buyShares > 0 {
		view.BuyPriceMilli = int64(domain.RoundHalfAwayFromZero(
			float64(buyAmount) / float64(buyShares) * 10))
	}
	view.BuyCostCents = buyCost
	if sellShares > 0 {
		price := int64(domain.RoundHalfAwayFromZero(
			float64(sellAmount) / float64(sellShares) * 10))
		net, pnl := sellNet, sellNet-buyCost
		view.SellPriceMilli, view.SellNetCents, view.NetPnlCents = &price, &net, &pnl
	}
	return view
}

// historyDate / historyShortDateTime render Unix seconds in the local zone,
// matching the old project's day / minute formats.
func historyDate(at int64) string {
	if at <= 0 {
		return "—"
	}
	return time.Unix(at, 0).Format("2006-01-02")
}

func historyShortDateTime(at int64) string {
	if at <= 0 {
		return "—"
	}
	return time.Unix(at, 0).Format("01-02 15:04")
}

func recordRoundRange(round domain.StockTradeRound) string {
	return historyDate(round.OpenedAt) + " → " + historyDate(round.ClosedAt)
}

func historyPercent(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64) + "%"
}

// historyRateColor is the A-share convention: a rise is red, a fall is green.
func historyRateColor(theme gfx.Theme, value float64) gfx.Color {
	switch {
	case value > 0:
		return semantic.Expense(theme)
	case value < 0:
		return semantic.Income(theme)
	}
	return theme.Foreground
}

func historyStockTitle(name, code string) string {
	if name == "" || name == code {
		return code
	}
	return name + " " + code
}

func historyTextOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func historyMilli(value int64) string {
	if value <= 0 {
		return "—"
	}
	return domain.MilliToYuan(value)
}

func historyContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func historyAbsInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
