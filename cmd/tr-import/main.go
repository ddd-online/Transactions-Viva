// Command tr-import copies an old Transactions workspace's accounting tables,
// saved charts, key events and stock data (mapped to workspace stock accounts)
// into a new schema-v1 workspace. The old database is opened read-only; the
// destination must not already contain transactions.db.
//
//	go run ./cmd/tr-import -from <old-workspace-or-transactions.db> -to <new-dir>
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"transactions/internal/store"
)

func main() {
	source := flag.String("from", "", "旧工作空间目录，或它的 transactions.db")
	destination := flag.String("to", "", "新工作空间目录（不能已存在 transactions.db）")
	strict := flag.Bool("strict", false, "遇到孤儿行时中止，不写入目标库")
	flag.Parse()
	if *source == "" || *destination == "" {
		fmt.Fprintln(os.Stderr, "用法：tr-import -from <旧工作空间或 transactions.db> -to <新目录> [-strict]")
		os.Exit(2)
	}
	sourcePath := *source
	if info, err := os.Stat(sourcePath); err == nil && info.IsDir() {
		sourcePath = filepath.Join(sourcePath, "transactions.db")
	}
	report, err := store.ImportOld(*destination, sourcePath, *strict)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("已导入到 %s（schema v1）\n", filepath.Join(*destination, "transactions.db"))
	fmt.Printf("账本 %d，分类 %d，标签 %d，记录 %d，记录标签 %d，模板 %d，图表 %d\n",
		report.Ledgers, report.Categories, report.Tags, report.Records, report.RecordTags, report.Templates, report.Charts)
	fmt.Printf("事件 %d，事件图片 %d，图片文件 %d\n",
		report.Events, report.EventImages, report.EventImageFiles)
	fmt.Printf("股票账户 %d，资金记录 %d，持仓 %d，成交 %d，轮次 %d，心得 %d，天梯日 %d\n",
		report.StockAccounts, report.StockFunds, report.StockPositions, report.StockTrades,
		report.StockRounds, report.StockLessons, report.StockLadderDays)
	fmt.Printf("待办主题 %d，事项 %d，进度 %d\n", report.TodoCards, report.TodoItems, report.TodoProgress)
	if report.SourceSchema > 0 {
		fmt.Printf("旧数据库格式版本：%d\n", report.SourceSchema)
	}
	for _, merged := range report.MergedEvents {
		fmt.Println("说明：" + merged)
	}
	for _, missing := range report.MissingAssets {
		fmt.Println("警告：缺少图片文件 " + missing)
	}
	for _, orphan := range report.Orphans {
		fmt.Println("警告：" + orphan)
	}
}
