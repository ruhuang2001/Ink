package printer

import (
	"context"
	"errors"
	"fmt"
	"time"

	memobird "github.com/ruhuang2001/memobird-go/memobird"
)

const (
	statusCheckInterval      = 10 * time.Second
	statusBatchBudget        = 15 * time.Second
	defaultStatusMaxAttempts = 6
	// Failure codes are the only values persisted in print_jobs.error_message.
	// The API returns them as errorMessage and the web client localizes them, so
	// they are stable across locales rather than user-facing prose.
	SubmissionOutcomeUnknown = "print_submission_unknown"
	ProviderPrintIDUnknown   = "print_provider_print_id_unknown"
)

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

// StatusCheckOutcome is one provider observation. Unknown means the provider
// has no record of the job, which is terminal but is not a completion.
type StatusCheckOutcome struct {
	Printed bool
	Unknown bool
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
		outcome, providerErr := s.check(checkCtx, job, s.timeout)
		if ctx.Err() != nil {
			finish(ctx.Err())
			return processed, syncError(ctx.Err())
		}
		now := s.service.clock.Now()
		result := StatusCheckResult{Completed: outcome.Printed, CheckedAt: now, NextCheck: now.Add(s.recheckInterval)}
		switch {
		case outcome.Unknown:
			// The provider has no record of this job, so no later check can
			// succeed. Fail on the first observation instead of backing off for
			// the full attempt budget.
			result.Failed = true
			result.ErrorMessage = ProviderPrintIDUnknown
			result.NextCheck = now
		case !outcome.Printed:
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
		switch {
		case outcome.Unknown:
			outcomeErr = errors.Join(outcomeErr, fmt.Errorf("print job %s: %w", job.ID, memobird.ErrUnknownPrintID))
			// The job is settled even when the provider answered cleanly, so the
			// caller sees why it failed rather than an unexplained drop.
			if err == nil {
				failedJobs++
				failures = append(failures, outcomeErr)
			}
		case result.Failed:
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

func (s *StatusSynchronizer) check(ctx context.Context, job StatusSyncJob, timeout time.Duration) (StatusCheckOutcome, error) {
	client, err := s.service.newClient(job.Binding)
	if err != nil {
		return StatusCheckOutcome{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	response, err := client.GetPrintStatus(ctx, job.ProviderPrintID)
	if err != nil {
		return StatusCheckOutcome{}, err
	}
	return StatusCheckOutcome{
		Printed: response.IsPrinted(),
		Unknown: response.IsUnknown(),
	}, nil
}
