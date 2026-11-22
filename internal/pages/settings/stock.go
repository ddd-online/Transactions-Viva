package settings

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"transactions/internal/components/layout"
	"transactions/internal/components/typography"
	"transactions/internal/domain"
	"transactions/internal/pages/settings/controls"
)

// StockConfig is the active stock account's settings as the shell reads them
// each paint. The page keeps no repository: every command is a spec callback.
type StockConfig struct {
	Account    domain.StockAccount
	Tags       []string
	Review     domain.StockReviewSetting
	Fee        domain.StockFeeSetting
	Operations []domain.StockOperation
}

// stockState is the section's drafts. Data itself comes from Spec.Stock, so a
// save is visible on the next paint without a second copy.
type stockState struct {
	newTag string

	templateOpen  bool
	templateDraft string
	templateError string

	dailyOpen  bool
	dailyDraft string
	dailyError string

	feeOpen  bool
	feeForm  stockFeeForm
	feeError string

	operationsOpen bool

	archiveOpen  bool
	archiveName  string
	archiveError string

	resetOpen bool
}

type stockFeeForm struct {
	commission    string
	minCommission string
	stamp         string
	transfer      string
}

func stockFeeFormOf(fee domain.StockFeeSetting) stockFeeForm {
	return stockFeeForm{
		commission:    strconv.FormatFloat(fee.CommissionRate*10000, 'f', -1, 64),
		minCommission: domain.CentsToYuan(fee.MinCommission),
		stamp:         strconv.FormatFloat(fee.StampDutyRate*100, 'f', -1, 64),
		transfer:      strconv.FormatFloat(fee.TransferFeeRate*100, 'f', -1, 64),
	}
}

// stockSettingsSection is 设置 · 股票设置: the stock tab's old 设置 screen,
// moved into the settings page and laid out as groups of settings rows.
func (m *Model) stockSettingsSection(c ui.C, spec Spec) {
	groups := []controls.GroupSpec{{
		Title: "账户",
		Body:  func() { m.stockAccountRows(c, spec) },
	}}
	if spec.Stock == nil {
		if spec.StockError != "" {
			groups = append(groups, controls.GroupSpec{Raw: true, Body: func() {
				typography.Caption(c, spec.StockError).Color(c.Theme().Danger)
			}})
		}
		controls.Page(c, "股票设置", groups...)
		return
	}
	stock := spec.Stock
	groups = append(groups,
		controls.GroupSpec{Title: "交易与复盘", Body: func() {
			m.stockTradeSettings(c, spec, stock)
		}},
		controls.GroupSpec{Title: "数据管理", Body: func() {
			m.stockDataSettings(c, spec, stock)
		}},
	)
	controls.Page(c, "股票设置", groups...)
}

func (m *Model) stockTradeSettings(c ui.C, spec Spec, stock *StockConfig) {
	m.stockTagsRow(c, spec, stock)
	ui.Divider(c)
	controls.Row(c, "本轮复盘模板", "最多 200 字；留空使用内置模板。", func() {
		if ui.Button(c, "修改").Secondary().Clicked() {
			m.openStockTemplate(stock, false)
		}
	})
	ui.Divider(c)
	controls.Row(c, "每日复盘模板", "最多 200 字；留空使用内置模板。", func() {
		if ui.Button(c, "修改").Secondary().Clicked() {
			m.openStockTemplate(stock, true)
		}
	})
	ui.Divider(c)
	controls.Row(c, "交易费用", stockFeeSummary(stock.Fee), func() {
		if ui.Button(c, "修改").Secondary().Clicked() {
			m.stock.feeForm = stockFeeFormOf(stock.Fee)
			m.stock.feeError = ""
			m.stock.feeOpen = true
		}
	})
}

