// Package storetest builds a real, temporary workspace for page and desktop
// tests. It is test infrastructure: production code never imports it.
package storetest

import (
	"context"
	"testing"
	"time"

	"transactions/internal/domain"
	"transactions/internal/store"
)

// Fixture is a temporary workspace with two ledgers, their default categories
// and tags, and deterministic records/templates the UI tests assert on.
type Fixture struct {
	Workspace *store.Workspace
	Home      domain.Ledger
	Travel    domain.Ledger
}

// Open creates a temporary workspace and seeds it. The workspace is closed by
// the test cleanup.
func Open(t testing.TB) *Fixture {
	t.Helper()
	workspace, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("storetest.Open: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Close() })
	fixture := &Fixture{Workspace: workspace}
	ctx := context.Background()
	fixture.Home, err = workspace.CreateLedger(ctx, "家庭账本", "")
	if err != nil {
		t.Fatalf("storetest: create home ledger: %v", err)
	}
	fixture.Travel, err = workspace.CreateLedger(ctx, "旅行账本", "")
	if err != nil {
		t.Fatalf("storetest: create travel ledger: %v", err)
	}
	seedRecords(t, ctx, fixture)
	seedTemplates(t, ctx, fixture)
	return fixture
}

func at(daysAgo, hour, minute int) time.Time {
	now := time.Now()
	day := now.AddDate(0, 0, -daysAgo)
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, time.Local)
}

func seedRecords(t testing.TB, ctx context.Context, fixture *Fixture) {
	t.Helper()
	today := time.Now().Format("2006-01-02")
	type row struct {
		id, ledger, kind, category, description string
		tags                                    []string
		amount                                  int64
		at                                      time.Time
		outlier                                 bool
		keyEventDate                            string
	}
	rows := []row{
		{"mock-1", fixture.Home.ID, "expense", "餐饮美食", "早餐 · 豆浆油条", []string{"日常"}, 1200, at(0, 8, 40), false, ""},
		{"mock-2", fixture.Home.ID, "expense", "餐饮美食", "午餐 · 公司楼下", []string{"日常", "同事"}, 3200, at(0, 12, 10), false, today},
		{"mock-3", fixture.Home.ID, "expense", "交通出行", "地铁通勤", []string{"通勤"}, 400, at(0, 18, 35), false, ""},
		{"mock-4", fixture.Home.ID, "income", "工资奖金", "工资到账", []string{"月薪"}, 1850000, at(0, 20, 5), true, ""},
		{"mock-5", fixture.Home.ID, "transfer", "五险一金", "社保代扣", []string{"固定支出"}, 500000, at(0, 21, 10), false, ""},
		{"mock-6", fixture.Home.ID, "expense", "购物消费", "超市采购", []string{"家庭"}, 12680, at(1, 9, 20), false, ""},
		{"mock-7", fixture.Home.ID, "expense", "娱乐休闲", "电影票", []string{"周末"}, 8000, at(4, 19, 30), false, ""},
		{"mock-8", fixture.Home.ID, "expense", "生活缴费", "房租", []string{"固定支出"}, 450000, at(9, 10, 0), false, ""},
		{"mock-9", fixture.Home.ID, "income", "二手转卖", "转卖旧相机", []string{"日常"}, 120000, at(15, 15, 20), false, ""},
		{"mock-13", fixture.Home.ID, "expense", "餐饮美食", "咖啡续杯", []string{"日常"}, 1800, at(2, 15, 0), false, ""},
		{"mock-14", fixture.Home.ID, "expense", "交通出行", "打车回家", []string{"通勤"}, 3600, at(3, 22, 10), false, ""},
		{"mock-15", fixture.Home.ID, "expense", "生活缴费", "水费", []string{"固定支出"}, 6800, at(5, 9, 30), false, ""},
		{"mock-16", fixture.Home.ID, "expense", "教育学习", "书店", []string{"书籍"}, 9800, at(6, 16, 20), false, ""},
		{"mock-17", fixture.Home.ID, "expense", "餐饮美食", "朋友聚餐", []string{"社交"}, 26000, at(7, 19, 0), false, ""},
		{"mock-18", fixture.Home.ID, "expense", "生活缴费", "话费", []string{"固定支出"}, 10000, at(8, 11, 0), false, ""},
		{"mock-19", fixture.Home.ID, "income", "投资理财", "理财收益", []string{"理财"}, 3250, at(11, 8, 30), false, ""},
		{"mock-20", fixture.Home.ID, "expense", "医疗健康", "健身月卡", []string{"健康"}, 29900, at(13, 18, 0), false, ""},
		{"mock-10", fixture.Home.ID, "expense", "医疗健康", "体检", []string{"健康"}, 68000, at(40, 8, 0), false, ""},
		{"mock-11", fixture.Home.ID, "income", "工资奖金", "年终奖", []string{"奖金"}, 3000000, at(90, 9, 0), false, ""},
		{"mock-12", fixture.Home.ID, "expense", "娱乐休闲", "旅行机票", []string{"旅行"}, 880000, at(400, 14, 0), false, ""},
		{"mock-travel-1", fixture.Travel.ID, "expense", "交通出行", "去程机票", []string{"旅行"}, 128000, at(0, 9, 15), false, ""},
		{"mock-travel-2", fixture.Travel.ID, "expense", "娱乐休闲", "民宿两晚", []string{"旅行"}, 86000, at(0, 18, 40), false, ""},
		{"mock-travel-3", fixture.Travel.ID, "income", "红包转账", "差旅报销", []string{"旅行"}, 30000, at(1, 12, 0), false, ""},
	}
	for _, one := range rows {
		flags := `{"outlier":false}`
		if one.outlier {
			flags = `{"outlier":true}`
		}
		if _, err := fixture.Workspace.CreateRecord(ctx, domain.TransactionRecord{
			TransactionID:   one.id,
			LedgerID:        one.ledger,
			AmountCents:     one.amount,
			TransactionType: one.kind,
			Category:        one.category,
			Description:     one.description,
			Flags:           flags,
			KeyEventDate:    one.keyEventDate,
			TransactionAt:   one.at.Unix(),
		}, one.tags); err != nil {
			t.Fatalf("storetest: seed record %s: %v", one.id, err)
		}
	}
}

