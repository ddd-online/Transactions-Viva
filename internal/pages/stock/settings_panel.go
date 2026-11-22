package stock

import (
	"context"
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
	"transactions/internal/repository"
)

// settingsPanel is the 设置 screen: the account's trade tags, review templates,
// fee setting, rollback log and the archive / reset actions. It is account
// scoped like every other stock tab; the account selector in the toolbar is the
// shared one, so switching here switches every tab.
type settingsPanel struct {
	accounts *accountStore
	repo     repository.StockRepository
	toaster  *ui.Toaster

	// onChange reloads the other stock tabs; onAccountsChanged refreshes the
	// shell's account list after an archive.
	onChange          func()
	onAccountsChanged func()

	errorMessage string

	tags       []string
	review     domain.StockReviewSetting
	fee        domain.StockFeeSetting
	operations []domain.StockOperation

	newTag string

	templateOpen  bool
	templateDraft string
	templateError string

	dailyOpen  bool
	dailyDraft string
	dailyError string

	feeOpen  bool
	feeForm  settingsFeeForm
	feeError string

	operationsOpen bool

	archiveOpen  bool
	archiveName  string
	archiveError string

	resetOpen bool
}

// settingsFeeForm is the dialog's draft: the commission in 万分之, the minimum
// commission in yuan, and the two rates in percent, exactly as the old screen
// showed them.
type settingsFeeForm struct {
	commission    string
	minCommission string
	stamp         string
	transfer      string
}

func newSettingsPanel(accounts *accountStore) *settingsPanel {
	p := &settingsPanel{accounts: accounts}
	p.fee = domain.DefaultStockFeeSetting()
	p.feeForm = settingsFeeFormOf(p.fee)
	return p
}

// setRepository connects the panel to the workspace. An empty account list is
// not an error: View draws the empty state the account panel uses.
func (p *settingsPanel) setRepository(repo repository.StockRepository) {
	p.repo = repo
	p.Reload()
}

func (p *settingsPanel) active() string {
	if p.accounts == nil {
		return ""
	}
	return p.accounts.activeID()
}

// Reload reads everything the screen shows for the active account. A failed
// read leaves the previous values alone and shows the reason above the card, as
// the account screen does.
func (p *settingsPanel) Reload() {
	p.errorMessage = ""
	if p.repo == nil || p.active() == "" {
		p.tags = nil
		p.review = domain.StockReviewSetting{}
		p.fee = domain.DefaultStockFeeSetting()
		p.feeForm = settingsFeeFormOf(p.fee)
		p.operations = nil
		return
	}
	ctx := context.Background()
	tags, err := p.repo.StockTradeTags(ctx, p.active())
	if err != nil {
		p.errorMessage = "读取交易标签失败：" + settingsErrorText(err)
		return
	}
	review, err := p.repo.StockReviewTemplates(ctx, p.active())
	if err != nil {
		p.errorMessage = "读取复盘模板失败：" + settingsErrorText(err)
		return
	}
	fee, err := p.repo.StockFeeSetting(ctx, p.active())
	if err != nil {
		p.errorMessage = "读取交易费用设置失败：" + settingsErrorText(err)
		return
	}
	operations, err := p.repo.StockOperations(ctx, p.active())
	if err != nil {
		p.errorMessage = "读取操作记录失败：" + settingsErrorText(err)
		return
	}
	if len(tags) == 0 {
		tags = append([]string(nil), domain.DefaultStockTradeTags[:]...)
	}
	p.tags = tags
	p.review = review
	p.fee = fee
	p.feeForm = settingsFeeFormOf(fee)
	p.operations = operations
}

// CloseOverlays closes every dialog of the screen.
func (p *settingsPanel) CloseOverlays() {
	p.templateOpen = false
	p.dailyOpen = false
	p.feeOpen = false
	p.operationsOpen = false
	p.archiveOpen = false
	p.resetOpen = false
}

