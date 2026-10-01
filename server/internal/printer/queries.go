package printer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/ruhuang/ink/server/internal/workspace"
)

const MaxJobPageSize = 100

// JobSummary contains only the fields needed to display a list row.
type JobSummary struct {
	ID        string                `json:"id"`
	Title     string                `json:"title"`
	Source    string                `json:"source"`
	DeviceID  string                `json:"deviceId"`
	Status    workspace.PrintStatus `json:"status"`
	CreatedAt string                `json:"createdAt"`
	UpdatedAt string                `json:"updatedAt"`
}

type JobSummaryRecord struct {
	ID        string
	Title     string
	Source    string
	DeviceID  string
	Status    workspace.PrintStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ListJobsInput struct {
	Status string
	Limit  int
	Cursor string
}

type JobCursor struct {
	CreatedAt time.Time `json:"createdAt"`
	ID        string    `json:"id"`
	Status    string    `json:"status"`
}

type JobPageQuery struct {
	Status string
	Limit  int
	Cursor *JobCursor
}

type JobPage struct {
	PrintJobs  []JobSummary `json:"printJobs"`
	NextCursor *string      `json:"nextCursor"`
}

type JobStatus struct {
	ID        string                `json:"id"`
	DeviceID  string                `json:"deviceId"`
	Status    workspace.PrintStatus `json:"status"`
	UpdatedAt string                `json:"updatedAt"`
}

type JobStatusRecord struct {
	ID        string
	DeviceID  string
	Status    workspace.PrintStatus
	UpdatedAt time.Time
}

type JobCounts struct {
	Pending        int `json:"pending"`
	Queued         int `json:"queued"`
	Completed      int `json:"completed"`
	Failed         int `json:"failed"`
	Cancelled      int `json:"cancelled"`
	TodayCompleted int `json:"todayCompleted"`
}

type JobStatusesInput struct {
	IDs   []string
	Since time.Time
}

type JobStatuses struct {
	PrintJobs   []JobStatus `json:"printJobs"`
	Counts      JobCounts   `json:"counts"`
	LatestJobID *string     `json:"latestJobId"`
}

func (s *Service) ListPrintJobs(ctx context.Context, accessToken string, input ListJobsInput) (JobPage, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return JobPage{}, err
	}
	if input.Limit == 0 {
		input.Limit = 20
	}
	if input.Limit < 1 || input.Limit > MaxJobPageSize {
		return JobPage{}, ErrInvalidInput
	}
	if input.Status == "" {
		input.Status = "all"
	}
	if input.Status != "all" && input.Status != "active" && input.Status != "history" {
		return JobPage{}, ErrInvalidInput
	}
	query := JobPageQuery{Status: input.Status, Limit: input.Limit + 1}
	if input.Cursor != "" {
		if len(input.Cursor) > 1024 {
			return JobPage{}, ErrInvalidInput
		}
		encoded, err := base64.RawURLEncoding.DecodeString(input.Cursor)
		if err != nil {
			return JobPage{}, ErrInvalidInput
		}
		var cursor JobCursor
		if err := json.Unmarshal(encoded, &cursor); err != nil || cursor.ID == "" || cursor.CreatedAt.IsZero() || cursor.Status != input.Status {
			return JobPage{}, ErrInvalidInput
		}
		query.Cursor = &cursor
	}
	jobs, err := s.repo.ListJobSummaries(ctx, currentUser.ID, query)
	if err != nil {
		return JobPage{}, err
	}
	page := JobPage{PrintJobs: make([]JobSummary, 0, min(input.Limit, len(jobs)))}
	if len(jobs) > input.Limit {
		jobs = jobs[:input.Limit]
		last := jobs[len(jobs)-1]
		encoded, err := json.Marshal(JobCursor{CreatedAt: last.CreatedAt, ID: last.ID, Status: input.Status})
		if err != nil {
			return JobPage{}, err
		}
		page.NextCursor = new(base64.RawURLEncoding.EncodeToString(encoded))
	}
	for _, job := range jobs {
		page.PrintJobs = append(page.PrintJobs, JobSummary{
			ID: job.ID, Title: job.Title, Source: job.Source, DeviceID: job.DeviceID, Status: job.Status,
			CreatedAt: job.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: job.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	return page, nil
}

func (s *Service) GetPrintJobStatuses(ctx context.Context, accessToken string, input JobStatusesInput) (JobStatuses, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return JobStatuses{}, err
	}
	if len(input.IDs) > MaxJobPageSize {
		return JobStatuses{}, ErrInvalidInput
	}
	ids := make([]string, 0, len(input.IDs))
	seen := make(map[string]bool, len(input.IDs))
	for _, id := range input.IDs {
		id = strings.TrimSpace(id)
		if id == "" || len(id) > 200 {
			return JobStatuses{}, ErrInvalidInput
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if input.Since.IsZero() {
		now := s.clock.Now().UTC()
		input.Since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	}
	counts, latestJobID, err := s.repo.GetJobCounts(ctx, currentUser.ID, input.Since)
	if err != nil {
		return JobStatuses{}, err
	}
	result := JobStatuses{PrintJobs: make([]JobStatus, 0, len(ids)), Counts: counts, LatestJobID: latestJobID}
	if len(ids) == 0 {
		return result, nil
	}
	jobs, err := s.repo.ListJobStatuses(ctx, currentUser.ID, ids)
	if err != nil {
		return JobStatuses{}, err
	}
	for _, job := range jobs {
		result.PrintJobs = append(result.PrintJobs, JobStatus{
			ID: job.ID, DeviceID: job.DeviceID, Status: job.Status, UpdatedAt: job.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	return result, nil
}

func (s *Service) GetPrintJob(ctx context.Context, accessToken string, jobID string) (workspace.PrintJob, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return workspace.PrintJob{}, err
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return workspace.PrintJob{}, ErrInvalidInput
	}
	job, err := s.repo.FindJobByID(ctx, currentUser.ID, jobID)
	if err != nil {
		return workspace.PrintJob{}, err
	}
	if job == nil {
		return workspace.PrintJob{}, ErrNotFound
	}
	return mapJob(*job), nil
}
