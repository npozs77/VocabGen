package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// SQLiteStore implements Store using modernc.org/sqlite.
type SQLiteStore struct {
	db     *sql.DB
	dbPath string
}

// NewSQLiteStore opens (or creates) the SQLite database at the given path,
// runs migrations, and returns a ready-to-use store.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	slog.Debug("opening database", slog.String("path", dbPath))

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	store := &SQLiteStore{db: db, dbPath: dbPath}
	if err := store.Migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	slog.Debug("database ready", slog.String("path", dbPath))
	return store, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- Word CRUD ---

// FindWord returns the first matching word entry or nil.
func (s *SQLiteStore) FindWord(ctx context.Context, word, sourceLang string) (*WordRow, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, word, part_of_speech, article, definition, english_definition,
			example, english, target_translation, notes, connotation, register,
			collocations, contrastive_notes, secondary_meanings, tags,
			source_language, target_language, created_at, updated_at
		FROM words WHERE word = ? AND source_language = ? LIMIT 1`,
		word, sourceLang,
	)
	return scanWordRow(row)
}

// GetWord returns a single word entry by ID, or nil if not found.
func (s *SQLiteStore) GetWord(ctx context.Context, id int64) (*WordRow, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, word, part_of_speech, article, definition, english_definition,
			example, english, target_translation, notes, connotation, register,
			collocations, contrastive_notes, secondary_meanings, tags,
			source_language, target_language, created_at, updated_at
		FROM words WHERE id = ?`,
		id,
	)
	return scanWordRow(row)
}

// FindWords returns all matching word entries for a given word and source language.
// Returns an empty slice (not nil) when no entries exist.
func (s *SQLiteStore) FindWords(ctx context.Context, word, sourceLang string) ([]WordRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, word, part_of_speech, article, definition, english_definition,
			example, english, target_translation, notes, connotation, register,
			collocations, contrastive_notes, secondary_meanings, tags,
			source_language, target_language, created_at, updated_at
		FROM words WHERE word = ? AND source_language = ?`,
		word, sourceLang,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]WordRow, 0)
	for rows.Next() {
		var w WordRow
		if err := rows.Scan(
			&w.ID, &w.Word, &w.PartOfSpeech, &w.Article, &w.Definition, &w.EnglishDefinition,
			&w.Example, &w.English, &w.TargetTranslation, &w.Notes, &w.Connotation, &w.Register,
			&w.Collocations, &w.ContrastiveNotes, &w.SecondaryMeanings, &w.Tags,
			&w.SourceLanguage, &w.TargetLanguage, &w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}

// InsertWord stores a new word entry with timestamps.
func (s *SQLiteStore) InsertWord(ctx context.Context, row *WordRow) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if row.CreatedAt == "" {
		row.CreatedAt = now
	}
	row.UpdatedAt = now

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO words (word, part_of_speech, article, definition, english_definition,
			example, english, target_translation, notes, connotation, register,
			collocations, contrastive_notes, secondary_meanings, tags,
			source_language, target_language, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.Word, row.PartOfSpeech, row.Article, row.Definition, row.EnglishDefinition,
		row.Example, row.English, row.TargetTranslation, row.Notes, row.Connotation, row.Register,
		row.Collocations, row.ContrastiveNotes, row.SecondaryMeanings, row.Tags,
		row.SourceLanguage, row.TargetLanguage, row.CreatedAt, row.UpdatedAt,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	row.ID = id
	return nil
}

// UpdateWord updates an existing word entry by ID.
func (s *SQLiteStore) UpdateWord(ctx context.Context, id int64, row *WordRow) error {
	row.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE words SET word=?, part_of_speech=?, article=?, definition=?, english_definition=?,
			example=?, english=?, target_translation=?, notes=?, connotation=?, register=?,
			collocations=?, contrastive_notes=?, secondary_meanings=?, tags=?,
			source_language=?, target_language=?, updated_at=?
		WHERE id=?`,
		row.Word, row.PartOfSpeech, row.Article, row.Definition, row.EnglishDefinition,
		row.Example, row.English, row.TargetTranslation, row.Notes, row.Connotation, row.Register,
		row.Collocations, row.ContrastiveNotes, row.SecondaryMeanings, row.Tags,
		row.SourceLanguage, row.TargetLanguage, row.UpdatedAt,
		id,
	)
	return err
}

// DeleteWord removes a word entry by ID.
func (s *SQLiteStore) DeleteWord(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM words WHERE id=?`, id)
	return err
}

// --- Expression CRUD ---

// FindExpression returns the first matching expression entry or nil.
func (s *SQLiteStore) FindExpression(ctx context.Context, expr, sourceLang string) (*ExpressionRow, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, expression, definition, english_definition, example, english,
			target_translation, notes, connotation, register, contrastive_notes, tags,
			source_language, target_language, created_at, updated_at
		FROM expressions WHERE expression = ? AND source_language = ? LIMIT 1`,
		expr, sourceLang,
	)
	return scanExpressionRow(row)
}

