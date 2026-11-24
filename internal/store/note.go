package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"transactions/internal/domain"
)

// Notes: one tree per workspace. A folder may hold folders and notes; a note is
// always a leaf and roots hold folders only. A note's body is a numbered list of
// blocks, its title lives on the node row.

const noteNodeColumns = "id, parent_id, kind, title, sort_order, expanded, created_at, updated_at"

const noteBlockColumns = "id, note_id, kind, level, text, image_path, image_width, image_height, image_scale, sort_order, created_at, updated_at"

// NoteTree returns every node of the notes tree. The page assembles the tree from
// ParentID and SortOrder, so one query is enough for a whole workspace.
func (w *Workspace) NoteTree(ctx context.Context) ([]domain.NoteNode, error) {
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+noteNodeColumns+" FROM note_node ORDER BY parent_id, sort_order, created_at, rowid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nodes []domain.NoteNode
	for rows.Next() {
		node, err := scanNoteNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}

// scanNoteNode reads one node row.
func scanNoteNode(rows interface{ Scan(...any) error }) (domain.NoteNode, error) {
	var node domain.NoteNode
	var expanded int
	if err := rows.Scan(&node.ID, &node.ParentID, &node.Kind, &node.Title, &node.SortOrder,
		&expanded, &node.CreatedAt, &node.UpdatedAt); err != nil {
		return domain.NoteNode{}, err
	}
	node.Expanded = expanded != 0
	return node, nil
}

// Note returns one note with its blocks, top to bottom.
func (w *Workspace) Note(ctx context.Context, id string) (domain.Note, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Note{}, invalidf("没有指定笔记")
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.Note{}, err
	}
	return readNote(ctx, db, id)
}

// readNote reads a note inside or outside a transaction.
func readNote(ctx context.Context, query interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, id string) (domain.Note, error) {
	node, err := scanNoteNode(query.QueryRowContext(ctx,
		"SELECT "+noteNodeColumns+" FROM note_node WHERE id = ?", id))
	if err == sql.ErrNoRows {
		return domain.Note{}, notFoundf("找不到该笔记")
	}
	if err != nil {
		return domain.Note{}, err
	}
	if node.Kind != domain.NoteKindNote {
		return domain.Note{}, invalidf("该节点不是笔记")
	}
	rows, err := query.QueryContext(ctx,
		"SELECT "+noteBlockColumns+" FROM note_block WHERE note_id = ? ORDER BY sort_order, rowid", id)
	if err != nil {
		return domain.Note{}, err
	}
	defer rows.Close()
	note := domain.Note{Node: node}
	for rows.Next() {
		block, err := scanNoteBlock(rows)
		if err != nil {
			return domain.Note{}, err
		}
		note.Blocks = append(note.Blocks, block)
	}
	return note, rows.Err()
}

// scanNoteBlock reads one block row.
func scanNoteBlock(rows interface{ Scan(...any) error }) (domain.NoteBlock, error) {
	var block domain.NoteBlock
	if err := rows.Scan(&block.ID, &block.NoteID, &block.Kind, &block.Level, &block.Text,
		&block.ImagePath, &block.ImageWidth, &block.ImageHeight, &block.ImageScale,
		&block.SortOrder, &block.CreatedAt, &block.UpdatedAt); err != nil {
		return domain.NoteBlock{}, err
	}
	return block, nil
}

