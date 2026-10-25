package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"bkplight.local/bkplight/internal/config"
)

func TestVersionCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"version"}, &out, &errOut); code != 0 {
		t.Fatalf("version exit: %d", code)
	}
	if out.String() != "bkplight dev\n" || errOut.Len() != 0 {
		t.Fatalf("unexpected version output: %q / %q", out.String(), errOut.String())
	}
}

func secretConfig(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, "secrets")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	raw := fmt.Sprintf(`version: 1
service:
  timezone: UTC
  state_dir: %[2]s/state
  socket_path: %[2]s/control.sock
  secret_roots: [%[1]s]
  staging_dir: %[2]s/staging
  staging_max_bytes: 1024
  min_free_bytes: 1024
sources:
  active: {type: postgres, host: database, port: 5432, credentials_file: %[1]s/source, databases: [app], tls: {enabled: true, ca_file: %[1]s/ca, server_name: database}}
  unused: {type: postgres, host: unused, port: 5432, credentials_file: %[1]s/missing-source, databases: [app]}
destinations:
  store: {type: s3, endpoint: https://storage.example.test, bucket: fixtures, prefix: backups, credentials_file: %[1]s/destination}
jobs:
  active:
    source: active
    schedule: {daily: '01:00'}
    destinations: [{id: store, required: true}]
    retention: {keep_last: 1}
    notify: [active, disabled]
  disabled:
    enabled: false
    source: unused
    schedule: {daily: '02:00'}
    destinations: [{id: store, required: true}]
    retention: {keep_last: 1}
encryption: {recipients: [synthetic-public-recipient], signing_key_file: %[1]s/signing, identity_file: %[1]s/missing-identity}
notifications:
  channels:
    active: {type: custom_http, url_file: %[1]s/webhook, auth_file: %[1]s/auth, hmac_file: %[1]s/hmac, events: [backup.failed]}
    disabled: {enabled: false, type: custom_http, url_file: %[1]s/missing-webhook, events: [backup.failed]}
logging: {directory: %[2]s/logs}
`, dir, base)
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"source", "ca", "destination", "signing", "webhook", "auth", "hmac"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("synthetic-private-value\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return path, dir
}

func TestCheckSecretsOnlyEnabledGraph(t *testing.T) {
	path, dir := secretConfig(t)
	run := func(check bool) (int, string, string) {
		var out, errOut bytes.Buffer
		args := []string{"config", "validate", "--config", path}
		if check {
			args = append(args, "--check-secrets")
		}
		code := Run(context.Background(), args, &out, &errOut)
		return code, out.String(), errOut.String()
	}
	if code, out, errOut := run(true); code != 0 || !regexp.MustCompile(`^valid [a-f0-9]{64}\n$`).MatchString(out) || errOut != "" {
		t.Fatalf("active graph check failed: %d %s", code, errOut)
	}
	for _, name := range []string{"source", "ca", "destination", "signing", "webhook", "auth", "hmac"} {
		p := filepath.Join(dir, name)
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		code, out, errOut := run(true)
		if code != 2 || out != "" || errOut != "denied: secrets\n" {
			t.Fatal("missing active dependency did not return safe error")
		}
		if err := os.WriteFile(p, []byte("synthetic-private-value"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "source"), make([]byte, 65537), 0600); err != nil {
		t.Fatal(err)
	}
	if code, out, errOut := run(true); code != 2 || out != "" || errOut != "denied: secrets\n" {
		t.Fatal("oversized secret accepted")
	}
	if code, _, errOut := run(false); code != 0 || errOut != "" {
		t.Fatal("default validation read secrets")
	}
}

func TestSecretsNotIncludedInRevision(t *testing.T) {
	path, dir := secretConfig(t)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := config.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	before, snapshot, err := config.Revision(c)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(snapshot, []byte("synthetic-private-value")) {
		t.Fatal("secret included in revision")
	}
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("rotated-synthetic-private-value"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"config", "validate", "--config", path, "--check-secrets"}, &out, &errOut); code != 0 || out.String() != "valid "+before+"\n" || errOut.Len() != 0 {
		t.Fatal("secret rotation changed revision or leaked output")
	}
}

