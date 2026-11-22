// Package backup writes and restores workspace snapshots.
//
// The pure naming and retention rules live here with the disk actions because
// the application is one process: a manual backup is a directory (database
// plus image assets), an automatic backup is one database file, and a
// pre-restore copy is one database file. Manual and automatic backups each keep
// at most two; pre-restore copies never participate in pruning.
package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"transactions/internal/store"
)

const (
	// DirName is the default directory under the user's Documents folder.
	DirName = "Transactions 备份"
	// ManualPrefix starts a manual backup directory.
	ManualPrefix = "Transactions-备份-"
	// AutoPrefix starts an automatic snapshot file.
	AutoPrefix = "auto-"
	// PreRestorePrefix starts the copy written before a restore.
	PreRestorePrefix = "pre-restore-"
	// ManualKeep and AutoKeep are the retention limits. The user asked for two.
	ManualKeep = 2
	AutoKeep   = 2
)

// Kind is what produced a backup.
type Kind string

const (
	KindManual     Kind = "manual"
	KindAuto       Kind = "auto"
	KindPreRestore Kind = "pre-restore"
	KindFile       Kind = "file"
)

// Entry is one backup the settings page may restore.
type Entry struct {
	Path          string
	Name          string
	Kind          Kind
	CreatedAt     int64
	SizeBytes     int64
	SchemaVersion int
	WorkspaceDir  string
	HasAssets     bool
	Restorable    bool
	Problem       string
}

// Result describes one manual backup.
type Result struct {
	Entry
	Duration time.Duration
}

// Service reads and writes backups for one application build.
type Service struct {
	SupportedSchema int
	Now             func() time.Time
}

// New returns a service for the current schema.
func New() *Service {
	return &Service{SupportedSchema: store.CurrentSchemaVersion(), Now: time.Now}
}

// DocumentsDir is the platform's Documents folder. The fallback keeps backups
// usable on a machine without a localized Documents folder.
func DocumentsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "Documents")
}

// ResolveDir trims the configured value, falling back to the default directory.
func ResolveDir(raw, documents string) string {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		return raw
	}
	if strings.TrimSpace(documents) == "" {
		return DirName
	}
	return filepath.Join(documents, DirName)
}

// DirProblem rejects a backup directory inside the workspace; backing up into
// the workspace would copy the backup into itself.
func DirProblem(backupDir, workspaceDir string) string {
	if strings.TrimSpace(workspaceDir) == "" {
		return ""
	}
	backup, err := filepath.Abs(backupDir)
	if err != nil {
		return ""
	}
	workspace, err := filepath.Abs(workspaceDir)
	if err != nil {
		return ""
	}
	backup, workspace = filepath.Clean(backup), filepath.Clean(workspace)
	if strings.EqualFold(backup, workspace) {
		return "备份位置不能是工作空间目录本身"
	}
	prefix := workspace + string(filepath.Separator)
	if strings.HasPrefix(strings.ToLower(backup), strings.ToLower(prefix)) {
		return "备份位置不能放在工作空间内部"
	}
	return ""
}

// List reads the directory and describes every backup-shaped entry, newest
// first. Unreadable or malformed entries are skipped, not fatal.
func (s *Service) List(directory string) []Entry {
	items, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}
	entries := make([]Entry, 0, len(items))
	for _, item := range items {
		info, err := item.Info()
		if err != nil {
			continue
		}
		entry, ok := s.entryOf(directory, item.Name(), info)
		if ok {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].CreatedAt != entries[j].CreatedAt {
			return entries[i].CreatedAt > entries[j].CreatedAt
		}
		return entries[i].Name < entries[j].Name
	})
	return entries
}

// Delete removes one backup inside directory. It refuses paths outside that
// directory, so a caller cannot turn a stale row into a delete anywhere on disk.
func (s *Service) Delete(directory, path string) error {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return fmt.Errorf("删除备份：%w", err)
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("删除备份：%w", err)
	}
	relative, err := filepath.Rel(directory, target)
	if err != nil || relative == "." || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return errors.New("删除备份：路径不在备份目录内")
	}
	info, err := os.Lstat(target)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("删除备份：%w", err)
	}
	if info.IsDir() {
		return os.RemoveAll(target)
	}
	return os.Remove(target)
}

