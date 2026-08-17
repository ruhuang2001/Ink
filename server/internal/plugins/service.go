package plugins

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ruhuang/ink/server/internal/auth"
)

var (
	ErrForbidden       = errors.New("forbidden")
	ErrInvalidInput    = errors.New("invalid plugin input")
	ErrInvalidPlugin   = errors.New("invalid plugin package")
	ErrNotFound        = errors.New("plugin not found")
	ErrExecutionFailed = errors.New("plugin execution failed")
	ErrMissingSecret   = errors.New("plugin encryption secret missing")
	ErrOutputTooLarge  = errors.New("plugin output exceeded size limit")
)

const (
	defaultPluginOutputMaxBytes        int64 = 1 << 20
	defaultPluginFetchMaxItems               = 100
	defaultPluginFetchMaxBlocksPerItem       = 80
	defaultPluginFetchMaxTextBytes           = 32 << 10
	defaultPluginFetchMaxURLBytes            = 2048
)

type ValidationFailure struct {
	Errors []FieldError
}

func (v ValidationFailure) Error() string {
	if len(v.Errors) == 0 {
		return "插件配置校验失败"
	}

	return fmt.Sprintf("%s: %s", v.Errors[0].Field, v.Errors[0].Message)
}

type Repository interface {
	ListInstallations(ctx context.Context) ([]Installation, error)
	FindInstallationByID(ctx context.Context, installationID string) (*Installation, error)
	FindInstallationByPluginKey(ctx context.Context, pluginKey string) (*Installation, error)
	SaveInstallation(ctx context.Context, installation Installation) error
	ListPluginBindingsByUserID(ctx context.Context, userID string) ([]Binding, error)
	FindPluginBindingByInstallationAndUserID(ctx context.Context, installationID string, userID string) (*Binding, error)
	FindPluginBindingByID(ctx context.Context, bindingID string) (*Binding, error)
	ClaimBindingsDueForFetch(ctx context.Context, now time.Time, leaseUntil time.Time, limit int) ([]Binding, error)
	SavePluginBinding(ctx context.Context, binding Binding) error
	UpdatePluginBindingCursor(ctx context.Context, bindingID string, cursor *string, updatedAt time.Time) error
}

type Authenticator interface {
	GetCurrentUser(ctx context.Context, accessToken string) (auth.UserDTO, error)
}

type Encryptor interface {
	Encrypt(plaintext string) ([]byte, []byte, error)
	Decrypt(ciphertext []byte, nonce []byte) (string, error)
}

type IDGenerator interface {
	New(prefix string) (string, error)
}

type Clock interface {
	Now() time.Time
}

type RunOptions struct {
	OutputMaxBytes int64
	EnvAllowlist   []string
}

type Runner interface {
	Run(ctx context.Context, workdir string, command []string, stdin []byte, options RunOptions) ([]byte, []byte, error)
}

type RuntimeLimits struct {
	OutputMaxBytes        int64
	FetchMaxItems         int
	FetchMaxBlocksPerItem int
	FetchMaxTextBytes     int
	FetchMaxURLBytes      int
	EnvAllowlist          []string
}

type Service struct {
	repo            Repository
	auth            Authenticator
	encryptor       Encryptor
	ids             IDGenerator
	clock           Clock
	runner          Runner
	pluginRoot      string
	execTimeout     time.Duration
	installTimeout  time.Duration
	runtimeLimits   RuntimeLimits
	gitCloner       GitCloner
	gitAllowedHosts []string
}

type validationPayload struct {
	WorkspaceConfig map[string]any    `json:"workspaceConfig"`
	Secrets         map[string]string `json:"secrets"`
}

type fetchPayload struct {
	WorkspaceConfig map[string]any    `json:"workspaceConfig"`
	Secrets         map[string]string `json:"secrets"`
	Cursor          *string           `json:"cursor"`
	Trigger         FetchTrigger      `json:"trigger"`
}

type execRunner struct{}

func NewService(
	repo Repository,
	authenticator Authenticator,
	encryptor Encryptor,
	ids IDGenerator,
	clock Clock,
	runner Runner,
	pluginRoot string,
	execTimeout time.Duration,
	installTimeout time.Duration,
	runtimeLimits RuntimeLimits,
	gitCloner GitCloner,
	gitAllowedHosts []string,
) *Service {
	if runner == nil {
		runner = execRunner{}
	}

	hosts := append([]string{}, gitAllowedHosts...)
	limits := normalizeRuntimeLimits(runtimeLimits)

	return &Service{
		repo:            repo,
		auth:            authenticator,
		encryptor:       encryptor,
		ids:             ids,
		clock:           clock,
		runner:          runner,
		pluginRoot:      pluginRoot,
		execTimeout:     execTimeout,
		installTimeout:  installTimeout,
		runtimeLimits:   limits,
		gitCloner:       gitCloner,
		gitAllowedHosts: hosts,
	}
}

func normalizeRuntimeLimits(limits RuntimeLimits) RuntimeLimits {
	if limits.OutputMaxBytes <= 0 {
		limits.OutputMaxBytes = defaultPluginOutputMaxBytes
	}
	if limits.FetchMaxItems <= 0 {
		limits.FetchMaxItems = defaultPluginFetchMaxItems
	}
	if limits.FetchMaxBlocksPerItem <= 0 {
		limits.FetchMaxBlocksPerItem = defaultPluginFetchMaxBlocksPerItem
	}
	if limits.FetchMaxTextBytes <= 0 {
		limits.FetchMaxTextBytes = defaultPluginFetchMaxTextBytes
	}
	if limits.FetchMaxURLBytes <= 0 {
		limits.FetchMaxURLBytes = defaultPluginFetchMaxURLBytes
	}
	limits.EnvAllowlist = append([]string{}, limits.EnvAllowlist...)
	return limits
}

