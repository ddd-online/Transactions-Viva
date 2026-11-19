// Package diary is the 日记 section: one entry per local date and ledger, a
// date tree, moods and import/export. It is migrated from the old project's
// src/pages/diary* and electron/services/diary.
package diary

import (
	"github.com/vivaui/viva/ui"
	"transactions/internal/components"
)

// Model is the 日记 page's state.
type Model struct{}

// New returns the page.
func New() *Model { return &Model{} }

// View declares the page.
func (m *Model) View(c ui.C) {
	components.Pending(c, "日记",
		"按账本隔离的日记目录树、正文编辑、心情与导入 / 导出。")
}
