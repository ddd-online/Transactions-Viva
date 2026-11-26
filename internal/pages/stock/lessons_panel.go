package stock

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

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
)

// lessonTextLimit is the project's STOCK_LESSON_TEXT_MAX.
const lessonTextLimit = 80

// Column widths of the lesson table. The 心得 cell takes the rest.
const (
	lessonSourceWidth = float32(240)
)

// lessonsPanel is 股票 · 心得: the account's one-sentence lessons, their linked
// closed round, and the new / edit dialog. The active account is read from the
// shared accountStore and never changed here.
type lessonsPanel struct {

	// pageScroll is the keyboard's handle on this tab's scroller: PageUp and
	// PageDown move it, so the rows below the fold are reachable without a pointer.
	pageScroll layout.PageScroll
	accounts   *accountStore
	repo       repository.StockRepository
	onChange   func()
	toaster    *ui.Toaster
	// onOpenRound is the 查看来源轮次 jump. The shell points it at the records
	// panel's FocusRound and the 记录 tab; nil disables the action.
	onOpenRound func(code, roundID string)

	lessons       []domain.StockLesson
	options       []domain.StockRoundOption
	optionsLoaded bool
	errorMessage  string
	// accountID is the account the loaded rows belong to; a reload after a
	// switch elsewhere in the section closes the composer and forgets its draft.
	accountID string

	formOpen      bool
	editingID     string
	text          string
	stockCode     string
	roundID       string
	sourceLocked  bool
	sourceMissing bool
	formError     string

	deleteOpen bool
	deleteID   string
	deleteText string
}

// lessonsPanel is one of the section's screens.
var _ panel = (*lessonsPanel)(nil)

// newLessonsPanel returns the 心得 screen over the section's shared accounts.
// stock.Model fills onChange, toaster and onOpenRound once the screen is wired.
func newLessonsPanel(accounts *accountStore) *lessonsPanel {
	return &lessonsPanel{accounts: accounts}
}

// setRepository connects the panel to the workspace and reads the active
// account's lessons and closed-round choices.
func (p *lessonsPanel) setRepository(repo repository.StockRepository) {
	p.repo = repo
	if repo == nil {
		p.clear()
		return
	}
	p.Reload()
}

func (p *lessonsPanel) active() string {
	if p.accounts == nil {
		return ""
	}
	return p.accounts.activeID()
}

// Reload rereads the lessons and the closed rounds the dialog offers.
func (p *lessonsPanel) Reload() {
	p.errorMessage = ""
	active := p.active()
	if p.accountID != active {
		p.accountID = active
		p.CloseOverlays()
		p.editingID, p.text, p.stockCode, p.roundID = "", "", "", ""
		p.sourceLocked, p.sourceMissing = false, false
		p.optionsLoaded = false
	}
	if p.repo == nil || active == "" {
		p.clear()
		return
	}
	ctx := context.Background()
	lessons, err := p.repo.StockLessons(ctx, active)
	if err != nil {
		p.lessons = nil
		p.errorMessage = "读取心得失败：" + err.Error()
		return
	}
	options, err := p.repo.StockRoundOptions(ctx, active)
	if err != nil {
		p.lessons = lessons
		p.options, p.optionsLoaded = nil, false
		p.errorMessage = "读取交易轮次失败：" + err.Error()
		return
	}
	p.lessons, p.options, p.optionsLoaded = lessons, options, true
}

func (p *lessonsPanel) clear() {
	p.lessons, p.options = nil, nil
	p.optionsLoaded = false
	p.errorMessage = ""
	p.CloseOverlays()
}

// ensureOptions loads the round choices when a form is opened before the page
// has been reloaded (a 记心得 jump can arrive that way).
func (p *lessonsPanel) ensureOptions() {
	if p.repo == nil || p.active() == "" || p.optionsLoaded {
		return
	}
	options, err := p.repo.StockRoundOptions(context.Background(), p.active())
	if err != nil {
		p.errorMessage = "读取交易轮次失败：" + err.Error()
		return
	}
	p.options, p.optionsLoaded = options, true
}

// CloseOverlays dismisses the composer and the confirmation.
func (p *lessonsPanel) CloseOverlays() {
	p.formOpen, p.deleteOpen = false, false
	p.formError = ""
}

// View declares the screen: toolbar, table and dialogs.
// Key gives this tab's scroller PageUp and PageDown.
func (p *lessonsPanel) Key(event gfx.KeyEvent) gfx.EventResult {
	return p.pageScroll.Key(event)
}

