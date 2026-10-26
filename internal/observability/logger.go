// Package observability writes a closed schema of safe local diagnostic records.
package observability

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"bkplight.local/bkplight/internal/safeerr"
)

type Record struct {
	Time        time.Time
	Level       string
	MessageCode string
	RunID       string
	ErrorCode   string
}

type Options struct {
	Directory string
	MaxBytes  int64
	MaxFiles  int
	MaxAge    time.Duration
	Now       func() time.Time
}

type Logger struct {
	mu         sync.Mutex
	opts       Options
	root       logRoot
	writer     io.WriteCloser
	activeInfo os.FileInfo
	size       int64
	next       uint64
	unhealthy  bool
	closed     bool
}

// logRoot keeps filesystem operations rooted and permits private test wrappers.
type logRoot interface {
	Open(string) (*os.File, error)
	OpenFile(string, int, os.FileMode) (*os.File, error)
	Lstat(string) (os.FileInfo, error)
	Remove(string) error
	Chtimes(string, time.Time, time.Time) error
	Close() error
}

func invalid() error     { return &safeerr.Error{Code: safeerr.Invalid} }
func unavailable() error { return &safeerr.Error{Code: safeerr.Unavailable} }

func Open(o Options) (*Logger, error) {
	return openWithRoot(o, os.OpenRoot)
}

func openWithRoot(o Options, acquire func(string) (*os.Root, error)) (*Logger, error) {
	if !filepath.IsAbs(o.Directory) || o.MaxBytes < 0 || o.MaxFiles < 0 || o.MaxAge < 0 {
		return nil, invalid()
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = 20 << 20
	}
	if o.MaxFiles == 0 {
		o.MaxFiles = 10
	}
	if o.MaxAge == 0 {
		o.MaxAge = 14 * 24 * time.Hour
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	parent, e := os.Open("/")
	if e != nil {
		return nil, unavailable()
	}
	directory, e := acquireLogDirectoryAt(parent, filepath.Clean(o.Directory)[1:], syscall.Mkdirat)
	_ = parent.Close()
	if e != nil {
		return nil, unavailable()
	}
	defer directory.Close()
	info, e := directory.Stat()
	if e != nil {
		return nil, unavailable()
	}
	root, e := acquire(fmt.Sprintf("/proc/self/fd/%d", directory.Fd()))
	if e != nil {
		return nil, unavailable()
	}
	// Bind the acquired directory identity before chmod or log mutation.
	// A writable ancestor may have replaced the configured path in between.
	openedDirectory, e := root.Open(".")
	if e != nil {
		_ = root.Close()
		return nil, unavailable()
	}
	opened, e := openedDirectory.Stat()
	current, pathErr := os.Lstat(o.Directory)
	if e != nil || pathErr != nil || !privateDirectory(opened) || !os.SameFile(info, opened) || !os.SameFile(info, current) {
		_ = openedDirectory.Close()
		_ = root.Close()
		return nil, unavailable()
	}
	// Tighten a same-owner directory (including testing.TempDir's 0755 child).
	// Group/world-writable directories were rejected before acquiring this handle.
	e = openedDirectory.Chmod(0700)
	closeErr := openedDirectory.Close()
	if e != nil || closeErr != nil {
		_ = root.Close()
		return nil, unavailable()
	}
	l := &Logger{opts: o, root: root}
	if e = l.scan(); e == nil {
		e = l.openActive()
	}
	if e == nil {
		e = l.prune()
	}
	if e != nil {
		if l.writer != nil {
			_ = l.writer.Close()
		}
		_ = root.Close()
		return nil, unavailable()
	}
	return l, nil
}

// acquireLogDirectoryAt retains each no-follow descriptor until the next is
// acquired. Creation is relative to that handle even if an ancestor is moved.
// mkdir is the filesystem boundary, permitting deterministic substitution tests.
func acquireLogDirectoryAt(parent *os.File, relative string, mkdir func(int, string, uint32) error) (*os.File, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return nil, unavailable()
	}
	parts := strings.Split(relative, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, unavailable()
		}
	}
	dir := parent
	for _, part := range parts {
		flags := syscall.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOFOLLOW | syscall.O_CLOEXEC
		fd, err := syscall.Openat(int(dir.Fd()), part, flags, 0)
		if err == syscall.ENOENT {
			if makeErr := mkdir(int(dir.Fd()), part, 0700); makeErr == nil || makeErr == syscall.EEXIST {
				fd, err = syscall.Openat(int(dir.Fd()), part, flags, 0)
			}
		}
		if err != nil {
			if dir != parent {
				_ = dir.Close()
			}
			return nil, unavailable()
		}
		next := os.NewFile(uintptr(fd), part)
		if dir != parent {
			_ = dir.Close()
		}
		dir = next
	}
	info, err := dir.Stat()
	if err != nil || !privateDirectory(info) {
		_ = dir.Close()
		return nil, unavailable()
	}
	return dir, nil
}

