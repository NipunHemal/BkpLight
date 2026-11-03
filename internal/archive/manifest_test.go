package archive

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
)

func TestWriteManifestEncryptsAndBindsArtifactScope(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "staging")
	identity, key := recipient(t)
	stager, err := OpenStager(Options{Directory: dir, MaxBytes: 1 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	archiveSession, err := stager.Begin(context.Background(), fixtureID, []string{key}, 0)
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
	start := time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC)
	m := Manifest{
		Version: 1, ArtifactID: fixtureID, RunID: "abcdef0123456789abcdef0123456789",
		SourceID: "pg", DatabaseScope: "app", ScopeKind: "database", Engine: "postgres",
		ToolVersion: "pg_dump 17.1", Consistency: "postgres_per_database", KeyID: "fixture-key",
		CapturedFrom: start, CapturedTo: start.Add(time.Second), Archive: archiveFile,
	}
	manifestFile, err := WriteManifest(context.Background(), stager, []string{key}, m)
	if err != nil {
		t.Fatal(err)
	}
	if manifestFile.RelativePath == archiveFile.RelativePath || manifestFile.CiphertextSize <= 0 {
		t.Fatal("manifest did not get a separate encrypted artifact")
	}
	ciphertext, err := os.ReadFile(filepath.Join(dir, manifestFile.RelativePath))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte("postgres_per_database")) || bytes.Contains(ciphertext, []byte("app")) {
		t.Fatal("manifest metadata persisted in plaintext")
	}
	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	var got Manifest
	if err := json.Unmarshal(plain, &got); err != nil {
		t.Fatal(err)
	}
	if got != m {
		t.Fatalf("manifest lost scope or receipt: %+v", got)
	}
	if err := stager.Verify(manifestFile.RelativePath, manifestFile.CiphertextSize, manifestFile.CiphertextSHA256); err != nil {
		t.Fatal(err)
	}
}

func TestWriteManifestRejectsUnverifiedArchiveReceipt(t *testing.T) {
	_, key := recipient(t)
	stager, err := OpenStager(Options{Directory: filepath.Join(t.TempDir(), "staging"), MaxBytes: 4096})
	if err != nil {
		t.Fatal(err)
	}
	defer stager.Close()
	start := time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC)
	m := Manifest{
		Version: 1, ArtifactID: fixtureID, RunID: "abcdef0123456789abcdef0123456789",
		SourceID: "pg", DatabaseScope: "app", ScopeKind: "database", Engine: "postgres",
		ToolVersion: "pg_dump 17.1", Consistency: "postgres_per_database", KeyID: "fixture-key",
		CapturedFrom: start, CapturedTo: start.Add(time.Second),
		Archive: ArtifactFile{RelativePath: fixtureID + ".age", CiphertextSize: 10, CiphertextSHA256: strings.Repeat("a", 64)},
	}
	if _, err := WriteManifest(context.Background(), stager, []string{key}, m); err == nil {
		t.Fatal("manifest accepted an archive that does not exist")
	}
}
