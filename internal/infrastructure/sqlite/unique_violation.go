package sqlite

import (
	"errors"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// isUniqueViolation reports whether err is a UNIQUE constraint violation on
// the given "table.column". The driver names the violated column in its
// message, which is the only way to tell one UNIQUE constraint from another
// on the same table (a clash on the id is a different failure).
func isUniqueViolation(err error, tableColumn string) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) || sqliteErr.Code() != sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return false
	}
	return strings.Contains(sqliteErr.Error(), tableColumn)
}
