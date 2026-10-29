package feedback

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/auth"
	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/user"
	"github.com/ruhuang/ink/server/internal/workspace"
)

func TestSubmitUsesAdminDefaultPrinter(t *testing.T) {
	repo := &fakePrinterRepo{
		bindings: []printer.Binding{
			{ID: "device-member", UserID: "user_admin", Status: workspace.DeviceStatusConnected},
			{ID: "device-backup", UserID: "user_admin", Status: workspace.DeviceStatusConnected},
		},
	}
	jobs := &capturedPrinterJobs{}
	service := NewService(
		fakeAuth{
			user: auth.UserDTO{
				ID:    "user-2",
				Email: "member@example.com",
				Name:  "Member",
				Role:  "member",
			},
		},
		fakeAdminRepo{
			admin: &user.User{
				ID:     "user_admin",
				Email:  "admin",
				Role:   user.RoleAdmin,
				Status: user.StatusActive,
			},
		},
		fakeWorkspaceRepo{
			state: &workspace.State{
				Preferences: workspace.Preferences{
					DefaultDeviceID: "device-member",
				},
			},
		},
		repo,
		jobs,
		fixedClock{now: time.Date(2026, 4, 11, 9, 30, 0, 0, time.UTC)},
	)

	if err := service.Submit(context.Background(), "access-token", SubmitInput{Content: "希望支持快捷反馈"}); err != nil {
		t.Fatalf("submit feedback: %v", err)
	}

	if jobs.userID != "user_admin" {
		t.Fatalf("expected feedback to print for admin user, got %s", jobs.userID)
	}
	if jobs.input.PrinterBindingID != "device-member" {
		t.Fatalf("expected default admin printer, got %s", jobs.input.PrinterBindingID)
	}
	if !jobs.input.SubmitImmediately {
		t.Fatal("expected feedback to submit immediately")
	}
	if !strings.Contains(jobs.input.Content, "Submitted by: Member") {
		t.Fatalf("expected sender name in print content, got %s", jobs.input.Content)
	}
	if !strings.Contains(jobs.input.Content, "希望支持快捷反馈") {
		t.Fatalf("expected feedback body in print content, got %s", jobs.input.Content)
	}
}

func TestSubmitFallsBackToFirstConnectedAdminPrinter(t *testing.T) {
	jobs := &capturedPrinterJobs{}
	service := NewService(
		fakeAuth{user: auth.UserDTO{ID: "user-2", Email: "member@example.com", Name: "Member", Role: "member"}},
		fakeAdminRepo{admin: &user.User{ID: "user_admin", Role: user.RoleAdmin, Status: user.StatusActive}},
		fakeWorkspaceRepo{
			state: &workspace.State{
				Preferences: workspace.Preferences{
					DefaultDeviceID: "device-missing",
				},
			},
		},
		&fakePrinterRepo{
			bindings: []printer.Binding{
				{ID: "offline-device", Status: workspace.DeviceStatusOffline},
				{ID: "connected-device", Status: workspace.DeviceStatusConnected},
			},
		},
		jobs,
		fixedClock{now: time.Now()},
	)

	if err := service.Submit(context.Background(), "access-token", SubmitInput{Content: "fallback"}); err != nil {
		t.Fatalf("submit feedback: %v", err)
	}

	if jobs.input.PrinterBindingID != "connected-device" {
		t.Fatalf("expected first connected admin printer, got %s", jobs.input.PrinterBindingID)
	}
}

func TestSubmitRejectsBlankFeedback(t *testing.T) {
	service := NewService(
		fakeAuth{user: auth.UserDTO{ID: "user-2", Email: "member@example.com", Name: "Member", Role: "member"}},
		fakeAdminRepo{},
		fakeWorkspaceRepo{},
		&fakePrinterRepo{},
		&capturedPrinterJobs{},
		fixedClock{now: time.Now()},
	)

	err := service.Submit(context.Background(), "access-token", SubmitInput{Content: "   "})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input error, got %v", err)
	}
}

func TestSubmitFailsWhenAdminHasNoAvailablePrinter(t *testing.T) {
	service := NewService(
		fakeAuth{user: auth.UserDTO{ID: "user-2", Email: "member@example.com", Name: "Member", Role: "member"}},
		fakeAdminRepo{admin: &user.User{ID: "user_admin", Role: user.RoleAdmin, Status: user.StatusActive}},
		fakeWorkspaceRepo{},
		&fakePrinterRepo{
			bindings: []printer.Binding{
				{ID: "offline-device", Status: workspace.DeviceStatusOffline},
			},
		},
		&capturedPrinterJobs{},
		fixedClock{now: time.Now()},
	)

	err := service.Submit(context.Background(), "access-token", SubmitInput{Content: "hello"})
	if !errors.Is(err, ErrNoAdminDevice) {
		t.Fatalf("expected missing admin device error, got %v", err)
	}
}

type fakeAuth struct {
	user auth.UserDTO
	err  error
}

func (f fakeAuth) GetCurrentUser(_ context.Context, _ string) (auth.UserDTO, error) {
	return f.user, f.err
}

type fakeAdminRepo struct {
	admin *user.User
	err   error
}

func (f fakeAdminRepo) FindPrimaryAdmin(_ context.Context) (*user.User, error) {
	return f.admin, f.err
}

type fakeWorkspaceRepo struct {
	state *workspace.State
	err   error
}

func (f fakeWorkspaceRepo) FindByUserID(_ context.Context, _ string) (*workspace.State, error) {
	return f.state, f.err
}

type fakePrinterRepo struct {
	bindings []printer.Binding
	err      error
}

func (f *fakePrinterRepo) ListBindingsByUserID(_ context.Context, _ string) ([]printer.Binding, error) {
	return f.bindings, f.err
}

type capturedPrinterJobs struct {
	userID string
	input  printer.CreateJobInput
}

func (p *capturedPrinterJobs) CreatePrintJobForUser(_ context.Context, userID string, input printer.CreateJobInput) (workspace.PrintJob, error) {
	p.userID = userID
	p.input = input
	return workspace.PrintJob{
		ID:       "print-feedback",
		Title:    input.Title,
		Source:   input.Source,
		DeviceID: input.PrinterBindingID,
		Status:   workspace.PrintStatusQueued,
		Content:  input.Content,
	}, nil
}

type fixedClock struct {
	now time.Time
}

func (f fixedClock) Now() time.Time {
	return f.now
}
