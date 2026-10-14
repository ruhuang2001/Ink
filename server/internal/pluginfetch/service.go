package pluginfetch

import (
	"context"
	"fmt"
	"time"

	"github.com/ruhuang/ink/server/internal/auth"
	"github.com/ruhuang/ink/server/internal/inbox"
	"github.com/ruhuang/ink/server/internal/platform/observability"
	"github.com/ruhuang/ink/server/internal/plugins"
)

const fallbackRetryInterval = 15 * time.Minute

type Authenticator interface {
	GetCurrentUser(ctx context.Context, accessToken string) (auth.UserDTO, error)
}

type PluginRuntime interface {
	GetInstallation(ctx context.Context, installationID string) (plugins.Installation, plugins.Manifest, error)
	GetBindingForUser(ctx context.Context, installationID string, userID string) (plugins.Binding, map[string]string, error)
	GetBindingByID(ctx context.Context, bindingID string) (plugins.Binding, map[string]string, error)
	ExecuteFetch(ctx context.Context, installation plugins.Installation, binding plugins.Binding, secrets map[string]string, trigger plugins.FetchTrigger) (plugins.FetchOutput, error)
	ClaimDueBindings(ctx context.Context, now time.Time, leaseUntil time.Time, limit int) ([]plugins.Binding, error)
	RecordFetchSuccess(ctx context.Context, bindingID string, cursor *string, fetchedAt time.Time, nextFetchAt time.Time) error
	RecordFetchFailure(ctx context.Context, bindingID string, message string, attemptedAt time.Time, nextFetchAt time.Time) error
}

type InboxService interface {
	Ingest(ctx context.Context, input inbox.IngestInput) (inbox.IngestResult, error)
}

type Clock interface {
	Now() time.Time
}

type ManualRunResult struct {
	FetchedCount   int      `json:"fetchedCount"`
	IngestedCount  int      `json:"ingestedCount"`
	InboxItemIDs   []string `json:"inboxItemIds"`
	CursorAdvanced bool     `json:"cursorAdvanced"`
}

type resolvedBinding struct {
	installation plugins.Installation
	manifest     plugins.Manifest
	binding      plugins.Binding
	secrets      map[string]string
}

type Service struct {
	Observer *observability.Operation
	auth     Authenticator
	plugins  PluginRuntime
	inbox    InboxService
	clock    Clock
}

func NewService(
	authenticator Authenticator,
	pluginRuntime PluginRuntime,
	inboxService InboxService,
	clock Clock,
) *Service {
	return &Service{
		auth:    authenticator,
		plugins: pluginRuntime,
		inbox:   inboxService,
		clock:   clock,
	}
}

func (s *Service) ProcessDue(ctx context.Context, limit int) (int, error) {
	now := s.clock.Now()
	claimed, err := s.plugins.ClaimDueBindings(ctx, now, now.Add(2*time.Minute), limit)
	if err != nil {
		return 0, err
	}

	for _, binding := range claimed {
		s.processBinding(ctx, binding, now)
	}
	return len(claimed), nil
}

func (s *Service) RunManual(ctx context.Context, accessToken string, installationID string) (result ManualRunResult, runErr error) {
	ctx, finish := s.Observer.Start(ctx)
	defer func() { finish(runErr) }()
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return ManualRunResult{}, err
	}
	resolved, err := s.resolveManualRunBinding(ctx, installationID, currentUser.ID)
	if err != nil {
		return ManualRunResult{}, err
	}
	now := s.clock.Now()
	output, err := s.plugins.ExecuteFetch(ctx, resolved.installation, resolved.binding, resolved.secrets, manualFetchTrigger(now))
	if err != nil {
		nextFetchAt := nextFetchAt(resolved.manifest, now)
		_ = s.plugins.RecordFetchFailure(ctx, resolved.binding.ID, err.Error(), now, nextFetchAt)
		return ManualRunResult{}, err
	}
	ingested, err := s.ingestOutput(ctx, currentUser.ID, resolved.installation, resolved.binding, output)
	if err != nil {
		return ManualRunResult{}, err
	}
	if err := s.plugins.RecordFetchSuccess(ctx, resolved.binding.ID, output.Cursor, now, nextFetchAt(resolved.manifest, now)); err != nil {
		return ManualRunResult{}, err
	}
	return buildManualRunResult(output, ingested), nil
}