// EntryAt describes one explicitly chosen file or directory. It is used by the
// "restore from file" path, where the file need not be in the backup directory.
func (s *Service) EntryAt(path string) (Entry, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Entry{}, err
	}
	entry, ok := s.entryOf(filepath.Dir(path), filepath.Base(path), info)
	if !ok && !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".db") {
		// A file the user picked does not have to follow the backup naming
		// scheme; the database probe below decides whether it can be restored.
		entry = Entry{
			Path: path, Name: filepath.Base(path), Kind: KindFile,
			CreatedAt: info.ModTime().Unix(), SizeBytes: info.Size(),
		}
		probe := store.ProbeDatabase(path, false)
		entry.SchemaVersion = probe.SchemaVersion
		entry.WorkspaceDir = probe.WorkspaceDir
		entry.Restorable, entry.Problem = judge(probe, s.SupportedSchema)
		ok = true
	}
	if !ok {
		return Entry{}, fmt.Errorf("不是可识别的备份")
	}
	return entry, nil
}

func (s *Service) entryOf(directory, name string, info os.FileInfo) (Entry, bool) {
	kind, created, ok := parseName(name)
	if !ok {
		return Entry{}, false
	}
	path := filepath.Join(directory, name)
	if kind == KindManual && !info.IsDir() {
		return Entry{}, false
	}
	if kind != KindManual && info.IsDir() {
		return Entry{}, false
	}
	database := path
	if kind == KindManual {
		database = filepath.Join(path, "transactions.db")
	}
	if _, err := os.Stat(database); err != nil {
		return Entry{}, false
	}
	entry := Entry{Path: path, Name: name, Kind: kind, CreatedAt: created}
	entry.SizeBytes = sizeOf(path)
	entry.HasAssets = kind == KindManual && exists(filepath.Join(path, "data", "assets"))
	probe := store.ProbeDatabase(database, false)
	entry.SchemaVersion = probe.SchemaVersion
	entry.WorkspaceDir = probe.WorkspaceDir
	entry.Restorable, entry.Problem = judge(probe, s.SupportedSchema)
	return entry, true
}

func judge(probe store.DBProbe, supported int) (bool, string) {
	if !probe.Opened {
		return false, "无法打开这个数据库"
	}
	if !probe.HasAppMeta {
		return false, "不是本应用的数据库"
	}
	if probe.QuickCheck != "ok" {
		return false, "数据库检查未通过"
	}
	if probe.SchemaVersion > supported {
		return false, fmt.Sprintf("库版本 v%d 高于当前应用支持的 v%d", probe.SchemaVersion, supported)
	}
	return true, ""
}

// CreateManual writes a directory with a consistent database snapshot and the
// workspace's image assets, then keeps only the newest two manual backups.
func (s *Service) CreateManual(ctx context.Context, workspace *store.Workspace, directory string) (Result, error) {
	if workspace == nil {
		return Result{}, fmt.Errorf("还没有打开工作空间")
	}
	if problem := DirProblem(directory, workspace.Directory()); problem != "" {
		return Result{}, errors.New(problem)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return Result{}, fmt.Errorf("创建备份目录失败：%w", err)
	}
	name := uniqueName(manualName(s.Now()), func(candidate string) bool {
		_, err := os.Stat(filepath.Join(directory, candidate))
		return err == nil
	})
	target := filepath.Join(directory, name)
	if err := os.Mkdir(target, 0o755); err != nil {
		return Result{}, fmt.Errorf("创建备份目录失败：%w", err)
	}
	started := s.Now()
	snapshot := filepath.Join(target, "transactions.db")
	if err := workspace.SnapshotTo(ctx, snapshot); err != nil {
		_ = os.RemoveAll(target)
		return Result{}, err
	}
	assets := filepath.Join(workspace.Directory(), "data", "assets")
	if exists(assets) {
		if err := copyDirectory(assets, filepath.Join(target, "data", "assets")); err != nil {
			_ = os.RemoveAll(target)
			return Result{}, fmt.Errorf("复制图片资产失败：%w", err)
		}
	}
	_ = s.pruneManual(directory, name)
	return Result{
		Entry: Entry{
			Path: target, Name: name, Kind: KindManual, CreatedAt: s.Now().Unix(),
			SizeBytes: sizeOf(target), SchemaVersion: s.SupportedSchema,
			WorkspaceDir: workspace.Directory(),
			HasAssets:    exists(filepath.Join(target, "data", "assets")), Restorable: true,
		},
		Duration: s.Now().Sub(started),
	}, nil
}

