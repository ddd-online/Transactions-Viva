// Package diary moves diary files between a directory and a ledger: the batch
// import and export behind the application's settings · 日记配置 section.
//
// Both directions are `.txt` only. A file imports when its name is a real
// `YYYY-MM-DD.txt`, and an export writes exactly that name, so an exported
// directory imports again unchanged. The bytes are UTF-8 on the way out; on the
// way in decode.go tries UTF-8, then UTF-16 with a byte-order mark, then GBK, so
// a diary written by another program years ago still arrives readable.
package diary

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

// File is one diary file a scan found.
type File struct {
	// Date is the day the name names, already checked against the calendar.
	Date string
	// Path is where the file is, as the scan built it.
	Path string
}

// FileError is one file that did not import or export.
type FileError struct {
	Date    string
	Message string
}

// ImportResult is what one batch import did. Total counts the files the scan
// found, Imported the ones that reached the ledger.
type ImportResult struct {
	Imported int
	Total    int
	Failures []FileError
}

// ExportResult is what one export wrote.
type ExportResult struct {
	Total    int
	Written  int
	Failures []FileError
}

// Scan collects the diary files under a directory, recursively, oldest first.
// Only `YYYY-MM-DD.txt` files count: another extension is skipped rather than
// guessed at, and the date in the name must be a real day.
func Scan(directory string) ([]File, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return nil, fmt.Errorf("扫描目录失败：路径为空")
	}
	var files []File
	walk := func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		date, ok := domain.DiaryFileDate(entry.Name())
		if !ok {
			return nil
		}
		files = append(files, File{Date: date, Path: path})
		return nil
	}
	if err := filepath.WalkDir(directory, walk); err != nil {
		return nil, fmt.Errorf("扫描目录失败：%w", err)
	}
	sort.SliceStable(files, func(left, right int) bool { return files[left].Date < files[right].Date })
	return files, nil
}

// Import scans directory and writes every diary file it finds into the ledger.
// One file that cannot be read is reported and the batch continues: a single
// broken file must not throw away the other nine hundred.
func Import(ctx context.Context, repo repository.Repository, ledgerID, directory string) (ImportResult, error) {
	files, err := Scan(directory)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportFiles(ctx, repo, ledgerID, files)
}

// ImportFiles writes the files a scan found, so a caller that listed the
// directory itself (or a test that wants one broken path among good ones) runs
// the same import.
func ImportFiles(ctx context.Context, repo repository.Repository, ledgerID string, files []File) (ImportResult, error) {
	if strings.TrimSpace(ledgerID) == "" {
		return ImportResult{}, fmt.Errorf("导入日记失败：没有账本")
	}
	result := ImportResult{Total: len(files)}
	for _, file := range files {
		raw, err := os.ReadFile(file.Path)
		if err != nil {
			result.Failures = append(result.Failures, FileError{Date: file.Date, Message: "读取文件失败 " + err.Error()})
			continue
		}
		// An imported file carries no mood: the import left the day's mood
		// as it found it, which for a new day is empty.
		if _, err := repo.SaveDiary(ctx, ledgerID, file.Date, DecodeText(raw), ""); err != nil {
			result.Failures = append(result.Failures, FileError{Date: file.Date, Message: err.Error()})
			continue
		}
		result.Imported++
	}
	return result, nil
}

// Export writes the ledger's diary to directory as `<date>.txt`, filtered to the
// query's scope, and reports how many files it wrote. The directory is created
// when it does not exist; a file that cannot be written is reported and the rest
// still go out.
func Export(ctx context.Context, repo repository.Repository, ledgerID, directory string, query domain.DiaryExportQuery) (ExportResult, error) {
	if strings.TrimSpace(ledgerID) == "" {
		return ExportResult{}, fmt.Errorf("导出日记失败：没有账本")
	}
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return ExportResult{}, fmt.Errorf("导出日记失败：路径为空")
	}
	entries, err := repo.DiaryEntries(ctx, ledgerID)
	if err != nil {
		return ExportResult{}, fmt.Errorf("读取日记失败：%w", err)
	}
	selected := make([]domain.DiaryEntry, 0, len(entries))
	for _, entry := range entries {
		if query.Includes(entry.Date) {
			selected = append(selected, entry)
		}
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return ExportResult{}, fmt.Errorf("创建导出目录失败：%w", err)
	}
	result := ExportResult{Total: len(selected)}
	for _, entry := range selected {
		path := filepath.Join(directory, domain.DiaryFileName(entry.Date))
		if err := os.WriteFile(path, []byte(entry.Content), 0o644); err != nil {
			result.Failures = append(result.Failures, FileError{Date: entry.Date, Message: err.Error()})
			continue
		}
		result.Written++
	}
	return result, nil
}
