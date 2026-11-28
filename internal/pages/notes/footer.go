package notes

import (
	"strconv"
	"unicode/utf8"

	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
	"transactions/internal/domain"
)

// footer is the note's status bar: the draft's state on the left and how much the
// note holds on the right, at the quiet foot of the window. It is chrome like every
// page footer — the caption step, the mono grid, one row — so it reads as the page's,
// not as the document's. It draws no rule above itself: the writing surface runs into
// it, and the numbers sit far enough below the last line to stay out of the way.
//
// The three figures are the note's own, not the selection's: 字符数 counts every
// character the blocks carry (spaces and punctuation included, the line breaks
// between blocks left out), 字数 counts words the way a person writing Chinese
// means them (see domain.NoteWordCount), and 图片数 counts the picture blocks.
func (m *Model) footer(c ui.C) {
	theme := c.Theme()
	if m.countsDirty {
		m.countNote()
	}
	layout.Footer(c, false, func() {
		m.saveState(c)
		ui.Spacer(c)
		layout.Reading(c, "字符数", strconv.Itoa(m.chars), "", theme.Foreground)
		layout.Reading(c, "字数", strconv.Itoa(m.words), "", theme.Foreground)
		layout.Reading(c, "图片数", strconv.Itoa(m.images), "", theme.Foreground)
	})
}

// countNote reads the note's size from the lines the editor holds and the blocks
// beside them, and marks the reading clean. It runs when a paint sees a change, not
// on every paint: a note is walked once per write, not once per frame.
func (m *Model) countNote() {
	m.chars, m.words, m.images = 0, 0, 0
	for _, line := range m.lines {
		m.chars += utf8.RuneCountInString(line)
		m.words += domain.NoteWordCount(line)
	}
	for _, block := range m.blocks {
		if block.Kind == domain.NoteBlockImage {
			m.images++
		}
	}
	m.countsDirty = false
}