func (execRunner) Run(ctx context.Context, workdir string, command []string, stdin []byte, options RunOptions) ([]byte, []byte, error) {
	if len(command) == 0 {
		return nil, nil, fmt.Errorf("empty command")
	}

	outputLimit := options.OutputMaxBytes
	if outputLimit <= 0 {
		outputLimit = defaultPluginOutputMaxBytes
	}

	tempDir, err := os.MkdirTemp("", "ink-plugin-run-*")
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()
	if err := preparePluginTempDirs(tempDir); err != nil {
		return nil, nil, err
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = workdir
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Env = isolatedPluginEnv(tempDir, options.EnvAllowlist)
	stdout := newLimitedBuffer(outputLimit)
	stderr := newLimitedBuffer(outputLimit)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Run()
	if stdout.Exceeded() || stderr.Exceeded() {
		return stdout.Bytes(), stderr.Bytes(), ErrOutputTooLarge
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

type limitedBuffer struct {
	buffer   bytes.Buffer
	limit    int64
	exceeded bool
}

func newLimitedBuffer(limit int64) *limitedBuffer {
	if limit <= 0 {
		limit = defaultPluginOutputMaxBytes
	}
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := int(b.limit) - b.buffer.Len()
	if remaining <= 0 {
		b.exceeded = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.buffer.Write(p[:remaining])
		b.exceeded = true
		return len(p), nil
	}
	return b.buffer.Write(p)
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buffer.Bytes()
}

func (b *limitedBuffer) Exceeded() bool {
	return b.exceeded
}

func preparePluginTempDirs(tempDir string) error {
	for _, subdir := range []string{
		filepath.Join(tempDir, ".cache"),
		filepath.Join(tempDir, "uv-cache"),
		filepath.Join(tempDir, "npm-cache"),
	} {
		if err := os.MkdirAll(subdir, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func isolatedPluginEnv(tempDir string, allowlist []string) []string {
	env := []string{
		"PATH=" + pluginPathEnv(),
		"HOME=" + tempDir,
		"TMPDIR=" + tempDir,
		"TEMP=" + tempDir,
		"TMP=" + tempDir,
		"XDG_CACHE_HOME=" + filepath.Join(tempDir, ".cache"),
		"UV_CACHE_DIR=" + filepath.Join(tempDir, "uv-cache"),
		"npm_config_cache=" + filepath.Join(tempDir, "npm-cache"),
		"PYTHONUNBUFFERED=1",
		"NO_COLOR=1",
	}

	seen := map[string]struct{}{}
	for _, entry := range env {
		key := entry
		if index := strings.Index(entry, "="); index >= 0 {
			key = entry[:index]
		}
		seen[key] = struct{}{}
	}

	for _, key := range allowlist {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
			seen[key] = struct{}{}
		}
	}
	return env
}

func pluginPathEnv() string {
	if path := strings.TrimSpace(os.Getenv("PATH")); path != "" {
		return path
	}
	return "/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
}

func (s *Service) ListAdminInstallations(ctx context.Context, accessToken string) ([]PluginDetails, error) {
	if err := s.requireAdmin(ctx, accessToken); err != nil {
		return nil, err
	}

	installations, err := s.repo.ListInstallations(ctx)
	if err != nil {
		return nil, err
	}

	sort.Slice(installations, func(i, j int) bool {
		return installations[i].UpdatedAt.After(installations[j].UpdatedAt)
	})

	result := make([]PluginDetails, 0, len(installations))
	for _, installation := range installations {
		result = append(result, s.detailsFromInstallation(installation, nil))
	}

	return result, nil
}

func (s *Service) UploadPlugin(ctx context.Context, accessToken string, filename string, source io.Reader) (PluginDetails, error) {
	currentUser, err := s.requireAdminUser(ctx, accessToken)
	if err != nil {
		return PluginDetails{}, err
	}
	if err := s.ensurePluginRoot(); err != nil {
		return PluginDetails{}, err
	}

	stagingDir, err := os.MkdirTemp(s.pluginRoot, "plugin-upload-*")
	if err != nil {
		return PluginDetails{}, err
	}
	defer func() {
		_ = os.RemoveAll(stagingDir)
	}()

	zipPath := filepath.Join(stagingDir, sanitizedUploadName(filename))
	file, err := os.Create(zipPath)
	if err != nil {
		return PluginDetails{}, err
	}
	if _, err := io.Copy(file, source); err != nil {
		_ = file.Close()
		return PluginDetails{}, err
	}
	if err := file.Close(); err != nil {
		return PluginDetails{}, err
	}

	extractedDir := filepath.Join(stagingDir, "extracted")
	if err := os.MkdirAll(extractedDir, 0o755); err != nil {
		return PluginDetails{}, err
	}

	if err := unzipSecure(zipPath, extractedDir); err != nil {
		return PluginDetails{}, err
	}

	pluginDir, err := resolvePluginDirectory(extractedDir)
	if err != nil {
		return PluginDetails{}, err
	}

	manifestJSON, err := os.ReadFile(filepath.Join(pluginDir, "ink-plugin.json"))
	if err != nil {
		return PluginDetails{}, fmt.Errorf("%w: missing ink-plugin.json", ErrInvalidPlugin)
	}

	manifest, err := ParseManifest(manifestJSON)
	if err != nil {
		return PluginDetails{}, err
	}

	if err := validateRuntimeFiles(pluginDir, manifest); err != nil {
		return PluginDetails{}, err
	}

	if err := s.installPlugin(ctx, pluginDir, manifest); err != nil {
		existing, lookupErr := s.repo.FindInstallationByPluginKey(ctx, manifest.PluginKey)
		if lookupErr == nil && existing == nil {
			now := s.clock.Now()
			installationID, idErr := s.ids.New("plugin")
			if idErr == nil {
				installErr := err.Error()
				_ = s.repo.SaveInstallation(ctx, Installation{
					ID:           installationID,
					PluginKey:    manifest.PluginKey,
					SourceType:   SourceTypeUpload,
					DisplayName:  manifest.Name,
					Version:      manifest.Version,
					RuntimeType:  manifest.Runtime.Type,
					ManifestJSON: manifestJSON,
					Status:       InstallationStatusFailed,
					LastError:    &installErr,
					InstalledBy:  &currentUser.ID,
					CreatedAt:    now,
					UpdatedAt:    now,
				})
			}
		}
		return PluginDetails{}, err
	}

	existing, err := s.repo.FindInstallationByPluginKey(ctx, manifest.PluginKey)
	if err != nil {
		return PluginDetails{}, err
	}

	now := s.clock.Now()
	installationID := ""
	createdAt := now
	if existing != nil {
		installationID = existing.ID
		createdAt = existing.CreatedAt
	} else {
		installationID, err = s.ids.New("plugin")
		if err != nil {
			return PluginDetails{}, err
		}
	}

	finalDir := filepath.Join(s.pluginRoot, "installations", fmt.Sprintf("%s-%d", installationID, now.UnixNano()))
	if err := os.MkdirAll(filepath.Dir(finalDir), 0o755); err != nil {
		return PluginDetails{}, err
	}
	if err := os.Rename(pluginDir, finalDir); err != nil {
		return PluginDetails{}, err
	}

	installation := Installation{
		ID:           installationID,
		PluginKey:    manifest.PluginKey,
		SourceType:   SourceTypeUpload,
		DisplayName:  manifest.Name,
		Version:      manifest.Version,
		RuntimeType:  manifest.Runtime.Type,
		ManifestJSON: manifestJSON,
		CurrentPath:  finalDir,
		Status:       InstallationStatusReady,
		InstalledBy:  &currentUser.ID,
		CreatedAt:    createdAt,
		UpdatedAt:    now,
	}
	if err := s.repo.SaveInstallation(ctx, installation); err != nil {
		return PluginDetails{}, err
	}

	return s.detailsFromInstallation(installation, nil), nil
}

// gitInstallContext bundles the inputs needed once URL/subdir validation has
// passed. It keeps InstallFromGit short enough for Codacy.
type gitInstallContext struct {
	user          string
	normalizedURL string
	ref           string
	subdir        string
	manifest      Manifest
	manifestJSON  []byte
	pluginDir     string
	commitSHA     string
}

// InstallFromGit clones a plugin from a remote git repository, validates and
// installs it using the same pipeline as UploadPlugin. The repository must
// use HTTPS and its host must be present in the configured allowlist.
func (s *Service) InstallFromGit(ctx context.Context, accessToken string, input GitInstallInput) (PluginDetails, error) {
	if s.gitCloner == nil {
		return PluginDetails{}, ErrGitInstallDisabled
	}
	currentUser, err := s.requireAdminUser(ctx, accessToken)
	if err != nil {
		return PluginDetails{}, err
	}
	normalizedURL, subdir, ref, err := s.validateGitInput(input)
	if err != nil {
		return PluginDetails{}, err
	}
	if err := s.ensurePluginRoot(); err != nil {
		return PluginDetails{}, err
	}

	stagingDir, err := os.MkdirTemp(s.pluginRoot, "plugin-git-*")
	if err != nil {
		return PluginDetails{}, err
	}
	defer func() {
		_ = os.RemoveAll(stagingDir)
	}()

	pluginDir, manifest, manifestJSON, commitSHA, err := s.cloneAndParse(ctx, stagingDir, normalizedURL, ref, subdir)
	if err != nil {
		return PluginDetails{}, err
	}

	gc := gitInstallContext{
		user:          currentUser.ID,
		normalizedURL: normalizedURL,
		ref:           ref,
		subdir:        subdir,
		manifest:      manifest,
		manifestJSON:  manifestJSON,
		pluginDir:     pluginDir,
		commitSHA:     commitSHA,
	}
	return s.finalizeGitInstall(ctx, gc)
}

func (s *Service) validateGitInput(input GitInstallInput) (string, string, string, error) {
	normalizedURL, _, err := validateGitURL(input.RepoURL, s.gitAllowedHosts)
	if err != nil {
		return "", "", "", err
	}
	subdir, err := sanitizeSubdir(input.Subdir)
	if err != nil {
		return "", "", "", err
	}
	return normalizedURL, subdir, strings.TrimSpace(input.Ref), nil
}

func (s *Service) cloneAndParse(ctx context.Context, stagingDir, normalizedURL, ref, subdir string) (string, Manifest, []byte, string, error) {
	cloneDir := filepath.Join(stagingDir, "clone")
	cloneCtx, cancel := context.WithTimeout(ctx, s.installTimeout)
	defer cancel()

	commitSHA, err := s.gitCloner.Clone(cloneCtx, normalizedURL, ref, cloneDir)
	if err != nil {
		return "", Manifest{}, nil, "", fmt.Errorf("%w: %s", ErrInvalidPlugin, err.Error())
	}
	pluginDir, err := resolvePluginDirectoryInClone(cloneDir, subdir)
	if err != nil {
		return "", Manifest{}, nil, "", err
	}
	manifestJSON, err := os.ReadFile(filepath.Join(pluginDir, "ink-plugin.json"))
	if err != nil {
		return "", Manifest{}, nil, "", fmt.Errorf("%w: missing ink-plugin.json", ErrInvalidPlugin)
	}
	manifest, err := ParseManifest(manifestJSON)
	if err != nil {
		return "", Manifest{}, nil, "", err
	}
	if err := validateRuntimeFiles(pluginDir, manifest); err != nil {
		return "", Manifest{}, nil, "", err
	}
	return pluginDir, manifest, manifestJSON, commitSHA, nil
}

func (s *Service) finalizeGitInstall(ctx context.Context, gc gitInstallContext) (PluginDetails, error) {
	if err := s.installPlugin(ctx, gc.pluginDir, gc.manifest); err != nil {
		s.recordFailedGitInstall(ctx, gc, err)
		return PluginDetails{}, err
	}
	installationID, createdAt, err := s.resolveInstallationID(ctx, gc.manifest.PluginKey)
	if err != nil {
		return PluginDetails{}, err
	}
	now := s.clock.Now()
	finalDir := filepath.Join(s.pluginRoot, "installations", fmt.Sprintf("%s-%d", installationID, now.UnixNano()))
	if err := os.MkdirAll(filepath.Dir(finalDir), 0o755); err != nil {
		return PluginDetails{}, err
	}
	if err := os.Rename(gc.pluginDir, finalDir); err != nil {
		return PluginDetails{}, err
	}
	installation := Installation{
		ID:            installationID,
		PluginKey:     gc.manifest.PluginKey,
		SourceType:    SourceTypeGit,
		DisplayName:   gc.manifest.Name,
		Version:       gc.manifest.Version,
		RuntimeType:   gc.manifest.Runtime.Type,
		ManifestJSON:  gc.manifestJSON,
		CurrentPath:   finalDir,
		Status:        InstallationStatusReady,
		InstalledBy:   &gc.user,
		RepoURL:       gc.normalizedURL,
		RepoRef:       gc.ref,
		RepoCommitSHA: gc.commitSHA,
		RepoSubdir:    gc.subdir,
		CreatedAt:     createdAt,
		UpdatedAt:     now,
	}
	if err := s.repo.SaveInstallation(ctx, installation); err != nil {
		return PluginDetails{}, err
	}
	return s.detailsFromInstallation(installation, nil), nil
}

func (s *Service) resolveInstallationID(ctx context.Context, pluginKey string) (string, time.Time, error) {
	existing, err := s.repo.FindInstallationByPluginKey(ctx, pluginKey)
	if err != nil {
		return "", time.Time{}, err
	}
	now := s.clock.Now()
	if existing != nil {
		return existing.ID, existing.CreatedAt, nil
	}
	id, err := s.ids.New("plugin")
	if err != nil {
		return "", time.Time{}, err
	}
	return id, now, nil
}

func (s *Service) recordFailedGitInstall(ctx context.Context, gc gitInstallContext, installErr error) {
	existing, lookupErr := s.repo.FindInstallationByPluginKey(ctx, gc.manifest.PluginKey)
	if lookupErr != nil || existing != nil {
		return
	}
	installationID, idErr := s.ids.New("plugin")
	if idErr != nil {
		return
	}
	now := s.clock.Now()
	message := installErr.Error()
	_ = s.repo.SaveInstallation(ctx, Installation{
		ID:            installationID,
		PluginKey:     gc.manifest.PluginKey,
		SourceType:    SourceTypeGit,
		DisplayName:   gc.manifest.Name,
		Version:       gc.manifest.Version,
		RuntimeType:   gc.manifest.Runtime.Type,
		ManifestJSON:  gc.manifestJSON,
		Status:        InstallationStatusFailed,
		LastError:     &message,
		InstalledBy:   &gc.user,
		RepoURL:       gc.normalizedURL,
		RepoRef:       gc.ref,
		RepoCommitSHA: gc.commitSHA,
		RepoSubdir:    gc.subdir,
		CreatedAt:     now,
		UpdatedAt:     now,
	})
}

func (s *Service) DisableInstallation(ctx context.Context, accessToken string, installationID string) (PluginDetails, error) {
	if err := s.requireAdmin(ctx, accessToken); err != nil {
		return PluginDetails{}, err
	}

	installation, manifest, err := s.GetInstallation(ctx, installationID)
	if err != nil {
		return PluginDetails{}, err
	}

	installation.Status = InstallationStatusDisabled
	installation.UpdatedAt = s.clock.Now()
	installation.LastError = nil
	if err := s.repo.SaveInstallation(ctx, installation); err != nil {
		return PluginDetails{}, err
	}

	details := s.detailsFromInstallation(installation, nil)
	details.Manifest = manifest
	return details, nil
}

func (s *Service) ListUserPlugins(ctx context.Context, accessToken string) ([]PluginDetails, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	installations, err := s.repo.ListInstallations(ctx)
	if err != nil {
		return nil, err
	}
	bindings, err := s.repo.ListPluginBindingsByUserID(ctx, currentUser.ID)
	if err != nil {
		return nil, err
	}

	bindingByInstallation := map[string]Binding{}
	for _, binding := range bindings {
		bindingByInstallation[binding.PluginInstallationID] = binding
	}

	result := make([]PluginDetails, 0, len(installations))
	for _, installation := range installations {
		if installation.Status == InstallationStatusFailed || installation.Status == InstallationStatusInstalling {
			continue
		}
		binding, hasBinding := bindingByInstallation[installation.ID]
		if hasBinding {
			result = append(result, s.detailsFromInstallation(installation, &binding))
			continue
		}
		result = append(result, s.detailsFromInstallation(installation, nil))
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Installation.DisplayName < result[j].Installation.DisplayName
	})

	return result, nil
}

func (s *Service) GetUserPlugin(ctx context.Context, accessToken string, installationID string) (PluginDetails, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return PluginDetails{}, err
	}

	installation, manifest, err := s.GetInstallation(ctx, installationID)
	if err != nil {
		return PluginDetails{}, err
	}

	binding, err := s.repo.FindPluginBindingByInstallationAndUserID(ctx, installation.ID, currentUser.ID)
	if err != nil {
		return PluginDetails{}, err
	}

	details := s.detailsFromInstallation(installation, binding)
	details.Manifest = manifest
	return details, nil
}

func (s *Service) SaveBinding(ctx context.Context, accessToken string, installationID string, input BindingInput) (PluginDetails, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return PluginDetails{}, err
	}

	installation, manifest, err := s.GetInstallation(ctx, installationID)
	if err != nil {
		return PluginDetails{}, err
	}
	if installation.Status != InstallationStatusReady && input.Enabled {
		return PluginDetails{}, fmt.Errorf("%w: plugin is not ready", ErrInvalidInput)
	}

	existing, err := s.repo.FindPluginBindingByInstallationAndUserID(ctx, installation.ID, currentUser.ID)
	if err != nil {
		return PluginDetails{}, err
	}

	baseConfig := map[string]any{}
	existingSecrets := map[string]string{}
	if existing != nil {
		baseConfig = cloneMap(existing.Config)
		existingSecrets, err = s.decryptSecrets(*existing)
		if err != nil {
			return PluginDetails{}, err
		}
	}
	for key, value := range input.Config {
		baseConfig[key] = value
	}

	normalizedConfig, incomingSecrets, fieldErrs := NormalizeConfigValues(manifest.WorkspaceConfigSchema, baseConfig, true)
	if len(fieldErrs) > 0 {
		return PluginDetails{}, ValidationFailure{Errors: fieldErrs}
	}

	mergedSecrets := mergeSecrets(existingSecrets, input.Secrets, incomingSecrets)
	if input.Enabled {
		validation, err := s.runValidation(ctx, installation, normalizedConfig, mergedSecrets, manifest)
		if err != nil {
			return PluginDetails{}, err
		}
		if !validation.Valid {
			return PluginDetails{}, ValidationFailure{Errors: validation.Errors}
		}
	}

	now := s.clock.Now()
	binding := Binding{
		PluginInstallationID: installation.ID,
		UserID:               currentUser.ID,
		Enabled:              input.Enabled,
		Config:               normalizedConfig,
		Status:               BindingStatusDisconnected,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if existing != nil {
		binding.ID = existing.ID
		binding.CreatedAt = existing.CreatedAt
		binding.Status = existing.Status
		binding.NextFetchAt = existing.NextFetchAt
		binding.LastFetchAt = existing.LastFetchAt
		binding.FetchLeaseUntil = existing.FetchLeaseUntil
		binding.LastFetchError = existing.LastFetchError
	}
	if binding.ID == "" {
		binding.ID, err = s.ids.New("binding")
		if err != nil {
			return PluginDetails{}, err
		}
	}

	if len(mergedSecrets) > 0 {
		if s.encryptor == nil {
			return PluginDetails{}, ErrMissingSecret
		}
		ciphertext, nonce, err := s.encryptSecrets(mergedSecrets)
		if err != nil {
			return PluginDetails{}, err
		}
		binding.Ciphertext = ciphertext
		binding.Nonce = nonce
	}

	if input.Enabled {
		binding.Status = BindingStatusConnected
		binding.LastValidatedAt = &now
		binding.LastError = nil
		binding.NextFetchAt = &now
		binding.FetchLeaseUntil = nil
		binding.LastFetchError = nil
	} else {
		binding.Status = BindingStatusDisconnected
		binding.LastValidatedAt = nil
		binding.LastError = nil
		binding.NextFetchAt = nil
		binding.FetchLeaseUntil = nil
		binding.LastFetchError = nil
	}

	if err := s.repo.SavePluginBinding(ctx, binding); err != nil {
		return PluginDetails{}, err
	}

	details := s.detailsFromInstallation(installation, &binding)
	details.Manifest = manifest
	return details, nil
}

func (s *Service) TestBinding(ctx context.Context, accessToken string, installationID string, input BindingInput) (ValidationResult, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return ValidationResult{}, err
	}

	installation, manifest, err := s.GetInstallation(ctx, installationID)
	if err != nil {
		return ValidationResult{}, err
	}
	if installation.Status != InstallationStatusReady {
		return ValidationResult{}, fmt.Errorf("%w: plugin is not ready", ErrInvalidInput)
	}

	existing, err := s.repo.FindPluginBindingByInstallationAndUserID(ctx, installation.ID, currentUser.ID)
	if err != nil {
		return ValidationResult{}, err
	}

	baseConfig := map[string]any{}
	existingSecrets := map[string]string{}
	if existing != nil {
		baseConfig = cloneMap(existing.Config)
		existingSecrets, err = s.decryptSecrets(*existing)
		if err != nil {
			return ValidationResult{}, err
		}
	}
	for key, value := range input.Config {
		baseConfig[key] = value
	}

	normalizedConfig, incomingSecrets, fieldErrs := NormalizeConfigValues(manifest.WorkspaceConfigSchema, baseConfig, true)
	if len(fieldErrs) > 0 {
		return ValidationResult{
			Valid:  false,
			Errors: fieldErrs,
		}, nil
	}

	mergedSecrets := mergeSecrets(existingSecrets, input.Secrets, incomingSecrets)
	return s.runValidation(ctx, installation, normalizedConfig, mergedSecrets, manifest)
}

func (s *Service) GetInstallation(ctx context.Context, installationID string) (Installation, Manifest, error) {
	installation, err := s.repo.FindInstallationByID(ctx, installationID)
	if err != nil {
		return Installation{}, Manifest{}, err
	}
	if installation == nil {
		return Installation{}, Manifest{}, ErrNotFound
	}

	manifest, err := ParseManifest(installation.ManifestJSON)
	if err != nil {
		return Installation{}, Manifest{}, err
	}

	return *installation, manifest, nil
}

func (s *Service) GetBindingForUser(ctx context.Context, installationID string, userID string) (Binding, map[string]string, error) {
	binding, err := s.repo.FindPluginBindingByInstallationAndUserID(ctx, installationID, userID)
	if err != nil {
		return Binding{}, nil, err
	}
	if binding == nil {
		return Binding{}, nil, ErrNotFound
	}

	secrets, err := s.decryptSecrets(*binding)
	if err != nil {
		return Binding{}, nil, err
	}

	return *binding, secrets, nil
}

// ExecuteFetch invokes the plugin's fetch entrypoint and returns a normalised
// list of items. The caller is responsible for persisting items and the cursor.
func (s *Service) ExecuteFetch(ctx context.Context, installation Installation, binding Binding, secrets map[string]string, trigger FetchTrigger) (FetchOutput, error) {
	manifest, err := ParseManifest(installation.ManifestJSON)
	if err != nil {
		return FetchOutput{}, err
	}

	payload := fetchPayload{
		WorkspaceConfig: cloneMap(binding.Config),
		Secrets:         secrets,
		Cursor:          binding.Cursor,
		Trigger:         trigger,
	}
	input, err := json.Marshal(payload)
	if err != nil {
		return FetchOutput{}, err
	}

	execCtx, cancel := context.WithTimeout(ctx, s.execTimeout)
	defer cancel()

	stdout, stderr, err := s.runner.Run(execCtx, installation.CurrentPath, manifest.Entrypoints.Fetch.Command, input, s.runOptions())
	if err != nil {
		return FetchOutput{}, fmt.Errorf("%w: %s", ErrExecutionFailed, trimExecOutput(stdout, stderr, err))
	}

	var result FetchOutput
	if err := json.Unmarshal(stdout, &result); err != nil {
		return FetchOutput{}, fmt.Errorf("%w: invalid fetch output", ErrExecutionFailed)
	}

	for index := range result.Items {
		if strings.TrimSpace(result.Items[index].SourceLabel) == "" {
			result.Items[index].SourceLabel = installation.DisplayName
		}
	}
	if err := validateFetchOutputLimits(result, s.runtimeLimits); err != nil {
		return FetchOutput{}, fmt.Errorf("%w: %s", ErrExecutionFailed, err.Error())
	}

	return result, nil
}

// UpdateBindingCursor persists the cursor returned by the last fetch so the
// next invocation can pass it back to the plugin verbatim.
func (s *Service) UpdateBindingCursor(ctx context.Context, bindingID string, cursor *string) error {
	return s.repo.UpdatePluginBindingCursor(ctx, bindingID, cursor, s.clock.Now())
}

func (s *Service) ClaimDueBindings(ctx context.Context, now time.Time, leaseUntil time.Time, limit int) ([]Binding, error) {
	return s.repo.ClaimBindingsDueForFetch(ctx, now, leaseUntil, limit)
}

func (s *Service) RecordFetchSuccess(ctx context.Context, bindingID string, cursor *string, fetchedAt time.Time, nextFetchAt time.Time) error {
	binding, err := s.repo.FindPluginBindingByID(ctx, bindingID)
	if err != nil {
		return err
	}
	if binding == nil {
		return ErrNotFound
	}

	binding.Cursor = cursor
	binding.LastFetchAt = &fetchedAt
	binding.NextFetchAt = &nextFetchAt
	binding.FetchLeaseUntil = nil
	binding.LastFetchError = nil
	binding.UpdatedAt = fetchedAt
	return s.repo.SavePluginBinding(ctx, *binding)
}

func (s *Service) RecordFetchFailure(ctx context.Context, bindingID string, message string, attemptedAt time.Time, nextFetchAt time.Time) error {
	binding, err := s.repo.FindPluginBindingByID(ctx, bindingID)
	if err != nil {
		return err
	}
	if binding == nil {
		return ErrNotFound
	}

	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		trimmed = "plugin fetch failed"
	}
	binding.FetchLeaseUntil = nil
	binding.NextFetchAt = &nextFetchAt
	binding.LastFetchError = &trimmed
	binding.UpdatedAt = attemptedAt
	return s.repo.SavePluginBinding(ctx, *binding)
}

// GetBindingByID loads a binding (plus decrypted secrets) without requiring a
// user access token. It is intended for background flows such as scheduled
// runs where the caller has already authorised access via a schedule record.
func (s *Service) GetBindingByID(ctx context.Context, bindingID string) (Binding, map[string]string, error) {
	binding, err := s.repo.FindPluginBindingByID(ctx, bindingID)
	if err != nil {
		return Binding{}, nil, err
	}
	if binding == nil {
		return Binding{}, nil, ErrNotFound
	}

	secrets, err := s.decryptSecrets(*binding)
	if err != nil {
		return Binding{}, nil, err
	}

	return *binding, secrets, nil
}

func (s *Service) runValidation(ctx context.Context, installation Installation, config map[string]any, secrets map[string]string, manifest Manifest) (ValidationResult, error) {
	payload := validationPayload{
		WorkspaceConfig: cloneMap(config),
		Secrets:         secrets,
	}
	input, err := json.Marshal(payload)
	if err != nil {
		return ValidationResult{}, err
	}

	execCtx, cancel := context.WithTimeout(ctx, s.execTimeout)
	defer cancel()

	stdout, stderr, err := s.runner.Run(execCtx, installation.CurrentPath, manifest.Entrypoints.Validate.Command, input, s.runOptions())
	if err != nil {
		return ValidationResult{}, fmt.Errorf("%w: %s", ErrExecutionFailed, trimExecOutput(stdout, stderr, err))
	}

	var result ValidationResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return ValidationResult{}, fmt.Errorf("%w: invalid validation output", ErrExecutionFailed)
	}

	if !result.Valid && len(result.Errors) == 0 {
		result.Errors = []FieldError{{
			Field:   "",
			Message: "插件校验失败",
		}}
	}

	return result, nil
}

