package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"bkplight.local/bkplight/internal/config"
	"bkplight.local/bkplight/internal/observability"
	"bkplight.local/bkplight/internal/process"
	"bkplight.local/bkplight/internal/safeerr"
	"bkplight.local/bkplight/internal/state"
	"go.yaml.in/yaml/v3"
)

func TestInvalidConfigCreatesNoState(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(file, []byte("version: 999\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Serve(context.Background(), file); err == nil {
		t.Fatal("invalid config started")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatal("invalid startup mutated filesystem")
	}
}

func appConfig(t *testing.T) config.Config {
	t.Helper()
	raw, err := os.ReadFile("../config/testdata/valid.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := config.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	c.Service.StateDir = filepath.Join(dir, "state")
	c.Service.StagingDir = filepath.Join(dir, "staging")
	c.Logging.Directory = filepath.Join(dir, "logs")
	c.Service.SecretRoots = []string{filepath.Join(dir, "secrets")}
	c.Service.SocketPath = filepath.Join(dir, "run", "c.sock")
	return c
}
func TestResolvedDirectoryOverlapPreventsStartup(t *testing.T) {
	c := appConfig(t)
	dir := filepath.Dir(c.Service.StateDir)
	if err := os.Mkdir(c.Service.StateDir, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(c.Service.StateDir, alias); err != nil {
		t.Fatal(err)
	}
	c.Logging.Directory = alias
	raw, err := yaml.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = Serve(context.Background(), path); err == nil {
		t.Fatal("resolved overlap accepted")
	}
	for _, path := range []string{filepath.Join(c.Service.StateDir, "state.db"), filepath.Join(alias, "bkplight.log"), c.Service.SocketPath} {
		if _, err = os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("overlap caused mutation", path, err)
		}
	}
}

func TestResolvedSecretRootAliasPreventsStartup(t *testing.T) {
	c := appConfig(t)
	alias := filepath.Join(filepath.Dir(c.Service.StateDir), "root-alias")
	if err := os.Symlink("/", alias); err != nil {
		t.Fatal(err)
	}
	c.Service.SecretRoots = []string{alias}
	raw, err := yaml.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(alias), "config.yaml")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err = Serve(ctx, path); err == nil {
		t.Fatal("resolved secret root accepted")
	}
	for _, path := range []string{c.Service.StateDir, c.Logging.Directory, c.Service.SocketPath} {
		if _, err = os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("resolved root alias caused mutation", path, err)
		}
	}
}
func backendFixture(t *testing.T) (*backend, config.Config) {
	t.Helper()
	c := appConfig(t)
	s, err := state.Open(context.Background(), state.Options{Directory: c.Service.StateDir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	logs, err := observability.Open(observability.Options{Directory: c.Logging.Directory})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { logs.Close() })
	b := &backend{store: s, logs: logs}
	b.ready.Store(true)
	return b, c
}
func TestHealthDegradesOnStateWriteFailure(t *testing.T) {
	b, _ := backendFixture(t)
	if h := b.Health(context.Background()); !h.Ready {
		t.Fatal(h)
	}
	if err := b.store.Close(); err != nil {
		t.Fatal(err)
	}
	b.mu.Lock()
	b.lastProbe = time.Time{}
	b.mu.Unlock()
	if h := b.Health(context.Background()); h.Ready || h.State != "unavailable" {
		t.Fatal(h)
	}
	if _, err := b.Cancel(context.Background(), strings.Repeat("1", 32), "cancel"); safeerr.Public(err).Code != safeerr.Unavailable {
		t.Fatal(err)
	}
}
func TestHealthDegradesOnLogFailure(t *testing.T) {
	b, _ := backendFixture(t)
	if err := b.logs.Close(); err != nil {
		t.Fatal(err)
	}
	if h := b.Health(context.Background()); h.Ready || h.Logs != "unavailable" {
		t.Fatal(h)
	}
	if _, err := b.Cancel(context.Background(), strings.Repeat("1", 32), "cancel"); safeerr.Public(err).Code != safeerr.Unavailable {
		t.Fatal(err)
	}
}
func TestCancelIdempotency(t *testing.T) {
	b, c := backendFixture(t)
	c.Sources["pg"] = config.Source{Type: "postgres", Host: "fixture.invalid", Port: 5432, CredentialsFile: filepath.Join(c.Service.SecretRoots[0], "pg"), Databases: []string{"app"}}
	c.Destinations["s3"] = config.Destination{Type: "s3", Endpoint: "https://example.invalid", Bucket: "fixture", Prefix: "owned/", CredentialsFile: filepath.Join(c.Service.SecretRoots[0], "s3")}
	c.Jobs["hourly"] = config.Job{Source: "pg", Schedule: config.Schedule{Cron: "0 * * * *"}, Destinations: []config.JobDestination{{ID: "s3", Required: true}}, Retention: config.Retention{KeepLast: 3, MinSuccessful: 1}}
	c.Service.StagingMaxBytes = 1 << 20
	c.Service.MinFreeBytes = 1024
	c.Encryption = config.Encryption{Recipients: []string{"synthetic-public-recipient"}, SigningKeyFile: filepath.Join(c.Service.SecretRoots[0], "sign")}
	raw, _ := yaml.Marshal(c)
	c, err := config.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.store.Activate(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	jobs, err := b.Jobs(context.Background())
	if err != nil || len(jobs) != 1 {
		t.Fatal(jobs, err)
	}
	for i := 1; i <= 2; i++ {
		_, _, err = b.store.BeginRun(context.Background(), state.Begin{ID: fmt.Sprintf("%032x", i), JobRevisionID: jobs[0].ID, RequestID: fmt.Sprint("seed", i), PayloadHash: strings.Repeat("a", 64)})
		if err != nil {
			t.Fatal(err)
		}
	}
	one, err := b.Cancel(context.Background(), fmt.Sprintf("%032x", 1), "cancel-1")
	if err != nil || one.State != "cancelled" {
		t.Fatal(one, err)
	}
	replay, err := b.Cancel(context.Background(), one.ID, "cancel-1")
	if err != nil || replay != one {
		t.Fatal(replay, err)
	}
	for _, arg := range []struct{ id, req string }{{fmt.Sprintf("%032x", 2), "cancel-1"}, {one.ID, "cancel-other"}} {
		if _, err = b.Cancel(context.Background(), arg.id, arg.req); safeerr.Public(err).Code != safeerr.Conflict {
			t.Fatal("command conflict missing", err)
		}
	}
	b.ready.Store(false)
	if _, err = b.Cancel(context.Background(), fmt.Sprintf("%032x", 2), "cancel-2"); safeerr.Public(err).Code != safeerr.Unavailable {
		t.Fatal(err)
	}
	r, err := b.GetRun(context.Background(), fmt.Sprintf("%032x", 2))
	if err != nil || r.State != "queued" {
		t.Fatal("degraded cancellation changed run", r, err)
	}
	page, err := b.ListRuns(context.Background(), 50, "")
	if err != nil || len(page.Items) != 2 {
		t.Fatal(page, err)
	}
}
func TestShutdownReapsOwnedHelper(t *testing.T) {
	// The production daemon starts no helpers. A fixture-owned helper and Serve
	// receive the same parent cancellation, exercising propagation and reaping.
	if os.Getenv("BKL_APP_HELPER") == "1" {
		if err := os.WriteFile(os.Getenv("BKL_APP_PID"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			os.Exit(7)
		}
		for {
			time.Sleep(time.Hour)
		}
	}
	dir, err := os.MkdirTemp("/tmp", "bkl-app-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	c := appConfig(t)
	c.Service.SocketPath = filepath.Join(dir, "c.sock")
	raw, _ := yaml.Marshal(c)
	path := filepath.Join(dir, "config.yaml")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	serveJoined := false
	go func() { served <- Serve(ctx, path) }()
	t.Cleanup(func() {
		cancel()
		if !serveJoined {
			select {
			case <-served:
			case <-time.After(6 * time.Second):
				t.Error("serve cleanup did not join")
			}
		}
	})
	helper := make(chan process.Result, 1)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(dir, "pid")
	helperJoined := false
	go func() {
		r, _ := (process.Runner{AllowedExecutables: []string{exe}}).Run(ctx, process.Spec{Executable: exe, Args: []string{"-test.run=^TestShutdownReapsOwnedHelper$"}, Env: []string{"BKL_APP_HELPER=1", "BKL_APP_PID=" + pidFile}, Stdout: io.Discard, Dir: dir, Grace: 20 * time.Millisecond})
		helper <- r
	}()
	t.Cleanup(func() {
		cancel()
		if !helperJoined {
			select {
			case <-helper:
			case <-time.After(6 * time.Second):
				t.Error("helper cleanup did not join")
			}
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	pid := 0
	for {
		raw, e := os.ReadFile(pidFile)
		if e == nil {
			pid, _ = strconv.Atoi(string(raw))
		}
		_, e = os.Lstat(c.Service.SocketPath)
		if e == nil && pid > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("serve/helper did not start")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-served:
		serveJoined = true
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve shutdown did not join")
	}
	select {
	case r := <-helper:
		helperJoined = true
		if !r.Cancelled {
			t.Fatal("helper was not cancelled", r)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("helper shutdown did not join")
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatal("helper PID remains", pid, err)
	}
}
