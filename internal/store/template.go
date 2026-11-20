package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"transactions/internal/domain"
	"transactions/internal/repository"
)

const templateColumns = "template_id, ledger_id, template_name, transaction_type, category, tags, flags, " +
	"description, sort_order, created_at, updated_at"

// Templates lists one ledger's templates in display order.
func (w *Workspace) Templates(ctx context.Context, ledgerID string) ([]domain.TransactionTemplate, error) {
	if strings.TrimSpace(ledgerID) == "" {
		return nil, fmt.Errorf("%w：账本 id 不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		"SELECT "+templateColumns+" FROM transaction_template WHERE ledger_id = ? "+
			"ORDER BY sort_order ASC, created_at DESC", ledgerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var templates []domain.TransactionTemplate
	for rows.Next() {
		var template domain.TransactionTemplate
		var tags, flags, description sql.NullString
		if err := rows.Scan(
			&template.TemplateID, &template.LedgerID, &template.TemplateName, &template.TransactionType,
			&template.Category, &tags, &flags, &description, &template.SortOrder,
			&template.CreatedAt, &template.UpdatedAt,
		); err != nil {
			return nil, err
		}
		template.Tags = tags.String
		template.Flags = flags.String
		template.Description = description.String
		templates = append(templates, template)
	}
	return templates, rows.Err()
}

// CreateTemplate inserts a template at the end of its ledger's order.
func (w *Workspace) CreateTemplate(ctx context.Context, template domain.TransactionTemplate) (string, error) {
	if err := validateTemplate(template); err != nil {
		return "", err
	}
	db, err := w.dbHandle()
	if err != nil {
		return "", err
	}
	if template.TemplateID == "" {
		template.TemplateID = newID()
	}
	now := nowUnix()
	template.CreatedAt, template.UpdatedAt = now, now
	err = inTx(ctx, db, func(tx *sql.Tx) error {
		if err := requireLedger(ctx, tx, template.LedgerID); err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx,
			"SELECT COALESCE(MAX(sort_order), 0) + 1 FROM transaction_template WHERE ledger_id = ?",
			template.LedgerID).Scan(&template.SortOrder); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			"INSERT INTO transaction_template (template_id, ledger_id, template_name, transaction_type, category, "+
				"tags, flags, description, sort_order, created_at, updated_at) "+
				"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			template.TemplateID, template.LedgerID, template.TemplateName, template.TransactionType,
			template.Category, template.Tags, template.Flags, template.Description, template.SortOrder,
			template.CreatedAt, template.UpdatedAt,
		)
		return err
	})
	if err != nil {
		return "", err
	}
	return template.TemplateID, nil
}

// DeleteTemplate removes one template; a missing row is fine.
func (w *Workspace) DeleteTemplate(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w：模板 id 不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, "DELETE FROM transaction_template WHERE template_id = ?", id)
	return err
}

// ReorderTemplates writes sort_order 0..n-1 for the given ids in one ledger.
func (w *Workspace) ReorderTemplates(ctx context.Context, ledgerID string, ids []string) error {
	if strings.TrimSpace(ledgerID) == "" {
		return fmt.Errorf("%w：账本 id 不能为空", repository.ErrInvalid)
	}
	db, err := w.dbHandle()
	if err != nil {
		return err
	}
	return inTx(ctx, db, func(tx *sql.Tx) error {
		for index, id := range ids {
			if _, err := tx.ExecContext(ctx,
				"UPDATE transaction_template SET sort_order = ?, updated_at = ? WHERE template_id = ? AND ledger_id = ?",
				index, nowUnix(), id, ledgerID); err != nil {
				return err
			}
		}
		return nil
	})
}

func validateTemplate(template domain.TransactionTemplate) error {
	if strings.TrimSpace(template.LedgerID) == "" {
		return fmt.Errorf("%w：账本 id 不能为空", repository.ErrInvalid)
	}
	if strings.TrimSpace(template.TemplateName) == "" {
		return fmt.Errorf("%w：模板名称不能为空", repository.ErrInvalid)
	}
	if !validTransactionType(template.TransactionType) {
		return fmt.Errorf("%w：交易类型 %q", repository.ErrInvalid, template.TransactionType)
	}
	if strings.TrimSpace(template.Category) == "" {
		return fmt.Errorf("%w：分类不能为空", repository.ErrInvalid)
	}
	return nil
}