func (p *lessonsPanel) View(c ui.C) {
	if p.repo == nil || p.active() == "" {
		emptyAccountPage(c)
		return
	}
	layout.Page(c, p.toolbar, func(c ui.C) {
		if p.errorMessage != "" {
			layout.ErrorBanner(c, p.errorMessage)
		}
		if len(p.lessons) == 0 {
			if p.errorMessage == "" {
				emptystate.Page(c, &icons.Pencil, "暂无心得",
					"点右上角「新建心得」，记下这一轮的判断与教训。", nil)
			} else {
				ui.Spacer(c).Grow(1)
			}
			return
		}
		ui.Column(c).Grow(1).Children(func() {
			p.tableHeader(c)
			p.pageScroll.Area(ui.Scroll(c).Grow(1)).Children(func() {
				for index := range p.lessons {
					p.lessonRow(c, &p.lessons[index])
				}
			})
		})
	})
	p.formModal(c)
	p.deleteModal(c)
}

func (p *lessonsPanel) toolbar(c ui.C) {
	layout.Toolbar(c, func() {
		if p.accounts.picker(c) {
			p.CloseOverlays()
			p.editingID, p.text, p.stockCode, p.roundID = "", "", "", ""
			p.sourceLocked, p.sourceMissing = false, false
			p.optionsLoaded = false
			if p.onChange != nil {
				p.onChange()
			} else {
				p.Reload()
			}
		}
		ui.Spacer(c)
		if ui.Button(c, "新建心得").Icon(icons.Plus).Clicked() {
			p.openNew()
		}
	})
}

func (p *lessonsPanel) tableHeader(c ui.C) {
	layout.TableHeader(c, layout.TableHeaderSpec{PadX: layout.Inset, Gap: layout.ColumnGap}, func(c ui.C) {
		typography.Caption(c, "心得").Grow(1)
		typography.Caption(c, "关联交易").Width(lessonSourceWidth)
	})
}

func (p *lessonsPanel) lessonRow(c ui.C, lesson *domain.StockLesson) {
	theme := c.Theme()
	c.Key(lesson.ID)
	// The wash and the ring are how the row says it takes the pointer and the
	// keyboard: its commands are behind a right click, so the hover is the only
	// hint there is, and a focused row must show it has the keyboard.
	row := ui.Row(c).MinHeight(layout.RowHeight).FillWidth().PadX(layout.Inset).
		Gap(layout.ColumnGap).Items(ui.Center).
		Hit(ui.ID(c), lessonName(lesson)).
		Hover(ui.Look{}.Background(theme.Hover)).
		Focus(ui.Look{}.Ring(2, theme.Ring))
	row.Children(func() {
		ui.Text(c, lesson.Text).FontSize(typography.SizeBody).
			Basis(0).Grow(1).MinWidth(0).MaxLines(2)
		p.sourceCell(c, lesson)
	})
	// The row's commands are its context menu, as the 记录 and 待办 tables do: the
	// table keeps no 操作 column, and the words get its width.
	ui.ContextMenu(c, row, func(items *ui.MenuItems) {
		if items.Item("编辑心得").Clicked() {
			p.openEdit(lesson)
		}
		canJump := lesson.RoundID != "" && lesson.SourceAvailable && p.onOpenRound != nil
		jump := items.Item("查看来源轮次").Disabled(!canJump).
			Description(lessonJumpTip(lesson, p.onOpenRound != nil))
		if jump.Clicked() && canJump {
			p.onOpenRound(lesson.StockCode, lesson.RoundID)
		}
		items.Separator()
		if items.Item("删除心得").Destructive().Clicked() {
			p.askDelete(lesson)
		}
	})
}

// lessonName is the row's name for assistive technology and for the pointer that
// opens its commands: its words, cut where the eye stops reading the line.
func lessonName(lesson *domain.StockLesson) string {
	runes := []rune(strings.TrimSpace(lesson.Text))
	if len(runes) > 30 {
		return string(runes[:30]) + "…"
	}
	return string(runes)
}

func (p *lessonsPanel) sourceCell(c ui.C, lesson *domain.StockLesson) {
	theme := c.Theme()
	ui.Column(c).Width(lessonSourceWidth).MinWidth(lessonSourceWidth).Shrink(0).
		Gap(2).Children(func() {
		if lesson.RoundID == "" {
			typography.Body(c, "未关联交易轮次").Muted().Ellipsis()
			return
		}
		ui.Row(c).FillWidth().Gap(6).Items(ui.Center).Children(func() {
			typography.Body(c, historyStockTitle(lesson.StockName, lesson.StockCode)).
				MaxWidth(160).Ellipsis().Shrink(1).MinWidth(0)
			typography.Caption(c, "第 "+strconv.Itoa(lesson.RoundNo)+" 轮").Shrink(0)
		})
		if !lesson.SourceAvailable {
			typography.Caption(c, "来源轮次已删除").
				Color(semantic.Outlier(*theme))
		}
	})
}

