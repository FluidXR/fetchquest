package manifest

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite manifest database.
type DB struct {
	db   *sql.DB
	path string
}

// Open opens (or creates) the manifest database.
func Open(configDir string) (*DB, error) {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	dbPath := filepath.Join(configDir, "manifest.db")
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// Enable WAL mode for better concurrent access
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	m := &DB{db: sqlDB, path: dbPath}
	if err := m.migrate(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return m, nil
}

// Close closes the database.
func (m *DB) Close() error {
	return m.db.Close()
}

// Path returns the path to the manifest database file.
func (m *DB) Path() string {
	return m.path
}

func (m *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_serial TEXT NOT NULL,
		remote_path TEXT NOT NULL,
		local_path TEXT NOT NULL DEFAULT '',
		size INTEGER NOT NULL DEFAULT 0,
		mtime INTEGER NOT NULL DEFAULT 0,
		sha256 TEXT NOT NULL DEFAULT '',
		pulled_at DATETIME,
		UNIQUE(device_serial, remote_path)
	);

	CREATE TABLE IF NOT EXISTS dest_syncs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_id INTEGER NOT NULL,
		destination TEXT NOT NULL,
		synced_at DATETIME NOT NULL,
		FOREIGN KEY (file_id) REFERENCES files(id),
		UNIQUE(file_id, destination)
	);

	CREATE INDEX IF NOT EXISTS idx_files_device ON files(device_serial);
	CREATE INDEX IF NOT EXISTS idx_dest_syncs_file ON dest_syncs(file_id);
	`
	if _, err := m.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
