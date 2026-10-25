// Package process supervises approved native tools on Linux.
package process

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"bkplight.local/bkplight/internal/safeerr"
	"golang.org/x/sys/unix"
)

// Spec is an internal native-tool contract. Executable and Dir must already be
// canonical absolute paths in trusted, operator-controlled directories. Args are
// literal arguments, never shell input. Env is the complete child environment.
// Stdout is caller-owned: Write must return, or honor caller cancellation; an
// indefinitely blocked arbitrary io.Writer cannot be interrupted by this API.
// StderrLimit is 0..1 MiB and Grace is 0..5 seconds, inclusive.
// Tools must keep descendants in their inherited process group.
type Spec struct {
	Executable  string
	Args        []string
	Env         []string
	Dir         string
	Stdout      io.Writer
	StderrLimit int
	Grace       time.Duration
}

type Result struct {
	ExitCode        int
	StderrBytes     int64
	StderrTruncated bool
	Cancelled       bool
}

type Runner struct{ AllowedExecutables []string }

func (r Runner) Run(ctx context.Context, spec Spec) (Result, error) {
	result := Result{ExitCode: -1}
	if err := r.validate(spec); err != nil {
		return result, err
	}
	if ctx.Err() != nil {
		result.Cancelled = true
		return result, processError(safeerr.Unavailable, "process.context")
	}
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return result, processError(safeerr.Unavailable, "process.pipe")
	}
	defer stdoutR.Close()
	defer stdoutW.Close()
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		return result, processError(safeerr.Unavailable, "process.pipe")
	}
	defer stderrR.Close()
	defer stderrW.Close()
	cmd := exec.Command(spec.Executable, spec.Args...)
	cmd.Env = append([]string{}, spec.Env...) // non-nil even for an empty Env
	cmd.Dir = spec.Dir
	cmd.Stdout, cmd.Stderr = stdoutW, stderrW
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if cmd.Start() != nil {
		return result, processError(safeerr.Unavailable, "process.start")
	}
	// The parent must release its write ends for EOF to follow child teardown.
	_ = stdoutW.Close()
	_ = stderrW.Close()
	pid := cmd.Process.Pid
	outDone := make(chan error, 1)
	errDone := make(chan error, 1)
	exitDone := make(chan error, 1)
	stderr := stderrCounter{limit: spec.StderrLimit}
	go func() { _, err := io.Copy(spec.Stdout, stdoutR); outDone <- err }()
	go func() { _, err := io.Copy(&stderr, stderrR); errDone <- err }()
	go func() {
		// Observe exit WITHOUT reaping. Keeping the child PID reserved until
		// the final group signal prevents PID/PGID reuse targeting another job.
		var info unix.Siginfo
		var err error
		for {
			err = unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOWAIT, nil)
			if !errors.Is(err, syscall.EINTR) {
				break
			}
		}
		exitDone <- err
	}()
	var outErr, stderrErr, exitErr error
	outPending, stderrPending, exitPending := outDone, errDone, exitDone
	running := true
	for running {
		select {
		case <-ctx.Done():
			result.Cancelled = true
			running = false
		case outErr = <-outPending:
			outPending = nil
			if outErr != nil {
				running = false
			}
		case stderrErr = <-stderrPending:
			stderrPending = nil
			if stderrErr != nil {
				running = false
			}
		case exitErr = <-exitPending:
			exitPending = nil
			running = false
		}
	}
	// This also tears down inherited pipes after an otherwise successful parent
	// exit. The unreaped direct child reserves this positive, owned group ID.
	signalErr := signalGroup(pid, syscall.SIGTERM)
	timer := time.NewTimer(spec.Grace)
	<-timer.C
	if err := signalGroup(pid, syscall.SIGKILL); err != nil {
		signalErr = err
	}
	if exitPending != nil {
		exitErr = <-exitPending
	}
	waitErr := cmd.Wait() // reaps exactly the direct child, after final signalling
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	// A descendant may have inherited descriptors. Give streams one second
	// after termination to drain, then close both read ends and join the copies.
	drain := time.NewTimer(time.Second)
	defer drain.Stop()
	drainExpired := false
	for outPending != nil || stderrPending != nil {
		select {
		case outErr = <-outPending:
			outPending = nil
		case stderrErr = <-stderrPending:
			stderrPending = nil
		case <-drain.C:
			drainExpired = true
			_ = stdoutR.Close()
			_ = stderrR.Close()
			if outPending != nil {
				outErr = <-outPending
				outPending = nil
			}
			if stderrPending != nil {
				stderrErr = <-stderrPending
				stderrPending = nil
			}
		}
	}
	result.StderrBytes = stderr.total
	result.StderrTruncated = stderr.total > int64(spec.StderrLimit)
	// Do not retain native stderr beyond the run, or expose bytes in errors.
	clear(stderr.prefix)
	result.Cancelled = result.Cancelled || ctx.Err() != nil
	if result.Cancelled {
		return result, processError(safeerr.Unavailable, "process.context")
	}
	if outErr != nil {
		return result, processError(safeerr.Unavailable, "process.stdout")
	}
	if stderrErr != nil || drainExpired {
		return result, processError(safeerr.Unavailable, "process.pipe")
	}
	if signalErr != nil || exitErr != nil || waitErr != nil || result.ExitCode != 0 {
		return result, processError(safeerr.Unavailable, "process.exit")
	}
	return result, nil
}

