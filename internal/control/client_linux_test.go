package control

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bkplight.local/bkplight/internal/safeerr"
)

func maliciousSocket(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "malicious.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	s := &http.Server{Handler: handler}
	go s.Serve(listener)
	t.Cleanup(func() { s.Close() })
	return path
}

func TestClientDoesNotUseProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("ALL_PROXY", "http://127.0.0.1:1")
	path := maliciousSocket(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "localhost" {
			t.Error("unexpected transport host")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"request_id":"","status":"ok","data":{"live":true,"ready":true,"scope":"foundation","state":"ok","logs":"ok","scheduler":"not_implemented","backup_engine":"not_implemented"}}`))
	})
	if _, err := NewClient(path).Do(context.Background(), "GET", "/v1/health", nil); err != nil {
		t.Fatal("proxy interfered", err)
	}
}

func TestClientRejectsUnsupportedHealthClaim(t *testing.T) {
	path := maliciousSocket(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"request_id":"","status":"ok","data":{"live":true,"ready":true,"scope":"foundation_only","state":"ok","logs":"ok","scheduler":"ready","backup_engine":"ready"}}`))
	})
	data, err := NewClient(path).Do(context.Background(), "GET", "/v1/health", nil)
	if err == nil || len(data) != 0 {
		t.Fatal("client accepted unsupported M01 health claim")
	}
}

func TestClientOversizedResponse(t *testing.T) {
	path := maliciousSocket(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"request_id":"","status":"ok","data":{"live":true,"ready":true,"scope":"` + strings.Repeat("x", 1<<20) + `"}}`))
	})
	if _, err := NewClient(path).Do(context.Background(), "GET", "/v1/health", nil); err == nil {
		t.Fatal("oversized response accepted")
	}
}

func TestClientCancellationAndRedirect(t *testing.T) {
	path := maliciousSocket(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	before := time.Now()
	if _, err := NewClient(path).Do(ctx, "GET", "/v1/health", nil); err == nil || time.Since(before) > time.Second || strings.Contains(err.Error(), path) {
		t.Fatal("client cancellation not bounded or safe")
	}
	calls := 0
	path = maliciousSocket(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Redirect(w, r, "http://other.invalid/v1/health", 302)
	})
	if _, err := NewClient(path).Do(context.Background(), "GET", "/v1/health", nil); err == nil || calls != 1 {
		t.Fatal("redirect followed")
	}
}

func TestClientRejectsAmbiguousResponse(t *testing.T) {
	for _, body := range []string{`{"request_id":"","status":"ok","status":"error","data":{}}`, `{"request_id":"","status":"ok","unknown":"synthetic-private-value","data":{}}`, `{"request_id":"","status":"ok","data":{"live":true,"ready":true,"unknown":"synthetic-private-value"}}`, `{"request_id":"","status":"ok","data":{"live":true,"ready":true}} {}`} {
		path := maliciousSocket(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) })
		raw, err := NewClient(path).Do(context.Background(), "GET", "/v1/health", nil)
		if err == nil || len(raw) != 0 || strings.Contains(err.Error(), "synthetic-private") {
			t.Fatal("ambiguous response accepted")
		}
	}
}

func TestClientSafeErrors(t *testing.T) {
	for _, body := range []string{"synthetic-private-value", `{"request_id":"","status":"error","error":{"code":"internal","message":"synthetic-private-value","retryable":false}}`, `{"request_id":"","status":"error","error":{"code":"synthetic-private-value","message":"x","retryable":false}}`} {
		path := maliciousSocket(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500); w.Write([]byte(body)) })
		data, err := NewClient(path).Do(context.Background(), "GET", "/v1/health", nil)
		if err == nil || len(data) != 0 || strings.Contains(err.Error(), "synthetic-private") {
			t.Fatal("unsafe error reflected")
		}
		var se *safeerr.Error
		if !errors.As(err, &se) {
			t.Fatal("error is not safe")
		}
	}
}

func TestClientRejectsNonLocalPaths(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "control.sock")
	if err := os.WriteFile(marker, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, socket := range []string{"relative.sock", "http://localhost/control.sock", "unix:///tmp/control.sock", "/tmp/../control.sock"} {
		if _, err := NewClient(socket).Do(context.Background(), "GET", "/v1/health", nil); safeerr.Public(err).Code != safeerr.Invalid {
			t.Fatal("unsafe socket accepted")
		}
	}
	for _, path := range []string{"http://evil.test/v1/health", "//evil.test/v1/health", "/v1/../health", "/v1/health#x", "/v1/health%2fextra"} {
		if _, err := NewClient(marker).Do(context.Background(), "GET", path, nil); safeerr.Public(err).Code != safeerr.Invalid {
			t.Fatal("unsafe route accepted")
		}
	}
}
