package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sqlitedriver "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/santaniello/athena/internal/domain/auth"
)

// AccountRepository is the SQLite-backed implementation of
// auth.AccountRepository.
type AccountRepository struct {
	db *sql.DB
}

// NewAccountRepository creates an AccountRepository backed by db. db must
// already have its migrations applied (see Open).
func NewAccountRepository(db *sql.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

// Create inserts a new account. A duplicate email is rejected with
// auth.ErrEmailAlreadyExists.
func (r *AccountRepository) Create(ctx context.Context, account auth.Account) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO accounts (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		account.ID, account.Email, account.PasswordHash, account.CreatedAt,
	)
	if err != nil {
		var sqliteErr *sqlitedriver.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return auth.ErrEmailAlreadyExists
		}
		return fmt.Errorf("sqlite: creating account: %w", err)
	}
	return nil
}

// FindByEmail returns the account with the given email, or
// auth.ErrAccountNotFound if none exists.
func (r *AccountRepository) FindByEmail(ctx context.Context, email string) (auth.Account, error) {
	var account auth.Account
	err := r.db.QueryRowContext(ctx,
		`SELECT id, email, password_hash, created_at FROM accounts WHERE email = ?`, email,
	).Scan(&account.ID, &account.Email, &account.PasswordHash, &account.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.Account{}, auth.ErrAccountNotFound
	}
	if err != nil {
		return auth.Account{}, fmt.Errorf("sqlite: finding account by email: %w", err)
	}
	return account, nil
}

// UpdatePassword sets a new password hash for the account with the given
// id, or returns auth.ErrAccountNotFound if it does not exist.
func (r *AccountRepository) UpdatePassword(ctx context.Context, id string, passwordHash string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE accounts SET password_hash = ? WHERE id = ?`, passwordHash, id,
	)
	if err != nil {
		return fmt.Errorf("sqlite: updating account password: %w", err)
	}
	return errIfNoRowsAffected(result)
}

// Delete removes the account with the given id, or returns
// auth.ErrAccountNotFound if it does not exist.
func (r *AccountRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: deleting account: %w", err)
	}
	return errIfNoRowsAffected(result)
}

func errIfNoRowsAffected(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: checking rows affected: %w", err)
	}
	if rows == 0 {
		return auth.ErrAccountNotFound
	}
	return nil
}
