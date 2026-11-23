package stock

import (
	"context"
	"fmt"
	"html"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/datepicker"
	"transactions/internal/components/emptystate"
	"transactions/internal/components/layout"
	"transactions/internal/components/semantic"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/icons"
	"transactions/internal/stocklimit"
)

// The rung cards' minimums: a card narrower than rungMinWidth starts a new
// column, and a card shorter than rungMinHeight makes the grid scroll as one
// column instead of squeezing its header and its stocks into nothing.
const (
	rungMinWidth  float32 = 280
	rungMinHeight float32 = 140

	// ladderWindowChrome is what the shell and the page padding take off the
	// window before the rungs see their width: the 204px navigation rail, the
	// 7px card margin and the 32px page inset. Page layout decisions elsewhere
	// use the window width too (the records toolbar wraps below 1020px); a
	// column count needs the content width, so the fixed parts come off here.
	ladderWindowChrome float32 = 243
)

// ladderRequest names one load of the ladder: the date, whether the user forced
// a refresh, and a serial that makes a Reload restart even on the same date.
type ladderRequest struct {
	Date    string
	Refresh bool
	Serial  int
}

// ladderPanel is the 天梯 screen: market data, so it has no account selector.
type ladderPanel struct {
	limits *stocklimit.Service

	date  string
	force bool
	// serial makes Reload (the shell entering the section) refetch even when the
	// date did not change; the request key includes it.
	serial int

	ladder       domain.StockLimitLadder
	have         bool
	pending      bool
	request      ladderRequest
	errorMessage string
	actionError  string
	actionNote   string

	effect *ui.EffectState[domain.StockLimitLadder, ladderRequest]

	// openKey is the copy of the ladder handed to the browser-export effect; a
	// pointer is a comparable effect key whose content does not change.
	openKey    *domain.StockLimitLadder
	openEffect *ui.EffectState[string, *domain.StockLimitLadder]

	pick   datepicker.State
	bounds gfx.Rect
}

func newLadderPanel(limits *stocklimit.Service) *ladderPanel {
	p := &ladderPanel{
		limits: limits,
		date:   latestTradingDay(todayYMD()),
		pick:   datepicker.NewState(),
	}
	p.effect = ui.NewEffectKeyed(func(ctx context.Context, request ladderRequest) (domain.StockLimitLadder, error) {
		if p.limits == nil {
			return domain.StockLimitLadder{}, fmt.Errorf("涨停池数据源未配置")
		}
		return p.limits.Ladder(ctx, request.Date, request.Refresh)
	}).Sequential()
	p.openEffect = ui.NewEffectKeyed(func(_ context.Context, ladder *domain.StockLimitLadder) (string, error) {
		path, err := writeLadderPage(*ladder)
		if err != nil {
			return "", err
		}
		if err := app.OpenPathInBrowser(path); err != nil {
			return path, err
		}
		return path, nil
	}).Sequential()
	return p
}

// Reload refetches the shown day. The shell calls it when the stock section or
// this tab becomes visible.
func (p *ladderPanel) Reload() {
	today := todayYMD()
	if p.date == "" || p.date > today {
		p.date = latestTradingDay(today)
	}
	p.pick.Open = false
	p.force = false
	p.serial++
	p.request = ladderRequest{}
	p.ladder = domain.StockLimitLadder{}
	p.have = false
	p.pending = false
	p.errorMessage = ""
	p.actionError = ""
	p.actionNote = ""
}

// CloseOverlays closes the date picker's panel.
func (p *ladderPanel) CloseOverlays() { p.pick.Open = false }

// View declares the screen.
func (p *ladderPanel) View(c ui.C) {
	p.bounds = c.Bounds()
	p.ensure(c)
	p.ensureOpen(c)
	layout.Page(c, p.toolbar, func(c ui.C) {
		switch {
		case p.actionError != "":
			typography.Caption(c, p.actionError).Color(c.Theme().Danger).
				PadX(layout.Inset).PadBottom(4).Ellipsis()
		case p.actionNote != "":
			typography.Caption(c, p.actionNote).
				PadX(layout.Inset).PadBottom(4).Ellipsis()
		}
		p.body(c)
		p.footer(c)
	})
}

