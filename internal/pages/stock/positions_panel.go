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

	"transactions/internal/components/datepicker"
	"transactions/internal/components/emptystate"
	"transactions/internal/components/layout"
	"transactions/internal/components/selectbox"
	"transactions/internal/components/semantic"
	"transactions/internal/components/state"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/repository"
	"transactions/internal/stockquote"
)

// positionsPanel is the 持仓 screen: the account's open positions in a 280 px
// rail, and the chosen position's detail on the right — quotes, the round's fee,
// the trade ledger, the daily reviews, the round review and the current round's
// trades. Every write goes through the account's repository; the panel keeps no
// second copy of the active account, it reads accountStore.
type positionsPanel struct {
	accounts *accountStore
	quotes   stockquote.Fetcher
	repo     repository.StockRepository
	// onChange is set by stock.Model to refresh the section after a write or an
	// account switch. toaster is the section's toast stack.
	onChange func()
	toaster  *ui.Toaster

	positions    []domain.StockPosition
	tradesByCode map[string][]domain.StockTrade
	dayReviews   []domain.StockDayReview
	tags         []string
	fee          domain.StockFeeSetting
	templates    domain.StockReviewSetting
	selectedCode string

	quoteEffect *ui.EffectState[quoteResult, quoteRequest]
	quoteKey    string
	quoteSerial int
	quoteMap    map[string]domain.StockQuote

	refreshEffect *ui.EffectState[positionsNameResult, positionsNameRequest]
	refreshKey    string
	refreshDone   string
	refreshCode   string
	refreshSerial int

	tradeNameEffect *ui.EffectState[positionsNameResult, positionsNameRequest]
	tradeNameKey    string
	tradeNameDone   string
	tradeAutoName   string

	errorMessage string

	ledger      *domain.StockLedgerEntry
	ledgerError string
	ledgerEdit  ledgerEditor

	dailyOpen     bool
	dailyEditDate string
	dailyDate     string
	dailyText     string
	dailyError    string
	dailyReadOpen bool
	dailyRead     domain.StockDayReview
	dailyPick     datepicker.State

	roundEditOpen bool
	roundReadOpen bool
	roundReadText string
	roundDraft    string
	roundError    string

	tradeOpen  bool
	tradeType  string
	tradeCode  string
	tradeName  string
	tradeFills []positionsTradeFill
	tradeDate  string
	tradeTag   string
	tradeError string
	tradeFee   positionsFeeForm
	tradePick  datepicker.State

	editOpen    bool
	editTradeID string
	editPrice   string
	editLots    string
	editError   string

	deleteOpen    bool
	deleteOrderID string
	deleteError   string
}

// positionsNameRequest / positionsNameResult are the name-lookup effect's key
// and value. Mode tells the consumer whether a refresh or a new order asked.
type positionsNameRequest struct {
	Key  string
	Code string
	Mode string
}

type positionsNameResult struct {
	Code   string
	Name   string
	Mode   string
	Failed bool
}

// positionsTradeFill is one fill row of the order dialog: the price is yuan as
// typed, the lots an integer.
type positionsTradeFill struct {
	price string
	lots  string
}

// positionsFeeForm is the four fee inputs of 建仓, in the units the form
// showed: commission in 万分之, the minimum in yuan, the two rates in percent.
type positionsFeeForm struct {
	commission string
	min        string
	stamp      string
	transfer   string
}

// positionsLedgerNumbers are the read-only numbers derived from a round's
// trades: average buy / sell price, buy cost, sell net and net profit.
type positionsLedgerNumbers struct {
	buyPrice    string
	buyCost     string
	sellPrice   string
	sellNet     string
	netPnl      string
	netPnlCents int64
	hasBuy      bool
	hasSell     bool
}

const (
	positionsTradeTimeWidth   float32 = 168
	positionsTradeTypeWidth   float32 = 56
	positionsTradePriceWidth  float32 = 96
	positionsTradeLotsWidth   float32 = 64
	positionsTradeAmountWidth float32 = 120
	positionsTradeFeeWidth    float32 = 96
	positionsTradeCashWidth   float32 = 120
	positionsTradeActionWidth float32 = 56
	// The table carries one spacer before 操作, so it has eight gaps.
	positionsTradeTableWidth = positionsTradeTimeWidth + positionsTradeTypeWidth +
		positionsTradePriceWidth + positionsTradeLotsWidth + positionsTradeAmountWidth +
		positionsTradeFeeWidth + positionsTradeCashWidth + positionsTradeActionWidth +
		8*layout.Gap + 2*layout.Gap
)

