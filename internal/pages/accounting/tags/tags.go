// Package tags is the 记账 · 标签 screen: the categories of a transaction type on
// the left and the tags of the chosen category on the right, with add, delete
// and drag reordering. It follows the application's category-tag page; the
// rows come from internal/store through repository.Repository.
package tags

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/emptystate"
	"transactions/internal/components/layout"
	"transactions/internal/components/typography"
	"transactions/internal/icons"
	"transactions/internal/pages/accounting/ledger"
	"transactions/internal/repository"
)

// kind is a transaction type: categories and tags belong to one of them.
type kind uint8

const (
	kindExpense kind = iota
	kindIncome
	kindTransfer
	kindCount
)

var kinds = [kindCount]kind{kindExpense, kindIncome, kindTransfer}

func (k kind) label() string {
	switch k {
	case kindIncome:
		return "收入"
	case kindTransfer:
		return "转账"
	}
	return "支出"
}

// value is the stored transaction type.
func (k kind) value() string {
	switch k {
	case kindIncome:
		return "income"
	case kindTransfer:
		return "transfer"
	}
	return "expense"
}

// color is the type's semantic hue, taken from the theme.
func (k kind) color(theme gfx.Theme) gfx.Color {
	switch k {
	case kindIncome:
		return theme.Success
	case kindTransfer:
		return theme.Primary
	}
	return theme.Danger
}

// category is one category of a type, in display order.
type category struct {
	id    int
	name  string
	count int
}

// tag is one tag of a category, in display order.
type tag struct {
	id    int
	name  string
	count int
}

const (
	categoryColumnWidth = layout.RailWidth
	listRowHeight       = layout.RowHeight
)

// Model is the 标签 screen's state: the chosen type, the categories and tags,
// the two lists (selection and drag reordering), and the dialogs.
type Model struct {
	kind       kind
	categories [kindCount][]category
	tags       [kindCount]map[string][]tag
	selected   string
	nextID     int

	// ledgers is the section's shared ledger list; ledgerID caches its active
	// choice so a switch made on another page can clear the chosen category.
	ledgers         *ledger.Store
	ledgerID        string
	repo            repository.Repository
	onLedgerChanged func()

	categoryList ui.ListState
	tagList      ui.ListState

	categoryOpen  bool
	categoryName  string
	categoryError string
	tagOpen       bool
	tagName       string
	tagError      string

	deleteOpen bool
	deleteTag  bool
	deleteName string

	toaster ui.Toaster
}

// New returns an empty 标签 page with its own ledger store. accounting calls
// SetDataSource once a workspace is open.
func New() *Model { return NewWithLedger(ledger.NewStore()) }

// NewWithLedger returns the page over the section's shared ledger store.
func NewWithLedger(ledgers *ledger.Store) *Model {
	if ledgers == nil {
		ledgers = ledger.NewStore()
	}
	m := &Model{kind: kindExpense, ledgers: ledgers, ledgerID: ledgers.Active()}
	for k := range m.tags {
		m.tags[k] = map[string][]tag{}
	}
	m.categoryList.Key = m.categoryKey
	m.categoryList.Label = m.categoryNameAt
	m.categoryList.Reorder = m.moveCategory
	m.tagList.Key = m.tagKey
	m.tagList.Label = m.tagNameAt
	m.tagList.Reorder = m.moveTag
	return m
}

// SetDataSource connects the page to the workspace; Reload fills both columns.
func (m *Model) SetDataSource(repo repository.Repository, onLedgerChanged func()) {
	m.repo = repo
	m.onLedgerChanged = onLedgerChanged
	if repo == nil {
		m.selected = ""
		m.nextID = 0
		for k := range m.tags {
			m.categories[k] = nil
			m.tags[k] = map[string][]tag{}
		}
		return
	}
	m.Reload()
}