// View declares the screen: the shared account selector in the toolbar, then
// one card of settings rows that scrolls under it.
func (p *settingsPanel) View(c ui.C) {
	if p.repo == nil || p.active() == "" {
		emptyAccountPage(c, "还没有股票账户", "工作空间里至少会有一个股票账户。")
		return
	}
	ui.Column(c).Fill().Children(func() {
		p.toolbar(c)
		if p.errorMessage != "" {
			typography.Caption(c, p.errorMessage).Color(c.Theme().Danger).
				PadX(layout.Inset).PadY(6)
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Pad(layout.Inset).Children(func() {
				p.card(c)
			})
		})
	})
	p.dialogs(c)
}

func (p *settingsPanel) toolbar(c ui.C) {
	ui.Row(c).Height(layout.ToolbarHeight).FillWidth().PadX(layout.Inset).
		Gap(layout.Gap).Items(ui.Center).Children(func() {
		if p.accounts == nil || !p.accounts.picker(c) {
			return
		}
		p.CloseOverlays()
		if p.onChange != nil {
			p.onChange()
		} else {
			p.Reload()
		}
	})
}

// card is the one settings card: rows separated by hairlines, as the reference
// application lays them out.
func (p *settingsPanel) card(c ui.C) {
	theme := c.Theme()
	ui.Column(c).FillWidth().Background(theme.Card).
		Radius(theme.Metrics.RadiusSurface).Border(1, theme.Border).Children(func() {
		p.tagsRow(c)
		ui.Divider(c)
		p.templateRow(c, false)
		ui.Divider(c)
		p.templateRow(c, true)
		ui.Divider(c)
		p.feeRow(c)
		ui.Divider(c)
		p.operationsRow(c)
		ui.Divider(c)
		p.archiveRow(c)
		ui.Divider(c)
		p.resetRow(c)
	})
}

// settingsRow is one row of the settings card: the name and its explanation on
// the left, an optional body under them, and the controls on the right. The row
// is at least 64 px tall and keeps the page's 16 px inset on every side.
func settingsRow(c ui.C, title, description string, body, control func()) {
	ui.Row(c).MinHeight(64).FillWidth().Pad(layout.Inset).
		Gap(layout.Inset).Items(ui.Center).Children(func() {
		ui.Column(c).Grow(1).Gap(4).Children(func() {
			typography.Body(c, title).FillWidth().Ellipsis()
			if description != "" {
				typography.CaptionParagraph(c, description, 2)
			}
			if body != nil {
				body()
			}
		})
		if control != nil {
			ui.Row(c).Shrink(0).Items(ui.Center).Justify(ui.End).
				Gap(layout.Gap).Children(control)
		}
	})
}

func (p *settingsPanel) tagsRow(c ui.C) {
	theme := c.Theme()
	settingsRow(c, "交易标签", "最多 20 个、每个 8 字；删除不影响历史记录。", func() {
		if len(p.tags) == 0 {
			return
		}
		ui.Flow(c).Gap(4).LineGap(4).Children(func() {
			for _, tag := range p.tags {
				tag := tag
				ui.Row(c.Key("stock-tag-" + tag)).PadX(8).PadY(2).Radius(5.5).
					Background(theme.Muted).Items(ui.Center).Gap(4).Children(func() {
					ui.Text(c, tag).FontSize(typography.SizeCaption).
						Color(theme.MutedForeground)
					if tag == domain.DefaultStockTradeTag {
						return
					}
					if ui.Button(c, "删除标签 "+tag).Ghost().Icon(settingsCloseIcon).
						IconOnly().Size(18, 18).Tooltip("删除标签「" + tag + "」").Clicked() {
						p.removeTag(tag)
					}
				})
			}
		})
	}, func() {
		field := ui.TextField(c, &p.newTag, "新增交易标签", "如：低吸").Width(220)
		if field.Submitted() {
			p.addTag()
		}
		if ui.Button(c, "添加").Secondary().Clicked() {
			p.addTag()
		}
	})
}

func (p *settingsPanel) addTag() {
	tag := strings.TrimSpace(p.newTag)
	if tag == "" {
		return
	}
	next := make([]string, 0, len(p.tags)+1)
	next = append(next, p.tags...)
	next = append(next, tag)
	p.saveTags(next)
}