func newPositionsPanel(accounts *accountStore, quotes stockquote.Fetcher) *positionsPanel {
	p := &positionsPanel{
		accounts:     accounts,
		quotes:       quotes,
		tradesByCode: map[string][]domain.StockTrade{},
		quoteMap:     map[string]domain.StockQuote{},
		tradeFills:   []positionsTradeFill{{}},
		dailyDate:    domain.DiaryToday(),
		tradeDate:    domain.DiaryToday(),
		dailyPick:    datepicker.NewState(),
		tradePick:    datepicker.NewState(),
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
	p.refreshEffect = p.newNameEffect()
	p.tradeNameEffect = p.newNameEffect()
	return p
}

func (p *positionsPanel) newNameEffect() *ui.EffectState[positionsNameResult, positionsNameRequest] {
	return ui.NewEffectKeyed(func(ctx context.Context, request positionsNameRequest) (positionsNameResult, error) {
		result := positionsNameResult{Code: request.Code, Mode: request.Mode}
		if p.quotes == nil {
			result.Failed = true
			return result, nil
		}
		name, err := p.quotes.Name(ctx, request.Code)
		if err != nil {
			result.Failed = true
			return result, nil
		}
		result.Name = strings.TrimSpace(name)
		return result, nil
	}).Sequential()
}

// setRepository connects the panel to a workspace and reads the active
// account. stock.Model calls it from SetRepository when the panel is wired.
func (p *positionsPanel) setRepository(repo repository.StockRepository) {
	p.repo = repo
	p.Reload()
}

func (p *positionsPanel) active() string {
	if p.accounts == nil {
		return ""
	}
	return p.accounts.activeID()
}

// Reload rereads the active account: positions, trades, current-round day
// reviews, tags, the account's fee setting, the review templates and the
// selected position's ledger. It also invalidates the quote and name effects.
func (p *positionsPanel) Reload() {
	p.errorMessage = ""
	p.quoteKey = ""
	p.quoteSerial++
	p.refreshKey, p.refreshDone = "", ""
	p.tradeNameKey, p.tradeNameDone = "", ""

	if p.repo == nil || p.active() == "" {
		p.positions = nil
		p.tradesByCode = map[string][]domain.StockTrade{}
		p.dayReviews = nil
		p.tags = nil
		p.fee = domain.StockFeeSetting{}
		p.templates = domain.StockReviewSetting{}
		p.selectedCode = ""
		p.ledger, p.ledgerError = nil, ""
		p.quoteMap = map[string]domain.StockQuote{}
		return
	}

	ctx := context.Background()
	p.positions = nil
	p.tradesByCode = map[string][]domain.StockTrade{}
	p.dayReviews = nil
	p.tags = nil
	p.fee = domain.StockFeeSetting{}
	p.templates = domain.StockReviewSetting{}
	p.ledger, p.ledgerError = nil, ""
	p.quoteMap = map[string]domain.StockQuote{}
	positions, err := p.repo.StockPositions(ctx, p.active())
	if err != nil {
		p.errorMessage = "读取持仓失败：" + positionsErrorText(err)
		return
	}
	trades, err := p.repo.StockTrades(ctx, p.active())
	if err != nil {
		p.errorMessage = "读取成交失败：" + positionsErrorText(err)
		return
	}
	dayReviews, err := p.repo.StockDayReviews(ctx, p.active())
	if err != nil {
		p.errorMessage = "读取每日复盘失败：" + positionsErrorText(err)
		return
	}
	tags, err := p.repo.StockTradeTags(ctx, p.active())
	if err != nil {
		p.errorMessage = "读取交易标签失败：" + positionsErrorText(err)
		return
	}
	fee, err := p.repo.StockFeeSetting(ctx, p.active())
	if err != nil {
		p.errorMessage = "读取费用设置失败：" + positionsErrorText(err)
		return
	}
	templates, err := p.repo.StockReviewTemplates(ctx, p.active())
	if err != nil {
		p.errorMessage = "读取复盘模板失败：" + positionsErrorText(err)
		return
	}

	byCode := make(map[string][]domain.StockTrade)
	for _, trade := range trades {
		byCode[trade.StockCode] = append(byCode[trade.StockCode], trade)
	}
	held := make([]domain.StockPosition, 0, len(positions))
	for _, position := range positions {
		if position.Quantity > 0 || len(domain.CurrentRoundTrades(byCode[position.StockCode])) > 0 {
			held = append(held, position)
		}
	}

	p.positions, p.tradesByCode = held, byCode
	p.dayReviews, p.tags = dayReviews, tags
	p.fee, p.templates = fee, templates
	if !positionsHasCode(held, p.selectedCode) {
		p.selectedCode = ""
		if len(held) > 0 {
			p.selectedCode = held[0].StockCode
		}
	}
	p.loadLedger()
}

// CloseOverlays closes every dialog and picker the screen can open.
func (p *positionsPanel) CloseOverlays() {
	p.ledgerEdit.close()
	p.dailyOpen = false
	p.dailyReadOpen = false
	p.roundEditOpen = false
	p.roundReadOpen = false
	p.tradeOpen = false
	p.editOpen = false
	p.deleteOpen = false
	p.dailyPick.Open = false
	p.tradePick.Open = false
}

func (p *positionsPanel) loadLedger() {
	p.ledger, p.ledgerError = nil, ""
	if p.repo == nil || p.active() == "" || p.selectedCode == "" {
		return
	}
	entry, err := p.repo.StockLedger(context.Background(), p.active(), p.selectedCode, "")
	if err != nil {
		p.ledgerError = "读取交易台账失败：" + positionsErrorText(err)
		return
	}
	p.ledger = entry
}

// View declares the 持仓 screen.
func (p *positionsPanel) View(c ui.C) {
	if p.repo == nil || p.active() == "" {
		emptyAccountPage(c)
		return
	}
	p.ensureQuotes(c)
	p.ensureRefreshName(c)

	layout.Page(c, p.toolbar, func(c ui.C) {
		if p.errorMessage != "" {
			layout.ErrorBanner(c, p.errorMessage)
		}
		ui.Row(c).Grow(1).Children(func() {
			p.rail(c)
			ui.Divider(c)
			p.detailColumn(c)
		})
	})
	p.dialogs(c)
}

func (p *positionsPanel) toolbar(c ui.C) {
	layout.Toolbar(c, func() {
		if p.accounts != nil && p.accounts.picker(c) {
			p.CloseOverlays()
			p.Reload()
			if p.onChange != nil {
				p.onChange()
			}
		}
		ui.Spacer(c)
		if ui.Button(c, "建仓").Clicked() {
			p.openTrade(domain.TradeOpen, nil)
		}
	})
}

// ------------------------------------------------------------------ 左栏

func (p *positionsPanel) rail(c ui.C) {
	layout.Rail(c, layout.RailWidth, func(c ui.C) {
		layout.SectionHeader(c, layout.Inset).Children(func() {
			typography.Section(c, "持仓")
			layout.SectionCount(c, strconv.Itoa(len(p.positions))+" 只")
			ui.Spacer(c)
		})
		if len(p.positions) == 0 {
			emptystate.Notice(c, "暂无持仓")
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(4).PadSides(4, 8, 4, 4).Children(func() {
				for i := range p.positions {
					position := &p.positions[i]
					p.positionCard(c, position)
				}
			})
		})
	})
}

func (p *positionsPanel) positionCard(c ui.C, position *domain.StockPosition) {
	theme := c.Theme()
	selected := position.StockCode == p.selectedCode
	quote, hasQuote := p.quoteMap[position.StockCode]
	knownQuote := hasQuote && quote.LatestPrice > 0

	marketValue := position.TotalCostCents
	if knownQuote {
		marketValue = domain.AmountOf(quote.LatestPrice, int64(position.Quantity))
	}
	flow := domain.RoundFlowOf(p.tradesByCode[position.StockCode])
	pnl := marketValue + flow.CashFlow
	rate := domain.FloatingPnlRate(pnl, flow.CashFlow)

	priceText := "—"
	changeText := "—"
	var changeValue int64
	if knownQuote {
		priceText = domain.MilliToYuan(quote.LatestPrice)
		changeValue = quote.LatestPrice - quote.PrevClose
		if percent := domain.QuoteChangePercent(quote.LatestPrice, quote.PrevClose); percent != nil {
			changeText = positionsSignedPercent(percent)
		}
	}

	card := ui.Column(c.Key(position.StockCode)).FillWidth().
		Pad(layout.BlockGap).Gap(4).
		Radius(theme.Metrics.RadiusControl).
		HitWith(ui.ID(c), position.StockName+" "+position.StockCode, gfx.HitOptions{
			Cursor: gfx.PointerCursorHand, Selected: selected,
		}).
		Hover(ui.Look{}.Background(state.Hover(*theme)))
	if selected {
		card = card.Background(state.Selected(*theme))
	}
	// MutedForeground on the selected wash is 4.24:1, so the card's own small
	// text lifts to the foreground while the row is chosen.
	label := theme.MutedForeground
	if selected {
		label = theme.Foreground
	}
	card.Children(func() {
		ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
			name := position.StockName
			if strings.TrimSpace(name) == "" {
				name = position.StockCode
			}
			typography.BodyStrong(c, name).Grow(1).Ellipsis()
			ui.Text(c, position.StockCode).FontSize(typography.SizeCaption).
				Mono().Color(label)
		})
		ui.Row(c).Gap(layout.Gap).Items(ui.Start).Children(func() {
			ui.Column(c).Grow(1).Gap(1).Children(func() {
				ui.Text(c, "现价").FontSize(typography.SizeCaption).Color(label)
				ui.Row(c).Gap(4).Items(ui.Baseline).Children(func() {
					ui.Text(c, priceText).FontSize(typography.SizeBody).Mono()
					ui.Text(c, changeText).FontSize(typography.SizeCaption).
						Mono().Color(pnlColor(*theme, changeValue))
				})
			})
			ui.Column(c).Grow(1).Gap(1).Children(func() {
				ui.Text(c, "浮动盈亏").FontSize(typography.SizeCaption).
					Color(label).TextEnd()
				pnlText := ui.Text(c, money(pnl)).FontSize(typography.SizeBody).
					Mono().TextEnd()
				if pnl != 0 {
					pnlText.Color(pnlColor(*theme, pnl))
				}
			})
		})
		ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
			ui.Text(c, positionsLotsText(position.Quantity)).
				FontSize(typography.SizeBody).Color(label)
			ui.Spacer(c)
			ui.Text(c, "浮盈率").FontSize(typography.SizeCaption).Color(label)
			rateText := ui.Text(c, percentOrDash(rate)).FontSize(typography.SizeBody).Mono()
			if pnl != 0 {
				rateText.Color(pnlColor(*theme, pnl))
			}
		})
	})
	if card.Clicked() {
		p.selectPosition(position.StockCode)
	}
}

func (p *positionsPanel) selectPosition(code string) {
	if p.selectedCode == code {
		return
	}
	p.selectedCode = code
	p.ledgerEdit.close()
	p.roundEditOpen = false
	p.roundReadOpen = false
	p.dailyOpen = false
	p.dailyReadOpen = false
	p.loadLedger()
}

// ------------------------------------------------------------------ 右栏

