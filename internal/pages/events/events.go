// Package events is the 事件 section: a calendar of key events with a title,
// body, colour and photo assets. It is migrated from the old project's
// src/pages/events*, event-* and electron/services/key-event.
package events

import (
	"github.com/vivaui/viva/ui"
	"transactions/internal/components"
)

// Model is the 事件 page's state.
type Model struct{}

// New returns the page.
func New() *Model { return &Model{} }

// View declares the page.
func (m *Model) View(c ui.C) {
	components.Pending(c, "事件",
		"按日期的事件画廊、编辑、图片与实况照片。图片资产保存在工作空间 data/assets 下。")
}
