package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"bkplight.local/bkplight/internal/safeerr"
	"bkplight.local/bkplight/internal/state"
)

type Server struct {
	http     *http.Server
	listener *net.UnixListener
	path     string
	inode    os.FileInfo
	parentFD int
	once     sync.Once
	closeErr error
}

// Listen never takes ownership of an existing path, even an existing socket.
func Listen(socket string, b Backend) (*Server, error) {
	if !validSocket(socket) || b == nil {
		return nil, invalid()
	}
	parentFD, err := openPrivateSocketParent(filepath.Dir(socket))
	if err != nil {
		return nil, err
	}
	listener, ownedPath, inode, err := listenOwnedSocket(parentFD, filepath.Base(socket))
	if err != nil {
		syscall.Close(parentFD)
		return nil, err
	}
	s := &Server{listener: listener, path: ownedPath, inode: inode, parentFD: parentFD}
	if err := os.Chmod(ownedPath, 0600); err != nil {
		s.Close(context.Background())
		return nil, unavailable()
	}
	s.http = &http.Server{Handler: newHandler(b), ReadHeaderTimeout: 2 * time.Second, ReadTimeout: requestTimeout, WriteTimeout: requestTimeout + time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 64 << 10, ErrorLog: log.New(io.Discard, "", 0)}
	return s, nil
}

// listenOwnedSocket uses the held parent rather than its original pathname.
// A replaced ancestor therefore cannot redirect the bind into a new tree.
func listenOwnedSocket(parentFD int, name string) (*net.UnixListener, string, os.FileInfo, error) {
	ownedPath := fmt.Sprintf("/proc/self/fd/%d/%s", parentFD, name)
	// The kernel sees this path, whose prefix can exceed the configured parent.
	// Reject it before inspecting an existing entry or attempting any bind.
	if !validSocket(ownedPath) {
		return nil, "", nil, invalid()
	}
	if _, err := os.Lstat(ownedPath); err == nil {
		return nil, "", nil, &safeerr.Error{Code: safeerr.Conflict}
	} else if !os.IsNotExist(err) {
		return nil, "", nil, unavailable()
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: ownedPath, Net: "unix"})
	if err != nil {
		return nil, "", nil, unavailable()
	}
	listener.SetUnlinkOnClose(false)
	inode, err := os.Lstat(ownedPath)
	if err != nil {
		listener.Close()
		return nil, "", nil, unavailable()
	}
	return listener, ownedPath, inode, nil
}

// openPrivateSocketParent walks from root without following symlinks. Existing
// parents must already be private; only missing directories are created.
func openPrivateSocketParent(parent string) (int, error) {
	flags := syscall.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOFOLLOW | syscall.O_CLOEXEC
	fd, err := syscall.Open("/", flags, 0)
	if err != nil {
		return -1, unavailable()
	}
	for _, component := range strings.Split(strings.TrimPrefix(parent, "/"), "/") {
		if component == "" {
			continue
		}
		next, err := syscall.Openat(fd, component, flags, 0)
		if errors.Is(err, syscall.ENOENT) {
			if err = syscall.Mkdirat(fd, component, 0700); err == nil {
				next, err = syscall.Openat(fd, component, flags, 0)
			}
		}
		if err != nil {
			syscall.Close(fd)
			return -1, &safeerr.Error{Code: safeerr.Denied}
		}
		syscall.Close(fd)
		fd = next
	}
	var st syscall.Stat_t
	if syscall.Fstat(fd, &st) != nil {
		syscall.Close(fd)
		return -1, unavailable()
	}
	if st.Uid != uint32(os.Geteuid()) || st.Mode&0777 != 0700 {
		syscall.Close(fd)
		return -1, &safeerr.Error{Code: safeerr.Denied}
	}
	return fd, nil
}

