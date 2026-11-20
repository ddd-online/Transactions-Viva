package records

import (
	"context"

	"github.com/vivaui/viva/ui"
)

// find returns the loaded row with id, or nil.
func (m *Model) find(id string) *record {
	for index := range m.records {
		if m.records[index].id == id {
			return &m.records[index]
		}
	}
	return nil
}

// deleteView declares the delete confirmation while one is pending.
func (m *Model) deleteView(c ui.C) {
	if m.deletingID == "" {
		return
	}
	ui.ModalWith(c, &m.deleteConfirm, "删除这条消费记录？", ui.ModalSpec{
		Width:       380,
		Description: "删除后不会保留。",
	}, func() {
		ui.Text(c, "这条记录会从当前账本移除。").FontSize(13).Muted()
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.deleteConfirm = false
		}
		if ui.Button(c, "删除").Destructive().Clicked() {
			m.deleteRecord(m.deletingID)
		}
	})
}

// deleteRecord removes one stored record and reports it in the footer notice.
func (m *Model) deleteRecord(id string) {
	m.deletingID, m.deleteConfirm = "", false
	if m.repo == nil {
		m.notice = "工作空间尚未打开"
		return
	}
	if err := m.repo.DeleteRecord(context.Background(), id); err != nil {
		m.notice = "删除失败：" + err.Error()
		return
	}
	m.Reload()
	m.notice = "记录已删除"
}

// syncTo copies the row to another ledger through the repository.
func (m *Model) syncTo(id, targetID string) {
	if m.repo == nil {
		m.notice = "工作空间尚未打开"
		return
	}
	if err := m.repo.SyncRecord(context.Background(), id, targetID); err != nil {
		m.notice = "同步失败：" + err.Error()
		return
	}
	m.notice = "已同步到「" + m.ledgers.Name(targetID) + "」"
}
