package observability

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func record() Record {
	return Record{Time: time.Date(2026, 10, 24, 1, 2, 3, 123, time.FixedZone("offset", 3600)), Level: "info", MessageCode: "service.started"}
}
func openTest(t *testing.T, o Options) *Logger {
	t.Helper()
	if o.Directory == "" {
		o.Directory = t.TempDir()
	}
	l, e := Open(o)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func TestLoggerRejectsUnsafeFields(t *testing.T) {
	l := openTest(t, Options{})
	secret := "synthetic-password-DO-NOT-LOG"
	for _, field := range []string{"message", "run", "error", "level"} {
		r := record()
		switch field {
		case "message":
			r.MessageCode = secret
		case "run":
			r.RunID = secret
		case "error":
			r.ErrorCode = secret
		case "level":
			r.Level = secret
		}
		e := l.Write(r)
		if e == nil || strings.Contains(e.Error(), secret) {
			t.Fatal("unsafe validation")
		}
	}
	b, e := os.ReadFile(filepath.Join(l.opts.Directory, "bkplight.log"))
	if e != nil {
		t.Fatal(e)
	}
	if len(b) != 0 {
		t.Fatal("rejected record was logged")
	}
	if !l.Healthy() {
		t.Fatal("validation degraded sink")
	}
}

func TestLoggerConcurrentWrites(t *testing.T) {
	l := openTest(t, Options{})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if e := l.Write(record()); e != nil {
					t.Error(e)
				}
			}
		}()
	}
	wg.Wait()
	if e := l.Close(); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(l.opts.Directory, "bkplight.log"))
	if e != nil {
		t.Fatal(e)
	}
	lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))
	if len(lines) != 400 {
		t.Fatalf("lines: %d", len(lines))
	}
	for _, line := range lines {
		var r map[string]string
		if e := json.Unmarshal(line, &r); e != nil {
			t.Fatal(e)
		}
		if r["time"] != "2026-10-24T00:02:03.000000123Z" {
			t.Fatal("time not normalized")
		}
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("synthetic-private-path") }
func (failedWriter) Close() error              { return nil }

func TestLoggerWriteFailureDegrades(t *testing.T) {
	l := openTest(t, Options{})
	stderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = writer
	t.Cleanup(func() { os.Stderr = stderr; _ = writer.Close(); _ = reader.Close() })
	_ = l.writer.Close()
	l.writer = failedWriter{}
	e := l.Write(record())
	if e == nil || strings.Contains(e.Error(), "synthetic-private-path") || l.Healthy() {
		t.Fatal("failure not safely exposed")
	}
	if l.Write(record()) == nil {
		t.Fatal("sticky failure lost")
	}
	_ = writer.Close()
	fallback, err := io.ReadAll(reader)
	if err != nil || string(fallback) != "logs.unavailable\n" {
		t.Fatal("unsafe stderr fallback")
	}
}
func TestOversizeRecordRejected(t *testing.T) {
	l := openTest(t, Options{MaxBytes: 32})
	if l.Write(record()) == nil {
		t.Fatal("oversize accepted")
	}
	if !l.Healthy() {
		t.Fatal("size rejection degraded sink")
	}
	b, _ := os.ReadFile(filepath.Join(l.opts.Directory, "bkplight.log"))
	if len(b) > 0 {
		t.Fatal("partial record")
	}
}
func TestLoggerOptionsAndLifecycle(t *testing.T) {
	for _, o := range []Options{{Directory: "relative"}, {Directory: t.TempDir(), MaxBytes: -1}, {Directory: t.TempDir(), MaxFiles: -1}, {Directory: t.TempDir(), MaxAge: -1}} {
		if l, e := Open(o); e == nil {
			_ = l.Close()
			t.Fatal("invalid options")
		}
	}
	l := openTest(t, Options{})
	if l.opts.MaxBytes != 20<<20 || l.opts.MaxFiles != 10 || l.opts.MaxAge != 14*24*time.Hour {
		t.Fatal("defaults")
	}
	if e := l.Close(); e != nil {
		t.Fatal(e)
	}
	if e := l.Close(); e != nil {
		t.Fatal(e)
	}
	if l.Write(record()) == nil {
		t.Fatal("write after close")
	}
}

var _ io.WriteCloser = failedWriter{}

func TestLoggerAllowlistAndTimestamp(t *testing.T) {
	l := openTest(t, Options{})
	for _, code := range []string{"service.started", "service.stopped", "service.degraded", "config.activated", "state.failed", "process.started", "process.finished", "process.cancelled"} {
		r := record()
		r.MessageCode = code
		r.RunID = strings.Repeat("a", 32)
		r.ErrorCode = "internal"
		if e := l.Write(r); e != nil {
			t.Fatal(e)
		}
	}
	for _, id := range []string{strings.Repeat("A", 32), strings.Repeat("a", 31), strings.Repeat("g", 32)} {
		r := record()
		r.RunID = id
		if l.Write(r) == nil {
			t.Fatal("bad run id")
		}
	}
	r := record()
	r.Time = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	if l.Write(r) == nil {
		t.Fatal("non RFC3339 timestamp accepted")
	}
}

func TestLoggerRejectsDirectorySwapBeforeChmod(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "logs")
	target := filepath.Join(parent, "other-owned")
	for _, p := range []string{dir, target} {
		if err := os.Mkdir(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	foreign := filepath.Join(target, "bkplight.00000000000000000001.log")
	if err := os.WriteFile(foreign, []byte("preserve\n"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-30 * 24 * time.Hour)
	if err := os.Chtimes(foreign, old, old); err != nil {
		t.Fatal(err)
	}
	l, err := openWithRoot(Options{Directory: dir}, func(path string) (*os.Root, error) {
		if err := os.Rename(dir, dir+".saved"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, dir); err != nil {
			t.Fatal(err)
		}
		return os.OpenRoot(path)
	})
	if err == nil {
		_ = l.Close()
		t.Error("swapped directory accepted")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0755 {
		t.Error("replacement directory chmodded")
	}
	if _, err = os.Stat(filepath.Join(target, activeName)); !os.IsNotExist(err) {
		t.Error("replacement directory received log file")
	}
	b, err := os.ReadFile(foreign)
	if err != nil || string(b) != "preserve\n" {
		t.Error("replacement directory archive pruned")
	}
}
