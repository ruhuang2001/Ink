package printer

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/auth"
	"github.com/ruhuang/ink/server/internal/workspace"
)

func TestRenderPreviewReturnsPNG(t *testing.T) {
	service := NewService(nil, nil, nil, nil, "", "", time.Second)
	image, err := service.RenderPreview(context.Background(), "标题", "正文内容")
	if err != nil {
		t.Fatalf("render preview: %v", err)
	}
	pngBytes, err := base64.StdEncoding.DecodeString(image)
	if err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if len(pngBytes) < 8 || string(pngBytes[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("expected PNG signature")
	}
	decoded, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	if decoded.Bounds().Dx() != 384 {
		t.Fatalf("expected preview width 384, got %d", decoded.Bounds().Dx())
	}
}

func TestCancelPrintJobRejectsQueuedJobs(t *testing.T) {
	repo := newFakePrinterRepo()
	repo.jobs["job-1"] = Job{
		ID:               "job-1",
		UserID:           "user-1",
		PrinterBindingID: "device-1",
		Status:           workspace.PrintStatusQueued,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	service := NewService(
		repo,
		fakeAuthenticator{},
		fakeIDGenerator{},
		fakeClock{now: time.Now().UTC()},
		"",
		"",
		time.Second,
	)

	_, err := service.CancelPrintJob(context.Background(), "access-token", "job-1")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected queued job cancellation to be rejected, got %v", err)
	}
}

func TestUpdatePrintJobDeviceRejectsQueuedJobs(t *testing.T) {
	repo := newFakePrinterRepo()
	repo.jobs["job-1"] = Job{
		ID:               "job-1",
		UserID:           "user-1",
		PrinterBindingID: "device-1",
		Status:           workspace.PrintStatusQueued,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	repo.bindings["device-2"] = Binding{
		ID:               "device-2",
		UserID:           "user-1",
		DeviceIdentifier: "m1-2",
		Status:           workspace.DeviceStatusConnected,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	service := NewService(
		repo,
		fakeAuthenticator{},
		fakeIDGenerator{},
		fakeClock{now: time.Now().UTC()},
		"",
		"",
		time.Second,
	)

	_, err := service.UpdatePrintJobDevice(context.Background(), "access-token", "job-1", UpdateJobDeviceInput{
		PrinterBindingID: "device-2",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected queued job rebind to be rejected, got %v", err)
	}
}

func TestDeleteDeviceRemovesBinding(t *testing.T) {
	now := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)
	repo := newFakePrinterRepo()
	repo.bindings["device-1"] = Binding{
		ID:               "device-1",
		UserID:           "user-1",
		Name:             "书桌咕咕机",
		DeviceIdentifier: "m1-1",
		Status:           workspace.DeviceStatusConnected,
		CreatedAt:        now.Add(-time.Hour),
		UpdatedAt:        now.Add(-time.Hour),
	}

	service := NewService(
		repo,
		fakeAuthenticator{},
		fakeIDGenerator{},
		fakeClock{now: now},
		"",
		"",
		time.Second,
	)

	if err := service.DeleteDevice(context.Background(), "access-token", "device-1"); err != nil {
		t.Fatalf("delete device failed: %v", err)
	}

	if _, exists := repo.bindings["device-1"]; exists {
		t.Fatalf("expected device binding to be removed")
	}
}

func TestListDevicesOmitsOfflineBindings(t *testing.T) {
	repo := newFakePrinterRepo()
	repo.bindings["device-1"] = Binding{
		ID:               "device-1",
		UserID:           "user-1",
		Name:             "书桌咕咕机",
		DeviceIdentifier: "m1-1",
		Status:           workspace.DeviceStatusConnected,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	repo.bindings["device-2"] = Binding{
		ID:               "device-2",
		UserID:           "user-1",
		Name:             "旧设备",
		DeviceIdentifier: "m1-2",
		Status:           workspace.DeviceStatusOffline,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	service := NewService(
		repo,
		fakeAuthenticator{},
		fakeIDGenerator{},
		fakeClock{now: time.Now().UTC()},
		"",
		"",
		time.Second,
	)

	devices, err := service.ListDevices(context.Background(), "access-token")
	if err != nil {
		t.Fatalf("list devices failed: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected only active devices, got %d", len(devices))
	}
	if devices[0].ID != "device-1" {
		t.Fatalf("unexpected device list: %+v", devices)
	}
}

func TestCreatePrintJobRejectsOfflineBindings(t *testing.T) {
	repo := newFakePrinterRepo()
	repo.bindings["device-1"] = Binding{
		ID:               "device-1",
		UserID:           "user-1",
		DeviceIdentifier: "m1-1",
		Status:           workspace.DeviceStatusOffline,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	service := NewService(
		repo,
		fakeAuthenticator{},
		fakeIDGenerator{},
		fakeClock{now: time.Now().UTC()},
		"",
		"",
		time.Second,
	)

	_, err := service.CreatePrintJob(context.Background(), "access-token", CreateJobInput{
		Title:            "测试",
		Source:           "手动打印",
		Content:          "内容",
		PrinterBindingID: "device-1",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected offline binding to be rejected, got %v", err)
	}
}

func TestCreatePrintJobSubmitImmediatelyRejectsUnconfiguredService(t *testing.T) {
	repo := newFakePrinterRepo()
	repo.bindings["device-1"] = Binding{
		ID:               "device-1",
		UserID:           "user-1",
		DeviceIdentifier: "m1-1",
		Status:           workspace.DeviceStatusConnected,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	service := NewService(
		repo,
		fakeAuthenticator{},
		fakeIDGenerator{},
		fakeClock{now: time.Now().UTC()},
		"",
		"",
		time.Second,
	)

	_, err := service.CreatePrintJob(context.Background(), "access-token", CreateJobInput{
		Title:             "测试",
		Source:            "手动打印",
		Content:           "内容",
		PrinterBindingID:  "device-1",
		SubmitImmediately: true,
	})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected unconfigured immediate submit to be rejected, got %v", err)
	}
	if len(repo.jobs) != 0 {
		t.Fatalf("expected no print job to be persisted when service is unconfigured")
	}
}

func TestSubmitPrintJobRejectsNonPendingJobs(t *testing.T) {
	now := time.Now().UTC()
	repo := newFakePrinterRepo()
	repo.jobs["job-1"] = Job{
		ID:               "job-1",
		UserID:           "user-1",
		PrinterBindingID: "device-1",
		Status:           workspace.PrintStatusQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	repo.bindings["device-1"] = Binding{
		ID:               "device-1",
		UserID:           "user-1",
		DeviceIdentifier: "m1-1",
		Status:           workspace.DeviceStatusConnected,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	service := NewService(
		repo,
		fakeAuthenticator{},
		fakeIDGenerator{},
		fakeClock{now: now},
		"access-key",
		"",
		time.Second,
	)

	_, err := service.SubmitPrintJob(context.Background(), "access-token", "job-1")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected non-pending job submission to be rejected, got %v", err)
	}
	if repo.jobs["job-1"].Status != workspace.PrintStatusQueued {
		t.Fatalf("expected rejected submission to leave job status unchanged, got %s", repo.jobs["job-1"].Status)
	}
}

func TestRenderPrintableTextKeepsTitleAndBodyStructure(t *testing.T) {
	rendered := renderPrintableText("  今日提醒  ", "  第一行\n第二行  ")

	if rendered != "今日提醒\n\n第一行\n第二行" {
		t.Fatalf("renderPrintableText() = %q", rendered)
	}
}

func TestRenderPrintableTextNormalizesEmojiAndListMarkers(t *testing.T) {
	rendered := renderPrintableText("  手动打印  ", "你好！很高兴见到你！😊\n\n- 回答问题\n• 协助写作\n\t编程支持")

	if rendered != "手动打印\n\n你好！很高兴见到你！\n\n回答问题\n协助写作\n编程支持" {
		t.Fatalf("renderPrintableText() = %q", rendered)
	}
}

func TestPrinterFontDataIsAvailable(t *testing.T) {
	if len(printerFontData) == 0 {
		t.Fatal("expected embedded printer font data")
	}
	if !strings.HasPrefix(string(printerFontData[:4]), "OTTO") && !strings.HasPrefix(string(printerFontData[:4]), "\x00\x01\x00\x00") {
		t.Fatalf("unexpected font header: %q", string(printerFontData[:4]))
	}
}

type fakePrinterRepo struct {
	bindings map[string]Binding
	jobs     map[string]Job
}

func TestCreatePrintJobForUserReusesInternalJobID(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	repo := newFakePrinterRepo()
	repo.bindings["device-1"] = Binding{ID: "device-1", UserID: "user-1", Status: workspace.DeviceStatusConnected}
	service := NewService(repo, fakeAuthenticator{}, fakeIDGenerator{}, fakeClock{now: now}, "", "", time.Second)
	input := CreateJobInput{JobID: "delivery-stable", Title: "Scheduled", Content: "content", PrinterBindingID: "device-1"}

	first, err := service.CreatePrintJobForUser(context.Background(), "user-1", input)
	if err != nil {
		t.Fatalf("create first job: %v", err)
	}
	second, err := service.CreatePrintJobForUser(context.Background(), "user-1", input)
	if err != nil {
		t.Fatalf("create repeated job: %v", err)
	}
	if first.ID != input.JobID || second.ID != first.ID || len(repo.jobs) != 1 {
		t.Fatalf("expected one stable job, first=%+v second=%+v jobs=%d", first, second, len(repo.jobs))
	}
}

func newFakePrinterRepo() *fakePrinterRepo {
	return &fakePrinterRepo{
		bindings: map[string]Binding{},
		jobs:     map[string]Job{},
	}
}

func (f *fakePrinterRepo) ListBindingsByUserID(_ context.Context, userID string) ([]Binding, error) {
	bindings := make([]Binding, 0, len(f.bindings))
	for _, binding := range f.bindings {
		if binding.UserID == userID {
			bindings = append(bindings, binding)
		}
	}
	return bindings, nil
}

func (f *fakePrinterRepo) FindBindingByID(_ context.Context, userID string, bindingID string) (*Binding, error) {
	binding, ok := f.bindings[bindingID]
	if !ok || binding.UserID != userID {
		return nil, nil
	}

	copy := binding
	return &copy, nil
}

func (f *fakePrinterRepo) SaveBinding(_ context.Context, binding Binding) error {
	f.bindings[binding.ID] = binding
	return nil
}

func (f *fakePrinterRepo) DeleteBinding(_ context.Context, userID string, bindingID string) error {
	binding, ok := f.bindings[bindingID]
	if ok && binding.UserID == userID {
		delete(f.bindings, bindingID)
	}
	return nil
}

func (f *fakePrinterRepo) ListJobsByUserID(_ context.Context, userID string) ([]Job, error) {
	jobs := make([]Job, 0, len(f.jobs))
	for _, job := range f.jobs {
		if job.UserID == userID {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func (f *fakePrinterRepo) FindJobByID(_ context.Context, userID string, jobID string) (*Job, error) {
	job, ok := f.jobs[jobID]
	if !ok || job.UserID != userID {
		return nil, nil
	}

	copy := job
	return &copy, nil
}

func (f *fakePrinterRepo) SaveJob(_ context.Context, job Job) error {
	f.jobs[job.ID] = job
	return nil
}

type fakeAuthenticator struct{}

func (fakeAuthenticator) GetCurrentUser(_ context.Context, _ string) (auth.UserDTO, error) {
	return auth.UserDTO{
		ID:    "user-1",
		Email: "name@example.com",
		Name:  "Ink User",
		Role:  "member",
	}, nil
}

type fakeIDGenerator struct{}

func (fakeIDGenerator) New(prefix string) (string, error) {
	return prefix + "-generated", nil
}

type fakeClock struct {
	now time.Time
}

func (f fakeClock) Now() time.Time {
	return f.now
}
