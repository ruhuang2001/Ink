package plugins

import (
	"archive/zip"
	"cmp"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/auth"
)

type memoryRepo struct {
	mu            sync.Mutex
	installations map[string]Installation
	bindings      map[string]Binding
	saveErr       error
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		installations: map[string]Installation{},
		bindings:      map[string]Binding{},
	}
}

func (r *memoryRepo) ListInstallations(_ context.Context) ([]Installation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]Installation, 0, len(r.installations))
	for _, installation := range r.installations {
		result = append(result, installation)
	}
	return result, nil
}

func (r *memoryRepo) FindInstallationByID(_ context.Context, installationID string) (*Installation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	installation, exists := r.installations[installationID]
	if !exists {
		return nil, nil
	}
	copy := installation
	return &copy, nil
}

func (r *memoryRepo) FindInstallationByPluginKey(_ context.Context, pluginKey string) (*Installation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, installation := range r.installations {
		if installation.PluginKey == pluginKey {
			copy := installation
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *memoryRepo) SaveInstallation(_ context.Context, installation Installation) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.saveErr != nil {
		return r.saveErr
	}
	r.installations[installation.ID] = installation
	return nil
}

func (r *memoryRepo) ListPluginBindingsByUserID(_ context.Context, userID string) ([]Binding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]Binding, 0, len(r.bindings))
	for _, binding := range r.bindings {
		if binding.UserID == userID {
			result = append(result, binding)
		}
	}
	return result, nil
}

