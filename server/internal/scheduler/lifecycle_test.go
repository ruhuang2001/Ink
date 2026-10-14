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

func (p *blockingProcessor) SyncDue(context.Context, int) (int, error) {
	close(p.started)
	<-p.release
	return 0, nil
}

func TestPrintStatusRunnerStartsImmediatelyAndWaitsForShutdown(t *testing.T) {
	processor := &blockingProcessor{started: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(t.Context())
	done := NewPrintStatusRunner(processor, nil, time.Hour, 20).Start(ctx)
	select {
	case <-processor.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start immediately")
	}
	cancel()
	assertNotClosed(t, done)
	close(processor.release)
	assertClosed(t, done)
	assertClosed(t, NewPrintStatusRunner(nil, nil, time.Second, 20).Start(t.Context()))
}