// detailColumn is the right side: a fixed 44 px header like the records page,
// then the scrollable metrics, ledger and reviews.
func (p *positionsPanel) detailColumn(c ui.C) {
	ui.Column(c).Grow(1).FillHeight().MinWidth(0).Children(func() {
		position := p.selectedPosition()
		if position == nil {
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Pad(layout.Inset).Gap(layout.BlockGap).Children(func() {
					if len(p.positions) == 0 {
						typography.Section(c, "暂无持仓")
						typography.Body(c, "点顶部「建仓」记录第一笔买入。").Muted()
					}
				})
			})
			return
		}
		p.detailHeader(c, position)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				ui.Column(c).Pad(layout.Inset).PadBottom(layout.BlockGap).
					Gap(layout.Gap).Children(func() {
					p.detailMetrics(c, position)
					p.feeLine(c, position)
				})
				ui.Divider(c)
				p.ledgerSection(c, position)
				ui.Divider(c)
				p.dailySection(c, position)
				ui.Divider(c)
				p.roundSection(c, position)
				ui.Divider(c)
				p.tradesSection(c, position)
			})
		})
	})
}

func (p *positionsPanel) detailHeader(c ui.C, position *domain.StockPosition) {
	theme := c.Theme()
	name := position.StockName
	if strings.TrimSpace(name) == "" {
		name = position.StockCode
	}
	layout.SectionHeader(c, layout.Inset).Children(func() {
		typography.Section(c, name).Ellipsis()
		ui.Text(c, position.StockCode).FontSize(typography.SizeCaption).
			Mono().Color(theme.MutedForeground).Shrink(0)
		if ui.Button(c, "刷新").Small().Ghost().Shrink(0).Disabled(p.quotes == nil).
			Tooltip("按代码重新查询名称").Clicked() {
			p.beginNameRefresh(position.StockCode)
		}
		ui.Spacer(c)
		ui.Row(c).Shrink(0).Gap(6).Items(ui.Center).Children(func() {
			tag := domain.EffectivePositionTag(position.Tag)
			if selectbox.Select(c, &tag, p.tagChoicesWith(tag), nil).
				Label("交易标签").Width(88).Changed() {
				p.saveTag(position.StockCode, tag)
			}
			if ui.Button(c, "清仓").Small().Clicked() {
				p.openTrade(domain.TradeClose, position)
			}
			if ui.Button(c, "减仓").Outline().Small().Clicked() {
				p.openTrade(domain.TradeReduce, position)
			}
			if ui.Button(c, "加仓").Outline().Small().Clicked() {
				p.openTrade(domain.TradeAdd, position)
			}
		})
	})
}

func (p *positionsPanel) detailMetrics(c ui.C, position *domain.StockPosition) {
	theme := c.Theme()
	quote, hasQuote := p.quoteMap[position.StockCode]
	knownQuote := hasQuote && quote.LatestPrice > 0

	marketValue := position.TotalCostCents
	if knownQuote {
		marketValue = domain.AmountOf(quote.LatestPrice, int64(position.Quantity))
	}
	flow := domain.RoundFlowOf(p.tradesByCode[position.StockCode])
	pnl := marketValue + flow.CashFlow
	rate := domain.FloatingPnlRate(pnl, flow.CashFlow)

	priceText := "—"
	changeText, changeNote := "—", ""
	var changeValue int64
	if knownQuote {
		priceText = domain.MilliToYuan(quote.LatestPrice)
		changeText, changeNote, changeValue = positionsQuoteChange(quote)
	}

	ui.Row(c).FillWidth().Gap(layout.ColumnGap).Children(func() {
		positionsMetric(c, "现价", priceText, "", theme.Foreground)
		changeColor := theme.Foreground
		if knownQuote && changeValue != 0 {
			changeColor = pnlColor(*theme, changeValue)
		}
		positionsMetric(c, "当日涨跌", changeText, changeNote, changeColor)
		positionsMetric(c, "持仓市值", money(marketValue), "", theme.Foreground)
		positionsMetric(c, "浮动盈亏", money(pnl), percentOrDash(rate),
			pnlColor(*theme, pnl))
	})
}

// positionsMetric is one reading of the detail header: the Caption label, the
// Figure and, when the reading has a percentage beside it, that percentage as a
// Caption on the same baseline. The note stays a Caption in every cell that has
// one (day change, floating profit), so the two percentages read as the same
// kind of number and the row keeps one type scale.
func positionsMetric(c ui.C, label, value, note string, color gfx.Color) {
	ui.Column(c).Grow(1).Basis(0).Gap(4).Children(func() {
		typography.Caption(c, label)
		ui.Row(c).Gap(6).Items(ui.Baseline).Children(func() {
			typography.Figure(c, value).Color(color)
			if note != "" {
				typography.Caption(c, note).Mono().Color(color)
			}
		})
	})
}

func (p *positionsPanel) feeLine(c ui.C, position *domain.StockPosition) {
	theme := c.Theme()
	ui.Row(c).Gap(layout.Gap).Items(ui.Baseline).Children(func() {
		typography.Caption(c, "本轮费率")
		ui.Text(c, positionsFeeText(p.roundFeeOf(position.StockCode), p.fee)).
			FontSize(typography.SizeCaption).Mono().
			Color(theme.MutedForeground).Grow(1).Ellipsis()
	})
}

// ------------------------------------------------------------------ 交易台账

func (p *positionsPanel) ledgerSection(c ui.C, position *domain.StockPosition) {
	sheet := p.ledgerSheet(position)
	stockSection(c, "交易台账", layout.Inset, func() {
		if ui.Button(c, ledgerActionLabel(sheet)).Outline().Small().Clicked() {
			p.openLedger()
		}
	}, func() {
		ui.Column(c).FillWidth().PadX(layout.Inset).Children(func() {
			ledgerSectionBody(c, sheet, p.ledgerError, p.openLedger)
		})
	})
}

// ledgerSheet maps the selected position's ledger and current-round numbers to
// the shared sheet the records page paints too.
func (p *positionsPanel) ledgerSheet(position *domain.StockPosition) ledgerSheet {
	numbers := positionsLedgerNumbersOf(p.currentTrades(position.StockCode))
	sheet := ledgerSheet{
		Entry:    p.ledger,
		BuyPrice: numbers.buyPrice,
		BuyCost:  numbers.buyCost,
		Closed:   position.Quantity == 0 && numbers.hasSell,
	}
	if sheet.Closed {
		sheet.SellPrice = numbers.sellPrice
		sheet.SellNet = numbers.sellNet
		sheet.NetPnl = numbers.netPnl
		sheet.NetCents = numbers.netPnlCents
		sheet.HasNet = true
	}
	return sheet
}

// ------------------------------------------------------------------ 每日复盘

func (p *positionsPanel) dailySection(c ui.C, position *domain.StockPosition) {
	theme := c.Theme()
	reviews := p.currentDayReviews(position.StockCode)
	stockSection(c, "每日复盘", layout.Inset, func() {
		if ui.Button(c, "写今日").Outline().Small().Clicked() {
			p.openDailyToday(position)
		}
	}, func() {
		ui.Column(c).FillWidth().PadX(layout.Inset).Children(func() {
			if len(reviews) == 0 {
				row := ui.Row(c.Key("daily-empty")).FillWidth().Height(layout.RowHeight).
					Gap(layout.Gap).Items(ui.Center).
					HitWith(ui.ID(c), "写今日的每日复盘", gfx.HitOptions{Cursor: gfx.PointerCursorHand}).
					Hover(ui.Look{}.Background(theme.Hover)).
					Radius(theme.Metrics.RadiusControl)
				row.Children(func() {
					typography.Body(c, "还没有写每日复盘，点「写今日」记下今天的盘面和判断。").
						Muted()
				})
				if row.Clicked() {
					p.openDailyToday(position)
				}
				return
			}
			for _, review := range reviews {
				review := review
				row := ui.Row(c.Key(review.ID)).FillWidth().Height(layout.RowHeight).
					Gap(layout.Gap).Items(ui.Center).
					HitWith(ui.ID(c), "查看 "+review.ReviewDate+" 的每日复盘",
						gfx.HitOptions{Cursor: gfx.PointerCursorHand}).
					Hover(ui.Look{}.Background(theme.Hover)).
					Radius(theme.Metrics.RadiusControl)
				row.Children(func() {
					ui.Text(c, review.ReviewDate).FontSize(typography.SizeBody).Mono().Width(88)
					ui.Text(c, review.Review).FontSize(typography.SizeBody).Grow(1).Ellipsis()
				})
				if row.Clicked() {
					p.openDailyRead(review)
				}
			}
		})
	})
}

