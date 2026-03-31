package db

import "database/sql"

// Migrate runs schema migrations up to the current version.
// Each migration runs in a transaction — if it fails, the DB is unchanged.
func (s *SQLiteStore) Migrate() error {
	// Create metadata table if it doesn't exist (outside transaction, idempotent).
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS metadata (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	)`); err != nil {
		return err
	}

	version := 0
	row := s.db.QueryRow(`SELECT value FROM metadata WHERE key = 'schema_version'`)
	var v string
	if err := row.Scan(&v); err == nil {
		// Parse existing version.
		for _, c := range v {
			version = version*10 + int(c-'0')
		}
	}

	if version < 1 {
		if err := s.migrateV1(); err != nil {
			return err
		}
	}

	return nil
}

// migrateV1 creates the initial schema: words, expressions, and indexes.
func (s *SQLiteStore) migrateV1() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS words (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			word TEXT NOT NULL,
			part_of_speech TEXT,
			article TEXT,
			definition TEXT,
			english_definition TEXT,
			example TEXT,
			english TEXT,
			target_translation TEXT,
			notes TEXT,
			connotation TEXT,
			register TEXT,
			collocations TEXT,
			contrastive_notes TEXT,
			secondary_meanings TEXT,
			tags TEXT,
			source_language TEXT NOT NULL,
			target_language TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS expressions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			expression TEXT NOT NULL,
			definition TEXT,
			english_definition TEXT,
			example TEXT,
			english TEXT,
			target_translation TEXT,
			notes TEXT,
			connotation TEXT,
			register TEXT,
			contrastive_notes TEXT,
			tags TEXT,
			source_language TEXT NOT NULL,
			target_language TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_words_source_word ON words (source_language, word)`,
		`CREATE INDEX IF NOT EXISTS idx_expressions_source_expr ON expressions (source_language, expression)`,
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	// Upsert schema version.
	if _, err := tx.Exec(
		`INSERT INTO metadata (key, value) VALUES ('schema_version', '1')
		 ON CONFLICT(key) DO UPDATE SET value = '1'`,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// tableExists checks whether a table exists in the database.
func tableExists(db *sql.DB, name string) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name,
	).Scan(&count)
	return count > 0, err
}
