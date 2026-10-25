package state

import (
	"context"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Store exclusively owns the database and retains the singleton lock until
// every database connection has closed. Business operations live in this package.
type Store struct {
	directory, lock *os.File
	db              *sql.DB
	now             func() time.Time
	closeOnce       sync.Once
	closeErr        error
}
type Options struct {
	Directory string
	Now       func() time.Time
}

func Open(ctx context.Context, o Options) (_ *Store, err error) {
	if err = ctx.Err(); err != nil {
		return nil, unavailable()
	}
	dir, err := acquireDirectory(o.Directory)
	if err != nil {
		return nil, err
	}
	s := &Store{directory: dir, now: o.Now}
	defer func() {
		if err != nil {
			s.Close()
		}
	}()
	if s.now == nil {
		s.now = time.Now
	}
	s.lock, err = acquireLock(dir)
	if err != nil {
		return nil, err
	}
	if err = s.prepareFiles(); err != nil {
		return nil, err
	}
	// Use a descriptor-relative path for all SQLite auxiliary files. No unrelated
	// open/close of state.db is allowed once the SQLite connection is live.
	location := fmt.Sprintf("/proc/self/fd/%d/state.db", dir.Fd())
	u := url.URL{Scheme: "file", Path: location}
	q := url.Values{}
	for _, pragma := range []string{"foreign_keys(1)", "journal_mode(WAL)", "synchronous(FULL)", "busy_timeout(1000)"} {
		q.Add("_pragma", pragma)
	}
	u.RawQuery = q.Encode()
	s.db, err = sql.Open("sqlite", u.String())
	if err != nil {
		return nil, unavailable()
	}
	s.db.SetMaxOpenConns(1)
	s.db.SetMaxIdleConns(1)
	if err = s.verifyPragmas(ctx); err != nil {
		return nil, err
	}
	ms, err := loadMigrations()
	if err != nil {
		return nil, unavailable()
	}
	if err = s.migrate(ctx, ms); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) prepareFiles() error {
	base := fmt.Sprintf("/proc/self/fd/%d", s.directory.Fd())
	for _, name := range []string{"state.db", "state.db-wal", "state.db-shm", "state.db-journal"} {
		info, err := os.Lstat(filepath.Join(base, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || !privateOwned(info, false) {
			return unavailable()
		}
		// Existing state is private already; never follow or chmod an untrusted path.
		if info.Mode().Perm() != 0600 {
			return unavailable()
		}
	}
	fd, err := syscall.Openat(int(s.directory.Fd()), "state.db", syscall.O_CREAT|syscall.O_EXCL|syscall.O_WRONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err == syscall.EEXIST {
		return nil
	}
	if err != nil {
		return unavailable()
	}
	if err = syscall.Close(fd); err != nil {
		return unavailable()
	}
	return nil
}

func (s *Store) verifyPragmas(ctx context.Context) error {
	for _, check := range []struct{ query, want string }{
		{"SELECT sqlite_version()", "3.54.1"}, {"PRAGMA foreign_keys", "1"},
		{"PRAGMA journal_mode", "wal"}, {"PRAGMA synchronous", "2"}, {"PRAGMA busy_timeout", "1000"},
	} {
		var got string
		if err := s.db.QueryRowContext(ctx, check.query).Scan(&got); err != nil || got != check.want {
			return unavailable()
		}
	}
	return nil
}

func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		if s.db != nil {
			if err := s.db.Close(); err != nil {
				s.closeErr = unavailable()
			}
		}
		// flock is released by closing this descriptor. Keep the inode on disk.
		if s.lock != nil {
			if err := s.lock.Close(); err != nil {
				s.closeErr = unavailable()
			}
		}
		if s.directory != nil {
			if err := s.directory.Close(); err != nil {
				s.closeErr = unavailable()
			}
		}
	})
	return s.closeErr
}
