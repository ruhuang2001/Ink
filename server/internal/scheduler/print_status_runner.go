package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type PrintStatusProcessor interface {
	SyncDue(ctx context.Context, limit int) (int, error)
}

type PrintStatusRunner struct {
	processor PrintStatusProcessor
	logger    *slog.Logger
	interval  time.Duration
	limit     int
}

func NewPrintStatusRunner(processor PrintStatusProcessor, logger *slog.Logger, interval time.Duration, limit int) *PrintStatusRunner {
	if logger == nil {
		logger = slog.Default()
	}
	return &PrintStatusRunner{processor: processor, logger: logger, interval: interval, limit: limit}
}

func (r *PrintStatusRunner) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	if r.processor == nil || r.interval <= 0 {
		close(done)
		return done
	}
	go func() {
		defer close(done)
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		r.runOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.runOnce(ctx)
			}
		}
	}()
	return done
}

func (r *PrintStatusRunner) runOnce(ctx context.Context) {
	started := time.Now()
	processed, err := r.processor.SyncDue(ctx, r.limit)
	if errors.Is(err, context.Canceled) {
		return
	}
	if err != nil {
		failed := 0
		if counted, ok := errors.AsType[interface {
			error
			FailureCount() int
		}](err); ok {
			failed = counted.FailureCount()
		}
		r.logger.Error("print status sync failed", "processed", processed, "failed", failed, "duration", time.Since(started), "error", err)
		return
	}
	if processed > 0 {
		r.logger.Info("synchronized print statuses", "processed", processed, "duration", time.Since(started))
	}
}
