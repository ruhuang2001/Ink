package scheduler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type failedStatusProcessor struct{ err error }

func (p failedStatusProcessor) SyncDue(context.Context, int) (int, error) { return 0, p.err }

func TestShutdownPreservesEarlierStatusFailures(t *testing.T) {
	for _, test := range []struct {
		name   string
		err    error
		logged bool
	}{
		{"cancellation", fmt.Errorf("shutdown: %w", context.Canceled), false},
		{"joined cancellations", errors.Join(context.Canceled, context.Canceled), false},
		{"provider failure before cancellation", errors.Join(errors.New("provider unavailable"), context.Canceled), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			runner := NewPrintStatusRunner(failedStatusProcessor{test.err}, slog.New(slog.NewTextHandler(&output, nil)), time.Second, 20)
			runner.runOnce(t.Context())
			if strings.Contains(output.String(), "print status sync failed") != test.logged {
				t.Fatalf("lost failure or logged normal cancellation: %s", output.String())
			}
		})
	}
}