// CreateNoteNode inserts a folder or a note at the end of its parent's children.
// A note is created with one empty text block, so a note always has something to
// edit. Root-level notes are refused: the root holds folders only.
func (w *Workspace) CreateNoteNode(ctx context.Context, parentID, kind, title string) (domain.NoteNode, error) {
	kind = strings.TrimSpace(kind)
	if kind != domain.NoteKindFolder && kind != domain.NoteKindNote {
		return domain.NoteNode{}, invalidf("未知的笔记节点类型：%s", kind)
	}
	parentID = strings.TrimSpace(parentID)
	if kind == domain.NoteKindNote && parentID == "" {
		return domain.NoteNode{}, invalidf("根级只能放文件夹，笔记要放在文件夹里")
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.NoteNode{}, err
	}
	now := nowUnix()
	node := domain.NoteNode{
		ID: newID(), ParentID: parentID, Kind: kind,
		Title: domain.NoteTitle(title), CreatedAt: now, UpdatedAt: now,
	}
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		if parentID != "" {
			parentKind, err := noteNodeKind(ctx, tx, parentID)
			if err != nil {
				return err
			}
			if parentKind != domain.NoteKindFolder {
				return invalidf("笔记只能放在文件夹里")
			}
		}
		if err := tx.QueryRowContext(ctx,
			"SELECT COALESCE(MAX(sort_order), -1) + 1 FROM note_node WHERE parent_id = ?",
			parentID).Scan(&node.SortOrder); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO note_node ("+noteNodeColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			node.ID, node.ParentID, node.Kind, node.Title, node.SortOrder, boolInt(node.Expanded),
			node.CreatedAt, node.UpdatedAt); err != nil {
			return fmt.Errorf("创建笔记节点失败：%w", err)
		}
		if kind == domain.NoteKindNote {
			block := domain.NoteBlock{
				ID: newID(), NoteID: node.ID, Kind: domain.NoteBlockText,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := insertNoteBlock(ctx, tx, block); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return domain.NoteNode{}, err
	}
	return node, nil
}

// RenameNoteNode rewrites a node's title; an empty title is allowed and shows as
// the default name. A missing node is ErrNotFound.
func (w *Workspace) RenameNoteNode(ctx context.Context, id, title string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return invalidf("没有指定笔记节点")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx,
		"UPDATE note_node SET title = ?, updated_at = ? WHERE id = ?",
		domain.NoteTitle(title), nowUnix(), id)
	if err != nil {
		return fmt.Errorf("重命名笔记失败：%w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return notFoundf("找不到该笔记节点")
	}
	return nil
}

// DeleteNoteNode removes one node and, when it is a folder, its whole subtree.
// It returns the deleted notes and the image paths they referenced, so the caller
// can delete the files. This deletion is not undoable.
func (w *Workspace) DeleteNoteNode(ctx context.Context, id string) (domain.NoteDeleted, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.NoteDeleted{}, invalidf("没有指定笔记节点")
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.NoteDeleted{}, err
	}
	var deleted domain.NoteDeleted
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		ids, err := noteSubtree(ctx, tx, id)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return notFoundf("找不到该笔记节点")
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		args := make([]any, 0, len(ids)+1)
		for _, one := range ids {
			args = append(args, one)
		}
		// The note ids among the subtree are the ones whose images must go.
		rows, err := tx.QueryContext(ctx,
			"SELECT id FROM note_node WHERE kind = ? AND id IN ("+placeholders+")",
			append([]any{domain.NoteKindNote}, args...)...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var noteID string
			if err := rows.Scan(&noteID); err != nil {
				rows.Close()
				return err
			}
			deleted.NoteIDs = append(deleted.NoteIDs, noteID)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if len(deleted.NoteIDs) > 0 {
			notePlaceholders := strings.TrimSuffix(strings.Repeat("?,", len(deleted.NoteIDs)), ",")
			imageArgs := make([]any, 0, len(deleted.NoteIDs))
			for _, one := range deleted.NoteIDs {
				imageArgs = append(imageArgs, one)
			}
			imageRows, err := tx.QueryContext(ctx,
				"SELECT image_path FROM note_block WHERE kind = ? AND image_path <> '' "+
					"AND note_id IN ("+notePlaceholders+")",
				append([]any{domain.NoteBlockImage}, imageArgs...)...)
			if err != nil {
				return err
			}
			for imageRows.Next() {
				var imagePath string
				if err := imageRows.Scan(&imagePath); err != nil {
					imageRows.Close()
					return err
				}
				deleted.ImagePaths = append(deleted.ImagePaths, imagePath)
			}
			if err := imageRows.Close(); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				"DELETE FROM note_block WHERE note_id IN ("+notePlaceholders+")",
				imageArgs...); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM note_node WHERE id IN ("+placeholders+")", args...); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return domain.NoteDeleted{}, err
	}
	return deleted, nil
}

