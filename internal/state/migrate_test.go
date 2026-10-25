package state

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFoundationSchema(t *testing.T) {
	s := openTestStore(t)
	var version, count int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 1 {
		t.Fatalf("schema version %d: %v", version, err)
	}
	if err := s.db.QueryRow("SELECT count(*) FROM schema_migrations WHERE version=1 AND length(checksum)=64").Scan(&count); err != nil || count != 1 {
		t.Fatalf("ledger: %d %v", count, err)
	}
	for _, table := range []string{"config_revisions", "active_config", "job_revisions", "runs", "run_commands", "events", "channel_revisions", "deliveries", "health_probe"} {
		if _, err := s.db.Exec("SELECT * FROM " + table + " LIMIT 0"); err != nil {
			t.Errorf("missing %s: %v", table, err)
		}
	}
}

func TestNewerSchemaRejected(t *testing.T) { rejectMutatedDatabase(t, "PRAGMA user_version=99") }
func TestMigrationChecksumMismatch(t *testing.T) {
	rejectMutatedDatabase(t, "UPDATE schema_migrations SET checksum='changed'")
}
func TestMissingMigrationLedgerRejected(t *testing.T) {
	rejectMutatedDatabase(t, "DROP TABLE schema_migrations")
}
func TestLedgerVersionMismatchRejected(t *testing.T) {
	rejectMutatedDatabase(t, "UPDATE schema_migrations SET version=2")
}
func TestZeroVersionNonemptyRejected(t *testing.T) { rejectMutatedDatabase(t, "PRAGMA user_version=0") }

func rejectMutatedDatabase(t *testing.T, statement string) {
	t.Helper()
	s := openTestStore(t)
	directory := s.directory.Name()
	if _, err := s.db.Exec(statement); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(context.Background(), Options{Directory: directory})
	if err == nil {
		reopened.Close()
		t.Fatal("incompatible database accepted")
	}
}

func TestUnknownExistingDatabaseRejected(t *testing.T) {
	for _, table := range []string{"unknown", "sqliteXapplication"} {
		t.Run(table, func(t *testing.T) {
			directory := t.TempDir()
			db, err := sql.Open("sqlite", filepath.Join(directory, "state.db"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec("CREATE TABLE " + table + "(value TEXT); INSERT INTO " + table + " VALUES('preserve')"); err != nil {
				t.Fatal(err)
			}
			db.Close()
			os.Chmod(filepath.Join(directory, "state.db"), 0600)
			s, err := Open(context.Background(), Options{Directory: directory})
			if err == nil {
				s.Close()
				t.Fatal("unrecognized database accepted")
			}
		})
	}
}

func upgrades(t *testing.T, sql string) []migration {
	t.Helper()
	ms, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	return append(ms, migration{version: 2, sql: []byte(sql)})
}

func TestMigrationBackupRestore(t *testing.T) {
	s := openTestStore(t)
	if _, err := s.db.Exec("UPDATE health_probe SET tick=73"); err != nil {
		t.Fatal(err)
	}
	if err := s.migrate(context.Background(), upgrades(t, "CREATE TABLE second(value INTEGER);")); err != nil {
		t.Fatal(err)
	}
	var version int
	s.db.QueryRow("PRAGMA user_version").Scan(&version)
	if version != 2 {
		t.Fatalf("version=%d", version)
	}
	files, err := filepath.Glob(filepath.Join(s.directory.Name(), "backup-*.db"))
	if err != nil || len(files) != 1 {
		t.Fatalf("backup files %v: %v", files, err)
	}
	info, err := os.Stat(files[0])
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("backup not private", err)
	}
	// Restore the completed backup into a separate isolated directory.
	restored := filepath.Join(t.TempDir(), "state.db")
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(restored, data, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", restored)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var tick int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 1 {
		t.Fatalf("restored version %d: %v", version, err)
	}
	if err = db.QueryRow("SELECT tick FROM health_probe").Scan(&tick); err != nil || tick != 73 {
		t.Fatalf("restored tick %d: %v", tick, err)
	}
	if _, err = db.Exec("SELECT * FROM second"); err == nil {
		t.Fatal("backup contains new schema")
	}
}

func TestMigrationRollback(t *testing.T) {
	s := openTestStore(t)
	err := s.migrate(context.Background(), upgrades(t, "CREATE TABLE partial(value TEXT); INSERT INTO missing_table VALUES(1);"))
	if err == nil {
		t.Fatal("invalid migration succeeded")
	}
	var version, count int
	s.db.QueryRow("PRAGMA user_version").Scan(&version)
	s.db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count)
	if version != 1 || count != 1 {
		t.Fatalf("partial migration persisted: version %d ledger %d", version, count)
	}
	if _, err = s.db.Exec("SELECT * FROM partial"); err == nil {
		t.Fatal("partial table survived rollback")
	}
	files, _ := filepath.Glob(filepath.Join(s.directory.Name(), "backup-*.db"))
	if len(files) != 1 {
		t.Fatal("rollback lost backup")
	}
}

func TestMigrationBackupFailure(t *testing.T) {
	s := openTestStore(t)
	if err := s.directory.Chmod(0500); err != nil {
		t.Fatal(err)
	}
	defer s.directory.Chmod(0700)
	err := s.migrate(context.Background(), upgrades(t, "CREATE TABLE second(value TEXT);"))
	if err == nil {
		t.Fatal("migration continued without backup")
	}
	var version int
	s.db.QueryRow("PRAGMA user_version").Scan(&version)
	if version != 1 {
		t.Fatal("version changed after backup failure")
	}
	if _, err = s.db.Exec("SELECT * FROM second"); err == nil {
		t.Fatal("schema changed after backup failure")
	}
}

func TestMigrationTimestamp(t *testing.T) {
	when := time.Date(2026, 10, 25, 8, 0, 0, 123456000, time.UTC)
	s, err := Open(context.Background(), Options{Directory: t.TempDir(), Now: func() time.Time { return when }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var got int64
	if err = s.db.QueryRow("SELECT applied_at FROM schema_migrations WHERE version=1").Scan(&got); err != nil || got != when.UnixMicro() {
		t.Fatalf("timestamp %d: %v", got, err)
	}
}
