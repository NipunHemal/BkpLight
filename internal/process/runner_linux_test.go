package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"bkplight.local/bkplight/internal/safeerr"
	"golang.org/x/sys/unix"
)

func TestProcessHelper(t *testing.T) {
	if os.Getenv("BKP_TEST_HELPER") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	switch mode {
	case "exit7":
		fmt.Fprint(os.Stderr, "synthetic-sensitive-stderr")
		os.Exit(7)
	case "stderr-flood":
		block := bytes.Repeat([]byte("x"), 4096)
		for range 2048 {
			if _, err := os.Stderr.Write(block); err != nil {
				os.Exit(91)
			}
		}
		os.Exit(0)
	case "stdout-flood":
		signal.Ignore(syscall.SIGTERM)
		block := bytes.Repeat([]byte("x"), 4096)
		for {
			_, _ = os.Stdout.Write(block)
		}
	case "echo":
		fmt.Fprint(os.Stdout, os.Args[len(os.Args)-2])
		os.Exit(0)
	case "environment":
		fmt.Fprint(os.Stdout, strings.Join(os.Environ(), "\n"))
		os.Exit(0)
	case "sleep":
		for {
			time.Sleep(time.Hour)
		}
	case "spawn-child", "spawn-exit":
		exe, err := os.Executable()
		if err != nil {
			os.Exit(92)
		}
		terms := make(chan os.Signal, 1)
		signal.Notify(terms, syscall.SIGTERM)
		child := exec.Command(exe, "-test.run=^TestProcessHelper$", "--", "sleep")
		child.Env = []string{"BKP_TEST_HELPER=1", "GORACE=atexit_sleep_ms=0"}
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if child.Start() != nil {
			os.Exit(93)
		}
		helperPID(child.Process.Pid)
		if mode == "spawn-exit" {
			os.Exit(0)
		}
		<-terms
		_ = child.Wait()
		os.Exit(0)
	case "ignore-term":
		signal.Ignore(os.Interrupt, syscall.SIGTERM)
		helperPID(os.Getpid())
		for {
			time.Sleep(time.Hour)
		}
	default:
		os.Exit(90)
	}
}

// This separate fixture permits testing a truly empty environment without
// weakening TestProcessHelper's mandatory BKP_TEST_HELPER guard.
func TestEmptyEnvironmentHelper(t *testing.T) {
	if os.Args[len(os.Args)-1] != "bkplight-empty-environment" {
		return
	}
	if len(os.Environ()) != 0 {
		os.Exit(95)
	}
	os.Exit(0)
}

// A Unix socket carries helper PIDs independently of the supervised stdout.
func helperPID(pid int) {
	path := os.Getenv("BKP_TEST_PID_SOCKET")
	if path == "" {
		return
	}
	c, err := net.Dial("unix", path)
	if err != nil {
		os.Exit(94)
	}
	_, _ = fmt.Fprintln(c, pid)
	_ = c.Close()
}

func pidChannel(t *testing.T, spec *Spec) func() int {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "bkp-pid-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	l, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(dir, "pid"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	spec.Env = append(spec.Env, "BKP_TEST_PID_SOCKET="+l.Addr().String())
	return func() int {
		t.Helper()
		_ = l.SetDeadline(time.Now().Add(3 * time.Second))
		c, err := l.AcceptUnix()
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		_ = c.SetReadDeadline(time.Now().Add(time.Second))
		var pid int
		if _, err := fmt.Fscan(c, &pid); err != nil || pid < 1 {
			t.Fatalf("helper PID: %d %v", pid, err)
		}
		return pid
	}
}

func assertGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			t.Logf("PID %d: ESRCH", pid)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("PID %d survives: %v", pid, err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func helperSpec(t *testing.T, mode string) (Runner, Spec) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Runner{AllowedExecutables: []string{exe}}, Spec{
		Executable: exe, Args: []string{"-test.run=^TestProcessHelper$", "--", mode},
		Env: []string{"BKP_TEST_HELPER=1", "GORACE=atexit_sleep_ms=0"}, Dir: t.TempDir(),
		Stdout: io.Discard, StderrLimit: 1024, Grace: 50 * time.Millisecond,
	}
}

func TestCancelReapsProcess(t *testing.T) {
	r, spec := helperSpec(t, "ignore-term")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	result, err := r.Run(ctx, spec)
	if err == nil || !result.Cancelled || time.Since(start) > 2*time.Second {
		t.Fatalf("cancellation not bounded: result=%+v err=%v elapsed=%s", result, err, time.Since(start))
	}
}

func TestCancelReapsDirectChild(t *testing.T) {
	r, spec := helperSpec(t, "ignore-term")
	readPID := pidChannel(t, &spec)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := r.Run(ctx, spec); done <- err }()
	pid := readPID()
	start := time.Now()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancel succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancel did not return")
	}
	assertGone(t, pid)
	t.Logf("cancellation and reap: %s", time.Since(start))
}

