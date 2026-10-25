// Package control implements bounded HTTP/JSON over a private Unix socket.
package control

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"path/filepath"
	"regexp"
	"time"

	"bkplight.local/bkplight/internal/safeerr"
	"bkplight.local/bkplight/internal/state"
)

const (
	maxBody        = 64 << 10
	maxResponse    = 1 << 20
	requestTimeout = 5 * time.Second
)

type Health struct {
	Live         bool   `json:"live"`
	Ready        bool   `json:"ready"`
	Scope        string `json:"scope"`
	State        string `json:"state"`
	Logs         string `json:"logs"`
	Scheduler    string `json:"scheduler"`
	BackupEngine string `json:"backup_engine"`
}

type Backend interface {
	Jobs(context.Context) ([]state.JobRevision, error)
	GetRun(context.Context, string) (state.Run, error)
	ListRuns(context.Context, int, string) (state.Page, error)
	Cancel(context.Context, string, string) (state.Run, error)
	Health(context.Context) Health
}

// JobView deliberately omits configuration revisions, snapshots and secret references.
type JobView struct {
	JobID    string `json:"job_id"`
	SourceID string `json:"source_id"`
	Engine   string `json:"engine"`
}
type RunView struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	Version   int64     `json:"version"`
	StartedAt time.Time `json:"started_at"`
	ErrorCode string    `json:"error_code"`
}
type RunPage struct {
	Items []RunView `json:"items"`
	Next  string    `json:"next"`
}
type wireError struct {
	Code      safeerr.Code `json:"code"`
	Message   string       `json:"message"`
	Retryable bool         `json:"retryable"`
}
type envelope struct {
	RequestID   string          `json:"request_id"`
	Status      string          `json:"status"`
	OperationID string          `json:"operation_id,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
	Error       *wireError      `json:"error,omitempty"`
}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var runIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var labelPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
var pathControlPattern = regexp.MustCompile(`[\x00-\x1f\x7f]`)

func invalid() error     { return &safeerr.Error{Code: safeerr.Invalid} }
func internal() error    { return &safeerr.Error{Code: safeerr.Internal} }
func unavailable() error { return &safeerr.Error{Code: safeerr.Unavailable, Retryable: true} }
func validSocket(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && path != "/" && len(path) < 108 && !pathControlPattern.MatchString(path)
}

func errorStatus(code safeerr.Code) (int, string) {
	switch code {
	case safeerr.Invalid:
		return 400, "Invalid input"
	case safeerr.Denied:
		return 403, "Access denied"
	case safeerr.NotFound:
		return 404, "Not found"
	case safeerr.Conflict:
		return 409, "Operation conflict"
	case safeerr.Busy:
		return 429, "Service busy"
	case safeerr.Unavailable:
		return 503, "Service unavailable"
	case safeerr.NotImplemented:
		return 501, "Not implemented"
	default:
		return 500, "Internal error"
	}
}

// validCursor mirrors the state's closed opaque (started_at,id) tuple syntax.
func validCursor(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 256 {
		return false
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(value)
	if err != nil {
		return false
	}
	var tuple []json.RawMessage
	var started *int64
	var id string
	return json.Unmarshal(raw, &tuple) == nil && len(tuple) == 2 && json.Unmarshal(tuple[0], &started) == nil && started != nil && json.Unmarshal(tuple[1], &id) == nil && runIDPattern.MatchString(id)
}

// strictJSON checks duplicate keys before decoding a closed typed schema.
func strictJSON(raw []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	nodes := 0
	var value func(int) error
	value = func(depth int) error {
		nodes++
		if depth > 32 || nodes > 20000 {
			return invalid()
		}
		token, err := d.Token()
		if err != nil {
			return invalid()
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				name, ok := key.(string)
				if err != nil || !ok || seen[name] {
					return invalid()
				}
				seen[name] = true
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		default:
			return invalid()
		}
		_, err = d.Token()
		if err != nil {
			return invalid()
		}
		return nil
	}
	if value(0) != nil {
		return invalid()
	}
	if _, err := d.Token(); err != io.EOF {
		return invalid()
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(dst) != nil {
		return invalid()
	}
	return nil
}
