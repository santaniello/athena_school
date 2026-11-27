package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/domain/folder"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// The pool has a single connection, so a repository method that ignores the
// transaction in the context waits forever for the connection that
// transaction holds. Every repository method must join the transaction.
func TestRepositories_joinTheTransactionInTheContext(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name string
		call func(ctx context.Context, db *sql.DB) error
	}{
		{"folder Create", func(ctx context.Context, db *sql.DB) error {
			return NewFolderRepository(db).Create(ctx, folder.Folder{ID: "f-new", Name: "New", CreatedAt: now})
		}},
		{"folder List", func(ctx context.Context, db *sql.DB) error {
			_, err := NewFolderRepository(db).List(ctx)
			return err
		}},
		{"folder SetArchLabEnabled", func(ctx context.Context, db *sql.DB) error {
			return NewFolderRepository(db).SetArchLabEnabled(ctx, "f-1", true)
		}},
		{"folder Rename", func(ctx context.Context, db *sql.DB) error {
			return NewFolderRepository(db).Rename(ctx, "f-1", "Renamed")
		}},
		{"folder Delete", func(ctx context.Context, db *sql.DB) error {
			return NewFolderRepository(db).Delete(ctx, "f-1")
		}},
		{"usage Record", func(ctx context.Context, db *sql.DB) error {
			return NewUsageRepository(db).Record(ctx, domainllm.UsageEntry{ID: "u-1", Model: "m", CreatedAt: now})
		}},
		{"usage SessionTotals", func(ctx context.Context, db *sql.DB) error {
			_, err := NewUsageRepository(db).SessionTotals(ctx, "none")
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a database whose only connection a transaction holds, and a stored folder
			db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			require.NoError(t, NewFolderRepository(db).Create(context.Background(), folder.Folder{ID: "f-1", Name: "Folder", CreatedAt: now}))
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// When the method runs inside that transaction
			err = NewSQLTransactor(db).WithinTx(ctx, func(ctx context.Context) error {
				return tt.call(ctx, db)
			})

			// Then it joins the transaction instead of waiting for a second connection
			require.NoError(t, err)
		})
	}
}
