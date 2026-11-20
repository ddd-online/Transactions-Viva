// Command tr-import copies the accounting tables of an old Transactions
// workspace into a new workspace. The old database is opened read-only; the
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
	destination := flag.String("to", "", "新工作空间目录（不能已有 transactions.db）")
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
		fmt.Fprintln(os.Stderr, "导入失败：", err)
		os.Exit(1)
	}
	fmt.Printf("已导入到 %s\n", filepath.Join(*destination, "transactions.db"))
	fmt.Printf("账本 %d，分类 %d，标签 %d，记录 %d，记录标签 %d，模板 %d\n",
		report.Ledgers, report.Categories, report.Tags, report.Records, report.RecordTags, report.Templates)
	if report.SourceSchema > 0 {
		fmt.Printf("旧库 schema 版本：%d\n", report.SourceSchema)
	}
	for _, orphan := range report.Orphans {
		fmt.Println("警告：", orphan)
	}
}