// GetExpression returns a single expression entry by ID, or nil if not found.
func (s *SQLiteStore) GetExpression(ctx context.Context, id int64) (*ExpressionRow, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, expression, definition, english_definition,
			example, english, target_translation, notes, connotation, register,
			contrastive_notes, tags,
			source_language, target_language, created_at, updated_at
		FROM expressions WHERE id = ?`,
		id,
	)
	return scanExpressionRow(row)
}

// FindExpressions returns all matching expression entries.
// Returns an empty slice (not nil) when no entries exist.
func (s *SQLiteStore) FindExpressions(ctx context.Context, expr, sourceLang string) ([]ExpressionRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, expression, definition, english_definition, example, english,
			target_translation, notes, connotation, register, contrastive_notes, tags,
			source_language, target_language, created_at, updated_at
		FROM expressions WHERE expression = ? AND source_language = ?`,
		expr, sourceLang,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]ExpressionRow, 0)
	for rows.Next() {
		var e ExpressionRow
		if err := rows.Scan(
			&e.ID, &e.Expression, &e.Definition, &e.EnglishDefinition, &e.Example, &e.English,
			&e.TargetTranslation, &e.Notes, &e.Connotation, &e.Register, &e.ContrastiveNotes, &e.Tags,
			&e.SourceLanguage, &e.TargetLanguage, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// InsertExpression stores a new expression entry with timestamps.
func (s *SQLiteStore) InsertExpression(ctx context.Context, row *ExpressionRow) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if row.CreatedAt == "" {
		row.CreatedAt = now
	}
	row.UpdatedAt = now

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO expressions (expression, definition, english_definition, example, english,
			target_translation, notes, connotation, register, contrastive_notes, tags,
			source_language, target_language, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.Expression, row.Definition, row.EnglishDefinition, row.Example, row.English,
		row.TargetTranslation, row.Notes, row.Connotation, row.Register, row.ContrastiveNotes, row.Tags,
		row.SourceLanguage, row.TargetLanguage, row.CreatedAt, row.UpdatedAt,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	row.ID = id
	return nil
}

// UpdateExpression updates an existing expression entry by ID.
func (s *SQLiteStore) UpdateExpression(ctx context.Context, id int64, row *ExpressionRow) error {
	row.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`UPDATE expressions SET expression=?, definition=?, english_definition=?, example=?, english=?,
			target_translation=?, notes=?, connotation=?, register=?, contrastive_notes=?, tags=?,
			source_language=?, target_language=?, updated_at=?
		WHERE id=?`,
		row.Expression, row.Definition, row.EnglishDefinition, row.Example, row.English,
		row.TargetTranslation, row.Notes, row.Connotation, row.Register, row.ContrastiveNotes, row.Tags,
		row.SourceLanguage, row.TargetLanguage, row.UpdatedAt,
		id,
	)
	return err
}

// DeleteExpression removes an expression entry by ID.
func (s *SQLiteStore) DeleteExpression(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM expressions WHERE id=?`, id)
	return err
}

// DeleteWords removes multiple word entries by their IDs using a single query
// with parameterized placeholders.
func (s *SQLiteStore) DeleteWords(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(`DELETE FROM words WHERE id IN (%s)`, strings.Join(placeholders, ","))
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

// DeleteExpressions removes multiple expression entries by their IDs using a
// single query with parameterized placeholders.
func (s *SQLiteStore) DeleteExpressions(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf(`DELETE FROM expressions WHERE id IN (%s)`, strings.Join(placeholders, ","))
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

// --- List with pagination and filtering ---

// ListWords returns paginated word entries with optional filters.
func (s *SQLiteStore) ListWords(ctx context.Context, filter ListFilter) ([]WordRow, int, error) {
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}

	where, args := buildWordFilter(filter)

	// Count total.
	var total int
	countQuery := "SELECT COUNT(*) FROM words" + where
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch page.
	offset := (page - 1) * pageSize
	dataQuery := `SELECT id, word, part_of_speech, article, definition, english_definition,
		example, english, target_translation, notes, connotation, register,
		collocations, contrastive_notes, secondary_meanings, tags,
		source_language, target_language, created_at, updated_at
		FROM words` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	dataArgs := make([]any, len(args), len(args)+2)
	copy(dataArgs, args)
	dataArgs = append(dataArgs, pageSize, offset)

	rows, err := s.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]WordRow, 0)
	for rows.Next() {
		var w WordRow
		if err := rows.Scan(
			&w.ID, &w.Word, &w.PartOfSpeech, &w.Article, &w.Definition, &w.EnglishDefinition,
			&w.Example, &w.English, &w.TargetTranslation, &w.Notes, &w.Connotation, &w.Register,
			&w.Collocations, &w.ContrastiveNotes, &w.SecondaryMeanings, &w.Tags,
			&w.SourceLanguage, &w.TargetLanguage, &w.CreatedAt, &w.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, w)
	}
	return result, total, rows.Err()
}

// ListExpressions returns paginated expression entries with optional filters.
func (s *SQLiteStore) ListExpressions(ctx context.Context, filter ListFilter) ([]ExpressionRow, int, error) {
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	page := filter.Page
	if page <= 0 {
		page = 1
	}

	where, args := buildExpressionFilter(filter)

	var total int
	countQuery := "SELECT COUNT(*) FROM expressions" + where
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	dataQuery := `SELECT id, expression, definition, english_definition, example, english,
		target_translation, notes, connotation, register, contrastive_notes, tags,
		source_language, target_language, created_at, updated_at
		FROM expressions` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	dataArgs := make([]any, len(args), len(args)+2)
	copy(dataArgs, args)
	dataArgs = append(dataArgs, pageSize, offset)

	rows, err := s.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]ExpressionRow, 0)
	for rows.Next() {
		var e ExpressionRow
		if err := rows.Scan(
			&e.ID, &e.Expression, &e.Definition, &e.EnglishDefinition, &e.Example, &e.English,
			&e.TargetTranslation, &e.Notes, &e.Connotation, &e.Register, &e.ContrastiveNotes, &e.Tags,
			&e.SourceLanguage, &e.TargetLanguage, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, e)
	}
	return result, total, rows.Err()
}

// --- Import ---

// ImportWords bulk-inserts word rows, skipping duplicates (same word + source_language).
func (s *SQLiteStore) ImportWords(ctx context.Context, rows []WordRow) (imported, skipped, failed int, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339)

	for i := range rows {
		// Check for existing entry.
		var count int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM words WHERE word = ? AND source_language = ?`,
			rows[i].Word, rows[i].SourceLanguage,
		).Scan(&count); err != nil {
			failed++
			continue
		}
		if count > 0 {
			skipped++
			continue
		}

		if rows[i].CreatedAt == "" {
			rows[i].CreatedAt = now
		}
		rows[i].UpdatedAt = now

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO words (word, part_of_speech, article, definition, english_definition,
				example, english, target_translation, notes, connotation, register,
				collocations, contrastive_notes, secondary_meanings, tags,
				source_language, target_language, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			rows[i].Word, rows[i].PartOfSpeech, rows[i].Article, rows[i].Definition, rows[i].EnglishDefinition,
			rows[i].Example, rows[i].English, rows[i].TargetTranslation, rows[i].Notes, rows[i].Connotation, rows[i].Register,
			rows[i].Collocations, rows[i].ContrastiveNotes, rows[i].SecondaryMeanings, rows[i].Tags,
			rows[i].SourceLanguage, rows[i].TargetLanguage, rows[i].CreatedAt, rows[i].UpdatedAt,
		); err != nil {
			failed++
			continue
		}
		imported++
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, len(rows), err
	}
	return imported, skipped, failed, nil
}

