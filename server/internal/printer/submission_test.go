package printer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/workspace"
)

func submissionFixture(providerURL string) (*Service, *fakePrinterRepo) {
	repo := newFakePrinterRepo()
	repo.bindings["device-1"] = Binding{ID: "device-1", UserID: "user-1", DeviceIdentifier: "synthetic-device", ProviderUserID: 123, Status: workspace.DeviceStatusConnected}
	service := NewService(repo, fakeAuthenticator{}, fakeIDGenerator{}, fakeClock{now: time.Now().UTC()}, "synthetic-key", providerURL, time.Second)
	return service, repo
}

func automaticSubmission() CreateJobInput {
	return CreateJobInput{JobID: "delivery-stable", Title: "Synthetic receipt", Content: "Synthetic content", PrinterBindingID: "device-1", SubmitImmediately: true}
}

func TestSubmissionErrorsPreserveUnknownOutcomeAndRejectRetry(t *testing.T) {
	for _, test := range []struct {
		name    string
		respond func(http.ResponseWriter, *http.Request)
	}{
		{"EOF after acceptance", func(w http.ResponseWriter, _ *http.Request) {
			connection, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = connection.Close()
			}
		}},
		{"truncated response body", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "100")
			_, _ = io.WriteString(w, `{"showapi_res_code":1}`)
		}},
		{"invalid response JSON", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"showapi_res_code":`) }},
		{"invalid rejection response", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"showapi_res_code":0,"printcontentid":"invalid"}`)
		}},
		{"non-2xx response", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, `{"showapi_res_code":0}`)
		}},
		{"timeout after acceptance", func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			var accepted atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				if r.URL.Path == "/home/getSignalBase64Pic" {
					_, _ = io.WriteString(w, `{"showapi_res_code":1,"result":"synthetic-bitmap"}`)
					return
				}
				accepted.Add(1)
				test.respond(w, r)
			}))
			defer provider.Close()
			service, repo := submissionFixture(provider.URL)
			service.timeout = 100 * time.Millisecond
			input := automaticSubmission()
			_, err := service.CreatePrintJobForUser(t.Context(), "user-1", input)
			if !errors.Is(err, ErrSubmissionOutcomeUnknown) || !errors.Is(err, ErrUnavailable) {
				t.Fatalf("expected provider error, got %v", err)
			}
			job := repo.jobs[input.JobID]
			if accepted.Load() != 1 || job.Status != workspace.PrintStatusFailed || job.ErrorMessage == nil || *job.ErrorMessage != SubmissionOutcomeUnknown {
				t.Fatalf("uncertain request must retain the recovery instruction: accepted=%d job=%+v", accepted.Load(), job)
			}
			if _, err := service.CreatePrintJobForUser(t.Context(), "user-1", input); !errors.Is(err, ErrSubmissionOutcomeUnknown) || accepted.Load() != 1 {
				t.Fatalf("uncertain request was resent: accepted=%d error=%v", accepted.Load(), err)
			}
		})
	}
}

func TestExplicitProviderRejectionRemainsRetryable(t *testing.T) {
	var prints atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.URL.Path == "/home/getSignalBase64Pic" {
			_, _ = io.WriteString(w, `{"showapi_res_code":1,"result":"synthetic-bitmap"}`)
			return
		}
		if prints.Add(1) == 1 {
			_, _ = io.WriteString(w, `{"showapi_res_code":0,"showapi_res_error":"device offline"}`)
			return
		}
		_, _ = io.WriteString(w, `{"showapi_res_code":1,"printcontentid":321}`)
	}))
	defer provider.Close()
	service, repo := submissionFixture(provider.URL)
	input := automaticSubmission()
	if _, err := service.CreatePrintJobForUser(t.Context(), "user-1", input); !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrSubmissionOutcomeUnknown) {
		t.Fatalf("expected explicit rejection, got %v", err)
	}
	if job := repo.jobs[input.JobID]; job.Status != workspace.PrintStatusFailed || job.ErrorMessage == nil || *job.ErrorMessage == SubmissionOutcomeUnknown {
		t.Fatalf("explicit rejection must remain retryable: %+v", job)
	}
	job, err := service.CreatePrintJobForUser(t.Context(), "user-1", input)
	if err != nil || prints.Load() != 2 || job.Status != workspace.PrintStatusQueued {
		t.Fatalf("explicit rejection retry failed: prints=%d job=%+v error=%v", prints.Load(), job, err)
	}
}