func (r Runner) validate(spec Spec) error {
	if !canonical(spec.Executable, false) {
		return processError(safeerr.Invalid, "process.executable")
	}
	allowed := false
	for _, path := range r.AllowedExecutables {
		if !canonical(path, false) {
			return processError(safeerr.Invalid, "process.allowed_executables")
		}
		allowed = allowed || path == spec.Executable
	}
	if !allowed {
		return processError(safeerr.Denied, "process.executable")
	}
	if !canonical(spec.Dir, true) {
		return processError(safeerr.Invalid, "process.directory")
	}
	if spec.Stdout == nil {
		return processError(safeerr.Invalid, "process.stdout")
	}
	if spec.StderrLimit < 0 || spec.StderrLimit > 1<<20 {
		return processError(safeerr.Invalid, "process.stderr_limit")
	}
	if spec.Grace < 0 || spec.Grace > 5*time.Second {
		return processError(safeerr.Invalid, "process.grace")
	}
	for _, arg := range spec.Args {
		if strings.ContainsRune(arg, 0) {
			return processError(safeerr.Invalid, "process.args")
		}
	}
	seen := make(map[string]bool, len(spec.Env))
	for _, entry := range spec.Env {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || !envKey(key) || seen[key] || strings.ContainsRune(entry, 0) || strings.HasPrefix(key, "LD_") || strings.HasPrefix(key, "DYLD_") {
			return processError(safeerr.Invalid, "process.env")
		}
		seen[key] = true
	}
	return nil
}

func canonical(path string, directory bool) bool {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return false
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil || real != path {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if directory {
		return info.IsDir()
	}
	return info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

func envKey(key string) bool {
	if key == "" {
		return false
	}
	for i, c := range []byte(key) {
		if c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}

type stderrCounter struct {
	limit  int
	total  int64
	prefix []byte
}

func (s *stderrCounter) Write(p []byte) (int, error) {
	s.total += int64(len(p))
	if remaining := s.limit - len(s.prefix); remaining > 0 {
		s.prefix = append(s.prefix, p[:min(remaining, len(p))]...)
	}
	return len(p), nil
}

func signalGroup(pid int, signal syscall.Signal) error {
	if pid <= 0 {
		return syscall.EINVAL
	}
	err := syscall.Kill(-pid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func processError(code safeerr.Code, field string) error {
	return &safeerr.Error{Code: code, Field: field}
}
