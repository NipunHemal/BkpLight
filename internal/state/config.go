package state

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"

	"bkplight.local/bkplight/internal/config"
	"bkplight.local/bkplight/internal/safeerr"
)

func revisionID(configID, id string) string {
	sum := sha256.Sum256([]byte(configID + "\x00" + id))
	return hex.EncodeToString(sum[:])
}
func enabled(value *bool) bool { return value == nil || *value }

// Activate retains immutable, reference-only snapshots. It does no secret IO.
func (s *Store) Activate(ctx context.Context, c config.Config) (string, error) {
	id, snapshot, err := config.Revision(c)
	if err != nil {
		return "", err
	}
	var normalized config.Config
	if json.Unmarshal(snapshot, &normalized) != nil {
		return "", unavailable()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", unavailable()
	}
	defer tx.Rollback()
	if err = immutable(ctx, tx, "config_revisions", id, snapshot); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO config_revisions(id,snapshot,created_at) VALUES(?,?,?)", id, snapshot, s.now().UTC().UnixMicro()); err != nil {
		return "", unavailable()
	}
	for jobID, j := range normalized.Jobs {
		data, err := json.Marshal(j)
		if err != nil {
			return "", unavailable()
		}
		jid := revisionID(id, jobID)
		if err = immutable(ctx, tx, "job_revisions", jid, data); err != nil {
			return "", err
		}
		if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO job_revisions(id,job_id,source_id,config_id,snapshot) VALUES(?,?,?,?,?)", jid, jobID, j.Source, id, data); err != nil {
			return "", unavailable()
		}
	}
	for channelID, ch := range normalized.Notifications.Channels {
		data, err := json.Marshal(ch)
		if err != nil {
			return "", unavailable()
		}
		cid := revisionID(id, channelID)
		if err = immutable(ctx, tx, "channel_revisions", cid, data); err != nil {
			return "", err
		}
		if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO channel_revisions(id,config_id,channel_id,snapshot) VALUES(?,?,?,?)", cid, id, channelID, data); err != nil {
			return "", unavailable()
		}
	}
	// Materialize IDs before updating on the single SQLite connection.
	rows, err := tx.QueryContext(ctx, "SELECT DISTINCT channel_id FROM channel_revisions")
	if err != nil {
		return "", unavailable()
	}
	var removed []string
	for rows.Next() {
		var channelID string
		if err = rows.Scan(&channelID); err != nil {
			rows.Close()
			return "", unavailable()
		}
		ch, ok := normalized.Notifications.Channels[channelID]
		if !ok || !enabled(ch.Enabled) {
			removed = append(removed, channelID)
		}
	}
	rowErr := rows.Err()
	closeErr := rows.Close()
	if rowErr != nil || closeErr != nil {
		return "", unavailable()
	}
	for _, channelID := range removed {
		if _, err = tx.ExecContext(ctx, "UPDATE deliveries SET state='cancelled' WHERE state='pending' AND channel_revision_id IN (SELECT id FROM channel_revisions WHERE channel_id=?)", channelID); err != nil {
			return "", unavailable()
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO active_config(singleton,config_id) VALUES(1,?) ON CONFLICT(singleton) DO UPDATE SET config_id=excluded.config_id", id); err != nil {
		return "", unavailable()
	}
	if err = tx.Commit(); err != nil {
		return "", unavailable()
	}
	return id, nil
}

// table is a package-owned SQL identifier, never supplied by a caller.
func immutable(ctx context.Context, tx *sql.Tx, table, id string, snapshot []byte) error {
	var old []byte
	err := tx.QueryRowContext(ctx, "SELECT snapshot FROM "+table+" WHERE id=?", id).Scan(&old)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return unavailable()
	}
	if !bytes.Equal(old, snapshot) {
		return &safeerr.Error{Code: safeerr.Conflict}
	}
	return nil
}

func (s *Store) Jobs(ctx context.Context) ([]JobRevision, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT j.id,j.job_id,j.source_id,j.config_id,j.snapshot FROM job_revisions j JOIN active_config a ON a.config_id=j.config_id ORDER BY j.job_id")
	if err != nil {
		return nil, unavailable()
	}
	defer rows.Close()
	result := make([]JobRevision, 0)
	for rows.Next() {
		var j JobRevision
		var value config.Job
		if err = rows.Scan(&j.ID, &j.JobID, &j.SourceID, &j.ConfigID, &j.Snapshot); err != nil {
			return nil, unavailable()
		}
		if json.Unmarshal(j.Snapshot, &value) != nil {
			return nil, unavailable()
		}
		if enabled(value.Enabled) {
			result = append(result, j)
		}
	}
	if rows.Err() != nil {
		return nil, unavailable()
	}
	if rows.Close() != nil {
		return nil, unavailable()
	}
	return result, nil
}
