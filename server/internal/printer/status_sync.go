package printer

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	statusCheckInterval      = 10 * time.Second
	statusBatchBudget        = 15 * time.Second
	defaultStatusMaxAttempts = 6
	SubmissionOutcomeUnknown = "The submission result is unknown. Check whether the device printed the job before creating another one."
	// Preserve the legacy marker so existing uncertain jobs cannot be resubmitted accidentally.
	legacySubmissionOutcomeUnknown = "\u63d0\u4ea4\u7ed3\u679c\u672a\u77e5\uff0c\u8bf7\u5148\u68c0\u67e5\u8bbe\u5907\u662f\u5426\u5df2\u6253\u5370\uff0c\u518d\u624b\u52a8\u521b\u5efa\u65b0\u4efb\u52a1\u3002"
)

// IsSubmissionOutcomeUnknown recognizes both current and legacy persisted markers.
func IsSubmissionOutcomeUnknown(message string) bool {
	return message == SubmissionOutcomeUnknown || message == legacySubmissionOutcomeUnknown
}

// StatusSyncJob excludes the rendered content and carries the version checked
// by SaveStatusCheck, so a delayed provider response cannot overwrite a retry.
type StatusSyncJob struct {
	ID                string
	UserID            string
	ProviderPrintID   int
	Binding           Binding
	UpdatedAt         time.Time
	NextStatusCheckAt time.Time
	Attempts          int
}

type StatusCheckResult struct {
	Completed    bool
	Failed       bool
	ErrorMessage string
	CheckedAt    time.Time
	NextCheck    time.Time
	Attempts     int
}

type StatusSyncError struct {
	Failures int
	Err      error
}

func (e *StatusSyncError) Error() string     { return e.Err.Error() }
func (e *StatusSyncError) Unwrap() error     { return e.Err }
func (e *StatusSyncError) FailureCount() int { return e.Failures }

type StatusSynchronizer struct {
	service            *Service
	timeout            time.Duration
	recheckInterval    time.Duration
	maxRecheckInterval time.Duration
	maxAttempts        int
}

func NewStatusSynchronizer(service *Service, timeout time.Duration) *StatusSynchronizer {
	return NewStatusSynchronizerWithPollingAndAttempts(service, timeout, statusCheckInterval, 5*time.Minute, defaultStatusMaxAttempts)
}

func NewStatusSynchronizerWithPolling(service *Service, timeout, interval, maxInterval time.Duration) *StatusSynchronizer {
	return NewStatusSynchronizerWithPollingAndAttempts(service, timeout, interval, maxInterval, defaultStatusMaxAttempts)
}

func NewStatusSynchronizerWithPollingAndAttempts(service *Service, timeout, interval, maxInterval time.Duration, maxAttempts int) *StatusSynchronizer {
	return &StatusSynchronizer{service: service, timeout: timeout, recheckInterval: interval, maxRecheckInterval: maxInterval, maxAttempts: max(1, maxAttempts)}
}

func (s *StatusSynchronizer) recheckDelay(attempts int, providerFailed bool) time.Duration {
	steps := attempts - 1
	if providerFailed {
		steps++
	}
	delay := s.recheckInterval
	for range steps {
		if delay > s.maxRecheckInterval/2 {
			return s.maxRecheckInterval
		}
		delay *= 2
	}
	return min(delay, s.maxRecheckInterval)
}

func (s *StatusSynchronizer) SyncDue(ctx context.Context, limit int) (int, error) {
	if s.service.accessKey == "" {
		return 0, nil
	}
	limit = min(max(limit, 1), MaxJobPageSize)
	now := s.service.clock.Now()
	jobs, err := s.service.repo.ClaimDueStatusJobs(ctx, now, now.Add(statusBatchBudget+s.timeout+5*time.Second), limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	// Keep a failing provider from monopolizing the worker. Persist each failed
	// check with the parent context before yielding to the next batch.
	deadline := time.Now().Add(statusBatchBudget)
	var failures []error
	failedJobs := 0
	syncError := func(extra error) error {
		if extra != nil {
			failures = append(failures, extra)
		}
		if len(failures) == 0 {
			return nil
		}
		return &StatusSyncError{Failures: failedJobs, Err: errors.Join(failures...)}
	}
	for _, job := range jobs {
		if time.Until(deadline) <= 0 {
			break
		}
		if err := ctx.Err(); err != nil {
			return processed, syncError(err)
		}
		// Each started check gets its full configured timeout. Yield between
		// checks instead of counting an exhausted batch budget as provider failure.
		checkCtx, finish := s.service.StatusObserver.Start(ctx)
		completed, providerErr := s.check(checkCtx, job, s.timeout)
		if ctx.Err() != nil {
			finish(ctx.Err())
			return processed, syncError(ctx.Err())
		}
		now := s.service.clock.Now()
		result := StatusCheckResult{Completed: completed, CheckedAt: now, NextCheck: now.Add(s.recheckInterval)}
		if !completed {
			// A successful "not printed" response also backs off. Keep an
			// accepted job queued: failing or resubmitting it can print twice.
			result.Attempts = min(max(job.Attempts+1, 1), s.maxAttempts)
			result.NextCheck = now.Add(s.recheckDelay(result.Attempts, providerErr != nil))
			if result.Attempts >= s.maxAttempts {
				result.Failed = true
				result.ErrorMessage = SubmissionOutcomeUnknown
				result.NextCheck = now
			}
		}
		if providerErr != nil {
			failedJobs++
			failures = append(failures, fmt.Errorf("print job %s status: %w", job.ID, providerErr))
		}
		updated, err := s.service.repo.SaveStatusCheck(ctx, job, result)
		outcomeErr := errors.Join(providerErr, err)
		if result.Failed {
			outcomeErr = errors.Join(outcomeErr, errors.New("print status attempts exhausted"))
		}
		finish(outcomeErr)
		if err != nil {
			if providerErr == nil {
				failedJobs++
			}
			failures = append(failures, fmt.Errorf("save print job %s status: %w", job.ID, err))
			continue
		}
		if updated {
			processed++
		}
	}
	return processed, syncError(nil)
}

func (s *StatusSynchronizer) check(ctx context.Context, job StatusSyncJob, timeout time.Duration) (bool, error) {
	client, err := s.service.newClient(job.Binding)
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	response, err := client.GetPrintStatus(ctx, job.ProviderPrintID)
	if err != nil {
		return false, err
	}
	return response.IsPrinted(), nil
}