func (r *memoryRepo) FindPluginBindingByInstallationAndUserID(_ context.Context, installationID string, userID string) (*Binding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, binding := range r.bindings {
		if binding.PluginInstallationID == installationID && binding.UserID == userID {
			copy := binding
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *memoryRepo) FindPluginBindingByID(_ context.Context, bindingID string) (*Binding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	binding, exists := r.bindings[bindingID]
	if !exists {
		return nil, nil
	}
	copy := binding
	return &copy, nil
}

func (r *memoryRepo) ClaimBindingsDueForFetch(_ context.Context, now time.Time, leaseUntil time.Time, limit int) ([]Binding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := []Binding{}
	for _, binding := range r.bindings {
		if !bindingDueForFetch(binding, now) {
			continue
		}
		binding.FetchLeaseUntil = &leaseUntil
		r.bindings[binding.ID] = binding
		result = append(result, binding)
	}
	slices.SortFunc(result, func(a, b Binding) int {
		switch {
		case a.NextFetchAt.Before(*b.NextFetchAt):
			return -1
		case a.NextFetchAt.After(*b.NextFetchAt):
			return 1
		default:
			return cmp.Compare(a.ID, b.ID)
		}
	})
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func bindingDueForFetch(binding Binding, now time.Time) bool {
	if !binding.Enabled || binding.Status != BindingStatusConnected || binding.NextFetchAt == nil {
		return false
	}
	if binding.FetchLeaseUntil != nil && binding.FetchLeaseUntil.After(now) {
		return false
	}
	return !binding.NextFetchAt.After(now)
}

func (r *memoryRepo) SavePluginBinding(_ context.Context, binding Binding) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.bindings[binding.ID] = binding
	return nil
}

func (r *memoryRepo) UpdatePluginBindingCursor(_ context.Context, bindingID string, cursor *string, updatedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	binding, exists := r.bindings[bindingID]
	if !exists {
		return nil
	}
	binding.Cursor = cursor
	binding.UpdatedAt = updatedAt
	r.bindings[bindingID] = binding
	return nil
}

type fakeAuthenticator struct{}

func (fakeAuthenticator) GetCurrentUser(_ context.Context, accessToken string) (auth.UserDTO, error) {
	switch accessToken {
	case "admin-token":
		return auth.UserDTO{
			ID:    "admin-user",
			Email: "admin",
			Name:  "Administrator",
			Role:  "admin",
		}, nil
	case "member-token":
		return auth.UserDTO{
			ID:    "member-user",
			Email: "member",
			Name:  "Member",
			Role:  "member",
		}, nil
	default:
		return auth.UserDTO{}, errors.New("invalid token")
	}
}

type fakeEncryptor struct{}

func (fakeEncryptor) Encrypt(plaintext string) ([]byte, []byte, error) {
	return []byte(plaintext), []byte("nonce"), nil
}

func (fakeEncryptor) Decrypt(ciphertext []byte, _ []byte) (string, error) {
	return string(ciphertext), nil
}

type fakeIDGenerator struct {
	mu      sync.Mutex
	counter int
}

func (g *fakeIDGenerator) New(prefix string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.counter++
	return prefix + "-" + time.Now().Format("150405") + "-" + string(rune('a'+g.counter)), nil
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time {
	return c.now
}

type installPassthroughRunner struct{}

func (installPassthroughRunner) Run(ctx context.Context, workdir string, command []string, stdin []byte, options RunOptions) ([]byte, []byte, error) {
	if len(command) >= 2 {
		if command[0] == "uv" && strings.Join(command[1:], " ") == "sync --frozen" {
			return nil, nil, nil
		}
		if command[0] == "pnpm" && (strings.Join(command[1:], " ") == "install --frozen-lockfile" || strings.Join(command[1:], " ") == "install --frozen-lockfile --ignore-scripts") {
			return nil, nil, nil
		}
	}

	return execRunner{}.Run(ctx, workdir, command, stdin, options)
}

type commandCaptureRunner struct {
	commands [][]string
}

func (r *commandCaptureRunner) Run(_ context.Context, _ string, command []string, _ []byte, _ RunOptions) ([]byte, []byte, error) {
	r.commands = append(r.commands, append([]string{}, command...))
	return nil, nil, nil
}

func TestInstallPluginDependencyCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		manifest    Manifest
		wantCommand []string
	}{
		{
			name:        "node permissions omitted ignores scripts",
			manifest:    Manifest{Runtime: RuntimeSpec{Type: "node"}},
			wantCommand: []string{"pnpm", "install", "--frozen-lockfile", "--ignore-scripts"},
		},
		{
			name: "node install scripts false ignores scripts",
			manifest: Manifest{
				Runtime:     RuntimeSpec{Type: "node"},
				Permissions: &PluginPermissions{InstallScripts: false},
			},
			wantCommand: []string{"pnpm", "install", "--frozen-lockfile", "--ignore-scripts"},
		},
		{
			name: "node install scripts true allows scripts",
			manifest: Manifest{
				Runtime:     RuntimeSpec{Type: "node"},
				Permissions: &PluginPermissions{InstallScripts: true},
			},
			wantCommand: []string{"pnpm", "install", "--frozen-lockfile"},
		},
		{
			name:        "python sync may execute package build hooks",
			manifest:    Manifest{Runtime: RuntimeSpec{Type: "python"}},
			wantCommand: []string{"uv", "sync", "--frozen"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runner := &commandCaptureRunner{}
			service := &Service{
				runner:         runner,
				installTimeout: time.Second,
				runtimeLimits:  normalizeRuntimeLimits(RuntimeLimits{}),
			}

			if err := service.installPlugin(context.Background(), t.TempDir(), tt.manifest); err != nil {
				t.Fatalf("installPlugin() error = %v", err)
			}
			if len(runner.commands) != 1 {
				t.Fatalf("captured %d commands, want 1", len(runner.commands))
			}
			if got := strings.Join(runner.commands[0], " "); got != strings.Join(tt.wantCommand, " ") {
				t.Fatalf("command = %q, want %q", got, strings.Join(tt.wantCommand, " "))
			}
		})
	}
}

func TestExecRunnerUsesIsolatedEnvironment(t *testing.T) {
	t.Setenv("JWT_SECRET", "server-secret")
	t.Setenv("INK_PLUGIN_ALLOWED", "visible")
	workdir := t.TempDir()
	runner := execRunner{}

	stdout, stderr, err := runner.Run(
		context.Background(),
		workdir,
		[]string{"sh", "-c", `printf "%s" "$JWT_SECRET"`},
		nil,
		RunOptions{OutputMaxBytes: 1024},
	)
	if err != nil {
		t.Fatalf("run secret check: %v stderr=%s", err, stderr)
	}
	if string(stdout) != "" {
		t.Fatalf("expected server secret to be hidden, got %q", string(stdout))
	}

	stdout, stderr, err = runner.Run(
		context.Background(),
		workdir,
		[]string{"sh", "-c", `printf "%s" "$INK_PLUGIN_ALLOWED"`},
		nil,
		RunOptions{OutputMaxBytes: 1024, EnvAllowlist: []string{"INK_PLUGIN_ALLOWED"}},
	)
	if err != nil {
		t.Fatalf("run allowlist check: %v stderr=%s", err, stderr)
	}
	if string(stdout) != "visible" {
		t.Fatalf("expected allowlisted env to be visible, got %q", string(stdout))
	}
}

func TestExecRunnerEnforcesOutputLimit(t *testing.T) {
	t.Parallel()

	stdout, _, err := execRunner{}.Run(
		context.Background(),
		t.TempDir(),
		[]string{"sh", "-c", `printf "1234567890"`},
		nil,
		RunOptions{OutputMaxBytes: 5},
	)
	if !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("expected output limit error, got %v", err)
	}
	if string(stdout) != "12345" {
		t.Fatalf("expected truncated stdout, got %q", string(stdout))
	}
}