func (s *Service) installPlugin(ctx context.Context, pluginDir string, manifest Manifest) error {
	var command []string
	switch manifest.Runtime.Type {
	case "node":
		command = []string{"pnpm", "install", "--frozen-lockfile", "--ignore-scripts"}
		if manifest.Permissions != nil && manifest.Permissions.InstallScripts {
			command = []string{"pnpm", "install", "--frozen-lockfile"}
		}
	case "python":
		command = []string{"uv", "sync", "--frozen"}
	default:
		return fmt.Errorf("%w: unsupported runtime %s", ErrInvalidPlugin, manifest.Runtime.Type)
	}

	installCtx, cancel := context.WithTimeout(ctx, s.installTimeout)
	defer cancel()

	stdout, stderr, err := s.runner.Run(installCtx, pluginDir, command, nil, s.runOptions())
	if err != nil {
		return fmt.Errorf("%w: %s", ErrExecutionFailed, trimExecOutput(stdout, stderr, err))
	}

	return nil
}

func (s *Service) runOptions() RunOptions {
	return RunOptions{
		OutputMaxBytes: s.runtimeLimits.OutputMaxBytes,
		EnvAllowlist:   s.runtimeLimits.EnvAllowlist,
	}
}

func validateFetchOutputLimits(output FetchOutput, limits RuntimeLimits) error {
	limits = normalizeRuntimeLimits(limits)
	if len(output.Items) > limits.FetchMaxItems {
		return fmt.Errorf("fetch returned %d items, limit is %d", len(output.Items), limits.FetchMaxItems)
	}

	for itemIndex, item := range output.Items {
		if err := validateItemLimits(item, itemIndex, limits); err != nil {
			return err
		}
	}
	return nil
}

