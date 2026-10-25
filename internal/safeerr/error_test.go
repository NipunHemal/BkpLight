package safeerr

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPublicHidesUnknownError(t *testing.T) {
	got := Public(errors.New("synthetic-sensitive-value"))
	if got.Code != Internal || strings.Contains(got.Error(), "synthetic-sensitive-value") {
		t.Fatalf("unsafe error classification: %q", got.Code)
	}
}

func TestErrorRejectsUnknownCodeAndField(t *testing.T) {
	for _, candidate := range []Error{
		{Code: "synthetic-sensitive-code"},
		{Code: Invalid, Field: "secret=value"},
		{Code: Invalid, Field: "config\nsecret"},
		{Code: Invalid, Field: "cönfig"},
		{Code: Invalid, Field: strings.Repeat("a", 129)},
	} {
		if candidate.Error() != "internal" {
			t.Fatal("unsafe error string was not rejected")
		}
		if got := Public(&candidate); got != (Error{Code: Internal}) {
			t.Fatal("unsafe public error was not rejected")
		}
	}
}

func TestPublicPreservesValidatedWrappedError(t *testing.T) {
	for _, code := range []Code{Invalid, Denied, Conflict, Busy, Unavailable, NotFound, NotImplemented, Internal} {
		want := Error{Code: code, Field: "jobs[12].db_name", Retryable: true}
		got := Public(fmt.Errorf("synthetic-sensitive-value: %w", &want))
		if got != want || got.Error() != string(code)+": jobs[12].db_name" {
			t.Fatalf("safe error contract not preserved for %q", code)
		}
	}
	want := Error{Code: Invalid, Field: strings.Repeat("a", 128)}
	if got := Public(&want); got != want {
		t.Fatal("maximum length valid field was rejected")
	}
	if got := (&Error{Code: Denied}).Error(); got != "denied" {
		t.Fatalf("empty field formatting: %q", got)
	}
}

func TestPublicHandlesNilErrors(t *testing.T) {
	var typedNil *Error
	for _, err := range []error{nil, typedNil} {
		if got := Public(err); got != (Error{Code: Internal}) {
			t.Fatal("nil error must produce internal")
		}
	}
}