func TestConversionFailureDoesNotMarkPrintOutcomeUnknown(t *testing.T) {
	var calls atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/home/getSignalBase64Pic" {
			t.Errorf("conversion failure reached print endpoint %s", r.URL.Path)
		}
		calls.Add(1)
		_, _ = io.WriteString(w, `{"showapi_res_code":0,"showapi_res_error":"conversion unavailable"}`)
	}))
	defer provider.Close()
	service, repo := submissionFixture(provider.URL)
	input := automaticSubmission()
	for range 2 {
		if _, err := service.CreatePrintJobForUser(t.Context(), "user-1", input); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("expected conversion failure, got %v", err)
		}
	}
	if calls.Load() != 2 || *repo.jobs[input.JobID].ErrorMessage == SubmissionOutcomeUnknown {
		t.Fatalf("conversion failure should safely retry: calls=%d job=%+v", calls.Load(), repo.jobs[input.JobID])
	}
}

func TestCanceledSubmissionPersistsUnknownOutcome(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var accepted atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.URL.Path == "/home/getSignalBase64Pic" {
			_, _ = io.WriteString(w, `{"showapi_res_code":1,"result":"synthetic-bitmap"}`)
			return
		}
		accepted.Add(1)
		cancel()
		<-r.Context().Done()
	}))
	defer provider.Close()
	service, base := submissionFixture(provider.URL)
	repo := &contextCheckingPrinterRepo{fakePrinterRepo: base}
	service.repo = repo
	input := automaticSubmission()
	base.jobs[input.JobID] = Job{ID: input.JobID, UserID: "user-1", PrinterBindingID: "device-1", Title: input.Title, Content: input.Content, Status: workspace.PrintStatusPending}
	if _, err := service.SubmitPrintJob(ctx, "access-token", input.JobID); !errors.Is(err, ErrSubmissionOutcomeUnknown) {
		t.Fatalf("expected cancellation error, got %v", err)
	}
	job := base.jobs[input.JobID]
	if accepted.Load() != 1 || job.ErrorMessage == nil || *job.ErrorMessage != SubmissionOutcomeUnknown || !repo.boundedSaveContext {
		t.Fatalf("canceled request did not persist bounded recovery state: calls=%d job=%+v bounded=%t", accepted.Load(), job, repo.boundedSaveContext)
	}
}

type submissionSaveFailureRepo struct {
	*fakePrinterRepo
	failResult bool
}

func (r *submissionSaveFailureRepo) SaveJob(ctx context.Context, job Job) error {
	if r.failResult && (job.Status == workspace.PrintStatusFailed || job.ProviderPrintContentID != nil) {
		return fmt.Errorf("synthetic result save failure")
	}
	return r.fakePrinterRepo.SaveJob(ctx, job)
}

func TestRetryResultSaveFailureLeavesQueuedJobWithoutResubmission(t *testing.T) {
	var prints atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/home/getSignalBase64Pic" {
			_, _ = io.WriteString(w, `{"showapi_res_code":1,"result":"synthetic-bitmap"}`)
			return
		}
		prints.Add(1)
		_, _ = io.WriteString(w, `{"showapi_res_code":1,"printcontentid":321}`)
	}))
	defer provider.Close()
	service, base := submissionFixture(provider.URL)
	input := automaticSubmission()
	base.jobs[input.JobID] = Job{ID: input.JobID, UserID: "user-1", PrinterBindingID: "device-1", Title: input.Title, Content: input.Content, Status: workspace.PrintStatusFailed}
	service.repo = &submissionSaveFailureRepo{fakePrinterRepo: base, failResult: true}
	if _, err := service.CreatePrintJobForUser(t.Context(), "user-1", input); !errors.Is(err, ErrSubmissionOutcomeUnknown) {
		t.Fatal("expected result save failure")
	}
	if job := base.jobs[input.JobID]; job.Status != workspace.PrintStatusQueued || job.ProviderPrintContentID != nil {
		t.Fatalf("failed result save must leave conservative in-flight state: %+v", job)
	}
	if _, err := service.CreatePrintJobForUser(t.Context(), "user-1", input); err != nil || prints.Load() != 1 {
		t.Fatalf("failed result save resent accepted print: calls=%d error=%v", prints.Load(), err)
	}
}
