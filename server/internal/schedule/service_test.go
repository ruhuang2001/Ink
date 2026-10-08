package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/auth"
	"github.com/ruhuang/ink/server/internal/dispatch"
	"github.com/ruhuang/ink/server/internal/plugins"
	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/workspace"
)

type scheduleRepo struct {
	schedules map[string]PrintSchedule
	claimed   []PrintSchedule
	onSave    func(*PrintSchedule)
}

func newScheduleRepo() *scheduleRepo {
	return &scheduleRepo{
		schedules: map[string]PrintSchedule{},
	}
}

func (r *scheduleRepo) ListByUserID(_ context.Context, userID string) ([]PrintSchedule, error) {
	result := []PrintSchedule{}
	for _, current := range r.schedules {
		if current.UserID == userID {
			result = append(result, current)
		}
	}
	return result, nil
}

func (r *scheduleRepo) FindByID(_ context.Context, userID string, scheduleID string) (*PrintSchedule, error) {
	current, exists := r.schedules[scheduleID]
	if !exists || current.UserID != userID {
		return nil, nil
	}
	copy := current
	return &copy, nil
}

func (r *scheduleRepo) Save(_ context.Context, schedule PrintSchedule) error {
	if r.onSave != nil {
		r.onSave(&schedule)
	}
	r.schedules[schedule.ID] = schedule
	return nil
}

func (r *scheduleRepo) Delete(_ context.Context, userID string, scheduleID string) error {
	current, exists := r.schedules[scheduleID]
	if exists && current.UserID == userID {
		delete(r.schedules, scheduleID)
	}
	return nil
}

func (r *scheduleRepo) ClaimDue(_ context.Context, _ time.Time, _ time.Time, limit int) ([]PrintSchedule, error) {
	if limit > 0 && len(r.claimed) > limit {
		return append([]PrintSchedule{}, r.claimed[:limit]...), nil
	}
	return append([]PrintSchedule{}, r.claimed...), nil
}

type fakeScheduleAuth struct{}

func (fakeScheduleAuth) GetCurrentUser(_ context.Context, accessToken string) (auth.UserDTO, error) {
	return auth.UserDTO{
		ID:    "member-user",
		Email: accessToken,
		Name:  "Member",
		Role:  "member",
	}, nil
}

type fakePluginRuntime struct{}

func (fakePluginRuntime) GetInstallation(_ context.Context, installationID string) (plugins.Installation, plugins.Manifest, error) {
	return plugins.Installation{
			ID:          installationID,
			DisplayName: "Fixture Source",
			Status:      plugins.InstallationStatusReady,
		}, plugins.Manifest{
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
		}, nil
}

func (fakePluginRuntime) GetBindingForUser(_ context.Context, installationID string, userID string) (plugins.Binding, map[string]string, error) {
	return plugins.Binding{
		ID:                   "binding-1",
		PluginInstallationID: installationID,
		UserID:               userID,
		Enabled:              true,
		Status:               plugins.BindingStatusConnected,
	}, map[string]string{}, nil
}

type dispatchCall struct {
	input dispatch.ScheduleRunInput
}

type fakeDispatcher struct {
	calls  []dispatchCall
	result dispatch.ScheduleRunResult
	err    error
}

func (d *fakeDispatcher) RunSchedule(_ context.Context, input dispatch.ScheduleRunInput) (dispatch.ScheduleRunResult, error) {
	d.calls = append(d.calls, dispatchCall{input: input})
	return d.result, d.err
}

type fakePrinterRepo struct {
	status  workspace.DeviceStatus
	missing bool
}