// Reload reads all categories, tags and counts for the active ledger. The
// chosen category survives when it still exists; a ledger switch clears it.
func (m *Model) Reload() {
	if m.repo == nil {
		return
	}
	active := m.ledgers.Active()
	switched := active != m.ledgerID
	m.ledgerID = active
	if switched {
		m.selected = ""
		m.categoryList.Select(-1)
		m.tagList.Select(-1)
		m.tagList.ScrollTo(0, ui.ListStart)
	}
	m.nextID = 0
	for k := range m.tags {
		m.categories[k] = nil
		m.tags[k] = map[string][]tag{}
	}
	if active == "" {
		return
	}
	snapshot, err := m.repo.Labels(context.Background(), active)
	if err != nil {
		m.toaster.Show("读取分类标签失败："+err.Error(), "", ui.ToastError)
		return
	}
	for _, one := range snapshot.Categories {
		k, ok := kindOf(one.TransactionType)
		if !ok {
			continue
		}
		m.categories[k] = append(m.categories[k], category{
			id: m.takeID(), name: one.Name, count: snapshot.CategoryCounts[one.Name],
		})
	}
	for _, one := range snapshot.Tags {
		name, transactionType, ok := splitCategoryKey(one.CategoryTransactionType)
		if !ok {
			continue
		}
		k, ok := kindOf(transactionType)
		if !ok {
			continue
		}
		if m.tags[k][name] == nil {
			m.tags[k][name] = []tag{}
		}
		m.tags[k][name] = append(m.tags[k][name], tag{
			id: m.takeID(), name: one.Name, count: snapshot.TagCounts[one.Name],
		})
	}
	if m.selected != "" && !m.categoryExists(m.selected) {
		m.selected = ""
		m.categoryList.Select(-1)
		m.tagList.Select(-1)
	}
}

func (m *Model) takeID() int {
	m.nextID++
	return m.nextID
}

// categoryExists reports whether the current type still has the category.
func (m *Model) categoryExists(name string) bool {
	for _, one := range m.categories[m.kind] {
		if one.name == name {
			return true
		}
	}
	return false
}

func kindOf(value string) (kind, bool) {
	switch value {
	case "expense":
		return kindExpense, true
	case "income":
		return kindIncome, true
	case "transfer":
		return kindTransfer, true
	}
	return 0, false
}

func splitCategoryKey(value string) (name, transactionType string, ok bool) {
	index := strings.LastIndex(value, ":")
	if index <= 0 || index == len(value)-1 {
		return "", "", false
	}
	return value[:index], value[index+1:], true
}

// View declares the page: the type cards, the two columns and the dialogs.
func (m *Model) View(c ui.C) {
	layout.Page(c, m.toolbar, func(c ui.C) {
		ui.Row(c).Grow(1).FillWidth().Children(func() {
			m.categoryColumn(c)
			ui.Divider(c)
			m.tagColumn(c)
		})
	})
	m.categoryModal(c)
	m.tagModal(c)
	m.deleteModal(c)
	ui.Toasts(c, &m.toaster)
}

// CloseOverlays closes the dialogs when the shell leaves the screen.
func (m *Model) CloseOverlays() {
	m.categoryOpen, m.tagOpen, m.deleteOpen = false, false, false
	m.categoryError, m.tagError = "", ""
}

func (m *Model) toolbar(c ui.C) {
	layout.Toolbar(c, func() {
		if ledger.Picker(c, m.ledgers) && m.onLedgerChanged != nil {
			m.onLedgerChanged()
		}
		ui.Divider(c).Height(24)
		for _, k := range kinds {
			m.typeCard(c, k)
		}
	})
}

