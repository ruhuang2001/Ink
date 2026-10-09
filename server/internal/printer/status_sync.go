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
	SubmissionOutcomeUnknown = "提交结果未知，请先检查设备是否已打印，再手动创建新任务。"
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
	Completed bool
	CheckedAt time.Time
	NextCheck time.Time
	Attempts  int
}

type StatusSyncError struct {
	Failures int
	Err      error
}

func (e *StatusSyncError) Error() string     { return e.Err.Error() }
func (e *StatusSyncError) Unwrap() error     { return e.Err }
func (e *StatusSyncError) FailureCount() int { return e.Failures }

type StatusSynchronizer struct {
	service *Service
	timeout time.Duration
}

func NewStatusSynchronizer(service *Service, timeout time.Duration) *StatusSynchronizer {
	return &StatusSynchronizer{service: service, timeout: timeout}
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
		completed, providerErr := s.check(ctx, job, s.timeout)
		if ctx.Err() != nil {
			return processed, syncError(ctx.Err())
		}
		now := s.service.clock.Now()
		result := StatusCheckResult{Completed: completed, CheckedAt: now, NextCheck: now.Add(statusCheckInterval)}
		if providerErr != nil {
			failedJobs++
			result.Attempts = min(job.Attempts+1, 6)
			result.NextCheck = now.Add(min(statusCheckInterval*time.Duration(1<<result.Attempts), 5*time.Minute))
			failures = append(failures, fmt.Errorf("print job %s status: %w", job.ID, providerErr))
		}
		updated, err := s.service.repo.SaveStatusCheck(ctx, job, result)
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
