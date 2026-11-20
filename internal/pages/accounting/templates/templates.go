// Package templates is the 记账 · 模板 screen: the ledger's record templates in a
// table, with 新建模板, delete and drag reordering. It follows the old
// application's templates page; the entries live in the shared mock store of
// internal/pages/accounting/template, which the 记一笔 dialog also uses.
package templates

import (
	"strings"

	viva "github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/selectbox"
	"transactions/internal/icons"
	tpl "transactions/internal/pages/accounting/template"
)

// Table metrics: the fixed columns, the row gutter and the table's horizontal
// padding. The flexible columns share the rest by weight, with the old table's
// minimum widths.
const (
	dragColumnWidth   = float32(36)
	typeColumnWidth   = float32(100)
	actionColumnWidth = float32(60)

	tablePadX = float32(12)
	tableGap  = float32(8)
	rowHeight = float32(40)

	nameWeight        = float32(1.4)
	categoryWeight    = float32(1)
	tagsWeight        = float32(1.2)
	markWeight        = float32(0.8)
	descriptionWeight = float32(1.6)
)

// Model is the 模板 page's state: the shared store, the list, and the 新建模板
// and delete dialogs.
type Model struct {
	store *tpl.Store
	list  ui.ListState

	// categories and tags are the form's mock choices, built once: the first is
	// per transaction type, the second per type and category.
	categories map[string][]string
	tags       map[string][]string

	formOpen bool
	form     form
	fields   formFields

	deleteOpen bool
	deleteID   string
	deleteName string

	toaster ui.Toaster
}

type form struct {
	name        string
	kind        string
	category    string
	description string
	tags        []string
	outlier     bool
	err         string
}

type formFields struct {
	name        ui.FormFieldState
	kind        ui.FormFieldState
	category    ui.FormFieldState
	tags        ui.FormFieldState
	description ui.FormFieldState
	mark        ui.FormFieldState
}

// New returns the 模板 page over the ledger's shared template store.
func New(store *tpl.Store) *Model {
	if store == nil {
		store = tpl.NewStore()
	}
	m := &Model{store: store, categories: map[string][]string{}, tags: map[string][]string{}}
	for kind, groups := range mockCategories {
		names := make([]string, len(groups))
		for index, group := range groups {
			names[index] = group.name
			m.tags[tagKey(kind, group.name)] = group.tags
		}
		m.categories[kind] = names
	}
	m.list.Key = func(index int) int { return index }
	m.list.Label = func(index int) string {
		items := m.store.List()
		if index < 0 || index >= len(items) {
			return ""
		}
		return items[index].Name
	}
	m.list.Reorder = func(rows []int, to int) {
		if len(rows) > 0 {
			m.store.Move(rows[0], to)
		}
	}
	return m
}

// View declares the page: the table and its dialogs.
func (m *Model) View(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		m.toolbar(c)
		ui.Divider(c)
		m.table(c)
	})
	m.formModal(c)
	m.deleteModal(c)
	ui.Toasts(c, &m.toaster)
}

// CloseOverlays closes the dialogs when the shell leaves the screen.
func (m *Model) CloseOverlays() {
	m.formOpen, m.deleteOpen = false, false
	m.form.err = ""
}

func (m *Model) toolbar(c ui.C) {
	ui.Row(c).Height(56).FillWidth().PadX(18).Items(ui.Center).Children(func() {
		ui.Spacer(c)
		if ui.Button(c, "新建模板").Icon(icons.Plus).Clicked() {
			m.openForm()
		}
	})
}

// table is the fixed header over the scrolling rows; the header stays while
// only the rows move.
func (m *Model) table(c ui.C) {
	ui.Column(c).Grow(1).FillWidth().Children(func() {
		m.header(c)
		ui.Divider(c)
		items := m.store.List()
		if len(items) == 0 {
			m.empty(c)
			return
		}
		ui.List(c, &m.list, len(items), m.row).Label("模板").Grow(1)
	})
}

