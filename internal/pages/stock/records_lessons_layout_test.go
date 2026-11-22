package stock

import (
	"context"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
	"transactions/internal/domain"
	"transactions/internal/repository"
)

// layoutRepo answers just the reads the two screens make. The embedded
// interface keeps the rest of repository.StockRepository out of the fixture.
type layoutRepo struct {
	repository.StockRepository
	histories []domain.StockTradeHistory
	trades    []domain.StockTrade
	rounds    []domain.StockTradeRound
	days      []domain.StockDayReview
	ledgers   []domain.StockLedgerEntry
	tags      []string
	lessons   []domain.StockLesson
	options   []domain.StockRoundOption
}

func (r layoutRepo) StockHistories(context.Context, string) ([]domain.StockTradeHistory, error) {
	return r.histories, nil
}
func (r layoutRepo) StockTrades(context.Context, string) ([]domain.StockTrade, error) {
	return r.trades, nil
}
func (r layoutRepo) StockRounds(context.Context, string) ([]domain.StockTradeRound, error) {
	return r.rounds, nil
}
func (r layoutRepo) StockDayReviews(context.Context, string) ([]domain.StockDayReview, error) {
	return r.days, nil
}
func (r layoutRepo) StockLedgerEntries(context.Context, string) ([]domain.StockLedgerEntry, error) {
	return r.ledgers, nil
}
func (r layoutRepo) StockTradeTags(context.Context, string) ([]string, error) {
	return r.tags, nil
}
func (r layoutRepo) StockReviewTemplates(context.Context, string) (domain.StockReviewSetting, error) {
	return domain.StockReviewSetting{Template: "买入理由：\n卖出理由："}, nil
}
func (r layoutRepo) StockLessons(context.Context, string) ([]domain.StockLesson, error) {
	return r.lessons, nil
}
func (r layoutRepo) StockRoundOptions(context.Context, string) ([]domain.StockRoundOption, error) {
	return r.options, nil
}

// The minimum window is 960 wide; the shell's navigation takes 204 and the
// workspace card's margin 7, so a page sees 749 logical pixels. The content
// height is 620 minus the 36 px title bar.
const (
	layoutPageWidth  = 749
	layoutPageHeight = 580
)

func recordLayoutFixture() (layoutRepo, []domain.StockAccount) {
	history := domain.StockTradeHistory{
		ID: "h1", StockCode: "600000", StockName: "浦发银行",
	}
	repo := layoutRepo{
		histories: []domain.StockTradeHistory{history},
		tags:      []string{"分析", "打板"},
	}
	for round := 1; round <= 6; round++ {
		id := "r" + string(rune('0'+round))
		repo.rounds = append(repo.rounds, domain.StockTradeRound{
			ID: id, StockCode: "600000", HistoryID: "h1", RoundNo: round,
			OpenedAt: 1_700_000_000 + int64(round)*1000,
			ClosedAt: 1_700_000_500 + int64(round)*1000,
			Tag:      "分析",
		})
		repo.trades = append(repo.trades,
			domain.StockTrade{
				ID: "o" + id, RoundID: id, StockCode: "600000", StockName: "浦发银行",
				TradeType: domain.TradeOpen, PriceMilli: 1000, Lots: 10, Shares: 1000,
				AmountCents: 100_000, FeeCents: 500, TradeTime: 1_700_000_000 + int64(round)*1000,
			},
			domain.StockTrade{
				ID: "c" + id, RoundID: id, StockCode: "600000", StockName: "浦发银行",
				TradeType: domain.TradeClose, PriceMilli: 1200, Lots: 10, Shares: 1000,
				AmountCents: 120_000, FeeCents: 600, TradeTime: 1_700_000_500 + int64(round)*1000,
			})
	}
	repo.days = []domain.StockDayReview{{
		ID: "d1", StockCode: "600000", RoundID: "r6", ReviewDate: "2026-03-02",
		Review: "放量突破前高，按计划持有。",
	}}
	repo.ledgers = []domain.StockLedgerEntry{{
		ID: "l1", StockCode: "600000", RoundID: "r6",
		WhyBuy: "突破平台", StopLoss: "4.60", TakeProfit: "6.50",
		FollowedSystem: "是", Summary: "按计划执行",
	}}
	return repo, []domain.StockAccount{{ID: "a1", Name: "默认股票账户"}}
}