// ImportExpressions bulk-inserts expression rows, skipping duplicates.
func (s *SQLiteStore) ImportExpressions(ctx context.Context, rows []ExpressionRow) (imported, skipped, failed int, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339)

	for i := range rows {
		var count int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM expressions WHERE expression = ? AND source_language = ?`,
			rows[i].Expression, rows[i].SourceLanguage,
		).Scan(&count); err != nil {
			failed++
			continue
		}
		if count > 0 {
			skipped++
			continue
		}

		if rows[i].CreatedAt == "" {
			rows[i].CreatedAt = now
		}
		rows[i].UpdatedAt = now

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO expressions (expression, definition, english_definition, example, english,
				target_translation, notes, connotation, register, contrastive_notes, tags,
				source_language, target_language, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			rows[i].Expression, rows[i].Definition, rows[i].EnglishDefinition, rows[i].Example, rows[i].English,
			rows[i].TargetTranslation, rows[i].Notes, rows[i].Connotation, rows[i].Register, rows[i].ContrastiveNotes, rows[i].Tags,
			rows[i].SourceLanguage, rows[i].TargetLanguage, rows[i].CreatedAt, rows[i].UpdatedAt,
		); err != nil {
			failed++
			continue
		}
		imported++
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, len(rows), err
	}
	return imported, skipped, failed, nil
}

// --- Backup and Restore ---

// BackupTo copies the SQLite file to destPath.
func (s *SQLiteStore) BackupTo(ctx context.Context, destPath string) error {
	slog.Info("creating database backup", slog.String("dest", destPath))
	// Checkpoint WAL to ensure all data is in the main file.
	if _, err := s.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("wal checkpoint: %w", err)
	}

	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}

	src, err := os.Open(s.dbPath)
	if err != nil {
		return fmt.Errorf("open source db: %w", err)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy database: %w", err)
	}
	return dst.Sync()
}

// RestoreFrom replaces the current database with the file at srcPath.
// Creates a backup of the current DB before overwriting.
func (s *SQLiteStore) RestoreFrom(ctx context.Context, srcPath string) error {
	slog.Info("restoring database", slog.String("from", srcPath))
	// Verify the backup is a valid SQLite file by opening it.
	testDB, err := sql.Open("sqlite", srcPath)
	if err != nil {
		return fmt.Errorf("open backup file: %w", err)
	}
	if err := testDB.Ping(); err != nil {
		_ = testDB.Close()
		return fmt.Errorf("invalid SQLite backup: %w", err)
	}
	_ = testDB.Close()

	// Create a safety backup of the current DB.
	safetyPath := s.dbPath + ".pre-restore." + time.Now().UTC().Format("20060102T150405Z")
	if err := s.BackupTo(ctx, safetyPath); err != nil {
		return fmt.Errorf("safety backup: %w", err)
	}

	// Close current connection.
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close current db: %w", err)
	}

	// Copy backup over current DB.
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer func() { _ = src.Close() }()

	dst, err := os.Create(s.dbPath)
	if err != nil {
		return fmt.Errorf("overwrite db: %w", err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy backup: %w", err)
	}
	if err := dst.Sync(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	// Reopen the database.
	newDB, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return fmt.Errorf("reopen database: %w", err)
	}
	s.db = newDB
	return nil
}

// --- Scan helpers ---

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// scanWordRow scans a single word row from a query result.
func scanWordRow(row scanner) (*WordRow, error) {
	var w WordRow
	err := row.Scan(
		&w.ID, &w.Word, &w.PartOfSpeech, &w.Article, &w.Definition, &w.EnglishDefinition,
		&w.Example, &w.English, &w.TargetTranslation, &w.Notes, &w.Connotation, &w.Register,
		&w.Collocations, &w.ContrastiveNotes, &w.SecondaryMeanings, &w.Tags,
		&w.SourceLanguage, &w.TargetLanguage, &w.CreatedAt, &w.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// scanExpressionRow scans a single expression row from a query result.
func scanExpressionRow(row scanner) (*ExpressionRow, error) {
	var e ExpressionRow
	err := row.Scan(
		&e.ID, &e.Expression, &e.Definition, &e.EnglishDefinition, &e.Example, &e.English,
		&e.TargetTranslation, &e.Notes, &e.Connotation, &e.Register, &e.ContrastiveNotes, &e.Tags,
		&e.SourceLanguage, &e.TargetLanguage, &e.CreatedAt, &e.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// --- Filter builders ---

// buildWordFilter constructs a WHERE clause and args for word list queries.
func buildWordFilter(f ListFilter) (string, []any) {
	var clauses []string
	var args []any

	if f.SourceLang != "" {
		clauses = append(clauses, "source_language = ?")
		args = append(args, f.SourceLang)
	}
	if f.TargetLang != "" {
		clauses = append(clauses, "target_language = ?")
		args = append(args, f.TargetLang)
	}
	if f.Search != "" {
		clauses = append(clauses, "(word LIKE ? OR definition LIKE ? OR english LIKE ? OR tags LIKE ?)")
		pattern := "%" + f.Search + "%"
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if f.Tags != "" {
		clauses = append(clauses, "(',' || tags || ',') LIKE ?")
		args = append(args, "%,"+f.Tags+",%")
	}

	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

// buildExpressionFilter constructs a WHERE clause and args for expression list queries.
func buildExpressionFilter(f ListFilter) (string, []any) {
	var clauses []string
	var args []any

	if f.SourceLang != "" {
		clauses = append(clauses, "source_language = ?")
		args = append(args, f.SourceLang)
	}
	if f.TargetLang != "" {
		clauses = append(clauses, "target_language = ?")
		args = append(args, f.TargetLang)
	}
	if f.Search != "" {
		clauses = append(clauses, "(expression LIKE ? OR definition LIKE ? OR english LIKE ? OR tags LIKE ?)")
		pattern := "%" + f.Search + "%"
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if f.Tags != "" {
		clauses = append(clauses, "(',' || tags || ',') LIKE ?")
		args = append(args, "%,"+f.Tags+",%")
	}

	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}