// typeCard is one transaction-type card: the chosen one takes its type's color
// on a light tint of the same hue.
func (m *Model) typeCard(c ui.C, k kind) {
	theme := c.Theme()
	color := k.color(*theme)
	active := m.kind == k
	foreground, border := theme.MutedForeground, theme.Border
	if active {
		foreground = color
		border = theme.Border.Mix(color, 0.35)
	}
	c.KeyInt(int(k))
	// A toolbar control, so it keeps the control height and padding of the
	// ledger picker beside it; the chosen type is a tint and its own border,
	// not a raised card (pages add no shadow to a surface at rest).
	card := ui.Row(c).Height(theme.Metrics.ControlHeight).
		PadX(theme.Metrics.PadControlX).Items(ui.Center).
		Radius(theme.Metrics.RadiusControl).Border(1, border).
		HitWith(ui.ID(c), k.label(), gfx.HitOptions{
			Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
		}).
		Focus(ui.Look{}.Ring(2, theme.Ring))
	if active {
		card = card.Background(theme.Background.Mix(color, 0.08))
	} else {
		card = card.Hover(ui.Look{}.Background(theme.Hover))
	}
	card.Children(func() {
		typography.Body(c, k.label()).Color(foreground)
	})
	if card.Clicked() {
		m.selectKind(k)
	}
}

// selectKind switches the transaction type and clears the chosen category, as
// the page does.
func (m *Model) selectKind(k kind) {
	if m.kind == k {
		return
	}
	m.kind = k
	m.selected = ""
	m.categoryList.Select(-1)
	m.categoryList.ScrollTo(0, ui.ListStart)
	m.tagList.Select(-1)
	m.tagList.ScrollTo(0, ui.ListStart)
}

// categoryColumn is the left column: the category list and its header.
func (m *Model) categoryColumn(c ui.C) {
	categories := m.categories[m.kind]
	layout.Rail(c, categoryColumnWidth, func(c ui.C) {
		add := m.columnHeader(c, "分类", len(categories), "新增分类", false)
		if add.Clicked() {
			m.openCategoryModal()
		}
		if len(categories) == 0 {
			emptystate.Notice(c, "暂无分类")
			return
		}
		ui.List(c, &m.categoryList, len(categories), m.categoryRow).Label("分类").Grow(1)
		if m.categoryList.Changed() {
			m.applyCategoryChoice()
		}
	})
}

// tagColumn is the right column: the tags of the chosen category. Its title is
// the category's name, or 标签 while nothing is chosen.
func (m *Model) tagColumn(c ui.C) {
	tags := m.currentTags()
	title := "标签"
	if m.selected != "" {
		title = m.selected
	}
	ui.Column(c).Grow(1).FillHeight().Children(func() {
		add := m.columnHeader(c, title, len(tags), "新增标签", m.selected == "")
		if add.Clicked() {
			m.openTagModal()
		}
		switch {
		case m.selected == "":
			emptystate.Notice(c, "选择分类查看标签")
		case len(tags) == 0:
			emptystate.Notice(c, "暂无标签")
		default:
			ui.List(c, &m.tagList, len(tags), m.tagRow).Label("标签").Grow(1)
		}
	})
}

// columnHeader is one column's title, its count badge and its add button. The
// caller reads Clicked from the returned button.
func (m *Model) columnHeader(c ui.C, title string, count int, addLabel string, disabled bool) ui.Box {
	var add ui.Box
	layout.SectionHeader(c, layout.Inset).Children(func() {
		typography.Section(c, title).Shrink(1).Ellipsis()
		layout.SectionCount(c, strconv.Itoa(count))
		ui.Spacer(c)
		add = ui.Button(c, addLabel).Small().Secondary().Icon(icons.Plus).Disabled(disabled)
	})
	return add
}

// countPill is a count in a muted capsule.
func countPill(c ui.C, count int) {
	theme := c.Theme()
	ui.Text(c, strconv.Itoa(count)).FontSize(typography.SizeCaption).
		Color(theme.MutedForeground).Background(theme.Muted).
		Radius(theme.Metrics.RadiusItem() + 2).PadX(6).PadY(1)
}

