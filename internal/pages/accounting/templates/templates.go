// Package templates is the 记账 · 模板 screen: the ledger's record templates in a
// table, with 新建模板, delete and drag reordering. The rows live in
// internal/store and the shared template cache keeps them available while the
// page paints.
package templates

import (
	"context"
	"strings"

	viva "github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/emptystate"
	"transactions/internal/components/layout"
	"transactions/internal/components/selectbox"
	"transactions/internal/components/semantic"
	"transactions/internal/components/tag"
	"transactions/internal/components/typography"
	"transactions/internal/icons"
	"transactions/internal/pages/accounting/ledger"
	tpl "transactions/internal/pages/accounting/template"
	"transactions/internal/repository"
)

// Table metrics: the fixed columns, the row gutter and the table's horizontal
// padding. The flexible columns share the rest by weight, with the table's
// minimum widths.
const (
	dragColumnWidth   = float32(36)
	typeColumnWidth   = float32(100)
	actionColumnWidth = float32(60)

	tablePadX = layout.Inset
	tableGap  = layout.ColumnGap
	rowHeight = layout.RowHeight

	nameWeight        = float32(1.4)
	categoryWeight    = float32(1)
	tagsWeight        = float32(1.2)
	markWeight        = float32(0.8)
	descriptionWeight = float32(1.6)
)