func (m *Model) stockDataSettings(c ui.C, spec Spec, stock *StockConfig) {
	controls.Row(c, "最近操作",
		"追加本金、支取、利息归本和每笔委托各记一条，保留最近 10 条；回滚会撤销最近一次操作。",
		func() {
			if ui.Button(c, "查看记录").Secondary().Clicked() {
				m.stock.operationsOpen = true
			}
			rollback := ui.Button(c, "回滚").Secondary().Disabled(len(stock.Operations) == 0)
			if len(stock.Operations) == 0 {
				rollback = rollback.Tooltip("暂无可回滚的操作")
			}
			if rollback.Clicked() {
				m.rollbackStock(spec)
			}
		})
	ui.Divider(c)
	controls.Row(c, "归档到新账户",
		"把当前账户的本金、资金记录和全部股票数据迁到新账户；原账户只保留标签、模板和费用设置。归档前请先清空全部持仓。",
		func() {
			if ui.Button(c, "归档到新账户").Secondary().Clicked() {
				m.stock.archiveOpen, m.stock.archiveError, m.stock.archiveName = true, "", ""
			}
		})
	ui.Divider(c)
	controls.Row(c, "重置股票数据",
		"清空当前账户的全部股票数据，包括本金、资金、持仓、成交、轮次、复盘、心得、标签、模板与费用设置。此操作不可恢复。",
		func() {
			if ui.Button(c, "重置股票数据").Destructive().Clicked() {
				m.stock.resetOpen = true
			}
		})
}

const (
	stockTagLimit    = 20
	stockTagMaxChars = 8
)

func (m *Model) stockTagsRow(c ui.C, spec Spec, stock *StockConfig) {
	theme := c.Theme()
	metrics := theme.Metrics
	atLimit := len(stock.Tags) >= stockTagLimit
	ui.Column(c).FillWidth().Shrink(0).
		PadX(metrics.PadSurface).PadY(metrics.GapItem * 2).
		Gap(metrics.GapItem).Children(func() {
		ui.Row(c).FillWidth().Items(ui.Center).Gap(metrics.GapItem).Children(func() {
			typography.Body(c, "交易标签").Grow(1)
			typography.Caption(c, fmt.Sprintf("%d / %d", len(stock.Tags), stockTagLimit)).
				Color(theme.MutedForeground)
		})
		typography.CaptionParagraph(c,
			fmt.Sprintf("最多 %d 个，每个最多 %d 个字；删除标签不会改动历史成交。", stockTagLimit, stockTagMaxChars), 2)
		if len(stock.Tags) > 0 {
			ui.Flow(c).Gap(6).LineGap(6).Children(func() {
				for _, tag := range stock.Tags {
					tag := tag
					ui.Row(c.Key("stock-tag-" + tag)).PadX(8).PadY(2).Radius(5.5).
						Background(theme.Muted).Items(ui.Center).Gap(4).Children(func() {
						ui.Text(c, tag).FontSize(typography.SizeCaption).Color(theme.MutedForeground)
						if tag == domain.DefaultStockTradeTag {
							return
						}
						if ui.Button(c, "删除标签 "+tag).Ghost().Icon(stockCloseIcon).
							IconOnly().Size(18, 18).Tooltip("删除标签「" + tag + "」").Clicked() {
							m.saveStockTags(spec, withoutTag(stock.Tags, tag))
						}
					})
				}
			})
		}
		ui.Row(c).FillWidth().Items(ui.Center).Gap(metrics.GapItem).Children(func() {
			field := ui.TextField(c, &m.stock.newTag, "新增交易标签", "如：低吸").
				Grow(1).MaxWidth(360).Disabled(atLimit)
			if field.Submitted() {
				m.addStockTag(spec, stock)
			}
			add := ui.Button(c, "添加标签").Secondary().Disabled(atLimit)
			if atLimit {
				add = add.Tooltip("最多 20 个标签")
			}
			if add.Clicked() {
				m.addStockTag(spec, stock)
			}
		})
	})
}

func (m *Model) addStockTag(spec Spec, stock *StockConfig) {
	tag := strings.TrimSpace(m.stock.newTag)
	if tag == "" {
		return
	}
	next := append(append([]string(nil), stock.Tags...), tag)
	m.saveStockTags(spec, next)
}

