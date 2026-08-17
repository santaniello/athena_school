package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/santaniello/athena/internal/domain/folder"
)

// FolderRepository is the SQLite-backed implementation of
// folder.Repository.
type FolderRepository struct {
	db *sql.DB
}

// NewFolderRepository creates a FolderRepository backed by db. db must
// already have its migrations applied (see Open).
func NewFolderRepository(db *sql.DB) *FolderRepository {
	return &FolderRepository{db: db}
}

// Create inserts a new folder.
func (r *FolderRepository) Create(ctx context.Context, f folder.Folder) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO folders (id, name, is_default, created_at) VALUES (?, ?, ?, ?)`,
		f.ID, f.Name, f.IsDefault, f.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: creating folder: %w", err)
	}
	return nil
}

// GetByID returns the folder with the given id, or folder.ErrFolderNotFound
// if it does not exist.
func (r *FolderRepository) GetByID(ctx context.Context, id string) (folder.Folder, error) {
	var f folder.Folder
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, is_default, created_at FROM folders WHERE id = ?`, id,
	).Scan(&f.ID, &f.Name, &f.IsDefault, &f.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return folder.Folder{}, folder.ErrFolderNotFound
	}
	if err != nil {
		return folder.Folder{}, fmt.Errorf("sqlite: getting folder: %w", err)
	}
	return f, nil
}

// List returns every folder.
func (r *FolderRepository) List(ctx context.Context) ([]folder.Folder, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, is_default, created_at FROM folders`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing folders: %w", err)
	}
	defer func() { _ = rows.Close() }()

	folders := []folder.Folder{}
	for rows.Next() {
		var f folder.Folder
		if err := rows.Scan(&f.ID, &f.Name, &f.IsDefault, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scanning folder: %w", err)
		}
		folders = append(folders, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating folders: %w", err)
	}
	return folders, nil
}

// Rename updates the name of the folder with the given id, or returns
// folder.ErrFolderNotFound if it does not exist.
func (r *FolderRepository) Rename(ctx context.Context, id, name string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE folders SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return fmt.Errorf("sqlite: renaming folder: %w", err)
	}
	return requireRowAffected(result, folder.ErrFolderNotFound)
}

// Delete removes the folder with the given id, or returns
// folder.ErrFolderNotFound if it does not exist.
func (r *FolderRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM folders WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: deleting folder: %w", err)
	}
	return requireRowAffected(result, folder.ErrFolderNotFound)
}

// requireRowAffected returns notFound if result reports zero rows affected.
func requireRowAffected(result sql.Result, notFound error) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: checking rows affected: %w", err)
	}
	if rows == 0 {
		return notFound
	}
	return nil
}