func (p *ladderPanel) toolbar(c ui.C) {
	layout.Toolbar(c, func() {
		if ui.Button(c, "前一天").Ghost().Icon(icons.ChevronLeft).IconOnly().Clicked() {
			p.shiftDate(-1)
		}
		datepicker.Picker(c, &p.pick, datepicker.Spec{
			Label: "天梯日期", Start: p.date, End: p.date,
			Grain: datepicker.ByDay, CloseOnPick: true,
			Width: 148, MaxDate: p.today(), Viewport: p.bounds,
			OnRange: func(start, _ string) { p.selectDate(start) },
		})
		next := ui.Button(c, "后一天").Ghost().Icon(icons.ChevronRight).IconOnly().
			Disabled(p.date >= p.today())
		if next.Clicked() {
			p.shiftDate(1)
		}
		ui.Spacer(c)
		typography.Caption(c, p.statusText()).MaxWidth(240).TextEnd().Ellipsis()

		open := ui.Button(c, "以网页打开").Outline().Icon(icons.ExternalLink).
			Disabled(p.pending || !p.have || p.ladder.Status != domain.StockLimitOK || p.ladder.Date != p.date)
		if open.Clicked() {
			p.openPage()
		}
		refresh := ui.Button(c, "刷新").Disabled(p.pending)
		if refresh.Clicked() {
			p.refresh()
		}
	})
}

// ensure declares the network work in an Effect, so painting never waits for it.
func (p *ladderPanel) ensure(c ui.C) {
	if p.limits == nil || !validDay(p.date) {
		return
	}
	request := ladderRequest{Date: p.date, Refresh: p.force, Serial: p.serial}
	if request != p.request {
		p.request = request
		p.pending = true
		p.errorMessage = ""
	}
	ui.Effect(c, p.effect, request)
	if result, err, ok := p.effect.Take(); ok {
		p.pending = false
		if err != nil {
			p.ladder = domain.StockLimitLadder{}
			p.have = false
			p.errorMessage = err.Error()
			return
		}
		p.ladder = result
		p.have = true
		p.errorMessage = ""
	}
}

func (p *ladderPanel) ensureOpen(c ui.C) {
	if p.openKey == nil {
		return
	}
	ui.Effect(c, p.openEffect, p.openKey)
	if path, err, ok := p.openEffect.Take(); ok {
		p.openKey = nil
		if err != nil {
			p.actionError = "以网页打开失败：" + err.Error()
			p.actionNote = ""
			return
		}
		p.actionError = ""
		p.actionNote = "已以网页打开：" + path
	}
}

func (p *ladderPanel) openPage() {
	if !p.have || p.ladder.Status != domain.StockLimitOK || p.ladder.Date != p.date {
		return
	}
	copied := p.ladder
	p.openKey = &copied
	p.actionError = ""
	p.actionNote = ""
}

func (p *ladderPanel) refresh() {
	p.force = true
	p.serial++
	p.errorMessage = ""
	p.actionError = ""
	p.actionNote = ""
}

func (p *ladderPanel) selectDate(date string) {
	if !validDay(date) || date > todayYMD() || date == p.date {
		return
	}
	p.date = date
	p.force = false
	p.serial++
	p.have = false
	p.ladder = domain.StockLimitLadder{}
	p.errorMessage = ""
	p.actionError = ""
	p.actionNote = ""
}

func (p *ladderPanel) shiftDate(delta int) {
	day, err := time.ParseInLocation("2006-01-02", p.date, time.Local)
	if err != nil {
		return
	}
	next := day.AddDate(0, 0, delta).Format("2006-01-02")
	if next > todayYMD() {
		return
	}
	p.selectDate(next)
}

func (p *ladderPanel) body(c ui.C) {
	switch {
	case p.limits == nil:
		p.noticeState(c, "涨停池数据源未配置")
		return
	case p.errorMessage != "":
		p.errorState(c)
		return
	case !p.have:
		text := "选择一个交易日查看连板天梯"
		if p.pending {
			text = "正在抓取涨停池…"
		}
		p.noticeState(c, text)
		return
	case p.ladder.Status != domain.StockLimitOK:
		title, description := ladderEmptyCopy(p.ladder)
		p.emptyState(c, title, description)
		return
	}

	ui.Column(c).Grow(1).FillWidth().MinHeight(0).Pad(layout.Inset).Gap(layout.BlockGap).Children(func() {
		p.boards(c)
		if p.ladder.PreviousDate == "" {
			typography.Caption(c, "缺少前一交易日的数据，断板与晋级率暂不计入。")
		}
		p.rungs(c)
	})
}

func (p *ladderPanel) boards(c ui.C) {
	boards := domain.BoardCounts(ladderEntries(p.ladder))
	if len(boards) == 0 {
		return
	}
	ui.Flow(c).Gap(layout.ColumnGap).LineGap(4).Items(ui.Center).Children(func() {
		typography.Caption(c, "板块")
		for _, board := range boards {
			board := board
			c.Key("board-" + board.Board)
			ui.Row(c).Gap(4).Items(ui.Baseline).Children(func() {
				typography.Caption(c, board.Board).Ellipsis().MaxWidth(120)
				typography.Caption(c, strconv.Itoa(board.Count)).Mono()
			})
		}
	})
}