func TestValidateFetchOutputLimitsRejectsOversizedOutput(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		output FetchOutput
		limits RuntimeLimits
	}{
		{
			name:   "too many items",
			output: FetchOutput{Items: []Item{{ExternalID: "1"}, {ExternalID: "2"}}},
			limits: RuntimeLimits{FetchMaxItems: 1},
		},
		{
			name: "too many blocks",
			output: FetchOutput{Items: []Item{{
				ExternalID: "1",
				Title:      "title",
				Blocks: []ContentBlock{
					{Type: BlockParagraph, Text: "one"},
					{Type: BlockParagraph, Text: "two"},
				},
			}}},
			limits: RuntimeLimits{FetchMaxBlocksPerItem: 1},
		},
		{
			name: "text too large",
			output: FetchOutput{Items: []Item{{
				ExternalID: "1",
				Title:      "title",
				Blocks:     []ContentBlock{{Type: BlockParagraph, Text: "abcdef"}},
			}}},
			limits: RuntimeLimits{FetchMaxTextBytes: 3},
		},
		{
			name: "url too large",
			output: FetchOutput{Items: []Item{{
				ExternalID: "1",
				Title:      "title",
				Blocks:     []ContentBlock{{Type: BlockImage, URL: "https://example.com/image.png"}},
			}}},
			limits: RuntimeLimits{FetchMaxURLBytes: 10},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if err := validateFetchOutputLimits(testCase.output, testCase.limits); err == nil {
				t.Fatalf("expected limit error")
			}
		})
	}
}