// TestRecordsPanelFitsMinimumWindow paints 记录 at the smallest allowed content
// area: nothing may poke out of the page horizontally, and the largest round
// must start expanded.
func TestRecordsPanelFitsMinimumWindow(t *testing.T) {
	repo, accounts := recordLayoutFixture()
	store := newAccountStore()
	store.reset(accounts)
	panel := newRecordsPanel(store)
	panel.setRepository(repo)
	panel.openRound = func(string, string) {}

	theme := ui.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: panel.View, Theme: &theme})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{
		Width: layoutPageWidth, Height: layoutPageHeight,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	snapshot := driver.Snapshot()
	assertFitsWidth(t, snapshot)
	round, ok := snapshotHit(driver, "收起第 6 轮")
	if !ok {
		t.Fatal("the largest round is not expanded by default")
	}
	// The 240 px history rail must keep its width when the expanded round's
	// table raises the detail's minimum width; otherwise the stock cards
	// compress and their text is clipped.
	if round.Bounds.X < layout.RailWidth {
		t.Fatalf("the history rail shrank to %.0f px, want at least %.0f",
			round.Bounds.X, layout.RailWidth)
	}
	if !clickLabel(driver, "展开第 1 轮") {
		t.Fatal("no control to expand round 1")
	}
	var opened bool
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "收起第 1 轮" {
			opened = true
		}
	}
	if !opened {
		t.Fatal("clicking a round header did not expand it")
	}
}

// TestRecordsRoundSections checks that an expanded round is four named
// sections, each under the same section head, and that 每日复盘 no longer
// carries a count on its right.
func TestRecordsRoundSections(t *testing.T) {
	repo, accounts := recordLayoutFixture()
	store := newAccountStore()
	store.reset(accounts)
	panel := newRecordsPanel(store)
	panel.setRepository(repo)
	panel.openRound = func(string, string) {}

	theme := ui.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: panel.View, Theme: &theme})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 960, Height: 1100}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	names := []string{"交易台账", "每日复盘", "本轮复盘", "交易记录"}
	positions := map[string]float32{}
	for _, text := range driver.Snapshot().Texts {
		for _, name := range names {
			if text.Value == name {
				positions[name] = text.Bounds.Y
			}
		}
	}
	for _, name := range names {
		if _, ok := positions[name]; !ok {
			t.Fatalf("the expanded round has no %q section", name)
		}
	}
	for index := 1; index < len(names); index++ {
		if positions[names[index-1]] >= positions[names[index]] {
			t.Fatalf("%q is not above %q: %v", names[index-1], names[index], positions)
		}
	}
	if hasSnapshotText(driver, "1 条") {
		t.Fatal("每日复盘 still paints its count on the right")
	}
}

// clickLabel presses the middle of the first control with the given name.
func clickLabel(driver *app.Driver, label string) bool {
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == label && !hit.Disabled {
			bounds := hit.Bounds
			if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
				return false
			}
			return true
		}
	}
	return false
}

func TestLessonsPanelFitsMinimumWindow(t *testing.T) {
	repo, accounts := recordLayoutFixture()
	repo.lessons = []domain.StockLesson{
		{ID: "s1", Text: "追高买入，下一次等回踩。", RoundID: "r6",
			StockCode: "600000", StockName: "浦发银行", RoundNo: 6, SourceAvailable: true},
		{ID: "s2", Text: "没有计划就不下单。", RoundID: "gone",
			StockCode: "600000", StockName: "浦发银行", RoundNo: 3, SourceAvailable: false},
		{ID: "s3", Text: "复盘比预测有用。"},
	}
	store := newAccountStore()
	store.reset(accounts)
	panel := newLessonsPanel(store)
	panel.setRepository(repo)

	theme := ui.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: panel.View, Theme: &theme})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{
		Width: layoutPageWidth, Height: layoutPageHeight,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	snapshot := driver.Snapshot()
	assertFitsWidth(t, snapshot)
	var create, edit bool
	for _, hit := range snapshot.Hits {
		switch hit.Label {
		case "新建心得":
			create = true
		case "编辑心得":
			edit = true
		}
	}
	if !create || !edit {
		t.Fatalf("create=%v edit=%v, want both controls", create, edit)
	}
}