func lessonJumpTip(lesson *domain.StockLesson, hasCallback bool) string {
	switch {
	case lesson.RoundID == "":
		return "未关联交易轮次"
	case !lesson.SourceAvailable:
		return "来源轮次已删除"
	case !hasCallback:
		return "暂时无法跳转"
	}
	return "查看来源轮次"
}

// openNew starts an unlinked lesson.
func (p *lessonsPanel) openNew() {
	p.ensureOptions()
	p.editingID, p.text = "", ""
	p.stockCode, p.roundID = "", ""
	p.sourceLocked, p.sourceMissing = false, false
	p.formError = ""
	p.formOpen = true
}

// openEdit starts an edit, restoring the linked round when it still exists.
func (p *lessonsPanel) openEdit(lesson *domain.StockLesson) {
	p.ensureOptions()
	p.editingID = lesson.ID
	p.text = lesson.Text
	p.roundID = lesson.RoundID
	p.stockCode = ""
	p.sourceLocked, p.sourceMissing = false, false
	p.formError = ""
	if lesson.RoundID != "" {
		found := false
		for _, option := range p.options {
			if option.RoundID == lesson.RoundID {
				p.stockCode = option.StockCode
				found = true
				break
			}
		}
		if !found {
			p.roundID = ""
			p.sourceMissing = true
		}
	}
	p.formOpen = true
}

// OpenForRound opens a new lesson with the stock and round preselected and
// locked. It is the records panel's 记心得 target (through stock.Model).
func (p *lessonsPanel) OpenForRound(code, roundID string) {
	p.ensureOptions()
	p.editingID, p.text = "", ""
	p.stockCode, p.roundID = code, roundID
	p.sourceLocked = roundID != ""
	p.sourceMissing = false
	p.formError = ""
	p.formOpen = true
}

func (p *lessonsPanel) saveForm() {
	if p.repo == nil || p.active() == "" {
		p.formError = "还没有打开工作空间"
		return
	}
	text := strings.TrimSpace(p.text)
	if text == "" {
		p.formError = "写一句心得再保存"
		return
	}
	if utf8.RuneCountInString(text) > lessonTextLimit {
		p.formError = fmt.Sprintf("心得最多 %d 个字", lessonTextLimit)
		return
	}
	roundID := p.roundID
	if p.sourceMissing {
		roundID = ""
	}
	ctx := context.Background()
	var err error
	if p.editingID == "" {
		_, err = p.repo.CreateStockLesson(ctx, p.active(), text, roundID)
	} else {
		_, err = p.repo.UpdateStockLesson(ctx, p.active(), p.editingID, text, roundID)
	}
	if err != nil {
		p.formError = err.Error()
		return
	}
	p.formOpen = false
	if p.editingID == "" {
		p.notify("心得已创建", ui.ToastSuccess)
	} else {
		p.notify("心得已保存", ui.ToastSuccess)
	}
	p.Reload()
	p.changed()
}

func (p *lessonsPanel) askDelete(lesson *domain.StockLesson) {
	p.deleteID, p.deleteText = lesson.ID, lesson.Text
	p.deleteOpen = true
}

func (p *lessonsPanel) confirmDelete() {
	if p.repo == nil {
		p.deleteOpen = false
		return
	}
	if err := p.repo.DeleteStockLesson(context.Background(), p.active(), p.deleteID); err != nil {
		p.deleteOpen = false
		p.notify("删除心得失败："+err.Error(), ui.ToastError)
		return
	}
	p.deleteOpen = false
	p.notify("心得已删除", ui.ToastSuccess)
	p.Reload()
	p.changed()
}

// stockChoices lists each stock once, keeping the store's newest-closed-first
// order. A locked jump keeps the requested code even before options load.
func (p *lessonsPanel) stockChoices() ([]string, map[string]string) {
	codes := make([]string, 0, len(p.options))
	labels := map[string]string{}
	for _, option := range p.options {
		if _, ok := labels[option.StockCode]; ok {
			continue
		}
		labels[option.StockCode] = historyStockTitle(option.StockName, option.StockCode)
		codes = append(codes, option.StockCode)
	}
	if p.stockCode != "" {
		if _, ok := labels[p.stockCode]; !ok {
			labels[p.stockCode] = p.stockCode
			codes = append([]string{p.stockCode}, codes...)
		}
	}
	return codes, labels
}