func (p *settingsPanel) removeTag(tag string) {
	next := make([]string, 0, len(p.tags))
	for _, one := range p.tags {
		if one != tag {
			next = append(next, one)
		}
	}
	p.saveTags(next)
}

// saveTags writes the list through the repository and keeps the normalized
// result it returns. A refusal keeps the list as it was and shows the reason,
// the way the old screen did.
func (p *settingsPanel) saveTags(tags []string) {
	if p.repo == nil {
		p.showToast("工作空间尚未打开", ui.ToastError)
		return
	}
	saved, err := p.repo.SaveStockTradeTags(context.Background(), p.active(), tags)
	if err != nil {
		p.showToast(settingsErrorText(err), ui.ToastError)
		return
	}
	p.tags = saved
	p.newTag = ""
}

func (p *settingsPanel) templateRow(c ui.C, daily bool) {
	title, description := "本轮复盘模板", "最多 200 字；留空用内置模板。"
	if daily {
		title = "每日复盘模板"
	}
	settingsRow(c, title, description, nil, func() {
		if ui.Button(c, "修改").Secondary().Clicked() {
			p.openTemplate(daily)
		}
	})
}

func (p *settingsPanel) openTemplate(daily bool) {
	if daily {
		p.dailyDraft = p.review.DailyTemplate
		p.dailyError = ""
		p.dailyOpen = true
		return
	}
	p.templateDraft = p.review.Template
	p.templateError = ""
	p.templateOpen = true
}

func (p *settingsPanel) feeRow(c ui.C) {
	settingsRow(c, "交易费用设置", feeSummary(p.fee), nil, func() {
		if ui.Button(c, "修改").Secondary().Clicked() {
			p.openFee()
		}
	})
}

func (p *settingsPanel) openFee() {
	p.feeForm = settingsFeeFormOf(p.fee)
	p.feeError = ""
	p.feeOpen = true
}

func (p *settingsPanel) operationsRow(c ui.C) {
	settingsRow(c, "操作记录",
		"追加本金 / 支取 / 利息归本 / 建仓 / 加仓 / 减仓 / 清仓 各记一条，最多保留最近 10 条；回滚撤销最近一次操作。",
		nil, func() {
			if ui.Button(c, "查看记录").Secondary().Clicked() {
				p.operationsOpen = true
			}
			if ui.Button(c, "回滚").Secondary().Disabled(len(p.operations) == 0).Clicked() {
				p.rollback()
			}
		})
}

func (p *settingsPanel) archiveRow(c ui.C) {
	settingsRow(c, "归档",
		"把当前账户的本金、资金记录、交易与轮次/复盘迁到一个新账户；原账户只保留交易标签、复盘模板与费用设置。需先清空全部持仓。",
		nil, func() {
			if ui.Button(c, "归档").Secondary().Clicked() {
				p.archiveOpen = true
				p.archiveError = ""
			}
		})
}

func (p *settingsPanel) resetRow(c ui.C) {
	settingsRow(c, "重置",
		"清空当前账户的股票数据（账户本金、持仓、交易记录、资金记录、费用设置、交易标签与复盘模板），此操作不可恢复。",
		nil, func() {
			if ui.Button(c, "重置").Destructive().Clicked() {
				p.resetOpen = true
			}
		})
}

// dialogs declares every floating surface of the screen. Each one is a no-op
// while closed, so the page pays only the open dialog's layout.
func (p *settingsPanel) dialogs(c ui.C) {
	p.templateDialog(c, false)
	p.templateDialog(c, true)
	p.feeDialog(c)
	p.operationsDialog(c)
	p.archiveDialog(c)
	p.resetDialog(c)
}