// categoryRow declares one category: drag handle, name, record count and the
// delete action.
func (m *Model) categoryRow(row ui.ListRow) {
	theme := row.C.Theme()
	categories := m.categories[m.kind]
	if row.Index >= len(categories) {
		return
	}
	cat := categories[row.Index]
	hover := theme.Hover
	if row.Selected() {
		hover = theme.Accent
	}
	row.Element.Height(listRowHeight).PadX(layout.BlockGap).Gap(layout.Gap).Items(ui.Center).
		Radius(theme.Metrics.RadiusControl).
		Hover(ui.Look{}.Background(hover)).
		Focus(ui.Look{}.Ring(2, theme.Ring))
	if row.Selected() {
		row.Element.Background(theme.Accent)
	}
	ui.Icon(row.C, icons.GripVertical).Size(16, 16).Color(theme.MutedForeground)
	typography.Body(row.C, cat.name).Grow(1).Ellipsis()
	if cat.count > 0 {
		countPill(row.C, cat.count)
	}
	remove := ui.Button(row.C, "删除分类 "+cat.name).
		Destructive().Icon(icons.Trash2).IconOnly().Tooltip("删除分类")
	if remove.Clicked() {
		m.askDeleteCategory(cat.name)
	}
}

// tagRow declares one tag of the chosen category.
func (m *Model) tagRow(row ui.ListRow) {
	theme := row.C.Theme()
	tags := m.currentTags()
	if row.Index >= len(tags) {
		return
	}
	one := tags[row.Index]
	row.Element.Height(listRowHeight).PadX(layout.BlockGap).Gap(layout.Gap).Items(ui.Center).
		Radius(theme.Metrics.RadiusControl).
		Hover(ui.Look{}.Background(theme.Hover)).
		Focus(ui.Look{}.Ring(2, theme.Ring))
	ui.Icon(row.C, icons.GripVertical).Size(16, 16).Color(theme.MutedForeground)
	typography.Body(row.C, one.name).Grow(1).Ellipsis()
	if one.count > 0 {
		countPill(row.C, one.count)
	}
	remove := ui.Button(row.C, "删除标签 "+one.name).
		Destructive().Icon(icons.Trash2).IconOnly().Tooltip("删除标签")
	if remove.Clicked() {
		m.askDeleteTag(one.name)
	}
}

// currentTags is the tag list of the chosen category.
func (m *Model) currentTags() []tag {
	if m.selected == "" {
		return nil
	}
	return m.tags[m.kind][m.selected]
}

// applyCategoryChoice copies the list's chosen row into the page's selection.
func (m *Model) applyCategoryChoice() {
	categories := m.categories[m.kind]
	index := m.categoryList.Selected()
	if index < 0 || index >= len(categories) {
		return
	}
	if categories[index].name == m.selected {
		return
	}
	m.selected = categories[index].name
	m.tagList.Select(-1)
	m.tagList.ScrollTo(0, ui.ListStart)
}

// categoryKey, categoryNameAt, tagKey and tagNameAt name the rows of the two
// lists: the ids keep a row chosen while the rows are dragged around it.
func (m *Model) categoryKey(index int) int {
	categories := m.categories[m.kind]
	if index < 0 || index >= len(categories) {
		return 0
	}
	return categories[index].id
}

func (m *Model) categoryNameAt(index int) string {
	categories := m.categories[m.kind]
	if index < 0 || index >= len(categories) {
		return ""
	}
	return categories[index].name
}

func (m *Model) tagKey(index int) int {
	tags := m.currentTags()
	if index < 0 || index >= len(tags) {
		return 0
	}
	return tags[index].id
}

func (m *Model) tagNameAt(index int) string {
	tags := m.currentTags()
	if index < 0 || index >= len(tags) {
		return ""
	}
	return tags[index].name
}

// moveCategory and moveTag apply a drag reordering and persist the new order.
func (m *Model) moveCategory(rows []int, to int) {
	if len(rows) == 0 {
		return
	}
	categories := m.categories[m.kind]
	m.categories[m.kind] = moveItem(categories, rows[0], to)
	if m.repo == nil {
		return
	}
	names := make([]string, len(m.categories[m.kind]))
	for index, cat := range m.categories[m.kind] {
		names[index] = cat.name
	}
	if err := m.repo.ReorderCategories(context.Background(), m.ledgerID, m.kind.value(), names); err != nil {
		m.toaster.Show("保存分类排序失败："+err.Error(), "", ui.ToastError)
		m.Reload()
	}
}