// ------------------------------------------------------------------ 本轮复盘

func (p *positionsPanel) roundSection(c ui.C, position *domain.StockPosition) {
	theme := c.Theme()
	filled := strings.TrimSpace(position.Review) != ""
	text := position.Review
	if !filled {
		text = "还没有写本轮复盘，点「编辑」记录买入与卖出的理由。"
	}
	stockSection(c, "本轮复盘", layout.Inset, func() {
		if ui.Button(c, "编辑").Outline().Small().Clicked() {
			p.openRoundEdit()
		}
	}, func() {
		ui.Column(c).FillWidth().PadX(layout.Inset).Children(func() {
			row := ui.Row(c.Key("round-review")).FillWidth().Height(layout.RowHeight).
				Gap(layout.Gap).Items(ui.Center).
				HitWith(ui.ID(c), "查看本轮复盘", gfx.HitOptions{Cursor: gfx.PointerCursorHand}).
				Hover(ui.Look{}.Background(theme.Hover)).
				Radius(theme.Metrics.RadiusControl)
			row.Children(func() {
				preview := ui.Text(c, text).FontSize(typography.SizeBody).Grow(1).Ellipsis()
				if !filled {
					preview.Color(theme.MutedForeground)
				}
			})
			if row.Clicked() {
				if filled {
					p.openRoundRead()
				} else {
					p.openRoundEdit()
				}
			}
		})
	})
}

// ------------------------------------------------------------------ 当前轮成交

func (p *positionsPanel) tradesSection(c ui.C, position *domain.StockPosition) {
	trades := p.visibleTrades(position.StockCode)
	var action func()
	if len(trades) > 0 {
		action = func() {
			typography.CaptionMoney(c, "费用合计 "+money(domain.StockRoundFee(trades)))
		}
	}
	stockSection(c, "成交记录", layout.Inset, action, func() {
		ui.Column(c).FillWidth().PadX(layout.Inset).Children(func() {
			if len(trades) == 0 {
				typography.Body(c, "本轮暂无成交").Muted().PadY(layout.BlockGap)
				return
			}
			// The table is wider than the detail column and wider than it is
			// comfortable to squeeze: give the horizontal scroll the table's own
			// height, because a Scroll with no height of its own collapses.
			height := layout.TableHeaderHeight +
				layout.RowHeight*float32(len(trades)) + 12
			ui.Scroll(c).Height(height).Sideways().Scrollbars().Children(func() {
				ui.Column(c).FillWidth().MinWidth(positionsTradeTableWidth).Children(func() {
					p.tradeHeader(c)
					for _, trade := range trades {
						p.tradeRow(c, trade)
					}
				})
			})
		})
	})
}

func (p *positionsPanel) tradeHeader(c ui.C) {
	layout.TableHeader(c, layout.TableHeaderSpec{PadX: layout.Gap, Gap: layout.Gap}, func(c ui.C) {
		positionsTradeHeaderCell(c, "时间", positionsTradeTimeWidth, tradeAlignCenter)
		positionsTradeHeaderCell(c, "类型", positionsTradeTypeWidth, tradeAlignCenter)
		positionsTradeHeaderCell(c, "成交价", positionsTradePriceWidth, tradeAlignEnd)
		positionsTradeHeaderCell(c, "手数", positionsTradeLotsWidth, tradeAlignEnd)
		positionsTradeHeaderCell(c, "成交金额", positionsTradeAmountWidth, tradeAlignEnd)
		positionsTradeHeaderCell(c, "费用", positionsTradeFeeWidth, tradeAlignEnd)
		positionsTradeHeaderCell(c, "资金变动", positionsTradeCashWidth, tradeAlignEnd)
		ui.Spacer(c)
		positionsTradeHeaderCell(c, "操作", positionsTradeActionWidth, tradeAlignCenter)
	})
}

type tradeCellAlign uint8

const (
	tradeAlignCenter tradeCellAlign = iota
	tradeAlignEnd
)

func positionsTradeHeaderCell(c ui.C, label string, width float32, align tradeCellAlign) {
	box := ui.Text(c, label).FontSize(typography.SizeCaption).Width(width).
		Color(c.Theme().MutedForeground)
	switch align {
	case tradeAlignCenter:
		box.TextCenter()
	case tradeAlignEnd:
		box.TextEnd()
	}
}

func (p *positionsPanel) tradeRow(c ui.C, trade domain.StockTrade) {
	theme := c.Theme()
	cash := trade.AmountCents - trade.FeeCents
	if domain.IsBuy(trade.TradeType) {
		cash = -(trade.AmountCents + trade.FeeCents)
	}
	ui.Row(c.Key(trade.ID)).Height(layout.RowHeight).FillWidth().
		PadX(layout.Gap).Gap(layout.Gap).Items(ui.Center).Children(func() {
		ui.Text(c, positionsDateTime(trade.TradeTime)).FontSize(typography.SizeBody).
			Mono().Width(positionsTradeTimeWidth).TextCenter()
		ui.Text(c, domain.TradeTypeLabel(trade.TradeType)).FontSize(typography.SizeBody).
			Color(positionsTradeTypeColor(*theme, trade.TradeType)).
			Width(positionsTradeTypeWidth).TextCenter()
		ui.Text(c, domain.MilliToYuan(trade.PriceMilli)).FontSize(typography.SizeBody).
			Mono().Width(positionsTradePriceWidth).TextEnd()
		ui.Text(c, strconv.Itoa(trade.Lots)).FontSize(typography.SizeBody).
			Mono().Width(positionsTradeLotsWidth).TextEnd()
		ui.Text(c, money(trade.AmountCents)).FontSize(typography.SizeBody).
			Mono().Width(positionsTradeAmountWidth).TextEnd()
		ui.Text(c, money(trade.FeeCents)).FontSize(typography.SizeBody).
			Mono().Width(positionsTradeFeeWidth).TextEnd()
		ui.Text(c, signedMoney(cash)).FontSize(typography.SizeBody).
			Mono().Width(positionsTradeCashWidth).TextEnd().Color(pnlColor(*theme, cash))
		ui.Spacer(c)
		ui.Row(c).Width(positionsTradeActionWidth).Gap(2).Items(ui.Center).Children(func() {
			if ui.Button(c, "编辑").Ghost().Small().IconOnly().
				Icon(icons.Pencil).Tooltip("编辑成交").Clicked() {
				p.openTradeEdit(trade)
			}
			if ui.Button(c, "删除").Ghost().Small().IconOnly().
				Icon(icons.Trash2).Tooltip("删除整笔委托").Clicked() {
				p.openDeleteOrder(trade)
			}
		})
	})
}

func positionsTradeTypeColor(theme gfx.Theme, tradeType string) gfx.Color {
	switch tradeType {
	case domain.TradeOpen:
		return semantic.Expense(theme)
	case domain.TradeClose:
		return semantic.Income(theme)
	}
	return theme.MutedForeground
}

// ------------------------------------------------------------------ 弹窗

func (p *positionsPanel) dialogs(c ui.C) {
	p.ledgerEdit.dialog(c, p.repo, p.active())
	p.dailyDialog(c)
	p.dailyReadDialog(c)
	p.roundEditDialog(c)
	p.roundReadDialog(c)
	p.tradeDialog(c)
	p.tradeEditDialog(c)
	p.deleteDialog(c)
}

// positionsFormColumn is a labelled control that shares a Row's width.
func positionsFormColumn(c ui.C, label string, control func()) {
	ui.Column(c).Grow(1).Gap(4).Children(func() {
		typography.Caption(c, label)
		control()
	})
}