func (m *Model) saveStockTags(spec Spec, tags []string) {
	if spec.OnSaveStockTags == nil {
		return
	}
	saved, err := spec.OnSaveStockTags(tags)
	if err != nil {
		m.toaster.Show(stockConfigError(err), "", ui.ToastError)
		return
	}
	m.stock.newTag = ""
	_ = saved
}

func withoutTag(tags []string, remove string) []string {
	next := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag != remove {
			next = append(next, tag)
		}
	}
	return next
}

func (m *Model) openStockTemplate(stock *StockConfig, daily bool) {
	if daily {
		m.stock.dailyDraft, m.stock.dailyError, m.stock.dailyOpen = stock.Review.DailyTemplate, "", true
		return
	}
	m.stock.templateDraft, m.stock.templateError, m.stock.templateOpen = stock.Review.Template, "", true
}

func (m *Model) stockTemplateDialog(c ui.C, spec Spec) {
	m.stockTemplateDialogFor(c, spec, false)
	m.stockTemplateDialogFor(c, spec, true)
}

func (m *Model) stockTemplateDialogFor(c ui.C, spec Spec, daily bool) {
	open, draft, errText := &m.stock.templateOpen, &m.stock.templateDraft, &m.stock.templateError
	title, height := "本轮复盘模板", float32(180)
	if daily {
		open, draft, errText = &m.stock.dailyOpen, &m.stock.dailyDraft, &m.stock.dailyError
		title, height = "每日复盘模板", 140
	}
	if !*open {
		return
	}
	ui.ModalWith(c, open, title, ui.ModalSpec{Width: 720}, func() {
		ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
			ui.TextArea(c, draft, title, "留空使用内置模板").Height(height)
			ui.Row(c).Gap(layout.Gap).Items(ui.Center).Children(func() {
				count := utf8.RuneCountInString(*draft)
				counter := typography.Caption(c, fmt.Sprintf("%d / %d 字", count, domain.StockReviewTemplateMaxChars))
				if count > domain.StockReviewTemplateMaxChars {
					counter.Color(c.Theme().Danger)
				}
				ui.Spacer(c)
				if *errText != "" {
					typography.Caption(c, *errText).Color(c.Theme().Danger)
				}
			})
		})
	}, func() {
		ui.Row(c).FillWidth().Gap(layout.Gap).Items(ui.Center).Children(func() {
			if ui.Button(c, "恢复默认").Secondary().Clicked() {
				m.resetStockTemplate(spec, daily)
			}
			ui.Spacer(c)
			if ui.Button(c, "取消").Outline().Clicked() {
				*open = false
			}
			if ui.Button(c, "保存").Clicked() {
				m.saveStockTemplate(spec, daily)
			}
		})
	})
}

func (m *Model) saveStockTemplate(spec Spec, daily bool) {
	if spec.OnSaveStockReviewTemplate == nil {
		return
	}
	var dailyTemplate *string
	if daily {
		dailyTemplate = &m.stock.dailyDraft
	}
	saved, err := spec.OnSaveStockReviewTemplate(m.stock.templateDraft, dailyTemplate)
	if err != nil {
		m.setStockTemplateError(daily, stockConfigError(err))
		return
	}
	m.stock.templateDraft, m.stock.dailyDraft = saved.Template, saved.DailyTemplate
	if daily {
		m.stock.dailyOpen = false
		m.toaster.Show("每日复盘模板已保存", "", ui.ToastSuccess)
		return
	}
	m.stock.templateOpen = false
	m.toaster.Show("本轮复盘模板已保存", "", ui.ToastSuccess)
}

func (m *Model) resetStockTemplate(spec Spec, daily bool) {
	if spec.OnSaveStockReviewTemplate == nil {
		return
	}
	template := m.stock.templateDraft
	var dailyTemplate *string
	if daily {
		empty := ""
		dailyTemplate = &empty
	} else {
		template = ""
	}
	saved, err := spec.OnSaveStockReviewTemplate(template, dailyTemplate)
	if err != nil {
		m.setStockTemplateError(daily, stockConfigError(err))
		return
	}
	m.stock.templateDraft, m.stock.dailyDraft = saved.Template, saved.DailyTemplate
	m.setStockTemplateError(daily, "")
}

