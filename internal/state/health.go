package state

import (
	"context"
)

// ProbeWrite proves that a durable transaction can commit, rather than treating
// a readable database as writable. No in-memory readiness flag can hide failure.
func (s *Store) ProbeWrite(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return unavailable()
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "UPDATE health_probe SET tick=CASE WHEN tick=9223372036854775807 THEN 0 ELSE tick+1 END WHERE singleton=1")
	if err != nil {
		return unavailable()
	}
	n, err := result.RowsAffected()
	if err != nil || n != 1 {
		return unavailable()
	}
	if err = tx.Commit(); err != nil {
		return unavailable()
	}
	return nil
}

// Recover interrupts only work wholly owned by M01. An ambiguous delivering
// state remains untouched and blocks readiness until provider reconciliation.
func (s *Store) Recover(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return unavailable()
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT "+runColumns+" FROM runs WHERE state IN ('queued','preflight','dumping') ORDER BY id")
	if err != nil {
		return unavailable()
	}
	var pending []Run
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			rows.Close()
			return err
		}
		pending = append(pending, r)
	}
	rowErr := rows.Err()
	closeErr := rows.Close()
	if rowErr != nil || closeErr != nil {
		return unavailable()
	}
	for _, r := range pending {
		if _, err = s.moveTx(ctx, tx, Transition{RunID: r.ID, From: r.State, To: "interrupted", Version: r.Version}); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "UPDATE artifacts SET state='interrupted' WHERE state='writing'"); err != nil {
		return unavailable()
	}
	var delivering int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM runs WHERE state='delivering'").Scan(&delivering); err != nil {
		return unavailable()
	}
	if err = tx.Commit(); err != nil {
		return unavailable()
	}
	if delivering != 0 {
		return unavailable()
	}
	return nil
}
