package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Clock abstracts the time source so tests do not need a real ticker.
type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

// InboxPurger deletes collected inbox items older than a cutoff.
type InboxPurger interface {
	PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// InboxJanitor purges collected items older than Retention on a fixed cadence.
type InboxJanitor struct {
	purger    InboxPurger
	clock     Clock
	logger    *slog.Logger
	interval  time.Duration
	retention time.Duration
}

func NewInboxJanitor(purger InboxPurger, clock Clock, logger *slog.Logger, interval time.Duration, retention time.Duration) *InboxJanitor {
	if logger == nil {
		logger = slog.Default()
	}
	if clock == nil {
		clock = systemClock{}
	}
	return &InboxJanitor{
		purger:    purger,
		clock:     clock,
		logger:    logger,
		interval:  interval,
		retention: retention,
	}
}

func (j *InboxJanitor) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	if j.purger == nil || j.interval <= 0 || j.retention <= 0 {
		close(done)
		return done
	}

	ticker := time.NewTicker(j.interval)
	go func() {
		defer close(done)
		defer ticker.Stop()

		j.runOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				j.runOnce(ctx)
			}
		}
	}()
	return done
}

func (j *InboxJanitor) runOnce(ctx context.Context) {
	cutoff := j.clock.Now().Add(-j.retention)
	removed, err := j.purger.PurgeOlderThan(ctx, cutoff)
	if err != nil {
		j.logger.Error("inbox janitor failed", "error", err)
		return
	}
	if removed > 0 {
		j.logger.Info("purged expired inbox items", "count", removed, "cutoff", cutoff)
	}
}