func (m *Model) moveTag(rows []int, to int) {
	if len(rows) == 0 || m.selected == "" {
		return
	}
	key := m.selected + ":" + m.kind.value()
	m.tags[m.kind][m.selected] = moveItem(m.currentTags(), rows[0], to)
	if m.repo == nil {
		return
	}
	tags := m.currentTags()
	names := make([]string, len(tags))
	for index, one := range tags {
		names[index] = one.name
	}
	if err := m.repo.ReorderTags(context.Background(), m.ledgerID, key, names); err != nil {
		m.toaster.Show("保存标签排序失败："+err.Error(), "", ui.ToastError)
		m.Reload()
	}
}

// moveItem takes the item at from out of the slice and puts it before to, where
// to is an index in the slice as it was before the move.
func moveItem[T any](items []T, from, to int) []T {
	if from < 0 || from >= len(items) || to < 0 || to > len(items) {
		return items
	}
	if to > from {
		to--
	}
	if to == from {
		return items
	}
	item := items[from]
	if from < to {
		copy(items[from:to], items[from+1:to+1])
	} else {
		copy(items[to+1:from+1], items[to:from])
	}
	items[to] = item
	return items
}

// openCategoryModal, openTagModal and the two close functions reset the
// dialogs' fields.
func (m *Model) openCategoryModal() {
	m.categoryOpen, m.categoryName, m.categoryError = true, "", ""
}

func (m *Model) closeCategoryModal() {
	m.categoryOpen, m.categoryName, m.categoryError = false, "", ""
}

func (m *Model) openTagModal() {
	if m.selected == "" {
		return
	}
	m.tagOpen, m.tagName, m.tagError = true, "", ""
}

func (m *Model) closeTagModal() {
	m.tagOpen, m.tagName, m.tagError = false, "", ""
}

// confirmCategory adds the typed category, rejecting a duplicate name.
func (m *Model) confirmCategory() {
	name := strings.TrimSpace(m.categoryName)
	if name == "" {
		return
	}
	for _, cat := range m.categories[m.kind] {
		if cat.name == name {
			m.categoryError = "该分类已存在"
			return
		}
	}
	if m.repo == nil {
		m.categoryError = "还没有打开工作空间"
		return
	}
	if err := m.repo.CreateCategory(context.Background(), m.ledgerID, name, m.kind.value()); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			m.categoryError = "该分类已存在"
		} else {
			m.categoryError = "添加分类失败：" + err.Error()
		}
		return
	}
	m.Reload()
	m.selected = name
	for index, one := range m.categories[m.kind] {
		if one.name != name {
			continue
		}
		m.categoryList.Select(index)
		m.categoryList.ScrollIntoView(index)
		break
	}
	m.tagList.Select(-1)
	m.tagList.ScrollTo(0, ui.ListStart)
	m.closeCategoryModal()
	m.toaster.Show("分类已创建", "", ui.ToastSuccess)
}

// confirmTag adds the typed tag under the chosen category.
func (m *Model) confirmTag() {
	name := strings.TrimSpace(m.tagName)
	if name == "" || m.selected == "" {
		return
	}
	for _, one := range m.currentTags() {
		if one.name == name {
			m.tagError = "该标签已存在"
			return
		}
	}
	if m.repo == nil {
		m.tagError = "还没有打开工作空间"
		return
	}
	key := m.selected + ":" + m.kind.value()
	if err := m.repo.CreateTag(context.Background(), m.ledgerID, name, key); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			m.tagError = "该标签已存在"
		} else {
			m.tagError = "添加标签失败：" + err.Error()
		}
		return
	}
	m.Reload()
	if len(m.currentTags()) > 0 {
		m.tagList.ScrollIntoView(len(m.currentTags()) - 1)
	}
	m.closeTagModal()
	m.toaster.Show("标签已创建", "", ui.ToastSuccess)
}

// askDeleteCategory and askDeleteTag open the delete confirmation.
func (m *Model) askDeleteCategory(name string) {
	m.deleteOpen, m.deleteTag, m.deleteName = true, false, name
}

func (m *Model) askDeleteTag(name string) {
	m.deleteOpen, m.deleteTag, m.deleteName = true, true, name
}

