package state

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"bkplight.local/bkplight/internal/config"
	"bkplight.local/bkplight/internal/safeerr"
)

func TestConfigHistorySurvivesRemoval(t *testing.T) {
	s, job := newFixture(t)
	ctx := context.Background()
	r := beginFixture(t, s, job, 1)
	c := fixtureConfig(t)
	id, snapshot, err := config.Revision(c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Activate(ctx, c)
	if err != nil || got != id {
		t.Fatal(got, err)
	}
	countRows(t, s, "config_revisions", 1)
	countRows(t, s, "job_revisions", 1)
	var stored []byte
	if err = s.db.QueryRow("SELECT snapshot FROM config_revisions WHERE id=?", id).Scan(&stored); err != nil || !bytes.Equal(stored, snapshot) {
		t.Fatal("snapshot mismatch", err)
	}
	if job != fmt.Sprintf("%x", sha256.Sum256([]byte(id+"\x00hourly"))) {
		t.Fatal("unstable job revision ID")
	}
	c.Jobs = map[string]config.Job{}
	if _, err = s.Activate(ctx, c); err != nil {
		t.Fatal(err)
	}
	jobs, err := s.Jobs(ctx)
	if err != nil || len(jobs) != 0 {
		t.Fatal(jobs, err)
	}
	historical, err := s.GetRun(ctx, r.ID)
	if err != nil || historical != r {
		t.Fatal("history lost", err)
	}
	countRows(t, s, "config_revisions", 2)
	countRows(t, s, "job_revisions", 1)
}

func TestActivateRejectsSnapshotCollisionAndRollsBack(t *testing.T) {
	s, err := Open(context.Background(), Options{Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	c := fixtureConfig(t)
	id, _, err := config.Revision(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("INSERT INTO config_revisions VALUES(?,?,?)", id, []byte(`{"fixture":"different"}`), 1); err != nil {
		t.Fatal(err)
	}
	_, err = s.Activate(ctx, c)
	requireCode(t, err, safeerr.Conflict)
	countRows(t, s, "active_config", 0)
	countRows(t, s, "job_revisions", 0)
}

func TestActivateAtomicOnJobFailure(t *testing.T) {
	s, job := newFixture(t)
	ctx := context.Background()
	if _, err := s.db.Exec(`CREATE TRIGGER fail_job BEFORE INSERT ON job_revisions BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	c := fixtureConfig(t)
	j := c.Jobs["hourly"]
	j.Schedule.Cron = "1 * * * *"
	c.Jobs["hourly"] = j
	_, err := s.Activate(ctx, c)
	requireCode(t, err, safeerr.Unavailable)
	jobs, err := s.Jobs(ctx)
	if err != nil || len(jobs) != 1 || jobs[0].ID != job {
		t.Fatal("active config changed after rollback", err)
	}
	countRows(t, s, "config_revisions", 1)
}
