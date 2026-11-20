// Package tags is the 记账 · 标签 screen: the categories of a transaction type on
// the left and the tags of the chosen category on the right, with add, delete
// and drag reordering. It follows the old application's category-tag page; the
// data is mock until internal/store lands.
package tags

import (
	"strconv"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/icons"
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
	categoryColumnWidth = float32(240)
	columnHeaderHeight  = float32(46)
	listRowHeight       = float32(40)
)

// Model is the 标签 screen's state: the chosen type, the categories and tags,
// the two lists (selection and drag reordering), and the dialogs.
type Model struct {
	kind       kind
	categories [kindCount][]category
	tags       [kindCount]map[string][]tag
	selected   string
	nextID     int

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

// New returns the 标签 page on 支出 with the mock defaults.
func New() *Model {
	m := &Model{kind: kindExpense}
	for k := range m.tags {
		m.tags[k] = map[string][]tag{}
	}
	for k := range mockGroups {
		for _, group := range mockGroups[k] {
			m.categories[k] = append(m.categories[k], category{
				id: m.takeID(), name: group.name, count: group.count,
			})
			for _, mockTag := range group.tags {
				m.tags[k][group.name] = append(m.tags[k][group.name], tag{
					id: m.takeID(), name: mockTag.name, count: mockTag.count,
				})
			}
		}
	}
	m.categoryList.Key = m.categoryKey
	m.categoryList.Label = m.categoryNameAt
	m.categoryList.Reorder = m.moveCategory
	m.tagList.Key = m.tagKey
	m.tagList.Label = m.tagNameAt
	m.tagList.Reorder = m.moveTag
	return m
}

func (m *Model) takeID() int {
	m.nextID++
	return m.nextID
}

// View declares the page: the type cards, the two columns and the dialogs.
func (m *Model) View(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		m.toolbar(c)
		ui.Divider(c)
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
	ui.Row(c).Height(56).FillWidth().PadX(18).Gap(8).Items(ui.Center).Children(func() {
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
	card := ui.Row(c).Height(36).PadX(14).Items(ui.Center).
		Radius(theme.Metrics.RadiusControl).Border(1, border).
		HitWith(ui.ID(c), k.label(), gfx.HitOptions{
			Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand,
		}).
		Focus(ui.Look{}.Ring(2, theme.Ring))
	if active {
		card = card.Background(theme.Background.Mix(color, 0.08)).Shadow(gfx.ShadowSM)
	} else {
		card = card.Hover(ui.Look{}.Background(theme.Hover))
	}
	card.Children(func() {
		ui.Text(c, k.label()).FontSize(theme.Metrics.FontSizeM).
			Weight(gfx.WeightMedium).Color(foreground)
	})
	if card.Clicked() {
		m.selectKind(k)
	}
}

// selectKind switches the transaction type and clears the chosen category, as
// the old page does.
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
	ui.Column(c).Width(categoryColumnWidth).FillHeight().Children(func() {
		add := m.columnHeader(c, "分类", len(categories), "新增分类", false, false)
		if add.Clicked() {
			m.openCategoryModal()
		}
		ui.Divider(c)
		if len(categories) == 0 {
			emptyState(c, "暂无分类")
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
		add := m.columnHeader(c, title, len(tags), "新增标签", m.selected == "", true)
		if add.Clicked() {
			m.openTagModal()
		}
		ui.Divider(c)
		switch {
		case m.selected == "":
			emptyState(c, "选择分类查看标签")
		case len(tags) == 0:
			emptyState(c, "暂无标签")
		default:
			ui.List(c, &m.tagList, len(tags), m.tagRow).Label("标签").Grow(1)
		}
	})
}

// columnHeader is one column's title, its count badge and its add button. The
// caller reads Clicked from the returned button.
func (m *Model) columnHeader(c ui.C, title string, count int, addLabel string, disabled, secondary bool) ui.Box {
	theme := c.Theme()
	var add ui.Box
	ui.Row(c).Height(columnHeaderHeight).FillWidth().PadX(18).Gap(8).
		Items(ui.Center).Children(func() {
		ui.Text(c, title).FontSize(theme.Metrics.FontSizeM).
			Weight(gfx.WeightSemibold).Shrink(1).Ellipsis()
		countPill(c, count)
		ui.Spacer(c)
		add = ui.Button(c, addLabel).Small().Icon(icons.Plus).Disabled(disabled)
		if secondary {
			add = add.Secondary()
		}
	})
	return add
}

// countPill is a count in a muted capsule.
func countPill(c ui.C, count int) {
	theme := c.Theme()
	ui.Text(c, strconv.Itoa(count)).FontSize(theme.Metrics.FontSizeS).
		Color(theme.MutedForeground).Background(theme.Muted).
		Radius(theme.Metrics.RadiusItem() + 2).PadX(6).PadY(1)
}

// emptyState fills the body below a column header with its explanation.
func emptyState(c ui.C, text string) {
	theme := c.Theme()
	ui.Column(c).Grow(1).FillWidth().Center().Children(func() {
		ui.Text(c, text).FontSize(theme.Metrics.FontSizeM).Muted().TextCenter()
	})
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
	row.Element.Height(listRowHeight).PadX(8).Gap(8).Items(ui.Center).
		Radius(theme.Metrics.RadiusControl).
		Hover(ui.Look{}.Background(hover)).
		Focus(ui.Look{}.Ring(2, theme.Ring))
	if row.Selected() {
		row.Element.Background(theme.Accent)
	}
	ui.Icon(row.C, icons.GripVertical).Size(16, 16).Color(theme.MutedForeground)
	ui.Text(row.C, cat.name).FontSize(theme.Metrics.FontSizeM).Grow(1).Ellipsis()
	if cat.count > 0 {
		countPill(row.C, cat.count)
	}
	remove := ui.Button(row.C, "删除分类 "+cat.name).
		Ghost().Icon(icons.Trash2).IconOnly().Tooltip("删除分类")
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
	row.Element.Height(listRowHeight).PadX(8).Gap(8).Items(ui.Center).
		Radius(theme.Metrics.RadiusControl).
		Hover(ui.Look{}.Background(theme.Hover)).
		Focus(ui.Look{}.Ring(2, theme.Ring))
	ui.Icon(row.C, icons.GripVertical).Size(16, 16).Color(theme.MutedForeground)
	ui.Text(row.C, one.name).FontSize(theme.Metrics.FontSizeM).Grow(1).Ellipsis()
	if one.count > 0 {
		countPill(row.C, one.count)
	}
	remove := ui.Button(row.C, "删除标签 "+one.name).
		Ghost().Icon(icons.Trash2).IconOnly().Tooltip("删除标签")
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

// moveCategory and moveTag apply a drag reordering; the slice order is the
// stored order, so moving the row is all there is to persist in the mock.
func (m *Model) moveCategory(rows []int, to int) {
	if len(rows) == 0 {
		return
	}
	categories := m.categories[m.kind]
	m.categories[m.kind] = moveItem(categories, rows[0], to)
}

func (m *Model) moveTag(rows []int, to int) {
	if len(rows) == 0 || m.selected == "" {
		return
	}
	m.tags[m.kind][m.selected] = moveItem(m.currentTags(), rows[0], to)
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
	m.categories[m.kind] = append(m.categories[m.kind], category{id: m.takeID(), name: name})
	m.selected = name
	index := len(m.categories[m.kind]) - 1
	m.categoryList.Select(index)
	m.categoryList.ScrollIntoView(index)
	m.tagList.Select(-1)
	m.tagList.ScrollTo(0, ui.ListStart)
	m.closeCategoryModal()
	m.toaster.Show("分类已添加", "", ui.ToastSuccess)
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
	m.tags[m.kind][m.selected] = append(m.currentTags(), tag{id: m.takeID(), name: name})
	m.tagList.ScrollIntoView(len(m.currentTags()) - 1)
	m.closeTagModal()
	m.toaster.Show("标签已添加", "", ui.ToastSuccess)
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
	if m.deleteTag {
		m.tags[m.kind][m.selected] = withoutTag(m.currentTags(), m.deleteName)
		m.tagList.Select(-1)
		m.toaster.Show("标签已删除", "", ui.ToastSuccess)
	} else {
		m.categories[m.kind] = withoutCategory(m.categories[m.kind], m.deleteName)
		delete(m.tags[m.kind], m.deleteName)
		if m.selected == m.deleteName {
			m.selected = ""
			m.categoryList.Select(-1)
			m.tagList.Select(-1)
			m.tagList.ScrollTo(0, ui.ListStart)
		}
		m.toaster.Show("分类已删除", "", ui.ToastSuccess)
	}
	m.deleteOpen = false
}

func withoutCategory(items []category, name string) []category {
	for index := range items {
		if items[index].name != name {
			continue
		}
		return append(items[:index], items[index+1:]...)
	}
	return items
}

func withoutTag(items []tag, name string) []tag {
	for index := range items {
		if items[index].name != name {
			continue
		}
		return append(items[:index], items[index+1:]...)
	}
	return items
}

// categoryModal is the 新增分类 dialog.
func (m *Model) categoryModal(c ui.C) {
	if !m.categoryOpen {
		return
	}
	theme := c.Theme()
	ui.ModalWith(c, &m.categoryOpen, "新增分类", ui.ModalSpec{Width: 400}, func() {
		field := ui.TextField(c, &m.categoryName, "分类名称", "输入分类名称").
			AutoFocus().FillWidth()
		if m.categoryError != "" {
			ui.Text(c, m.categoryError).FontSize(12).Color(theme.Danger).MarginTop(6)
		}
		if field.Submitted() {
			m.confirmCategory()
		}
	}, func() {
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, "取消").Secondary().Clicked() {
				m.closeCategoryModal()
			}
			confirm := ui.Button(c, "确认").
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
		field := ui.TextField(c, &m.tagName, "标签名称", "输入标签名称").
			AutoFocus().FillWidth()
		if m.tagError != "" {
			ui.Text(c, m.tagError).FontSize(12).Color(theme.Danger).MarginTop(6)
		}
		if field.Submitted() {
			m.confirmTag()
		}
	}, func() {
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, "取消").Secondary().Clicked() {
				m.closeTagModal()
			}
			confirm := ui.Button(c, "确认").Disabled(strings.TrimSpace(m.tagName) == "")
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
	title, what := "删除分类", "分类"
	if m.deleteTag {
		title, what = "删除标签", "标签"
	}
	ui.ModalWith(c, &m.deleteOpen, title, ui.ModalSpec{Width: 380}, func() {
		ui.Text(c, "确定删除"+what+"「"+m.deleteName+"」？").FontSize(13)
	}, func() {
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