func validRecord(r Record) bool {
	if r.Time.UTC().Year() < 0 || r.Time.UTC().Year() > 9999 {
		return false
	}
	switch r.Level {
	case "debug", "info", "warn", "error":
	default:
		return false
	}
	switch r.MessageCode {
	case "service.started", "service.stopped", "service.degraded", "service.health_checked", "config.activated", "state.failed", "process.started", "process.finished", "process.cancelled":
	default:
		return false
	}
	if r.RunID != "" {
		if len(r.RunID) != 32 {
			return false
		}
		for _, c := range r.RunID {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
				return false
			}
		}
	}
	switch safeerr.Code(r.ErrorCode) {
	case "", safeerr.Invalid, safeerr.Denied, safeerr.Conflict, safeerr.Busy, safeerr.Unavailable, safeerr.NotFound, safeerr.NotImplemented, safeerr.Internal:
	default:
		return false
	}
	return true
}

func (l *Logger) Write(r Record) error {
	if !validRecord(r) {
		return invalid()
	}
	// Closed field lengths above bound encoding independently of caller input.
	b, e := json.Marshal(struct {
		Time        string `json:"time"`
		Level       string `json:"level"`
		MessageCode string `json:"message_code"`
		RunID       string `json:"run_id,omitempty"`
		ErrorCode   string `json:"error_code,omitempty"`
	}{r.Time.UTC().Format(time.RFC3339Nano), r.Level, r.MessageCode, r.RunID, r.ErrorCode})
	if e != nil {
		return invalid()
	}
	b = append(b, '\n')
	if len(b) > 16<<10 || int64(len(b)) > l.opts.MaxBytes {
		return invalid()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.unhealthy {
		return unavailable()
	}
	if l.size > l.opts.MaxBytes-int64(len(b)) {
		if e = l.rotate(); e != nil {
			return l.fail()
		}
	}
	n, e := l.writer.Write(b)
	l.size += int64(n)
	if e != nil || n != len(b) {
		return l.fail()
	}
	if e = l.prune(); e != nil {
		return l.fail()
	}
	return nil
}

func (l *Logger) fail() error {
	if !l.unhealthy {
		l.unhealthy = true
		_, _ = io.WriteString(os.Stderr, "logs.unavailable\n")
	}
	return unavailable()
}

func (l *Logger) Healthy() bool { l.mu.Lock(); defer l.mu.Unlock(); return !l.unhealthy && !l.closed }

func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		if l.unhealthy {
			return unavailable()
		}
		return nil
	}
	l.closed = true
	// Writes are unbuffered; closing the descriptor flushes the owned writer once.
	e := l.writer.Close()
	re := l.root.Close()
	if e != nil || re != nil {
		return l.fail()
	}
	if l.unhealthy {
		return unavailable()
	}
	return nil
}
