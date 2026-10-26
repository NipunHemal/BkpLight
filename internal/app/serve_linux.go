// Package app composes the local foundation; it starts no backup workers.
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"bkplight.local/bkplight/internal/config"
	"bkplight.local/bkplight/internal/control"
	"bkplight.local/bkplight/internal/observability"
	"bkplight.local/bkplight/internal/safeerr"
	"bkplight.local/bkplight/internal/secrets"
	"bkplight.local/bkplight/internal/state"
)

func unavailable() error { return &safeerr.Error{Code: safeerr.Unavailable, Retryable: true} }

func Serve(parent context.Context, configPath string) (result error) {
	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	file, err := os.Open(configPath)
	if err != nil {
		return &safeerr.Error{Code: safeerr.Invalid, Field: "config"}
	}
	c, err := config.Decode(file)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return unavailable()
	}
	if err = checkResolvedDirectories(c); err != nil {
		return err
	}
	if err = checkSecrets(ctx, c); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return unavailable()
	}
	store, err := state.Open(ctx, state.Options{Directory: c.Service.StateDir})
	if err != nil {
		return err
	}
	// Registered first, released last: the singleton outlives the HTTP drain and
	// every state/log lease. Store.Close itself waits for outstanding SQL leases.
	defer func() {
		if err := store.Close(); result == nil {
			result = err
		}
	}()
	logs, err := observability.Open(observability.Options{Directory: c.Logging.Directory, MaxBytes: c.Logging.MaxBytes, MaxFiles: c.Logging.MaxFiles, MaxAge: time.Duration(c.Logging.MaxDays) * 24 * time.Hour})
	if err != nil {
		fmt.Fprintln(os.Stderr, "logs.unavailable")
		return err
	}
	defer func() {
		if err := logs.Close(); result == nil {
			result = err
		}
	}()
	if err = store.Recover(ctx); err != nil {
		return err
	}
	if _, err = store.Activate(ctx, c); err != nil {
		return err
	}
	if err = store.ProbeWrite(ctx); err != nil {
		return err
	}
	if err = logs.Write(observability.Record{Time: time.Now(), Level: "info", MessageCode: "service.started"}); err != nil {
		return err
	}
	b := &backend{store: store, logs: logs, lastProbe: time.Now(), stateOK: true}
	b.ready.Store(true)
	defer b.ready.Store(false)
	runtime, err := claimRuntimeSocket(c.Service.SocketPath)
	if err != nil {
		return err
	}
	// The runtime claim spans recovery, bind, serving and Server.Close. State's
	// singleton is acquired first and still released last; different state dirs
	// do not permit two daemons to recover or clean up the same socket at once.
	defer func() {
		if err := runtime.Close(); result == nil {
			result = err
		}
	}()
	server, err := runtime.Listen(b)
	if err != nil {
		return err
	}
	finished := make(chan error, 1)
	go func() { finished <- server.Serve() }()
	select {
	case <-ctx.Done():
	case err = <-finished:
		finished = nil
	}
	b.ready.Store(false)
	drain, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	closeErr = server.Close(drain)
	cancel()
	if finished != nil {
		if serveErr := <-finished; err == nil {
			err = serveErr
		}
	}
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return logs.Write(observability.Record{Time: time.Now(), Level: "info", MessageCode: "service.stopped"})
}

func checkSecrets(ctx context.Context, c config.Config) error {
	refs := config.SecretRefs(c)
	if len(refs) == 0 {
		return nil
	}
	r, err := secrets.New(c.Service.SecretRoots, 64<<10)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, ref := range refs {
		data, err := r.Read(ctx, ref)
		clear(data)
		if err != nil {
			return err
		}
	}
	return nil
}

// Resolve existing ancestors without creating anything; the owning components
// subsequently acquire no-follow directory handles before mutation.
func resolved(path string) (string, error) {
	p := path
	var tail []string
	for {
		real, err := filepath.EvalSymlinks(p)
		if err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				real = filepath.Join(real, tail[i])
			}
			return real, nil
		}
		if !os.IsNotExist(err) || p == filepath.Dir(p) {
			return "", unavailable()
		}
		tail = append(tail, filepath.Base(p))
		p = filepath.Dir(p)
	}
}
func contains(a, b string) bool { return a == "/" || a == b || strings.HasPrefix(b, a+"/") }
func checkResolvedDirectories(c config.Config) error {
	var dirs []string
	for _, path := range []string{c.Service.StateDir, c.Service.StagingDir, c.Logging.Directory} {
		p, err := resolved(path)
		if err != nil {
			return err
		}
		for _, q := range dirs {
			if contains(p, q) || contains(q, p) {
				return &safeerr.Error{Code: safeerr.Invalid, Field: "directories"}
			}
		}
		dirs = append(dirs, p)
	}
	socket, err := resolved(c.Service.SocketPath)
	if err != nil {
		return err
	}
	for _, root := range c.Service.SecretRoots {
		r, err := resolved(root)
		if err != nil {
			return err
		}
		for _, p := range dirs {
			if contains(r, p) || contains(p, r) {
				return &safeerr.Error{Code: safeerr.Invalid, Field: "directories"}
			}
		}
		if contains(r, socket) {
			return &safeerr.Error{Code: safeerr.Invalid, Field: "service.socket_path"}
		}
	}
	return nil
}

