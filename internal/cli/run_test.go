package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
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
