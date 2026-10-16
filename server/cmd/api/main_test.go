package main

import (
	"context"
	"testing"

	"github.com/ruhuang/ink/server/internal/platform/observability"
)

func TestDisabledWorkersDoNotStartOrBlockReadiness(t *testing.T) {
	workers := make(map[string]<-chan struct{})
	for _, name := range []string{"print_status_sync"} {
		startWorker(false, name, workers, func() <-chan struct{} {
			t.Fatal("disabled worker startup must not be invoked")
			return nil
		})
	}
	pinged := false
	readiness := observability.NewReadiness(t.Context(), func(context.Context) error {
		pinged = true
		return nil
	}, workers)
	if err := readiness.Check(t.Context()); err != nil || !pinged || len(workers) != 0 {
		t.Fatalf("disabled-worker readiness: error=%v, database checked=%v, worker count=%d", err, pinged, len(workers))
	}
}

func TestEnabledWorkerLifecycleIsChecked(t *testing.T) {
	workers := make(map[string]<-chan struct{})
	done := make(chan struct{})
	startWorker(true, "print_status_sync", workers, func() <-chan struct{} { return done })
	readiness := observability.NewReadiness(t.Context(), func(context.Context) error { return nil }, workers)
	if err := readiness.Check(t.Context()); err != nil {
		t.Fatal(err)
	}
	close(done)
	if err := readiness.Check(t.Context()); err == nil {
		t.Fatal("enabled worker exit must make readiness fail")
	}
}
