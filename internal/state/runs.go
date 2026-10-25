package state

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"bkplight.local/bkplight/internal/config"
	"bkplight.local/bkplight/internal/safeerr"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func validHex(value string, size int) bool {
	if len(value) != size {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func invalid(field string) error { return &safeerr.Error{Code: safeerr.Invalid, Field: field} }
func conflict() error            { return &safeerr.Error{Code: safeerr.Conflict} }

const runColumns = "id,job_revision_id,request_id,payload_hash,state,version,started_at,error_code"

type scanner interface{ Scan(...any) error }

func scanRun(row scanner) (Run, error) {
	var r Run
	var started int64
	err := row.Scan(&r.ID, &r.JobRevisionID, &r.RequestID, &r.PayloadHash, &r.State, &r.Version, &started, &r.ErrorCode)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, &safeerr.Error{Code: safeerr.NotFound}
	}
	if err != nil {
		return Run{}, unavailable()
	}
	r.StartedAt = time.UnixMicro(started).UTC()
	return r, nil
}
func exists(ctx context.Context, tx *sql.Tx, query string, args ...any) (bool, error) {
	var n int
	err := tx.QueryRowContext(ctx, query, args...).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, unavailable()
	}
	return true, nil
}

// BeginRun binds an accepted command to its immutable job and canonical command
// hash. Replays precede current eligibility checks so restart/config changes do
// not erase an accepted request.
func (s *Store) BeginRun(ctx context.Context, b Begin) (Run, bool, error) {
	if !validHex(b.ID, 32) {
		return Run{}, false, invalid("run_id")
	}
	if !validHex(b.JobRevisionID, 64) {
		return Run{}, false, invalid("job_revision_id")
	}
	if !requestIDPattern.MatchString(b.RequestID) {
		return Run{}, false, invalid("request_id")
	}
	if !validHex(b.PayloadHash, 64) {
		return Run{}, false, invalid("payload_hash")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, false, unavailable()
	}
	defer tx.Rollback()
	previous, err := scanRun(tx.QueryRowContext(ctx, "SELECT "+runColumns+" FROM runs WHERE request_id=?", b.RequestID))
	if err == nil {
		if previous.JobRevisionID != b.JobRevisionID || previous.PayloadHash != b.PayloadHash {
			return Run{}, false, conflict()
		}
		if err = tx.Commit(); err != nil {
			return Run{}, false, unavailable()
		}
		return previous, true, nil
	}
	if safeerr.Public(err).Code != safeerr.NotFound {
		return Run{}, false, err
	}
	used, err := exists(ctx, tx, "SELECT 1 FROM run_commands WHERE request_id=?", b.RequestID)
	if err != nil {
		return Run{}, false, err
	}
	if used {
		return Run{}, false, conflict()
	}
	used, err = exists(ctx, tx, "SELECT 1 FROM runs WHERE id=?", b.ID)
	if err != nil {
		return Run{}, false, err
	}
	if used {
		return Run{}, false, conflict()
	}
	var snapshot []byte
	err = tx.QueryRowContext(ctx, "SELECT j.snapshot FROM job_revisions j JOIN active_config a ON j.config_id=a.config_id WHERE j.id=?", b.JobRevisionID).Scan(&snapshot)
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, false, conflict()
	}
	if err != nil {
		return Run{}, false, unavailable()
	}
	var job config.Job
	if json.Unmarshal(snapshot, &job) != nil {
		return Run{}, false, unavailable()
	}
	if !enabled(job.Enabled) {
		return Run{}, false, conflict()
	}
	started := s.now().UTC().UnixMicro()
	r := Run{ID: b.ID, JobRevisionID: b.JobRevisionID, RequestID: b.RequestID, PayloadHash: b.PayloadHash, State: "queued", Version: 1, StartedAt: time.UnixMicro(started).UTC()}
	if _, err = tx.ExecContext(ctx, "INSERT INTO runs(id,job_revision_id,request_id,payload_hash,state,version,started_at) VALUES(?,?,?,?,'queued',1,?)", b.ID, b.JobRevisionID, b.RequestID, b.PayloadHash, started); err != nil {
		return Run{}, false, unavailable()
	}
	if err = s.eventTx(ctx, tx, r, ""); err != nil {
		return Run{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return Run{}, false, unavailable()
	}
	return r, false, nil
}