// confirmDelete removes the category (with its tags) or the tag.
func (m *Model) confirmDelete() {
	if m.repo == nil {
		m.deleteOpen = false
		m.toaster.Show("还没有打开工作空间", "", ui.ToastError)
		return
	}
	if m.deleteTag {
		key := m.selected + ":" + m.kind.value()
		if err := m.repo.DeleteTag(context.Background(), m.ledgerID, m.deleteName, key); err != nil {
			m.deleteOpen = false
			m.toaster.Show("删除标签失败："+err.Error(), "", ui.ToastError)
			return
		}
		m.tagList.Select(-1)
		m.Reload()
		m.toaster.Show("标签已删除", "", ui.ToastSuccess)
	} else {
		if err := m.repo.DeleteCategory(context.Background(), m.ledgerID, m.deleteName, m.kind.value()); err != nil {
			m.deleteOpen = false
			m.toaster.Show("删除分类失败："+err.Error(), "", ui.ToastError)
			return
		}
		if m.selected == m.deleteName {
			m.selected = ""
			m.categoryList.Select(-1)
			m.tagList.Select(-1)
			m.tagList.ScrollTo(0, ui.ListStart)
		}
		m.Reload()
		m.toaster.Show("分类已删除", "", ui.ToastSuccess)
	}
	m.deleteOpen = false
}

// categoryModal is the 新增分类 dialog.
func (m *Model) categoryModal(c ui.C) {
	if !m.categoryOpen {
		return
	}
	theme := c.Theme()
	ui.ModalWith(c, &m.categoryOpen, "新增分类", ui.ModalSpec{Width: 400}, func() {
		field := ui.TextField(c, &m.categoryName, "分类名称", "例如：餐饮美食").
			AutoFocus().FillWidth()
		if m.categoryError != "" {
			typography.Caption(c, m.categoryError).Color(theme.Danger).MarginTop(6)
		}
		if field.Submitted() {
			m.confirmCategory()
		}
	}, func() {
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, "取消").Secondary().Clicked() {
				m.closeCategoryModal()
			}
			confirm := ui.Button(c, "创建").
				Disabled(strings.TrimSpace(m.categoryName) == "")
			if confirm.Clicked() {
				m.confirmCategory()
			}
		})
	})
}

// tagModal is the 新增标签 dialog.
func (m *Model) tagModal(c ui.C) {
	if !m.tagOpen {
		return
	}
	theme := c.Theme()
	ui.ModalWith(c, &m.tagOpen, "新增标签", ui.ModalSpec{Width: 400}, func() {
		field := ui.TextField(c, &m.tagName, "标签名称", "例如：三餐").
			AutoFocus().FillWidth()
		if m.tagError != "" {
			typography.Caption(c, m.tagError).Color(theme.Danger).MarginTop(6)
		}
		if field.Submitted() {
			m.confirmTag()
		}
	}, func() {
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, "取消").Secondary().Clicked() {
				m.closeTagModal()
			}
			confirm := ui.Button(c, "创建").Disabled(strings.TrimSpace(m.tagName) == "")
			if confirm.Clicked() {
				m.confirmTag()
			}
		})
	})
}

// deleteModal is the confirmation shared by both deletions.
func (m *Model) deleteModal(c ui.C) {
	if !m.deleteOpen {
		return
	}
	title := "删除分类「" + m.deleteName + "」？"
	// A category takes its tags with it; a tag only loses its links.
	description := "它下面的标签会一起删除。"
	if m.deleteTag {
		title = "删除标签「" + m.deleteName + "」？"
		description = "记录上的这个标签会一起移除，记录本身不受影响。"
	}
	ui.ModalWith(c, &m.deleteOpen, title, ui.ModalSpec{
		Width:       380,
		Description: description,
	}, nil, func() {
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, "取消").Secondary().Clicked() {
				m.deleteOpen = false
			}
			if ui.Button(c, "删除").Destructive().Clicked() {
				m.confirmDelete()
			}
		})
	})
}