// Model is the 模板 page's state: the shared store, the list, and the 新建模板
// and delete dialogs.
type Model struct {
	cache           *tpl.Cache
	ledgers         *ledger.Store
	ledgerID        string
	repo            repository.Repository
	onLedgerChanged func()
	list            ui.ListState

	// categories and tags are the form's choices, loaded from the repository:
	// the first is per transaction type, the second per type and category.
	categories map[string][]string
	tags       map[string][]string

	// tagSelect is the Viva multi-select behind the form's 标签 field, with the
	// stable identities its painted hit regions need.
	tagSelect tagSelect

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

// tagSelect is the form's 标签 multi-select. Its identities come from a tree of
// its own, so its options and removals do not share scopes with the table.
type tagSelect struct {
	tree    gfx.Tree
	feature gfx.Scope
	options gfx.Scope
	removes gfx.Scope
	input   gfx.ID
	trigger gfx.ID
	dismiss gfx.ID
	scope   gfx.ID
	state   viva.MultiSelectState
	values  []string // the options the state was built from
}

// New returns an empty 模板 page over a cache of its own.
func New(cache *tpl.Cache) *Model { return NewWithLedger(cache, ledger.NewStore()) }

// NewWithLedger returns the page over the section's shared template cache and
// ledger store, so its 账本 Select follows the records and tags pages.
func NewWithLedger(cache *tpl.Cache, ledgers *ledger.Store) *Model {
	if cache == nil {
		cache = tpl.NewCache()
	}
	if ledgers == nil {
		ledgers = ledger.NewStore()
	}
	m := &Model{
		cache: cache, ledgers: ledgers, ledgerID: ledgers.Active(),
		categories: map[string][]string{}, tags: map[string][]string{},
	}
	m.tagSelect.feature = m.tagSelect.tree.Feature("template-tags")
	m.tagSelect.options = m.tagSelect.tree.Sub(m.tagSelect.feature, m.tagSelect.tree.Name("option"))
	m.tagSelect.removes = m.tagSelect.tree.Sub(m.tagSelect.feature, m.tagSelect.tree.Name("remove"))
	m.tagSelect.state.MatchCaseInsensitive = true
	m.list.Key = func(index int) int { return index }
	m.list.Label = func(index int) string {
		items := m.items()
		if index < 0 || index >= len(items) {
			return ""
		}
		return items[index].Name
	}
	m.list.Reorder = func(rows []int, to int) {
		if len(rows) > 0 {
			m.reorder(rows[0], to)
		}
	}
	return m
}

// SetDataSource connects the page to the workspace. Reload fills the template
// cache and the form's category/tag choices.
func (m *Model) SetDataSource(repo repository.Repository, onLedgerChanged func()) {
	m.repo = repo
	m.onLedgerChanged = onLedgerChanged
	if repo == nil {
		m.categories = map[string][]string{}
		m.tags = map[string][]string{}
		m.cache.Clear()
		return
	}
	m.Reload()
}

// Reload loads the active ledger's templates into the shared cache and its
// categories/tags into the form's choices.
func (m *Model) Reload() {
	if m.repo == nil {
		return
	}
	active := m.ledgers.Active()
	switched := active != m.ledgerID
	m.ledgerID = active
	if switched {
		m.list.Select(-1)
		m.list.ScrollTo(0, ui.ListStart)
	}
	m.categories = map[string][]string{}
	m.tags = map[string][]string{}
	if active == "" {
		m.cache.Clear()
		return
	}
	if snapshot, err := m.repo.Labels(context.Background(), active); err == nil {
		for _, category := range snapshot.Categories {
			m.categories[category.TransactionType] = append(m.categories[category.TransactionType], category.Name)
		}
		for _, tag := range snapshot.Tags {
			name, transactionType, ok := splitCategoryKey(tag.CategoryTransactionType)
			if !ok {
				continue
			}
			key := tagKey(transactionType, name)
			m.tags[key] = append(m.tags[key], tag.Name)
		}
	} else {
		m.toaster.Show("读取分类标签失败："+err.Error(), "", ui.ToastError)
	}
	templates, err := m.repo.Templates(context.Background(), active)
	if err != nil {
		m.toaster.Show("读取模板失败："+err.Error(), "", ui.ToastError)
		return
	}
	list := make([]tpl.Template, 0, len(templates))
	for _, one := range templates {
		list = append(list, tpl.FromDomain(one))
	}
	m.cache.Set(active, list)
}

// View declares the page: the table and its dialogs.
func (m *Model) View(c ui.C) {
	layout.Page(c, m.toolbar, func(c ui.C) {
		m.table(c)
	})
	m.formModal(c)
	m.deleteModal(c)
	ui.Toasts(c, &m.toaster)
}

// CloseOverlays closes the dialogs when the shell leaves the screen.
func (m *Model) CloseOverlays() {
	m.formOpen, m.deleteOpen = false, false
	m.tagSelect.state.Open = false
	m.form.err = ""
}

func (m *Model) toolbar(c ui.C) {
	layout.Toolbar(c, func() {
		if ledger.Picker(c, m.ledgers) && m.onLedgerChanged != nil {
			m.onLedgerChanged()
		}
		ui.Spacer(c)
		if ui.Button(c, "新建模板").Icon(icons.Plus).Clicked() {
			m.openForm()
		}
	})
}

// items are the active ledger's templates.
func (m *Model) items() []tpl.Template { return m.cache.List(m.ledgerID) }

// reorder moves one row and persists the new order through the repository.
func (m *Model) reorder(from, to int) {
	items := append([]tpl.Template(nil), m.items()...)
	if from < 0 || from >= len(items) || to < 0 || to > len(items) {
		return
	}
	if to > from {
		to--
	}
	if to == from {
		return
	}
	one := items[from]
	items = append(items[:from], items[from+1:]...)
	items = append(items[:to], append([]tpl.Template{one}, items[to:]...)...)
	m.cache.Set(m.ledgerID, items)
	if m.repo == nil {
		return
	}
	ids := make([]string, len(items))
	for index, item := range items {
		ids[index] = item.ID
	}
	if err := m.repo.ReorderTemplates(context.Background(), m.ledgerID, ids); err != nil {
		m.toaster.Show("保存模板排序失败："+err.Error(), "", ui.ToastError)
		m.Reload()
	}
}

// table is the fixed header over the scrolling rows; the header stays while
// only the rows move.
func (m *Model) table(c ui.C) {
	ui.Column(c).Grow(1).FillWidth().Children(func() {
		m.header(c)
		items := m.items()
		if len(items) == 0 {
			m.empty(c)
			return
		}
		ui.List(c, &m.list, len(items), m.row).Label("模板").Grow(1)
	})
}

func (m *Model) header(c ui.C) {
	layout.TableHeader(c, layout.TableHeaderSpec{PadX: tablePadX, Gap: tableGap}, func(c ui.C) {
		ui.Row(c).Width(dragColumnWidth)
		headerText(c, "模板名称", nameWeight, 120)
		headerCenter(c, "交易类型", typeColumnWidth)
		headerMid(c, "分类", categoryWeight, 90)
		headerText(c, "标签", tagsWeight, 110)
		headerText(c, "标记", markWeight, 70)
		headerText(c, "描述", descriptionWeight, 140)
		headerCenter(c, "操作", actionColumnWidth)
	})
}

func headerText(c ui.C, text string, grow, minWidth float32) {
	typography.Caption(c, text).Grow(grow).Basis(0).MinWidth(minWidth).Ellipsis()
}

// headerMid is headerText with the caption centred in its column, for a column
// whose values are centred too.
func headerMid(c ui.C, text string, grow, minWidth float32) {
	typography.Caption(c, text).Grow(grow).Basis(0).MinWidth(minWidth).Ellipsis().TextCenter()
}

func headerCenter(c ui.C, text string, width float32) {
	typography.Caption(c, text).Width(width).TextCenter()
}

// empty is the table body while the ledger has no templates.
func (m *Model) empty(c ui.C) {
	emptystate.Page(c, nil, "暂无模板", "把常记的一笔存成模板，记一笔时可以直接选。", nil)
}

// row declares one template: drag handle, name, type chip, category, tags,
// mark, description and the delete action.
func (m *Model) row(row ui.ListRow) {
	items := m.items()
	if row.Index >= len(items) {
		return
	}
	one := items[row.Index]
	theme := row.C.Theme()
	row.Element.FillWidth().MinHeight(rowHeight).PadSides(4, tablePadX, 4, tablePadX).
		Gap(tableGap).Items(ui.Center).
		Hover(ui.Look{}.Background(theme.Hover)).
		Focus(ui.Look{}.Ring(2, theme.Ring))

	ui.Row(row.C).Width(dragColumnWidth).Justify(ui.Center).Children(func() {
		ui.Icon(row.C, icons.GripVertical).Size(16, 16).Color(theme.MutedForeground)
	})
	typography.Body(row.C, one.Name).
		Grow(nameWeight).Basis(0).MinWidth(120).Ellipsis()

	ui.Row(row.C).Width(typeColumnWidth).Justify(ui.Center).Children(func() {
		chip(row.C, tpl.KindLabel(one.Kind), kindColor(one.Kind, *theme))
	})

	// The category column is centred: a short, single value between two blocks
	// of chips, and the same in the header.
	if one.Category != "" {
		typography.Body(row.C, one.Category).
			Grow(categoryWeight).Basis(0).MinWidth(90).Ellipsis().TextCenter()
	} else {
		ui.Row(row.C).Grow(categoryWeight).Basis(0).MinWidth(90).
			Justify(ui.Center).Children(func() {
			dash(row.C)
		})
	}

	ui.Flow(row.C).Grow(tagsWeight).Basis(0).MinWidth(110).Gap(4).Children(func() {
		if len(one.Tags) == 0 {
			dash(row.C)
			return
		}
		for _, name := range one.Tags {
			tag.Chip(row.C, name)
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
		typography.Body(row.C, one.Description).
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
	ui.Text(c, text).FontSize(typography.SizeCaption).
		Color(color).Background(theme.Background.Mix(color, 0.12)).
		PadX(8).PadY(2).Radius(theme.Metrics.RadiusItem())
}

// dash is the table's empty cell: the muted em dash every table uses for a
// missing value.
func dash(c ui.C) ui.Box {
	theme := c.Theme()
	return ui.Text(c, "—").FontSize(typography.SizeBody).Color(theme.MutedForeground)
}

// kindColor and outlierColor come from the same semantic package the records
// table and the analysis charts use: 支出 red, 收入 green, 转账 blue (its own
// series hue, not the action accent), 离群值 amber.
func kindColor(kind string, theme gfx.Theme) gfx.Color { return semantic.Kind(theme, kind) }

func outlierColor(theme gfx.Theme) gfx.Color { return semantic.Outlier(theme) }

// openForm and closeForm reset the form; the type and category are 支出 and
// nothing, as the dialog starts.
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
	m.resetTagSelect()
}

// resetTagSelect clears the multi-select's query and chosen tags.
func (m *Model) resetTagSelect() {
	m.tagSelect.state.Open = false
	query := &m.tagSelect.state.Query
	query.ReplaceRange(text.Range{Start: 0, End: query.RuneCount()}, "")
	m.tagSelect.state.SetValues(nil)
	m.form.tags = nil
}

// changeKind clears the category and the tags, since a category belongs to one
// type.
func (m *Model) changeKind() {
	m.form.category = ""
	m.resetTagSelect()
	m.form.err = ""
}

// changeCategory clears the tags: they belong to the category.
func (m *Model) changeCategory() {
	m.resetTagSelect()
	m.form.err = ""
}

// tagOptions are the 标签 choices of the form's current category.
func (m *Model) tagOptions() []string {
	if m.form.category == "" {
		return nil
	}
	return m.tags[tagKey(m.form.kind, m.form.category)]
}

// save validates the form and appends the tpl. The dialog requires a
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
	if m.repo == nil {
		m.form.err = "还没有打开工作空间"
		return
	}
	one := tpl.ToDomain(tpl.Template{
		LedgerID:    m.ledgerID,
		Name:        name,
		Kind:        m.form.kind,
		Category:    m.form.category,
		Description: strings.TrimSpace(m.form.description),
		Tags:        append([]string(nil), m.form.tags...),
		Outlier:     m.form.outlier,
	})
	if _, err := m.repo.CreateTemplate(context.Background(), one); err != nil {
		m.form.err = "保存模板失败：" + err.Error()
		return
	}
	m.Reload()
	m.list.ScrollIntoView(len(m.items()) - 1)
	m.closeForm()
	m.toaster.Show("模板已创建", "", ui.ToastSuccess)
}

func (m *Model) askDelete(one tpl.Template) {
	m.deleteOpen, m.deleteID, m.deleteName = true, one.ID, one.Name
}

func (m *Model) confirmDelete() {
	if m.repo == nil {
		m.deleteOpen = false
		m.toaster.Show("还没有打开工作空间", "", ui.ToastError)
		return
	}
	if err := m.repo.DeleteTemplate(context.Background(), m.deleteID); err != nil {
		m.deleteOpen = false
		m.toaster.Show("删除模板失败："+err.Error(), "", ui.ToastError)
		return
	}
	m.Reload()
	m.deleteOpen = false
	m.toaster.Show("模板已删除", "", ui.ToastSuccess)
}

// formModal is 新建模板.
func (m *Model) formModal(c ui.C) {
	if !m.formOpen {
		return
	}
	theme := c.Theme()
	m.syncTagOptions()
	ui.ModalWith(c, &m.formOpen, "新建模板", ui.ModalSpec{Width: 520}, func() {
		ui.Column(c).FillWidth().Gap(14).Children(func() {
			ui.FormField(c, &m.fields.name, "模板名称").FillWidth().Height(64).Children(func() {
				ui.TextField(c, &m.form.name, "模板名称", "例如：工作日午餐").FillWidth()
			})
			ui.FormField(c, &m.fields.kind, "交易类型").FillWidth().Height(64).Children(func() {
				if ui.Choice(c, &m.form.kind, tpl.Kinds[:], tpl.KindLabel).Changed() {
					m.changeKind()
				}
			})
			ui.FormField(c, &m.fields.category, "分类").FillWidth().Height(64).Children(func() {
				if selectbox.Select(c, &m.form.category, m.categories[m.form.kind], nil).
					Placeholder("选择分类").Label("分类").Changed() {
					m.changeCategory()
				}
			})
			ui.FormField(c, &m.fields.tags, "标签").FillWidth().Height(64).Children(func() {
				m.tagField(c)
			})
			ui.FormField(c, &m.fields.description, "描述").FillWidth().Height(64).Children(func() {
				ui.TextField(c, &m.form.description, "描述", "例如：公司楼下的午餐").FillWidth()
			})
			ui.FormField(c, &m.fields.mark, "标记").FillWidth().Height(64).Children(func() {
				ui.Checkbox(c, &m.form.outlier, "离群值")
			})
			if m.form.err != "" {
				typography.Caption(c, m.form.err).Color(theme.Danger)
			}
			ui.Divider(c)
		})
	}, func() {
		if ui.Button(c, "取消").Secondary().Clicked() {
			m.closeForm()
		}
		if ui.Button(c, "创建").Clicked() {
			m.save()
		}
	})
}

// tagField paints Viva's multi-select in the 标签 field. It is disabled (with a
// hint in the placeholder) until the category has tags.
func (m *Model) tagField(c ui.C) {
	theme := c.Theme()
	options := m.tagOptions()
	disabled := len(options) == 0
	placeholder := "选择一个或多个标签"
	switch {
	case m.form.category == "":
		placeholder = "请先选择分类"
	case disabled:
		placeholder = "该分类下暂无标签"
	}
	// These four are focus and input identities, so they come from the page's
	// tree (ui.ID) rather than the component's option scopes.
	m.tagSelect.input = ui.ID(c)
	m.tagSelect.trigger = ui.ID(c)
	m.tagSelect.dismiss = ui.ID(c)
	m.tagSelect.scope = ui.ID(c)
	// The dialog has its own compositing layer; 20 keeps both the field and its
	// popup above it.
	ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight).Draw(func(d *ui.DrawContext) {
		viva.PaintMultiSelect(d.Canvas, *d.Theme, d.Rect, &m.tagSelect.state, viva.MultiSelectOptions{
			InputID: m.tagSelect.input, TriggerID: m.tagSelect.trigger,
			DismissID: m.tagSelect.dismiss, Scope: m.tagSelect.scope,
			OptionIDs: m.tagSelect.options, RemoveIDs: m.tagSelect.removes,
			Label: "标签", Placeholder: placeholder,
			EmptyLabel: "没有匹配项", CreatePrefix: "创建：",
			Viewport: c.Bounds(), Layer: 20, MaxRows: 7, Disabled: disabled,
		})
		if disabled {
			return
		}
		changed := false
		switch {
		case d.Canvas.Clicked(m.tagSelect.input):
			m.tagSelect.state.Open, changed = true, true
		case d.Canvas.Clicked(m.tagSelect.trigger):
			m.tagSelect.state.Open, changed = !m.tagSelect.state.Open, true
		case d.Canvas.Clicked(m.tagSelect.dismiss):
			m.tagSelect.state.Open, changed = false, true
		default:
			if index, ok := d.Canvas.ClickedIn(m.tagSelect.options); ok {
				changed = m.tagSelect.state.Choose(index) || changed
			}
			if index, ok := d.Canvas.ClickedIn(m.tagSelect.removes); ok {
				changed = m.tagSelect.state.Remove(index) || changed
			}
		}
		m.syncTags()
		if changed {
			d.Canvas.RequestFrame()
		}
	})
}

// syncTagOptions hands the multi-select its choices when the category changes.
func (m *Model) syncTagOptions() {
	values := m.tagOptions()
	same := len(values) == len(m.tagSelect.values)
	for index := range values {
		if !same || values[index] != m.tagSelect.values[index] {
			same = false
			break
		}
	}
	if same {
		return
	}
	m.tagSelect.values = append(m.tagSelect.values[:0], values...)
	choices := make([]viva.MultiSelectOption, len(values))
	for index, value := range values {
		choices[index] = viva.MultiSelectOption{Value: value, Label: value}
	}
	m.tagSelect.state.SetOptions(choices)
}

// syncTags copies the multi-select's values into the form.
func (m *Model) syncTags() {
	m.form.tags = append(m.form.tags[:0], m.tagSelect.state.Values()...)
}

// Key routes the multi-select popup's navigation keys while the dialog is open.
func (m *Model) Key(event gfx.KeyEvent) gfx.EventResult {
	if !m.formOpen {
		return gfx.EventResult{}
	}
	if event.Target != m.tagSelect.input &&
		(!m.tagSelect.state.Open || event.Key != gfx.KeyEscape && event.Key != gfx.KeyTab) {
		return gfx.EventResult{}
	}
	result := m.tagSelect.state.Key(event)
	m.syncTags()
	return result
}

// Scroll moves the multi-select popup while it is open.
func (m *Model) Scroll(event gfx.ScrollEvent) gfx.EventResult {
	if !m.formOpen || !m.tagSelect.state.Open || !m.tagSelect.state.Wheel(event) {
		return gfx.EventResult{}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

// TagsEditor is the native editor of the multi-select query while the field has
// focus, for Config.TextInput.
func (m *Model) TagsEditor(id gfx.ID) *text.Editor {
	if !m.formOpen || id != m.tagSelect.input {
		return nil
	}
	return &m.tagSelect.state.Query
}

// deleteModal is the 删除模板 confirmation.
func (m *Model) deleteModal(c ui.C) {
	if !m.deleteOpen {
		return
	}
	ui.ModalWith(c, &m.deleteOpen, "删除模板「"+m.deleteName+"」？", ui.ModalSpec{
		Width:       380,
		Description: "只删除模板，已经记下的记录不受影响。",
	}, nil, func() {
		if ui.Button(c, "取消").Secondary().Clicked() {
			m.deleteOpen = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
			m.confirmDelete()
		}
	})
}

func tagKey(kind, category string) string { return kind + "\x00" + category }

func splitCategoryKey(value string) (name, transactionType string, ok bool) {
	index := strings.LastIndex(value, ":")
	if index <= 0 || index == len(value)-1 {
		return "", "", false
	}
	return value[:index], value[index+1:], true
}
