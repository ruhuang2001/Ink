package pluginfetch

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/auth"
	"github.com/ruhuang/ink/server/internal/inbox"
	"github.com/ruhuang/ink/server/internal/platform/observability"
	"github.com/ruhuang/ink/server/internal/plugins"
)

type fakeAuthenticator struct{}

func (fakeAuthenticator) GetCurrentUser(_ context.Context, accessToken string) (auth.UserDTO, error) {
	return auth.UserDTO{
		ID:    "user-1",
		Email: accessToken,
		Name:  "Ink User",
		Role:  "member",
	}, nil
}

type executeCall struct {
	installation plugins.Installation
	binding      plugins.Binding
	trigger      plugins.FetchTrigger
}

type successCall struct {
	bindingID   string
	cursor      *string
	fetchedAt   time.Time
	nextFetchAt time.Time
}

type failureCall struct {
	bindingID   string
	message     string
	attemptedAt time.Time
	nextFetchAt time.Time
}

type fakeRuntime struct {
	installation plugins.Installation
	manifest     plugins.Manifest
	binding      plugins.Binding
	claimed      []plugins.Binding
	output       plugins.FetchOutput
	executeErr   error

	executeCalls []executeCall
	successCalls []successCall
	failureCalls []failureCall
}

func (r *fakeRuntime) GetInstallation(_ context.Context, installationID string) (plugins.Installation, plugins.Manifest, error) {
	return r.installation, r.manifest, nil
}

func (r *fakeRuntime) GetBindingForUser(_ context.Context, installationID string, userID string) (plugins.Binding, map[string]string, error) {
	return r.binding, map[string]string{"apiToken": "secret"}, nil
}

func (r *fakeRuntime) GetBindingByID(_ context.Context, bindingID string) (plugins.Binding, map[string]string, error) {
	return r.binding, map[string]string{"apiToken": "secret"}, nil
}

func (r *fakeRuntime) ExecuteFetch(_ context.Context, installation plugins.Installation, binding plugins.Binding, secrets map[string]string, trigger plugins.FetchTrigger) (plugins.FetchOutput, error) {
	if secrets["apiToken"] != "secret" {
		return plugins.FetchOutput{}, errors.New("missing secret")
	}
	r.executeCalls = append(r.executeCalls, executeCall{
		installation: installation,
		binding:      binding,
		trigger:      trigger,
	})
	if r.executeErr != nil {
		return plugins.FetchOutput{}, r.executeErr
	}
	return r.output, nil
}

func (r *fakeRuntime) ClaimDueBindings(_ context.Context, now time.Time, leaseUntil time.Time, limit int) ([]plugins.Binding, error) {
	if limit > 0 && len(r.claimed) > limit {
		return append([]plugins.Binding{}, r.claimed[:limit]...), nil
	}
	return append([]plugins.Binding{}, r.claimed...), nil
}

func (r *fakeRuntime) RecordFetchSuccess(_ context.Context, bindingID string, cursor *string, fetchedAt time.Time, nextFetchAt time.Time) error {
	r.successCalls = append(r.successCalls, successCall{
		bindingID:   bindingID,
		cursor:      cursor,
		fetchedAt:   fetchedAt,
		nextFetchAt: nextFetchAt,
	})
	return nil
}

func (r *fakeRuntime) RecordFetchFailure(_ context.Context, bindingID string, message string, attemptedAt time.Time, nextFetchAt time.Time) error {
	r.failureCalls = append(r.failureCalls, failureCall{
		bindingID:   bindingID,
		message:     message,
		attemptedAt: attemptedAt,
		nextFetchAt: nextFetchAt,
	})
	return nil
}

type capturedInbox struct {
	inputs []inbox.IngestInput
}