// MoveNoteNode moves a node under parentID, before the node at position among its
// new siblings (a negative or too large position appends it). Both orders are
// rewritten so every parent keeps 0..n-1. A folder cannot move into its own
// subtree, and a note cannot move to the root.
func (w *Workspace) MoveNoteNode(ctx context.Context, id, parentID string, position int) error {
	id, parentID = strings.TrimSpace(id), strings.TrimSpace(parentID)
	if id == "" {
		return invalidf("没有指定笔记节点")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		var kind string
		if err := tx.QueryRowContext(ctx, "SELECT kind FROM note_node WHERE id = ?", id).Scan(&kind); err == sql.ErrNoRows {
			return notFoundf("找不到该笔记节点")
		} else if err != nil {
			return err
		}
		if kind == domain.NoteKindNote && parentID == "" {
			return invalidf("根级只能放文件夹，笔记要放在文件夹里")
		}
		if parentID != "" {
			parentKind, err := noteNodeKind(ctx, tx, parentID)
			if err != nil {
				return err
			}
			if parentKind != domain.NoteKindFolder {
				return invalidf("笔记只能放在文件夹里")
			}
			if parentID == id {
				return invalidf("不能把文件夹移到它自己里面")
			}
			inside, err := noteSubtreeContains(ctx, tx, id, parentID)
			if err != nil {
				return err
			}
			if inside {
				return invalidf("不能把文件夹移到它自己里面")
			}
		}
		siblings, err := noteSiblingIDs(ctx, tx, parentID, id)
		if err != nil {
			return err
		}
		if position < 0 || position > len(siblings) {
			position = len(siblings)
		}
		siblings = append(siblings, "")
		copy(siblings[position+1:], siblings[position:])
		siblings[position] = id
		if _, err := tx.ExecContext(ctx,
			"UPDATE note_node SET parent_id = ?, updated_at = ? WHERE id = ?",
			parentID, nowUnix(), id); err != nil {
			return err
		}
		return writeNoteOrder(ctx, tx, siblings)
	})
}

// ReorderNoteNodes rewrites the order of one folder's children to the given ids,
// which must be exactly that folder's children.
func (w *Workspace) ReorderNoteNodes(ctx context.Context, parentID string, ids []string) error {
	parentID = strings.TrimSpace(parentID)
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		existing, err := noteSiblingIDs(ctx, tx, parentID, "")
		if err != nil {
			return err
		}
		if len(existing) != len(ids) {
			return invalidf("排序列表与文件夹的子项不一致")
		}
		seen := make(map[string]bool, len(ids))
		for _, id := range ids {
			seen[id] = true
		}
		for _, id := range existing {
			if !seen[id] {
				return invalidf("排序列表与文件夹的子项不一致")
			}
		}
		return writeNoteOrder(ctx, tx, ids)
	})
}

// SetNoteNodeExpanded stores a folder's fold state. The page also keeps it in
// memory; the database copy is what survives a restart.
func (w *Workspace) SetNoteNodeExpanded(ctx context.Context, id string, expanded bool) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return invalidf("没有指定笔记节点")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	result, err := db.ExecContext(ctx,
		"UPDATE note_node SET expanded = ? WHERE id = ?", boolInt(expanded), id)
	if err != nil {
		return fmt.Errorf("保存折叠状态失败：%w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return notFoundf("找不到该笔记节点")
	}
	return nil
}

// SetNoteFoldersExpanded opens or closes every folder at once, the way the
// tree's 展开全部 / 收起全部 command does. It touches only folders, so a note's
// stored state is never rewritten by it.
func (w *Workspace) SetNoteFoldersExpanded(ctx context.Context, expanded bool) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx,
		"UPDATE note_node SET expanded = ? WHERE kind = ?", boolInt(expanded), domain.NoteKindFolder); err != nil {
		return fmt.Errorf("保存折叠状态失败：%w", err)
	}
	return nil
}