func (r fakePrinterRepo) FindBindingByID(_ context.Context, userID string, bindingID string) (*printer.Binding, error) {
	if r.missing {
		return nil, nil
	}
	status := r.status
	if status == "" {
		status = workspace.DeviceStatusConnected
	}
	return &printer.Binding{
		ID:        bindingID,
		UserID:    userID,
		Name:      "Desk Printer",
		Status:    status,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

type fakeScheduleIDs struct{}

func (fakeScheduleIDs) New(prefix string) (string, error) {
	return prefix + "-1", nil
}

type fakeScheduleClock struct {
	now time.Time
}

func (c fakeScheduleClock) Now() time.Time {
	return c.now
}

func newScheduleService(now time.Time, repo *scheduleRepo, dispatcher *fakeDispatcher) *Service {
	return NewService(
		repo,
		fakeScheduleAuth{},
		fakePluginRuntime{},
		fakePrinterRepo{},
		dispatcher,
		fakeScheduleIDs{},
		fakeScheduleClock{now: now},
	)
}

func assertDispatchInput(t *testing.T, input dispatch.ScheduleRunInput, scheduleID string, deviceID string, batchSize int) {
	t.Helper()
	if input.ScheduleID != scheduleID {
		t.Fatalf("unexpected schedule id: %s", input.ScheduleID)
	}
	if input.Binding.ID != "binding-1" {
		t.Fatalf("unexpected binding id: %s", input.Binding.ID)
	}
	if input.DeviceID != deviceID {
		t.Fatalf("unexpected device id: %s", input.DeviceID)
	}
	if input.BatchSize != batchSize {
		t.Fatalf("expected batch size %d, got %d", batchSize, input.BatchSize)
	}
}

func assertCreatedScheduleDefaults(t *testing.T, created ScheduleView) {
	t.Helper()
	if created.PrintPolicy.BatchSize != 1 {
		t.Fatalf("expected default batch size 1, got %+v", created.PrintPolicy)
	}
	if created.PluginDisplayName != "Fixture Source" {
		t.Fatalf("unexpected plugin display name: %s", created.PluginDisplayName)
	}
}

func assertProcessedScheduleState(t *testing.T, updated PrintSchedule, now time.Time) {
	t.Helper()
	if updated.LastRunAt == nil {
		t.Fatalf("expected last run at to be populated")
	}
	if !updated.NextRunAt.After(now) {
		t.Fatalf("expected next run to move forward, got %s", updated.NextRunAt)
	}
	if updated.LastError != nil {
		t.Fatalf("expected no last error, got %s", *updated.LastError)
	}
}

func TestCreateAndProcessDueScheduleUsesPrintPolicyBatchSize(t *testing.T) {
	t.Parallel()

	repo := newScheduleRepo()
	dispatcher := &fakeDispatcher{}
	now := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	service := newScheduleService(now, repo, dispatcher)

	created, err := service.Create(context.Background(), "member-token", UpsertInput{
		Title:                "Morning Digest",
		PluginInstallationID: "plugin-1",
		FrequencyType:        FrequencyTypeDaily,
		Timezone:             "Asia/Shanghai",
		Hour:                 9,
		Minute:               30,
		DeviceID:             "device-1",
		Enabled:              true,
	})
	if err != nil {
		t.Fatalf("create schedule: %v", err)
	}
	assertCreatedScheduleDefaults(t, created)

	saved := repo.schedules["schedule-1"]
	saved.NextRunAt = now
	repo.schedules[saved.ID] = saved
	repo.claimed = []PrintSchedule{saved}

	processed, err := service.ProcessDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("process due: %v", err)
	}
	if processed != 1 {
		t.Fatalf("expected 1 processed schedule, got %d", processed)
	}
	if len(dispatcher.calls) != 1 {
		t.Fatalf("expected 1 dispatcher call, got %d", len(dispatcher.calls))
	}
	assertDispatchInput(t, dispatcher.calls[0].input, "schedule-1", "device-1", 1)
	assertProcessedScheduleState(t, repo.schedules["schedule-1"], now)
}

func TestToggleRejectsEnablingRemovedPrinter(t *testing.T) {
	for _, device := range []fakePrinterRepo{{status: workspace.DeviceStatusOffline}, {status: workspace.DeviceStatusPending}, {missing: true}} {
		repo := newScheduleRepo()
		repo.schedules["schedule-1"] = PrintSchedule{ID: "schedule-1", UserID: "member-user", DeviceID: "device-1", Enabled: false}
		service := newScheduleService(time.Now(), repo, &fakeDispatcher{})
		service.printerRepo = device
		if _, err := service.Toggle(t.Context(), "member-token", "schedule-1"); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("expected removed printer enable to be rejected, got %v", err)
		}
		if repo.schedules["schedule-1"].Enabled {
			t.Fatal("rejected toggle enabled the schedule")
		}
	}
}