func validateItemLimits(item Item, itemIndex int, limits RuntimeLimits) error {
	if len(item.ExternalID) > limits.FetchMaxTextBytes {
		return fmt.Errorf("items[%d].externalId exceeds %d bytes", itemIndex, limits.FetchMaxTextBytes)
	}
	if len(item.Title) > limits.FetchMaxTextBytes {
		return fmt.Errorf("items[%d].title exceeds %d bytes", itemIndex, limits.FetchMaxTextBytes)
	}
	if len(item.SourceLabel) > limits.FetchMaxTextBytes {
		return fmt.Errorf("items[%d].sourceLabel exceeds %d bytes", itemIndex, limits.FetchMaxTextBytes)
	}
	if len(item.Blocks) > limits.FetchMaxBlocksPerItem {
		return fmt.Errorf(
			"items[%d].blocks has %d blocks, limit is %d",
			itemIndex,
			len(item.Blocks),
			limits.FetchMaxBlocksPerItem,
		)
	}
	for blockIndex, block := range item.Blocks {
		if err := validateBlockLimits(block, itemIndex, blockIndex, limits); err != nil {
			return err
		}
	}
	return nil
}

func validateBlockLimits(block ContentBlock, itemIndex int, blockIndex int, limits RuntimeLimits) error {
	label := fmt.Sprintf("items[%d].blocks[%d]", itemIndex, blockIndex)
	if len(block.Text) > limits.FetchMaxTextBytes {
		return fmt.Errorf("%s.text exceeds %d bytes", label, limits.FetchMaxTextBytes)
	}
	if len(block.Alt) > limits.FetchMaxTextBytes {
		return fmt.Errorf("%s.alt exceeds %d bytes", label, limits.FetchMaxTextBytes)
	}
	if len(block.URL) > limits.FetchMaxURLBytes {
		return fmt.Errorf("%s.url exceeds %d bytes", label, limits.FetchMaxURLBytes)
	}
	return nil
}