func (p *settingsPanel) templateDialog(c ui.C, daily bool) {
	open, draft, errText := &p.templateOpen, &p.templateDraft, &p.templateError
	title, height := "本轮复盘模板", float32(180)
	if daily {
		open, draft, errText = &p.dailyOpen, &p.dailyDraft, &p.dailyError
		title, height = "每日复盘模板", 140
	}
	if !*open {
		return
	}
	ui.ModalWith(c, open, title, ui.ModalSpec{Width: 720}, func() {
		ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
			ui.TextArea(c, draft, title, "留空使用内置模板").Height(height)
			ui.Row(c).Gap(layout.Gap).Items(ui.Center).Children(func() {
				typography.Caption(c, fmt.Sprintf("%d / %d 字",
					utf8.RuneCountInString(*draft), domain.StockReviewTemplateMaxChars))
				ui.Spacer(c)
				if *errText != "" {
					typography.Caption(c, *errText).Color(c.Theme().Danger)
				}
			})
		})
	}, func() {
		ui.Row(c).FillWidth().Gap(layout.Gap).Items(ui.Center).Children(func() {
			if ui.Button(c, "恢复默认").Secondary().Clicked() {
				p.resetTemplate(daily)
			}
			ui.Spacer(c)
			if ui.Button(c, "取消").Outline().Clicked() {
				*open = false
			}
			if ui.Button(c, "保存").Clicked() {
				p.saveTemplate(daily)
			}
		})
	})
}

func (p *settingsPanel) saveTemplate(daily bool) {
	if p.repo == nil {
		p.showToast("工作空间尚未打开", ui.ToastError)
		return
	}
	var dailyTemplate *string
	if daily {
		dailyTemplate = &p.dailyDraft
	}
	saved, err := p.repo.SaveStockReviewTemplate(context.Background(), p.active(), p.templateDraft, dailyTemplate)
	if err != nil {
		p.setTemplateError(daily, settingsErrorText(err))
		return
	}
	p.review = saved
	p.templateDraft, p.dailyDraft = saved.Template, saved.DailyTemplate
	if daily {
		p.dailyOpen = false
		p.dailyError = ""
		p.showToast("每日复盘模板已保存", ui.ToastSuccess)
		return
	}
	p.templateOpen = false
	p.templateError = ""
	p.showToast("复盘模板已保存", ui.ToastSuccess)
}

// resetTemplate writes the empty string immediately, keeps the dialog open and
// fills the draft with the effective template the repository returns.
func (p *settingsPanel) resetTemplate(daily bool) {
	if p.repo == nil {
		p.showToast("工作空间尚未打开", ui.ToastError)
		return
	}
	template := p.templateDraft
	var dailyTemplate *string
	if daily {
		empty := ""
		dailyTemplate = &empty
	} else {
		template = ""
	}
	saved, err := p.repo.SaveStockReviewTemplate(context.Background(), p.active(), template, dailyTemplate)
	if err != nil {
		p.setTemplateError(daily, settingsErrorText(err))
		return
	}
	p.review = saved
	p.templateDraft, p.dailyDraft = saved.Template, saved.DailyTemplate
	p.setTemplateError(daily, "")
}

func (p *settingsPanel) setTemplateError(daily bool, message string) {
	if daily {
		p.dailyError = message
		return
	}
	p.templateError = message
}

func (p *settingsPanel) feeDialog(c ui.C) {
	if !p.feeOpen {
		return
	}
	ui.ModalWith(c, &p.feeOpen, "交易费用设置", ui.ModalSpec{Width: 640}, func() {
		ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
			ui.Grid(c, 250).FillWidth().Gap(layout.BlockGap).Children(func() {
				feeField(c, "佣金费率", "万分之", &p.feeForm.commission, "2.354")
				feeField(c, "最低佣金", "元", &p.feeForm.minCommission, "5.00")
				feeField(c, "印花税", "%", &p.feeForm.stamp, "0.05")
				feeField(c, "过户费", "%", &p.feeForm.transfer, "0.001")
			})
			typography.CaptionParagraph(c, "建仓时按这里的值预填；建仓之后本轮费率固定，改动只影响新轮次。", 2)
			if p.feeError != "" {
				typography.Caption(c, p.feeError).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			p.feeOpen = false
		}
		if ui.Button(c, "保存").Clicked() {
			p.saveFee()
		}
	})
}

