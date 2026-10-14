package observability

import (
	"context"
	"errors"
	"maps"
	"time"
)

// Readiness checks dependencies required to accept API traffic. Workers contains
// only enabled workers whose Start calls have returned. A done channel closing
// means that worker exited; an open channel does not prove forward progress.
type Readiness struct {
	root    context.Context
	ping    func(context.Context) error
	workers map[string]<-chan struct{}
}

func NewReadiness(root context.Context, ping func(context.Context) error, workers map[string]<-chan struct{}) *Readiness {
	return &Readiness{root: root, ping: ping, workers: maps.Clone(workers)}
}

func (r *Readiness) Check(ctx context.Context) error {
	if r.root.Err() != nil {
		return errors.New("service is shutting down")
	}
	for name, done := range r.workers {
		if done == nil {
			return errors.New("worker not started: " + name)
		}
		select {
		case <-done:
			return errors.New("worker stopped: " + name)
		default:
		}
	}
	probeCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := r.ping(probeCtx); err != nil {
		return errors.New("database unavailable")
	}
	return r.root.Err()
}
