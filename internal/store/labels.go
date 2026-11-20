package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

// Labels returns the ledger's categories and tags plus their record counts. All
// four pieces are separate queries so the page does not do an N+1 walk.
func (w *Workspace) Labels(ctx context.Context, ledgerID string) (repository.LabelSnapshot, error) {
	snapshot := repository.LabelSnapshot{
		CategoryCounts: map[string]int{},
		TagCounts:      map[string]int{},
	}
	if strings.TrimSpace(ledgerID) == "" {
		return snapshot, fmt.Errorf("%w：账本 id 不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return snapshot, err
	}

	categoryRows, err := db.QueryContext(ctx,
		"SELECT ledger_id, name, transaction_type, sort_order FROM category WHERE ledger_id = ? "+
			"ORDER BY sort_order ASC, name DESC", ledgerID)
	if err != nil {
		return snapshot, err
	}
	for categoryRows.Next() {
		var category domain.Category
		if err := categoryRows.Scan(&category.LedgerID, &category.Name, &category.TransactionType, &category.SortOrder); err != nil {
			categoryRows.Close()
			return snapshot, err
		}
		snapshot.Categories = append(snapshot.Categories, category)
	}
	if err := categoryRows.Err(); err != nil {
		categoryRows.Close()
		return snapshot, err
	}
	categoryRows.Close()

	tagRows, err := db.QueryContext(ctx,
		"SELECT ledger_id, name, category_transaction_type, sort_order FROM tag WHERE ledger_id = ? "+
			"ORDER BY sort_order ASC, name DESC", ledgerID)
	if err != nil {
		return snapshot, err
	}
	for tagRows.Next() {
		var tag domain.Tag
		if err := tagRows.Scan(&tag.LedgerID, &tag.Name, &tag.CategoryTransactionType, &tag.SortOrder); err != nil {
			tagRows.Close()
			return snapshot, err
		}
		snapshot.Tags = append(snapshot.Tags, tag)
	}
	if err := tagRows.Err(); err != nil {
		tagRows.Close()
		return snapshot, err
	}
	tagRows.Close()

	if err := loadCounts(ctx, db,
		"SELECT category, COUNT(*) FROM transaction_record WHERE ledger_id = ? GROUP BY category",
		ledgerID, snapshot.CategoryCounts); err != nil {
		return snapshot, err
	}
	if err := loadCounts(ctx, db,
		"SELECT tag, COUNT(*) FROM transaction_record_tag WHERE ledger_id = ? GROUP BY tag",
		ledgerID, snapshot.TagCounts); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func loadCounts(ctx context.Context, db *sql.DB, query, ledgerID string, into map[string]int) error {
	rows, err := db.QueryContext(ctx, query, ledgerID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return err
		}
		into[name] = count
	}
	return rows.Err()
}

// CreateCategory appends a category with the next sort order.
func (w *Workspace) CreateCategory(ctx context.Context, ledgerID, name, transactionType string) error {
	name = strings.TrimSpace(name)
	if strings.TrimSpace(ledgerID) == "" || name == "" || !validTransactionType(transactionType) {
		return fmt.Errorf("%w：分类名称或交易类型无效", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		exists, err := rowExists(ctx, tx,
			"SELECT 1 FROM category WHERE ledger_id = ? AND name = ? AND transaction_type = ?",
			ledgerID, name, transactionType)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("%w：分类「%s」", repository.ErrDuplicate, name)
		}
		var sortOrder int
		if err := tx.QueryRowContext(ctx,
			"SELECT COALESCE(MAX(sort_order), 0) + 1 FROM category WHERE ledger_id = ? AND transaction_type = ?",
			ledgerID, transactionType).Scan(&sortOrder); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			"INSERT INTO category (ledger_id, name, transaction_type, sort_order) VALUES (?, ?, ?, ?)",
			ledgerID, name, transactionType, sortOrder)
		return duplicateError(err)
	})
}