func (s *Service) processBinding(ctx context.Context, claimed plugins.Binding, now time.Time) {
	ctx, finish := s.Observer.Start(ctx)
	var runErr error
	defer func() { finish(runErr) }()
	resolved, nextAttempt, err := s.resolveAutomaticRunBinding(ctx, claimed, now)
	if err != nil {
		runErr = err
		_ = s.plugins.RecordFetchFailure(ctx, claimed.ID, err.Error(), now, nextAttempt)
		return
	}
	output, err := s.plugins.ExecuteFetch(
		ctx,
		resolved.installation,
		resolved.binding,
		resolved.secrets,
		automaticFetchTrigger(claimed, now),
	)
	if err != nil {
		runErr = err
		_ = s.plugins.RecordFetchFailure(ctx, claimed.ID, err.Error(), now, nextAttempt)
		return
	}
	if _, err := s.ingestOutput(ctx, resolved.binding.UserID, resolved.installation, resolved.binding, output); err != nil {
		runErr = err
		_ = s.plugins.RecordFetchFailure(ctx, claimed.ID, err.Error(), now, nextAttempt)
		return
	}
	runErr = s.plugins.RecordFetchSuccess(ctx, resolved.binding.ID, output.Cursor, now, nextAttempt)
}

func nextFetchAt(manifest plugins.Manifest, from time.Time) time.Time {
	if manifest.FetchPolicy.Minutes <= 0 {
		return from.Add(fallbackRetryInterval)
	}
	return from.Add(time.Duration(manifest.FetchPolicy.Minutes) * time.Minute)
}

func (s *Service) resolveManualRunBinding(ctx context.Context, installationID string, userID string) (resolvedBinding, error) {
	installation, manifest, err := s.plugins.GetInstallation(ctx, installationID)
	if err != nil {
		return resolvedBinding{}, err
	}
	if installation.Status != plugins.InstallationStatusReady {
		return resolvedBinding{}, fmt.Errorf("%w: plugin is not ready", plugins.ErrInvalidInput)
	}
	binding, secrets, err := s.plugins.GetBindingForUser(ctx, installation.ID, userID)
	if err != nil {
		return resolvedBinding{}, err
	}
	if !binding.Enabled || binding.Status != plugins.BindingStatusConnected {
		return resolvedBinding{}, fmt.Errorf("%w: plugin binding must be enabled", plugins.ErrInvalidInput)
	}
	return resolvedBinding{
		installation: installation,
		manifest:     manifest,
		binding:      binding,
		secrets:      secrets,
	}, nil
}

func (s *Service) resolveAutomaticRunBinding(
	ctx context.Context,
	claimed plugins.Binding,
	now time.Time,
) (resolvedBinding, time.Time, error) {
	nextAttempt := now.Add(fallbackRetryInterval)
	installation, manifest, err := s.plugins.GetInstallation(ctx, claimed.PluginInstallationID)
	if err != nil {
		return resolvedBinding{}, nextAttempt, err
	}
	nextAttempt = nextFetchAt(manifest, now)
	if installation.Status != plugins.InstallationStatusReady {
		return resolvedBinding{}, nextAttempt, fmt.Errorf("插件当前不可用")
	}
	binding, secrets, err := s.plugins.GetBindingByID(ctx, claimed.ID)
	if err != nil {
		return resolvedBinding{}, nextAttempt, err
	}
	if !binding.Enabled || binding.Status != plugins.BindingStatusConnected {
		return resolvedBinding{}, nextAttempt, fmt.Errorf("插件连接未启用")
	}
	return resolvedBinding{
		installation: installation,
		manifest:     manifest,
		binding:      binding,
		secrets:      secrets,
	}, nextAttempt, nil
}

func (s *Service) ingestOutput(
	ctx context.Context,
	userID string,
	installation plugins.Installation,
	binding plugins.Binding,
	output plugins.FetchOutput,
) (inbox.IngestResult, error) {
	return s.inbox.Ingest(ctx, inbox.IngestInput{
		UserID:               userID,
		PluginInstallationID: installation.ID,
		PluginBindingID:      binding.ID,
		SourceLabelFallback:  installation.DisplayName,
		Items:                output.Items,
	})
}

func manualFetchTrigger(now time.Time) plugins.FetchTrigger {
	return plugins.FetchTrigger{
		Kind:        plugins.TriggerKindManual,
		TriggeredAt: now.UTC().Format(time.RFC3339),
		Timezone:    "UTC",
	}
}

func automaticFetchTrigger(claimed plugins.Binding, now time.Time) plugins.FetchTrigger {
	scheduledFor := now.UTC().Format(time.RFC3339)
	if claimed.NextFetchAt != nil {
		scheduledFor = claimed.NextFetchAt.UTC().Format(time.RFC3339)
	}
	return plugins.FetchTrigger{
		Kind:         plugins.TriggerKindAutomatic,
		ScheduledFor: scheduledFor,
		TriggeredAt:  now.UTC().Format(time.RFC3339),
		Timezone:     "UTC",
	}
}

func buildManualRunResult(output plugins.FetchOutput, ingested inbox.IngestResult) ManualRunResult {
	return ManualRunResult{
		FetchedCount:   len(output.Items),
		IngestedCount:  ingested.Inserted,
		InboxItemIDs:   ingested.ItemIDs,
		CursorAdvanced: output.Cursor != nil,
	}
}