func TestCheckSecretsEmptyGraphDoesNotOpenRoots(t *testing.T) {
	raw, err := os.ReadFile("../config/testdata/valid.yaml")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	raw = bytes.ReplaceAll(raw, []byte("/run/secrets"), []byte(filepath.Join(dir, "absent")))
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"config", "validate", "--config", file, "--check-secrets"}, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatal("empty graph opened missing roots")
	}
}

func TestCheckSecretsVerificationDependencies(t *testing.T) {
	path, dir := secretConfig(t)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.Replace(raw, []byte("    notify: [active, disabled]"), []byte("    notify: [active, disabled]\n    verification: {schedule: {weekly: 'SUN@03:00'}, destination: store, target: isolated, profile: portable_data}"), 1)
	raw = append(raw, []byte(fmt.Sprintf("\nverification_targets:\n  isolated: {type: postgres, host: isolated, port: 5432, identity: isolated-fixture, credentials_file: %s/target, tls: {enabled: true, ca_file: %s/target-ca, server_name: isolated}}\n", dir, dir))...)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"target", "target-ca", "missing-identity"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("synthetic-private-value"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run := func() int {
		var out, errOut bytes.Buffer
		return Run(context.Background(), []string{"config", "validate", "--config", path, "--check-secrets"}, &out, &errOut)
	}
	if run() != 0 {
		t.Fatal("valid verification graph rejected")
	}
	for _, name := range []string{"target", "target-ca", "missing-identity"} {
		p := filepath.Join(dir, name)
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
		if run() != 2 {
			t.Fatal("missing verification dependency accepted")
		}
		if err := os.WriteFile(p, []byte("synthetic-private-value"), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConfigValidateOffline(t *testing.T) {
	raw, err := os.ReadFile("../config/testdata/valid.yaml")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	raw = bytes.ReplaceAll(raw, []byte("/var/lib/bkplight"), []byte(filepath.Join(dir, "absent")))
	raw = bytes.ReplaceAll(raw, []byte("/var/log/bkplight"), []byte(filepath.Join(dir, "logs")))
	if err = os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"config", "validate", "--config", file}, &out, &errOut); code != 0 || errOut.Len() != 0 {
		t.Fatalf("validation failed: %d %s", code, errOut.String())
	}
	if !regexp.MustCompile(`^valid [a-f0-9]{64}\n$`).MatchString(out.String()) {
		t.Fatal("unsafe validation output")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatal("offline validation created resources")
	}
}
func TestConfigValidateSafeFailures(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("version: 1\npassword: synthetic-private-value"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"config"}, {"config", "validate"}, {"config", "other"}, {"config", "validate", "--config", file, "extra"}, {"config", "validate", "--check-secrets"}, {"config", "validate", "--config", file}, {"config", "validate", "--config", "/missing/synthetic-private-value"}} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), args, &out, &errOut); code != 2 {
			t.Fatalf("unexpected failure exit: %d", code)
		}
		if out.Len() != 0 || !strings.HasPrefix(errOut.String(), "invalid_config: ") || strings.Contains(errOut.String(), "synthetic-private-value") {
			t.Fatal("unsafe validation failure")
		}
	}
}

func TestUnknownCommandDoesNotEchoArgs(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"synthetic-credential=private", "--token=synthetic-sensitive-value"},
		{"--synthetic-sensitive-value"},
	} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), args, &out, &errOut); code != 2 {
			t.Fatalf("invalid command exit: %d", code)
		}
		if out.Len() != 0 || errOut.String() != "invalid_config: command\n" {
			t.Fatal("invalid command did not produce the safe envelope")
		}
	}
}

func TestVersionRejectsArgumentsWithoutEcho(t *testing.T) {
	for _, args := range [][]string{
		{"version", "--token=synthetic-sensitive-value"},
		{"version", "synthetic-sensitive-value"},
		{"version", "-h"},
	} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), args, &out, &errOut); code != 2 {
			t.Fatalf("invalid version argument exit: %d", code)
		}
		if out.Len() != 0 || errOut.String() != "invalid_config: command\n" {
			t.Fatal("invalid version argument escaped the safe envelope")
		}
	}
}

func TestCLISafeErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "malicious.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte("synthetic-private-value"))
	})}
	go server.Serve(listener)
	defer server.Close()
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"health", "--socket", path}, &out, &errOut); code != 3 || out.Len() != 0 || errOut.String() != "internal\n" {
		t.Fatalf("unsafe daemon failure: %d %q %q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := Run(context.Background(), []string{"history", "show", "synthetic-private-value", "--socket", path}, &out, &errOut); code != 2 || out.Len() != 0 || strings.Contains(errOut.String(), "synthetic-private") {
		t.Fatal("invalid run reached daemon")
	}
}