func (m *Model) setStockTemplateError(daily bool, message string) {
	if daily {
		m.stock.dailyError = message
		return
	}
	m.stock.templateError = message
}

func (m *Model) stockFeeDialog(c ui.C, spec Spec) {
	if !m.stock.feeOpen {
		return
	}
	ui.ModalWith(c, &m.stock.feeOpen, "交易费用", ui.ModalSpec{Width: 640}, func() {
		ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
			ui.Grid(c, 250).FillWidth().Gap(layout.BlockGap).Children(func() {
				stockFeeField(c, "佣金费率", "万分之", &m.stock.feeForm.commission, "2.354")
				stockFeeField(c, "最低佣金", "元", &m.stock.feeForm.minCommission, "5.00")
				stockFeeField(c, "印花税", "%", &m.stock.feeForm.stamp, "0.05")
				stockFeeField(c, "过户费", "%", &m.stock.feeForm.transfer, "0.001")
			})
			typography.CaptionParagraph(c, "建仓时按这里的值预填；建仓后本轮费率固定，修改只影响之后新建的轮次。", 2)
			if m.stock.feeError != "" {
				typography.Caption(c, m.stock.feeError).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.stock.feeOpen = false
		}
		if ui.Button(c, "保存").Clicked() {
			m.saveStockFee(spec)
		}
	})
}

func stockFeeField(c ui.C, label, unit string, value *string, placeholder string) {
	ui.Column(c).Gap(4).Children(func() {
		typography.Caption(c, label)
		ui.Row(c).Gap(layout.Gap).Items(ui.Center).Children(func() {
			ui.TextField(c, value, label, placeholder).Grow(1).Mono()
			typography.Caption(c, unit)
		})
	})
}

func (m *Model) saveStockFee(spec Spec) {
	if spec.OnSaveStockFee == nil {
		return
	}
	commission, err := parseStockFeeNumber(m.stock.feeForm.commission)
	if err != nil {
		m.stock.feeError = "佣金费率请填数字，例如 2.354"
		return
	}
	stamp, err := parseStockFeeNumber(m.stock.feeForm.stamp)
	if err != nil {
		m.stock.feeError = "印花税请填数字，例如 0.05"
		return
	}
	transfer, err := parseStockFeeNumber(m.stock.feeForm.transfer)
	if err != nil {
		m.stock.feeError = "过户费请填数字，例如 0.001"
		return
	}
	minCommission, err := domain.YuanToCents(m.stock.feeForm.minCommission)
	if err != nil {
		m.stock.feeError = "最低佣金请填金额，例如 5.00"
		return
	}
	fee := domain.RoundFee{
		CommissionRate: commission / 10000, MinCommission: minCommission,
		StampDutyRate: stamp / 100, TransferFeeRate: transfer / 100,
	}
	if err := domain.ValidateRoundFee(fee); err != nil {
		m.stock.feeError = err.Error()
		return
	}
	if err := spec.OnSaveStockFee(fee); err != nil {
		m.stock.feeError = stockConfigError(err)
		return
	}
	m.stock.feeError = ""
	m.stock.feeOpen = false
	m.toaster.Show("交易费用已保存", "", ui.ToastSuccess)
}

func parseStockFeeNumber(value string) (float64, error) {
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("无效的费率")
	}
	return number, nil
}

func (m *Model) stockOperationsDialog(c ui.C, spec Spec) {
	if !m.stock.operationsOpen || spec.Stock == nil {
		return
	}
	ui.ModalWith(c, &m.stock.operationsOpen, "操作记录", ui.ModalSpec{Width: 640}, func() {
		if len(spec.Stock.Operations) == 0 {
			typography.Caption(c, "还没有可回滚的操作").PadY(8)
			return
		}
		ui.Column(c).FillWidth().Children(func() {
			for index, operation := range spec.Stock.Operations {
				operation := operation
				if index > 0 {
					ui.Divider(c)
				}
				ui.Row(c.Key("stock-operation-" + operation.ID)).FillWidth().
					MinHeight(40).PadY(8).Gap(layout.BlockGap).Items(ui.Center).Children(func() {
					typography.Body(c, operation.Action).Shrink(0)
					typography.Caption(c, operation.Detail).Grow(1).TextEnd().Ellipsis()
				})
			}
		})
	}, nil)
}

