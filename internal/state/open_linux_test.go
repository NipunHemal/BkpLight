package state

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bkplight.local/bkplight/internal/safeerr"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), Options{Directory: t.TempDir(), Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPragmasAndRuntimeVersion(t *testing.T) {
	s := openTestStore(t)
	for _, check := range []struct{ query, want string }{
		{"SELECT sqlite_version()", "3.54.1"},
		{"PRAGMA foreign_keys", "1"}, {"PRAGMA journal_mode", "wal"},
		{"PRAGMA synchronous", "2"}, {"PRAGMA busy_timeout", "1000"},
	} {
		var got string
		if err := s.db.QueryRow(check.query).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != check.want {
			t.Errorf("%s = %s, want %s", check.query, got, check.want)
		}
	}
	if _, err := s.db.Exec("INSERT INTO active_config VALUES(1,'missing')"); err == nil {
		t.Fatal("foreign key violation accepted")
	}
	// A fresh pooled connection must inherit the complete pragma policy.
	s.db.SetMaxIdleConns(0)
	for _, check := range []struct{ query, want string }{
		{"PRAGMA foreign_keys", "1"}, {"PRAGMA synchronous", "2"}, {"PRAGMA busy_timeout", "1000"},
	} {
		var got string
		if err := s.db.QueryRow(check.query).Scan(&got); err != nil || got != check.want {
			t.Fatalf("reconnected %s: %q, %v", check.query, got, err)
		}
	}
	if s.db.Stats().MaxOpenConnections != 1 {
		t.Fatal("pool permits multiple owners")
	}
}

