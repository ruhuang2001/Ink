package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

type FetchProcessor interface {
	ProcessDue(ctx context.Context, limit int) (int, error)
}

type FetchRunner struct {
	processor FetchProcessor
	logger    *slog.Logger
	interval  time.Duration
	limit     int
}

func NewFetchRunner(processor FetchProcessor, logger *slog.Logger, interval time.Duration, limit int) *FetchRunner {
	if logger == nil {
		logger = slog.Default()
	}
	return &FetchRunner{
		processor: processor,
		logger:    logger,
		interval:  interval,
		limit:     limit,
	}
}

func (r *FetchRunner) Start(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	if r.processor == nil || r.interval <= 0 {
		close(done)
		return done
	}

	ticker := time.NewTicker(r.interval)
	go func() {
		defer close(done)
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

func (r *FetchRunner) runOnce(ctx context.Context) {
	processed, err := r.processor.ProcessDue(ctx, r.limit)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		r.logger.Error("plugin fetch processor failed", "error", err)
		return
	}
	if processed > 0 {
		r.logger.Info("processed due plugin fetches", "count", processed)
	}
}
