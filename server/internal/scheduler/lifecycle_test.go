package scheduler

import (
	"context"
	"testing"
	"time"
)

type blockingProcessor struct {
	started chan struct{}
	release chan struct{}
}

func (p *blockingProcessor) ProcessDue(context.Context, int) (int, error) {
	close(p.started)
	<-p.release
	return 0, nil
}

type blockingPurger struct {
	started chan struct{}
	release chan struct{}
}

func (p *blockingPurger) PurgeOlderThan(context.Context, time.Time) (int64, error) {
	close(p.started)
	<-p.release
	return 0, nil
}

func TestRunnerDoneWaitsForInFlightProcessDue(t *testing.T) {
	tests := []struct {
		name  string
		start func(context.Context, *blockingProcessor) <-chan struct{}
	}{
		{name: "schedule", start: func(ctx context.Context, processor *blockingProcessor) <-chan struct{} {
			return NewRunner(processor, nil, time.Hour, 1).Start(ctx)
		}},
		{name: "fetch", start: func(ctx context.Context, processor *blockingProcessor) <-chan struct{} {
			return NewFetchRunner(processor, nil, time.Hour, 1).Start(ctx)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			processor := &blockingProcessor{started: make(chan struct{}), release: make(chan struct{})}
			ctx, cancel := context.WithCancel(context.Background())
			done := test.start(ctx, processor)
			<-processor.started
			cancel()

			assertNotClosed(t, done)
			close(processor.release)
			assertClosed(t, done)
		})
	}
}

func TestInboxJanitorDoneWaitsForInFlightPurge(t *testing.T) {
	purger := &blockingPurger{started: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	done := NewInboxJanitor(purger, nil, nil, time.Hour, time.Hour).Start(ctx)
	<-purger.started
	cancel()

	assertNotClosed(t, done)
	close(purger.release)
	assertClosed(t, done)
}

func TestDisabledRunnerReturnsClosedDone(t *testing.T) {
	done := NewRunner(nil, nil, time.Hour, 1).Start(context.Background())
	assertClosed(t, done)
}

func assertNotClosed(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
		t.Fatal("done closed before in-flight work returned")
	default:
	}
}

func assertClosed(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for done to close")
	}
}