// openRuntimeParent keeps the path anchored across stale-socket checks. Only
// this UID's already-private final directory is accepted; foreign modes stay
// untouched. A live listener is a conflict regardless of its state directory.
func openRuntimeParent(path string) (*os.File, error) {
	flags := syscall.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOFOLLOW | syscall.O_CLOEXEC
	fd, err := syscall.Open("/", flags, 0)
	if err != nil {
		return nil, unavailable()
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" {
			continue
		}
		next, err := syscall.Openat(fd, part, flags, 0)
		if errors.Is(err, syscall.ENOENT) {
			if err = syscall.Mkdirat(fd, part, 0700); err == nil {
				next, err = syscall.Openat(fd, part, flags, 0)
			}
		}
		syscall.Close(fd)
		if err != nil {
			return nil, unavailable()
		}
		fd = next
	}
	f := os.NewFile(uintptr(fd), path)
	var st syscall.Stat_t
	if syscall.Fstat(fd, &st) != nil || st.Uid != uint32(os.Geteuid()) || st.Mode&0777 != 0700 {
		f.Close()
		return nil, &safeerr.Error{Code: safeerr.Denied}
	}
	return f, nil
}

type runtimeSocket struct {
	parent   *os.File
	lock     *os.File
	path     string
	once     sync.Once
	closeErr error
}

func claimRuntimeSocket(path string) (_ *runtimeSocket, err error) {
	parent, err := openRuntimeParent(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	r := &runtimeSocket{parent: parent, path: path}
	defer func() {
		if err != nil {
			r.Close()
		}
	}()
	name := "." + filepath.Base(path) + ".lock"
	fd, err := syscall.Openat(int(parent.Fd()), name, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, unavailable()
	}
	r.lock = os.NewFile(uintptr(fd), name)
	info, err := r.lock.Stat()
	if err != nil {
		return nil, unavailable()
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || st.Uid != uint32(os.Geteuid()) || st.Nlink != 1 {
		return nil, &safeerr.Error{Code: safeerr.Denied}
	}
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, &safeerr.Error{Code: safeerr.Conflict}
		}
		return nil, unavailable()
	}
	// A cooperating daemon never unlinks the lock, including on shutdown. Bind
	// this acquisition to the still-present inode before trusting ownership.
	current, err := os.Lstat(fmt.Sprintf("/proc/self/fd/%d/%s", parent.Fd(), name))
	if err != nil || !os.SameFile(info, current) {
		return nil, unavailable()
	}
	return r, nil
}
func (r *runtimeSocket) Close() error {
	r.once.Do(func() {
		if r.lock != nil {
			if r.lock.Close() != nil {
				r.closeErr = unavailable()
			}
		}
		if r.parent.Close() != nil {
			r.closeErr = unavailable()
		}
	})
	return r.closeErr
}
func (r *runtimeSocket) ownedPath() string {
	return fmt.Sprintf("/proc/self/fd/%d/%s", r.parent.Fd(), filepath.Base(r.path))
}
func (r *runtimeSocket) parentMatches() bool {
	held, err := r.parent.Stat()
	if err != nil {
		return false
	}
	current, err := os.Stat(filepath.Dir(r.path))
	return err == nil && os.SameFile(held, current)
}
func (r *runtimeSocket) Listen(b control.Backend) (*control.Server, error) {
	if !r.parentMatches() {
		return nil, unavailable()
	}
	if err := r.removeStale(); err != nil {
		return nil, err
	}
	server, err := control.Listen(r.path, b)
	if err != nil {
		return nil, err
	}
	// Control keeps its own rooted directory handle. Before serving, confirm
	// its socket appeared in our claimed directory and the configured parent
	// still names that directory. A replacement aborts and closes that server.
	held, e1 := os.Lstat(r.ownedPath())
	current, e2 := os.Lstat(r.path)
	if e1 != nil || e2 != nil || !r.parentMatches() || !os.SameFile(held, current) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Close(ctx)
		return nil, unavailable()
	}
	return server, nil
}

func (r *runtimeSocket) removeStale() error {
	owned := r.ownedPath()
	info, err := os.Lstat(owned)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return unavailable()
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || info.Mode()&os.ModeSocket == 0 || st.Uid != uint32(os.Geteuid()) {
		return &safeerr.Error{Code: safeerr.Conflict}
	}
	conn, err := net.DialTimeout("unix", owned, 200*time.Millisecond)
	if err == nil {
		conn.Close()
		return &safeerr.Error{Code: safeerr.Conflict}
	}
	// Only ECONNREFUSED proves a stale filesystem socket. Timeouts, permission
	// failures and all ambiguous outcomes preserve the existing path.
	if !errors.Is(err, syscall.ECONNREFUSED) {
		return &safeerr.Error{Code: safeerr.Conflict}
	}
	after, err := os.Lstat(owned)
	if err != nil || !os.SameFile(info, after) {
		return &safeerr.Error{Code: safeerr.Conflict}
	}
	if os.Remove(owned) != nil {
		return unavailable()
	}
	return nil
}
