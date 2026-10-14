package observability_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/platform/observability"
	"github.com/ruhuang/ink/server/internal/scheduler"
)

type scanProcessor struct {
	started chan struct{}
	release chan struct{}
	err     error
}

func (p *scanProcessor) ProcessDue(context.Context, int) (int, error) {
	close(p.started)
	<-p.release
	return 0, p.err
}

func (p *scanProcessor) SyncDue(ctx context.Context, limit int) (int, error) {
	return p.ProcessDue(ctx, limit)
}

func (p *scanProcessor) PurgeOlderThan(ctx context.Context, _ time.Time) (int64, error) {
	_, err := p.ProcessDue(ctx, 0)
	return 0, err
}

func scrape(t *testing.T, m *observability.Metrics) string {
	t.Helper()
	w := httptest.NewRecorder()
	m.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	if w.Code != 200 {
		t.Fatalf("scrape status %d: %s", w.Code, w.Body)
	}
	return w.Body.String()
}

func sample(t *testing.T, body, name string) float64 {
	t.Helper()
	for line := range strings.SplitSeq(body, "\n") {
		if value, ok := strings.CutPrefix(line, name+" "); ok {
			n, err := strconv.ParseFloat(value, 64)
			if err != nil {
				t.Fatal(err)
			}
			return n
		}
	}
	t.Fatalf("missing sample %s", name)
	return 0
}

func TestWorkerScanMetricsThroughRunners(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, name := range []string{"schedule", "plugin_fetch", "print_status_sync", "inbox_janitor"} {
		for _, outcome := range []struct {
			name, result string
			err          error
		}{
			{"idle", "success", nil},
			{"failure", "error", errors.New("secret provider detail")},
			{"shutdown", "canceled", fmt.Errorf("wrapped: %w", errors.Join(context.Canceled, context.Canceled))},
			{"deadline", "error", context.DeadlineExceeded},
			{"partial failure during shutdown", "error", errors.Join(errors.New("secret provider detail"), context.Canceled)},
		} {
			t.Run(name+"/"+outcome.name, func(t *testing.T) {
				m := observability.NewMetrics()
				observer := m.Worker(name, 6*time.Hour)
				labels := `{worker="` + name + `"}`
				initial := scrape(t, m)
				if sample(t, initial, "ink_worker_last_successful_scan_timestamp_seconds"+labels) != 0 || sample(t, initial, "ink_worker_poll_interval_seconds"+labels) != 21600 {
					t.Fatal("incorrect initial state or interval")
				}
				p := &scanProcessor{started: make(chan struct{}), release: make(chan struct{}), err: outcome.err}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				defer close(p.release)
				var done <-chan struct{}
				switch name {
				case "schedule":
					r := scheduler.NewRunner(p, logger, 6*time.Hour, 10)
					r.Observer = observer
					done = r.Start(ctx)
				case "plugin_fetch":
					r := scheduler.NewFetchRunner(p, logger, 6*time.Hour, 10)
					r.Observer = observer
					done = r.Start(ctx)
				case "print_status_sync":
					r := scheduler.NewPrintStatusRunner(p, logger, 6*time.Hour, 10)
					r.Observer = observer
					done = r.Start(ctx)
				case "inbox_janitor":
					r := scheduler.NewInboxJanitor(p, nil, logger, 6*time.Hour, time.Hour)
					r.Observer = observer
					done = r.Start(ctx)
				}
				select {
				case <-p.started:
				case <-time.After(time.Second):
					t.Fatal("scan did not start")
				}
				body := scrape(t, m)
				if sample(t, body, "ink_worker_scan_in_progress"+labels) != 1 || sample(t, body, "ink_worker_last_scan_completed_timestamp_seconds"+labels) != 0 || sample(t, body, "ink_worker_last_scan_started_timestamp_seconds"+labels) <= 0 {
					t.Fatal("blocked scan must be visible without reporting completion")
				}
				cancel()
				p.release <- struct{}{}
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Fatal("worker did not stop")
				}
				body = scrape(t, m)
				if strings.Contains(body, "secret provider detail") {
					t.Fatal("error detail leaked into metrics")
				}
				if sample(t, body, "ink_worker_scan_in_progress"+labels) != 0 || sample(t, body, "ink_worker_scan_duration_seconds_count"+labels) != 1 || sample(t, body, "ink_worker_last_scan_completed_timestamp_seconds"+labels) <= 0 {
					t.Fatal("completion metrics incorrect")
				}
				if (sample(t, body, "ink_worker_last_successful_scan_timestamp_seconds"+labels) > 0) != (outcome.err == nil) {
					t.Fatal("only successful scans should update success timestamp")
				}
				for _, result := range []string{"success", "error", "canceled"} {
					want := float64(0)
					if result == outcome.result {
						want = 1
					}
					if got := sample(t, body, `ink_worker_scans_total{result="`+result+`",worker="`+name+`"}`); got != want {
						t.Fatalf("result %s: got %v, want %v", result, got, want)
					}
				}
			})
		}
	}
}
