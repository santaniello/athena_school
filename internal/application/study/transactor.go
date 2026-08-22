package study

import "context"

// Transactor runs fn inside a single atomic unit of work, so a message
// write and its accompanying ContextUsage update either both land or
// neither does. Defined here (consumer side) per Go convention; implemented
// by internal/infrastructure/sqlite.SQLTransactor.
type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