// positionsFormField is a labelled control stacked in a Column; it keeps its
// content height instead of taking the column's leftover space.
func positionsFormField(c ui.C, label string, control func()) {
	ui.Column(c).Gap(4).Children(func() {
		typography.Caption(c, label)
		control()
	})
}

func (p *positionsPanel) dailyDialog(c ui.C) {
	if !p.dailyOpen {
		return
	}
	title := "每日复盘"
	if p.dailyEditDate != "" {
		title = "每日复盘 · " + p.dailyEditDate
	}
	ui.ModalWith(c, &p.dailyOpen, title, ui.ModalSpec{Width: 560}, func() {
		ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
			if p.dailyEditDate != "" {
				ui.Text(c, p.dailyEditDate).FontSize(typography.SizeBody).Mono()
			} else {
				datepicker.Picker(c, &p.dailyPick, datepicker.Spec{
					Label: "日期", Start: p.dailyDate, Grain: datepicker.ByDay,
					Fill: true, CloseOnPick: true, MaxDate: domain.DiaryToday(),
					Viewport: c.Bounds(),
					OnRange:  func(start, _ string) { p.dailyDate = start },
				})
			}
			ui.TextArea(c, &p.dailyText, "复盘", "记录今天的盘面与判断").Height(160)
			if p.dailyError != "" {
				typography.Caption(c, p.dailyError).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			p.dailyOpen = false
			p.dailyPick.Open = false
		}
		if ui.Button(c, "保存").Clicked() {
			p.saveDaily()
		}
	})
}

func (p *positionsPanel) dailyReadDialog(c ui.C) {
	if !p.dailyReadOpen {
		return
	}
	title := "每日复盘 · " + p.dailyRead.ReviewDate
	ui.ModalWith(c, &p.dailyReadOpen, title, ui.ModalSpec{Width: 560}, func() {
		ui.Scroll(c).MaxHeight(320).Children(func() {
			positionsTextBlock(c, p.dailyRead.Review)
		})
	}, func() {
		if ui.Button(c, "关闭").Outline().Clicked() {
			p.dailyReadOpen = false
		}
		if p.selectedPosition() != nil {
			if ui.Button(c, "编辑").Clicked() {
				review := p.dailyRead
				p.dailyReadOpen = false
				p.openDailyEdit(review)
			}
		}
	})
}

func (p *positionsPanel) roundEditDialog(c ui.C) {
	if !p.roundEditOpen {
		return
	}
	ui.ModalWith(c, &p.roundEditOpen, "本轮复盘", ui.ModalSpec{Width: 560}, func() {
		ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
			ui.TextArea(c, &p.roundDraft, "复盘",
				"买入理由 / 卖出理由 / 买点审视 / 卖点审视 / 交易心得").Height(180)
			if p.roundError != "" {
				typography.Caption(c, p.roundError).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			p.roundEditOpen = false
		}
		if ui.Button(c, "保存复盘").Clicked() {
			p.saveRound()
		}
	})
}

func (p *positionsPanel) roundReadDialog(c ui.C) {
	if !p.roundReadOpen {
		return
	}
	ui.ModalWith(c, &p.roundReadOpen, "本轮复盘", ui.ModalSpec{Width: 560}, func() {
		ui.Scroll(c).MaxHeight(320).Children(func() {
			positionsTextBlock(c, p.roundReadText)
		})
	}, func() {
		if ui.Button(c, "关闭").Outline().Clicked() {
			p.roundReadOpen = false
		}
		if ui.Button(c, "编辑").Clicked() {
			p.roundReadOpen = false
			p.openRoundEdit()
		}
	})
}

func (p *positionsPanel) tradeDialog(c ui.C) {
	if !p.tradeOpen {
		return
	}
	p.ensureTradeName(c)
	title := "委托" + domain.TradeTypeLabel(p.tradeType)
	position := p.positionByCode(p.tradeCode)
	fixedLots := p.tradeType == domain.TradeClose && position != nil && position.Quantity > 0
	isOpen := p.tradeType == domain.TradeOpen

	ui.ModalWith(c, &p.tradeOpen, title, ui.ModalSpec{Width: 640}, func() {
		ui.Scroll(c).MaxHeight(430).Children(func() {
			ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
				ui.Row(c).Gap(layout.BlockGap).Children(func() {
					positionsFormColumn(c, "股票名称", func() {
						if ui.TextField(c, &p.tradeName, "股票名称", "例如：浦发银行").
							Disabled(!isOpen).Grow(1).Changed() {
							p.tradeAutoName = ""
						}
					})
					positionsFormColumn(c, "股票代码", func() {
						if ui.TextField(c, &p.tradeCode, "股票代码", "例如：600000").
							Disabled(!isOpen).Grow(1).Changed() {
							if p.tradeAutoName != "" && p.tradeName == p.tradeAutoName {
								p.tradeName = ""
							}
							p.tradeAutoName = ""
							p.tradeNameKey, p.tradeNameDone = "", ""
						}
					})
				})

				ui.Column(c).Gap(layout.Gap).Children(func() {
					ui.Row(c).FillWidth().Items(ui.Center).Children(func() {
						typography.Section(c, "成交明细")
						ui.Spacer(c)
						if !fixedLots {
							if ui.Button(c, "添加一笔成交").Ghost().Small().Clicked() {
								p.tradeFills = append(p.tradeFills, positionsTradeFill{})
							}
						}
					})
					for index := range p.tradeFills {
						index := index
						fill := &p.tradeFills[index]
						ui.Row(c.KeyInt(index)).Gap(layout.Gap).Items(ui.Center).Children(func() {
							ui.TextField(c, &fill.price, "成交价（元）", "0.00").
								Mono().Grow(1)
							ui.TextField(c, &fill.lots, "手数", p.lotsPlaceholder()).
								Mono().Grow(1).Disabled(fixedLots)
							if !fixedLots && len(p.tradeFills) > 1 {
								if ui.Button(c, "移除这笔成交").Ghost().Small().IconOnly().
									Icon(icons.Trash2).Tooltip("移除这笔成交").Clicked() {
									i := index
									c.After(func() { p.removeTradeFill(i) })
								}
							}
						})
					}
				})

				p.tradeSummary(c)

				ui.Row(c).Gap(layout.BlockGap).Children(func() {
					positionsFormColumn(c, "委托时间", func() {
						datepicker.Picker(c, &p.tradePick, datepicker.Spec{
							Label: "委托时间", Start: p.tradeDate, Grain: datepicker.ByDay,
							Fill: true, CloseOnPick: true, Viewport: c.Bounds(),
							OnRange: func(start, _ string) { p.tradeDate = start },
						})
					})
					positionsFormColumn(c, "交易标签", func() {
						selectbox.Select(c, &p.tradeTag, p.tagChoicesWith(p.tradeTag), nil).
							Label("交易标签")
					})
				})

				ui.Column(c).Gap(layout.Gap).Children(func() {
					ui.Row(c).FillWidth().Items(ui.Center).Children(func() {
						typography.Section(c, "本轮费用设置")
						ui.Spacer(c)
						if isOpen {
							if ui.Button(c, "恢复系统配置").Ghost().Small().Clicked() {
								p.tradeFee = positionsFeeFormOf(p.fee)
							}
						}
					})
					if isOpen {
						ui.Row(c).Gap(layout.BlockGap).Children(func() {
							positionsFormColumn(c, "佣金费率（万分之）", func() {
								ui.TextField(c, &p.tradeFee.commission, "佣金费率（万分之）", "0").
									Mono().Grow(1)
							})
							positionsFormColumn(c, "最低佣金（元/委托）", func() {
								ui.TextField(c, &p.tradeFee.min, "最低佣金（元/委托）", "0.00").
									Mono().Grow(1)
							})
						})
						ui.Row(c).Gap(layout.BlockGap).Children(func() {
							positionsFormColumn(c, "印花税（%）", func() {
								ui.TextField(c, &p.tradeFee.stamp, "印花税（%）", "0").
									Mono().Grow(1)
							})
							positionsFormColumn(c, "过户费（%）", func() {
								ui.TextField(c, &p.tradeFee.transfer, "过户费（%）", "0").
									Mono().Grow(1)
							})
						})
					} else {
						typography.Caption(c, positionsFeeText(p.roundFeeOf(p.tradeCode), p.fee)).
							Color(c.Theme().MutedForeground)
					}
				})

				if p.tradeError != "" {
					typography.Caption(c, p.tradeError).Color(c.Theme().Danger)
				}
			})
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			p.closeTrade()
		}
		if ui.Button(c, domain.TradeTypeLabel(p.tradeType)).Clicked() {
			p.submitTrade()
		}
	})
}

