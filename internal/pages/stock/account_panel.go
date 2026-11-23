package stock

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
	"transactions/internal/components/pagination"
	"transactions/internal/components/semantic"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/repository"
	"transactions/internal/stockquote"
)

type quoteRequest struct {
	Key   string
	Codes string
}

type quoteResult struct {
	Quotes map[string]domain.StockQuote
	Failed bool
}

type amountAction uint8

const (
	actionPrincipal amountAction = iota
	actionWithdraw
	actionInterest
)

type accountPanel struct {
	accounts *accountStore
	quotes   stockquote.Fetcher
	repo     repository.StockRepository
	onChange func()

	summary   domain.StockAccountSummary
	positions []domain.StockPosition
	trades    []domain.StockTrade
	fund      domain.StockFundPage

	page     int
	pageSize int

	quoteEffect *ui.EffectState[quoteResult, quoteRequest]
	quoteKey    string
	quoteSerial int
	quoteMap    map[string]domain.StockQuote
	quoteFailed bool

	errorMessage string
	amountOpen   bool
	amountKind   amountAction
	amount       string
	amountError  string

	toaster *ui.Toaster
}

func newAccountPanel(accounts *accountStore, quotes stockquote.Fetcher) *accountPanel {
	p := &accountPanel{
		accounts: accounts,
		quotes:   quotes,
		page:     1,
		pageSize: 10,
		quoteMap: map[string]domain.StockQuote{},
	}
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

func (p *accountPanel) setRepository(repo repository.StockRepository) {
	p.repo = repo
	p.Reload()
}

func (p *accountPanel) setError(message string) { p.errorMessage = message }

func (p *accountPanel) active() string {
	if p.accounts == nil {
		return ""
	}
	return p.accounts.activeID()
}

// Reload rereads the active account's cash summary, positions, trades and fund
// page. The quote request is invalidated so the next paint asks again.
func (p *accountPanel) Reload() {
	p.errorMessage = ""
	p.quoteKey = ""
	p.quoteSerial++
	if p.repo == nil || p.active() == "" {
		p.summary = domain.StockAccountSummary{}
		p.positions, p.trades = nil, nil
		p.fund = domain.StockFundPage{}
		return
	}
	ctx := context.Background()
	summary, err := p.repo.StockAccountSummary(ctx, p.active())
	if err != nil {
		p.errorMessage = "读取股票账户失败：" + err.Error()
		return
	}
	positions, err := p.repo.StockPositions(ctx, p.active())
	if err != nil {
		p.errorMessage = "读取持仓失败：" + err.Error()
		return
	}
	trades, err := p.repo.StockTrades(ctx, p.active())
	if err != nil {
		p.errorMessage = "读取成交失败：" + err.Error()
		return
	}
	fund, err := p.repo.StockFundRecords(ctx, p.active(), p.page, p.pageSize)
	if err != nil {
		p.errorMessage = "读取资金记录失败：" + err.Error()
		return
	}
	p.summary, p.positions, p.trades, p.fund = summary, positions, trades, fund
}

func (p *accountPanel) CloseOverlays() { p.amountOpen = false }

func (p *accountPanel) View(c ui.C) {
	if p.repo == nil || p.active() == "" {
		emptyAccountPage(c, "还没有股票账户", "工作空间里至少会有一个股票账户。")
		return
	}
	p.ensureQuotes(c)
	overview := p.overview()
	layout.Page(c, p.toolbar, func(c ui.C) {
		if p.errorMessage != "" {
			typography.Caption(c, p.errorMessage).Color(c.Theme().Danger).PadX(layout.Inset).PadY(6)
		}
		// The page fills its card: the summary keeps its height, the fund table
		// takes the rest and its pager sits on the bottom edge, so a tall window
		// grows the table instead of leaving a blank strip under the pager.
		ui.Column(c).Grow(1).MinHeight(0).Children(func() {
			p.hero(c, overview)
			ui.Divider(c)
			p.metrics(c, overview)
			ui.Divider(c)
			p.fundSection(c)
		})
	})
	p.amountModal(c)
}

func (p *accountPanel) toolbar(c ui.C) {
	layout.Toolbar(c, func() {
		if p.accounts.picker(c) && p.onChange != nil {
			p.onChange()
		}
		ui.Spacer(c)
		if ui.Button(c, "支取").Outline().Clicked() {
			p.openAmount(actionWithdraw)
		}
		if ui.Button(c, "利息归本").Outline().Clicked() {
			p.openAmount(actionInterest)
		}
		if ui.Button(c, "追加本金").Clicked() {
			p.openAmount(actionPrincipal)
		}
	})
}

func (p *accountPanel) hero(c ui.C, overview domain.StockOverview) {
	ui.Column(c).FillWidth().PadX(layout.Inset).PadY(layout.BlockGap).
		Gap(4).Children(func() {
		typography.Caption(c, "总资产")
		typography.Metric(c, money(overview.TotalAssets))
	})
}

func (p *accountPanel) metrics(c ui.C, overview domain.StockOverview) {
	metrics := []struct {
		label string
		value string
		pnl   *int64
	}{
		{"本金", money(overview.Principal), nil},
		{"持仓市值", money(overview.PositionMarketValue), nil},
		{"浮动盈亏", money(overview.UnrealizedPnl), &overview.UnrealizedPnl},
		{"已实现盈亏", money(overview.RealizedPnl), &overview.RealizedPnl},
		{"累计支取", money(overview.WithdrawnTotal), nil},
		{"利息归本", money(overview.InterestTotal), nil},
		{"可用现金", money(overview.AvailableCash), nil},
		{"仓位", percentOrDash(overview.PositionRatio), nil},
	}
	const perRow = 4
	ui.Column(c).FillWidth().PadX(layout.Inset).
		PadTop(layout.BlockGap).PadBottom(layout.Inset).
		Gap(layout.BlockGap).Children(func() {
		for start := 0; start < len(metrics); start += perRow {
			ui.Row(c).FillWidth().Gap(layout.ColumnGap).Children(func() {
				for index := start; index < start+perRow; index++ {
					if index >= len(metrics) {
						ui.Spacer(c).Grow(1).Basis(0)
						continue
					}
					metric := metrics[index]
					ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(4).Children(func() {
						typography.Caption(c, metric.label)
						value := ui.Text(c, metric.value).FontSize(typography.SizeSection).
							Bold().Mono().Ellipsis()
						if metric.pnl != nil {
							value.Color(pnlColor(*c.Theme(), *metric.pnl))
						}
					})
				}
			})
		}
	})
}

func (p *accountPanel) fundSection(c ui.C) {
	ui.Column(c).Grow(1).MinHeight(0).Children(func() {
		typography.Section(c, "资金变化记录").
			PadSides(layout.BlockGap, layout.Inset, layout.Gap, layout.Inset)
		if p.fund.Total == 0 {
			ui.Column(c).Grow(1).MinHeight(0).Center().Children(func() {
				typography.Caption(c, "还没有资金记录").
					Color(c.Theme().MutedForeground)
			})
			return
		}
		fundHeader(c)
		ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
			for _, record := range p.fund.Items {
				fundRow(c, record)
			}
		})
		pagination.Bar(c, pagination.Spec{
			Total: p.fund.Total, Page: p.page, PageSize: p.pageSize,
			PageSizes: []int{10, 20, 50},
			OnPage: func(page int) {
				p.page = page
				p.Reload()
			},
			OnPageSize: func(size int) {
				p.pageSize, p.page = size, 1
				p.Reload()
			},
		})
	})
}