// SaveNote writes a note's title and blocks in one transaction. Blocks with an id
// are updated in place, blocks without one are inserted, and rows the caller
// dropped are deleted; only rows that actually changed are written, so saving a
// thousand-block note costs a scan rather than a rewrite. The returned note
// carries the assigned ids and timestamps.
func (w *Workspace) SaveNote(ctx context.Context, note domain.Note) (domain.Note, error) {
	id := strings.TrimSpace(note.Node.ID)
	if id == "" {
		return domain.Note{}, invalidf("没有指定笔记")
	}
	blocks := make([]domain.NoteBlock, 0, len(note.Blocks)+1)
	for _, block := range note.Blocks {
		normalized, err := normalizeNoteBlock(block)
		if err != nil {
			return domain.Note{}, err
		}
		blocks = append(blocks, normalized)
	}
	if len(blocks) == 0 {
		blocks = append(blocks, domain.NoteBlock{Kind: domain.NoteBlockText})
	}
	db, err := w.dbHandle()
	if err != nil {
		return domain.Note{}, err
	}
	now := nowUnix()
	var saved domain.Note
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		var kind string
		if err := tx.QueryRowContext(ctx, "SELECT kind FROM note_node WHERE id = ?", id).Scan(&kind); err == sql.ErrNoRows {
			return notFoundf("找不到该笔记")
		} else if err != nil {
			return err
		}
		if kind != domain.NoteKindNote {
			return invalidf("该节点不是笔记")
		}
		title := domain.NoteTitle(note.Node.Title)
		if _, err := tx.ExecContext(ctx,
			"UPDATE note_node SET title = ?, updated_at = ? WHERE id = ?", title, now, id); err != nil {
			return err
		}
		existing, err := readNoteBlocks(ctx, tx, id)
		if err != nil {
			return err
		}
		keep := make(map[string]bool, len(blocks))
		for index := range blocks {
			block := blocks[index]
			block.SortOrder = index
			keep[block.ID] = block.ID != ""
			if block.ID == "" {
				block.ID = newID()
				block.NoteID = id
				block.CreatedAt, block.UpdatedAt = now, now
				if err := insertNoteBlock(ctx, tx, block); err != nil {
					return err
				}
				blocks[index] = block
				continue
			}
			block.NoteID = id
			old, found := existing[block.ID]
			if !found {
				// A block restored by undo keeps its id.
				block.CreatedAt, block.UpdatedAt = now, now
				if err := insertNoteBlock(ctx, tx, block); err != nil {
					return err
				}
				blocks[index] = block
				continue
			}
			block.CreatedAt = old.CreatedAt
			block.UpdatedAt = old.UpdatedAt
			if !sameNoteBlock(old, block) {
				block.UpdatedAt = now
				if err := updateNoteBlock(ctx, tx, block); err != nil {
					return err
				}
			}
			blocks[index] = block
		}
		for blockID := range existing {
			if !keep[blockID] {
				if _, err := tx.ExecContext(ctx, "DELETE FROM note_block WHERE id = ?", blockID); err != nil {
					return err
				}
			}
		}
		saved, err = readNote(ctx, tx, id)
		return err
	})
	if err != nil {
		return domain.Note{}, err
	}
	if len(saved.Blocks) == 0 {
		saved.Blocks = blocks
	}
	return saved, nil
}

// LastNoteID is the note the 笔记 page opened last, or empty when there is none
// (or the note has since been deleted).
func (w *Workspace) LastNoteID(ctx context.Context) (string, error) {
	db, err := w.dbHandle()
	if err != nil {
		return "", err
	}
	var value string
	err = db.QueryRowContext(ctx, "SELECT value FROM app_meta WHERE key = 'last_note_id'").Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if _, err := w.Note(ctx, value); err != nil {
		return "", nil
	}
	return value, nil
}

