package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"bkplight.local/bkplight/internal/config"
	"bkplight.local/bkplight/internal/safeerr"
	"modernc.org/sqlite"
)

func TestZeroChannelsNoDeliveries(t *testing.T) {
	s, job := newFixture(t)
	r := beginFixture(t, s, job, 1)
	moved, err := s.Move(context.Background(), Transition{RunID: r.ID, From: "queued", To: "preflight", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Move(context.Background(), Transition{RunID: r.ID, From: "preflight", To: "dumping", Version: moved.Version}); err != nil {
		t.Fatal(err)
	}
	countRows(t, s, "events", 3)
	countRows(t, s, "deliveries", 0)
	var kind string
	var payload []byte
	var sequence int
	if err = s.db.QueryRow("SELECT event_type,payload,sequence FROM events WHERE run_id=? ORDER BY sequence DESC LIMIT 1", r.ID).Scan(&kind, &payload, &sequence); err != nil {
		t.Fatal(err)
	}
	if kind != "operation.transition" || sequence != 3 || !strings.Contains(string(payload), `"to":"dumping"`) || strings.Contains(string(payload), "credentials") {
		t.Fatal("unsafe or missing transition event", kind, string(payload), sequence)
	}
}

func channelFixture(t *testing.T) (*Store, string, config.Config) {
	t.Helper()
	s, _ := newFixture(t)
	c := fixtureConfig(t)
	c.Notifications.Channels = map[string]config.ChannelConfig{"alerts": {Type: "custom_http", URLFile: "/run/secrets/webhook", Events: []string{"backup.started"}}, "other": {Type: "discord", URLFile: "/run/secrets/discord", Events: []string{"backup.failed"}}}
	j := c.Jobs["hourly"]
	j.Notify = []string{"alerts", "other"}
	c.Jobs["hourly"] = j
	if _, err := s.Activate(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	jobs, err := s.Jobs(context.Background())
	if err != nil || len(jobs) != 1 {
		t.Fatal(err)
	}
	return s, jobs[0].ID, c
}

// This inserts a test-owned event row to exercise the future executor's outbox
// seam. No M01 public operation emits a backup event or calls a provider.
func fixtureDelivery(t *testing.T, s *Store, r Run) int64 {
	t.Helper()
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "INSERT INTO events(run_id,sequence,event_type,payload,occurred_at) VALUES(?,100,'backup.started','{}',1)", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if err = routeEventTx(ctx, tx, id, r.JobRevisionID, "backup.started"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestRoutesUseChannelRevision(t *testing.T) {
	s, job, c := channelFixture(t)
	r := beginFixture(t, s, job, 1)
	event := fixtureDelivery(t, s, r)
	countRows(t, s, "deliveries", 1)
	var original string
	if err := s.db.QueryRow("SELECT channel_revision_id FROM deliveries WHERE event_id=?", event).Scan(&original); err != nil {
		t.Fatal(err)
	}
	ch := c.Notifications.Channels["alerts"]
	ch.URLFile = "/run/secrets/rotated"
	c.Notifications.Channels["alerts"] = ch
	if _, err := s.Activate(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	var after string
	var state string
	if err := s.db.QueryRow("SELECT channel_revision_id,state FROM deliveries WHERE event_id=?", event).Scan(&after, &state); err != nil || after != original || state != "pending" {
		t.Fatal("historical route rewritten", err)
	}
	jobs, err := s.Jobs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	r2 := beginFixture(t, s, jobs[0].ID, 2)
	event2 := fixtureDelivery(t, s, r2)
	if err = s.db.QueryRow("SELECT channel_revision_id FROM deliveries WHERE event_id=?", event2).Scan(&after); err != nil || after == original {
		t.Fatal("new route did not bind active revision", err)
	}
	countRows(t, s, "deliveries", 2)
}

func TestDisabledChannelCancelsPending(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(fmt.Sprint(remove), func(t *testing.T) {
			s, job, c := channelFixture(t)
			r := beginFixture(t, s, job, 1)
			fixtureDelivery(t, s, r)
			if remove {
				delete(c.Notifications.Channels, "alerts")
				j := c.Jobs["hourly"]
				j.Notify = []string{"other"}
				c.Jobs["hourly"] = j
			} else {
				ch := c.Notifications.Channels["alerts"]
				ch.Enabled = new(false)
				c.Notifications.Channels["alerts"] = ch
			}
			if _, err := s.Activate(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			var state string
			if err := s.db.QueryRow("SELECT state FROM deliveries").Scan(&state); err != nil || state != "cancelled" {
				t.Fatal("pending route survived channel removal", state, err)
			}
			countRows(t, s, "events", 2)
		})
	}
}

func TestInternalEventsNeverRoute(t *testing.T) {
	s, job, _ := channelFixture(t)
	r := beginFixture(t, s, job, 1)
	if _, err := s.Move(context.Background(), Transition{RunID: r.ID, From: "queued", To: "preflight", Version: 1}); err != nil {
		t.Fatal(err)
	}
	countRows(t, s, "events", 2)
	countRows(t, s, "deliveries", 0)
}

func TestDeliveryFailureRollsBackEvent(t *testing.T) {
	s, job, _ := channelFixture(t)
	r := beginFixture(t, s, job, 1)
	ctx := context.Background()
	if _, err := s.db.Exec(`CREATE TRIGGER fail_delivery BEFORE INSERT ON deliveries BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "INSERT INTO events(run_id,sequence,event_type,payload,occurred_at) VALUES(?,100,'backup.started','{}',1)", r.ID)
	if err != nil {
		t.Fatal(err)
	}
	event, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	err = routeEventTx(ctx, tx, event, r.JobRevisionID, "backup.started")
	requireCode(t, err, safeerr.Unavailable)
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	countRows(t, s, "events", 1)
	countRows(t, s, "deliveries", 0)
}

func TestRecoverIsIdempotent(t *testing.T) {
	s, job := newFixture(t)
	ctx := context.Background()
	for i, state := range []string{"queued", "preflight", "dumping"} {
		r := beginFixture(t, s, job, i+1)
		if _, err := s.db.Exec("UPDATE runs SET state=? WHERE id=?", state, r.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	countRows(t, s, "events", 6)
	var n int
	if err := s.db.QueryRow("SELECT count(*) FROM runs WHERE state='interrupted' AND version=2 AND finished_at IS NOT NULL").Scan(&n); err != nil || n != 3 {
		t.Fatal("lost recovery", n, err)
	}
	r := beginFixture(t, s, job, 4)
	if _, err := s.db.Exec("UPDATE runs SET state='delivering' WHERE id=?", r.ID); err != nil {
		t.Fatal(err)
	}
	requireCode(t, s.Recover(ctx), safeerr.Unavailable)
	got, err := s.GetRun(ctx, r.ID)
	if err != nil || got.State != "delivering" || got.Version != 1 {
		t.Fatal("unreconciled delivery mutated", err)
	}
}

func TestRecoverEventFailureRollsBackAll(t *testing.T) {
	s, job := newFixture(t)
	beginFixture(t, s, job, 1)
	beginFixture(t, s, job, 2)
	if _, err := s.db.Exec(`CREATE TRIGGER fail_recovery BEFORE INSERT ON events WHEN NEW.sequence=2 BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	requireCode(t, s.Recover(context.Background()), safeerr.Unavailable)
	var n int
	if err := s.db.QueryRow("SELECT count(*) FROM runs WHERE state='queued' AND version=1").Scan(&n); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	countRows(t, s, "events", 2)
}

func TestWriteProbeFullDisk(t *testing.T) {
	s, job := newFixture(t)
	ctx := context.Background()
	if err := s.ProbeWrite(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("CREATE TABLE fixture_fill(data BLOB)"); err != nil {
		t.Fatal(err)
	}
	var pages int
	if err := s.db.QueryRow("PRAGMA page_count").Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(fmt.Sprintf("PRAGMA max_page_count=%d", pages)); err != nil {
		t.Fatal(err)
	}
	// Bound allocation to a small private DB, never exhaust the host filesystem.
	var full error
	for i := 0; i < 100; i++ {
		_, full = s.db.Exec("INSERT INTO fixture_fill VALUES(zeroblob(4096))")
		if full != nil {
			break
		}
	}
	var sqliteErr *sqlite.Error
	if !errors.As(full, &sqliteErr) || sqliteErr.Code() != 13 {
		t.Fatalf("want driver SQLITE_FULL(13), got %v", full)
	}
	t.Logf("bounded SQLite max_page_count=%d produced SQLITE_FULL(13)", pages)
	c := fixtureConfig(t)
	source := c.Sources["pg"]
	for i := 0; i < 1000; i++ {
		source.Databases = append(source.Databases, fmt.Sprintf("fixture_database_%04d", i))
	}
	c.Sources["pg"] = source
	_, err := s.Activate(ctx, c)
	requireCode(t, err, safeerr.Unavailable)
	countRows(t, s, "config_revisions", 1)
	if _, err = s.db.Exec(`CREATE TRIGGER fill_event BEFORE INSERT ON events BEGIN INSERT INTO fixture_fill VALUES(zeroblob(4096)); END`); err != nil {
		t.Fatal(err)
	}
	_, _, err = s.BeginRun(ctx, Begin{ID: strings.Repeat("a", 32), JobRevisionID: job, RequestID: "full", PayloadHash: strings.Repeat("b", 64)})
	requireCode(t, err, safeerr.Unavailable)
	countRows(t, s, "runs", 0)
	countRows(t, s, "events", 0)
	if _, err = s.db.Exec(`CREATE TRIGGER fill_probe BEFORE UPDATE ON health_probe BEGIN INSERT INTO fixture_fill VALUES(zeroblob(4096)); END`); err != nil {
		t.Fatal(err)
	}
	requireCode(t, s.ProbeWrite(ctx), safeerr.Unavailable)
}

func TestWriteProbeInjectedFailure(t *testing.T) {
	s, _ := newFixture(t)
	ctx := context.Background()
	if _, err := s.db.Exec(`CREATE TRIGGER fail_probe BEFORE UPDATE ON health_probe BEGIN SELECT RAISE(ABORT,'fixture-write-failure'); END`); err != nil {
		t.Fatal(err)
	}
	requireCode(t, s.ProbeWrite(ctx), safeerr.Unavailable)
	var tick int
	if err := s.db.QueryRow("SELECT tick FROM health_probe").Scan(&tick); err != nil || tick != 0 {
		t.Fatal(tick, err)
	}
	if _, err := s.db.Exec("DROP TRIGGER fail_probe"); err != nil {
		t.Fatal(err)
	}
	if err := s.ProbeWrite(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRow("SELECT tick FROM health_probe").Scan(&tick); err != nil || tick != 1 {
		t.Fatal(tick, err)
	}
}

func TestWriteProbeCommitFailure(t *testing.T) {
	s, _ := newFixture(t)
	// A deferred FK rejects COMMIT after UPDATE itself succeeds. This covers
	// the durable-write boundary separately from statement/trigger failures.
	_, err := s.db.Exec(`CREATE TABLE fixture_parent(id INTEGER PRIMARY KEY);
CREATE TABLE fixture_child(parent_id INTEGER REFERENCES fixture_parent(id) DEFERRABLE INITIALLY DEFERRED);
CREATE TRIGGER fail_commit AFTER UPDATE ON health_probe BEGIN INSERT INTO fixture_child VALUES(99); END`)
	if err != nil {
		t.Fatal(err)
	}
	requireCode(t, s.ProbeWrite(context.Background()), safeerr.Unavailable)
	var tick int
	if err = s.db.QueryRow("SELECT tick FROM health_probe").Scan(&tick); err != nil || tick != 0 {
		t.Fatal("failed commit survived", tick, err)
	}
	if _, err = s.db.Exec("DROP TRIGGER fail_commit"); err != nil {
		t.Fatal(err)
	}
	if err = s.ProbeWrite(context.Background()); err != nil {
		t.Fatal("connection could not recover after failed commit", err)
	}
}

// Keep the transaction type explicit at this seam, preventing accidental calls
// to a Store method while its sole connection is already leased.
var _ func(context.Context, *sql.Tx, int64, string, string) error = routeEventTx