func knownState(state string) bool {
	switch state {
	case "queued", "preflight", "dumping", "delivering", "succeeded", "degraded", "failed", "cancelled", "interrupted":
		return true
	}
	return false
}
func allowed(from, to string) bool {
	switch from {
	case "queued":
		return to == "preflight" || to == "cancelled" || to == "interrupted"
	case "preflight":
		return to == "dumping" || to == "failed" || to == "cancelled" || to == "interrupted"
	case "dumping":
		return to == "delivering" || to == "failed" || to == "cancelled" || to == "interrupted"
	case "delivering":
		return to == "succeeded" || to == "degraded" || to == "failed" || to == "cancelled" || to == "interrupted"
	}
	return false
}
func terminal(state string) bool {
	return state == "succeeded" || state == "degraded" || state == "failed" || state == "cancelled" || state == "interrupted"
}
func knownCode(code string) bool {
	switch safeerr.Code(code) {
	case "", safeerr.Invalid, safeerr.Denied, safeerr.Conflict, safeerr.Busy, safeerr.Unavailable, safeerr.NotFound, safeerr.NotImplemented, safeerr.Internal:
		return true
	}
	return false
}
func validateTransition(t Transition) error {
	if !validHex(t.RunID, 32) {
		return invalid("run_id")
	}
	if !knownState(t.From) || !knownState(t.To) || t.Version < 1 {
		return invalid("transition")
	}
	if !knownCode(t.ErrorCode) {
		return invalid("error_code")
	}
	return nil
}
func (s *Store) Move(ctx context.Context, t Transition) (Run, error) {
	if err := validateTransition(t); err != nil {
		return Run{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, unavailable()
	}
	defer tx.Rollback()
	r, err := s.moveTx(ctx, tx, t)
	if err != nil {
		return Run{}, err
	}
	if err = tx.Commit(); err != nil {
		return Run{}, unavailable()
	}
	return r, nil
}
func (s *Store) moveTx(ctx context.Context, tx *sql.Tx, t Transition) (Run, error) {
	if err := validateTransition(t); err != nil {
		return Run{}, err
	}
	if !allowed(t.From, t.To) {
		return Run{}, conflict()
	}
	r, err := scanRun(tx.QueryRowContext(ctx, "SELECT "+runColumns+" FROM runs WHERE id=?", t.RunID))
	if err != nil {
		return Run{}, err
	}
	if r.State != t.From || r.Version != t.Version {
		return Run{}, conflict()
	}
	var finished any
	if terminal(t.To) {
		finished = s.now().UTC().UnixMicro()
	}
	result, err := tx.ExecContext(ctx, "UPDATE runs SET state=?,version=version+1,finished_at=?,error_code=? WHERE id=? AND state=? AND version=?", t.To, finished, t.ErrorCode, t.RunID, t.From, t.Version)
	if err != nil {
		return Run{}, unavailable()
	}
	n, err := result.RowsAffected()
	if err != nil {
		return Run{}, unavailable()
	}
	if n != 1 {
		return Run{}, conflict()
	}
	r.State = t.To
	r.Version++
	r.ErrorCode = t.ErrorCode
	if err = s.eventTx(ctx, tx, r, t.From); err != nil {
		return Run{}, err
	}
	return r, nil
}

func (s *Store) Cancel(ctx context.Context, runID, requestID string) (Run, error) {
	if !validHex(runID, 32) {
		return Run{}, invalid("run_id")
	}
	if !requestIDPattern.MatchString(requestID) {
		return Run{}, invalid("request_id")
	}
	sum := sha256.Sum256([]byte("cancel\x00" + runID))
	hash := hex.EncodeToString(sum[:])
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, unavailable()
	}
	defer tx.Rollback()
	var previousID, kind, previousHash string
	var version int64
	err = tx.QueryRowContext(ctx, "SELECT run_id,kind,payload_hash,result_version FROM run_commands WHERE request_id=?", requestID).Scan(&previousID, &kind, &previousHash, &version)
	if err == nil {
		if previousID != runID || kind != "cancel" || previousHash != hash {
			return Run{}, conflict()
		}
		r, err := scanRun(tx.QueryRowContext(ctx, "SELECT "+runColumns+" FROM runs WHERE id=?", runID))
		if err != nil {
			return Run{}, err
		}
		if r.State != "cancelled" || r.Version != version {
			return Run{}, conflict()
		}
		if err = tx.Commit(); err != nil {
			return Run{}, unavailable()
		}
		return r, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Run{}, unavailable()
	}
	used, err := exists(ctx, tx, "SELECT 1 FROM runs WHERE request_id=?", requestID)
	if err != nil {
		return Run{}, err
	}
	if used {
		return Run{}, conflict()
	}
	current, err := scanRun(tx.QueryRowContext(ctx, "SELECT "+runColumns+" FROM runs WHERE id=?", runID))
	if err != nil {
		return Run{}, err
	}
	if current.State != "queued" {
		return Run{}, conflict()
	}
	r, err := s.moveTx(ctx, tx, Transition{RunID: runID, From: "queued", To: "cancelled", Version: current.Version})
	if err != nil {
		return Run{}, err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO run_commands(request_id,run_id,kind,payload_hash,result_version) VALUES(?,?,'cancel',?,?)", requestID, runID, hash, r.Version); err != nil {
		return Run{}, unavailable()
	}
	if err = tx.Commit(); err != nil {
		return Run{}, unavailable()
	}
	return r, nil
}

