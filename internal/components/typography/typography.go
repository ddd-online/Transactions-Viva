// Package typography is the application's type scale.
//
// It answers two questions in one place — how big is this text, and how heavy —
// so the same job looks the same on every screen. A page picks a role; it does
// not pick a number.
//
// Seven roles share five steps, so a role is never only a size: Caption 12,
// Body 14, BodyStrong 14 bold, Section 16 bold, Figure 16 bold mono, Title 20
// bold, Metric 24 bold mono. Two roles may share a step — a heading and a
// figure do, because the money face already tells them apart — but no role has
// a step of its own, and nothing is written at 13, 15 or 19.
//
// The steps are the reference application's 11/12/13/14/16/18/20/24 ladder, cut
// down to the five this application uses: the reference's 12/13/14 sat one pixel
// apart, so caption and reading text were the only two jobs they could name here,
// and the dense template table reads at the body size like every other table. Its
// measurement of the platform faces holds here too: Chinese ships only Regular and
// Bold — 500 and 600 resolve to the same face as 400 and 700 — so a Chinese heading
// is Bold and the step from body to label comes from size and colour, never from a
// half weight. Medium (500) is kept for Latin, digits and the mono face, where a
// real face exists.
package typography

import "github.com/vivaui/viva/ui"

// The five steps. Gaps are at least 2 px so that two roles never read as one.
const (
	// SizeCaption is a label: a table header, a badge, a count, a hint, an
	// error under a field, a tag chip. Never the reading line.
	SizeCaption float32 = 12
	// SizeBody is the reading size: table cells, rail rows, settings rows,
	// form values, a dialog's body, an empty state's sentence.
	SizeBody float32 = 14
	// SizeSection names a region inside a page: a rail header, the summary
	// panel, an empty state's title. Bold, because weight is the only step
	// Chinese has left at this size.
	SizeSection float32 = 16
	// SizeTitle names the page itself or the chart being read.
	SizeTitle float32 = 20
	// SizeMetric is the one figure a panel leads with.
	SizeMetric float32 = 24
)

// Caption is a label. Muted: it is never the reading line.
func Caption(c ui.C, value string) ui.Box {
	return ui.Text(c, value).FontSize(SizeCaption).Color(c.Theme().MutedForeground)
}

// CaptionMoney is a label that carries an amount, such as a section's 费用合计
// or a note's 从高点回落. Mono, so the figure sits in the same face as the
// column it summarizes even though the line also holds Chinese.
func CaptionMoney(c ui.C, value string) ui.Box {
	return ui.Text(c, value).FontSize(SizeCaption).Mono().
		Color(c.Theme().MutedForeground)
}

// Body is the reading size.
func Body(c ui.C, value string) ui.Box {
	return ui.Text(c, value).FontSize(SizeBody)
}

// BodyStrong is the one strong line inside a block: a card's title, a group's
// name, a total at the end of a form. Same size as Body, one weight up — the
// hierarchy below Section, which names the block itself.
func BodyStrong(c ui.C, value string) ui.Box {
	return ui.Text(c, value).FontSize(SizeBody).Bold()
}

// Section names a region inside a page.
func Section(c ui.C, value string) ui.Box {
	return ui.Text(c, value).FontSize(SizeSection).Bold()
}

// Figure is a number a panel reads out: one cell of a metric grid, a ladder
// rung's level. The same step as Section, in the money face — a figure is data,
// never a heading, and the two never trade places.
func Figure(c ui.C, value string) ui.Box {
	return ui.Text(c, value).FontSize(SizeSection).Bold().Mono()
}

// Title names the page itself.
func Title(c ui.C, value string) ui.Box {
	return ui.Text(c, value).FontSize(SizeTitle).Bold()
}

// Metric is the one figure a panel leads with. It is money, so it is mono.
func Metric(c ui.C, value string) ui.Box {
	return ui.Text(c, value).FontSize(SizeMetric).Bold().Mono()
}

// CaptionParagraph is a caption that wraps, for a hint or an error that may be
// longer than its column.
func CaptionParagraph(c ui.C, value string, lines int) ui.Box {
	return ui.Paragraph(c, value, lines).FontSize(SizeCaption).
		Color(c.Theme().MutedForeground)
}

// BodyParagraph is reading prose that wraps: the explanation on a gate card, a
// settings section's lead.
func BodyParagraph(c ui.C, value string, lines int) ui.Box {
	return ui.Paragraph(c, value, lines).FontSize(SizeBody)
}