func fundHeader(c ui.C) {
	theme := c.Theme()
	layout.TableHeader(c, layout.TableHeaderSpec{PadX: layout.Inset, Gap: layout.ColumnGap}, func(c ui.C) {
		ui.Text(c, "日期").FontSize(typography.SizeCaption).Color(theme.MutedForeground).Width(92).TextCenter()
		ui.Text(c, "事件").FontSize(typography.SizeCaption).Color(theme.MutedForeground).Width(240)
		ui.Text(c, "金额变化").FontSize(typography.SizeCaption).Color(theme.MutedForeground).Width(104).TextEnd()
		ui.Text(c, "现金余额").FontSize(typography.SizeCaption).Color(theme.MutedForeground).Width(104).TextEnd()
		ui.Text(c, "备注").FontSize(typography.SizeCaption).Color(theme.MutedForeground).Grow(1).MinWidth(0).TextEnd()
	})
}

func fundRow(c ui.C, record domain.StockFundRecord) {
	theme := c.Theme()
	ui.Row(c).MinHeight(layout.RowHeight).FillWidth().PadX(layout.Inset).
		Gap(layout.ColumnGap).Items(ui.Center).Children(func() {
		ui.Text(c, record.RecordDate).FontSize(typography.SizeBody).Mono().Width(92).TextCenter()
		ui.Text(c, record.EventText).FontSize(typography.SizeBody).Width(240).MaxLines(2)
		ui.Text(c, signedMoney(record.AmountChangeCents)).FontSize(typography.SizeBody).Mono().
			Width(104).TextEnd().Color(pnlColor(*theme, record.AmountChangeCents))
		ui.Text(c, money(record.CashBalanceCents)).FontSize(typography.SizeBody).Mono().
			Width(104).TextEnd()
		ui.Text(c, record.Remark).FontSize(typography.SizeBody).Muted().
			Grow(1).MinWidth(0).TextEnd().Ellipsis()
	})
}