func TestScheduleMutationsReturnPersistedDisabledState(t *testing.T) {
	for _, operation := range []string{"create", "update", "toggle"} {
		t.Run(operation, func(t *testing.T) {
			repo := newScheduleRepo()
			repo.schedules["schedule-1"] = PrintSchedule{ID: "schedule-1", UserID: "member-user", PluginInstallationID: "plugin-1", DeviceID: "device-1", FrequencyType: FrequencyTypeDaily, Timezone: "UTC", Enabled: false}
			// Model device removal between service validation and repository save.
			repo.onSave = func(current *PrintSchedule) { current.Enabled = false }
			service := newScheduleService(time.Now(), repo, &fakeDispatcher{})
			input := UpsertInput{Title: "Morning", PluginInstallationID: "plugin-1", DeviceID: "device-1", FrequencyType: FrequencyTypeDaily, Timezone: "UTC", Enabled: true}
			var view ScheduleView
			var err error
			switch operation {
			case "create":
				view, err = service.Create(t.Context(), "member-token", input)
			case "update":
				view, err = service.Update(t.Context(), "member-token", "schedule-1", input)
			case "toggle":
				view, err = service.Toggle(t.Context(), "member-token", "schedule-1")
			}
			if err != nil || view.Enabled || repo.schedules["schedule-1"].Enabled {
				t.Fatalf("response disagreed with persisted state: %+v, %v", view, err)
			}
		})
	}
}

func TestRunNowDispatchesWithoutFetching(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	repo := newScheduleRepo()
	repo.schedules["schedule-1"] = PrintSchedule{
		ID:                   "schedule-1",
		UserID:               "member-user",
		PluginInstallationID: "plugin-1",
		PluginBindingID:      "binding-1",
		Title:                "Lunch Digest",
		FrequencyType:        FrequencyTypeDaily,
		Timezone:             "Asia/Shanghai",
		Hour:                 12,
		Minute:               0,
		PrintPolicy:          PrintPolicy{BatchSize: 3},
		DeviceID:             "device-1",
		Enabled:              true,
		NextRunAt:            now.Add(2 * time.Hour),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	dispatcher := &fakeDispatcher{
		result: dispatch.ScheduleRunResult{
			Printed:     1,
			Failed:      1,
			Skipped:     1,
			PrintJobIDs: []string{"print-job-1"},
		},
	}
	service := newScheduleService(now, repo, dispatcher)

	result, err := service.RunNow(context.Background(), "member-token", "schedule-1")
	if err != nil {
		t.Fatalf("run now: %v", err)
	}
	if result.PrintedCount != 1 || result.FailedCount != 1 || result.SkippedCount != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.PrintJobIDs) != 1 || result.PrintJobIDs[0] != "print-job-1" {
		t.Fatalf("unexpected print job ids: %+v", result.PrintJobIDs)
	}
	if len(dispatcher.calls) != 1 {
		t.Fatalf("expected one dispatch call, got %d", len(dispatcher.calls))
	}
	assertDispatchInput(t, dispatcher.calls[0].input, "schedule-1", "device-1", 3)
}

func TestNextRunAtWeekly(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	next, err := NextRunAt(FrequencyTypeWeekly, "Asia/Shanghai", 9, 30, []int{1, 3, 5}, now)
	if err != nil {
		t.Fatalf("next run at: %v", err)
	}

	location, _ := time.LoadLocation("Asia/Shanghai")
	local := next.In(location)
	if local.Weekday() != time.Friday {
		t.Fatalf("expected Friday run, got %s", local.Weekday())
	}
	if local.Hour() != 9 || local.Minute() != 30 {
		t.Fatalf("expected 09:30 local time, got %02d:%02d", local.Hour(), local.Minute())
	}
}
