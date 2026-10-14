package observability

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReadinessDependencies(t *testing.T) {
	for _, test := range []struct {
		name                                        string
		stopped, missing, shutdown, databaseFailure bool
	}{
		{name: "healthy"},
		{name: "worker stopped", stopped: true},
		{name: "worker not started", missing: true},
		{name: "shutdown", shutdown: true},
		{name: "database unavailable", databaseFailure: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan struct{})
			if test.stopped {
				close(done)
			}
			if test.missing {
				done = nil
			}
			if test.shutdown {
				cancel()
			}
			probe := NewReadiness(root, func(ctx context.Context) error {
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > time.Second {
					t.Fatal("probe must have a bounded deadline")
				}
				if test.databaseFailure {
					return errors.New("secret database details")
				}
				return nil
			}, map[string]<-chan struct{}{"worker": done})
			wantErr := test.stopped || test.missing || test.shutdown || test.databaseFailure
			if err := probe.Check(t.Context()); (err != nil) != wantErr {
				t.Fatalf("Check() = %v, want failure %v", err, wantErr)
			}
		})
	}
}

func TestReadinessCopiesWorkerSetAndAllowsDisabledWorkers(t *testing.T) {
	workers := map[string]<-chan struct{}{}
	probe := NewReadiness(t.Context(), func(context.Context) error { return nil }, workers)
	workers["disabled"] = nil
	if err := probe.Check(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestReadinessDatabaseTimeout(t *testing.T) {
	probe := NewReadiness(t.Context(), func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}, nil)
	started := time.Now()
	if err := probe.Check(t.Context()); err == nil {
		t.Fatal("expected timeout")
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("probe took %s", elapsed)
	}
}