// assertFitsWidth fails when a laid-out box or a painted text crosses the
// page's left or right edge. Vertical scrolling is expected; horizontal
// overflow is not.
func assertFitsWidth(t *testing.T, snapshot app.Snapshot) {
	t.Helper()
	limit := float32(snapshot.Width) + 1
	for _, hit := range snapshot.Hits {
		if hit.Bounds.X < -1 || hit.Bounds.X+hit.Bounds.Width > limit {
			t.Errorf("control %q at %.1f..%.1f exceeds %.0f",
				hit.Label, hit.Bounds.X, hit.Bounds.X+hit.Bounds.Width, limit)
		}
	}
	for index, text := range snapshot.Texts {
		// Text bounds keep the allocated cell width but move X to the painted
		// start, so a centered or end-aligned cell must be measured by the
		// text's own painted width.
		width := text.Bounds.Width
		if index < len(snapshot.TextWidths) && snapshot.TextWidths[index] > 0 {
			width = snapshot.TextWidths[index]
		}
		if text.Bounds.X < -1 || text.Bounds.X+width > limit {
			t.Errorf("text %q at %.1f..%.1f exceeds %.0f",
				text.Value, text.Bounds.X, text.Bounds.X+width, limit)
		}
	}
}

// TestLessonsPanelComposerFromRound opens the dialog the way 记录 · 记心得 does:
// the stock and round arrive preselected and locked, and the dialog paints
// without a panic.
func TestLessonsPanelComposerFromRound(t *testing.T) {
	repo, accounts := recordLayoutFixture()
	repo.options = []domain.StockRoundOption{{
		RoundID: "r6", StockCode: "600000", StockName: "浦发银行", RoundNo: 6,
	}}
	store := newAccountStore()
	store.reset(accounts)
	panel := newLessonsPanel(store)
	panel.setRepository(repo)
	panel.OpenForRound("600000", "r6")

	theme := ui.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: panel.View, Theme: &theme})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{
		Width: layoutPageWidth, Height: layoutPageHeight,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	snapshot := driver.Snapshot()
	assertFitsWidth(t, snapshot)
	var save, stockLocked, roundLocked bool
	for _, hit := range snapshot.Hits {
		switch hit.Label {
		case "保存":
			save = true
		case "股票":
			stockLocked = hit.Disabled
		case "交易轮次":
			roundLocked = hit.Disabled
		}
	}
	if !save || !stockLocked || !roundLocked {
		t.Fatalf("save=%v stockLocked=%v roundLocked=%v", save, stockLocked, roundLocked)
	}
	if !clickLabel(driver, "保存") {
		t.Fatal("no 保存 button")
	}
	var emptyError bool
	for _, text := range driver.Snapshot().Texts {
		if text.Value == "写一句心得再保存" {
			emptyError = true
		}
	}
	if !emptyError {
		t.Fatal("saving an empty lesson did not show the empty-text error")
	}
}

// TestRecordsPanelFocusRound is the 心得 / 统计 jump: FocusRound selects the
// stock, folds every other round and leaves the target expanded.
func TestRecordsPanelFocusRound(t *testing.T) {
	repo, accounts := recordLayoutFixture()
	store := newAccountStore()
	store.reset(accounts)
	panel := newRecordsPanel(store)
	panel.setRepository(repo)
	panel.FocusRound("600000", "r3")

	theme := ui.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: panel.View, Theme: &theme})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{
		Width: layoutPageWidth, Height: layoutPageHeight,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	snapshot := driver.Snapshot()
	assertFitsWidth(t, snapshot)
	var arrived bool
	for _, hit := range snapshot.Hits {
		if hit.Label == "收起第 3 轮" {
			arrived = true
		}
	}
	if !arrived {
		t.Fatal("the jumped-to round is not expanded")
	}
}

// TestRecordsPanelReviewComposer paints the editable round review.
func TestRecordsPanelReviewComposer(t *testing.T) {
	repo, accounts := recordLayoutFixture()
	repo.rounds[0].Review = "买入理由：突破。"
	store := newAccountStore()
	store.reset(accounts)
	panel := newRecordsPanel(store)
	panel.setRepository(repo)
	panel.openReviewEditor(repo.rounds[0])

	theme := ui.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: panel.View, Theme: &theme})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{
		Width: layoutPageWidth, Height: layoutPageHeight,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	snapshot := driver.Snapshot()
	assertFitsWidth(t, snapshot)
	var save bool
	for _, hit := range snapshot.Hits {
		if hit.Label == "保存复盘" {
			save = true
		}
	}
	if !save {
		t.Fatal("the review composer has no 保存复盘 button")
	}
}