// roundChoices lists the selected stock's closed rounds, labelled 第 N 轮.
func (p *lessonsPanel) roundChoices() ([]string, map[string]string) {
	codes := make([]string, 0, len(p.options))
	labels := map[string]string{}
	for _, option := range p.options {
		if option.StockCode != p.stockCode {
			continue
		}
		codes = append(codes, option.RoundID)
		labels[option.RoundID] = "第 " + strconv.Itoa(option.RoundNo) + " 轮"
	}
	if p.roundID != "" {
		if _, ok := labels[p.roundID]; !ok {
			labels[p.roundID] = "当前轮次"
			codes = append(codes, p.roundID)
		}
	}
	return codes, labels
}

func (p *lessonsPanel) stockPlaceholder() string {
	if len(p.options) == 0 {
		return "暂无已完成的交易轮次"
	}
	return "先选择股票"
}

func (p *lessonsPanel) roundPlaceholder(rounds int) string {
	if p.stockCode == "" {
		return "先选择股票"
	}
	if rounds == 0 {
		return "该股票没有已完成的轮次"
	}
	return "选择轮次"
}

func (p *lessonsPanel) formModal(c ui.C) {
	if !p.formOpen {
		return
	}
	theme := c.Theme()
	title := "新建心得"
	if p.editingID != "" {
		title = "编辑心得"
	}
	stockCodes, stockLabels := p.stockChoices()
	ui.ModalWith(c, &p.formOpen, title, ui.ModalSpec{Width: 560}, func() {
		ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				typography.Caption(c, "心得")
				ui.TextField(c, &p.text, "心得",
					"例如：按计划止损，没有追高").AutoFocus()
				count := utf8.RuneCountInString(p.text)
				color := theme.MutedForeground
				if count > lessonTextLimit {
					color = theme.Danger
				}
				typography.Caption(c, fmt.Sprintf("%d/%d", count, lessonTextLimit)).Color(color)
			})
			ui.Column(c).Gap(4).Children(func() {
				typography.Caption(c, "股票")
				if selectbox.Select(c, &p.stockCode, stockCodes,
					func(code string) string { return stockLabels[code] }).
					Placeholder(p.stockPlaceholder()).Label("股票").
					Disabled(p.sourceLocked || len(stockCodes) == 0).Changed() {
					p.roundID = ""
					p.sourceMissing = false
				}
			})
			ui.Column(c).Gap(4).Children(func() {
				roundCodes, roundLabels := p.roundChoices()
				typography.Caption(c, "交易轮次")
				if selectbox.Select(c, &p.roundID, roundCodes,
					func(id string) string { return roundLabels[id] }).
					Placeholder(p.roundPlaceholder(len(roundCodes))).Label("交易轮次").
					Disabled(p.sourceLocked || p.stockCode == "" || len(roundCodes) == 0).Changed() {
					p.sourceMissing = false
				}
				if !p.sourceLocked && p.roundID != "" {
					if ui.Button(c, "解除关联").Ghost().Small().Clicked() {
						p.stockCode, p.roundID = "", ""
						p.sourceMissing = false
					}
				}
			})
			if p.sourceMissing {
				typography.Caption(c, "原关联的交易轮次已被删除，保存将解除关联。").
					Color(semantic.Outlier(*theme))
			}
			if p.formError != "" {
				typography.Caption(c, p.formError).Color(theme.Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			p.formOpen = false
		}
		action := "创建"
		if p.editingID != "" {
			action = "保存"
		}
		if ui.Button(c, action).Clicked() {
			p.saveForm()
		}
	})
}

func (p *lessonsPanel) deleteModal(c ui.C) {
	if !p.deleteOpen {
		return
	}
	ui.ModalWith(c, &p.deleteOpen, "删除这条心得？", ui.ModalSpec{
		Width:       380,
		Description: "关联的交易轮次不会被删除。",
	}, func() {
		typography.Caption(c, p.deleteText).MaxLines(2)
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			p.deleteOpen = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
			p.confirmDelete()
		}
	})
}

func (p *lessonsPanel) notify(text string, kind ui.ToastKind) {
	if p.toaster != nil {
		p.toaster.Show(text, "", kind)
	}
}

func (p *lessonsPanel) changed() {
	if p.onChange != nil {
		p.onChange()
	}
}