// SetLastNoteID remembers the note the 笔记 page opened last in this workspace.
func (w *Workspace) SetLastNoteID(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id != "" {
		if _, err := w.Note(ctx, id); err != nil {
			return err
		}
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx,
		"INSERT INTO app_meta(key, value) VALUES ('last_note_id', ?) "+
			"ON CONFLICT(key) DO UPDATE SET value = excluded.value", id)
	return err
}

// NoteRailWidth is the width the 笔记 tree column had when the section was last
// left, or zero when it was never dragged.
func (w *Workspace) NoteRailWidth(ctx context.Context) (float32, error) {
	db, err := w.dbHandle()
	if err != nil {
		return 0, err
	}
	var value string
	err = db.QueryRowContext(ctx, "SELECT value FROM app_meta WHERE key = 'note_rail_width'").Scan(&value)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	width, err := strconv.ParseFloat(strings.TrimSpace(value), 32)
	if err != nil || width <= 0 {
		return 0, nil
	}
	return float32(width), nil
}

// NoteRailHidden says the 笔记 tree column was left hidden.
func (w *Workspace) NoteRailHidden(ctx context.Context) (bool, error) {
	db, err := w.dbHandle()
	if err != nil {
		return false, err
	}
	var value string
	err = db.QueryRowContext(ctx, "SELECT value FROM app_meta WHERE key = 'note_rail_hidden'").Scan(&value)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(value) == "1", nil
}

// SetNoteRailHidden remembers whether the 笔记 tree column is showing.
func (w *Workspace) SetNoteRailHidden(ctx context.Context, hidden bool) error {
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	value := "0"
	if hidden {
		value = "1"
	}
	_, err = db.ExecContext(ctx,
		"INSERT INTO app_meta(key, value) VALUES ('note_rail_hidden', ?) "+
			"ON CONFLICT(key) DO UPDATE SET value = excluded.value", value)
	return err
}