func (s *Server) Serve() error {
	err := s.http.Serve(s.listener)
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	if err != nil {
		return unavailable()
	}
	return nil
}
func (s *Server) Close(ctx context.Context) error {
	s.once.Do(func() {
		if s.http != nil {
			if err := s.http.Shutdown(ctx); err != nil {
				s.http.Close()
				s.closeErr = unavailable()
			}
		} else {
			s.listener.Close()
		}
		s.listener.Close()
		// net.UnixListener's default unlink behavior is disabled. Identity, rather
		// than the pathname alone, proves that this is our created socket.
		fi, err := os.Lstat(s.path)
		if err == nil && fi.Mode()&os.ModeSocket != 0 && os.SameFile(fi, s.inode) {
			if os.Remove(s.path) != nil {
				s.closeErr = unavailable()
			}
		} else if err != nil && !os.IsNotExist(err) {
			s.closeErr = unavailable()
		}
		if syscall.Close(s.parentFD) != nil {
			s.closeErr = unavailable()
		}
	})
	return s.closeErr
}

func decodeCancel(r io.Reader) (string, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxBody+1))
	if err != nil || len(raw) > maxBody {
		return "", invalid()
	}
	var keys map[string]json.RawMessage
	if strictJSON(raw, &keys) != nil || len(keys) != 1 {
		return "", invalid()
	}
	value, ok := keys["request_id"]
	if !ok {
		return "", invalid()
	}
	var requestID string
	if json.Unmarshal(value, &requestID) != nil || !requestIDPattern.MatchString(requestID) {
		return "", invalid()
	}
	return requestID, nil
}

func newHandler(b Backend) http.Handler {
	slots := make(chan struct{}, 32)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			writeError(w, "", &safeerr.Error{Code: safeerr.Busy, Retryable: true})
			return
		}
		defer func() {
			if recover() != nil {
				writeError(w, "", internal())
			}
		}()
		handle(w, r, b)
	})
	// TimeoutHandler cancels the backend context and buffers output. A backend
	// that ignores cancellation keeps its slot until it returns.
	timeout := http.TimeoutHandler(handler, requestTimeout, `{"request_id":"","status":"error","error":{"code":"unavailable","message":"Service unavailable","retryable":true}}`)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		timeout.ServeHTTP(w, r)
	})
}

func routePath(r *http.Request) bool {
	return !r.URL.IsAbs() && r.URL.Host == "" && r.URL.RawPath == "" && r.URL.Fragment == "" && strings.HasPrefix(r.URL.Path, "/v1/") && !strings.Contains(r.URL.Path, "//") && path.Clean(r.URL.Path) == r.URL.Path && len(r.URL.Path) <= 256 && len(r.URL.RawQuery) <= 512
}

