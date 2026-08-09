package dispatch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ruhuang/ink/server/internal/inbox"
	"github.com/ruhuang/ink/server/internal/plugins"
	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/workspace"
)

type DeliveryStatus string

const (
	DeliveryStatusReserved DeliveryStatus = "reserved"
	DeliveryStatusPrinted  DeliveryStatus = "printed"
	DeliveryStatusFailed   DeliveryStatus = "failed"

	DefaultDailyCap     = 50
	MaxDeliveryAttempts = 3
	ReservationLease    = 2 * time.Minute
)

type Delivery struct {
	ID              string
	PrintScheduleID string
	PluginItemID    string
	Status          DeliveryStatus
	AttemptCount    int
	LastError       *string
	PrintJobID      *string
	DeliveredAt     *time.Time
	LeaseUntil      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type DeliveryItem struct {
	Delivery Delivery
	Item     inbox.Item
}

type ScheduleRunInput struct {
	ScheduleID   string
	Binding      plugins.Binding
	Installation plugins.Installation
	DeviceID     string
	BatchSize    int
}

type ScheduleRunResult struct {
	Printed     int      `json:"printedCount"`
	Failed      int      `json:"failedCount"`
	Skipped     int      `json:"skippedCount"`
	PrintJobIDs []string `json:"printJobIds"`
}

type Repository interface {
	ListRetryableBySchedule(ctx context.Context, scheduleID string, limit int) ([]DeliveryItem, error)
	ListUndeliveredBySchedule(ctx context.Context, scheduleID string, bindingID string, limit int) ([]inbox.Item, error)
	ClaimDelivery(ctx context.Context, delivery Delivery, leaseUntil time.Time) (Delivery, bool, error)
	SaveDelivery(ctx context.Context, delivery Delivery) error
	CountPrintedInLast24h(ctx context.Context, bindingID string, since time.Time) (int, error)
}

type PrinterJobCreator interface {
	CreatePrintJobForUser(ctx context.Context, userID string, input printer.CreateJobInput) (workspace.PrintJob, error)
}

type WorkspaceRepository interface {
	FindByUserID(ctx context.Context, userID string) (*workspace.State, error)
}

type IDGenerator interface {
	New(prefix string) (string, error)
}

type Clock interface {
	Now() time.Time
}

type Service struct {
	repo      Repository
	printer   PrinterJobCreator
	workspace WorkspaceRepository
	ids       IDGenerator
	clock     Clock
}

func NewService(
	repo Repository,
	printerCreator PrinterJobCreator,
	workspaceRepo WorkspaceRepository,
	ids IDGenerator,
	clock Clock,
) *Service {
	return &Service{
		repo:      repo,
		printer:   printerCreator,
		workspace: workspaceRepo,
		ids:       ids,
		clock:     clock,
	}
}

func (s *Service) RunSchedule(ctx context.Context, input ScheduleRunInput) (ScheduleRunResult, error) {
	result := ScheduleRunResult{}
	scheduleID, deviceID, err := normalizeScheduleRunInput(input)
	if err != nil {
		return result, err
	}
	input.ScheduleID = scheduleID
	input.DeviceID = deviceID

	budget, err := s.dispatchBudget(ctx, input.Binding, input.BatchSize)
	if err != nil {
		return result, err
	}
	if budget <= 0 {
		return result, nil
	}

	candidates, err := s.loadScheduleCandidates(ctx, scheduleID, input.Binding.ID, budget)
	if err != nil {
		return result, err
	}

	sendConfirmation := s.resolveSendConfirmation(ctx, input.Binding.UserID)
	for _, candidate := range candidates {
		outcome, jobID, err := s.dispatchOne(ctx, input, candidate, sendConfirmation)
		if err != nil {
			return result, err
		}
		result.record(outcome, jobID)
	}

	return result, nil
}

func (s *Service) dispatchBudget(ctx context.Context, binding plugins.Binding, batchSize int) (int, error) {
	perRun := resolvePerRunLimit(binding.MaxPrintsPerRun, batchSize)
	remainingDay, err := s.remainingDailyBudget(ctx, binding)
	if err != nil {
		return 0, err
	}
	if remainingDay <= 0 {
		return 0, nil
	}
	return minInt(perRun, remainingDay), nil
}

func (s *Service) resolveSendConfirmation(ctx context.Context, userID string) bool {
	if s.workspace == nil {
		return workspace.EmptyState().Preferences.SendConfirmationEnabled
	}
	state, err := s.workspace.FindByUserID(ctx, userID)
	if err != nil {
		return true
	}
	if state == nil {
		return workspace.EmptyState().Preferences.SendConfirmationEnabled
	}
	return workspace.NormalizeState(*state).Preferences.SendConfirmationEnabled
}

func (s *Service) dispatchOne(
	ctx context.Context,
	input ScheduleRunInput,
	current DeliveryItem,
	sendConfirmation bool,
) (DeliveryStatus, string, error) {
	delivery, err := s.buildDelivery(current, input.ScheduleID)
	if err != nil {
		return "", "", err
	}
	delivery, claimed, err := s.repo.ClaimDelivery(ctx, delivery, s.clock.Now().Add(ReservationLease))
	if err != nil {
		return "", "", err
	}
	if !claimed {
		return "", "", nil
	}
	current.Delivery = delivery

	rendered, err := printer.RenderBlocksToText(current.Item.Blocks)
	if err != nil {
		return s.saveFailed(ctx, current, input.ScheduleID, fmt.Sprintf("render: %s", err.Error()))
	}

	source := strings.TrimSpace(current.Item.SourceLabel)
	if source == "" {
		source = input.Installation.DisplayName
	}

	job, err := s.printer.CreatePrintJobForUser(ctx, input.Binding.UserID, printer.CreateJobInput{
		JobID:             delivery.ID,
		Title:             current.Item.Title,
		Source:            source,
		Content:           rendered,
		PrinterBindingID:  input.DeviceID,
		SubmitImmediately: !sendConfirmation,
	})
	if err != nil {
		return s.saveFailed(ctx, current, input.ScheduleID, err.Error())
	}

	now := s.clock.Now()
	delivery.Status = DeliveryStatusPrinted
	delivery.LastError = nil
	delivery.LeaseUntil = nil
	delivery.UpdatedAt = now
	delivery.DeliveredAt = &now
	delivery.PrintJobID = &job.ID
	if err := s.repo.SaveDelivery(ctx, delivery); err != nil {
		return "", "", err
	}
	return DeliveryStatusPrinted, job.ID, nil
}

func (s *Service) saveFailed(ctx context.Context, current DeliveryItem, scheduleID string, reason string) (DeliveryStatus, string, error) {
	now := s.clock.Now()
	delivery, err := s.buildDelivery(current, scheduleID)
	if err != nil {
		return "", "", err
	}
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		trimmed = "dispatch failed"
	}
	delivery.Status = DeliveryStatusFailed
	delivery.LastError = &trimmed
	delivery.LeaseUntil = nil
	delivery.PrintJobID = nil
	delivery.DeliveredAt = nil
	delivery.UpdatedAt = now
	if err := s.repo.SaveDelivery(ctx, delivery); err != nil {
		return "", "", err
	}
	return DeliveryStatusFailed, "", nil
}

