package app

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"bkplight.local/bkplight/internal/control"
	"bkplight.local/bkplight/internal/observability"
	"bkplight.local/bkplight/internal/state"
)

type backend struct {
	store          *state.Store
	logs           *observability.Logger
	ready          atomic.Bool
	mu             sync.Mutex
	lastProbe      time.Time
	stateOK        bool
	logProbeFailed bool
}

func (b *backend) Jobs(ctx context.Context) ([]state.JobRevision, error) { return b.store.Jobs(ctx) }
func (b *backend) GetRun(ctx context.Context, id string) (state.Run, error) {
	return b.store.GetRun(ctx, id)
}
func (b *backend) ListRuns(ctx context.Context, limit int, after string) (state.Page, error) {
	return b.store.ListRuns(ctx, limit, after)
}
func (b *backend) Cancel(ctx context.Context, id, requestID string) (state.Run, error) {
	if !b.Health(ctx).Ready {
		return state.Run{}, unavailable()
	}
	return b.store.Cancel(ctx, id, requestID)
}
func (b *backend) Health(ctx context.Context) control.Health {
	h := control.Health{Live: true, Scope: "foundation", State: "unavailable", Logs: "unavailable", Scheduler: "not_implemented", BackupEngine: "not_implemented"}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ready.Load() && time.Since(b.lastProbe) >= time.Second {
		b.stateOK = b.store.ProbeWrite(ctx) == nil
		b.logProbeFailed = b.logs.Write(observability.Record{Time: time.Now(), Level: "debug", MessageCode: "service.health_checked"}) != nil
		b.lastProbe = time.Now()
	}
	if b.stateOK {
		h.State = "ready"
	}
	if b.logs.Healthy() && !b.logProbeFailed {
		h.Logs = "ready"
	}
	h.Ready = b.ready.Load() && b.stateOK && h.Logs == "ready"
	return h
}

var _ control.Backend = (*backend)(nil)