func (s *Service) detailsFromInstallation(installation Installation, binding *Binding) PluginDetails {
	manifest, _ := ParseManifest(installation.ManifestJSON)
	details := PluginDetails{
		Installation: InstallationSummary{
			ID:            installation.ID,
			PluginKey:     installation.PluginKey,
			SourceType:    installation.SourceType,
			DisplayName:   installation.DisplayName,
			Version:       installation.Version,
			RuntimeType:   installation.RuntimeType,
			Status:        installation.Status,
			Description:   manifest.Description,
			RepoURL:       installation.RepoURL,
			RepoRef:       installation.RepoRef,
			RepoCommitSHA: installation.RepoCommitSHA,
			RepoSubdir:    installation.RepoSubdir,
			CreatedAt:     &installation.CreatedAt,
			UpdatedAt:     &installation.UpdatedAt,
		},
		Manifest: manifest,
	}
	if installation.LastError != nil {
		details.Installation.LastError = *installation.LastError
	}

	if binding != nil {
		details.Binding = &BindingSummary{
			ID:              binding.ID,
			Enabled:         binding.Enabled,
			Status:          binding.Status,
			Config:          cloneMap(binding.Config),
			LastValidatedAt: binding.LastValidatedAt,
			NextFetchAt:     binding.NextFetchAt,
			LastFetchAt:     binding.LastFetchAt,
		}
		if binding.LastError != nil {
			details.Binding.LastError = *binding.LastError
		}
		if binding.LastFetchError != nil {
			details.Binding.LastFetchError = *binding.LastFetchError
		}
	}

	return details
}