func TestPrivateFilesAndEscapedPath(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state ?#% space")
	s, err := Open(context.Background(), Options{Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, name := range []string{"", "state.lock", "state.db", "state.db-wal", "state.db-shm"} {
		info, err := os.Lstat(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		want := os.FileMode(0600)
		if name == "" {
			want = 0700
		}
		if info.Mode().Perm() != want {
			t.Errorf("%s mode %o want %o", name, info.Mode().Perm(), want)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal("close must be idempotent", err)
	}
	if _, err := os.Stat(filepath.Join(directory, "state.lock")); err != nil {
		t.Fatal("lock inode removed", err)
	}
	reopened, err := Open(context.Background(), Options{Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	reopened.Close()
}

func TestUnsafeStatePathsRejected(t *testing.T) {
	for _, name := range []string{"directory", "ancestor", "state.lock", "state.db", "state.db-wal", "state.db-shm", "state.db-journal", "hardlink", "writable"} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			directory := filepath.Join(base, "state")
			outside := filepath.Join(base, "outside")
			if err := os.Mkdir(directory, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(outside, 0700); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "directory":
				os.Remove(directory)
				if err := os.Symlink(outside, directory); err != nil {
					t.Fatal(err)
				}
			case "ancestor":
				alias := filepath.Join(base, "alias")
				if err := os.Symlink(base, alias); err != nil {
					t.Fatal(err)
				}
				directory = filepath.Join(alias, "state")
			case "writable":
				if err := os.Chmod(directory, 0777); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				target := filepath.Join(outside, "file")
				os.WriteFile(target, []byte("unchanged"), 0600)
				if err := os.Link(target, filepath.Join(directory, "state.db")); err != nil {
					t.Fatal(err)
				}
			default:
				target := filepath.Join(outside, "file")
				os.WriteFile(target, []byte("unchanged"), 0600)
				if err := os.Symlink(target, filepath.Join(directory, name)); err != nil {
					t.Fatal(err)
				}
			}
			s, err := Open(context.Background(), Options{Directory: directory})
			if err == nil {
				s.Close()
				t.Fatal("unsafe state path accepted")
			}
			if b, e := os.ReadFile(filepath.Join(outside, "file")); e == nil && string(b) != "unchanged" {
				t.Fatal("outside file changed")
			}
		})
	}
}

func TestBusyDeadline(t *testing.T) {
	s := openTestStore(t)
	other, err := sql.Open("sqlite", filepath.Join(s.directory.Name(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err = other.Exec("BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer other.Exec("ROLLBACK")
	start := time.Now()
	_, err = s.db.ExecContext(context.Background(), "UPDATE health_probe SET tick=tick+1")
	var busy interface{ Code() int }
	if !errors.As(err, &busy) || busy.Code()&255 != 5 {
		t.Fatalf("expected SQLite busy, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond || elapsed < 800*time.Millisecond {
		t.Fatalf("busy timeout not bounded to policy: %v", elapsed)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start = time.Now()
	_, err = s.db.ExecContext(ctx, "UPDATE health_probe SET tick=tick+1")
	if err == nil {
		t.Fatal("write bypassed held write lock")
	}
	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
		t.Fatalf("busy wait unbounded: %v", elapsed)
	}
	if _, err = other.Exec("ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("UPDATE health_probe SET tick=tick+1"); err != nil {
		t.Fatal("connection unusable after contention", err)
	}
}

func TestOpenFailureReleasesLockAndRedacts(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "state.db"), []byte("private-canary-corrupt-database"), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(context.Background(), Options{Directory: directory})
	if err == nil {
		s.Close()
		t.Fatal("corrupt database accepted")
	}
	if safeerr.Public(err).Code != safeerr.Unavailable || strings.Contains(err.Error(), "canary") {
		t.Fatalf("unsafe error: %v", err)
	}
	// The failed Open released its descriptor, so correcting this disposable
	// fixture allows a new owner without deleting or replacing state.lock.
	if err = os.Remove(filepath.Join(directory, "state.db")); err != nil {
		t.Fatal(err)
	}
	s, err = Open(context.Background(), Options{Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
}

func TestLockReleasedAfterCrash(t *testing.T) {
	if os.Getenv("BKPLIGHT_STATE_CRASH_HELPER") == "1" {
		s, err := Open(context.Background(), Options{Directory: os.Getenv("BKPLIGHT_STATE_CRASH_DIR")})
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		if _, err = s.db.Exec("UPDATE health_probe SET tick=42"); err != nil {
			t.Fatal(err)
		}
		if _, err = s.db.Exec("BEGIN IMMEDIATE; UPDATE health_probe SET tick=99"); err != nil {
			t.Fatal(err)
		}
		ready := os.NewFile(3, "ready")
		ready.Write([]byte("ready"))
		ready.Close()
		for {
			time.Sleep(time.Second)
		}
	}
	directory := t.TempDir()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockReleasedAfterCrash$")
	cmd.Env = append(os.Environ(), "BKPLIGHT_STATE_CRASH_HELPER=1", "BKPLIGHT_STATE_CRASH_DIR="+directory)
	cmd.ExtraFiles = []*os.File{writer}
	var logs strings.Builder
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	reader.SetReadDeadline(time.Now().Add(10 * time.Second))
	ready := make([]byte, 5)
	if _, err = io.ReadFull(reader, ready); err != nil {
		t.Fatalf("helper not ready: %v", err)
	}
	if string(ready) != "ready" {
		t.Fatal("unexpected ready signal")
	}
	if err = cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err = cmd.Wait(); err == nil {
		t.Fatal("helper was not killed")
	}
	s, err := Open(context.Background(), Options{Directory: directory})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var tick int
	if err = s.db.QueryRow("SELECT tick FROM health_probe").Scan(&tick); err != nil || tick != 42 {
		t.Fatalf("crash recovery tick %d: %v", tick, err)
	}
}

func TestDriverRuntimeQualification(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version string
	if err = db.QueryRow("SELECT sqlite_version()").Scan(&version); err != nil {
		t.Fatal(err)
	}
	t.Logf("SQLite runtime: %s", version)
	if version != "3.54.1" {
		t.Fatalf("driver qualification failed: runtime %s", version)
	}
}

func TestSecondOwnerRejected(t *testing.T) {
	o := Options{Directory: t.TempDir(), Now: time.Now}
	a, err := Open(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Open(context.Background(), o)
	if err == nil {
		b.Close()
		t.Fatal("second owner accepted")
	}
}

func TestAncestorSwapDuringDirectoryAcquisition(t *testing.T) {
	base := t.TempDir()
	ancestor := filepath.Join(base, "ancestor")
	moved := filepath.Join(base, "original-ancestor")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(filepath.Join(ancestor, "state"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(outside, "state"), 0750); err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(filepath.Join(ancestor, "state"))
	if err != nil {
		t.Fatal(err)
	}
	parent, err := os.Open(ancestor)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	// The ancestor changes after acquisition and before the final directory is
	// resolved. Both targets belong to this UID; ownership checks alone cannot
	// detect the escape through the substituted ancestor.
	if err = os.Rename(ancestor, moved); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(outside, ancestor); err != nil {
		t.Fatal(err)
	}
	dir, err := acquireDirectoryAt(parent, "state")
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	acquired, err := dir.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(original, acquired) {
		t.Fatal("ancestor substitution escaped the acquired directory")
	}
	external, err := os.Stat(filepath.Join(outside, "state"))
	if err != nil {
		t.Fatal(err)
	}
	if external.Mode().Perm() != 0750 {
		t.Fatal("outside directory permissions changed")
	}
}

func TestCloseRetainsOwnershipUntilLeasesDrain(t *testing.T) {
	for _, kind := range []string{"connection", "transaction"} {
		t.Run(kind, func(t *testing.T) {
			s := openTestStore(t)
			directory := s.directory.Name()
			var release func() error
			var write func() error
			if kind == "connection" {
				conn, err := s.db.Conn(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				release = conn.Close
				write = func() error {
					_, err := conn.ExecContext(context.Background(), "UPDATE health_probe SET tick=81")
					return err
				}
			} else {
				tx, err := s.db.BeginTx(context.Background(), nil)
				if err != nil {
					t.Fatal(err)
				}
				release = tx.Commit
				write = func() error {
					_, err := tx.ExecContext(context.Background(), "UPDATE health_probe SET tick=81")
					return err
				}
				defer tx.Rollback()
			}
			defer release()
			done := make(chan error, 1)
			go func() { done <- s.Close() }()
			// Wait for shutdown to close admission. A retained lease prevents pool
			// access before shutdown, so context expiry means shutdown has not run yet.
			deadline := time.Now().Add(5 * time.Second)
			for {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
				conn, err := s.db.Conn(ctx)
				cancel()
				if conn != nil {
					conn.Close()
					t.Fatal("second pool connection admitted")
				}
				if err != nil && !errors.Is(err, context.DeadlineExceeded) {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("shutdown did not close admission")
				}
			}
			other, err := Open(context.Background(), Options{Directory: directory})
			if err == nil {
				other.Close()
				t.Fatal("second owner accepted while old lease can still write")
			}
			if err = write(); err != nil {
				t.Fatal("retained lease failed before release", err)
			}
			if err = release(); err != nil {
				t.Fatal(err)
			}
			select {
			case err = <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("Close did not finish after lease drained")
			}
			other, err = Open(context.Background(), Options{Directory: directory})
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			var tick int
			if err = other.db.QueryRow("SELECT tick FROM health_probe").Scan(&tick); err != nil || tick != 81 {
				t.Fatalf("handoff value %d: %v", tick, err)
			}
			if err = write(); err == nil {
				t.Fatal("released old lease can still write")
			}
		})
	}
}