func (p *ladderPanel) rungs(c ui.C) {
	rungs := p.ladder.Rungs
	if len(rungs) == 0 {
		return
	}
	cols := ladderColumns(c.Bounds().Width - ladderWindowChrome)
	// The rows share the height the page leaves: each card takes its row's
	// height and scrolls its own stocks. The grid only scrolls as a whole when
	// the window is too short to keep a row readable at rungMinHeight.
	ui.Scroll(c).Grow(1).FillWidth().MinHeight(0).Children(func() {
		ui.Column(c).FillWidth().Grow(1).Gap(layout.Gap).Children(func() {
			for start := 0; start < len(rungs); start += cols {
				ui.Row(c).FillWidth().Grow(1).Gap(layout.Gap).Children(func() {
					for index := start; index < start+cols; index++ {
						if index >= len(rungs) {
							ui.Spacer(c).Grow(1).Basis(0)
							continue
						}
						rung := rungs[index]
						c.Key("rung-" + strconv.Itoa(rung.Level))
						p.rung(c, rung)
					}
				})
			}
		})
	})
}

// ladderColumns is how many rung cards fit side by side: one per 280px of
// content with the shared gap between them, the rule the Grid used to apply.
func ladderColumns(width float32) int {
	if width <= 0 {
		return 1
	}
	return max(1, int((width+layout.Gap)/(rungMinWidth+layout.Gap)))
}

func (p *ladderPanel) rung(c ui.C, rung domain.StockLimitRung) {
	theme := c.Theme()
	isTop := p.ladder.Metrics.MaxLevel != nil && *p.ladder.Metrics.MaxLevel == rung.Level
	// The card fills its row; the level and its rate stay put while the stocks
	// below scroll. rungMinHeight is what keeps that header readable when the
	// window cannot give every row a full share.
	column := ui.Column(c).Grow(1).Basis(0).FillHeight().MinHeight(rungMinHeight).
		Background(theme.Card).Border(1, theme.Border).Radius(8).Pad(8).Gap(4)
	column.Children(func() {
		ui.Row(c).FillWidth().Items(ui.Baseline).Gap(4).Children(func() {
			// The level is the ladder's height: the top rung is the page's one
			// large figure, every other rung reads at the grid's figure size.
			if isTop {
				typography.Metric(c, strconv.Itoa(rung.Level))
			} else {
				typography.Figure(c, strconv.Itoa(rung.Level))
			}
			ui.Text(c, "板").FontSize(typography.SizeCaption).Muted()
			if len(rung.Entries) > 0 {
				ui.Text(c, strconv.Itoa(len(rung.Entries))).
					FontSize(typography.SizeCaption).Muted().Mono()
			}
		})
		rate := "首板"
		if rung.Level > 1 {
			rate = "—"
			if rung.Rate != nil {
				rate = "晋级 " + strconv.FormatFloat(*rung.Rate, 'f', 0, 64) + "%"
			}
		}
		typography.Caption(c, rate)
		ui.Divider(c)
		rows := domain.LadderRows(rung)
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
			for index := range rows {
				row := rows[index]
				if index > 0 {
					ui.Divider(c)
				}
				prefix := "limit-"
				if row.Broken != nil {
					prefix = "broken-"
				}
				c.Key(prefix + row.Entry.StockCode)
				p.row(c, row)
			}
		})
	})
}

func (p *ladderPanel) row(c ui.C, row domain.LadderRow) {
	theme := c.Theme()
	broken := row.Broken != nil
	column := ui.Column(c).FillWidth().PadSides(8, 8, 6, 6).Gap(2)
	if !broken {
		column = column.Background(semantic.Wash(semantic.Expense(*theme))).Radius(4)
	}
	column.Children(func() {
		ui.Row(c).FillWidth().Items(ui.Center).Gap(6).Children(func() {
			name := ui.Text(c, row.Entry.StockName).FontSize(typography.SizeBody).
				Ellipsis().Grow(1)
			if broken {
				name.Muted()
			}
			if price := ladderPrice(row); price != "" {
				priceText := ui.Text(c, price).FontSize(typography.SizeCaption).Mono()
				if broken {
					priceText.Muted()
				}
			}
			ui.Text(c, row.Entry.StockCode).FontSize(typography.SizeCaption).Mono().Muted()
			seal := domain.LadderSealTime(row.Entry.FirstSeal)
			if broken {
				seal = "昨" + seal
			}
			ui.Text(c, seal).FontSize(typography.SizeCaption).Mono().Muted()
		})
		p.rowTags(c, row)
	})
}