func handle(w http.ResponseWriter, r *http.Request, b Backend) {
	if !routePath(r) {
		writeError(w, "", invalid())
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeError(w, "", invalid())
		return
	}
	// Unsupported families are recognized before decoding payloads and never
	// delegate to the backend. They provide no hidden command execution path.
	family := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/"), "/")[0]
	switch family {
	case "backups", "uploads", "verifications", "restore-plans", "restores", "retention", "notifications", "checks", "deliveries", "catalog":
		writeError(w, "", &safeerr.Error{Code: safeerr.NotImplemented})
		return
	}
	if r.URL.Path == "/v1/runs" && r.Method == http.MethodPost {
		writeError(w, "", &safeerr.Error{Code: safeerr.NotImplemented})
		return
	}
	if r.Method == http.MethodGet {
		raw, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
		if err != nil || len(raw) != 0 {
			writeError(w, "", invalid())
			return
		}
		if r.URL.Path != "/v1/runs" && len(query) != 0 {
			writeError(w, "", invalid())
			return
		}
		switch r.URL.Path {
		case "/v1/health":
			health := safeHealth(b.Health(r.Context()))
			status := http.StatusOK
			if !health.Ready {
				status = http.StatusServiceUnavailable
			}
			writeData(w, status, "", "", health)
			return
		case "/v1/jobs":
			jobs, err := b.Jobs(r.Context())
			if err != nil {
				writeError(w, "", err)
				return
			}
			views := make([]JobView, 0, len(jobs))
			for _, j := range jobs {
				if !labelPattern.MatchString(j.JobID) || !labelPattern.MatchString(j.SourceID) {
					writeError(w, "", internal())
					return
				}
				views = append(views, JobView{JobID: j.JobID, SourceID: j.SourceID, Engine: "not_implemented"})
			}
			writeData(w, 200, "", "", views)
			return
		case "/v1/runs":
			limit, after, err := historyQuery(query)
			if err != nil {
				writeError(w, "", err)
				return
			}
			page, err := b.ListRuns(r.Context(), limit, after)
			if err != nil {
				writeError(w, "", err)
				return
			}
			if len(page.Items) > limit || !validCursor(page.Next) {
				writeError(w, "", internal())
				return
			}
			views := make([]RunView, 0, len(page.Items))
			for _, run := range page.Items {
				view, err := safeRun(run)
				if err != nil {
					writeError(w, "", err)
					return
				}
				views = append(views, view)
			}
			writeData(w, 200, "", "", RunPage{Items: views, Next: page.Next})
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/v1/runs/")
		if strings.HasPrefix(r.URL.Path, "/v1/runs/") && runIDPattern.MatchString(id) {
			run, err := b.GetRun(r.Context(), id)
			if err != nil {
				writeError(w, "", err)
				return
			}
			view, err := safeRun(run)
			if err != nil {
				writeError(w, "", err)
				return
			}
			writeData(w, 200, "", "", view)
			return
		}
	}
	if r.Method == http.MethodPost && len(query) == 0 && strings.HasPrefix(r.URL.Path, "/v1/runs/") && strings.HasSuffix(r.URL.Path, "/cancel") {
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/runs/"), "/cancel")
		if !runIDPattern.MatchString(id) {
			writeError(w, "", invalid())
			return
		}
		requestID, err := decodeCancel(r.Body)
		if err != nil {
			writeError(w, "", err)
			return
		}
		run, err := b.Cancel(r.Context(), id, requestID)
		if err != nil {
			writeError(w, requestID, err)
			return
		}
		view, err := safeRun(run)
		if err != nil {
			writeError(w, requestID, err)
			return
		}
		writeData(w, 200, requestID, run.ID, view)
		return
	}
	writeError(w, "", &safeerr.Error{Code: safeerr.NotFound})
}

func historyQuery(q url.Values) (int, string, error) {
	limit := 50
	for key, values := range q {
		if len(values) != 1 || (key != "limit" && key != "after") {
			return 0, "", invalid()
		}
	}
	if values, ok := q["limit"]; ok {
		n, err := strconv.Atoi(values[0])
		if err != nil || n < 1 || n > 200 {
			return 0, "", invalid()
		}
		limit = n
	}
	after := q.Get("after")
	if !validCursor(after) {
		return 0, "", invalid()
	}
	return limit, after, nil
}

func safeRun(r state.Run) (RunView, error) {
	if !runIDPattern.MatchString(r.ID) {
		return RunView{}, internal()
	}
	switch r.State {
	case "queued", "preflight", "dumping", "delivering", "succeeded", "degraded", "failed", "cancelled", "interrupted":
	default:
		return RunView{}, internal()
	}
	code := r.ErrorCode
	if code != "" {
		public := safeerr.Public(&safeerr.Error{Code: safeerr.Code(code)})
		code = string(public.Code)
	}
	return RunView{ID: r.ID, State: r.State, Version: r.Version, StartedAt: r.StartedAt.UTC(), ErrorCode: code}, nil
}
func safeHealth(h Health) Health {
	// Health labels describe components, never arbitrary diagnostic text.
	clean := func(s string) string {
		switch s {
		case "ok", "ready", "healthy", "writable", "degraded", "unavailable", "not_implemented", "maintenance":
			return s
		default:
			return "unknown"
		}
	}
	h.Scope = "foundation"
	h.State = clean(h.State)
	h.Logs = clean(h.Logs)
	h.Scheduler = "not_implemented"
	h.BackupEngine = "not_implemented"
	return h
}
func writeError(w http.ResponseWriter, requestID string, err error) {
	public := safeerr.Public(err)
	status, message := errorStatus(public.Code)
	writeEnvelope(w, status, envelope{RequestID: requestID, Status: "error", Error: &wireError{Code: public.Code, Message: message, Retryable: public.Retryable}})
}
func writeData(w http.ResponseWriter, status int, requestID, operationID string, data any) {
	raw, err := json.Marshal(data)
	if err != nil || len(raw) > maxResponse-1024 {
		writeError(w, requestID, internal())
		return
	}
	writeEnvelope(w, status, envelope{RequestID: requestID, Status: "ok", OperationID: operationID, Data: raw})
}
func writeEnvelope(w http.ResponseWriter, status int, value envelope) {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > maxResponse {
		writeError(w, "", internal())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}