// AutoBackupIfNeeded writes today's automatic snapshot for this workspace, at
// most once per day, and then keeps only the newest two automatic snapshots.
// A nil result means today was already covered or the switch is off.
func (s *Service) AutoBackupIfNeeded(ctx context.Context, workspace *store.Workspace, directory string, enabled bool) (*Result, error) {
	if !enabled || workspace == nil {
		return nil, nil
	}
	if DirProblem(directory, workspace.Directory()) != "" {
		return nil, nil
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, err
	}
	name := autoName(s.Now())
	covered := false
	for _, candidate := range s.List(directory) {
		if candidate.Kind != KindAuto || candidate.CreatedAt != dayStart(s.Now()) {
			continue
		}
		if candidate.WorkspaceDir == "" || strings.EqualFold(filepath.Clean(candidate.WorkspaceDir), filepath.Clean(workspace.Directory())) {
			covered = true
			break
		}
	}
	created := ""
	if !covered {
		created = uniqueName(name, func(candidate string) bool {
			_, err := os.Stat(filepath.Join(directory, candidate))
			return err == nil
		})
		if err := workspace.SnapshotTo(ctx, filepath.Join(directory, created)); err != nil {
			return nil, err
		}
	}
	_ = s.pruneAuto(directory, created)
	if created == "" {
		return nil, nil
	}
	entry, err := s.EntryAt(filepath.Join(directory, created))
	if err != nil {
		return nil, err
	}
	return &Result{Entry: entry}, nil
}

