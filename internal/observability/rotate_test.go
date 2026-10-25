package observability

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRotationAgeAndRestart(t *testing.T) {
	dir := t.TempDir()
	clock := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	now := func() time.Time { return clock }
	o := Options{Directory: dir, MaxBytes: 128, MaxFiles: 3, MaxAge: time.Hour, Now: now}
	l := openTest(t, o)
	for i := 0; i < 3; i++ {
		if e := l.Write(record()); e != nil {
			t.Fatal(e)
		}
	}
	_ = l.Close()
	before, _ := filepath.Glob(filepath.Join(dir, "bkplight.*.log"))
	if len(before) != 2 {
		t.Fatalf("archives missing: %d; active bytes=%d", len(before), l.size)
	}
	clock = clock.Add(2 * time.Hour)
	l = openTest(t, o)
	after, _ := filepath.Glob(filepath.Join(dir, "bkplight.*.log"))
	if len(after) != 0 {
		t.Fatal("old archives retained")
	}
	if e := l.Write(record()); e != nil {
		t.Fatal(e)
	}
	after, _ = filepath.Glob(filepath.Join(dir, "bkplight.*.log"))
	if len(after) != 1 || after[0] <= before[len(before)-1] {
		t.Fatal("suffix not monotonic across restart")
	}
}

func TestRotationPreservesForeignFiles(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "target")
	if e := os.WriteFile(outside, []byte("preserve"), 0600); e != nil {
		t.Fatal(e)
	}
	foreign := []string{"notes.log", "bkplight.unknown.log", "bkplight.00000000000000000080.log"}
	for _, name := range foreign {
		if e := os.WriteFile(filepath.Join(dir, name), []byte("preserve"), 0644); e != nil {
			t.Fatal(e)
		}
	}
	link := filepath.Join(dir, "bkplight.00000000000000000081.log")
	if e := os.Symlink(outside, link); e != nil {
		t.Fatal(e)
	}
	subdir := filepath.Join(dir, "bkplight.00000000000000000082.log")
	if e := os.Mkdir(subdir, 0700); e != nil {
		t.Fatal(e)
	}
	l := openTest(t, Options{Directory: dir, MaxBytes: 128, MaxFiles: 1})
	for i := 0; i < 5; i++ {
		if e := l.Write(record()); e != nil {
			t.Fatal(e)
		}
	}
	for _, name := range foreign {
		b, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil || string(b) != "preserve" {
			t.Fatal("foreign file changed")
		}
	}
	if _, e := os.Readlink(link); e != nil {
		t.Fatal("symlink changed")
	}
	if info, e := os.Stat(subdir); e != nil || !info.IsDir() {
		t.Fatal("directory changed")
	}
}

func TestLoggerPrivateFilesAndSymlinkRejection(t *testing.T) {
	writable := t.TempDir()
	if e := os.Chmod(writable, 0777); e != nil {
		t.Fatal(e)
	}
	if l, e := Open(Options{Directory: writable}); e == nil {
		_ = l.Close()
		t.Fatal("writable directory accepted")
	}
	dir := t.TempDir()
	l := openTest(t, Options{Directory: dir, MaxBytes: 128, MaxFiles: 3})
	for i := 0; i < 3; i++ {
		if e := l.Write(record()); e != nil {
			t.Fatal(e)
		}
	}
	_ = l.Close()
	files, _ := filepath.Glob(filepath.Join(dir, "bkplight*.log"))
	for _, name := range files {
		info, e := os.Stat(name)
		if e != nil || info.Mode().Perm() != 0600 {
			t.Fatal("nonprivate log")
		}
	}
	info, e := os.Stat(dir)
	if e != nil || info.Mode().Perm() != 0700 {
		t.Fatal("nonprivate directory")
	}
	bad := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside")
	_ = os.WriteFile(target, []byte("preserve"), 0600)
	_ = os.Symlink(target, filepath.Join(bad, "bkplight.log"))
	if l, e := Open(Options{Directory: bad}); e == nil {
		_ = l.Close()
		t.Fatal("active symlink accepted")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "preserve" {
		t.Fatal("target changed")
	}
}

func TestRotationFailureDegradesAndPreservesReplacement(t *testing.T) {
	dir := t.TempDir()
	l := openTest(t, Options{Directory: dir, MaxBytes: 128})
	if e := l.Write(record()); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, activeName)
	if e := os.Rename(path, path+".saved"); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(path, []byte("preserve\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := l.Write(record()); e == nil || l.Healthy() {
		t.Fatal("replacement accepted")
	}
	b, e := os.ReadFile(path)
	if e != nil || string(b) != "preserve\n" {
		t.Fatal("replacement changed")
	}
}

func TestRotationKeepsBoundedFiles(t *testing.T) {
	dir := t.TempDir()
	now := func() time.Time { return time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC) }
	l, err := Open(Options{Directory: dir, MaxBytes: 256, MaxFiles: 3, MaxAge: 24 * time.Hour, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if err = l.Write(Record{Time: now(), Level: "info", MessageCode: "service.started"}); err != nil {
			t.Fatal(err)
		}
	}
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "bkplight*.log"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("file count: %d", len(files))
	}
	for _, name := range files {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) > 256 || !bytes.HasSuffix(b, []byte("\n")) {
			t.Fatal("invalid rotation boundary")
		}
	}
}