func feeField(c ui.C, label, unit string, value *string, placeholder string) {
	ui.Column(c).Gap(4).Children(func() {
		typography.Caption(c, label)
		ui.Row(c).Gap(layout.Gap).Items(ui.Center).Children(func() {
			ui.TextField(c, value, label, placeholder).Grow(1).Mono()
			typography.Caption(c, unit)
		})
	})
}

func (p *settingsPanel) saveFee() {
	if p.repo == nil {
		p.showToast("工作空间尚未打开", ui.ToastError)
		return
	}
	commission, err := parseFeeNumber(p.feeForm.commission)
	if err != nil {
		p.feeError = "佣金费率必须是数字"
		return
	}
	stamp, err := parseFeeNumber(p.feeForm.stamp)
	if err != nil {
		p.feeError = "印花税必须是数字"
		return
	}
	transfer, err := parseFeeNumber(p.feeForm.transfer)
	if err != nil {
		p.feeError = "过户费必须是数字"
		return
	}
	minCommission, err := domain.YuanToCents(p.feeForm.minCommission)
	if err != nil {
		p.feeError = "最低佣金输入无效"
		return
	}
	fee := domain.RoundFee{
		CommissionRate:  commission / 10000,
		MinCommission:   minCommission,
		StampDutyRate:   stamp / 100,
		TransferFeeRate: transfer / 100,
	}
	if err := domain.ValidateRoundFee(fee); err != nil {
		p.feeError = err.Error()
		return
	}
	saved, err := p.repo.SaveStockFeeSetting(context.Background(), p.active(), fee)
	if err != nil {
		p.feeError = settingsErrorText(err)
		return
	}
	p.fee = saved
	p.feeForm = settingsFeeFormOf(saved)
	p.feeError = ""
	p.feeOpen = false
	p.showToast("交易费用已保存", ui.ToastSuccess)
}

func parseFeeNumber(value string) (float64, error) {
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("无效的费率")
	}
	return number, nil
}

func (p *settingsPanel) operationsDialog(c ui.C) {
	if !p.operationsOpen {
		return
	}
	ui.ModalWith(c, &p.operationsOpen, "操作记录", ui.ModalSpec{Width: 640}, func() {
		if len(p.operations) == 0 {
			typography.Caption(c, "还没有可回滚的操作").PadY(8)
			return
		}
		ui.Column(c).FillWidth().Children(func() {
			for index, operation := range p.operations {
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

func (p *settingsPanel) rollback() {
	if p.repo == nil || len(p.operations) == 0 {
		return
	}
	outcome, err := p.repo.RollbackStockOperation(context.Background(), p.active())
	if err != nil {
		p.showToast(settingsErrorText(err), ui.ToastError)
		return
	}
	if outcome.Skipped {
		p.showToast("目标已不存在，已移除："+outcome.Action, ui.ToastSuccess)
	} else {
		p.showToast("已回滚："+outcome.Action, ui.ToastSuccess)
	}
	if p.onChange != nil {
		p.onChange()
	} else {
		p.Reload()
	}
}

func (p *settingsPanel) archiveDialog(c ui.C) {
	if !p.archiveOpen {
		return
	}
	ui.ModalWith(c, &p.archiveOpen, "归档到新账户", ui.ModalSpec{Width: 420}, func() {
		ui.Column(c).FillWidth().Gap(layout.BlockGap).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				typography.Caption(c, "新账户名称")
				field := ui.TextField(c, &p.archiveName, "新账户名称", "例如：2025 年账户").AutoFocus()
				if field.Submitted() {
					p.submitArchive()
				}
			})
			if p.archiveError != "" {
				typography.Caption(c, p.archiveError).Color(c.Theme().Danger)
			}
		})
	}, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			p.archiveOpen = false
		}
		if ui.Button(c, "归档").Disabled(strings.TrimSpace(p.archiveName) == "").Clicked() {
			p.submitArchive()
		}
	})
}