func TestCLIUnsupportedHasNoEffects(t *testing.T) {
	for _, family := range []string{"backup", "upload", "verify", "restore", "retention", "notifications", "notification"} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), []string{family, "run", "synthetic-private-value"}, &out, &errOut); code != 4 || out.Len() != 0 || errOut.String() != "not_implemented\n" {
			t.Fatal("unsupported command has wrong output", family, code)
		}
	}
}

func TestCLIControlCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	runID := "0123456789abcdef0123456789abcdef"
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/health":
			fmt.Fprint(w, `{"request_id":"","status":"ok","data":{"live":true,"ready":true,"scope":"foundation","state":"ok","logs":"ok","scheduler":"not_implemented","backup_engine":"not_implemented"}}`)
		case "/v1/jobs":
			fmt.Fprint(w, `{"request_id":"","status":"ok","data":[{"job_id":"nightly","source_id":"primary","engine":"not_implemented"}]}`)
		case "/v1/runs":
			if r.URL.Query().Get("limit") != "2" {
				t.Error("limit not forwarded")
			}
			fmt.Fprint(w, `{"request_id":"","status":"ok","data":{"items":[],"next":""}}`)
		case "/v1/runs/" + runID:
			fmt.Fprintf(w, `{"request_id":"","status":"ok","data":{"id":"%s","state":"queued","version":1,"started_at":"2026-10-25T00:00:00Z","error_code":""}}`, runID)
		case "/v1/runs/" + runID + "/cancel":
			var body struct {
				RequestID string `json:"request_id"`
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil || body.RequestID != "cancel-one" || r.Method != "POST" {
				t.Error("cancel body not forwarded")
			}
			fmt.Fprintf(w, `{"request_id":"cancel-one","operation_id":"%s","status":"ok","data":{"id":"%s","state":"cancelled","version":2,"started_at":"2026-10-25T00:00:00Z","error_code":""}}`, runID, runID)
		default:
			t.Error("unexpected request")
			w.WriteHeader(404)
		}
	})}
	go server.Serve(listener)
	defer server.Close()
	for _, args := range [][]string{{"health", "--socket", path}, {"jobs", "list", "--socket", path}, {"history", "list", "--limit", "2", "--socket", path}, {"history", "show", runID, "--socket", path}, {"operation", "cancel", runID, "--request-id", "cancel-one", "--socket", path}} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), args, &out, &errOut); code != 0 || !json.Valid(out.Bytes()) || errOut.Len() != 0 {
			t.Fatalf("control command failed: %v code=%d err=%s", args, code, errOut.String())
		}
	}
}

func TestCLIControlInvalidArguments(t *testing.T) {
	for _, args := range [][]string{{"health", "--unexpected", "synthetic-private-value"}, {"jobs", "show"}, {"history", "list", "--limit", "201"}, {"history", "list", "--limit", "0"}, {"history", "show"}, {"operation", "cancel", "0123456789abcdef0123456789abcdef"}, {"operation", "cancel", "0123456789abcdef0123456789abcdef", "--request-id", "unsafe/value"}, {"health", "extra"}} {
		var out, errOut bytes.Buffer
		if code := Run(context.Background(), args, &out, &errOut); code != 2 || out.Len() != 0 || strings.Contains(errOut.String(), "synthetic-private") {
			t.Fatal("invalid command not safely rejected", args, code)
		}
	}
}

func TestCLIHealthNotReadyStillPrintsHealth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		fmt.Fprint(w, `{"request_id":"","status":"ok","data":{"live":true,"ready":false,"scope":"foundation","state":"unavailable","logs":"ok","scheduler":"not_implemented","backup_engine":"not_implemented"}}`)
	})}
	go server.Serve(listener)
	defer server.Close()
	var out, errOut bytes.Buffer
	if code := Run(context.Background(), []string{"health", "--socket", path}, &out, &errOut); code != 3 || !json.Valid(out.Bytes()) || !strings.Contains(out.String(), `"ready":false`) || errOut.String() != "unavailable\n" {
		t.Fatal("unready health not retained", code)
	}
}