func (m *Model) header(c ui.C) {
	theme := c.Theme()
	ui.Row(c).Height(36).FillWidth().PadX(tablePadX).Gap(tableGap).Items(ui.Center).
		Background(theme.Muted).Children(func() {
		ui.Row(c).Width(dragColumnWidth)
		headerText(c, "模板名称", nameWeight, 120)
		headerCenter(c, "交易类型", typeColumnWidth)
		headerText(c, "分类", categoryWeight, 90)
		headerText(c, "标签", tagsWeight, 110)
		headerText(c, "标记", markWeight, 70)
		headerText(c, "描述", descriptionWeight, 140)
		headerCenter(c, "操作", actionColumnWidth)
	})
}

func headerText(c ui.C, text string, grow, minWidth float32) {
	theme := c.Theme()
	ui.Text(c, text).FontSize(theme.Metrics.FontSizeS).Weight(gfx.WeightMedium).Muted().
		Grow(grow).Basis(0).MinWidth(minWidth).Ellipsis()
}

func headerCenter(c ui.C, text string, width float32) {
	theme := c.Theme()
	ui.Text(c, text).FontSize(theme.Metrics.FontSizeS).Weight(gfx.WeightMedium).Muted().
		Width(width).TextCenter()
}

// empty is the table body while the ledger has no templates.
func (m *Model) empty(c ui.C) {
	ui.Slot(c).Grow(1).FillWidth().Draw(func(d *ui.DrawContext) {
		viva.Empty(d.Canvas, *d.Theme, d.Rect, viva.EmptyData{
			Media: viva.EmptyMediaDefault,
			Title: "暂无模板",
		})
	})
}

// row declares one template: drag handle, name, type chip, category, tags,
// mark, description and the delete action.
func (m *Model) row(row ui.ListRow) {
	items := m.store.List()
	if row.Index >= len(items) {
		return
	}
	one := items[row.Index]
	theme := row.C.Theme()
	row.Element.FillWidth().MinHeight(rowHeight).PadSides(4, tablePadX, 4, tablePadX).
		Gap(tableGap).Items(ui.Center).
		Hover(ui.Look{}.Background(theme.Hover)).
		Focus(ui.Look{}.Ring(2, theme.Ring)).
		Draw(func(d *ui.DrawContext) {
			width := d.Rect.Width - 2*tablePadX
			if width <= 0 {
				return
			}
			d.Canvas.Fill(gfx.Rect{
				X:      d.Rect.X + tablePadX,
				Y:      d.Rect.Bottom() - 1,
				Width:  width,
				Height: 1,
			}, d.Theme.Border)
		})

	ui.Row(row.C).Width(dragColumnWidth).Justify(ui.Center).Children(func() {
		ui.Icon(row.C, icons.GripVertical).Size(16, 16).Color(theme.MutedForeground)
	})
	ui.Text(row.C, one.Name).FontSize(theme.Metrics.FontSizeM).
		Grow(nameWeight).Basis(0).MinWidth(120).Ellipsis()

	ui.Row(row.C).Width(typeColumnWidth).Justify(ui.Center).Children(func() {
		chip(row.C, tpl.KindLabel(one.Kind), kindColor(one.Kind, *theme))
	})

	if one.Category != "" {
		ui.Text(row.C, one.Category).FontSize(theme.Metrics.FontSizeM).
			Grow(categoryWeight).Basis(0).MinWidth(90).Ellipsis()
	} else {
		dash(row.C).Grow(categoryWeight).Basis(0).MinWidth(90)
	}

	ui.Flow(row.C).Grow(tagsWeight).Basis(0).MinWidth(110).Gap(4).Children(func() {
		if len(one.Tags) == 0 {
			dash(row.C)
			return
		}
		for _, name := range one.Tags {
			tagChip(row.C, name)
		}
	})

	ui.Row(row.C).Grow(markWeight).Basis(0).MinWidth(70).Items(ui.Start).Children(func() {
		if one.Outlier {
			chip(row.C, "离群值", outlierColor(*theme))
			return
		}
		dash(row.C)
	})

	if one.Description != "" {
		ui.Text(row.C, one.Description).FontSize(theme.Metrics.FontSizeM).
			Grow(descriptionWeight).Basis(0).MinWidth(140).Ellipsis()
	} else {
		dash(row.C).Grow(descriptionWeight).Basis(0).MinWidth(140)
	}

	ui.Row(row.C).Width(actionColumnWidth).Justify(ui.Center).Children(func() {
		remove := ui.Button(row.C, "删除模板 "+one.Name).
			Ghost().Icon(icons.Trash2).IconOnly().Tooltip("删除模板")
		if remove.Clicked() {
			m.askDelete(one)
		}
	})
}

