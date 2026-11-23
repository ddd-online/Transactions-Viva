package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

// Key events: the workspace's memories, one event per local date. They are not
// ledger data — the application stored one event per (ledger, date) and the
// page followed the ledger, but a ledger was never what an event was about, so
// this schema keeps a single set for the whole workspace.
//
// Images are rows in key_event_image, ordered per date. Their files live under
// data/assets/key_events/<date>/ and the store never touches the disk: deleting
// an event returns the image rows so the caller can remove the files.

const keyEventColumns = "id, date, title, content, color, created_at, updated_at"

const keyEventImageColumns = "id, event_date, file_path, thumb_path, original_path, " +
	"motion_path, sort_order, created_at"

// KeyEventsByYear lists one calendar year's events, newest date first. year is
// four digits; the page's picker is the only caller, so a malformed year is a
// programming error rather than user input.
func (w *Workspace) KeyEventsByYear(ctx context.Context, year string) ([]domain.KeyEvent, error) {
	year = strings.TrimSpace(year)
	if _, err := time.Parse("2006", year); err != nil {
		return nil, invalidf("年份「%s」无法识别", year)
	}
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+keyEventColumns+" FROM key_event WHERE date LIKE ? ORDER BY date DESC",
		year+"-%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []domain.KeyEvent
	for rows.Next() {
		event, err := scanKeyEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

// KeyEvent reads one day's event; a day without one is ErrNotFound.
func (w *Workspace) KeyEvent(ctx context.Context, date string) (domain.KeyEvent, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return domain.KeyEvent{}, invalidf("事件日期不能为空")
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.KeyEvent{}, err
	}
	row := db.QueryRowContext(ctx,
		"SELECT "+keyEventColumns+" FROM key_event WHERE date = ?", date)
	event, err := scanKeyEvent(row)
	if err == sql.ErrNoRows {
		return domain.KeyEvent{}, notFoundf("找不到这一天的事件")
	}
	if err != nil {
		return domain.KeyEvent{}, err
	}
	return event, nil
}

// SaveKeyEvent writes one day's event, keeping the row's id and creation time
// when the day already has one. The title follows the domain rule: the typed
// title, or the body's first non-empty line, cut to 200 code points.
func (w *Workspace) SaveKeyEvent(ctx context.Context, date, title, content, color string) (domain.KeyEvent, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return domain.KeyEvent{}, invalidf("事件日期不能为空")
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.KeyEvent{}, err
	}
	now := nowUnix()
	name := domain.KeyEventTitle(title, content)
	if _, err := db.ExecContext(ctx,
		"INSERT INTO key_event ("+keyEventColumns+") VALUES (?, ?, ?, ?, ?, ?, ?) "+
			"ON CONFLICT(date) DO UPDATE SET title = excluded.title, content = excluded.content, "+
			"color = excluded.color, updated_at = excluded.updated_at",
		newID(), date, name, content, strings.TrimSpace(color), now, now,
	); err != nil {
		return domain.KeyEvent{}, fmt.Errorf("保存事件失败：%w", err)
	}
	return w.KeyEvent(ctx, date)
}

// KeyEventImages lists one day's images in the order they were added.
func (w *Workspace) KeyEventImages(ctx context.Context, date string) ([]domain.KeyEventImage, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return nil, invalidf("事件日期不能为空")
	}
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+keyEventImageColumns+" FROM key_event_image WHERE event_date = ? "+
			"ORDER BY sort_order, created_at, rowid", date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var images []domain.KeyEventImage
	for rows.Next() {
		image, err := scanKeyEventImage(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	return images, rows.Err()
}

// CreateKeyEventImage inserts one image at the end of its day's list. The row's
// id is generated when the caller left it empty, and sort_order is the store's:
// it takes the day's current maximum plus one, so two quick imports cannot both
// claim the same slot. The returned row carries both.
func (w *Workspace) CreateKeyEventImage(ctx context.Context, image domain.KeyEventImage) (domain.KeyEventImage, error) {
	image.EventDate = strings.TrimSpace(image.EventDate)
	image.FilePath = strings.TrimSpace(image.FilePath)
	if image.EventDate == "" {
		return domain.KeyEventImage{}, invalidf("图片没有对应的日期")
	}
	if image.FilePath == "" {
		return domain.KeyEventImage{}, invalidf("图片缺少展示图")
	}
	if image.ID == "" {
		image.ID = newID()
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.KeyEventImage{}, err
	}
	var next int
	if err := db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(sort_order), 0) + 1 FROM key_event_image WHERE event_date = ?",
		image.EventDate).Scan(&next); err != nil {
		return domain.KeyEventImage{}, err
	}
	image.SortOrder = next
	image.CreatedAt = nowUnix()
	if _, err := db.ExecContext(ctx,
		"INSERT INTO key_event_image ("+keyEventImageColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		image.ID, image.EventDate, image.FilePath, image.ThumbPath,
		image.OriginalPath, image.MotionPath, image.SortOrder, image.CreatedAt,
	); err != nil {
		return domain.KeyEventImage{}, fmt.Errorf("保存事件图片失败：%w", err)
	}
	return image, nil
}

