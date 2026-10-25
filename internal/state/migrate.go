package state

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type migration struct {
	version int
	sql     []byte
}

func loadMigrations() ([]migration, error) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return nil, unavailable()
	}
	var ms []migration
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".sql") {
			return nil, unavailable()
		}
		number, _, ok := strings.Cut(name, "_")
		if !ok {
			return nil, unavailable()
		}
		version, err := strconv.Atoi(number)
		if err != nil || version != len(ms)+1 {
			return nil, unavailable()
		}
		b, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return nil, unavailable()
		}
		ms = append(ms, migration{version: version, sql: b})
	}
	if len(ms) == 0 {
		return nil, unavailable()
	}
	return ms, nil
}

// migrate uses one owner connection for validation, a consistent pre-upgrade
// backup and one transaction containing all pending SQL, checksums and version.
func (s *Store) migrate(ctx context.Context, ms []migration) error {
	if len(ms) == 0 {
		return unavailable()
	}
	for i, m := range ms {
		if m.version != i+1 || len(m.sql) == 0 {
			return unavailable()
		}
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return unavailable()
	}
	defer conn.Close()
	var version int
	if err = conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil || version < 0 || version > len(ms) {
		return unavailable()
	}
	if version == 0 {
		var objects int
		if err = conn.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE name NOT GLOB 'sqlite_*'").Scan(&objects); err != nil || objects != 0 {
			return unavailable()
		}
	} else {
		rows, err := conn.QueryContext(ctx, "SELECT version,checksum FROM schema_migrations ORDER BY version")
		if err != nil {
			return unavailable()
		}
		count := 0
		for rows.Next() {
			var applied int
			var checksum string
			if err = rows.Scan(&applied, &checksum); err != nil || applied != count+1 || applied > version || checksum != fmt.Sprintf("%x", sha256.Sum256(ms[applied-1].sql)) {
				rows.Close()
				return unavailable()
			}
			count++
		}
		rowErr := rows.Err()
		closeErr := rows.Close()
		if rowErr != nil || closeErr != nil || count != version {
			return unavailable()
		}
	}
	if version == len(ms) {
		return nil
	}
	if version > 0 {
		if err = s.backup(ctx, conn); err != nil {
			return err
		}
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return unavailable()
	}
	defer tx.Rollback()
	for _, m := range ms[version:] {
		if _, err = tx.ExecContext(ctx, string(m.sql)); err != nil {
			return unavailable()
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version,checksum,applied_at) VALUES(?,?,?)", m.version, fmt.Sprintf("%x", sha256.Sum256(m.sql)), s.now().UTC().UnixMicro()); err != nil {
			return unavailable()
		}
	}
	if _, err = tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version=%d", len(ms))); err != nil {
		return unavailable()
	}
	if err = tx.Commit(); err != nil {
		return unavailable()
	}
	return nil
}

func (s *Store) backup(ctx context.Context, conn *sql.Conn) error {
	// Reserve an unpredictable empty 0600 file. SQLite permits VACUUM INTO an
	// existing empty file and includes committed WAL data in the resulting copy.
	file, err := os.CreateTemp(fmt.Sprintf("/proc/self/fd/%d", s.directory.Fd()), "backup-*.db")
	if err != nil {
		return unavailable()
	}
	path := file.Name()
	if err = file.Close(); err != nil {
		return unavailable()
	}
	if _, err = conn.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		// Incomplete output must never be mistaken for a recoverable backup.
		_ = os.Remove(path)
		return unavailable()
	}
	complete, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return unavailable()
	}
	syncErr := complete.Sync()
	closeErr := complete.Close()
	if syncErr != nil || closeErr != nil {
		return unavailable()
	}
	if err = s.directory.Sync(); err != nil {
		return unavailable()
	}
	return nil
}