func (p *ladderPanel) rowTags(c ui.C, row domain.LadderRow) {
	theme := c.Theme()
	broken := row.Broken != nil
	board := domain.BoardOf(row.Entry)
	openPct := row.Entry.OpenPct
	var todayPct *float64
	if row.Broken != nil {
		todayPct = row.Broken.TodayPct
	}
	oneWord := !broken && domain.IsOneWordBoard(row.Entry)
	if board == "" && !oneWord && row.Entry.BrokenCount == 0 && !broken &&
		openPct == nil && todayPct == nil {
		return
	}
	ui.Flow(c).Gap(4).LineGap(3).Items(ui.Center).Children(func() {
		if board != "" {
			typography.Caption(c, board)
		}
		if oneWord {
			ladderTag(c, "一字", semantic.Expense(*theme), semantic.Tint(semantic.Expense(*theme)), false)
		}
		if !broken && row.Entry.BrokenCount > 0 {
			ladderTag(c, "炸 "+strconv.Itoa(row.Entry.BrokenCount),
				semantic.Outlier(*theme), semantic.Tint(semantic.Outlier(*theme)), false)
		}
		if broken {
			ladderTag(c, "断", theme.MutedForeground, theme.Muted, false)
		}
		if openPct != nil {
			pctTag(c, "开 "+ladderPctText(*openPct), *openPct)
		}
		if todayPct != nil {
			pctTag(c, "收 "+ladderPctText(*todayPct), *todayPct)
		}
	})
}

func ladderTag(c ui.C, text string, foreground, background gfx.Color, mono bool) {
	tag := ui.Text(c, text).FontSize(typography.SizeCaption).Color(foreground).
		Background(background).PadSides(1, 5, 1, 5).Radius(3)
	if mono {
		tag.Mono()
	}
}

func pctTag(c ui.C, text string, value float64) {
	theme := c.Theme()
	color := theme.MutedForeground
	switch {
	case value > 0:
		color = semantic.Expense(*theme)
	case value < 0:
		color = semantic.Income(*theme)
	}
	ladderTag(c, text, color, semantic.Tint(color), true)
}

func (p *ladderPanel) footer(c ui.C) {
	limit, previous := "—", ""
	maxLevel, topName := "—", ""
	rate, promoted := "—", ""
	broken := "—"
	if p.have && p.ladder.Status == domain.StockLimitOK {
		metrics := p.ladder.Metrics
		limit = strconv.Itoa(metrics.LimitCount)
		if metrics.PreviousCount != nil {
			previous = "昨 " + strconv.Itoa(*metrics.PreviousCount)
		} else {
			previous = "昨 —"
		}
		if metrics.MaxLevel != nil {
			maxLevel = strconv.Itoa(*metrics.MaxLevel) + " 板"
			for _, rung := range p.ladder.Rungs {
				if rung.Level == *metrics.MaxLevel && len(rung.Entries) > 0 {
					topName = rung.Entries[0].StockName
					break
				}
			}
		}
		if metrics.Rate != nil {
			rate = strconv.FormatFloat(*metrics.Rate, 'f', 0, 64) + "%"
		}
		if metrics.PromotedCount != nil && metrics.PreviousCount != nil {
			promoted = strconv.Itoa(*metrics.PromotedCount) + "/" + strconv.Itoa(*metrics.PreviousCount)
		} else {
			promoted = "—"
		}
		if metrics.BrokenCount != nil {
			broken = strconv.Itoa(*metrics.BrokenCount)
		}
	}
	theme := c.Theme()
	layout.Footer(c, true, func() {
		layout.Reading(c, "涨停", limit, previous, theme.Foreground)
		layout.Reading(c, "空间板", maxLevel, topName, theme.Foreground)
		layout.Reading(c, "晋级率", rate, promoted, theme.Foreground)
		layout.Reading(c, "断板", broken, "", theme.Foreground)
	})
}