func (s *Service) requireAdmin(ctx context.Context, accessToken string) error {
	_, err := s.requireAdminUser(ctx, accessToken)
	return err
}

func (s *Service) requireAdminUser(ctx context.Context, accessToken string) (auth.UserDTO, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return auth.UserDTO{}, err
	}
	if currentUser.Role != "admin" {
		return auth.UserDTO{}, ErrForbidden
	}
	return currentUser, nil
}

func (s *Service) ensurePluginRoot() error {
	return os.MkdirAll(s.pluginRoot, 0o755)
}

func (s *Service) encryptSecrets(secrets map[string]string) ([]byte, []byte, error) {
	if len(secrets) == 0 {
		return nil, nil, nil
	}
	if s.encryptor == nil {
		return nil, nil, ErrMissingSecret
	}

	payload, err := json.Marshal(secrets)
	if err != nil {
		return nil, nil, err
	}
	return s.encryptor.Encrypt(string(payload))
}

func (s *Service) decryptSecrets(binding Binding) (map[string]string, error) {
	if len(binding.Ciphertext) == 0 {
		return map[string]string{}, nil
	}
	if s.encryptor == nil {
		return nil, ErrMissingSecret
	}

	plaintext, err := s.encryptor.Decrypt(binding.Ciphertext, binding.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decrypt binding secrets: %w", err)
	}

	var secrets map[string]string
	if err := json.Unmarshal([]byte(plaintext), &secrets); err != nil {
		return nil, err
	}
	if secrets == nil {
		secrets = map[string]string{}
	}
	return secrets, nil
}