// Restore replaces the workspace database with source (a manual directory or a
// single .db). It snapshots the current database into backupDir first, closes
// the workspace, swaps the files and returns the pre-restore path. The caller
// must reopen the workspace afterwards.
func (s *Service) Restore(ctx context.Context, workspace *store.Workspace, backupDir, source string) (string, error) {
	if workspace == nil {
		return "", fmt.Errorf("还没有打开工作空间")
	}
	requested, err := filepath.Abs(source)
	if err != nil {
		return "", fmt.Errorf("备份路径无效：%w", err)
	}
	info, err := os.Stat(requested)
	if err != nil {
		return "", fmt.Errorf("备份不存在：%s", source)
	}
	manual := info.IsDir()
	database := requested
	if manual {
		database = filepath.Join(requested, "transactions.db")
	}
	if _, err := os.Stat(database); err != nil {
		return "", fmt.Errorf("备份里没有 transactions.db")
	}
	if samePath(database, workspace.DatabasePath()) {
		return "", fmt.Errorf("不能从当前正在使用的库恢复")
	}
	if problem := DirProblem(backupDir, workspace.Directory()); problem != "" {
		return "", errors.New(problem)
	}
	probe := store.ProbeDatabase(database, true)
	ok, problem := judge(probe, s.SupportedSchema)
	if !ok {
		return "", errors.New(problem)
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", fmt.Errorf("创建备份目录失败：%w", err)
	}
	preName := uniqueName(preRestoreName(s.Now()), func(candidate string) bool {
		_, err := os.Stat(filepath.Join(backupDir, candidate))
		return err == nil
	})
	prePath := filepath.Join(backupDir, preName)
	if err := workspace.SnapshotTo(ctx, prePath); err != nil {
		return "", err
	}
	if err := workspace.Close(); err != nil {
		return "", fmt.Errorf("关闭工作空间失败：%w", err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(workspace.DatabasePath() + suffix); err != nil && !os.IsNotExist(err) {
			return prePath, fmt.Errorf("清理当前数据库失败：%w", err)
		}
	}
	if err := copyFile(database, workspace.DatabasePath()); err != nil {
		// The old database is already gone. Put the pre-restore snapshot back so
		// the caller can reopen a complete workspace instead of creating an empty
		// one on the next Open.
		_ = copyFile(prePath, workspace.DatabasePath())
		return prePath, fmt.Errorf("写入恢复数据库失败：%w", err)
	}
	if manual && exists(filepath.Join(requested, "data", "assets")) {
		assets := filepath.Join(workspace.Directory(), "data", "assets")
		_ = os.RemoveAll(assets)
		if err := copyDirectory(filepath.Join(requested, "data", "assets"), assets); err != nil {
			return prePath, fmt.Errorf("恢复图片资产失败：%w", err)
		}
	}
	return prePath, nil
}

func (s *Service) pruneManual(directory, preserve string) error {
	entries := s.List(directory)
	var manuals []Entry
	for _, entry := range entries {
		if entry.Kind == KindManual && entry.Name != preserve {
			manuals = append(manuals, entry)
		}
	}
	sort.Slice(manuals, func(i, j int) bool {
		if manuals[i].CreatedAt != manuals[j].CreatedAt {
			return manuals[i].CreatedAt > manuals[j].CreatedAt
		}
		return manuals[i].Name > manuals[j].Name
	})
	if len(manuals) >= ManualKeep {
		for _, stale := range manuals[ManualKeep-1:] {
			_ = os.RemoveAll(stale.Path)
		}
	}
	return nil
}

func (s *Service) pruneAuto(directory, preserve string) error {
	entries := s.List(directory)
	var autos []Entry
	for _, entry := range entries {
		if entry.Kind == KindAuto && entry.Name != preserve {
			autos = append(autos, entry)
		}
	}
	sort.Slice(autos, func(i, j int) bool {
		if autos[i].CreatedAt != autos[j].CreatedAt {
			return autos[i].CreatedAt > autos[j].CreatedAt
		}
		return autos[i].Name > autos[j].Name
	})
	if len(autos) >= AutoKeep {
		for _, stale := range autos[AutoKeep-1:] {
			_ = os.Remove(stale.Path)
		}
	}
	return nil
}

func manualName(when time.Time) string {
	return ManualPrefix + when.Format("20060102-150405")
}

func autoName(when time.Time) string {
	return AutoPrefix + when.Format("20060102") + ".db"
}

func preRestoreName(when time.Time) string {
	return PreRestorePrefix + when.Format("20060102-150405") + ".db"
}

func parseName(name string) (Kind, int64, bool) {
	if strings.HasPrefix(name, ManualPrefix) {
		when, ok := parseStamp(stampPart(strings.TrimPrefix(name, ManualPrefix)))
		return KindManual, when, ok
	}
	if strings.HasPrefix(name, AutoPrefix) && strings.HasSuffix(name, ".db") {
		date := stampPart(strings.TrimSuffix(strings.TrimPrefix(name, AutoPrefix), ".db"))
		if len(date) != 8 {
			return "", 0, false
		}
		when, err := time.ParseInLocation("20060102", date, time.Local)
		if err != nil {
			return "", 0, false
		}
		return KindAuto, when.Unix(), true
	}
	if strings.HasPrefix(name, PreRestorePrefix) && strings.HasSuffix(name, ".db") {
		when, ok := parseStamp(stampPart(strings.TrimSuffix(strings.TrimPrefix(name, PreRestorePrefix), ".db")))
		return KindPreRestore, when, ok
	}
	return "", 0, false
}

// stampPart removes the -2 / -3 suffix uniqueName adds to a same-second copy.
// The timestamp's own time segment is six digits, so a suffix is only the short
// final segment after the last hyphen.
func stampPart(text string) string {
	parts := strings.Split(text, "-")
	if len(parts) > 1 {
		last := parts[len(parts)-1]
		if len(last) > 0 && len(last) <= 4 {
			allDigits := true
			for _, digit := range last {
				if digit < '0' || digit > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return strings.Join(parts[:len(parts)-1], "-")
			}
		}
	}
	return text
}

func parseStamp(text string) (int64, bool) {
	when, err := time.ParseInLocation("20060102-150405", text, time.Local)
	if err != nil {
		return 0, false
	}
	return when.Unix(), true
}

func dayStart(when time.Time) int64 {
	y, m, d := when.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, when.Location()).Unix()
}

func uniqueName(name string, exists func(string) bool) string {
	if !exists(name) {
		return name
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for index := 2; index < 10_000; index++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, index, ext)
		if !exists(candidate) {
			return candidate
		}
	}
	return name
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sizeOf(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if !info.IsDir() {
		return info.Size()
	}
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func copyDirectory(source, target string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if info.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		return copyFile(path, destination)
	})
}

func copyFile(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func samePath(left, right string) bool {
	left, leftErr := filepath.Abs(left)
	right, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