func seedTemplates(t testing.TB, ctx context.Context, fixture *Fixture) {
	t.Helper()
	templates := []domain.TransactionTemplate{
		{
			LedgerID: fixture.Home.ID, TemplateName: "午餐 · 餐饮美食", TransactionType: "expense",
			Category: "餐饮美食", Tags: `["三餐"]`, Description: "午餐",
		},
		{
			LedgerID: fixture.Home.ID, TemplateName: "地铁通勤 · 交通出行", TransactionType: "expense",
			Category: "交通出行", Tags: `["地铁"]`, Description: "地铁通勤",
		},
		{
			LedgerID: fixture.Home.ID, TemplateName: "工资到账 · 工资奖金", TransactionType: "income",
			Category: "工资奖金", Tags: `["工资"]`, Description: "工资到账", Flags: "outlier",
		},
		{
			LedgerID: fixture.Travel.ID, TemplateName: "机票 · 交通出行", TransactionType: "expense",
			Category: "交通出行", Tags: `["高铁"]`, Description: "机票",
		},
		{
			LedgerID: fixture.Travel.ID, TemplateName: "民宿 · 娱乐休闲", TransactionType: "expense",
			Category: "娱乐休闲", Tags: `[]`, Description: "民宿两晚",
		},
	}
	for _, one := range templates {
		if _, err := fixture.Workspace.CreateTemplate(ctx, one); err != nil {
			t.Fatalf("storetest: seed template %s: %v", one.TemplateName, err)
		}
	}
}
