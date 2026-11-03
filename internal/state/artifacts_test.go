package state

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bkplight.local/bkplight/internal/archive"
	"bkplight.local/bkplight/internal/safeerr"
	"filippo.io/age"
)

func fixtureArtifact(runID, id, scopeKind, scope string) ArtifactIntent {
	return ArtifactIntent{
		ID: id, RunID: runID, SourceID: "pg", ScopeKind: scopeKind,
		DatabaseScope: scope, Engine: "postgres", ToolVersion: "pg_dump 17.1",
		Consistency: "postgres_per_database", KeyID: "fixture-key",
		CapturedFrom: time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC),
	}
}

func TestArtifactIntentAndStageRequireManifest(t *testing.T) {
	s, job := newFixture(t)
	run := beginFixture(t, s, job, 101)
	intent := fixtureArtifact(run.ID, strings.Repeat("a", 32), "database", "app")
	if err := s.BeginArtifact(context.Background(), intent); err != nil {
		t.Fatal(err)
	}
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	stagingDir := filepath.Join(t.TempDir(), "staging")
	stager, err := archive.OpenStager(archive.Options{Directory: stagingDir, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	archiveSession, err := stager.Begin(context.Background(), intent.ID, []string{identity.Recipient().String()}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := archiveSession.Write([]byte("fixture dump")); err != nil {
		t.Fatal(err)
	}
	archiveFile, err := archiveSession.Finalize()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.StageArtifact(context.Background(), ArtifactStage{ID: intent.ID, RelativePath: archiveFile.RelativePath, CiphertextSize: archiveFile.CiphertextSize, CiphertextSHA256: archiveFile.CiphertextSHA256}, stager); err == nil {
		t.Fatal("staged artifact accepted without completed manifest")
	}
	manifestFile, err := archive.WriteManifest(context.Background(), stager, []string{identity.Recipient().String()}, archive.Manifest{
		Version: 1, ArtifactID: intent.ID, RunID: intent.RunID, SourceID: intent.SourceID,
		DatabaseScope: intent.DatabaseScope, ScopeKind: intent.ScopeKind, Engine: intent.Engine,
		ToolVersion: intent.ToolVersion, Consistency: intent.Consistency, KeyID: intent.KeyID,
		CapturedFrom: intent.CapturedFrom, CapturedTo: intent.CapturedFrom.Add(time.Second), Archive: archiveFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	stage := ArtifactStage{
		ID: intent.ID, RelativePath: archiveFile.RelativePath, CiphertextSize: archiveFile.CiphertextSize,
		CiphertextSHA256: archiveFile.CiphertextSHA256, ManifestRef: manifestFile.RelativePath,
		ManifestSize: manifestFile.CiphertextSize, ManifestSHA256: manifestFile.CiphertextSHA256,
		CapturedTo: intent.CapturedFrom.Add(time.Second),
	}
	if err := s.StageArtifact(context.Background(), stage, stager); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetArtifact(context.Background(), intent.ID)
	if err != nil || got.State != "staged" || got.Consistency != "postgres_per_database" || got.ManifestRef != manifestFile.RelativePath {
		t.Fatalf("staged artifact mismatch: %+v %v", got, err)
	}
	if err := s.StageArtifact(context.Background(), stage, stager); err == nil {
		t.Fatal("duplicate stage accepted")
	}
	if err := os.WriteFile(filepath.Join(stagingDir, archiveFile.RelativePath), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileArtifacts(context.Background(), stager); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetArtifact(context.Background(), intent.ID)
	if err != nil || got.State != "interrupted" {
		t.Fatalf("tampered staged artifact still eligible: %+v %v", got, err)
	}
}

func TestArtifactScopeIsUniquePerRun(t *testing.T) {
	s, job := newFixture(t)
	run := beginFixture(t, s, job, 102)
	first := fixtureArtifact(run.ID, strings.Repeat("a", 32), "database", "app")
	if err := s.BeginArtifact(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	duplicate := fixtureArtifact(run.ID, strings.Repeat("b", 32), "database", "app")
	if err := s.BeginArtifact(context.Background(), duplicate); err == nil {
		t.Fatal("duplicate database scope accepted")
	}
	globals := fixtureArtifact(run.ID, strings.Repeat("c", 32), "globals", "app")
	if err := s.BeginArtifact(context.Background(), globals); err != nil {
		t.Fatal(err)
	}
}

func TestArtifactWritingInterruptedOnRecovery(t *testing.T) {
	s, job := newFixture(t)
	run := beginFixture(t, s, job, 103)
	intent := fixtureArtifact(run.ID, strings.Repeat("d", 32), "database", "app")
	if err := s.BeginArtifact(context.Background(), intent); err != nil {
		t.Fatal(err)
	}
	if err := s.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetArtifact(context.Background(), intent.ID)
	if err != nil || got.State != "interrupted" {
		t.Fatalf("writing artifact survived recovery: %+v %v", got, err)
	}
}

func TestArtifactIntentDatabaseFailureIsNotAConflict(t *testing.T) {
	s, job := newFixture(t)
	run := beginFixture(t, s, job, 104)
	if _, err := s.db.Exec(`CREATE TRIGGER deny_artifact BEFORE INSERT ON artifacts BEGIN SELECT RAISE(ABORT,'synthetic failure'); END`); err != nil {
		t.Fatal(err)
	}
	intent := fixtureArtifact(run.ID, strings.Repeat("f", 32), "database", "app")
	if err := s.BeginArtifact(context.Background(), intent); err == nil || safeerr.Public(err).Code != safeerr.Unavailable {
		t.Fatalf("database failure must be unavailable, got %v", err)
	}
	countRows(t, s, "artifacts", 0)
}
