package state

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"bkplight.local/bkplight/internal/safeerr"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type ArtifactIntent struct {
	ID, RunID, SourceID, DatabaseScope, ScopeKind string
	Engine, ToolVersion, Consistency, KeyID       string
	CapturedFrom                                  time.Time
}

type ArtifactStage struct {
	ID, RelativePath, CiphertextSHA256, ManifestRef, ManifestSHA256 string
	CiphertextSize, ManifestSize                                    int64
	CapturedTo                                                      time.Time
}

type Artifact struct {
	ArtifactIntent
	RelativePath, CiphertextSHA256, ManifestRef, ManifestSHA256 string
	CiphertextSize, ManifestSize                                int64
	CapturedTo                                                  time.Time
	State                                                       string
}

const artifactColumns = "id,run_id,source_id,database_scope,scope_kind,engine,tool_version,consistency,key_id,captured_from,captured_to,state,relative_path,ciphertext_size,ciphertext_sha256,manifest_ref,manifest_size,manifest_sha256"

type ArtifactVerifier interface {
	Verify(name string, size int64, digest string) error
}

func validArtifactIntent(a ArtifactIntent) bool {
	if !validHex(a.ID, 32) || !validHex(a.RunID, 32) || a.SourceID == "" || len(a.SourceID) > 128 || a.DatabaseScope == "" || len(a.DatabaseScope) > 256 || a.KeyID == "" || len(a.KeyID) > 128 || a.ToolVersion == "" || len(a.ToolVersion) > 128 || a.CapturedFrom.IsZero() {
		return false
	}
	if a.ScopeKind != "database" && a.ScopeKind != "globals" {
		return false
	}
	if a.Engine == "postgres" {
		return a.Consistency == "postgres_per_database"
	}
	return a.Engine == "mongo" && a.ScopeKind == "database" && a.Consistency == "mongo_live_not_guaranteed"
}

func (s *Store) BeginArtifact(ctx context.Context, a ArtifactIntent) error {
	if !validArtifactIntent(a) {
		return invalid("artifact")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO artifacts(id,run_id,source_id,database_scope,scope_kind,engine,tool_version,consistency,key_id,captured_from,state) VALUES(?,?,?,?,?,?,?,?,?,?,'writing')`, a.ID, a.RunID, a.SourceID, a.DatabaseScope, a.ScopeKind, a.Engine, a.ToolVersion, a.Consistency, a.KeyID, a.CapturedFrom.UTC().UnixMicro())
	if err != nil {
		var databaseErr *sqlite.Error
		if errors.As(err, &databaseErr) && (databaseErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE || databaseErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY) {
			return conflict()
		}
		return unavailable()
	}
	return nil
}

func validManifestName(name string) bool {
	if name == "" || len(name) > 128 || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, "\\\x00\r\n") {
		return false
	}
	return true
}

func (s *Store) StageArtifact(ctx context.Context, a ArtifactStage, files ArtifactVerifier) error {
	if files == nil || !validHex(a.ID, 32) || a.RelativePath != a.ID+".age" || a.CiphertextSize <= 0 || !validHex(a.CiphertextSHA256, 64) || !validManifestName(a.ManifestRef) || a.ManifestSize <= 0 || !validHex(a.ManifestSHA256, 64) || a.CapturedTo.IsZero() {
		return invalid("artifact_stage")
	}
	if err := files.Verify(a.RelativePath, a.CiphertextSize, a.CiphertextSHA256); err != nil {
		return unavailable()
	}
	if err := files.Verify(a.ManifestRef, a.ManifestSize, a.ManifestSHA256); err != nil {
		return unavailable()
	}
	result, err := s.db.ExecContext(ctx, `UPDATE artifacts SET state='staged',captured_to=?,relative_path=?,ciphertext_size=?,ciphertext_sha256=?,manifest_ref=?,manifest_size=?,manifest_sha256=? WHERE id=? AND state='writing' AND captured_from<=?`, a.CapturedTo.UTC().UnixMicro(), a.RelativePath, a.CiphertextSize, a.CiphertextSHA256, a.ManifestRef, a.ManifestSize, a.ManifestSHA256, a.ID, a.CapturedTo.UTC().UnixMicro())
	if err != nil {
		return unavailable()
	}
	n, err := result.RowsAffected()
	if err != nil {
		return unavailable()
	}
	if n != 1 {
		return conflict()
	}
	return nil
}

func (s *Store) GetArtifact(ctx context.Context, id string) (Artifact, error) {
	if !validHex(id, 32) {
		return Artifact{}, invalid("artifact_id")
	}
	var a Artifact
	var from int64
	var to sql.NullInt64
	var path, hash, manifest, manifestHash sql.NullString
	var size, manifestSize sql.NullInt64
	err := s.db.QueryRowContext(ctx, "SELECT "+artifactColumns+" FROM artifacts WHERE id=?", id).Scan(&a.ID, &a.RunID, &a.SourceID, &a.DatabaseScope, &a.ScopeKind, &a.Engine, &a.ToolVersion, &a.Consistency, &a.KeyID, &from, &to, &a.State, &path, &size, &hash, &manifest, &manifestSize, &manifestHash)
	if errors.Is(err, sql.ErrNoRows) {
		return Artifact{}, &safeerr.Error{Code: safeerr.NotFound}
	}
	if err != nil {
		return Artifact{}, unavailable()
	}
	a.CapturedFrom = time.UnixMicro(from).UTC()
	if to.Valid {
		a.CapturedTo = time.UnixMicro(to.Int64).UTC()
	}
	a.RelativePath, a.CiphertextSHA256, a.ManifestRef, a.ManifestSHA256 = path.String, hash.String, manifest.String, manifestHash.String
	a.CiphertextSize = size.Int64
	a.ManifestSize = manifestSize.Int64
	return a, nil
}

// ReconcileArtifacts removes staged eligibility when either encrypted file no
// longer matches its committed receipt. It leaves bytes for operator diagnosis.
func (s *Store) ReconcileArtifacts(ctx context.Context, files ArtifactVerifier) error {
	if files == nil {
		return invalid("artifact_verifier")
	}
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM artifacts WHERE state='staged' ORDER BY id")
	if err != nil {
		return unavailable()
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return unavailable()
		}
		ids = append(ids, id)
	}
	rowErr := rows.Err()
	closeErr := rows.Close()
	if rowErr != nil || closeErr != nil {
		return unavailable()
	}
	for _, id := range ids {
		a, err := s.GetArtifact(ctx, id)
		if err != nil {
			return err
		}
		if files.Verify(a.RelativePath, a.CiphertextSize, a.CiphertextSHA256) == nil && files.Verify(a.ManifestRef, a.ManifestSize, a.ManifestSHA256) == nil {
			continue
		}
		result, err := s.db.ExecContext(ctx, "UPDATE artifacts SET state='interrupted' WHERE id=? AND state='staged'", id)
		if err != nil {
			return unavailable()
		}
		n, err := result.RowsAffected()
		if err != nil || n != 1 {
			return unavailable()
		}
	}
	return nil
}