// chip is a semantic label: the type of a template, or its mark.
func chip(c ui.C, text string, color gfx.Color) {
	theme := c.Theme()
	ui.Text(c, text).FontSize(theme.Metrics.FontSizeS).Weight(gfx.WeightMedium).
		Color(color).Background(theme.Background.Mix(color, 0.12)).
		PadX(8).PadY(2).Radius(theme.Metrics.RadiusItem())
}

// tagChip is one tag of a template, in the old table's muted capsule.
func tagChip(c ui.C, text string) {
	theme := c.Theme()
	ui.Text(c, text).FontSize(theme.Metrics.FontSizeS).
		Color(theme.MutedForeground).Background(theme.Muted).
		PadX(6).PadY(1).Radius(theme.Metrics.RadiusItem())
}

// dash is the old table's empty cell: a muted hyphen.
func dash(c ui.C) ui.Box {
	theme := c.Theme()
	return ui.Text(c, "-").FontSize(theme.Metrics.FontSizeM).Color(theme.MutedForeground)
}

// kindColor is the type chip's hue: 支出 red, 收入 green, 转账 blue.
func kindColor(kind string, theme gfx.Theme) gfx.Color {
	switch kind {
	case tpl.KindIncome:
		return theme.Success
	case tpl.KindTransfer:
		return theme.Primary
	}
	return theme.Danger
}

// The 离群值 mark has no theme color; it keeps the old project's amber.
var (
	outlierLight = gfx.RGB24(0xd97706)
	outlierDark  = gfx.RGB24(0xfbbf24)
)

func outlierColor(theme gfx.Theme) gfx.Color {
	if theme.Background.Red < 0.5 {
		return outlierDark
	}
	return outlierLight
}

// openForm and closeForm reset the form; the type and category are 支出 and
// nothing, as the old dialog starts.
func (m *Model) openForm() {
	m.resetForm()
	m.formOpen = true
}

func (m *Model) closeForm() {
	m.formOpen = false
	m.resetForm()
}

func (m *Model) resetForm() {
	m.form = form{kind: tpl.KindExpense}
	m.form.tags = nil
}

// changeKind clears the category and the tags, since a category belongs to one
// type.
func (m *Model) changeKind() {
	m.form.category = ""
	m.form.tags = nil
	m.form.err = ""
}

// changeCategory clears the tags: they belong to the category.
func (m *Model) changeCategory() {
	m.form.tags = nil
	m.form.err = ""
}

// tagOptions are the 标签 choices of the form's current category.
func (m *Model) tagOptions() []string {
	if m.form.category == "" {
		return nil
	}
	return m.tags[tagKey(m.form.kind, m.form.category)]
}

func (m *Model) hasTag(name string) bool {
	for _, tag := range m.form.tags {
		if tag == name {
			return true
		}
	}
	return false
}

func (m *Model) toggleTag(name string, on bool) {
	kept := m.form.tags[:0]
	for _, tag := range m.form.tags {
		if tag != name {
			kept = append(kept, tag)
		}
	}
	m.form.tags = kept
	if on {
		m.form.tags = append(m.form.tags, name)
	}
}