func TestUploadPluginSaveBindingAndExecuteFetch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := newMemoryRepo()
	pluginRoot := t.TempDir()
	fixtureDir := filepath.Join("..", "..", "testdata", "plugins", "python-hello-plugin")
	zipPath := filepath.Join(t.TempDir(), "python-hello-plugin.zip")
	if err := zipDirectory(fixtureDir, zipPath, true); err != nil {
		t.Fatalf("zip fixture: %v", err)
	}

	service := NewService(
		repo,
		fakeAuthenticator{},
		fakeEncryptor{},
		&fakeIDGenerator{},
		fakeClock{now: time.Date(2026, 4, 10, 2, 0, 0, 0, time.UTC)},
		installPassthroughRunner{},
		pluginRoot,
		5*time.Second,
		30*time.Second,
		RuntimeLimits{},
		nil,
		nil,
	)

	file, err := os.Open(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	uploaded, err := service.UploadPlugin(ctx, "admin-token", "python-hello-plugin.zip", file)
	if err != nil {
		t.Fatalf("upload plugin: %v", err)
	}

	if uploaded.Installation.Status != InstallationStatusReady {
		t.Fatalf("expected ready installation, got %s", uploaded.Installation.Status)
	}
	if uploaded.Installation.PluginKey != "python-hello-source" {
		t.Fatalf("unexpected plugin key: %s", uploaded.Installation.PluginKey)
	}

	installation, manifest, err := service.GetInstallation(ctx, uploaded.Installation.ID)
	if err != nil {
		t.Fatalf("get installation: %v", err)
	}
	if manifest.Runtime.Type != "python" {
		t.Fatalf("expected python runtime, got %s", manifest.Runtime.Type)
	}
	if _, err := os.Stat(filepath.Join(installation.CurrentPath, "ink-plugin.json")); err != nil {
		t.Fatalf("expected installed manifest: %v", err)
	}

	saved, err := service.SaveBinding(ctx, "member-token", installation.ID, BindingInput{
		Enabled: true,
		Config: map[string]any{
			"sourceName": "Fixture Source",
			"message":    "Hello plugin",
			"uppercase":  true,
		},
		Secrets: map[string]string{
			"apiToken": "super-secret",
		},
	})
	if err != nil {
		t.Fatalf("save binding: %v", err)
	}
	if saved.Binding == nil || saved.Binding.Status != BindingStatusConnected {
		t.Fatalf("expected connected binding, got %+v", saved.Binding)
	}
	if saved.Binding.NextFetchAt == nil {
		t.Fatalf("expected enabled binding to schedule immediate fetch")
	}

	validation, err := service.TestBinding(ctx, "member-token", installation.ID, BindingInput{
		Enabled: true,
		Config: map[string]any{
			"sourceName": "Fixture Source",
		},
	})
	if err != nil {
		t.Fatalf("test binding: %v", err)
	}
	if !validation.Valid {
		t.Fatalf("expected valid binding, got %+v", validation)
	}

	binding, secrets, err := service.GetBindingForUser(ctx, installation.ID, "member-user")
	if err != nil {
		t.Fatalf("get binding: %v", err)
	}
	if secrets["apiToken"] != "super-secret" {
		t.Fatalf("expected decrypted secret, got %+v", secrets)
	}

	result, err := service.ExecuteFetch(ctx, installation, binding, secrets, FetchTrigger{
		Kind:         TriggerKindAutomatic,
		ScheduledFor: "2026-04-10T10:00:00+08:00",
		TriggeredAt:  "2026-04-10T02:00:00Z",
		Timezone:     "Asia/Shanghai",
	})
	if err != nil {
		t.Fatalf("execute fetch: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result.Items))
	}
	item := result.Items[0]
	if item.Title != "Fixture Source Digest" {
		t.Fatalf("unexpected title: %s", item.Title)
	}
	if item.SourceLabel != "Fixture Source" {
		t.Fatalf("unexpected source label: %s", item.SourceLabel)
	}
	if len(item.Blocks) != 2 {
		t.Fatalf("expected heading + paragraph, got %d blocks", len(item.Blocks))
	}
	paragraphs := 0
	var paragraphText string
	for _, block := range item.Blocks {
		if block.Type == BlockParagraph {
			paragraphs++
			paragraphText = block.Text
		}
	}
	if paragraphs != 1 {
		t.Fatalf("expected 1 paragraph block, got %d", paragraphs)
	}
	if paragraphText != "HELLO PLUGIN" {
		t.Fatalf("expected uppercased message paragraph, got %q", paragraphText)
	}
	if result.Cursor == nil || *result.Cursor != "2026-04-10T02:00:00Z" {
		t.Fatalf("unexpected cursor: %v", result.Cursor)
	}

	disabled, err := service.SaveBinding(ctx, "member-token", installation.ID, BindingInput{
		Enabled: false,
		Config: map[string]any{
			"sourceName": "Fixture Source",
		},
	})
	if err != nil {
		t.Fatalf("disable binding: %v", err)
	}
	if disabled.Binding == nil || disabled.Binding.NextFetchAt != nil {
		t.Fatalf("expected disabled binding to stop automatic fetches, got %+v", disabled.Binding)
	}
}

func TestSaveBindingRejectsUndeclaredSecrets(t *testing.T) {
	repo := newMemoryRepo()
	service := NewService(repo, fakeAuthenticator{}, fakeEncryptor{}, &fakeIDGenerator{}, fakeClock{now: time.Now()}, installPassthroughRunner{}, t.TempDir(), time.Second, time.Second, RuntimeLimits{}, nil, nil)
	installation := Installation{ID: "plugin-1", PluginKey: "demo", Status: InstallationStatusReady, ManifestJSON: []byte(`{"schemaVersion":2,"kind":"source","pluginKey":"demo","name":"Demo","version":"1","runtime":{"type":"node"},"fetchPolicy":{"type":"fixed_interval","minutes":5},"entrypoints":{"validate":{"command":["node","validate.js"]},"fetch":{"command":["node","fetch.js"]}},"workspaceConfigSchema":[{"key":"token","label":"Token","type":"secret"},{"key":"feed","label":"Feed","type":"url"}]}`)}
	repo.installations[installation.ID] = installation

	_, err := service.SaveBinding(context.Background(), "member-token", installation.ID, BindingInput{Secrets: map[string]string{"feed": "not-a-secret", "unknown": "value"}})
	if err == nil {
		t.Fatal("expected undeclared secrets to be rejected")
	}
	var validation ValidationFailure
	if !errors.As(err, &validation) || len(validation.Errors) != 2 {
		t.Fatalf("expected two field validation errors, got %v", err)
	}
}

func TestRemovePublishedPluginDirectoryStaysWithinRoot(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "installations", "old")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := removePublishedPluginDirectory(root, inside); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(inside); !os.IsNotExist(err) {
		t.Fatalf("expected published directory removed, got %v", err)
	}
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := removePublishedPluginDirectory(root, outside); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("expected outside path preserved, got %v", err)
	}
}