func TestNonzeroExit(t *testing.T) {
	r, spec := helperSpec(t, "exit7")
	result, err := r.Run(context.Background(), spec)
	if err == nil || result.ExitCode != 7 || result.StderrBytes != 26 || result.Cancelled {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, ok := err.(*safeerr.Error); !ok || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestStderrFloodIsBounded(t *testing.T) {
	for _, limit := range []int{0, 1024, 1 << 20} {
		t.Run(strconv.Itoa(limit), func(t *testing.T) {
			r, spec := helperSpec(t, "stderr-flood")
			spec.StderrLimit = limit
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			result, err := r.Run(ctx, spec)
			if err != nil || result.ExitCode != 0 || result.StderrBytes != 8388608 || !result.StderrTruncated {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

type failingWriter struct{ short bool }

func (w failingWriter) Write(p []byte) (int, error) {
	if w.short {
		return len(p) - 1, nil
	}
	return 0, errors.New("synthetic-sensitive-writer-error")
}

func TestStdoutFailureCancelsProducer(t *testing.T) {
	for _, short := range []bool{false, true} {
		t.Run(strconv.FormatBool(short), func(t *testing.T) {
			r, spec := helperSpec(t, "stdout-flood")
			spec.Stdout = failingWriter{short: short}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			start := time.Now()
			result, err := r.Run(ctx, spec)
			if err == nil || result.Cancelled || time.Since(start) > 2*time.Second {
				t.Fatalf("producer not stopped: %+v %v", result, err)
			}
			if _, ok := err.(*safeerr.Error); !ok || strings.Contains(err.Error(), "sensitive") {
				t.Fatalf("unsafe error: %v", err)
			}
		})
	}
}

func TestArgumentIsNotShellEvaluated(t *testing.T) {
	r, spec := helperSpec(t, "echo")
	marker := filepath.Join(spec.Dir, "must-not-exist")
	literal := "$(touch " + marker + "); ' $HOME *"
	spec.Args = []string{"-test.run=^TestProcessHelper$", "--", literal, "echo"}
	var out bytes.Buffer
	spec.Stdout = &out
	result, err := r.Run(context.Background(), spec)
	if err != nil || result.ExitCode != 0 || out.String() != literal {
		t.Fatalf("literal argument lost: %q %+v %v", out.String(), result, err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("shell side effect: %v", err)
	}
}

func TestUnapprovedExecutableRejected(t *testing.T) {
	r, spec := helperSpec(t, "echo")
	r.AllowedExecutables = nil
	_, err := r.Run(context.Background(), spec)
	if err == nil || safeerr.Public(err).Code != safeerr.Denied {
		t.Fatalf("unapproved executable: %v", err)
	}
}

func TestNoInheritedSecrets(t *testing.T) {
	t.Setenv("BKP_PROVIDER_SECRET", "synthetic-inherited-secret")
	r, spec := helperSpec(t, "environment")
	var out bytes.Buffer
	spec.Stdout = &out
	_, err := r.Run(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(out.String(), "\n")
	if len(got) != 2 || !strings.Contains(out.String(), "BKP_TEST_HELPER=1") || !strings.Contains(out.String(), "GORACE=atexit_sleep_ms=0") {
		t.Fatalf("environment not explicit: %q", out.String())
	}
}

func TestNilEnvironmentDoesNotInherit(t *testing.T) {
	t.Setenv("BKP_PROVIDER_SECRET", "synthetic-inherited-secret")
	r, spec := helperSpec(t, "unused")
	spec.Args = []string{"-test.run=^TestEmptyEnvironmentHelper$", "--", "bkplight-empty-environment"}
	spec.Env = nil
	result, err := r.Run(context.Background(), spec)
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("nil environment inherited: %+v %v", result, err)
	}
}

func TestConcurrentRunnerIsolation(t *testing.T) {
	r, first := helperSpec(t, "ignore-term")
	firstPID := pidChannel(t, &first)
	_, second := helperSpec(t, "ignore-term")
	secondPID := pidChannel(t, &second)
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	done1, done2 := make(chan error, 1), make(chan error, 1)
	go func() { _, err := r.Run(ctx1, first); done1 <- err }()
	go func() { _, err := r.Run(ctx2, second); done2 <- err }()
	pid1, pid2 := firstPID(), secondPID()
	cancel1()
	select {
	case err := <-done1:
		if err == nil {
			t.Fatal("cancel succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first teardown blocked")
	}
	assertGone(t, pid1)
	if err := syscall.Kill(pid2, 0); err != nil {
		t.Fatalf("other job was signalled: %v", err)
	}
	select {
	case err := <-done2:
		t.Fatalf("other job exited: %v", err)
	default:
	}
	cancel2()
	select {
	case err := <-done2:
		if err == nil {
			t.Fatal("cancel succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second teardown blocked")
	}
	assertGone(t, pid2)
}

func TestSpawnedChildDoesNotSurvive(t *testing.T) {
	r, spec := helperSpec(t, "spawn-child")
	readPID := pidChannel(t, &spec)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := r.Run(ctx, spec); done <- err }()
	pid := readPID()
	start := time.Now()
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancel succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("group teardown did not return")
	}
	assertGone(t, pid)
	t.Logf("descendant teardown: %s", time.Since(start))
}

func TestNormalExitCleansDescendantAndInheritedPipes(t *testing.T) {
	// Adopt this fixture's orphan so ESRCH does not depend on WSL init reaping
	// zombies. Production only owns/reaps its direct child, per the contract.
	if err := unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	defer unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 0, 0, 0, 0)
	r, spec := helperSpec(t, "spawn-exit")
	readPID := pidChannel(t, &spec)
	done := make(chan error, 1)
	start := time.Now()
	go func() { _, err := r.Run(context.Background(), spec); done <- err }()
	pid := readPID()
	reaped := false
	defer func() {
		if !reaped {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			_, _ = syscall.Wait4(pid, nil, 0, nil)
		}
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("inherited pipes blocked Run")
	}
	var status syscall.WaitStatus
	got, err := syscall.Wait4(pid, &status, syscall.WNOHANG, nil)
	if err != nil || got != pid || !status.Signaled() {
		t.Fatalf("descendant not terminated: wait=%d status=%v error=%v", got, status, err)
	}
	reaped = true
	assertGone(t, pid)
	t.Logf("normal-exit group and pipes: %s", time.Since(start))
}

func TestInvalidSpecRejected(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Spec)
	}{
		{"relative_executable", func(s *Spec) { s.Executable = "go" }},
		{"unclean_executable", func(s *Spec) {
			s.Executable = filepath.Dir(s.Executable) + "/../" + filepath.Base(filepath.Dir(s.Executable)) + "/" + filepath.Base(s.Executable)
		}},
		{"negative_stderr", func(s *Spec) { s.StderrLimit = -1 }},
		{"large_stderr", func(s *Spec) { s.StderrLimit = 1<<20 + 1 }},
		{"negative_grace", func(s *Spec) { s.Grace = -1 }},
		{"large_grace", func(s *Spec) { s.Grace = 5*time.Second + 1 }},
		{"missing_stdout", func(s *Spec) { s.Stdout = nil }},
		{"relative_dir", func(s *Spec) { s.Dir = "." }},
		{"missing_dir", func(s *Spec) { s.Dir = filepath.Join(s.Dir, "absent") }},
		{"malformed_env", func(s *Spec) { s.Env = []string{"NO_EQUALS"} }},
		{"duplicate_env", func(s *Spec) { s.Env = []string{"A=1", "A=2"} }},
		{"loader_env", func(s *Spec) { s.Env = []string{"LD_PRELOAD=/untrusted"} }},
		{"nul_arg", func(s *Spec) { s.Args = []string{"\x00"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, spec := helperSpec(t, "echo")
			tc.change(&spec)
			_, err := r.Run(context.Background(), spec)
			if err == nil || safeerr.Public(err).Code != safeerr.Invalid {
				t.Fatalf("invalid spec accepted: %v", err)
			}
		})
	}
}

func TestSymlinkExecutableAndDirectoryRejected(t *testing.T) {
	for _, which := range []string{"executable", "directory"} {
		t.Run(which, func(t *testing.T) {
			r, spec := helperSpec(t, "echo")
			alias := filepath.Join(t.TempDir(), "alias")
			target := spec.Executable
			if which == "directory" {
				target = spec.Dir
			}
			if err := os.Symlink(target, alias); err != nil {
				t.Fatal(err)
			}
			if which == "executable" {
				spec.Executable = alias
				r.AllowedExecutables = []string{alias}
			} else {
				spec.Dir = alias
			}
			_, err := r.Run(context.Background(), spec)
			if err == nil || safeerr.Public(err).Code != safeerr.Invalid {
				t.Fatalf("symlink accepted: %v", err)
			}
		})
	}
}

func TestCancelledContextDoesNotStartHelper(t *testing.T) {
	r, spec := helperSpec(t, "echo")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := r.Run(ctx, spec)
	if err == nil || !result.Cancelled || result.ExitCode != -1 {
		t.Fatalf("pre-cancelled run: %+v %v", result, err)
	}
}

func TestGraceBoundariesAccepted(t *testing.T) {
	for _, grace := range []time.Duration{0, 5 * time.Second} {
		t.Run(grace.String(), func(t *testing.T) {
			r, spec := helperSpec(t, "echo")
			spec.Grace = grace
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			result, err := r.Run(ctx, spec)
			if err == nil || safeerr.Public(err).Code != safeerr.Unavailable || !result.Cancelled {
				t.Fatalf("valid grace rejected: %+v %v", result, err)
			}
		})
	}
}

func TestStartFailureIsSafe(t *testing.T) {
	r, spec := helperSpec(t, "echo")
	broken := filepath.Join(spec.Dir, "synthetic-sensitive-executable")
	if err := os.WriteFile(broken, []byte("invalid executable format"), 0700); err != nil {
		t.Fatal(err)
	}
	spec.Executable = broken
	r.AllowedExecutables = []string{broken}
	result, err := r.Run(context.Background(), spec)
	if err == nil || result.ExitCode != -1 || safeerr.Public(err).Code != safeerr.Unavailable || strings.Contains(err.Error(), "sensitive") {
		t.Fatalf("unsafe start failure: %+v %v", result, err)
	}
}