// save validates the form and appends the tpl. The old dialog requires a
// name and a category, in that order.
func (m *Model) save() {
	name := strings.TrimSpace(m.form.name)
	if name == "" {
		m.form.err = "模板名称不能为空"
		return
	}
	if m.form.category == "" {
		m.form.err = "分类不能为空"
		return
	}
	m.store.Add(tpl.Template{
		Name:        name,
		Kind:        m.form.kind,
		Category:    m.form.category,
		Description: strings.TrimSpace(m.form.description),
		Tags:        append([]string(nil), m.form.tags...),
		Outlier:     m.form.outlier,
	})
	m.list.ScrollIntoView(len(m.store.List()) - 1)
	m.closeForm()
	m.toaster.Show("保存模板成功", "", ui.ToastSuccess)
}

func (m *Model) askDelete(one tpl.Template) {
	m.deleteOpen, m.deleteID, m.deleteName = true, one.ID, one.Name
}

func (m *Model) confirmDelete() {
	m.store.Delete(m.deleteID)
	m.deleteOpen = false
	m.toaster.Show("删除模板成功", "", ui.ToastSuccess)
}

// formModal is 新建模板.
func (m *Model) formModal(c ui.C) {
	if !m.formOpen {
		return
	}
	theme := c.Theme()
	ui.ModalWith(c, &m.formOpen, "新建模板", ui.ModalSpec{Width: 520}, func() {
		ui.Column(c).FillWidth().Gap(14).Children(func() {
			ui.FormField(c, &m.fields.name, "模板名称").FillWidth().Height(64).Children(func() {
				ui.TextField(c, &m.form.name, "模板名称", "请输入模板名称").FillWidth()
			})
			ui.FormField(c, &m.fields.kind, "交易类型").FillWidth().Height(64).Children(func() {
				if ui.Choice(c, &m.form.kind, tpl.Kinds[:], tpl.KindLabel).Changed() {
					m.changeKind()
				}
			})
			ui.FormField(c, &m.fields.category, "分类").FillWidth().Height(64).Children(func() {
				if selectbox.Select(c, &m.form.category, m.categories[m.form.kind], nil).
					Placeholder("选择消费分类").Label("分类").Changed() {
					m.changeCategory()
				}
			})
			ui.FormField(c, &m.fields.tags, "标签").FillWidth().Height(64).Children(func() {
				m.tagChoices(c)
			})
			ui.FormField(c, &m.fields.description, "描述").FillWidth().Height(64).Children(func() {
				ui.TextField(c, &m.form.description, "描述", "请输入描述").FillWidth()
			})
			ui.FormField(c, &m.fields.mark, "标记").FillWidth().Height(64).Children(func() {
				ui.Checkbox(c, &m.form.outlier, "离群值")
			})
			if m.form.err != "" {
				ui.Text(c, m.form.err).FontSize(12).Color(theme.Danger)
			}
			ui.Divider(c)
		})
	}, func() {
		if ui.Button(c, "取消").Secondary().Clicked() {
			m.closeForm()
		}
		if ui.Button(c, "保存").Clicked() {
			m.save()
		}
	})
}

// tagChoices are the 标签 checkboxes of the chosen category, or the hint the old
// form shows while it has none.
func (m *Model) tagChoices(c ui.C) {
	choices := m.tagOptions()
	if len(choices) == 0 {
		ui.Text(c, "该分类下暂无标签").FontSize(12).Muted()
		return
	}
	ui.Flow(c).Gap(8).Children(func() {
		for _, name := range choices {
			c.Key(name)
			on := m.hasTag(name)
			if ui.Checkbox(c, &on, name).Changed() {
				m.toggleTag(name, on)
			}
		}
	})
}

// deleteModal is the 删除模板 confirmation.
func (m *Model) deleteModal(c ui.C) {
	if !m.deleteOpen {
		return
	}
	ui.ModalWith(c, &m.deleteOpen, "删除模板", ui.ModalSpec{Width: 380}, func() {
		ui.Text(c, "删除模板「"+m.deleteName+"」？").FontSize(13)
	}, func() {
		if ui.Button(c, "取消").Secondary().Clicked() {
			m.deleteOpen = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
			m.confirmDelete()
		}
	})
}

func tagKey(kind, category string) string { return kind + "\x00" + category }
