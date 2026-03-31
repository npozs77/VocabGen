package db

import "context"

// Store defines the database operations interface.
// Using an interface enables test doubles without a mocking framework.
type Store interface {
	// FindWord looks up a cached word entry by text and source language.
	// Returns the first matching entry or nil.
	FindWord(ctx context.Context, word, sourceLang string) (*WordRow, error)

	// FindExpression looks up a cached expression entry.
	// Returns the first matching entry or nil.
	FindExpression(ctx context.Context, expr, sourceLang string) (*ExpressionRow, error)

	// FindWords returns all matching word entries for a given word and source language.
	// Returns an empty slice (not nil) when no entries exist.
	FindWords(ctx context.Context, word, sourceLang string) ([]WordRow, error)

	// FindExpressions returns all matching expression entries.
	// Returns an empty slice (not nil) when no entries exist.
	FindExpressions(ctx context.Context, expr, sourceLang string) ([]ExpressionRow, error)

	// InsertWord stores a new word entry.
	InsertWord(ctx context.Context, row *WordRow) error

	// InsertExpression stores a new expression entry.
	InsertExpression(ctx context.Context, row *ExpressionRow) error

	// ListWords returns paginated word entries with optional filters.
	ListWords(ctx context.Context, filter ListFilter) ([]WordRow, int, error)

	// ListExpressions returns paginated expression entries with optional filters.
	ListExpressions(ctx context.Context, filter ListFilter) ([]ExpressionRow, int, error)

	// UpdateWord updates an existing word entry by ID.
	UpdateWord(ctx context.Context, id int64, row *WordRow) error

	// UpdateExpression updates an existing expression entry by ID.
	UpdateExpression(ctx context.Context, id int64, row *ExpressionRow) error

	// DeleteWord removes a word entry by ID.
	DeleteWord(ctx context.Context, id int64) error

	// DeleteExpression removes an expression entry by ID.
	DeleteExpression(ctx context.Context, id int64) error

	// ImportWords bulk-inserts word rows, skipping duplicates.
	ImportWords(ctx context.Context, rows []WordRow) (imported, skipped, failed int, err error)

	// ImportExpressions bulk-inserts expression rows, skipping duplicates.
	ImportExpressions(ctx context.Context, rows []ExpressionRow) (imported, skipped, failed int, err error)

	// Close closes the database connection.
	Close() error

	// BackupTo copies the database file to the given path.
	BackupTo(ctx context.Context, destPath string) error

	// RestoreFrom replaces the current database with the file at srcPath.
	RestoreFrom(ctx context.Context, srcPath string) error
}

// ListFilter holds pagination and filter parameters for list queries.
type ListFilter struct {
	SourceLang string
	TargetLang string
	Search     string // matches against word/expression, definition, english, tags
	Page       int    // 1-based
	PageSize   int    // default 50
}
