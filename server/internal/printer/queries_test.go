package printer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/workspace"
)

func TestPrintJobReadsAreBoundedAndNeverContactProvider(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = fmt.Fprint(w, `{"showapi_res_code":1,"printflag":1}`)
	}))
	defer provider.Close()
	now := time.Date(2026, 10, 1, 5, 0, 0, 123456000, time.UTC)
	repo := newFakePrinterRepo()
	for index := range 35 {
		id := fmt.Sprintf("job-%02d", index)
		repo.jobs[id] = Job{ID: id, UserID: "user-1", PrinterBindingID: "device-1", Title: "title", Content: strings.Repeat("secret-body", 1000), Status: workspace.PrintStatusQueued, ProviderPrintContentID: new(index + 1), CreatedAt: now, UpdatedAt: now}
	}
	repo.jobs["foreign"] = Job{ID: "foreign", UserID: "user-2", Content: "other account", Status: workspace.PrintStatusPending, CreatedAt: now.Add(time.Hour)}
	service := NewService(repo, fakeAuthenticator{}, fakeIDGenerator{}, fakeClock{now: now}, "key", provider.URL, time.Second)

	first, err := service.ListPrintJobs(t.Context(), "token", ListJobsInput{Status: "active", Limit: 20})
	if err != nil || len(first.PrintJobs) != 20 || first.NextCursor == nil {
		t.Fatalf("first page: %+v, %v", first, err)
	}
	// Changes to mutable status timestamps cannot move the keyset cursor.
	job := repo.jobs["job-01"]
	job.UpdatedAt = now.Add(time.Hour)
	repo.jobs[job.ID] = job
	second, err := service.ListPrintJobs(t.Context(), "token", ListJobsInput{Status: "active", Limit: 20, Cursor: *first.NextCursor})
	if err != nil || len(second.PrintJobs) != 15 || second.NextCursor != nil {
		t.Fatalf("second page: %+v, %v", second, err)
	}
	seen := map[string]bool{}
	for _, summary := range append(first.PrintJobs, second.PrintJobs...) {
		if seen[summary.ID] {
			t.Fatalf("duplicate row %s", summary.ID)
		}
		seen[summary.ID] = true
	}
	encoded, err := json.Marshal(first)
	if err != nil || strings.Contains(string(encoded), "content") || len(encoded) > 6000 {
		t.Fatalf("expected small summary without content, size=%d err=%v", len(encoded), err)
	}
	statuses, err := service.GetPrintJobStatuses(t.Context(), "token", JobStatusesInput{IDs: []string{"job-00", "foreign", "job-00"}})
	if err != nil || len(statuses.PrintJobs) != 1 || statuses.Counts.Queued != 35 || statuses.LatestJobID == nil || *statuses.LatestJobID != "job-34" {
		t.Fatalf("status/count account isolation: %+v, %v", statuses, err)
	}
	if statuses.PrintJobs[0].UpdatedAt != now.Format(time.RFC3339Nano) {
		t.Fatal("status lost subsecond precision")
	}
	detail, err := service.GetPrintJob(t.Context(), "token", "job-00")
	if err != nil || detail.Content != repo.jobs[detail.ID].Content || detail.UpdatedAt != statuses.PrintJobs[0].UpdatedAt {
		t.Fatalf("detail: %+v, %v", detail, err)
	}
	if _, err := service.GetPrintJob(t.Context(), "token", "foreign"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign detail: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("reads contacted provider %d times", calls.Load())
	}
	if repo.jobs["job-00"].Status != workspace.PrintStatusQueued {
		t.Fatal("read mutated persisted status")
	}
}

func TestPrintJobCursorAndStatusInputValidation(t *testing.T) {
	now := time.Now().UTC()
	repo := newFakePrinterRepo()
	for index := range 3 {
		id := fmt.Sprintf("job-%d", index)
		repo.jobs[id] = Job{ID: id, UserID: "user-1", Status: workspace.PrintStatusPending, CreatedAt: now, UpdatedAt: now}
	}
	service := NewService(repo, fakeAuthenticator{}, nil, fakeClock{now: now}, "", "", time.Second)
	page, err := service.ListPrintJobs(t.Context(), "token", ListJobsInput{Status: "active", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []ListJobsInput{
		{Limit: -1}, {Limit: 101}, {Status: "bad"}, {Cursor: "not-json"},
		{Status: "history", Cursor: *page.NextCursor}, {Cursor: strings.Repeat("a", 1025)},
	} {
		if _, err := service.ListPrintJobs(t.Context(), "token", input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("input %+v: %v", input, err)
		}
	}
	for _, input := range []JobStatusesInput{{IDs: []string{""}}, {IDs: make([]string, 101)}} {
		if _, err := service.GetPrintJobStatuses(t.Context(), "token", input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("status input: %v", err)
		}
	}
}

func TestPrintJobCountsRespectLocalDayAndDetectNewJobs(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	repo := newFakePrinterRepo()
	repo.jobs["completed"] = Job{ID: "completed", UserID: "user-1", Status: workspace.PrintStatusCompleted, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	service := NewService(repo, fakeAuthenticator{}, nil, fakeClock{now: now}, "", "", time.Second)
	result, err := service.GetPrintJobStatuses(t.Context(), "token", JobStatusesInput{Since: now.Add(-8 * time.Hour)})
	if err != nil || result.Counts.TodayCompleted != 1 {
		t.Fatalf("local day: %+v, %v", result, err)
	}
	repo.jobs["scheduled-new"] = Job{ID: "scheduled-new", UserID: "user-1", Status: workspace.PrintStatusQueued, CreatedAt: now, UpdatedAt: now}
	result, err = service.GetPrintJobStatuses(t.Context(), "token", JobStatusesInput{})
	if err != nil || len(result.PrintJobs) != 0 || result.Counts.Queued != 1 || result.Counts.TodayCompleted != 0 || *result.LatestJobID != "scheduled-new" {
		t.Fatalf("new background job detection: %+v, %v", result, err)
	}
}

func TestStatusChunksCanSkipAccountMetadata(t *testing.T) {
	now := time.Now()
	repo := statusFixture(now, 2)
	service := NewService(repo, fakeAuthenticator{}, nil, fakeClock{now: now}, "", "", time.Second)
	first, err := service.GetPrintJobStatuses(t.Context(), "token", JobStatusesInput{IDs: []string{"job-00"}})
	if err != nil || first.Counts == nil || first.Counts.Queued != 2 {
		t.Fatalf("first metadata: %+v, %v", first, err)
	}
	second, err := service.GetPrintJobStatuses(t.Context(), "token", JobStatusesInput{IDs: []string{"job-01"}, SkipMetadata: true})
	if err != nil || len(second.PrintJobs) != 1 || second.Counts != nil || second.LatestJobID != nil || repo.countCalls != 1 {
		t.Fatalf("extra chunk repeated metadata: %+v, calls=%d, %v", second, repo.countCalls, err)
	}
}