func (m *Model) rollbackStock(spec Spec) {
	if spec.OnRollbackStockOperation == nil {
		return
	}
	outcome, err := spec.OnRollbackStockOperation()
	if err != nil {
		m.toaster.Show(stockConfigError(err), "", ui.ToastError)
		return
	}
	if outcome.Skipped {
		m.toaster.Show("目标已不存在，已移除："+outcome.Action, "", ui.ToastSuccess)
	} else {
		m.toaster.Show("已回滚："+outcome.Action, "", ui.ToastSuccess)
	}
}

func (m *Model) stockArchiveDialog(c ui.C, spec Spec) {
	if !m.stock.archiveOpen {
		return
	}
	ui.ModalWith(c, &m.stock.archiveOpen, "归档到新账户", ui.ModalSpec{Width: 420}, func() {
		ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				typography.Caption(c, "新账户名称")
				field := ui.TextField(c, &m.stock.archiveName, "新账户名称", "例如：2025 年账户").AutoFocus()
				if field.Submitted() {
					m.submitStockArchive(spec)
				}
			})
			if m.stock.archiveError != "" {
				typography.Caption(c, m.stock.archiveError).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.stock.archiveOpen = false
		}
		if ui.Button(c, "归档到新账户").Clicked() {
			m.submitStockArchive(spec)
		}
	})
}

func (m *Model) submitStockArchive(spec Spec) {
	name := strings.TrimSpace(m.stock.archiveName)
	if name == "" {
		m.stock.archiveError = "新账户名称不能为空"
		return
	}
	if spec.OnArchiveStockAccount == nil {
		return
	}
	if err := spec.OnArchiveStockAccount(name); err != nil {
		m.stock.archiveError = stockConfigError(err)
		return
	}
	m.stock.archiveOpen = false
	m.toaster.Show("股票数据已归档", "", ui.ToastSuccess)
}

func (m *Model) stockResetDialog(c ui.C, spec Spec) {
	if !m.stock.resetOpen {
		return
	}
	ui.ModalWith(c, &m.stock.resetOpen, "重置股票数据", ui.ModalSpec{
		Width:       420,
		Description: "清空当前账户的全部股票数据（本金、资金、持仓、成交、轮次、复盘、心得与设置），且不可恢复。",
	}, nil, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			m.stock.resetOpen = false
		}
		if ui.Button(c, "重置股票数据").Destructive().Clicked() {
			if spec.OnResetStockAccount == nil {
				return
			}
			if err := spec.OnResetStockAccount(); err != nil {
				m.toaster.Show(stockConfigError(err), "", ui.ToastError)
				return
			}
			m.stock.resetOpen = false
			m.toaster.Show("股票数据已重置", "", ui.ToastSuccess)
		}
	})
}

func stockFeeSummary(fee domain.StockFeeSetting) string {
	return fmt.Sprintf("佣金 万分之 %s · 最低 ¥%s · 印花税 %s%% · 过户费 %s%%",
		trimNumber(fee.CommissionRate*10000), domain.CentsToYuan(fee.MinCommission),
		trimNumber(fee.StampDutyRate*100), trimNumber(fee.TransferFeeRate*100))
}

func trimNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func stockConfigError(err error) string {
	message := err.Error()
	return strings.TrimPrefix(message, "无效的数据：")
}

// stockCloseIcon is the chip's ×. The shared icon set has no close glyph.
var stockCloseIcon = gfx.Icon{
	ViewBox: 24,
	Stroke:  2,
	Cap:     gfx.CapRound,
	Join:    gfx.JoinRound,
	Commands: []gfx.PathCommand{
		gfx.MoveTo(6, 6),
		gfx.LineTo(18, 18),
		gfx.MoveTo(18, 6),
		gfx.LineTo(6, 18),
	},
}