func TestUploadPluginRemovesPublishedDirectoryWhenPersistenceFails(t *testing.T) {
	t.Parallel()

	fixtureDir := filepath.Join("..", "..", "testdata", "plugins", "python-hello-plugin")
	zipPath := filepath.Join(t.TempDir(), "python-hello-plugin.zip")
	if err := zipDirectory(fixtureDir, zipPath, true); err != nil {
		t.Fatalf("zip fixture: %v", err)
	}
	file, err := os.Open(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer func() { _ = file.Close() }()

	repo := newMemoryRepo()
	repo.saveErr = errors.New("database unavailable")
	pluginRoot := t.TempDir()
	service := NewService(
		repo,
		fakeAuthenticator{},
		fakeEncryptor{},
		&fakeIDGenerator{},
		fakeClock{now: time.Date(2026, 4, 10, 2, 0, 0, 0, time.UTC)},
		installPassthroughRunner{},
		pluginRoot,
		5*time.Second,
		30*time.Second,
		RuntimeLimits{},
		nil,
		nil,
	)

	if _, err := service.UploadPlugin(context.Background(), "admin-token", "plugin.zip", file); err == nil {
		t.Fatal("expected persistence error")
	}
	entries, err := os.ReadDir(filepath.Join(pluginRoot, "installations"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read installation directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("published directories after persistence failure = %d, want 0", len(entries))
	}
}

func TestUnzipSecureRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	zipPath := filepath.Join(t.TempDir(), "invalid.zip")
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}

	writer := zip.NewWriter(file)
	entry, err := writer.Create("../escape.txt")
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if _, err := entry.Write([]byte("escape")); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}

	err = unzipSecure(zipPath, t.TempDir())
	if !errors.Is(err, ErrInvalidPlugin) {
		t.Fatalf("expected invalid plugin error, got %v", err)
	}
}

func TestUnzipSecureRejectsArchiveExceedingActualLimit(t *testing.T) {
	t.Parallel()

	zipPath := filepath.Join(t.TempDir(), "oversized.zip")
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}

	writer := zip.NewWriter(file)
	entry, err := writer.Create("large.txt")
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if _, err := entry.Write([]byte("0123456789")); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}

	err = unzipSecureWithLimit(zipPath, t.TempDir(), 8)
	if !errors.Is(err, ErrInvalidPlugin) {
		t.Fatalf("expected invalid plugin error, got %v", err)
	}
}

func zipDirectory(sourceDir string, zipPath string, wrapTopLevel bool) error {
	file, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	writer := zip.NewWriter(file)

	basePrefix := ""
	if wrapTopLevel {
		basePrefix = filepath.Base(sourceDir)
	}

	if err := filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == sourceDir {
			return nil
		}

		relativePath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		zipName := filepath.ToSlash(relativePath)
		if basePrefix != "" {
			zipName = filepath.ToSlash(filepath.Join(basePrefix, relativePath))
		}

		if entry.IsDir() {
			_, err := writer.Create(zipName + "/")
			return err
		}

		header, err := zip.FileInfoHeader(mustStat(path))
		if err != nil {
			return err
		}
		header.Name = zipName
		header.Method = zip.Deflate

		target, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}

		source, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() {
			_ = source.Close()
		}()

		_, err = io.Copy(target, source)
		return err
	}); err != nil {
		_ = writer.Close()
		return err
	}

	return writer.Close()
}

func mustStat(path string) os.FileInfo {
	info, err := os.Stat(path)
	if err != nil {
		panic(err)
	}
	return info
}
