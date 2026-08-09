package dispatch

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/inbox"
	"github.com/ruhuang/ink/server/internal/plugins"
	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/workspace"
)

type memoryRepo struct {
	mu         sync.Mutex
	items      map[string]inbox.Item
	deliveries map[string]Delivery
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		items:      map[string]inbox.Item{},
		deliveries: map[string]Delivery{},
	}
}

func deliveryKey(scheduleID string, itemID string) string {
	return scheduleID + ":" + itemID
}

func (r *memoryRepo) ListRetryableBySchedule(_ context.Context, scheduleID string, limit int) ([]DeliveryItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := []DeliveryItem{}
	for _, delivery := range r.deliveries {
		if delivery.PrintScheduleID != scheduleID {
			continue
		}
		if delivery.Status != DeliveryStatusFailed && delivery.Status != DeliveryStatusReserved {
			continue
		}
		if delivery.Status == DeliveryStatusFailed && delivery.AttemptCount >= MaxDeliveryAttempts {
			continue
		}
		if delivery.Status == DeliveryStatusReserved && (delivery.LeaseUntil == nil || delivery.LeaseUntil.After(time.Now())) {
			continue
		}

		item := r.items[delivery.PluginItemID]
		result = append(result, DeliveryItem{
			Delivery: delivery,
			Item:     item,
		})
	}
	sort.Slice(result, func(i int, j int) bool {
		if result[i].Item.CreatedAt.Equal(result[j].Item.CreatedAt) {
			return result[i].Delivery.UpdatedAt.Before(result[j].Delivery.UpdatedAt)
		}
		return result[i].Item.CreatedAt.Before(result[j].Item.CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (r *memoryRepo) ClaimDelivery(_ context.Context, candidate Delivery, leaseUntil time.Time) (Delivery, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := deliveryKey(candidate.PrintScheduleID, candidate.PluginItemID)
	delivery, exists := r.deliveries[key]
	now := leaseUntil.Add(-ReservationLease)
	if exists {
		if delivery.Status == DeliveryStatusPrinted ||
			(delivery.Status == DeliveryStatusFailed && delivery.AttemptCount >= MaxDeliveryAttempts) ||
			(delivery.Status == DeliveryStatusReserved && (delivery.LeaseUntil == nil || delivery.LeaseUntil.After(now))) {
			return Delivery{}, false, nil
		}
	} else {
		delivery = candidate
	}
	delivery.Status = DeliveryStatusReserved
	delivery.AttemptCount++
	delivery.LastError = nil
	delivery.PrintJobID = nil
	delivery.DeliveredAt = nil
	delivery.LeaseUntil = &leaseUntil
	delivery.UpdatedAt = now
	r.deliveries[key] = delivery
	return delivery, true, nil
}

func (r *memoryRepo) ListUndeliveredBySchedule(_ context.Context, scheduleID string, bindingID string, limit int) ([]inbox.Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := []inbox.Item{}
	for _, item := range r.items {
		if item.PluginBindingID != bindingID {
			continue
		}
		if item.Status != inbox.StatusPending {
			continue
		}
		if _, exists := r.deliveries[deliveryKey(scheduleID, item.ID)]; exists {
			continue
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i int, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (r *memoryRepo) SaveDelivery(_ context.Context, delivery Delivery) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.deliveries[deliveryKey(delivery.PrintScheduleID, delivery.PluginItemID)] = delivery
	return nil
}

func (r *memoryRepo) CountPrintedInLast24h(_ context.Context, bindingID string, since time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	for _, delivery := range r.deliveries {
		if delivery.Status != DeliveryStatusPrinted {
			continue
		}
		if delivery.DeliveredAt == nil || delivery.DeliveredAt.Before(since) {
			continue
		}
		item := r.items[delivery.PluginItemID]
		if item.PluginBindingID == bindingID {
			count++
		}
	}
	return count, nil
}

type incrementingIDs struct {
	mu    sync.Mutex
	count int
}

func (g *incrementingIDs) New(prefix string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.count++
	return fmt.Sprintf("%s-%d", prefix, g.count), nil
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

type stubPrinter struct {
	mu       sync.Mutex
	created  []printer.CreateJobInput
	nextID   int
	failNext error
	failOnce bool
}

func (p *stubPrinter) CreatePrintJobForUser(_ context.Context, _ string, input printer.CreateJobInput) (workspace.PrintJob, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.failNext != nil {
		err := p.failNext
		if p.failOnce {
			p.failNext = nil
		}
		return workspace.PrintJob{}, err
	}

	p.created = append(p.created, input)
	if input.JobID != "" {
		return workspace.PrintJob{ID: input.JobID}, nil
	}
	p.nextID++
	return workspace.PrintJob{ID: fmt.Sprintf("job-%d", p.nextID)}, nil
}

type stubWorkspace struct{}

func (stubWorkspace) FindByUserID(_ context.Context, _ string) (*workspace.State, error) {
	state := workspace.EmptyState()
	return &state, nil
}

func buildItem(id string, bindingID string, title string, createdAt time.Time) inbox.Item {
	return inbox.Item{
		ID:                   id,
		UserID:               "user-1",
		PluginInstallationID: "install-1",
		PluginBindingID:      bindingID,
		ExternalID:           id,
		Title:                title,
		SourceLabel:          "Fixture Source",
		Status:               inbox.StatusPending,
		Blocks:               []plugins.ContentBlock{{Type: plugins.BlockParagraph, Text: title}},
		FetchedAt:            createdAt,
		CreatedAt:            createdAt,
		UpdatedAt:            createdAt,
	}
}

func newService(now time.Time, repo *memoryRepo, printerStub *stubPrinter) *Service {
	return NewService(repo, printerStub, stubWorkspace{}, &incrementingIDs{}, fixedClock{now: now})
}

func buildScheduleRunInput(scheduleID string, batchSize int) ScheduleRunInput {
	return ScheduleRunInput{
		ScheduleID:   scheduleID,
		Binding:      plugins.Binding{ID: "binding-1", UserID: "user-1", MaxPrintsPerRun: 10, MaxPrintsPerDay: 10},
		Installation: plugins.Installation{DisplayName: "Fixture Source"},
		DeviceID:     "device-1",
		BatchSize:    batchSize,
	}
}

func mustRunSchedule(t *testing.T, service *Service, input ScheduleRunInput) ScheduleRunResult {
	t.Helper()

	result, err := service.RunSchedule(context.Background(), input)
	if err != nil {
		t.Fatalf("run schedule: %v", err)
	}
	return result
}

func assertRunCounts(t *testing.T, result ScheduleRunResult, printed int, failed int, skipped int) {
	t.Helper()
	if result.Printed != printed || result.Failed != failed || result.Skipped != skipped {
		t.Fatalf("unexpected run result: %+v", result)
	}
}

func assertDeliveryPresence(t *testing.T, repo *memoryRepo, scheduleID string, itemID string, want bool) {
	t.Helper()
	_, exists := repo.deliveries[deliveryKey(scheduleID, itemID)]
	if exists != want {
		t.Fatalf("unexpected delivery presence for %s/%s: got=%t want=%t", scheduleID, itemID, exists, want)
	}
}

func TestRunSchedulePrintsOldestUndeliveredItemsUpToBatchSize(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepo()
	repo.items["item-1"] = buildItem("item-1", "binding-1", "Oldest", now.Add(-3*time.Minute))
	repo.items["item-2"] = buildItem("item-2", "binding-1", "Middle", now.Add(-2*time.Minute))
	repo.items["item-3"] = buildItem("item-3", "binding-1", "Newest", now.Add(-1*time.Minute))

	printerStub := &stubPrinter{}
	service := newService(now, repo, printerStub)
	result := mustRunSchedule(t, service, buildScheduleRunInput("schedule-1", 2))
	assertRunCounts(t, result, 2, 0, 0)
	if len(printerStub.created) != 2 {
		t.Fatalf("expected 2 print jobs, got %d", len(printerStub.created))
	}
	if printerStub.created[0].Title != "Oldest" || printerStub.created[1].Title != "Middle" {
		t.Fatalf("expected oldest items first, got %+v", printerStub.created)
	}
	assertDeliveryPresence(t, repo, "schedule-1", "item-1", true)
	assertDeliveryPresence(t, repo, "schedule-1", "item-2", true)
	assertDeliveryPresence(t, repo, "schedule-1", "item-3", false)
}

func TestRunScheduleAllowsDifferentSchedulesToPrintSameCollectedItem(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepo()
	repo.items["item-1"] = buildItem("item-1", "binding-1", "Shared Item", now.Add(-1*time.Minute))

	printerStub := &stubPrinter{}
	service := newService(now, repo, printerStub)
	first := mustRunSchedule(t, service, buildScheduleRunInput("schedule-1", 1))
	second := mustRunSchedule(t, service, buildScheduleRunInput("schedule-2", 1))

	if first.Printed != 1 || second.Printed != 1 {
		t.Fatalf("expected both schedules to print the shared item, got first=%+v second=%+v", first, second)
	}
	if len(printerStub.created) != 2 {
		t.Fatalf("expected 2 print jobs, got %d", len(printerStub.created))
	}
	assertDeliveryPresence(t, repo, "schedule-1", "item-1", true)
	assertDeliveryPresence(t, repo, "schedule-2", "item-1", true)
}

func TestRunScheduleRetriesFailedDeliveriesWithoutDuplicatingRows(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepo()
	repo.items["item-1"] = buildItem("item-1", "binding-1", "Retry Me", now.Add(-1*time.Minute))

	printerStub := &stubPrinter{
		failNext: errors.New("printer offline"),
		failOnce: true,
	}
	service := newService(now, repo, printerStub)
	input := buildScheduleRunInput("schedule-1", 1)
	first := mustRunSchedule(t, service, input)
	assertRunCounts(t, first, 0, 1, 0)

	failedDelivery := repo.deliveries[deliveryKey("schedule-1", "item-1")]
	if failedDelivery.Status != DeliveryStatusFailed || failedDelivery.AttemptCount != 1 {
		t.Fatalf("expected failed delivery with one attempt, got %+v", failedDelivery)
	}

	second := mustRunSchedule(t, service, input)
	assertRunCounts(t, second, 1, 0, 0)

	retried := repo.deliveries[deliveryKey("schedule-1", "item-1")]
	if retried.ID != failedDelivery.ID {
		t.Fatalf("expected retry to reuse delivery row, got old=%s new=%s", failedDelivery.ID, retried.ID)
	}
	if retried.Status != DeliveryStatusPrinted || retried.AttemptCount != 2 {
		t.Fatalf("unexpected retried delivery state: %+v", retried)
	}
	if len(repo.deliveries) != 1 {
		t.Fatalf("expected one delivery row after retry, got %d", len(repo.deliveries))
	}
}

func TestConcurrentRunScheduleClaimsOnce(t *testing.T) {
	now := time.Now().UTC()
	repo := newMemoryRepo()
	repo.items["item-1"] = buildItem("item-1", "binding-1", "Once", now.Add(-time.Minute))
	printerStub := &stubPrinter{}
	service := newService(now, repo, printerStub)
	input := buildScheduleRunInput("schedule-1", 1)

	start := make(chan struct{})
	results := make(chan ScheduleRunResult, 2)
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			result, err := service.RunSchedule(context.Background(), input)
			results <- result
			errs <- err
		}()
	}
	close(start)
	printed := 0
	for range 2 {
		printed += (<-results).Printed
		if err := <-errs; err != nil {
			t.Fatalf("run schedule: %v", err)
		}
	}
	if printed != 1 || len(printerStub.created) != 1 || len(repo.deliveries) != 1 {
		t.Fatalf("expected one claim and print, printed=%d jobs=%d deliveries=%d", printed, len(printerStub.created), len(repo.deliveries))
	}
}

func TestDispatchOneSkipsActiveReservation(t *testing.T) {
	now := time.Now().UTC()
	repo := newMemoryRepo()
	item := buildItem("item-1", "binding-1", "Reserved", now.Add(-time.Minute))
	lease := now.Add(time.Minute)
	delivery := Delivery{ID: "delivery-stable", PrintScheduleID: "schedule-1", PluginItemID: item.ID, Status: DeliveryStatusReserved, AttemptCount: 1, LeaseUntil: &lease, CreatedAt: now, UpdatedAt: now}
	repo.items[item.ID] = item
	repo.deliveries[deliveryKey("schedule-1", item.ID)] = delivery
	printerStub := &stubPrinter{}
	service := newService(now, repo, printerStub)

	outcome, _, err := service.dispatchOne(context.Background(), buildScheduleRunInput("schedule-1", 1), DeliveryItem{Delivery: delivery, Item: item}, true)
	if err != nil || outcome != "" || len(printerStub.created) != 0 {
		t.Fatalf("active reservation was not skipped: outcome=%q jobs=%d err=%v", outcome, len(printerStub.created), err)
	}
}

func TestRunScheduleReclaimsExpiredReservationWithStableID(t *testing.T) {
	now := time.Now().UTC()
	repo := newMemoryRepo()
	item := buildItem("item-1", "binding-1", "Recover", now.Add(-time.Minute))
	expired := now.Add(-time.Minute)
	delivery := Delivery{ID: "delivery-stable", PrintScheduleID: "schedule-1", PluginItemID: item.ID, Status: DeliveryStatusReserved, AttemptCount: 1, LeaseUntil: &expired, CreatedAt: now.Add(-time.Hour), UpdatedAt: expired}
	repo.items[item.ID] = item
	repo.deliveries[deliveryKey("schedule-1", item.ID)] = delivery
	printerStub := &stubPrinter{}
	service := newService(now, repo, printerStub)

	result := mustRunSchedule(t, service, buildScheduleRunInput("schedule-1", 1))
	assertRunCounts(t, result, 1, 0, 0)
	got := repo.deliveries[deliveryKey("schedule-1", item.ID)]
	if got.ID != delivery.ID || got.AttemptCount != 2 || len(printerStub.created) != 1 || printerStub.created[0].JobID != delivery.ID {
		t.Fatalf("expired reservation not stably reclaimed: delivery=%+v jobs=%+v", got, printerStub.created)
	}
}
