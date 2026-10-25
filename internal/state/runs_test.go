package state

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"bkplight.local/bkplight/internal/config"
	"bkplight.local/bkplight/internal/safeerr"
	"go.yaml.in/yaml/v3"
)

func fixtureConfig(t *testing.T) config.Config {
	t.Helper()
	raw, err := os.ReadFile("../config/testdata/valid.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := config.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c.Sources["pg"] = config.Source{Type: "postgres", Host: "fixture-pg", Port: 5432, CredentialsFile: "/run/secrets/pg", Databases: []string{"app"}}
	c.Destinations["r2"] = config.Destination{Type: "r2", Endpoint: "https://example.invalid", Bucket: "fixture", Prefix: "test/", CredentialsFile: "/run/secrets/r2"}
	c.Jobs["hourly"] = config.Job{Source: "pg", Schedule: config.Schedule{Cron: "0 * * * *"}, Destinations: []config.JobDestination{{ID: "r2", Required: true}}, Retention: config.Retention{KeepLast: 5, MinSuccessful: 1}}
	c.Service.StagingMaxBytes = 1048576
	c.Service.MinFreeBytes = 65536
	c.Encryption = config.Encryption{Recipients: []string{"synthetic-public-recipient"}, SigningKeyFile: "/run/secrets/signing"}
	raw, err = yaml.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	c, err = config.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func newFixture(t *testing.T) (*Store, string) {
	t.Helper()
	s, err := Open(context.Background(), Options{Directory: t.TempDir(), Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err = s.Activate(context.Background(), fixtureConfig(t)); err != nil {
		t.Fatal(err)
	}
	jobs, err := s.Jobs(context.Background())
	if err != nil || len(jobs) != 1 {
		t.Fatal("missing fixture job", err)
	}
	return s, jobs[0].ID
}

func beginFixture(t *testing.T, s *Store, job string, n int) Run {
	t.Helper()
	r, replay, err := s.BeginRun(context.Background(), Begin{ID: fmt.Sprintf("%032x", n), JobRevisionID: job, RequestID: fmt.Sprintf("req-%d", n), PayloadHash: strings.Repeat("b", 64)})
	if err != nil || replay {
		t.Fatal("begin", replay, err)
	}
	return r
}
func countRows(t *testing.T, s *Store, table string, want int) {
	t.Helper()
	var got int
	if err := s.db.QueryRow("SELECT count(*) FROM " + table).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s rows=%d want %d", table, got, want)
	}
}
func requireCode(t *testing.T, err error, code safeerr.Code) {
	t.Helper()
	if err == nil || safeerr.Public(err).Code != code {
		t.Fatalf("error=%v want %s", err, code)
	}
}

func TestRunAndOutboxRollbackTogether(t *testing.T) {
	s, jobID := newFixture(t)
	_, err := s.db.Exec(`CREATE TRIGGER fail_event BEFORE INSERT ON events BEGIN SELECT RAISE(ABORT,'fixture'); END`)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = s.BeginRun(context.Background(), Begin{ID: strings.Repeat("a", 32), JobRevisionID: jobID, RequestID: "req-1", PayloadHash: strings.Repeat("b", 64)})
	if err == nil {
		t.Fatal("expected rollback")
	}
	countRows(t, s, "runs", 0)
	countRows(t, s, "events", 0)
	countRows(t, s, "deliveries", 0)
}

func TestBeginRunReplayAndConflict(t *testing.T) {
	s, job := newFixture(t)
	ctx := context.Background()
	b := Begin{ID: strings.Repeat("a", 32), JobRevisionID: job, RequestID: "req", PayloadHash: strings.Repeat("b", 64)}
	first, replay, err := s.BeginRun(ctx, b)
	if err != nil || replay {
		t.Fatal(err)
	}
	b.ID = strings.Repeat("c", 32)
	got, replay, err := s.BeginRun(ctx, b)
	if err != nil || !replay || got != first {
		t.Fatal("replay mismatch", got, err)
	}
	b.PayloadHash = strings.Repeat("d", 64)
	_, _, err = s.BeginRun(ctx, b)
	requireCode(t, err, safeerr.Conflict)
	b.PayloadHash = first.PayloadHash
	b.JobRevisionID = strings.Repeat("e", 64)
	_, _, err = s.BeginRun(ctx, b)
	requireCode(t, err, safeerr.Conflict)
	countRows(t, s, "runs", 1)
	countRows(t, s, "events", 1)
}

func TestConcurrentBeginRunSingleEvent(t *testing.T) {
	s, job := newFixture(t)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, _, err := s.BeginRun(context.Background(), Begin{ID: fmt.Sprintf("%032x", i+1), JobRevisionID: job, RequestID: "shared", PayloadHash: strings.Repeat("b", 64)})
			if err == nil && r.Version != 1 {
				err = fmt.Errorf("version %d", r.Version)
			}
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	countRows(t, s, "runs", 1)
	countRows(t, s, "events", 1)
}

func TestStaleTransitionRollback(t *testing.T) {
	s, job := newFixture(t)
	r := beginFixture(t, s, job, 1)
	ctx := context.Background()
	moved, err := s.Move(ctx, Transition{RunID: r.ID, From: "queued", To: "preflight", Version: 1})
	if err != nil || moved.Version != 2 {
		t.Fatal(err)
	}
	_, err = s.Move(ctx, Transition{RunID: r.ID, From: "queued", To: "cancelled", Version: 1})
	requireCode(t, err, safeerr.Conflict)
	got, err := s.GetRun(ctx, r.ID)
	if err != nil || got != moved {
		t.Fatal("CAS changed run", err)
	}
	countRows(t, s, "events", 2)
	if _, err = s.db.Exec(`CREATE TRIGGER fail_move BEFORE INSERT ON events BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	_, err = s.Move(ctx, Transition{RunID: r.ID, From: "preflight", To: "failed", Version: 2, ErrorCode: "unavailable"})
	requireCode(t, err, safeerr.Unavailable)
	got, err = s.GetRun(ctx, r.ID)
	if err != nil || got != moved {
		t.Fatal("failed event left partial transition", err)
	}
}

func TestTerminalStateImmutable(t *testing.T) {
	s, job := newFixture(t)
	r := beginFixture(t, s, job, 1)
	ctx := context.Background()
	done, err := s.Move(ctx, Transition{RunID: r.ID, From: "queued", To: "cancelled", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, to := range []string{"preflight", "cancelled", "succeeded"} {
		_, err = s.Move(ctx, Transition{RunID: r.ID, From: "cancelled", To: to, Version: 2})
		requireCode(t, err, safeerr.Conflict)
	}
	got, err := s.GetRun(ctx, r.ID)
	if err != nil || got != done {
		t.Fatal("terminal run mutated", err)
	}
	countRows(t, s, "events", 2)
}

func TestCancelIdempotenceAndRequestNamespace(t *testing.T) {
	s, job := newFixture(t)
	r := beginFixture(t, s, job, 1)
	ctx := context.Background()
	_, err := s.Cancel(ctx, r.ID, "req-1")
	requireCode(t, err, safeerr.Conflict)
	first, err := s.Cancel(ctx, r.ID, "cancel-1")
	if err != nil || first.State != "cancelled" || first.Version != 2 {
		t.Fatal(first, err)
	}
	again, err := s.Cancel(ctx, r.ID, "cancel-1")
	if err != nil || again != first {
		t.Fatal("cancel replay", err)
	}
	_, _, err = s.BeginRun(ctx, Begin{ID: strings.Repeat("d", 32), JobRevisionID: job, RequestID: "cancel-1", PayloadHash: strings.Repeat("b", 64)})
	requireCode(t, err, safeerr.Conflict)
	other := beginFixture(t, s, job, 2)
	_, err = s.Cancel(ctx, other.ID, "cancel-1")
	requireCode(t, err, safeerr.Conflict)
	if _, err = s.Move(ctx, Transition{RunID: other.ID, From: "queued", To: "preflight", Version: 1}); err != nil {
		t.Fatal(err)
	}
	_, err = s.Cancel(ctx, other.ID, "cancel-2")
	requireCode(t, err, safeerr.Conflict)
	countRows(t, s, "run_commands", 1)
	countRows(t, s, "events", 4)
}

func TestReplaySurvivesDisabledJob(t *testing.T) {
	s, job := newFixture(t)
	ctx := context.Background()
	b := Begin{ID: strings.Repeat("a", 32), JobRevisionID: job, RequestID: "req-1", PayloadHash: strings.Repeat("b", 64)}
	first, _, err := s.BeginRun(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	c := fixtureConfig(t)
	c.Jobs = map[string]config.Job{}
	if _, err = s.Activate(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, yes, err := s.BeginRun(ctx, b)
	if err != nil || !yes || got != first {
		t.Fatal("lost accepted request", err)
	}
	b.RequestID = "req-2"
	b.ID = strings.Repeat("c", 32)
	_, _, err = s.BeginRun(ctx, b)
	requireCode(t, err, safeerr.Conflict)
}

func TestHistoryCursorStable(t *testing.T) {
	s, job := newFixture(t)
	s.now = func() time.Time { return time.Unix(100, 0) }
	for i := 1; i <= 5; i++ {
		beginFixture(t, s, job, i)
	}
	ctx := context.Background()
	p, err := s.ListRuns(ctx, 2, "")
	if err != nil || len(p.Items) != 2 || p.Next == "" {
		t.Fatal(p, err)
	}
	if p.Items[0].ID != fmt.Sprintf("%032x", 5) || p.Items[1].ID != fmt.Sprintf("%032x", 4) {
		t.Fatal("unstable tie order")
	}
	beginFixture(t, s, job, 6)
	p2, err := s.ListRuns(ctx, 2, p.Next)
	if err != nil || len(p2.Items) != 2 || p2.Items[0].ID != fmt.Sprintf("%032x", 3) {
		t.Fatal(p2, err)
	}
	p3, err := s.ListRuns(ctx, 2, p2.Next)
	if err != nil || len(p3.Items) != 1 || p3.Next != "" || p3.Items[0].ID != fmt.Sprintf("%032x", 1) {
		t.Fatal(p3, err)
	}
	for _, bad := range []string{"!", "e30", "WzEsImJhZCJd"} {
		_, err = s.ListRuns(ctx, 2, bad)
		requireCode(t, err, safeerr.Invalid)
	}
	for _, limit := range []int{-1, 201} {
		_, err = s.ListRuns(ctx, limit, "")
		requireCode(t, err, safeerr.Invalid)
	}
	_, err = s.GetRun(ctx, strings.Repeat("f", 32))
	requireCode(t, err, safeerr.NotFound)
}

func TestRunValidationNoEffects(t *testing.T) {
	s, job := newFixture(t)
	ctx := context.Background()
	valid := Begin{ID: strings.Repeat("a", 32), JobRevisionID: job, RequestID: "valid", PayloadHash: strings.Repeat("b", 64)}
	for _, field := range []string{"id", "job", "request", "hash"} {
		b := valid
		switch field {
		case "id":
			b.ID = "bad"
		case "job":
			b.JobRevisionID = "bad"
		case "request":
			b.RequestID = "secret value"
		case "hash":
			b.PayloadHash = "bad"
		}
		_, _, err := s.BeginRun(ctx, b)
		requireCode(t, err, safeerr.Invalid)
	}
	countRows(t, s, "runs", 0)
	countRows(t, s, "events", 0)
	r := beginFixture(t, s, job, 1)
	for _, tr := range []Transition{{RunID: r.ID, From: "queued", To: "succeeded", Version: 1}, {RunID: r.ID, From: "queued", To: "unknown", Version: 1}, {RunID: r.ID, From: "queued", To: "interrupted", Version: 1, ErrorCode: "secret value"}} {
		if _, err := s.Move(ctx, tr); err == nil {
			t.Fatal("invalid transition accepted")
		}
	}
	countRows(t, s, "events", 1)
}

func TestHistoryRejectsNullTimestamp(t *testing.T) {
	s, _ := newFixture(t)
	cursor := base64.RawURLEncoding.EncodeToString([]byte(`[null,"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]`))
	_, err := s.ListRuns(context.Background(), 2, cursor)
	requireCode(t, err, safeerr.Invalid)
}

func TestCancelCommandFailureRollsBackRunAndEvent(t *testing.T) {
	s, job := newFixture(t)
	r := beginFixture(t, s, job, 1)
	if _, err := s.db.Exec(`CREATE TRIGGER fail_command BEFORE INSERT ON run_commands BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	_, err := s.Cancel(context.Background(), r.ID, "cancel-1")
	requireCode(t, err, safeerr.Unavailable)
	got, err := s.GetRun(context.Background(), r.ID)
	if err != nil || got != r {
		t.Fatal("cancel left partial state", err)
	}
	countRows(t, s, "events", 1)
	countRows(t, s, "run_commands", 0)
}

func TestExplicitlyDisabledJobRejectsNewWorkButReplays(t *testing.T) {
	s, job := newFixture(t)
	r := beginFixture(t, s, job, 1)
	c := fixtureConfig(t)
	j := c.Jobs["hourly"]
	j.Enabled = new(false)
	c.Jobs["hourly"] = j
	if _, err := s.Activate(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	jobs, err := s.Jobs(context.Background())
	if err != nil || len(jobs) != 0 {
		t.Fatal("disabled job visible", err)
	}
	again, replay, err := s.BeginRun(context.Background(), Begin{ID: r.ID, JobRevisionID: job, RequestID: r.RequestID, PayloadHash: r.PayloadHash})
	if err != nil || !replay || again != r {
		t.Fatal("disabled replay lost", err)
	}
	var disabledID string
	if err = s.db.QueryRow("SELECT id FROM job_revisions WHERE config_id=(SELECT config_id FROM active_config)").Scan(&disabledID); err != nil {
		t.Fatal(err)
	}
	_, _, err = s.BeginRun(context.Background(), Begin{ID: strings.Repeat("f", 32), JobRevisionID: disabledID, RequestID: "new-request", PayloadHash: r.PayloadHash})
	requireCode(t, err, safeerr.Conflict)
	countRows(t, s, "runs", 1)
}
