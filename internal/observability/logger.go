// Package observability writes a closed schema of safe local diagnostic records.
package observability

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
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
	// Reject symlink components before creating or opening the private directory.
	for p := filepath.Clean(o.Directory); ; p = filepath.Dir(p) {
		info, e := os.Lstat(p)
		if e != nil && !os.IsNotExist(e) {
			return nil, unavailable()
		}
		if e == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return nil, unavailable()
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	if e := os.MkdirAll(o.Directory, 0700); e != nil {
		return nil, unavailable()
	}
	info, e := os.Lstat(o.Directory)
	if e != nil || !privateDirectory(info) {
		return nil, unavailable()
	}
	root, e := acquire(o.Directory)
	if e != nil {
		return nil, unavailable()
	}
	// Bind the pre-open checks to the acquired directory before any mutation.
	// A writable ancestor may have replaced the configured path in between.
	directory, e := root.Open(".")
	if e != nil {
		_ = root.Close()
		return nil, unavailable()
	}
	opened, e := directory.Stat()
	if e != nil || !privateDirectory(opened) || !os.SameFile(info, opened) {
		_ = directory.Close()
		_ = root.Close()
		return nil, unavailable()
	}
	// Tighten a same-owner directory (including testing.TempDir's 0755 child).
	// Group/world-writable directories were rejected before acquiring this handle.
	e = directory.Chmod(0700)
	closeErr := directory.Close()
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
	case "service.started", "service.stopped", "service.degraded", "config.activated", "state.failed", "process.started", "process.finished", "process.cancelled":
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
