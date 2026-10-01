package printer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/workspace"
	memobirdapi "github.com/ruhuang2001/memobird-go/memobird"
)

func statusFixture(now time.Time, count int) *fakePrinterRepo {
	repo := newFakePrinterRepo()
	repo.bindings["device-1"] = Binding{ID: "device-1", UserID: "user-1", DeviceIdentifier: "test-device", ProviderUserID: 1, Status: workspace.DeviceStatusConnected}
	for index := range count {
		id := fmt.Sprintf("job-%02d", index)
		repo.jobs[id] = Job{ID: id, UserID: "user-1", PrinterBindingID: "device-1", Status: workspace.PrintStatusQueued, ProviderPrintContentID: new(index + 1), CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute), NextStatusCheckAt: new(now)}
	}
	return repo
}

func TestStatusSyncIsBoundedFairAndWorksWithoutListRequests(t *testing.T) {
	var calls atomic.Int32
	var printed atomic.Bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		flag := 0
		if printed.Load() {
			flag = 1
		}
		_, _ = fmt.Fprintf(w, `{"showapi_res_code":1,"printflag":%d}`, flag)
	}))
	defer provider.Close()
	now := time.Now().UTC()
	repo := statusFixture(now, 5)
	service := NewService(repo, nil, nil, fakeClock{now: now}, "key", provider.URL, time.Second)
	syncer := NewStatusSynchronizer(service, time.Second)
	for _, expected := range []int{2, 2, 1, 0} {
		processed, err := syncer.SyncDue(t.Context(), 2)
		if err != nil || processed != expected {
			t.Fatalf("sync got %d, %v; want %d", processed, err, expected)
		}
	}
	if calls.Load() != 5 {
		t.Fatalf("old queued jobs starved newer ones or batch exceeded limit: calls=%d", calls.Load())
	}
	for _, job := range repo.jobs {
		if job.Status != workspace.PrintStatusQueued || !job.NextStatusCheckAt.Equal(now.Add(statusCheckInterval)) || !job.UpdatedAt.Equal(now.Add(-time.Minute)) {
			t.Fatalf("unfinished job refresh changed user-visible timestamp: %+v", job)
		}
	}
	service.clock = fakeClock{now: now.Add(statusCheckInterval)}
	printed.Store(true)
	processed, err := syncer.SyncDue(t.Context(), 5)
	if err != nil || processed != 5 || calls.Load() != 10 {
		t.Fatalf("complete in background: %d, %v", processed, err)
	}
	for _, job := range repo.jobs {
		if job.Status != workspace.PrintStatusCompleted || job.NextStatusCheckAt != nil || !job.UpdatedAt.Equal(now.Add(statusCheckInterval)) {
			t.Fatalf("not completed: %+v", job)
		}
	}
}

func TestStatusSyncFailureBackoffAndProviderDeadline(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer provider.Close()
	defer close(release)
	now := time.Now().UTC()
	repo := statusFixture(now, 1)
	service := NewService(repo, nil, nil, fakeClock{now: now}, "key", provider.URL, 30*time.Second)
	syncer := NewStatusSynchronizer(service, 20*time.Millisecond)
	started := time.Now()
	processed, err := syncer.SyncDue(t.Context(), 20)
	if processed != 1 || !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > time.Second {
		t.Fatalf("provider deadline: processed=%d duration=%v err=%v", processed, time.Since(started), err)
	}
	job := repo.jobs["job-00"]
	if job.Status != workspace.PrintStatusQueued || job.StatusCheckAttempts != 1 || !job.NextStatusCheckAt.Equal(now.Add(20*time.Second)) {
		t.Fatalf("failure was not deferred: %+v", job)
	}
	processed, err = syncer.SyncDue(t.Context(), 20)
	if err != nil || processed != 0 || calls.Load() != 1 {
		t.Fatalf("rechecked during backoff: %d calls=%d err=%v", processed, calls.Load(), err)
	}
	service.clock = fakeClock{now: now.Add(20 * time.Second)}
	processed, err = syncer.SyncDue(t.Context(), 20)
	if processed != 1 || err == nil || repo.jobs["job-00"].StatusCheckAttempts != 2 || !repo.jobs["job-00"].NextStatusCheckAt.Equal(now.Add(60*time.Second)) {
		t.Fatalf("backoff did not grow: %d, %v, %+v", processed, err, repo.jobs["job-00"])
	}
	if counted, ok := errors.AsType[*StatusSyncError](err); !ok || counted.Failures != 1 {
		t.Fatalf("missing failure metric: %v", err)
	}
}

func TestStatusSyncDoesNotReportCompletionWhenPersistenceFails(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"showapi_res_code":1,"printflag":1}`)
	}))
	defer provider.Close()
	now := time.Now().UTC()
	repo := statusFixture(now, 1)
	repo.statusSaveErr = errors.New("write failed")
	service := NewService(repo, nil, nil, fakeClock{now: now}, "key", provider.URL, time.Second)
	processed, err := NewStatusSynchronizer(service, time.Second).SyncDue(t.Context(), 1)
	if processed != 0 || !errors.Is(err, repo.statusSaveErr) || repo.jobs["job-00"].Status != workspace.PrintStatusQueued {
		t.Fatalf("reported unpersisted completion: %d, %v", processed, err)
	}
}

type successfulImagePipeline struct{}

func (successfulImagePipeline) PrintJob(context.Context, *memobirdapi.Client, Job) (*memobirdapi.PrintResponse, error) {
	return &memobirdapi.PrintResponse{PrintContentID: 123}, nil
}

func TestSubmitOnlySendsPrintAndSchedulesBackgroundCheck(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = fmt.Fprint(w, `{"showapi_res_code":1,"printflag":1}`)
	}))
	defer provider.Close()
	now := time.Date(2026, 10, 1, 0, 0, 0, 234567000, time.UTC)
	repo := statusFixture(now, 0)
	repo.jobs["job-1"] = Job{ID: "job-1", UserID: "user-1", PrinterBindingID: "device-1", Status: workspace.PrintStatusPending, CreatedAt: now.Add(-time.Minute), UpdatedAt: now.Add(-time.Second)}
	service := NewService(repo, fakeAuthenticator{}, nil, fakeClock{now: now}, "key", provider.URL, time.Second)
	service.imagePrinter = successfulImagePipeline{}
	job, err := service.SubmitPrintJob(t.Context(), "token", "job-1")
	if err != nil || job.Status != workspace.PrintStatusQueued || job.UpdatedAt != now.Format(time.RFC3339Nano) || calls.Load() != 0 {
		t.Fatalf("submit performed synchronous status query: %+v calls=%d err=%v", job, calls.Load(), err)
	}
	if repo.jobs["job-1"].NextStatusCheckAt == nil || !repo.jobs["job-1"].NextStatusCheckAt.Equal(now) {
		t.Fatal("accepted print was not scheduled for background status check")
	}
}