func mergeSecrets(existing map[string]string, inputs ...map[string]string) map[string]string {
	result := map[string]string{}
	for key, value := range existing {
		result[key] = value
	}
	for _, current := range inputs {
		for key, value := range current {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			result[key] = trimmed
		}
	}
	return result
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func sanitizedUploadName(filename string) string {
	base := filepath.Base(strings.TrimSpace(filename))
	if base == "." || base == "" {
		return "plugin.zip"
	}
	if !strings.HasSuffix(strings.ToLower(base), ".zip") {
		return base + ".zip"
	}
	return base
}

func unzipSecure(zipPath string, destination string) error {
	const maxUncompressedBytes int64 = 64 << 20
	return unzipSecureWithLimit(zipPath, destination, maxUncompressedBytes)
}

func unzipSecureWithLimit(zipPath string, destination string, maxUncompressedBytes int64) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = reader.Close()
	}()

	var totalUncompressed int64
	destination = filepath.Clean(destination)

	for _, file := range reader.File {
		if !filepath.IsLocal(file.Name) {
			return fmt.Errorf("%w: invalid zip entry path", ErrInvalidPlugin)
		}
		cleanName := filepath.Clean(file.Name)
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symbolic links are not allowed", ErrInvalidPlugin)
		}

		targetPath := filepath.Join(destination, cleanName)
		relativePath, err := filepath.Rel(destination, targetPath)
		if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("%w: invalid zip entry path", ErrInvalidPlugin)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		src, err := file.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			_ = src.Close()
			return err
		}

		remaining := maxUncompressedBytes - totalUncompressed
		if remaining < 0 {
			remaining = 0
		}

		written, err := io.Copy(dst, &io.LimitedReader{R: src, N: remaining + 1})
		totalUncompressed += written
		if err != nil {
			_ = dst.Close()
			_ = src.Close()
			return err
		}
		if totalUncompressed > maxUncompressedBytes {
			_ = dst.Close()
			_ = src.Close()
			return fmt.Errorf("%w: plugin archive is too large", ErrInvalidPlugin)
		}
		if err := dst.Close(); err != nil {
			_ = src.Close()
			return err
		}
		if err := src.Close(); err != nil {
			return err
		}
	}

	return nil
}