func (s *Store) GetRun(ctx context.Context, id string) (Run, error) {
	if !validHex(id, 32) {
		return Run{}, invalid("run_id")
	}
	return scanRun(s.db.QueryRowContext(ctx, "SELECT "+runColumns+" FROM runs WHERE id=?", id))
}

type historyCursor struct {
	Started int64
	ID      string
}

func decodeCursor(after string) (historyCursor, error) {
	var c historyCursor
	if len(after) > 256 {
		return c, invalid("cursor")
	}
	data, err := base64.RawURLEncoding.Strict().DecodeString(after)
	if err != nil {
		return c, invalid("cursor")
	}
	var tuple []json.RawMessage
	var started *int64
	if json.Unmarshal(data, &tuple) != nil || len(tuple) != 2 || json.Unmarshal(tuple[0], &started) != nil || started == nil || json.Unmarshal(tuple[1], &c.ID) != nil || !validHex(c.ID, 32) {
		return historyCursor{}, invalid("cursor")
	}
	c.Started = *started
	return c, nil
}
func (s *Store) ListRuns(ctx context.Context, limit int, after string) (Page, error) {
	if limit < 0 || limit > 200 {
		return Page{}, invalid("limit")
	}
	if limit == 0 {
		limit = 50
	}
	query := "SELECT " + runColumns + " FROM runs"
	var args []any
	if after != "" {
		cursor, err := decodeCursor(after)
		if err != nil {
			return Page{}, err
		}
		query += " WHERE started_at<? OR (started_at=? AND id<?)"
		args = append(args, cursor.Started, cursor.Started, cursor.ID)
	}
	query += " ORDER BY started_at DESC,id DESC LIMIT ?"
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Page{}, unavailable()
	}
	defer rows.Close()
	p := Page{Items: make([]Run, 0, limit)}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return Page{}, err
		}
		p.Items = append(p.Items, r)
	}
	if rows.Err() != nil || rows.Close() != nil {
		return Page{}, unavailable()
	}
	if len(p.Items) > limit {
		p.Items = p.Items[:limit]
		last := p.Items[limit-1]
		data, err := json.Marshal([2]any{last.StartedAt.UnixMicro(), last.ID})
		if err != nil {
			return Page{}, unavailable()
		}
		p.Next = base64.RawURLEncoding.EncodeToString(data)
	}
	return p, nil
}
