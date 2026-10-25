package secrets

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"bkplight.local/bkplight/internal/safeerr"
)

func put(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func resolver(t *testing.T, dir string, max int64) *Resolver {
	t.Helper()
	r, err := New([]string{dir}, max)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func requireSafeFailure(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("unsafe input accepted")
	}
	p := safeerr.Public(err)
	if p.Field != "secrets" || err.Error() != p.Error() {
		t.Fatal("error escaped safe envelope")
	}
}

func TestReadRejectsOversizeAndEscape(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, "large"), make([]byte, 65))
	r := resolver(t, dir, 64)
	for _, path := range []string{filepath.Join(dir, "large"), filepath.Join(dir, "..", "outside"), dir + "-sibling/secret", "relative", dir + "/../" + filepath.Base(dir) + "/large"} {
		_, err := r.Read(context.Background(), path)
		requireSafeFailure(t, err)
	}
}

func TestReadExactLimitPreservesBytes(t *testing.T) {
	dir := t.TempDir()
	want := append(bytes.Repeat([]byte("x"), 63), '\n')
	path := filepath.Join(dir, "secret")
	put(t, path, want)
	r := resolver(t, dir, 64)
	got, err := r.Read(context.Background(), path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("exact-limit secret changed")
	}
	put(t, path, []byte{' ', 0, '\r', '\n'})
	got, err = r.Read(context.Background(), path)
	if err != nil || !bytes.Equal(got, []byte{' ', 0, '\r', '\n'}) {
		t.Fatal("binary secret changed")
	}
}

func TestReadMissingUnreadableNonRegular(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("permission coverage requires non-root")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "unreadable")
	put(t, path, []byte("synthetic-private-value"))
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	r := resolver(t, dir, 64)
	for _, path := range []string{filepath.Join(dir, "missing"), path, dir, fifo} {
		done := make(chan error, 1)
		go func() { _, err := r.Read(context.Background(), path); done <- err }()
		select {
		case err := <-done:
			requireSafeFailure(t, err)
		case <-time.After(time.Second):
			t.Fatal("nonregular read blocked")
		}
	}
}

func TestReadSymlinkEscape(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	put(t, filepath.Join(dir, "inside"), []byte("inside-fixture"))
	put(t, filepath.Join(outside, "outside"), []byte("outside-fixture"))
	for name, target := range map[string]string{"internal": "inside", "escape": filepath.Join(outside, "outside"), "parent": "../" + filepath.Base(outside) + "/outside"} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	r := resolver(t, dir, 64)
	got, err := r.Read(context.Background(), filepath.Join(dir, "internal"))
	if err != nil || string(got) != "inside-fixture" {
		t.Fatal("confined projection rejected")
	}
	for _, name := range []string{"escape", "parent"} {
		_, err := r.Read(context.Background(), filepath.Join(dir, name))
		requireSafeFailure(t, err)
	}
}

func TestReadSymlinkSwapStaysConfined(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	put(t, filepath.Join(dir, "inside"), []byte("inside-fixture"))
	put(t, filepath.Join(outside, "outside"), []byte("outside-fixture"))
	link := filepath.Join(dir, "link")
	if err := os.Symlink("inside", link); err != nil {
		t.Fatal(err)
	}
	r := resolver(t, dir, 64)
	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			for _, target := range []string{"inside", filepath.Join(outside, "outside")} {
				tmp := filepath.Join(dir, "replacement")
				if os.Symlink(target, tmp) == nil {
					_ = os.Rename(tmp, link)
				}
			}
		}
	}()
	defer func() { close(done); wg.Wait() }()
	for i := 0; i < 250; i++ {
		got, err := r.Read(context.Background(), link)
		if err == nil && string(got) != "inside-fixture" {
			t.Fatal("symlink swap escaped root")
		}
		if err != nil {
			requireSafeFailure(t, err)
		}
	}
}

func TestCancelledRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	put(t, path, []byte("synthetic-private-value"))
	r := resolver(t, dir, 64)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := r.Read(ctx, path)
	if got != nil {
		t.Fatal("cancelled read returned bytes")
	}
	requireSafeFailure(t, err)
}

func TestRootHandleAndMultipleRoots(t *testing.T) {
	base := t.TempDir()
	root, other := filepath.Join(base, "root"), filepath.Join(base, "other")
	for _, dir := range []string{root, other} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	put(t, filepath.Join(root, "secret"), []byte("opened-root"))
	put(t, filepath.Join(other, "secret"), nil)
	r, err := New([]string{root, other}, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := os.Rename(root, filepath.Join(base, "renamed")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(root, "secret"), []byte("replacement-root"))
	got, err := r.Read(context.Background(), filepath.Join(root, "secret"))
	if err != nil || string(got) != "opened-root" {
		t.Fatal("root pathname replacement changed trusted descriptor")
	}
	got, err = r.Read(context.Background(), filepath.Join(other, "secret"))
	if err != nil || len(got) != 0 {
		t.Fatal("empty file in second root rejected")
	}
}

func TestOverlappingRootsUseSpecificBoundary(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(dir, "secret"), []byte("parent-fixture"))
	if err := os.Symlink("../secret", filepath.Join(child, "link")); err != nil {
		t.Fatal(err)
	}
	r, err := New([]string{dir, child}, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	_, err = r.Read(context.Background(), filepath.Join(child, "link"))
	requireSafeFailure(t, err)
}

func TestNewValidationAndClose(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		roots []string
		max   int64
	}{{[]string{"relative"}, 64}, {[]string{dir, filepath.Join(dir, "missing")}, 64}, {[]string{dir}, 0}, {[]string{dir}, 65537}, {[]string{dir}, -1}} {
		r, err := New(tc.roots, tc.max)
		if r != nil {
			r.Close()
			t.Fatal("invalid resolver returned")
		}
		requireSafeFailure(t, err)
	}
	put(t, filepath.Join(dir, "secret"), []byte("fixture"))
	r := resolver(t, dir, 64)
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal("close not idempotent")
	}
	_, err := r.Read(context.Background(), filepath.Join(dir, "secret"))
	requireSafeFailure(t, err)
}
