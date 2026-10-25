package state

import (
	"context"
	"database/sql"
	"encoding/json"

	"bkplight.local/bkplight/internal/config"
)

type transitionPayload struct {
	RunID     string `json:"run_id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Version   int64  `json:"version"`
	ErrorCode string `json:"error_code,omitempty"`
}

// M01 records audit transitions only. Backup lifecycle events belong to later
// real execution; moving a fixture through states does not claim backup success.
func (s *Store) eventTx(ctx context.Context, tx *sql.Tx, r Run, from string) error {
	kind := "operation.transition"
	switch r.State {
	case "queued":
		kind = "operation.queued"
	case "cancelled":
		kind = "operation.cancelled"
	case "interrupted":
		kind = "operation.interrupted"
	}
	payload, err := json.Marshal(transitionPayload{RunID: r.ID, From: from, To: r.State, Version: r.Version, ErrorCode: r.ErrorCode})
	if err != nil {
		return unavailable()
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO events(run_id,sequence,event_type,payload,occurred_at) VALUES(?,?,?,?,?)", r.ID, r.Version, kind, payload, s.now().UTC().UnixMicro())
	if err != nil {
		return unavailable()
	}
	id, err := result.LastInsertId()
	if err != nil {
		return unavailable()
	}
	return routeEventTx(ctx, tx, id, r.JobRevisionID, kind)
}

// routeEventTx belongs inside the producer's state/event transaction. It binds
// each delivery to a persisted active channel revision, never to a live URL.
func routeEventTx(ctx context.Context, tx *sql.Tx, eventID int64, jobRevisionID, eventType string) error {
	switch eventType {
	case "operation.queued", "operation.cancelled", "operation.interrupted", "operation.transition":
		return nil
	case "backup.started", "backup.completed", "backup.degraded", "backup.failed", "upload.completed", "upload.failed", "archive_validation.completed", "archive_validation.failed", "verification.started", "verification.completed", "verification.failed", "restore.started", "restore.completed", "restore.failed", "retention.completed", "retention.failed", "backup.overdue", "storage.low_space":
	default:
		return invalid("event_type")
	}
	var snapshot []byte
	if err := tx.QueryRowContext(ctx, "SELECT snapshot FROM job_revisions WHERE id=?", jobRevisionID).Scan(&snapshot); err != nil {
		return unavailable()
	}
	var job config.Job
	if json.Unmarshal(snapshot, &job) != nil {
		return unavailable()
	}
	notify := make(map[string]bool, len(job.Notify))
	for _, id := range job.Notify {
		notify[id] = true
	}
	if len(notify) == 0 {
		return nil
	}
	rows, err := tx.QueryContext(ctx, "SELECT c.id,c.channel_id,c.snapshot FROM channel_revisions c JOIN active_config a ON a.config_id=c.config_id")
	if err != nil {
		return unavailable()
	}
	var revisions []string
	for rows.Next() {
		var id, channelID string
		var data []byte
		if err = rows.Scan(&id, &channelID, &data); err != nil {
			rows.Close()
			return unavailable()
		}
		if !notify[channelID] {
			continue
		}
		var ch config.ChannelConfig
		if json.Unmarshal(data, &ch) != nil {
			rows.Close()
			return unavailable()
		}
		if !enabled(ch.Enabled) {
			continue
		}
		for _, subscribed := range ch.Events {
			if subscribed == eventType {
				revisions = append(revisions, id)
				break
			}
		}
	}
	rowErr := rows.Err()
	closeErr := rows.Close()
	if rowErr != nil || closeErr != nil {
		return unavailable()
	}
	for _, id := range revisions {
		if _, err = tx.ExecContext(ctx, "INSERT INTO deliveries(event_id,channel_revision_id,state) VALUES(?,?,'pending')", eventID, id); err != nil {
			return unavailable()
		}
	}
	return nil
}