// DeleteKeyEventImage removes one image row and returns what it removed, so the
// caller can delete the row's files. A missing id is not an error: a repeated
// delete is the same request.
func (w *Workspace) DeleteKeyEventImage(ctx context.Context, imageID string) (domain.KeyEventImage, error) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return domain.KeyEventImage{}, invalidf("没有指定图片")
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.KeyEventImage{}, err
	}
	row := db.QueryRowContext(ctx,
		"SELECT "+keyEventImageColumns+" FROM key_event_image WHERE id = ?", imageID)
	image, err := scanKeyEventImage(row)
	if err == sql.ErrNoRows {
		return domain.KeyEventImage{}, nil
	}
	if err != nil {
		return domain.KeyEventImage{}, err
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM key_event_image WHERE id = ?", imageID); err != nil {
		return domain.KeyEventImage{}, fmt.Errorf("删除事件图片失败：%w", err)
	}
	return image, nil
}

// DeleteKeyEvent removes one day's event and its image rows in one transaction
// and returns the images, so the caller can delete their files after the commit.
// A day without an event is not an error.
func (w *Workspace) DeleteKeyEvent(ctx context.Context, date string) ([]domain.KeyEventImage, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return nil, invalidf("事件日期不能为空")
	}
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	images, err := w.KeyEventImages(ctx, date)
	if err != nil {
		return nil, err
	}
	if err := inTx(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM key_event_image WHERE event_date = ?", date); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM key_event WHERE date = ?", date); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("删除事件失败：%w", err)
	}
	return images, nil
}

// KeyEventRecords lists every record in the workspace whose 关联事件日期 is the
// day, across all ledgers, newest transaction first. The event page shows them
// beside the day and names each record's ledger, because an event is no longer
// scoped to one ledger.
func (w *Workspace) KeyEventRecords(ctx context.Context, date string) ([]repository.RecordEntry, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return nil, invalidf("事件日期不能为空")
	}
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+recordColumns+" FROM transaction_record WHERE key_event_date = ? "+
			"ORDER BY transaction_at DESC, transaction_id ASC", date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []repository.RecordEntry
	var ids []string
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		ids = append(ids, record.TransactionID)
		records = append(records, repository.RecordEntry{Record: record})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	tags, err := tagsForRecords(ctx, db, ids)
	if err != nil {
		return nil, err
	}
	for index := range records {
		records[index].Tags = tags[records[index].Record.TransactionID]
	}
	return records, nil
}

// scanKeyEvent reads one row; title, content and colour are nullable because
// the database allowed NULL there.
func scanKeyEvent(row interface{ Scan(...any) error }) (domain.KeyEvent, error) {
	var event domain.KeyEvent
	var title, content, color sql.NullString
	if err := row.Scan(&event.ID, &event.Date, &title, &content, &color,
		&event.CreatedAt, &event.UpdatedAt); err != nil {
		return domain.KeyEvent{}, err
	}
	event.Title, event.Content, event.Color = title.String, content.String, color.String
	return event, nil
}

// scanKeyEventImage reads one image row.
func scanKeyEventImage(row interface{ Scan(...any) error }) (domain.KeyEventImage, error) {
	var image domain.KeyEventImage
	if err := row.Scan(&image.ID, &image.EventDate, &image.FilePath, &image.ThumbPath,
		&image.OriginalPath, &image.MotionPath, &image.SortOrder, &image.CreatedAt); err != nil {
		return domain.KeyEventImage{}, err
	}
	return image, nil
}