func (p *settingsPanel) submitArchive() {
	name := strings.TrimSpace(p.archiveName)
	if name == "" {
		p.archiveError = "请输入账户名称"
		return
	}
	if p.repo == nil {
		p.showToast("工作空间尚未打开", ui.ToastError)
		return
	}
	created, err := p.repo.ArchiveStockAccount(context.Background(), p.active(), name)
	if err != nil {
		p.archiveError = settingsErrorText(err)
		return
	}
	p.archiveOpen = false
	p.archiveName = ""
	p.archiveError = ""
	p.showToast("股票数据已归档", ui.ToastSuccess)
	if p.onAccountsChanged != nil {
		p.onAccountsChanged()
	}
	// The shell's callback refreshes the list; select the archived account after
	// it so every tab follows the data that just moved.
	if p.accounts != nil {
		p.accounts.setActive(created.ID)
	}
	if p.onChange != nil {
		p.onChange()
	} else {
		p.Reload()
	}
}

func (p *settingsPanel) resetDialog(c ui.C) {
	if !p.resetOpen {
		return
	}
	ui.ModalWith(c, &p.resetOpen, "重置全部股票数据", ui.ModalSpec{
		Width:       420,
		Description: "将清空当前账户的股票数据，且不可恢复。",
	}, nil, func() {
		if ui.Button(c, "取消").Outline().Clicked() {
			p.resetOpen = false
		}
		if ui.Button(c, "重置").Destructive().Clicked() {
			p.submitReset()
		}
	})
}

func (p *settingsPanel) submitReset() {
	if p.repo == nil {
		p.showToast("工作空间尚未打开", ui.ToastError)
		return
	}
	if err := p.repo.ResetStockAccount(context.Background(), p.active()); err != nil {
		p.showToast(settingsErrorText(err), ui.ToastError)
		return
	}
	p.resetOpen = false
	p.showToast("股票数据已重置", ui.ToastSuccess)
	if p.onChange != nil {
		p.onChange()
	} else {
		p.Reload()
	}
}

func (p *settingsPanel) showToast(message string, kind ui.ToastKind) {
	if p.toaster == nil || message == "" {
		return
	}
	p.toaster.Show(message, "", kind)
}

// settingsErrorText drops the repository's "无效的数据：" classifier, so the
// toast says the rule ("标签不能重复") rather than repeating what kind of error
// it is.
func settingsErrorText(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	for _, classifier := range []error{repository.ErrInvalid, repository.ErrDuplicate, repository.ErrNotFound} {
		if stripped := strings.TrimPrefix(text, classifier.Error()+"："); stripped != text {
			return stripped
		}
	}
	return text
}

// feeSummary is the row's one line: commission in 万分之, minimum in yuan, the
// two percentage rates, separated by the same dot the old screen used.
func feeSummary(fee domain.StockFeeSetting) string {
	return fmt.Sprintf("佣金 万%s · 最低 ¥%s · 印花税 %s%% · 过户费 %s%%",
		formatRate(fee.CommissionRate*10000, 4),
		domain.CentsToYuan(fee.MinCommission),
		formatRate(fee.StampDutyRate*100, 4),
		formatRate(fee.TransferFeeRate*100, 4))
}

func settingsFeeFormOf(fee domain.StockFeeSetting) settingsFeeForm {
	return settingsFeeForm{
		commission:    formatRate(fee.CommissionRate*10000, 4),
		minCommission: domain.CentsToYuan(fee.MinCommission),
		stamp:         formatRate(fee.StampDutyRate*100, 4),
		transfer:      formatRate(fee.TransferFeeRate*100, 4),
	}
}

// formatRate prints at most decimals fractional digits without trailing zeros:
// 2.354 → "2.354", 0.001 → "0.001", 5 → "5".
func formatRate(value float64, decimals int) string {
	text := strconv.FormatFloat(value, 'f', decimals, 64)
	if strings.Contains(text, ".") {
		text = strings.TrimRight(text, "0")
		text = strings.TrimRight(text, ".")
	}
	if text == "" || text == "-0" {
		return "0"
	}
	return text
}

// settingsCloseIcon is the chip's ×. The shared icon set has no close glyph and
// this screen is the only place that needs one.
var settingsCloseIcon = gfx.Icon{
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
