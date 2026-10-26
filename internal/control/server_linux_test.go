package control

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"bkplight.local/bkplight/internal/safeerr"
	"bkplight.local/bkplight/internal/state"
)

const testRunID = "0123456789abcdef0123456789abcdef"

func privateSocketPath(t *testing.T) string {
	t.Helper()
	parent := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(parent, "control.sock")
}

type fakeBackend struct {
	mu            sync.Mutex
	mutations     int
	calls         int
	limit         int
	after         string
	run           state.Run
	jobs          []state.JobRevision
	page          state.Page
	err           error
	health        Health
	block         <-chan struct{}
	entered       chan<- struct{}
	deadline      time.Time
	ignoreContext bool
}

func (b *fakeBackend) Jobs(ctx context.Context) ([]state.JobRevision, error) {
	if b.entered != nil {
		b.entered <- struct{}{}
	}
	if b.block != nil {
		if b.ignoreContext {
			<-b.block
		} else {
			select {
			case <-b.block:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	b.deadline, _ = ctx.Deadline()
	return b.jobs, b.err
}
func (b *fakeBackend) GetRun(context.Context, string) (state.Run, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	return b.run, b.err
}
func (b *fakeBackend) ListRuns(_ context.Context, limit int, after string) (state.Page, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	b.limit = limit
	b.after = after
	return b.page, b.err
}
func (b *fakeBackend) Cancel(_ context.Context, id, request string) (state.Run, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	if b.run.RequestID != "" && b.run.RequestID != request {
		return state.Run{}, &safeerr.Error{Code: safeerr.Conflict}
	}
	if b.run.RequestID == "" {
		b.mutations++
		b.run = state.Run{ID: id, RequestID: request, State: "cancelled", Version: 2}
	}
	return b.run, b.err
}
func (b *fakeBackend) Health(context.Context) Health { return b.health }

func TestServerUsesUnixSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "control.sock")
	s, err := Listen(path, &fakeBackend{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	go s.Serve()
	body, err := NewClient(path).Do(context.Background(), "GET", "/v1/jobs", nil)
	if err != nil || !json.Valid(body) {
		t.Fatal("local request failed", err)
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Mode().Perm() != 0600 {
		t.Fatal("socket permissions")
	}
	fi, err = os.Stat(filepath.Dir(path))
	if err != nil || fi.Mode().Perm() != 0700 {
		t.Fatal("parent permissions")
	}
}

func request(t *testing.T, b *fakeBackend, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	newHandler(b).ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestCancelRejectsDuplicateKeys(t *testing.T) {
	for _, body := range []string{`{"request_id":"one","request_id":"two"}`, `{"request_id":"one","request_id":"one"}`} {
		if _, err := decodeCancel(strings.NewReader(body)); err == nil {
			t.Fatal("ambiguous request accepted")
		}
	}
}

func TestHTTPBoundsAndUnknownFields(t *testing.T) {
	for _, body := range []string{`{}`, `[]`, `null`, `{"request_id":1}`, `{"request_id":{"nested":"one"}}`, `{"request_id":"one","unknown":"secret"}`, `{"Request_id":"one"}`, `{"request_id":"one"} {}`, `{"request_id":""}`, `{"request_id":"secret/unsafe"}`, `{"request_id":"` + strings.Repeat("a", 129) + `"}`, `{"request_id":"` + strings.Repeat("a", 65536) + `"}`} {
		b := &fakeBackend{}
		w := request(t, b, "POST", "/v1/runs/"+testRunID+"/cancel", body)
		if w.Code != http.StatusBadRequest || b.calls != 0 || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("invalid cancel accepted or reflected: code=%d calls=%d", w.Code, b.calls)
		}
	}
}

func TestHistoryLimits(t *testing.T) {
	for _, tc := range []struct {
		query         string
		status, limit int
	}{{"", 200, 50}, {"?limit=200", 200, 200}, {"?limit=0", 400, 0}, {"?limit=-1", 400, 0}, {"?limit=201", 400, 0}, {"?limit=abc", 400, 0}, {"?limit=1&limit=2", 400, 0}, {"?secret=x", 400, 0}, {"?after=bad", 400, 0}} {
		b := &fakeBackend{}
		w := request(t, b, "GET", "/v1/runs"+tc.query, "")
		if w.Code != tc.status || b.limit != tc.limit {
			t.Fatalf("query %s: code=%d limit=%d", tc.query, w.Code, b.limit)
		}
	}
}

func TestCancelRequestReplay(t *testing.T) {
	b := &fakeBackend{}
	h := newHandler(b)
	for i, body := range []string{`{"request_id":"one"}`, `{"request_id":"one"}`, `{"request_id":"two"}`} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("POST", "/v1/runs/"+testRunID+"/cancel", strings.NewReader(body)))
		want := 200
		if i == 2 {
			want = 409
		}
		if w.Code != want {
			t.Fatalf("cancel %d code %d", i, w.Code)
		}
		if i < 2 && !strings.Contains(w.Body.String(), `"operation_id":"`+testRunID+`"`) {
			t.Fatal("operation identity missing")
		}
	}
	if b.mutations != 1 {
		t.Fatal("replay mutated twice")
	}
}

func TestUnsupportedHasNoEffects(t *testing.T) {
	for _, path := range []string{"/v1/runs", "/v1/backups/run", "/v1/uploads", "/v1/verifications", "/v1/restore-plans", "/v1/restores", "/v1/retention/apply", "/v1/notifications/test", "/v1/checks", "/v1/deliveries/retry", "/v1/catalog/import"} {
		b := &fakeBackend{}
		w := request(t, b, "POST", path, `{"secret":"synthetic-private"}`)
		if w.Code != 501 || b.calls != 0 || strings.Contains(w.Body.String(), "synthetic-private") {
			t.Fatalf("unsupported route %s has effects: %d", path, w.Code)
		}
	}
}

func TestExistingSocketNotRemoved(t *testing.T) {
	for _, kind := range []string{"file", "symlink", "socket"} {
		t.Run(kind, func(t *testing.T) {
			path := privateSocketPath(t)
			switch kind {
			case "file":
				if err := os.WriteFile(path, []byte("owned by another"), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink("missing", path); err != nil {
					t.Fatal(err)
				}
			case "socket":
				s, err := net.Listen("unix", path)
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if s, err := Listen(path, &fakeBackend{}); err == nil {
				s.Close(context.Background())
				t.Fatal("existing path accepted")
			} else if safeerr.Public(err).Code != safeerr.Conflict {
				t.Fatal("existing path did not conflict")
			}
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(before, after) {
				t.Fatal("existing inode changed")
			}
		})
	}
}

func TestServerCloseDoesNotDeleteReplacement(t *testing.T) {
	path := privateSocketPath(t)
	s, err := Listen(path, &fakeBackend{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "replacement" {
		t.Fatal("replacement removed")
	}
}

func TestSafeViewsAndHealth(t *testing.T) {
	b := &fakeBackend{jobs: []state.JobRevision{{JobID: "nightly", SourceID: "primary", ConfigID: "synthetic-private-value", Snapshot: []byte("synthetic-private-value")}}, run: state.Run{ID: testRunID, State: "queued", RequestID: "synthetic-private-value", PayloadHash: "synthetic-private-value"}, health: Health{Live: true, Ready: false, Scope: "foundation", State: "ok", Logs: "ok", Scheduler: "not_implemented", BackupEngine: "not_implemented"}}
	for _, path := range []string{"/v1/jobs", "/v1/runs/" + testRunID, "/v1/health"} {
		w := request(t, b, "GET", path, "")
		want := 200
		if path == "/v1/health" {
			want = 503
		}
		if w.Code != want || strings.Contains(w.Body.String(), "synthetic-private") || !json.Valid(w.Body.Bytes()) {
			t.Fatal("unsafe view", w.Code, w.Body.String())
		}
	}
	w := request(t, b, "GET", "/v1/jobs", "")
	if !strings.Contains(w.Body.String(), `"engine":"not_implemented"`) || !strings.Contains(w.Body.String(), `"job_id":"nightly"`) {
		t.Fatal("safe job labels missing")
	}
	b.err = errors.New("synthetic-private-value")
	w = request(t, b, "GET", "/v1/jobs", "")
	if w.Code != 500 || strings.Contains(w.Body.String(), "synthetic-private") {
		t.Fatal("backend error reflected")
	}
}

func TestHealthNeverClaimsBackupCapability(t *testing.T) {
	b := &fakeBackend{health: Health{Live: true, Ready: true, Scope: "synthetic-private", State: "ok", Logs: "ok", Scheduler: "ready", BackupEngine: "ready"}}
	w := request(t, b, "GET", "/v1/health", "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "synthetic-private") {
		t.Fatal("health exposed unsupported scope or diagnostics")
	}
	var response struct {
		Data Health `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Data.Scope != "foundation" || response.Data.Scheduler != "not_implemented" || response.Data.BackupEngine != "not_implemented" {
		t.Fatal("M01 health advertised unsupported capability")
	}
}

func TestRouteAndMethodValidation(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{{"GET", "/v1/jobs?unknown=secret", ""}, {"GET", "/v1/health?x=1", ""}, {"GET", "/v1/jobs", "secret"}, {"PUT", "/v1/jobs", ""}, {"POST", "/v1/runs/unsafe/cancel", `{"request_id":"one"}`}, {"POST", "/v1/runs/" + testRunID + "/transition", `{"state":"succeeded"}`}, {"GET", "/v1/runs/../jobs", ""}, {"GET", "/v1/jobs%2fextra", ""}} {
		b := &fakeBackend{}
		w := request(t, b, tc.method, tc.path, tc.body)
		if w.Code < 400 || b.calls != 0 || strings.Contains(w.Body.String(), "secret") {
			t.Fatal("invalid route reached backend", tc.path, w.Code)
		}
	}
}

func TestHandlerConcurrencyBound(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{}, 32)
	b := &fakeBackend{block: release, entered: entered}
	h := newHandler(b)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/v1/jobs", nil))
		}()
	}
	deadline := time.After(2 * time.Second)
	for i := 0; i < 32; i++ {
		select {
		case <-entered:
		case <-deadline:
			close(release)
			wg.Wait()
			t.Fatal("handlers did not start")
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/v1/jobs", nil))
	if w.Code != 429 {
		t.Fatal("handler capacity not enforced")
	}
	close(release)
	wg.Wait()
}

func TestHandlerDeadline(t *testing.T) {
	b := &fakeBackend{}
	before := time.Now()
	w := request(t, b, "GET", "/v1/jobs", "")
	if w.Code != 200 || b.deadline.IsZero() || b.deadline.Before(before) || b.deadline.After(before.Add(5*time.Second+100*time.Millisecond)) {
		t.Fatal("backend request has no bounded deadline")
	}
}

func TestIncompleteHeadersTimeOut(t *testing.T) {
	b := &fakeBackend{}
	socket := privateSocketPath(t)
	s, err := Listen(socket, b)
	if err != nil {
		t.Fatal(err)
	}
	go s.Serve()
	defer s.Close(context.Background())
	conn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(4 * time.Second))
	before := time.Now()
	if _, err := io.WriteString(conn, "GET /v1/jobs HTTP/1.1\r\nHost: localhost\r\n"); err != nil {
		t.Fatal(err)
	}
	_, err = io.ReadAll(conn)
	if err != nil || time.Since(before) > 3500*time.Millisecond {
		t.Fatal("incomplete headers exceeded deadline", err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.calls != 0 {
		t.Fatal("incomplete headers reached backend")
	}
}

func TestServerCloseRemovesOwnedSocket(t *testing.T) {
	socket := privateSocketPath(t)
	s, err := Listen(socket, &fakeBackend{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(socket); !os.IsNotExist(err) {
		t.Fatal("owned socket not removed")
	}
	if conn, err := net.Dial("unix", socket); err == nil {
		conn.Close()
		t.Fatal("closed listener still accepts")
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatal("close not idempotent")
	}
}

func TestServerClosePreservesReplacementSocket(t *testing.T) {
	socket := privateSocketPath(t)
	s, err := Listen(socket, &fakeBackend{})
	if err != nil {
		t.Fatal(err)
	}
	moved := socket + ".old"
	if err := os.Rename(socket, moved); err != nil {
		t.Fatal(err)
	}
	replacement, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer replacement.Close()
	before, _ := os.Lstat(socket)
	if err := s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	after, err := os.Lstat(socket)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("replacement socket removed")
	}
}

func TestSocketParentSymlinkRejected(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if s, err := Listen(filepath.Join(link, "control.sock"), &fakeBackend{}); err == nil {
		s.Close(context.Background())
		t.Fatal("symlink parent accepted")
	}
	entries, _ := os.ReadDir(real)
	if len(entries) != 0 {
		t.Fatal("symlink target mutated")
	}
}

func TestExistingSocketParentPermissionsUnchanged(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "shared")
	if err := os.Mkdir(parent, 0755); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(parent, "control.sock")
	if s, err := Listen(socket, &fakeBackend{}); err == nil {
		s.Close(context.Background())
		t.Fatal("nonprivate existing parent accepted")
	} else if safeerr.Public(err).Code != safeerr.Denied {
		t.Fatal("nonprivate parent did not return denied")
	}
	fi, err := os.Stat(parent)
	if err != nil || fi.Mode().Perm() != 0755 {
		t.Fatal("existing parent permissions changed")
	}
	if _, err := os.Lstat(socket); !os.IsNotExist(err) {
		t.Fatal("socket created in nonprivate parent")
	}
}

func TestSocketBindUsesHeldParentAfterSymlinkSwap(t *testing.T) {
	base := t.TempDir()
	private := filepath.Join(base, "private")
	alternate := filepath.Join(base, "alternate")
	for _, dir := range []string{private, alternate} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	fd, err := openPrivateSocketParent(private)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)
	moved := filepath.Join(base, "moved")
	if err := os.Rename(private, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(alternate, private); err != nil {
		t.Fatal(err)
	}
	listener, _, _, err := listenOwnedSocket(fd, "control.sock")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if fi, err := os.Lstat(filepath.Join(moved, "control.sock")); err != nil || fi.Mode()&os.ModeSocket == 0 {
		t.Fatal("socket was not bound under held parent")
	}
	if _, err := os.Lstat(filepath.Join(alternate, "control.sock")); !os.IsNotExist(err) {
		t.Fatal("socket followed replacement symlink")
	}
}

func TestErrorStatusMapping(t *testing.T) {
	for _, tc := range []struct {
		code   safeerr.Code
		status int
	}{{safeerr.Invalid, 400}, {safeerr.Denied, 403}, {safeerr.NotFound, 404}, {safeerr.Conflict, 409}, {safeerr.Busy, 429}, {safeerr.Unavailable, 503}, {safeerr.NotImplemented, 501}, {safeerr.Internal, 500}} {
		w := request(t, &fakeBackend{err: &safeerr.Error{Code: tc.code}}, "GET", "/v1/jobs", "")
		if w.Code != tc.status || !strings.Contains(w.Body.String(), `"code":"`+string(tc.code)+`"`) {
			t.Fatal("wrong safe error mapping", tc.code, w.Code)
		}
	}
}

func TestServerBoundsOversizedResponse(t *testing.T) {
	jobs := make([]state.JobRevision, 20000)
	for i := range jobs {
		jobs[i] = state.JobRevision{JobID: "nightly", SourceID: "primary"}
	}
	w := request(t, &fakeBackend{jobs: jobs}, "GET", "/v1/jobs", "")
	if w.Code != 500 || w.Body.Len() > 1024 || !json.Valid(w.Body.Bytes()) {
		t.Fatal("oversized backend response not bounded")
	}
}

func TestHandlerTimeoutReturnsSafeJSON(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	b := &fakeBackend{block: release, ignoreContext: true}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	w := httptest.NewRecorder()
	newHandler(b).ServeHTTP(w, httptest.NewRequest("GET", "/v1/jobs", nil).WithContext(ctx))
	if w.Code != 503 || w.Header().Get("Content-Type") != "application/json" || !json.Valid(w.Body.Bytes()) || !strings.Contains(w.Body.String(), `"code":"unavailable"`) {
		t.Fatal("timeout did not return a safe JSON envelope", w.Code, w.Header().Get("Content-Type"))
	}
}

func TestUnixSocketHandlerTimeoutReturnsSafeJSON(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	socket := privateSocketPath(t)
	s, err := Listen(socket, &fakeBackend{block: release, ignoreContext: true})
	if err != nil {
		t.Fatal(err)
	}
	go s.Serve()
	defer s.Close(context.Background())
	conn, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(7 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, "GET /v1/jobs HTTP/1.1\r\nHost: localhost\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal("timeout response lost on Unix socket", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != 503 || !json.Valid(data) || !strings.Contains(string(data), `"code":"unavailable"`) {
		t.Fatal("Unix timeout did not return safe JSON 503", response.StatusCode, err)
	}
}

func TestRootedSocketPathLengthBoundary(t *testing.T) {
	parent := filepath.Dir(privateSocketPath(t))
	fd, err := openPrivateSocketParent(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)
	// A larger descriptor makes the rooted path longer than the configured
	// short pathname. F_DUPFD_CLOEXEC reserves an unused descriptor atomically.
	high, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_DUPFD_CLOEXEC, 10000)
	if errno != 0 {
		t.Fatal(errno)
	}
	defer syscall.Close(int(high))
	prefix := fmt.Sprintf("/proc/self/fd/%d/", high)
	for _, length := range []int{107, 108} {
		name := strings.Repeat("s", length-len(prefix))
		listener, _, _, err := listenOwnedSocket(int(high), name)
		if length == 107 {
			if err != nil {
				t.Fatal("107-byte rooted path rejected", err)
			}
			listener.Close()
			if err := os.Remove(filepath.Join(parent, name)); err != nil {
				t.Fatal(err)
			}
		} else {
			if listener != nil {
				listener.Close()
			}
			if safeerr.Public(err).Code != safeerr.Invalid {
				t.Error("108-byte rooted path must be invalid", err)
			}
			if _, err := os.Lstat(filepath.Join(parent, name)); !os.IsNotExist(err) {
				t.Error("invalid path created socket", err)
			}
			// Length rejection must precede inspection/recovery of any existing inode.
			if err := os.WriteFile(filepath.Join(parent, name), []byte("preserve"), 0600); err != nil {
				t.Fatal(err)
			}
			_, _, _, err = listenOwnedSocket(int(high), name)
			if safeerr.Public(err).Code != safeerr.Invalid {
				t.Error("invalid path inspected existing inode", err)
			}
		}
	}
}