// DeleteCategory deletes the category and its tags. Record-tag associations are
// deliberately left alone, as the old service did.
func (w *Workspace) DeleteCategory(ctx context.Context, ledgerID, name, transactionType string) error {
	if strings.TrimSpace(ledgerID) == "" || strings.TrimSpace(name) == "" || !validTransactionType(transactionType) {
		return fmt.Errorf("%w：分类名称或交易类型无效", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		key := name + ":" + transactionType
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM tag WHERE ledger_id = ? AND category_transaction_type = ?", ledgerID, key); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			"DELETE FROM category WHERE ledger_id = ? AND name = ? AND transaction_type = ?",
			ledgerID, name, transactionType)
		return err
	})
}

// ReorderCategories writes the new display order into sort_order.
func (w *Workspace) ReorderCategories(ctx context.Context, ledgerID, transactionType string, names []string) error {
	if strings.TrimSpace(ledgerID) == "" || !validTransactionType(transactionType) {
		return fmt.Errorf("%w：交易类型无效", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		for index, name := range names {
			if _, err := tx.ExecContext(ctx,
				"UPDATE category SET sort_order = ? WHERE ledger_id = ? AND name = ? AND transaction_type = ?",
				index, ledgerID, name, transactionType); err != nil {
				return err
			}
		}
		return nil
	})
}

// CreateTag appends a tag under its category/type key.
func (w *Workspace) CreateTag(ctx context.Context, ledgerID, name, categoryTransactionType string) error {
	name = strings.TrimSpace(name)
	if strings.TrimSpace(ledgerID) == "" || name == "" || !validCategoryTransactionType(categoryTransactionType) {
		return fmt.Errorf("%w：标签名称或分类无效", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		exists, err := rowExists(ctx, tx,
			"SELECT 1 FROM tag WHERE ledger_id = ? AND name = ? AND category_transaction_type = ?",
			ledgerID, name, categoryTransactionType)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("%w：标签「%s」", repository.ErrDuplicate, name)
		}
		var sortOrder int
		if err := tx.QueryRowContext(ctx,
			"SELECT COALESCE(MAX(sort_order), 0) + 1 FROM tag WHERE ledger_id = ? AND category_transaction_type = ?",
			ledgerID, categoryTransactionType).Scan(&sortOrder); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			"INSERT INTO tag (ledger_id, name, category_transaction_type, sort_order) VALUES (?, ?, ?, ?)",
			ledgerID, name, categoryTransactionType, sortOrder)
		return duplicateError(err)
	})
}

// DeleteTag removes the tag and its record associations.
func (w *Workspace) DeleteTag(ctx context.Context, ledgerID, name, categoryTransactionType string) error {
	name = strings.TrimSpace(name)
	if strings.TrimSpace(ledgerID) == "" || name == "" || !validCategoryTransactionType(categoryTransactionType) {
		return fmt.Errorf("%w：标签名称或分类无效", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			"DELETE FROM transaction_record_tag WHERE ledger_id = ? AND tag = ?", ledgerID, name); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			"DELETE FROM tag WHERE ledger_id = ? AND name = ? AND category_transaction_type = ?",
			ledgerID, name, categoryTransactionType)
		return err
	})
}

// ReorderTags writes the new display order into sort_order.
func (w *Workspace) ReorderTags(ctx context.Context, ledgerID, categoryTransactionType string, names []string) error {
	if strings.TrimSpace(ledgerID) == "" || !validCategoryTransactionType(categoryTransactionType) {
		return fmt.Errorf("%w：分类无效", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		for index, name := range names {
			if _, err := tx.ExecContext(ctx,
				"UPDATE tag SET sort_order = ? WHERE ledger_id = ? AND name = ? AND category_transaction_type = ?",
				index, ledgerID, name, categoryTransactionType); err != nil {
				return err
			}
		}
		return nil
	})
}

// rowExists is the duplicate pre-check used before a unique insert. The store
// uses one connection, so there is no race inside the application.
func rowExists(ctx context.Context, tx *sql.Tx, query string, args ...any) (bool, error) {
	var one int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// validCategoryTransactionType accepts the old "分类名:交易类型" key shape used
// by tag.category_transaction_type.
func validCategoryTransactionType(value string) bool {
	index := strings.LastIndex(value, ":")
	if index <= 0 || index == len(value)-1 {
		return false
	}
	return validTransactionType(value[index+1:])
}