func (p *positionsPanel) lotsPlaceholder() string {
	switch p.tradeType {
	case domain.TradeReduce:
		return "手数（可用）"
	}
	return "手数"
}

func (p *positionsPanel) tradeSummary(c ui.C) {
	amounts, lots, total := p.tradeAmounts()
	buy := domain.IsBuy(p.tradeType)
	instrument := domain.TradeInstrumentOf(strings.TrimSpace(p.tradeCode))
	feeKnown := len(amounts) > 0
	var feeTotal int64
	if feeKnown {
		feeTotal = domain.ComputeOrderFee(amounts, instrument, p.tradeFee.setting(), buy).Total
	}
	feeText := "—"
	totalText := "—"
	if feeKnown {
		feeText = money(feeTotal)
		if buy {
			totalText = money(total + feeTotal)
		} else {
			totalText = money(total - feeTotal)
		}
	}
	ui.Column(c).Gap(4).Children(func() {
		positionsSummaryRow(c, "合计",
			fmt.Sprintf("%d 笔 · %d 手 · %s", len(amounts), lots, money(total)))
		positionsSummaryRow(c, "预估费用", feeText)
		label := "预计支出"
		if !buy {
			label = "预计到手"
		}
		ui.Row(c).FillWidth().Items(ui.Center).Children(func() {
			typography.BodyStrong(c, label)
			ui.Spacer(c)
			typography.BodyStrong(c, totalText).Mono()
		})
	})
}

func positionsSummaryRow(c ui.C, label, value string) {
	ui.Row(c).FillWidth().Items(ui.Center).Children(func() {
		typography.Caption(c, label)
		ui.Spacer(c)
		ui.Text(c, value).FontSize(typography.SizeBody).Mono()
	})
}