func (c *capturedInbox) Ingest(_ context.Context, input inbox.IngestInput) (inbox.IngestResult, error) {
	c.inputs = append(c.inputs, input)
	itemIDs := make([]string, len(input.Items))
	for index := range input.Items {
		itemIDs[index] = input.Items[index].ExternalID
	}
	return inbox.IngestResult{
		Inserted: len(input.Items),
		ItemIDs:  itemIDs,
	}, nil
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

func buildRuntime(now time.Time) *fakeRuntime {
	cursor := "cursor-1"
	return &fakeRuntime{
		installation: plugins.Installation{
			ID:          "install-1",
			DisplayName: "Fixture Source",
			Status:      plugins.InstallationStatusReady,
		},
		manifest: plugins.Manifest{
			SchemaVersion: 2,
			Kind:          "source",
			PluginKey:     "fixture-source",
			Name:          "Fixture Source",
			Version:       "1.0.0",
			Runtime:       plugins.RuntimeSpec{Type: "node"},
			FetchPolicy: plugins.FetchPolicy{
				Type:    plugins.FetchPolicyTypeFixedInterval,
				Minutes: 15,
			},
		},
		binding: plugins.Binding{
			ID:                   "binding-1",
			PluginInstallationID: "install-1",
			UserID:               "user-1",
			Enabled:              true,
			Status:               plugins.BindingStatusConnected,
		},
		claimed: []plugins.Binding{
			{
				ID:                   "binding-1",
				PluginInstallationID: "install-1",
				UserID:               "user-1",
				Enabled:              true,
				Status:               plugins.BindingStatusConnected,
				NextFetchAt:          timePtr(now.Add(-5 * time.Minute)),
			},
		},
		output: plugins.FetchOutput{
			Items: []plugins.Item{
				{
					ExternalID:  "item-1",
					Title:       "Fetched Item",
					SourceLabel: "Fixture Source",
					Blocks:      []plugins.ContentBlock{{Type: plugins.BlockParagraph, Text: "body"}},
				},
			},
			Cursor: &cursor,
		},
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func newFetchService(now time.Time, runtime *fakeRuntime, inboxCapture *capturedInbox) *Service {
	return NewService(fakeAuthenticator{}, runtime, inboxCapture, fixedClock{now: now})
}

func assertRecordedCounts(t *testing.T, runtime *fakeRuntime, successCount int, failureCount int) {
	t.Helper()
	if len(runtime.successCalls) != successCount {
		t.Fatalf("expected %d success record(s), got %+v", successCount, runtime.successCalls)
	}
	if len(runtime.failureCalls) != failureCount {
		t.Fatalf("expected %d failure record(s), got %+v", failureCount, runtime.failureCalls)
	}
}

func TestRunManualFetchesAndIngestsWithoutPrinting(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	runtime := buildRuntime(now)
	inboxCapture := &capturedInbox{}
	service := newFetchService(now, runtime, inboxCapture)

	result, err := service.RunManual(context.Background(), "member-token", "install-1")
	if err != nil {
		t.Fatalf("run manual: %v", err)
	}

	if result.FetchedCount != 1 || result.IngestedCount != 1 {
		t.Fatalf("unexpected result counts: %+v", result)
	}
	if !result.CursorAdvanced {
		t.Fatalf("expected cursor to advance")
	}
	if len(inboxCapture.inputs) != 1 {
		t.Fatalf("expected one ingest call, got %d", len(inboxCapture.inputs))
	}
	if len(runtime.executeCalls) != 1 || runtime.executeCalls[0].trigger.Kind != plugins.TriggerKindManual {
		t.Fatalf("expected one manual execute call, got %+v", runtime.executeCalls)
	}
	assertRecordedCounts(t, runtime, 1, 0)
}

func TestProcessDueFetchesClaimedBindingsAndSchedulesNextRun(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	runtime := buildRuntime(now)
	inboxCapture := &capturedInbox{}
	service := newFetchService(now, runtime, inboxCapture)

	processed, err := service.ProcessDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("process due: %v", err)
	}

	if processed != 1 {
		t.Fatalf("expected 1 processed binding, got %d", processed)
	}
	if len(runtime.executeCalls) != 1 {
		t.Fatalf("expected one execute call, got %+v", runtime.executeCalls)
	}
	call := runtime.executeCalls[0]
	if call.trigger.Kind != plugins.TriggerKindAutomatic {
		t.Fatalf("expected automatic trigger, got %+v", call.trigger)
	}
	if call.trigger.ScheduledFor != runtime.claimed[0].NextFetchAt.UTC().Format(time.RFC3339) {
		t.Fatalf("unexpected scheduledFor value: %+v", call.trigger)
	}
	if len(inboxCapture.inputs) != 1 {
		t.Fatalf("expected one ingest call, got %d", len(inboxCapture.inputs))
	}
	assertRecordedCounts(t, runtime, 1, 0)
	expectedNextFetch := now.Add(15 * time.Minute)
	if !runtime.successCalls[0].nextFetchAt.Equal(expectedNextFetch) {
		t.Fatalf("expected next fetch at %s, got %s", expectedNextFetch, runtime.successCalls[0].nextFetchAt)
	}
}

func TestProcessDueRecordsFailureWhenFetchFails(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	runtime := buildRuntime(now)
	runtime.executeErr = errors.New("upstream unavailable")
	inboxCapture := &capturedInbox{}
	service := NewService(fakeAuthenticator{}, runtime, inboxCapture, fixedClock{now: now})
	metrics := observability.NewMetrics()
	service.Observer = metrics.Operation("plugin_fetch")

	processed, err := service.ProcessDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("process due: %v", err)
	}

	if processed != 1 {
		t.Fatalf("expected 1 processed binding, got %d", processed)
	}
	if len(inboxCapture.inputs) != 0 {
		t.Fatalf("expected no ingest on fetch failure, got %+v", inboxCapture.inputs)
	}
	if len(runtime.successCalls) != 0 {
		t.Fatalf("did not expect success records, got %+v", runtime.successCalls)
	}
	if len(runtime.failureCalls) != 1 {
		t.Fatalf("expected one failure record, got %+v", runtime.failureCalls)
	}
	w := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(w.Body.String(), `ink_operations_total{operation="plugin_fetch",result="error"} 1`) ||
		!strings.Contains(w.Body.String(), `ink_operations_total{operation="plugin_fetch",result="success"} 0`) {
		t.Fatalf("successful batch hid individual fetch failure: %s", w.Body.String())
	}
}
