package components

import (
	"strconv"

	viva "github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// paginationSlots is the widest page window Viva's pagination shows before it
// falls back to an ellipsis.
const paginationSlots = 7

// PaginationSpec is one page of a list: what is shown, where it is, and what to
// do when the user moves to another page or changes the page size.
//
// Page is 1-based. PageSizes defaults to 15, 30 and 50 when empty; PageSize
// should be one of them.
type PaginationSpec struct {
	Total      int
	Page       int
	PageSize   int
	PageSizes  []int
	OnPage     func(page int)
	OnPageSize func(size int)
}

// paginationItem is one slot of the page chooser: a page number, an ellipsis,
// or an unused slot when there are fewer pages than slots.
type paginationItem struct {
	number   int
	ellipsis bool
}

// Pagination is the shared bottom bar for a paginated list: the page-size
// Select on the left of Viva's pagination component, and the total on its
// right.
//
// It paints Viva's components.BeginPagination through a Slot, naming the
// component's hit regions in the page before the slot is declared. A click is
// applied after the paint, through the caller's OnPage.
func Pagination(c ui.C, spec PaginationSpec) {
	sizes := spec.PageSizes
	if len(sizes) == 0 {
		sizes = []int{15, 30, 50}
	}
	pageSize := spec.PageSize
	if pageSize <= 0 {
		pageSize = sizes[0]
	}
	total := max(spec.Total, 0)
	pages := 1
	if pageSize > 0 {
		pages = max((total+pageSize-1)/pageSize, 1)
	}
	page := min(max(spec.Page, 1), pages)
	items := paginationItems(page, pages)

	size := pageSize
	ui.Row(c).Height(48).FillWidth().PadX(18).Gap(10).Items(ui.Center).Children(func() {
		ui.Spacer(c)
		ui.Text(c, "每页").FontSize(12).Muted()
		selectBox := Select(c, &size, sizes, strconv.Itoa).Label("每页条数")
		selectBox.Width(84)
		if selectBox.Changed() && spec.OnPageSize != nil {
			spec.OnPageSize(size)
		}
		ui.Text(c, "条").FontSize(12).Muted()

		// Viva's pagination paints its own hits; name them before the slot.
		var ids [paginationSlots + 2]gfx.ID
		for index := range ids {
			ids[index] = ui.ID(c)
		}
		height := c.Theme().Metrics.ControlHeight
		width := paginationWidth(c.Canvas(), c.Theme().Metrics, items, height)
		ui.Slot(c).Width(width).Height(height).Draw(func(d *ui.DrawContext) {
			drawPagination(d, spec, ids, items, page, pages)
		})

		ui.Text(c, "共 "+strconv.Itoa(total)+" 条").FontSize(12).Muted()
	})
}

// paginationItems fills the seven page slots: every page when they fit,
// otherwise the first and last page with an ellipsis around the current one.
func paginationItems(page, pages int) [paginationSlots]paginationItem {
	var items [paginationSlots]paginationItem
	if pages <= paginationSlots {
		for number := 1; number <= pages; number++ {
			items[number-1].number = number
		}
		return items
	}
	switch {
	case page <= 4:
		for index := 0; index < 5; index++ {
			items[index].number = index + 1
		}
		items[5].ellipsis = true
		items[6].number = pages
	case page >= pages-3:
		items[0].number = 1
		items[1].ellipsis = true
		for index := 0; index < 5; index++ {
			items[2+index].number = pages - 4 + index
		}
	default:
		items[0].number = 1
		items[1].ellipsis = true
		items[2].number = page - 1
		items[3].number = page
		items[4].number = page + 1
		items[5].ellipsis = true
		items[6].number = pages
	}
	return items
}

// paginationWidth measures Viva's pagination row: the previous and next
// buttons, the page squares and the gaps Viva places between them.
func paginationWidth(canvas *gfx.Canvas, metrics gfx.Metrics,
	items [paginationSlots]paginationItem, itemHeight float32) float32 {
	edge := metrics.GapItem
	inner := metrics.GapItem * 1.25
	gap := metrics.GapItem * 0.75
	spacing := metrics.GapItem / 2
	previousWidth := edge + 16 + gap +
		canvas.TextWidthWeight("上一页", metrics.FontSizeM, gfx.WeightMedium) + inner
	nextWidth := edge + 16 + gap +
		canvas.TextWidthWeight("下一页", metrics.FontSizeM, gfx.WeightMedium) + inner
	count := 0
	for _, item := range items {
		if item.number > 0 || item.ellipsis {
			count++
		}
	}
	return previousWidth + nextWidth + float32(count)*itemHeight +
		float32(count+1)*spacing
}

// drawPagination paints Viva's pagination in its slot and turns a click into a
// page change through the caller's OnPage.
func drawPagination(d *ui.DrawContext, spec PaginationSpec, ids [paginationSlots + 2]gfx.ID,
	items [paginationSlots]paginationItem, page, pages int) {
	theme := *d.Theme
	metrics := theme.Metrics
	itemHeight := min(metrics.ControlHeight, d.Rect.Height)
	width := min(paginationWidth(d.Canvas, metrics, items, itemHeight), d.Rect.Width)
	bounds := gfx.Rect{
		X: d.Rect.X + d.Rect.Width - width, Y: d.Rect.Y,
		Width: width, Height: itemHeight,
	}

	pagination := viva.BeginPagination(d.Canvas, theme, bounds)
	pagination.AddPrevious(ids[0], "上一页", page <= 1)
	for index, item := range items {
		switch {
		case item.ellipsis:
			pagination.AddEllipsis()
		case item.number > 0:
			pagination.AddPage(ids[index+1], strconv.Itoa(item.number), item.number == page)
		}
	}
	pagination.AddNext(ids[paginationSlots+1], "下一页", page >= pages)

	if id, ok := d.Canvas.ClickTarget(); ok {
		if target, ok := paginationTarget(id, ids, items, page, pages); ok {
			d.Canvas.AfterPaint(func() {
				if spec.OnPage != nil {
					spec.OnPage(target)
				}
			})
		}
	}
}

// paginationTarget maps a pagination hit to the page it asks for.
func paginationTarget(id gfx.ID, ids [paginationSlots + 2]gfx.ID,
	items [paginationSlots]paginationItem, page, pages int) (int, bool) {
	switch id {
	case ids[0]:
		return max(page-1, 1), page > 1
	case ids[paginationSlots+1]:
		return min(page+1, pages), page < pages
	}
	for index, item := range items {
		if item.number > 0 && ids[index+1] == id {
			return item.number, item.number != page
		}
	}
	return page, false
}