func (p *ladderPanel) statusText() string {
	if p.limits == nil {
		return "涨停池数据源未配置"
	}
	if p.errorMessage != "" {
		return "读取涨停池失败"
	}
	if p.pending {
		if p.have {
			return "刷新中…"
		}
		return "正在抓取…"
	}
	if !p.have {
		return "选择交易日查看"
	}
	ladder := p.ladder
	switch {
	case ladder.Status == domain.StockLimitOffday:
		return "周末休市 · 没有当天的涨停数据"
	case ladder.Status == domain.StockLimitPreopen:
		return "开盘前 · 09:30 后才有当天数据"
	case ladder.Temporary:
		return "盘中临时数据 · 15:00 收盘后写入本地"
	case ladder.Stale:
		return "刷新失败，显示 " + fetchedText(ladder.FetchedAt) + " 的本地缓存"
	case ladder.FromCache:
		return fetchedText(ladder.FetchedAt) + " 抓取 · 本地缓存"
	case ladder.Status == domain.StockLimitUnavailable:
		return "数据源未收录这一天，本地也没有缓存"
	}
	return fetchedText(ladder.FetchedAt) + " 抓取 · 已写入本地"
}

func (p *ladderPanel) errorState(c ui.C) {
	ui.Column(c).Grow(1).FillWidth().Pad(layout.Inset).Center().Gap(layout.Gap).Children(func() {
		typography.BodyParagraph(c, p.errorMessage, 3).Color(c.Theme().Danger).TextCenter()
		if ui.Button(c, "重试").Outline().Clicked() {
			p.refresh()
		}
	})
}

func (p *ladderPanel) noticeState(c ui.C, text string) {
	emptystate.Notice(c, text)
}

func (p *ladderPanel) emptyState(c ui.C, title, description string) {
	emptystate.Page(c, &icons.ChartCandlestick, title, description, nil)
}

func (p *ladderPanel) today() string { return todayYMD() }

func todayYMD() string { return time.Now().Format("2006-01-02") }

func validDay(date string) bool {
	_, err := time.ParseInLocation("2006-01-02", date, time.Local)
	return err == nil
}

// latestTradingDay moves a weekend date back to Friday; a weekday is unchanged.
func latestTradingDay(date string) string {
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return date
	}
	for day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		day = day.AddDate(0, 0, -1)
	}
	return day.Format("2006-01-02")
}

func ladderEntries(ladder domain.StockLimitLadder) []domain.StockLimitEntry {
	var entries []domain.StockLimitEntry
	for _, rung := range ladder.Rungs {
		entries = append(entries, rung.Entries...)
	}
	return entries
}

func ladderPrice(row domain.LadderRow) string {
	milli := row.Entry.PriceMilli
	if row.Broken != nil {
		if row.Broken.TodayPrice == nil {
			return ""
		}
		milli = *row.Broken.TodayPrice
	}
	if milli <= 0 {
		return ""
	}
	return domain.MilliToYuan(milli)
}

func ladderPctText(value float64) string {
	text := strconv.FormatFloat(value, 'f', 2, 64) + "%"
	if value > 0 {
		return "+" + text
	}
	return text
}

func fetchedText(seconds int64) string {
	if seconds <= 0 {
		return "—"
	}
	return time.Unix(seconds, 0).Format("15:04")
}

func ladderEmptyCopy(ladder domain.StockLimitLadder) (string, string) {
	if ladder.Status == domain.StockLimitOffday {
		return "周末休市", "A 股周末不开盘，没有当天的涨停数据；左右箭头可以翻到最近的交易日。"
	}
	if ladder.Status == domain.StockLimitUnavailable {
		return "数据源没有收录这一天", "数据源只保留最近一个多月；更早的日期要当时已经存到本地。"
	}
	if ladder.Status == domain.StockLimitPreopen {
		return "还没开盘", "09:30 开盘后才有当天的涨停数据；现在可以先看前一交易日。"
	}
	if ladder.Temporary {
		return "今天还没有涨停数据", "刚开盘或盘中还没有涨停股，刷新看看。"
	}
	return "这一天没有涨停", "非交易日，或当天确实没有涨停股。"
}

// ---------------------------------------------------------------- HTML page

