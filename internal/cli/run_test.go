package cli

import (
	"bytes"
	"context"
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