func (p *accountPanel) overview() domain.StockOverview {
	byCode := make(map[string][]domain.StockTrade)
	for _, trade := range p.trades {
		byCode[trade.StockCode] = append(byCode[trade.StockCode], trade)
	}
	var realizedPnl int64
	for _, trades := range byCode {
		realizedPnl += domain.SettledPnl(trades)
	}
	held := make([]domain.StockPosition, 0, len(p.positions))
	for _, position := range p.positions {
		if position.Quantity > 0 || len(domain.CurrentRoundTrades(byCode[position.StockCode])) > 0 {
			held = append(held, position)
		}
	}
	var marketValue, unrealized, quoteFailed int64
	for _, position := range held {
		flow := domain.RoundFlowOf(byCode[position.StockCode])
		if position.Quantity <= 0 {
			unrealized += flow.CashFlow
			continue
		}
		quote, ok := p.quoteMap[position.StockCode]
		value := position.TotalCostCents
		if ok && quote.LatestPrice > 0 {
			value = domain.AmountOf(quote.LatestPrice, int64(position.Quantity))
		} else {
			quoteFailed++
		}
		marketValue += value
		unrealized += value + flow.CashFlow
	}
	overview := domain.StockOverview{
		Principal: p.summary.Principal, AvailableCash: p.summary.AvailableCash,
		PositionMarketValue: marketValue, WithdrawnTotal: p.summary.WithdrawnTotal,
		InterestTotal: p.summary.InterestTotal, RealizedPnl: realizedPnl,
		UnrealizedPnl: unrealized, QuoteFailedCount: int(quoteFailed),
	}
	overview.TotalAssets = overview.AvailableCash + marketValue
	if overview.Principal > 0 {
		overview.TotalPnlPercent = domain.PercentOf(realizedPnl+unrealized, overview.Principal)
	}
	base := overview.Principal + realizedPnl + overview.InterestTotal - overview.WithdrawnTotal
	if base > 0 {
		invested := base - overview.AvailableCash
		if invested < 0 {
			invested = 0
		}
		ratio := domain.PercentOf(invested, base)
		overview.PositionRatio = &ratio
	}
	return overview
}

func (p *accountPanel) ensureQuotes(c ui.C) {
	byCode := make(map[string][]domain.StockTrade)
	for _, trade := range p.trades {
		byCode[trade.StockCode] = append(byCode[trade.StockCode], trade)
	}
	codeSet := make(map[string]bool)
	for _, position := range p.positions {
		if position.Quantity > 0 || len(domain.CurrentRoundTrades(byCode[position.StockCode])) > 0 {
			codeSet[position.StockCode] = true
		}
	}
	codes := make([]string, 0, len(codeSet))
	for code := range codeSet {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	if len(codes) == 0 {
		p.quoteKey = ""
		p.quoteMap = map[string]domain.StockQuote{}
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
		p.quoteFailed = result.Failed
	}
}

func (p *accountPanel) openAmount(kind amountAction) {
	p.amountKind, p.amountOpen, p.amount, p.amountError = kind, true, "", ""
}

func (p *accountPanel) amountTitle() string {
	switch p.amountKind {
	case actionWithdraw:
		return "支取"
	case actionInterest:
		return "利息归本"
	}
	return "追加本金"
}

func (p *accountPanel) amountButton() string {
	switch p.amountKind {
	case actionWithdraw:
		return "支取"
	case actionInterest:
		return "归本"
	}
	return "追加"
}

func (p *accountPanel) amountModal(c ui.C) {
	if !p.amountOpen {
		return
	}
	ui.Modal(c, &p.amountOpen, p.amountTitle(), func() {
		ui.Column(c).Gap(layout.BlockGap).Children(func() {
			typography.Caption(c, "金额（元）")
			ui.TextField(c, &p.amount, "金额（元）", "0.00").Grow(1).Mono()
			if p.amountError != "" {
				typography.Caption(c, p.amountError).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			p.amountOpen = false
		}
		if ui.Button(c, p.amountButton()).Clicked() {
			p.submitAmount()
		}
	})
}

func (p *accountPanel) submitAmount() {
	cents, err := domain.YuanToCents(p.amount)
	if err != nil || cents <= 0 {
		p.amountError = "请填写大于 0 的金额"
		return
	}
	ctx := context.Background()
	switch p.amountKind {
	case actionWithdraw:
		err = p.repo.WithdrawStock(ctx, p.active(), cents, "")
	case actionInterest:
		err = p.repo.AddStockInterest(ctx, p.active(), cents, "")
	default:
		err = p.repo.AddStockPrincipal(ctx, p.active(), cents, "")
	}
	if err != nil {
		p.amountError = err.Error()
		return
	}
	title := p.amountTitle()
	p.amountOpen = false
	if p.toaster != nil {
		p.toaster.Show(title+"已记录", "", ui.ToastSuccess)
	}
	p.Reload()
	if p.onChange != nil {
		p.onChange()
	}
}

func money(cents int64) string { return "¥" + domain.CentsToYuan(cents) }

func signedMoney(cents int64) string {
	if cents < 0 {
		return "-¥" + domain.CentsToYuan(-cents)
	}
	return "+¥" + domain.CentsToYuan(cents)
}

func pnlColor(theme gfx.Theme, value int64) gfx.Color {
	switch {
	case value > 0:
		return semantic.Expense(theme)
	case value < 0:
		return semantic.Income(theme)
	}
	return theme.Foreground
}

func percentOrDash(value *float64) string {
	if value == nil {
		return "—"
	}
	return strconv.FormatFloat(*value, 'f', 2, 64) + "%"
}

func emptyAccountPage(c ui.C, title, note string) {
	ui.Column(c).Fill().Pad(layout.Inset).Gap(layout.Gap).Children(func() {
		typography.Title(c, title)
		typography.Body(c, note).Muted()
	})
}