// writeLadderPage writes the self-contained page to a temp file and returns its
// path. The browser-open call itself runs in an Effect, away from the window
// thread.
func writeLadderPage(ladder domain.StockLimitLadder) (string, error) {
	file, err := os.CreateTemp("", "transactions-ladder-*.html")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if _, err := file.WriteString(renderLadderPage(ladder)); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func renderLadderPage(ladder domain.StockLimitLadder) string {
	var out strings.Builder
	out.WriteString(`<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light dark">
<title>`)
	out.WriteString(html.EscapeString("连板天梯 · " + ladder.Date))
	out.WriteString(`</title>
<style>
:root{color-scheme:light dark;--paper:#fff;--card:#fafafa;--ink:#18181b;--muted:#646b76;--rule:#e4e4e7;--up:#b91c1c;--down:#0f6b34;--hot:#8a5a06;--accent:#1d4ed8}
@media (prefers-color-scheme:dark){:root{--paper:#18181b;--card:#27272a;--ink:#fafafa;--muted:#a1a1aa;--rule:rgba(250,250,250,.12);--up:#f87171;--down:#4ade80;--hot:#d9a04a;--accent:#2f5ce0}}
*{box-sizing:border-box}
html,body{margin:0;background:var(--paper);color:var(--ink)}
body{font:14px/1.6 -apple-system,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;padding:32px}
.sheet{max-width:1200px;margin:0 auto}
.masthead{display:flex;align-items:flex-start;justify-content:space-between;gap:20px;padding-bottom:16px;border-bottom:1px solid var(--rule)}
h1{margin:0;font-size:26px;line-height:1.2;letter-spacing:-.02em}
.date{margin:6px 0 0;color:var(--muted);font-variant-numeric:tabular-nums}
.print{padding:4px 10px;border:1px solid var(--rule);border-radius:8px;background:transparent;color:var(--muted);font:inherit;cursor:pointer}
.print:hover{color:var(--ink)}
.summary{display:flex;flex-wrap:wrap;gap:8px 24px;margin:16px 0 0;padding:0;list-style:none}
.stat{display:flex;align-items:baseline;gap:6px}
.stat-label{font-size:12px;color:var(--muted)}
.stat-value{font-family:ui-monospace,Consolas,monospace;font-size:16px;font-weight:600;font-variant-numeric:tabular-nums}
.stat-note{font-family:ui-monospace,Consolas,monospace;font-size:11px;color:var(--muted)}
.boards{display:flex;flex-wrap:wrap;align-items:center;gap:6px 14px;margin:12px 0 0;font-size:12px;color:var(--muted)}
.board-items{display:flex;flex-wrap:wrap;gap:6px 14px}
.board em{font-style:normal;color:var(--ink)}
.hint{margin:10px 0 0;font-size:12px;color:var(--muted)}
.ladder{margin-top:8px}
.level{display:grid;grid-template-columns:112px minmax(0,1fr);gap:20px;padding:20px 0;border-top:1px solid var(--rule)}
.level-head{display:flex;flex-direction:column;gap:2px}
.level-num{font-family:ui-monospace,Consolas,monospace;font-size:22px;font-weight:600;line-height:1.1}
.level-unit{font-size:12px;color:var(--muted)}
.level-meta{margin:0;font-family:ui-monospace,Consolas,monospace;font-size:11px;color:var(--muted)}
.cards{display:grid;grid-template-columns:repeat(auto-fill,minmax(190px,1fr));gap:8px;margin:0;padding:0;list-style:none}
.card{padding:10px;border:1px solid var(--rule);border-radius:10px;background:var(--card);min-width:0}
.card.limit{background:color-mix(in srgb,var(--up) 6%,var(--card))}
.card.broken{border-style:dashed}
.card-top{display:flex;gap:8px;align-items:baseline}
.name{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-weight:600}
.card.broken .name{color:var(--muted);font-weight:400}
.price{margin-left:auto;font-family:ui-monospace,Consolas,monospace;font-size:12px;font-variant-numeric:tabular-nums;white-space:nowrap}
.card.broken .price{color:var(--muted)}
.quote{display:flex;gap:8px;margin-top:2px;font-family:ui-monospace,Consolas,monospace;font-size:11px;color:var(--muted)}
.tags{display:flex;flex-wrap:wrap;align-items:center;gap:4px;margin-top:5px}
.board{font-size:11px;color:var(--muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.tag{padding:0 5px;border-radius:4px;font-size:11px;line-height:17px;white-space:nowrap}
.tag-word{color:var(--up);background:color-mix(in srgb,var(--up) 12%,transparent)}
.tag-hot{color:var(--hot);background:color-mix(in srgb,var(--hot) 12%,transparent)}
.tag-off{color:var(--muted);background:color-mix(in srgb,var(--muted) 12%,transparent)}
.tag-up{color:var(--up);background:color-mix(in srgb,var(--up) 12%,transparent);font-family:ui-monospace,Consolas,monospace}
.tag-down{color:var(--down);background:color-mix(in srgb,var(--down) 12%,transparent);font-family:ui-monospace,Consolas,monospace}
.tag-flat{color:var(--muted);background:color-mix(in srgb,var(--muted) 12%,transparent);font-family:ui-monospace,Consolas,monospace}
footer{margin-top:26px;padding-top:12px;border-top:1px solid var(--rule);font-size:11px;color:var(--muted);display:flex;justify-content:space-between;gap:12px}
.brand{font-family:ui-monospace,Consolas,monospace;letter-spacing:.08em}
@media (max-width:720px){body{padding:20px}.level{grid-template-columns:1fr;gap:8px}.level-head{flex-direction:row;flex-wrap:wrap;align-items:baseline;gap:2px 10px}.cards{grid-template-columns:repeat(auto-fill,minmax(160px,1fr))}footer{flex-direction:column}}
@media print{body{padding:0}.print{display:none}.card{break-inside:avoid}}
</style>
</head>
<body><main class="sheet">
`)
	out.WriteString(`<header class="masthead"><div><h1>连板天梯</h1><p class="date">`)
	out.WriteString(html.EscapeString(ladder.Date))
	out.WriteString(" · ")
	out.WriteString(html.EscapeString(weekdayZh(ladder.Date)))
	out.WriteString(" · ")
	out.WriteString(html.EscapeString(pageStatusText(ladder)))
	out.WriteString(`</p></div><button class="print" type="button" onclick="window.print()">打印 / 存为 PDF</button></header>`)
	out.WriteString(ladderSummaryHTML(ladder))
	out.WriteString(ladderBoardsHTML(ladder))
	if ladder.PreviousDate == "" {
		out.WriteString(`<p class="hint">缺少前一交易日数据，断板与晋级率未计入。</p>`)
	}
	out.WriteString(`<div class="ladder">`)
	rungs := append([]domain.StockLimitRung(nil), ladder.Rungs...)
	for i := 0; i < len(rungs); i++ {
		for j := i + 1; j < len(rungs); j++ {
			if rungs[j].Level > rungs[i].Level {
				rungs[i], rungs[j] = rungs[j], rungs[i]
			}
		}
	}
	for _, rung := range rungs {
		out.WriteString(ladderRungHTML(rung))
	}
	out.WriteString(`</div>`)
	out.WriteString(ladderFooterHTML(ladder))
	out.WriteString(`</main></body></html>`)
	return out.String()
}

func pageStatusText(ladder domain.StockLimitLadder) string {
	switch {
	case ladder.Temporary:
		return "盘中快照"
	case ladder.Stale:
		return "刷新失败 · 本地缓存"
	case ladder.FromCache:
		return "本地缓存"
	}
	return "收盘后定稿"
}

func ladderSummaryHTML(ladder domain.StockLimitLadder) string {
	metrics := ladder.Metrics
	rate := "—"
	if metrics.Rate != nil {
		rate = strconv.FormatFloat(*metrics.Rate, 'f', 0, 64) + "%"
	}
	previous := "昨 —"
	if metrics.PreviousCount != nil {
		previous = "昨 " + strconv.Itoa(*metrics.PreviousCount)
	}
	top := "—"
	topNote := ""
	if metrics.MaxLevel != nil {
		top = strconv.Itoa(*metrics.MaxLevel) + " 板"
		for _, rung := range ladder.Rungs {
			if rung.Level == *metrics.MaxLevel && len(rung.Entries) > 0 {
				topNote = rung.Entries[0].StockName
				break
			}
		}
	}
	promoted := "—"
	if metrics.PromotedCount != nil && metrics.PreviousCount != nil {
		promoted = strconv.Itoa(*metrics.PromotedCount) + "/" + strconv.Itoa(*metrics.PreviousCount)
	}
	broken := "—"
	if metrics.BrokenCount != nil {
		broken = strconv.Itoa(*metrics.BrokenCount)
	}
	var out strings.Builder
	out.WriteString(`<ul class="summary">`)
	out.WriteString(summaryItem("涨停", strconv.Itoa(metrics.LimitCount), previous))
	out.WriteString(summaryItem("空间板", top, topNote))
	out.WriteString(summaryItem("晋级率", rate, promoted))
	out.WriteString(summaryItem("断板", broken, ""))
	out.WriteString(`</ul>`)
	return out.String()
}

func summaryItem(label, value, note string) string {
	var out strings.Builder
	out.WriteString(`<li class="stat"><span class="stat-label">`)
	out.WriteString(html.EscapeString(label))
	out.WriteString(`</span><span class="stat-value">`)
	out.WriteString(html.EscapeString(value))
	out.WriteString(`</span>`)
	if note != "" {
		out.WriteString(`<span class="stat-note">`)
		out.WriteString(html.EscapeString(note))
		out.WriteString(`</span>`)
	}
	out.WriteString(`</li>`)
	return out.String()
}

func ladderBoardsHTML(ladder domain.StockLimitLadder) string {
	boards := domain.BoardCounts(ladderEntries(ladder))
	if len(boards) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString(`<p class="boards"><span>板块</span><span class="board-items">`)
	for _, board := range boards {
		out.WriteString(`<span class="board"><em>`)
		out.WriteString(html.EscapeString(board.Board))
		out.WriteString(`</em> `)
		out.WriteString(strconv.Itoa(board.Count))
		out.WriteString(`</span>`)
	}
	out.WriteString(`</span></p>`)
	return out.String()
}

func ladderRungHTML(rung domain.StockLimitRung) string {
	var out strings.Builder
	out.WriteString(`<section class="level"><header class="level-head"><span class="level-num">`)
	out.WriteString(strconv.Itoa(rung.Level))
	out.WriteString(`</span><span class="level-unit">板</span><p class="level-meta">`)
	meta := make([]string, 0, 3)
	if len(rung.Entries) > 0 {
		meta = append(meta, strconv.Itoa(len(rung.Entries))+" 家")
	}
	if rung.Level > 1 && rung.Rate != nil {
		meta = append(meta, "晋级 "+strconv.FormatFloat(*rung.Rate, 'f', 0, 64)+"%")
	}
	if len(rung.Broken) > 0 {
		meta = append(meta, "断 "+strconv.Itoa(len(rung.Broken)))
	}
	out.WriteString(html.EscapeString(strings.Join(meta, " · ")))
	out.WriteString(`</p></header><ul class="cards">`)
	for _, row := range domain.LadderRows(rung) {
		out.WriteString(ladderCardHTML(row))
	}
	out.WriteString(`</ul></section>`)
	return out.String()
}

func ladderCardHTML(row domain.LadderRow) string {
	broken := row.Broken != nil
	class := "card limit"
	if broken {
		class = "card broken"
	}
	var out strings.Builder
	out.WriteString(`<li class="` + class + `"><div class="card-top"><span class="name">`)
	out.WriteString(html.EscapeString(row.Entry.StockName))
	out.WriteString(`</span>`)
	if price := ladderPrice(row); price != "" {
		out.WriteString(`<span class="price">` + html.EscapeString(price) + `</span>`)
	}
	out.WriteString(`</div><div class="quote"><span>`)
	out.WriteString(html.EscapeString(row.Entry.StockCode))
	out.WriteString(`</span><span>`)
	seal := domain.LadderSealTime(row.Entry.FirstSeal)
	if broken {
		seal = "昨" + seal
	}
	out.WriteString(html.EscapeString(seal))
	out.WriteString(`</span></div><div class="tags">`)
	if board := domain.BoardOf(row.Entry); board != "" {
		out.WriteString(`<span class="board">` + html.EscapeString(board) + `</span>`)
	}
	if !broken && domain.IsOneWordBoard(row.Entry) {
		out.WriteString(`<span class="tag tag-word">一字</span>`)
	}
	if !broken && row.Entry.BrokenCount > 0 {
		out.WriteString(`<span class="tag tag-hot">炸 ` + strconv.Itoa(row.Entry.BrokenCount) + `</span>`)
	}
	if broken {
		out.WriteString(`<span class="tag tag-off">断</span>`)
	}
	if row.Entry.OpenPct != nil {
		out.WriteString(pctTagHTML("开", *row.Entry.OpenPct))
	}
	if broken && row.Broken.TodayPct != nil {
		out.WriteString(pctTagHTML("收", *row.Broken.TodayPct))
	}
	out.WriteString(`</div></li>`)
	return out.String()
}

func pctTagHTML(prefix string, value float64) string {
	class := "flat"
	switch {
	case value > 0:
		class = "up"
	case value < 0:
		class = "down"
	}
	return `<span class="tag tag-` + class + `">` + prefix + " " + ladderPctText(value) + `</span>`
}

func ladderFooterHTML(ladder domain.StockLimitLadder) string {
	parts := []string{"数据来源：东方财富涨停股池", fetchedText(ladder.FetchedAt) + " 抓取"}
	switch {
	case ladder.Temporary:
		parts = append(parts, "盘中快照，15:00 后定稿", "价格为抓取时的价格")
	case ladder.Stale:
		parts = append(parts, "刷新失败，显示本地缓存", "价格为当天收盘价")
	case ladder.FromCache:
		parts = append(parts, "本地缓存", "价格为当天收盘价")
	default:
		parts = append(parts, "收盘后定稿", "价格为当天收盘价")
	}
	var out strings.Builder
	out.WriteString(`<footer><p>`)
	out.WriteString(html.EscapeString(strings.Join(parts, " · ")))
	out.WriteString(`</p><p class="brand">TRANSACTIONS</p></footer>`)
	return out.String()
}

func weekdayZh(date string) string {
	day, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		return ""
	}
	return [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}[day.Weekday()]
}