func (p *positionsPanel) tradeEditDialog(c ui.C) {
	if !p.editOpen {
		return
	}
	ui.ModalWith(c, &p.editOpen, "编辑成交", ui.ModalSpec{Width: 420}, func() {
		ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
			positionsFormField(c, "价格（元）", func() {
				ui.TextField(c, &p.editPrice, "价格（元）", "0.00").Mono()
			})
			positionsFormField(c, "手数", func() {
				ui.TextField(c, &p.editLots, "手数", "1").Mono()
			})
			if p.editError != "" {
				typography.Caption(c, p.editError).Color(c.Theme().Danger)
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

func (p *positionsPanel) deleteDialog(c ui.C) {
	if !p.deleteOpen {
		return
	}
	ui.ModalWith(c, &p.deleteOpen, "删除整笔委托", ui.ModalSpec{Width: 420}, func() {
		ui.Column(c).Gap(layout.BlockGap).Children(func() {
			typography.Body(c, "删除后持仓与资金会按重放结果回滚，确定继续？")
			if p.deleteError != "" {
				typography.Caption(c, p.deleteError).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			p.deleteOpen = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
			p.confirmDeleteOrder()
		}
	})
}

// ------------------------------------------------------------------ 效果

func (p *positionsPanel) ensureQuotes(c ui.C) {
	if p.quotes == nil {
		return
	}
	codeSet := make(map[string]bool)
	for _, position := range p.positions {
		if position.Quantity > 0 ||
			len(domain.CurrentRoundTrades(p.tradesByCode[position.StockCode])) > 0 {
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
		if p.quoteMap == nil {
			p.quoteMap = map[string]domain.StockQuote{}
		}
	}
}

func (p *positionsPanel) beginNameRefresh(code string) {
	p.refreshSerial++
	p.refreshCode = code
	p.refreshKey = code + "|" + strconv.Itoa(p.refreshSerial)
	p.refreshDone = ""
}

func (p *positionsPanel) ensureRefreshName(c ui.C) {
	if p.refreshKey == "" || p.refreshDone == p.refreshKey {
		return
	}
	ui.Effect(c, p.refreshEffect, positionsNameRequest{
		Key: p.refreshKey, Code: p.refreshCode, Mode: "refresh",
	})
	result, _, ok := p.refreshEffect.Take()
	if !ok {
		return
	}
	p.refreshDone = p.refreshKey
	if result.Mode != "refresh" {
		return
	}
	position := p.positionByCode(result.Code)
	if result.Failed || result.Name == "" {
		p.notify("刷新名称失败", ui.ToastError)
		return
	}
	if position == nil {
		return
	}
	old := position.StockName
	if result.Name == old {
		p.notify("名称未变", ui.ToastDefault)
		return
	}
	if p.repo != nil {
		if err := p.repo.RefreshStockName(context.Background(), p.active(), result.Code, result.Name); err != nil {
			p.notify("刷新名称失败："+err.Error(), ui.ToastError)
			return
		}
	}
	p.renameLocal(result.Code, result.Name)
	p.notify("名称已更新："+result.Name, ui.ToastSuccess)
}

func (p *positionsPanel) ensureTradeName(c ui.C) {
	if !p.tradeOpen || p.tradeType != domain.TradeOpen || p.tradeName != "" {
		return
	}
	code := strings.TrimSpace(p.tradeCode)
	if !domain.IsValidStockCode(code) {
		return
	}
	if p.tradeNameKey != code {
		p.tradeNameKey, p.tradeNameDone = code, ""
	}
	if p.tradeNameDone == p.tradeNameKey {
		return
	}
	ui.Effect(c, p.tradeNameEffect, positionsNameRequest{
		Key: "trade|" + code, Code: code, Mode: "trade",
	})
	result, _, ok := p.tradeNameEffect.Take()
	if !ok {
		return
	}
	p.tradeNameDone = p.tradeNameKey
	if result.Failed || result.Name == "" {
		return
	}
	if p.tradeOpen && p.tradeType == domain.TradeOpen &&
		strings.TrimSpace(p.tradeCode) == result.Code && p.tradeName == "" {
		p.tradeName = result.Name
		p.tradeAutoName = result.Name
	}
}

// ------------------------------------------------------------------ 写入

func (p *positionsPanel) notify(message string, kind ui.ToastKind) {
	if p.toaster != nil {
		p.toaster.Show(message, "", kind)
	}
}

// afterWrite reloads the screen and tells the section that another screen's
// data changed too.
func (p *positionsPanel) afterWrite() {
	p.Reload()
	if p.onChange != nil {
		p.onChange()
	}
}

func (p *positionsPanel) saveTag(code, tag string) {
	if p.repo == nil {
		return
	}
	if err := p.repo.UpdateStockPositionTag(context.Background(), p.active(), code,
		strings.TrimSpace(tag)); err != nil {
		p.notify(positionsErrorText(err), ui.ToastError)
		return
	}
	p.afterWrite()
}

func (p *positionsPanel) openLedger() {
	position := p.selectedPosition()
	if position == nil {
		return
	}
	sheet := p.ledgerSheet(position)
	p.ledgerEdit.begin(sheet, position.StockCode, "",
		func(saved domain.StockLedgerEntry) {
			p.ledger = &saved
			p.notify("台账已保存", ui.ToastSuccess)
			p.afterWrite()
		})
}

func (p *positionsPanel) openDailyToday(position *domain.StockPosition) {
	today := domain.DiaryToday()
	p.dailyOpen = true
	p.dailyEditDate = ""
	p.dailyDate = today
	p.dailyText = p.templates.DailyTemplate
	for _, review := range p.currentDayReviews(position.StockCode) {
		if review.ReviewDate == today {
			p.dailyText = review.Review
			break
		}
	}
	p.dailyError = ""
	p.dailyPick.Open = false
}

func (p *positionsPanel) openDailyEdit(review domain.StockDayReview) {
	p.dailyOpen = true
	p.dailyEditDate = review.ReviewDate
	p.dailyDate = review.ReviewDate
	p.dailyText = review.Review
	p.dailyError = ""
	p.dailyPick.Open = false
}

func (p *positionsPanel) openDailyRead(review domain.StockDayReview) {
	p.dailyRead = review
	p.dailyReadOpen = true
}

func (p *positionsPanel) saveDaily() {
	position := p.selectedPosition()
	if position == nil || p.repo == nil {
		return
	}
	date := p.dailyDate
	if p.dailyEditDate != "" {
		date = p.dailyEditDate
	}
	if _, ok := domain.ParseDiaryDate(date); !ok {
		p.dailyError = "日期要写成 YYYY-MM-DD"
		return
	}
	if date > domain.DiaryToday() {
		p.dailyError = "每日复盘只能写到当天"
		return
	}
	opened := p.roundOpenedDate(position.StockCode)
	if opened != "" && date < opened {
		p.dailyError = "每日复盘不能早于本轮建仓日（" + opened + "）"
		return
	}
	editing := p.dailyEditDate != ""
	if _, err := p.repo.UpdateStockDayReview(context.Background(), p.active(),
		position.StockCode, date, p.dailyText); err != nil {
		p.dailyError = positionsErrorText(err)
		return
	}
	p.dailyOpen = false
	p.dailyPick.Open = false
	if editing {
		p.notify("复盘已更新", ui.ToastSuccess)
	} else {
		p.notify("复盘已保存", ui.ToastSuccess)
	}
	p.afterWrite()
}

func (p *positionsPanel) openRoundEdit() {
	position := p.selectedPosition()
	if position == nil {
		return
	}
	p.roundEditOpen = true
	p.roundReadOpen = false
	p.roundError = ""
	if strings.TrimSpace(position.Review) != "" {
		p.roundDraft = position.Review
	} else {
		p.roundDraft = p.templates.Template
	}
}

func (p *positionsPanel) openRoundRead() {
	position := p.selectedPosition()
	if position == nil {
		return
	}
	p.roundReadText = position.Review
	p.roundReadOpen = true
	p.roundEditOpen = false
}

func (p *positionsPanel) saveRound() {
	position := p.selectedPosition()
	if position == nil || p.repo == nil {
		return
	}
	if err := p.repo.UpdateStockPositionReview(context.Background(), p.active(),
		position.StockCode, p.roundDraft); err != nil {
		p.roundError = positionsErrorText(err)
		return
	}
	p.roundEditOpen = false
	p.notify("复盘已保存", ui.ToastSuccess)
	p.afterWrite()
}

func (p *positionsPanel) openTrade(kind string, position *domain.StockPosition) {
	p.tradeType = kind
	p.tradeError = ""
	p.tradeAutoName = ""
	p.tradeNameKey, p.tradeNameDone = "", ""
	p.tradePick.Open = false
	p.tradeCode, p.tradeName = "", ""
	positionLots := ""
	if position != nil {
		p.tradeCode, p.tradeName = position.StockCode, position.StockName
	}
	if kind == domain.TradeClose && position != nil && position.Quantity > 0 {
		positionLots = strconv.Itoa(position.Quantity / 100)
	}
	p.tradeFills = []positionsTradeFill{{lots: positionLots}}
	p.tradeDate = domain.DiaryToday()
	p.tradeTag = domain.DefaultStockTradeTag
	if position != nil {
		p.tradeTag = domain.EffectivePositionTag(position.Tag)
	} else if len(p.tags) > 0 {
		p.tradeTag = p.tags[0]
	}
	p.tradeFee = p.feeFormForTrade(kind, position)
	p.tradeOpen = true
}

func (p *positionsPanel) closeTrade() {
	p.tradeOpen = false
	p.tradePick.Open = false
	p.tradeError = ""
}

func (p *positionsPanel) removeTradeFill(index int) {
	if index < 0 || index >= len(p.tradeFills) || len(p.tradeFills) <= 1 {
		return
	}
	p.tradeFills = append(p.tradeFills[:index], p.tradeFills[index+1:]...)
}

func (p *positionsPanel) submitTrade() {
	if p.repo == nil {
		p.tradeError = "还没有打开工作空间"
		return
	}
	fills := make([]repository.StockFill, 0, len(p.tradeFills))
	for _, fill := range p.tradeFills {
		fills = append(fills, repository.StockFill{
			PriceMilli: positionsPriceMilli(fill.price),
			Lots:       positionsLots(fill.lots),
		})
	}
	tag := strings.TrimSpace(p.tradeTag)
	if tag == "" {
		tag = domain.DefaultStockTradeTag
	}
	request := repository.StockOrderRequest{
		AccountID: p.active(),
		StockCode: strings.TrimSpace(p.tradeCode),
		StockName: strings.TrimSpace(p.tradeName),
		TradeType: p.tradeType,
		TradeTime: positionsTradeTime(p.tradeDate),
		Tag:       tag,
		Fills:     fills,
	}
	if p.tradeType == domain.TradeOpen {
		round := domain.RoundFeeOfSetting(p.tradeFee.setting())
		request.RoundFee = &round
	}
	if _, err := p.repo.CreateStockOrder(context.Background(), request); err != nil {
		p.tradeError = positionsErrorText(err)
		return
	}
	p.closeTrade()
	p.notify("成交已记录", ui.ToastSuccess)
	p.afterWrite()
}

func (p *positionsPanel) openTradeEdit(trade domain.StockTrade) {
	p.editTradeID = trade.ID
	p.editPrice = domain.MilliToYuan(trade.PriceMilli)
	p.editLots = strconv.Itoa(trade.Lots)
	p.editError = ""
	p.editOpen = true
}

func (p *positionsPanel) saveTradeEdit() {
	if p.repo == nil || p.editTradeID == "" {
		return
	}
	if err := p.repo.UpdateStockTrade(context.Background(), repository.StockTradeUpdate{
		AccountID:  p.active(),
		TradeID:    p.editTradeID,
		PriceMilli: positionsPriceMilli(p.editPrice),
		Lots:       positionsLots(p.editLots),
	}); err != nil {
		p.editError = positionsErrorText(err)
		return
	}
	p.editOpen = false
	p.notify("成交已更新", ui.ToastSuccess)
	p.afterWrite()
}

func (p *positionsPanel) openDeleteOrder(trade domain.StockTrade) {
	p.deleteOrderID = trade.OrderID
	p.deleteError = ""
	p.deleteOpen = true
}

func (p *positionsPanel) confirmDeleteOrder() {
	if p.repo == nil || p.deleteOrderID == "" {
		return
	}
	if err := p.repo.DeleteStockOrder(context.Background(), p.active(), p.deleteOrderID); err != nil {
		p.deleteError = positionsErrorText(err)
		return
	}
	p.deleteOpen = false
	p.deleteOrderID = ""
	p.afterWrite()
}

// ------------------------------------------------------------------ 读取辅助

func (p *positionsPanel) selectedPosition() *domain.StockPosition {
	return p.positionByCode(p.selectedCode)
}

func (p *positionsPanel) positionByCode(code string) *domain.StockPosition {
	for i := range p.positions {
		if p.positions[i].StockCode == code {
			return &p.positions[i]
		}
	}
	return nil
}

// renameLocal updates the displayed name of one stock without reloading the
// screen, so an open dialog and the scroll position survive the refresh.
func (p *positionsPanel) renameLocal(code, name string) {
	for i := range p.positions {
		if p.positions[i].StockCode == code {
			p.positions[i].StockName = name
		}
	}
	for i := range p.tradesByCode[code] {
		p.tradesByCode[code][i].StockName = name
	}
}

func (p *positionsPanel) currentTrades(code string) []domain.StockTrade {
	return domain.CurrentRoundTrades(p.tradesByCode[code])
}

func (p *positionsPanel) visibleTrades(code string) []domain.StockTrade {
	current := p.currentTrades(code)
	visible := make([]domain.StockTrade, 0, len(current))
	for i := len(current) - 1; i >= 0; i-- {
		if current[i].Shares > 0 {
			visible = append(visible, current[i])
		}
	}
	return visible
}

func (p *positionsPanel) currentDayReviews(code string) []domain.StockDayReview {
	reviews := make([]domain.StockDayReview, 0)
	for _, review := range p.dayReviews {
		if review.StockCode == code && review.RoundID == "" {
			reviews = append(reviews, review)
		}
	}
	return reviews
}

func (p *positionsPanel) roundOpenedDate(code string) string {
	current := p.currentTrades(code)
	if len(current) == 0 {
		return ""
	}
	return time.Unix(current[0].TradeTime, 0).Format(domain.DiaryDateLayout)
}

func (p *positionsPanel) roundFeeOf(code string) *domain.RoundFee {
	return domain.CurrentRoundFee(p.tradesByCode[code])
}

func (p *positionsPanel) tagChoices() []string {
	if len(p.tags) > 0 {
		return p.tags
	}
	return domain.DefaultStockTradeTags[:]
}

func (p *positionsPanel) tagChoicesWith(current string) []string {
	choices := append([]string{}, p.tagChoices()...)
	if current != "" && !positionsContains(choices, current) {
		choices = append(choices, current)
	}
	return choices
}

// ------------------------------------------------------------------ 费用辅助

func (p *positionsPanel) feeFormForTrade(kind string, position *domain.StockPosition) positionsFeeForm {
	if kind == domain.TradeOpen {
		return positionsFeeFormOf(p.fee)
	}
	setting := p.fee
	if position != nil {
		round := domain.CurrentRoundFee(p.tradesByCode[position.StockCode])
		if round != nil {
			setting = domain.SettingOfRoundFee(*round)
		}
	}
	return positionsFeeFormOf(setting)
}

func positionsFeeFormOf(setting domain.StockFeeSetting) positionsFeeForm {
	return positionsFeeForm{
		commission: positionsFormatRate(setting.CommissionRate*10000, 4),
		min:        domain.CentsToYuan(setting.MinCommission),
		stamp:      positionsFormatRate(setting.StampDutyRate*100, 4),
		transfer:   positionsFormatRate(setting.TransferFeeRate*100, 4),
	}
}

func (f positionsFeeForm) setting() domain.StockFeeSetting {
	commission, _ := strconv.ParseFloat(strings.TrimSpace(f.commission), 64)
	minimum, _ := strconv.ParseFloat(strings.TrimSpace(f.min), 64)
	stamp, _ := strconv.ParseFloat(strings.TrimSpace(f.stamp), 64)
	transfer, _ := strconv.ParseFloat(strings.TrimSpace(f.transfer), 64)
	return domain.StockFeeSetting{
		CommissionRate:  commission / 10000,
		MinCommission:   int64(domain.RoundHalfAwayFromZero(minimum * 100)),
		StampDutyRate:   stamp / 100,
		TransferFeeRate: transfer / 100,
	}
}

func positionsFeeText(round *domain.RoundFee, setting domain.StockFeeSetting) string {
	rates := domain.RoundFeeOfSetting(setting)
	if round != nil {
		rates = *round
	}
	return fmt.Sprintf("佣金 万%.3f · 最低 %s · 印花税 %.2f%% · 过户费 %.3f%%",
		rates.CommissionRate*10000, money(rates.MinCommission),
		rates.StampDutyRate*100, rates.TransferFeeRate*100)
}

func positionsFormatRate(value float64, decimals int) string {
	text := strconv.FormatFloat(value, 'f', decimals, 64)
	if strings.Contains(text, ".") {
		text = strings.TrimRight(text, "0")
		text = strings.TrimRight(text, ".")
	}
	if text == "" || text == "-" {
		return "0"
	}
	return text
}

func (p *positionsPanel) tradeAmounts() (amounts []int64, lots int, total int64) {
	for _, fill := range p.tradeFills {
		price := positionsPriceMilli(fill.price)
		count := positionsLots(fill.lots)
		if price <= 0 || count <= 0 {
			continue
		}
		amount := domain.AmountOf(price, domain.SharesOfLots(int64(count)))
		amounts = append(amounts, amount)
		lots += count
		total += amount
	}
	return amounts, lots, total
}

// ------------------------------------------------------------------ 数字辅助

// positionsQuoteChange is the day's move as two readings: the price change and,
// when it can be computed, the percentage beside it.
func positionsQuoteChange(quote domain.StockQuote) (change, percent string, diff int64) {
	if quote.LatestPrice <= 0 || quote.PrevClose <= 0 {
		return "—", "", 0
	}
	diff = quote.LatestPrice - quote.PrevClose
	change = domain.MilliToYuan(diff)
	if diff >= 0 {
		change = "+" + change
	}
	rate := domain.QuoteChangePercent(quote.LatestPrice, quote.PrevClose)
	if rate == nil {
		return change, "", diff
	}
	return change, positionsSignedPercent(rate), diff
}

func positionsSignedPercent(value *float64) string {
	if value == nil {
		return "—"
	}
	text := strconv.FormatFloat(*value, 'f', 2, 64) + "%"
	if *value >= 0 {
		return "+" + text
	}
	return text
}

func positionsLotsText(shares int) string {
	text := strconv.FormatFloat(float64(shares)/100, 'f', -1, 64)
	return text + " 手"
}

func positionsDateTime(at int64) string {
	return time.Unix(at, 0).Format("2006-01-02 15:04")
}

func positionsTradeTime(date string) int64 {
	day, err := time.ParseInLocation(domain.DiaryDateLayout, date, time.Local)
	if err != nil {
		return 0
	}
	return day.Add(12 * time.Hour).Unix()
}

func positionsPriceMilli(text string) int64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || value <= 0 {
		return 0
	}
	return domain.PriceYuanToMilli(value)
}

func positionsLots(text string) int {
	value, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func positionsLedgerNumbersOf(trades []domain.StockTrade) positionsLedgerNumbers {
	var buyAmount, buyCost, sellAmount, sellNet int64
	var buyShares, sellShares int
	for _, trade := range trades {
		switch {
		case domain.IsBuy(trade.TradeType):
			buyAmount += trade.AmountCents
			buyCost += trade.AmountCents + trade.FeeCents
			buyShares += trade.Shares
		case domain.IsSell(trade.TradeType):
			sellAmount += trade.AmountCents
			sellNet += trade.AmountCents - trade.FeeCents
			sellShares += trade.Shares
		}
	}
	numbers := positionsLedgerNumbers{hasBuy: buyShares > 0, hasSell: sellShares > 0}
	if numbers.hasBuy {
		numbers.buyPrice = domain.MilliToYuan(positionsAverageMilli(buyAmount, buyShares))
		numbers.buyCost = money(buyCost)
	}
	if numbers.hasSell {
		numbers.sellPrice = domain.MilliToYuan(positionsAverageMilli(sellAmount, sellShares))
		numbers.sellNet = money(sellNet)
		numbers.netPnlCents = sellNet - buyCost
		numbers.netPnl = money(numbers.netPnlCents)
	}
	return numbers
}

func positionsAverageMilli(amountCents int64, shares int) int64 {
	if shares <= 0 {
		return 0
	}
	return int64(domain.RoundHalfAwayFromZero(
		float64(amountCents) * 10 / float64(shares)))
}

func positionsTextBlock(c ui.C, text string) {
	ui.Column(c).Gap(6).Children(func() {
		for _, line := range strings.Split(text, "\n") {
			if strings.TrimSpace(line) == "" {
				ui.Spacer(c).Height(8)
				continue
			}
			ui.Wrap(c, line).FontSize(typography.SizeBody)
		}
	})
}

func positionsHasCode(items []domain.StockPosition, code string) bool {
	for _, item := range items {
		if item.StockCode == code {
			return true
		}
	}
	return false
}

func positionsContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// positionsErrorText is the message a panel shows for a failed call. The store
// writes it as a sentence already (see store.invalidf), so there is nothing to
// trim.
func positionsErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
