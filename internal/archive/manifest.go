package archive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"bkplight.local/bkplight/internal/safeerr"
)

// Manifest is encrypted separately from the native archive so future storage
// adapters can upload and authenticate both components independently.
type Manifest struct {
	Version       int          `json:"version"`
	ArtifactID    string       `json:"artifact_id"`
	RunID         string       `json:"run_id"`
	SourceID      string       `json:"source_id"`
	DatabaseScope string       `json:"database_scope"`
	ScopeKind     string       `json:"scope_kind"`
	Engine        string       `json:"engine"`
	ToolVersion   string       `json:"tool_version"`
	Consistency   string       `json:"consistency"`
	KeyID         string       `json:"key_id"`
	CapturedFrom  time.Time    `json:"captured_from"`
	CapturedTo    time.Time    `json:"captured_to"`
	Archive       ArtifactFile `json:"archive"`
}

func validManifest(m Manifest) bool {
	if m.Version != 1 || !validArtifactID(m.ArtifactID) || !validArtifactID(m.RunID) || m.SourceID == "" || m.DatabaseScope == "" || m.ToolVersion == "" || m.KeyID == "" || m.CapturedFrom.IsZero() || m.CapturedTo.Before(m.CapturedFrom) || m.Archive.RelativePath != m.ArtifactID+".age" || m.Archive.CiphertextSize <= 0 || !validSHA256(m.Archive.CiphertextSHA256) {
		return false
	}
	if m.ScopeKind != "database" && m.ScopeKind != "globals" {
		return false
	}
	if m.Engine == "postgres" {
		return m.Consistency == "postgres_per_database"
	}
	return m.Engine == "mongo" && m.ScopeKind == "database" && m.Consistency == "mongo_live_not_guaranteed"
}

// WriteManifest persists only age ciphertext; the plaintext JSON exists only
// in memory and is bound to the finalized archive receipt.
func WriteManifest(ctx context.Context, s *Stager, recipients []string, m Manifest) (ArtifactFile, error) {
	if s == nil || !validManifest(m) {
		return ArtifactFile{}, stageError(safeerr.Invalid)
	}
	if err := s.Verify(m.Archive.RelativePath, m.Archive.CiphertextSize, m.Archive.CiphertextSHA256); err != nil {
		return ArtifactFile{}, stageError(safeerr.Unavailable)
	}
	plain, err := json.Marshal(m)
	if err != nil {
		return ArtifactFile{}, stageError(safeerr.Invalid)
	}
	sum := sha256.Sum256([]byte("bkplight-manifest-v1\x00" + m.ArtifactID))
	id := hex.EncodeToString(sum[:16])
	session, err := s.Begin(ctx, id, recipients, int64(len(plain))+512)
	if err != nil {
		return ArtifactFile{}, err
	}
	defer session.Abort()
	if _, err := session.Write(plain); err != nil {
		return ArtifactFile{}, stageError(safeerr.Unavailable)
	}
	file, err := session.Finalize()
	if err != nil {
		return ArtifactFile{}, err
	}
	return file, nil
}