func resolvePluginDirectory(extractedDir string) (string, error) {
	rootManifest := filepath.Join(extractedDir, "ink-plugin.json")
	if _, err := os.Stat(rootManifest); err == nil {
		return extractedDir, nil
	}

	entries, err := os.ReadDir(extractedDir)
	if err != nil {
		return "", err
	}

	directories := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if entry.Name() == "__MACOSX" {
			continue
		}
		directories = append(directories, filepath.Join(extractedDir, entry.Name()))
	}
	if len(directories) == 1 {
		manifestPath := filepath.Join(directories[0], "ink-plugin.json")
		if _, err := os.Stat(manifestPath); err == nil {
			return directories[0], nil
		}
	}

	return "", fmt.Errorf("%w: plugin archive must contain ink-plugin.json at root or a single top-level directory", ErrInvalidPlugin)
}

func validateRuntimeFiles(pluginDir string, manifest Manifest) error {
	switch manifest.Runtime.Type {
	case "node":
		if !fileExists(filepath.Join(pluginDir, "package.json")) || !fileExists(filepath.Join(pluginDir, "pnpm-lock.yaml")) {
			return fmt.Errorf("%w: node plugins must include package.json and pnpm-lock.yaml", ErrInvalidPlugin)
		}
	case "python":
		if !fileExists(filepath.Join(pluginDir, "pyproject.toml")) || !fileExists(filepath.Join(pluginDir, "uv.lock")) {
			return fmt.Errorf("%w: python plugins must include pyproject.toml and uv.lock", ErrInvalidPlugin)
		}
	default:
		return fmt.Errorf("%w: unsupported runtime %s", ErrInvalidPlugin, manifest.Runtime.Type)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func trimExecOutput(stdout []byte, stderr []byte, runErr error) string {
	parts := []string{}
	if text := strings.TrimSpace(string(stdout)); text != "" {
		parts = append(parts, text)
	}
	if text := strings.TrimSpace(string(stderr)); text != "" {
		parts = append(parts, text)
	}
	if runErr != nil && len(parts) == 0 {
		parts = append(parts, runErr.Error())
	}
	return strings.Join(parts, " | ")
}
