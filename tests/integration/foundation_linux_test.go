//go:build linux

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"bkplight.local/bkplight/internal/config"
	"bkplight.local/bkplight/internal/control"
	"bkplight.local/bkplight/internal/safeerr"
	"bkplight.local/bkplight/internal/state"
	"go.yaml.in/yaml/v3"
	_ "modernc.org/sqlite"
)

var binary string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("/tmp", "bkl-bin-")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "bkplight")
	cmd := exec.Command("go", "build", "-o", binary, "../../cmd/bkplight")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintln(os.Stderr, string(out), err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type fixture struct {
	dir, path, socket string
	c                 config.Config
}

func newFixture(t *testing.T, enabled bool) *fixture {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "bkl-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	raw, err := os.ReadFile("../../internal/config/testdata/valid.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := config.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c.Service.StateDir = filepath.Join(dir, "state")
	c.Service.StagingDir = filepath.Join(dir, "staging")
	c.Service.SocketPath = filepath.Join(dir, "run", "c.sock")
	c.Service.SecretRoots = []string{filepath.Join(dir, "secrets")}
	c.Logging.Directory = filepath.Join(dir, "logs")
	if enabled {
		root := c.Service.SecretRoots[0]
		if err = os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"pg", "s3", "sign"} {
			if err = os.WriteFile(filepath.Join(root, name), []byte("synthetic-fixture"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		c.Sources["pg"] = config.Source{Type: "postgres", Host: "fixture.invalid", Port: 5432, CredentialsFile: filepath.Join(root, "pg"), Databases: []string{"app"}}
		c.Destinations["s3"] = config.Destination{Type: "s3", Endpoint: "https://example.invalid", Bucket: "fixture", Prefix: "owned/", CredentialsFile: filepath.Join(root, "s3")}
		c.Jobs["hourly"] = config.Job{Source: "pg", Schedule: config.Schedule{Cron: "0 * * * *"}, Destinations: []config.JobDestination{{ID: "s3", Required: true}}, Retention: config.Retention{KeepLast: 3, MinSuccessful: 1}}
		c.Service.StagingMaxBytes = 1 << 20
		c.Service.MinFreeBytes = 1024
		c.Encryption = config.Encryption{Recipients: []string{"synthetic-public-recipient"}, SigningKeyFile: filepath.Join(root, "sign")}
	}
	f := &fixture{dir: dir, path: filepath.Join(dir, "config.yaml"), socket: c.Service.SocketPath, c: c}
	f.write(t)
	return f
}
func (f *fixture) write(t *testing.T) {
	t.Helper()
	raw, err := yaml.Marshal(f.c)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(f.path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

type daemon struct {
	cmd    *exec.Cmd
	done   chan error
	output bytes.Buffer
	waited bool
}

func launch(t *testing.T, f *fixture) *daemon {
	t.Helper()
	d := &daemon{cmd: exec.Command(binary, "serve", "--config", f.path), done: make(chan error, 1)}
	d.cmd.Stdout = &d.output
	d.cmd.Stderr = &d.output
	if err := d.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { d.done <- d.cmd.Wait() }()
	t.Cleanup(func() {
		if !d.waited {
			_ = d.cmd.Process.Kill()
			select {
			case <-d.done:
			case <-time.After(8 * time.Second):
				t.Error("daemon not reaped")
			}
		}
	})
	return d
}
func (d *daemon) wait(t *testing.T, success bool) {
	t.Helper()
	select {
	case err := <-d.done:
		d.waited = true
		if (err == nil) != success {
			t.Fatalf("exit=%v output=%s", err, d.output.String())
		}
	case <-time.After(8 * time.Second):
		t.Fatal("daemon exit timed out")
	}
}
func waitReady(t *testing.T, f *fixture, d *daemon) control.Health {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-d.done:
			d.waited = true
			t.Fatalf("early exit=%v output=%s", err, d.output.String())
		default:
		}
		raw, err := control.NewClient(f.socket).Do(context.Background(), "GET", "/v1/health", nil)
		var env struct {
			Data control.Health `json:"data"`
		}
		_ = json.Unmarshal(raw, &env)
		if err == nil && env.Data.Ready {
			return env.Data
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("readiness deadline")
	return control.Health{}
}
func request(t *testing.T, f *fixture, method, path string, body []byte) []byte {
	t.Helper()
	raw, err := control.NewClient(f.socket).Do(context.Background(), method, path, body)
	if err != nil {
		t.Fatalf("request: %v %s", err, raw)
	}
	return raw
}
func openDB(t *testing.T, f *fixture) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(f.c.Service.StateDir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return db
}
func scalar(t *testing.T, db *sql.DB, q string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestFoundationLifecycle(t *testing.T) {
	f := newFixture(t, false)
	d := launch(t, f)
	h := waitReady(t, f, d)
	if !h.Live || h.Scope != "foundation" || h.Scheduler != "not_implemented" || h.BackupEngine != "not_implemented" || h.State != "ready" || h.Logs != "ready" {
		t.Fatalf("health=%+v", h)
	}
	for path, want := range map[string]os.FileMode{f.socket: 0600, filepath.Dir(f.socket): 0700, f.c.Service.StateDir: 0700, filepath.Join(f.c.Service.StateDir, "state.db"): 0600, f.c.Logging.Directory: 0700} {
		fi, err := os.Stat(path)
		if err != nil || fi.Mode().Perm() != want {
			t.Fatalf("private mode %s %v %v", path, fi, err)
		}
	}
	for _, path := range []string{"/v1/jobs", "/v1/runs"} {
		raw := request(t, f, "GET", path, nil)
		if !bytes.Contains(raw, []byte(`[]`)) {
			t.Fatalf("expected empty collection: %s", raw)
		}
	}
	_ = d.cmd.Process.Signal(syscall.SIGTERM)
	d.wait(t, true)
	if _, err := os.Lstat(f.socket); !os.IsNotExist(err) {
		t.Fatal("owned socket retained", err)
	}
	db := openDB(t, f)
	if scalar(t, db, "SELECT count(*) FROM config_revisions") != 1 {
		t.Fatal("config not persisted")
	}
	db.Close()
	d = launch(t, f)
	waitReady(t, f, d)
	_ = d.cmd.Process.Signal(syscall.SIGINT)
	d.wait(t, true)
	db = openDB(t, f)
	if scalar(t, db, "SELECT count(*) FROM config_revisions") != 1 {
		t.Fatal("restart duplicated revision")
	}
}
func TestInvalidConfigCreatesNoState(t *testing.T) {
	f := newFixture(t, false)
	if err := os.WriteFile(f.path, []byte("version: 999\n"), 0600); err != nil {
		t.Fatal(err)
	}
	d := launch(t, f)
	d.wait(t, false)
	entries, err := os.ReadDir(f.dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("invalid startup mutated", entries, err)
	}
}
func TestEnabledMissingSecretPreventsListen(t *testing.T) {
	f := newFixture(t, true)
	if err := os.Remove(filepath.Join(f.c.Service.SecretRoots[0], "pg")); err != nil {
		t.Fatal(err)
	}
	d := launch(t, f)
	d.wait(t, false)
	for _, path := range []string{f.socket, f.c.Service.StateDir, f.c.Logging.Directory} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("missing secret startup mutated", path, err)
		}
	}
	if strings.Contains(d.output.String(), "synthetic-fixture") {
		t.Fatal("secret in output")
	}
}
func TestDaemonSingleton(t *testing.T) {
	f := newFixture(t, false)
	d := launch(t, f)
	waitReady(t, f, d)
	second := launch(t, f)
	second.wait(t, false)
	request(t, f, "GET", "/v1/health", nil)
	_ = d.cmd.Process.Signal(syscall.SIGTERM)
	d.wait(t, true)
}
func TestStaleSocketSafety(t *testing.T) {
	t.Run("live listener different state", func(t *testing.T) {
		f := newFixture(t, false)
		d := launch(t, f)
		waitReady(t, f, d)
		other := newFixture(t, false)
		other.c.Service.SocketPath = f.socket
		other.socket = f.socket
		other.write(t)
		second := launch(t, other)
		second.wait(t, false)
		request(t, f, "GET", "/v1/health", nil)
		_ = d.cmd.Process.Signal(syscall.SIGTERM)
		d.wait(t, true)
	})
	t.Run("dead socket", func(t *testing.T) {
		f := newFixture(t, false)
		d := launch(t, f)
		waitReady(t, f, d)
		_ = d.cmd.Process.Kill()
		d.wait(t, false)
		if fi, err := os.Lstat(f.socket); err != nil || fi.Mode()&os.ModeSocket == 0 {
			t.Fatal("no crash socket", err)
		}
		d = launch(t, f)
		waitReady(t, f, d)
		_ = d.cmd.Process.Signal(syscall.SIGTERM)
		d.wait(t, true)
	})
	for _, kind := range []string{"regular", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t, false)
			if err := os.Mkdir(filepath.Dir(f.socket), 0700); err != nil {
				t.Fatal(err)
			}
			if kind == "regular" {
				if err := os.WriteFile(f.socket, []byte("preserve"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Symlink(f.path, f.socket); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.Lstat(f.socket)
			d := launch(t, f)
			d.wait(t, false)
			after, err := os.Lstat(f.socket)
			if err != nil || !os.SameFile(before, after) {
				t.Fatal("foreign path replaced", err)
			}
		})
	}
}
func TestHealthDegradesOnStateWriteFailure(t *testing.T) {
	f := newFixture(t, false)
	d := launch(t, f)
	waitReady(t, f, d)
	db := openDB(t, f)
	if _, err := db.Exec(`CREATE TRIGGER fail_probe BEFORE UPDATE ON health_probe BEGIN SELECT RAISE(ABORT,'synthetic'); END`); err != nil {
		t.Fatal(err)
	}
	awaitDegraded(t, f, "state")
	raw, err := control.NewClient(f.socket).Do(context.Background(), "POST", "/v1/runs/00000000000000000000000000000001/cancel", []byte(`{"request_id":"cancel-fixture"}`))
	if safeerr.Public(err).Code != safeerr.Unavailable || scalar(t, db, "SELECT count(*) FROM run_commands") != 0 {
		t.Fatal("degraded cancel mutated", err, string(raw))
	}
	_ = d.cmd.Process.Signal(syscall.SIGTERM)
	d.wait(t, true)
}
func TestHealthProbeCoalescesConcurrentRequests(t *testing.T) {
	f := newFixture(t, false)
	d := launch(t, f)
	waitReady(t, f, d)
	db := openDB(t, f)
	before := scalar(t, db, "SELECT tick FROM health_probe")
	var wg sync.WaitGroup
	failures := make(chan error, 24)
	for range 24 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := control.NewClient(f.socket).Do(context.Background(), "GET", "/v1/health", nil)
			if err != nil {
				failures <- err
			}
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	if delta := scalar(t, db, "SELECT tick FROM health_probe") - before; delta > 1 {
		t.Fatal("health write storm", delta)
	}
	_ = d.cmd.Process.Signal(syscall.SIGTERM)
	d.wait(t, true)
}
func awaitDegraded(t *testing.T, f *fixture, component string) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		raw, _ := control.NewClient(f.socket).Do(context.Background(), "GET", "/v1/health", nil)
		var env struct {
			Data control.Health `json:"data"`
		}
		_ = json.Unmarshal(raw, &env)
		h := env.Data
		if h.Live && !h.Ready && ((component == "state" && h.State == "unavailable") || (component == "logs" && h.Logs == "unavailable")) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("health did not degrade")
}
func TestHealthDegradesOnLogFailure(t *testing.T) {
	f := newFixture(t, false)
	f.c.Logging.MaxBytes = 256
	f.write(t)
	d := launch(t, f)
	waitReady(t, f, d)
	// Force a real rotation failure without changing another component's state.
	if err := os.Chmod(f.c.Logging.Directory, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(f.c.Logging.Directory, 0700) })
	awaitDegraded(t, f, "logs")
	_ = d.cmd.Process.Signal(syscall.SIGTERM)
	d.wait(t, false)
}
func TestCrashRecoveryDoesNotReplayEffects(t *testing.T) {
	f := newFixture(t, true)
	s, err := state.Open(context.Background(), state.Options{Directory: f.c.Service.StateDir})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	raw, _ := os.ReadFile(f.path)
	c, err := config.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Activate(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	jobs, err := s.Jobs(context.Background())
	if err != nil || len(jobs) != 1 {
		t.Fatal(jobs, err)
	}
	r, _, err := s.BeginRun(context.Background(), state.Begin{ID: strings.Repeat("1", 32), JobRevisionID: jobs[0].ID, RequestID: "seed", PayloadHash: strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	r, err = s.Move(context.Background(), state.Transition{RunID: r.ID, From: "queued", To: "preflight", Version: r.Version})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Move(context.Background(), state.Transition{RunID: r.ID, From: "preflight", To: "dumping", Version: r.Version})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	d := launch(t, f)
	waitReady(t, f, d)
	raw = request(t, f, "GET", "/v1/runs/"+r.ID, nil)
	if !bytes.Contains(raw, []byte(`"interrupted"`)) {
		t.Fatal(string(raw))
	}
	_ = d.cmd.Process.Kill()
	d.wait(t, false)
	d = launch(t, f)
	waitReady(t, f, d)
	_ = d.cmd.Process.Signal(syscall.SIGTERM)
	d.wait(t, true)
	db := openDB(t, f)
	if scalar(t, db, "SELECT count(*) FROM events WHERE event_type='operation.interrupted'") != 1 || scalar(t, db, "SELECT count(*) FROM deliveries") != 0 || scalar(t, db, "SELECT count(*) FROM runs") != 1 {
		t.Fatal("recovery replayed effects")
	}
	if _, err = os.Lstat(f.c.Service.StagingDir); !os.IsNotExist(err) {
		t.Fatal("worker staging created", err)
	}
}
func TestNoNotificationsWithoutChannels(t *testing.T) {
	f := newFixture(t, true)
	d := launch(t, f)
	waitReady(t, f, d)
	request(t, f, "GET", "/v1/jobs", nil)
	_ = d.cmd.Process.Signal(syscall.SIGTERM)
	d.wait(t, true)
	db := openDB(t, f)
	if scalar(t, db, "SELECT count(*) FROM deliveries") != 0 || scalar(t, db, "SELECT count(*) FROM channel_revisions") != 0 || scalar(t, db, "SELECT count(*) FROM runs") != 0 {
		t.Fatal("unscheduled notification/worker effects")
	}
}
func TestUnsupportedCommandsNoMutations(t *testing.T) {
	f := newFixture(t, false)
	d := launch(t, f)
	waitReady(t, f, d)
	for _, command := range []string{"backup", "upload", "verify", "restore", "retention", "notifications"} {
		cmd := exec.Command(binary, command)
		out, err := cmd.CombinedOutput()
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 4 || !bytes.Contains(out, []byte("not_implemented")) {
			t.Fatalf("%s: %v %s", command, err, out)
		}
	}
	for _, path := range []string{"/v1/backups", "/v1/restores", "/v1/notifications"} {
		raw, err := control.NewClient(f.socket).Do(context.Background(), "POST", path, []byte(`{}`))
		if safeerr.Public(err).Code != safeerr.NotImplemented {
			t.Fatal(err, string(raw))
		}
	}
	_ = d.cmd.Process.Signal(syscall.SIGTERM)
	d.wait(t, true)
	db := openDB(t, f)
	for _, table := range []string{"runs", "events", "deliveries", "run_commands"} {
		if scalar(t, db, "SELECT count(*) FROM "+table) != 0 {
			t.Fatal("unsupported command mutated", table)
		}
	}
}
func TestNoTCPListener(t *testing.T) {
	f := newFixture(t, false)
	d := launch(t, f)
	waitReady(t, f, d)
	pid := strconv.Itoa(d.cmd.Process.Pid)
	entries, err := os.ReadDir("/proc/" + pid + "/fd")
	if err != nil {
		t.Fatal(err)
	}
	inodes := map[string]bool{}
	for _, entry := range entries {
		target, err := os.Readlink("/proc/" + pid + "/fd/" + entry.Name())
		if err == nil && strings.HasPrefix(target, "socket:[") {
			inodes[strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")] = true
		}
	}
	if len(inodes) == 0 {
		t.Fatal("no Unix descriptor")
	}
	for _, table := range []string{"tcp", "tcp6"} {
		raw, err := os.ReadFile("/proc/" + pid + "/net/" + table)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n")[1:] {
			fields := strings.Fields(line)
			if len(fields) > 9 && inodes[fields[9]] {
				t.Fatal("daemon owns TCP socket", table)
			}
		}
	}
	raw, err := os.ReadFile("/proc/" + pid + "/net/unix")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 6 && inodes[fields[6]] {
			found = true
		}
	}
	if !found {
		t.Fatal("Unix socket absent")
	}
	_ = d.cmd.Process.Signal(syscall.SIGTERM)
	d.wait(t, true)
}