// SetNoteRailWidth remembers the width of the 笔记 tree column, so the split the
// user dragged comes back with the workspace.
func (w *Workspace) SetNoteRailWidth(ctx context.Context, width float32) error {
	if !(width > 0) {
		return invalidf("目录宽度无效")
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx,
		"INSERT INTO app_meta(key, value) VALUES ('note_rail_width', ?) "+
			"ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		strconv.FormatFloat(float64(width), 'f', 1, 32))
	return err
}

// normalizeNoteBlock checks a block's kind and folds its fields into the stored
// shape: heading levels 1..5, image scale in range, image paths trimmed.
func normalizeNoteBlock(block domain.NoteBlock) (domain.NoteBlock, error) {
	block.Kind = strings.TrimSpace(block.Kind)
	switch block.Kind {
	case domain.NoteBlockText:
		block.Level = 0
	case domain.NoteBlockHeading:
		block.Level = domain.NoteHeadingLevel(block.Level)
	case domain.NoteBlockImage:
		block.Level = 0
		block.ImagePath = strings.TrimSpace(block.ImagePath)
		block.ImageScale = domain.NoteImageScale(block.ImageScale)
	default:
		return domain.NoteBlock{}, invalidf("未知的块类型：%s", block.Kind)
	}
	block.ImageWidth = maxInt(block.ImageWidth, 0)
	block.ImageHeight = maxInt(block.ImageHeight, 0)
	return block, nil
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

// insertNoteBlock writes a new block row.
func insertNoteBlock(ctx context.Context, tx *sql.Tx, block domain.NoteBlock) error {
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO note_block ("+noteBlockColumns+") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		block.ID, block.NoteID, block.Kind, block.Level, block.Text, block.ImagePath,
		block.ImageWidth, block.ImageHeight, block.ImageScale, block.SortOrder,
		block.CreatedAt, block.UpdatedAt); err != nil {
		return fmt.Errorf("保存笔记块失败：%w", err)
	}
	return nil
}

// updateNoteBlock rewrites one existing block row.
func updateNoteBlock(ctx context.Context, tx *sql.Tx, block domain.NoteBlock) error {
	if _, err := tx.ExecContext(ctx,
		"UPDATE note_block SET kind = ?, level = ?, text = ?, image_path = ?, image_width = ?, "+
			"image_height = ?, image_scale = ?, sort_order = ?, updated_at = ? WHERE id = ?",
		block.Kind, block.Level, block.Text, block.ImagePath, block.ImageWidth,
		block.ImageHeight, block.ImageScale, block.SortOrder, block.UpdatedAt, block.ID); err != nil {
		return fmt.Errorf("保存笔记块失败：%w", err)
	}
	return nil
}

func sameNoteBlock(left, right domain.NoteBlock) bool {
	return left.Kind == right.Kind && left.Level == right.Level && left.Text == right.Text &&
		left.ImagePath == right.ImagePath && left.ImageWidth == right.ImageWidth &&
		left.ImageHeight == right.ImageHeight && left.ImageScale == right.ImageScale &&
		left.SortOrder == right.SortOrder
}

// readNoteBlocks reads one note's block rows keyed by id.
func readNoteBlocks(ctx context.Context, tx *sql.Tx, noteID string) (map[string]domain.NoteBlock, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT "+noteBlockColumns+" FROM note_block WHERE note_id = ?", noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	blocks := map[string]domain.NoteBlock{}
	for rows.Next() {
		block, err := scanNoteBlock(rows)
		if err != nil {
			return nil, err
		}
		blocks[block.ID] = block
	}
	return blocks, rows.Err()
}

// noteNodeKind reads a node's kind.
func noteNodeKind(ctx context.Context, tx *sql.Tx, id string) (string, error) {
	var kind string
	err := tx.QueryRowContext(ctx, "SELECT kind FROM note_node WHERE id = ?", id).Scan(&kind)
	if err == sql.ErrNoRows {
		return "", notFoundf("找不到该文件夹")
	}
	return kind, err
}

// noteSubtree lists a node and every node below it.
func noteSubtree(ctx context.Context, tx *sql.Tx, id string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		"WITH RECURSIVE subtree(id) AS ("+
			"SELECT id FROM note_node WHERE id = ? "+
			"UNION ALL SELECT child.id FROM note_node child JOIN subtree ON child.parent_id = subtree.id"+
			") SELECT id FROM subtree", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var one string
		if err := rows.Scan(&one); err != nil {
			return nil, err
		}
		ids = append(ids, one)
	}
	return ids, rows.Err()
}

// noteSubtreeContains reports whether target is inside root's subtree.
func noteSubtreeContains(ctx context.Context, tx *sql.Tx, root, target string) (bool, error) {
	for target != "" {
		if target == root {
			return true, nil
		}
		var parent string
		err := tx.QueryRowContext(ctx, "SELECT parent_id FROM note_node WHERE id = ?", target).Scan(&parent)
		if err == sql.ErrNoRows {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		target = parent
	}
	return false, nil
}

// noteSiblingIDs lists one parent's children in order, leaving out skip.
func noteSiblingIDs(ctx context.Context, tx *sql.Tx, parentID, skip string) ([]string, error) {
	rows, err := tx.QueryContext(ctx,
		"SELECT id FROM note_node WHERE parent_id = ? AND id <> ? ORDER BY sort_order, created_at, rowid",
		parentID, skip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var one string
		if err := rows.Scan(&one); err != nil {
			return nil, err
		}
		ids = append(ids, one)
	}
	return ids, rows.Err()
}

// writeNoteOrder writes sort_order 0..n-1 for the given ids.
func writeNoteOrder(ctx context.Context, tx *sql.Tx, ids []string) error {
	for index, id := range ids {
		if _, err := tx.ExecContext(ctx,
			"UPDATE note_node SET sort_order = ? WHERE id = ?", index, id); err != nil {
			return fmt.Errorf("保存笔记顺序失败：%w", err)
		}
	}
	return nil
}