func (s *Service) buildDelivery(current DeliveryItem, scheduleID string) (Delivery, error) {
	if current.Delivery.ID != "" {
		return current.Delivery, nil
	}
	id, err := s.ids.New("delivery")
	if err != nil {
		return Delivery{}, err
	}
	now := s.clock.Now()
	return Delivery{
		ID:              id,
		PrintScheduleID: scheduleID,
		PluginItemID:    current.Item.ID,
		AttemptCount:    0,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func normalizeScheduleRunInput(input ScheduleRunInput) (string, string, error) {
	scheduleID := strings.TrimSpace(input.ScheduleID)
	if scheduleID == "" {
		return "", "", fmt.Errorf("schedule id is required")
	}
	deviceID := strings.TrimSpace(input.DeviceID)
	if deviceID == "" {
		return "", "", fmt.Errorf("device id is required")
	}
	return scheduleID, deviceID, nil
}

func (s *Service) loadScheduleCandidates(
	ctx context.Context,
	scheduleID string,
	bindingID string,
	budget int,
) ([]DeliveryItem, error) {
	failed, err := s.repo.ListRetryableBySchedule(ctx, scheduleID, budget)
	if err != nil {
		return nil, err
	}
	candidates := make([]DeliveryItem, 0, budget)
	candidates = append(candidates, failed...)
	remaining := budget - len(candidates)
	if remaining <= 0 {
		return candidates, nil
	}
	items, err := s.repo.ListUndeliveredBySchedule(ctx, scheduleID, bindingID, remaining)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		candidates = append(candidates, DeliveryItem{Item: item})
	}
	return candidates, nil
}

func resolvePerRunLimit(maxPerRun int, batchSize int) int {
	perRun := maxPerRun
	if perRun <= 0 {
		perRun = batchSize
	}
	if perRun <= 0 {
		perRun = 1
	}
	if batchSize > 0 {
		perRun = minInt(perRun, batchSize)
	}
	return perRun
}

func (s *Service) remainingDailyBudget(ctx context.Context, binding plugins.Binding) (int, error) {
	perDay := binding.MaxPrintsPerDay
	if perDay <= 0 {
		perDay = DefaultDailyCap
	}
	printedToday, err := s.repo.CountPrintedInLast24h(ctx, binding.ID, s.clock.Now().Add(-24*time.Hour))
	if err != nil {
		return 0, err
	}
	return perDay - printedToday, nil
}

func minInt(left int, right int) int {
	if right < left {
		return right
	}
	return left
}

func (r *ScheduleRunResult) record(outcome DeliveryStatus, jobID string) {
	switch outcome {
	case DeliveryStatusPrinted:
		r.Printed++
		r.PrintJobIDs = append(r.PrintJobIDs, jobID)
	case DeliveryStatusFailed:
		r.Failed++
	default:
		r.Skipped++
	}
}
