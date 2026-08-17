package plugins

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateGitURL(t *testing.T) {
	t.Parallel()

	allowed := []string{"github.com", "*.gitlab.com"}

	cases := []struct {
		name     string
		url      string
		wantErr  bool
		wantHost string
	}{
		{name: "ok github", url: "https://github.com/owner/repo.git", wantHost: "github.com"},
		{name: "ok github mixed case", url: "https://GitHub.Com/owner/repo.git", wantHost: "github.com"},
		{name: "ok wildcard subdomain", url: "https://source.gitlab.com/owner/repo", wantHost: "source.gitlab.com"},
		{name: "reject http", url: "http://github.com/owner/repo", wantErr: true},
		{name: "reject ssh", url: "git@github.com:owner/repo.git", wantErr: true},
		{name: "reject embedded creds", url: "https://token@github.com/owner/repo", wantErr: true},
		{name: "reject disallowed host", url: "https://evil.internal/owner/repo", wantErr: true},
		{name: "reject empty", url: "   ", wantErr: true},
		{name: "reject bare host suffix", url: "https://gitlab.com/owner/repo", wantErr: true}, // *.gitlab.com requires a sub-label
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			normalized, host, err := validateGitURL(tc.url, allowed)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (normalized=%q host=%q)", normalized, host)
				}
				if !errors.Is(err, ErrInvalidInput) {
					t.Fatalf("expected ErrInvalidInput, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if host != tc.wantHost {
				t.Fatalf("host = %q, want %q", host, tc.wantHost)
			}
			if !strings.HasPrefix(normalized, "https://") {
				t.Fatalf("normalized URL must start with https://, got %q", normalized)
			}
			if strings.Contains(normalized, "GitHub.Com") {
				t.Fatalf("normalized URL must canonicalize host casing, got %q", normalized)
			}
		})
	}
}

func TestValidateGitURLStripsQuery(t *testing.T) {
	t.Parallel()

	allowed := []string{"github.com"}
	normalized, _, err := validateGitURL("https://github.com/owner/repo?token=abc#readme", allowed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(normalized, "token") || strings.Contains(normalized, "#") {
		t.Fatalf("normalized URL must strip query/fragment, got %q", normalized)
	}
}

func TestSanitizeSubdir(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty", input: "   ", want: ""},
		{name: "relative", input: "plugins/hello", want: filepath.Join("plugins", "hello")},
		{name: "cleaned", input: "./plugins/./hello/", want: filepath.Join("plugins", "hello")},
		{name: "absolute", input: "/etc/passwd", wantErr: true},
		{name: "escape", input: "../../../etc/passwd", wantErr: true},
		{name: "escape dotdot", input: "..", wantErr: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := sanitizeSubdir(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (got=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

// stubGitCloner copies a fixture directory into destDir so tests can exercise
// the full install pipeline without hitting the network. If simulateErr is
// non-nil it is returned instead of performing the copy.
type stubGitCloner struct {
	fixtureDir  string
	commitSHA   string
	simulateErr error
	lastURL     string
	lastRef     string
}

func (s *stubGitCloner) Clone(_ context.Context, repoURL, ref, destDir string) (string, error) {
	s.lastURL = repoURL
	s.lastRef = ref
	if s.simulateErr != nil {
		return "", s.simulateErr
	}
	if err := copyTree(s.fixtureDir, destDir); err != nil {
		return "", err
	}
	return s.commitSHA, nil
}

func copyTree(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(srcPath, dstPath string) (err error) {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := in.Close(); err == nil {
			err = closeErr
		}
	}()
	out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := out.Close(); err == nil {
			err = closeErr
		}
	}()
	_, err = io.Copy(out, in)
	return err
}

func newGitTestService(t *testing.T, cloner GitCloner, hosts []string) (*Service, *memoryRepo, string) {
	t.Helper()
	repo := newMemoryRepo()
	pluginRoot := t.TempDir()
	service := NewService(
		repo,
		fakeAuthenticator{},
		fakeEncryptor{},
		&fakeIDGenerator{},
		fakeClock{now: time.Date(2026, 4, 20, 1, 0, 0, 0, time.UTC)},
		passthroughRunner{},
		pluginRoot,
		5*time.Second,
		30*time.Second,
		RuntimeLimits{},
		cloner,
		hosts,
	)
	return service, repo, pluginRoot
}

// passthroughRunner recognizes dependency install commands without requiring
// Node/Python tooling, including Node's default no-lifecycle-scripts mode.
type passthroughRunner struct{}

func (passthroughRunner) Run(_ context.Context, _ string, command []string, _ []byte, _ RunOptions) ([]byte, []byte, error) {
	joined := strings.Join(command, " ")
	if joined == "uv sync --frozen" || joined == "pnpm install --frozen-lockfile" || joined == "pnpm install --frozen-lockfile --ignore-scripts" {
		return nil, nil, nil
	}
	return nil, nil, fmt.Errorf("unexpected test command: %s", joined)
}

func assertInstallFromGitHappyPath(t *testing.T, cloner *stubGitCloner, details PluginDetails) {
	t.Helper()
	if cloner.lastURL == "" {
		t.Fatal("expected cloner.Clone to be invoked")
	}
	if cloner.lastRef != "main" {
		t.Fatalf("cloner received ref %q, want main", cloner.lastRef)
	}
	if details.Installation.SourceType != SourceTypeGit {
		t.Fatalf("SourceType = %q, want git", details.Installation.SourceType)
	}
	if details.Installation.RepoCommitSHA != "deadbeef" {
		t.Fatalf("RepoCommitSHA = %q, want deadbeef", details.Installation.RepoCommitSHA)
	}
	if details.Installation.RepoURL != "https://github.com/owner/repo.git" {
		t.Fatalf("RepoURL = %q", details.Installation.RepoURL)
	}
}

func assertInstallPersisted(t *testing.T, installations []Installation, pluginRoot string) {
	t.Helper()
	if len(installations) != 1 {
		t.Fatalf("installations = %d, want 1", len(installations))
	}
	persisted := installations[0]
	if persisted.Status != InstallationStatusReady {
		t.Fatalf("Status = %q, want ready", persisted.Status)
	}
	if persisted.RepoRef != "main" || persisted.RepoCommitSHA != "deadbeef" {
		t.Fatalf("persisted git metadata = %+v", persisted)
	}
	if !strings.HasPrefix(persisted.CurrentPath, filepath.Join(pluginRoot, "installations")) {
		t.Fatalf("CurrentPath = %q, expected under pluginRoot/installations", persisted.CurrentPath)
	}
	if _, err := os.Stat(filepath.Join(persisted.CurrentPath, "ink-plugin.json")); err != nil {
		t.Fatalf("plugin directory missing ink-plugin.json: %v", err)
	}
}

func TestInstallFromGit(t *testing.T) {
	t.Parallel()

	fixture := filepath.Join("..", "..", "testdata", "plugins", "python-hello-plugin")
	cloner := &stubGitCloner{fixtureDir: fixture, commitSHA: "deadbeef"}
	service, repo, pluginRoot := newGitTestService(t, cloner, []string{"github.com"})

	details, err := service.InstallFromGit(context.Background(), "admin-token", GitInstallInput{
		RepoURL: "https://github.com/owner/repo.git",
		Ref:     "main",
	})
	if err != nil {
		t.Fatalf("InstallFromGit failed: %v", err)
	}

	assertInstallFromGitHappyPath(t, cloner, details)

	installations, err := repo.ListInstallations(context.Background())
	if err != nil {
		t.Fatalf("ListInstallations: %v", err)
	}
	assertInstallPersisted(t, installations, pluginRoot)
}

func TestInstallFromGitRejectsDisallowedHost(t *testing.T) {
	t.Parallel()

	cloner := &stubGitCloner{fixtureDir: "ignored"}
	service, _, _ := newGitTestService(t, cloner, []string{"github.com"})

	_, err := service.InstallFromGit(context.Background(), "admin-token", GitInstallInput{
		RepoURL: "https://gitee.com/owner/repo.git",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if cloner.lastURL != "" {
		t.Fatalf("cloner.Clone must not be called for rejected host, got %q", cloner.lastURL)
	}
}

func TestInstallFromGitRequiresAdmin(t *testing.T) {
	t.Parallel()

	cloner := &stubGitCloner{fixtureDir: "ignored"}
	service, _, _ := newGitTestService(t, cloner, []string{"github.com"})

	_, err := service.InstallFromGit(context.Background(), "member-token", GitInstallInput{
		RepoURL: "https://github.com/owner/repo.git",
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestInstallFromGitDisabledWithoutCloner(t *testing.T) {
	t.Parallel()

	service, _, _ := newGitTestService(t, nil, []string{"github.com"})
	_, err := service.InstallFromGit(context.Background(), "admin-token", GitInstallInput{
		RepoURL: "https://github.com/owner/repo.git",
	})
	if !errors.Is(err, ErrGitInstallDisabled) {
		t.Fatalf("expected ErrGitInstallDisabled, got %v", err)
	}
}

func TestInstallFromGitSubdir(t *testing.T) {
	t.Parallel()

	// Build a fixture that places the plugin inside plugins/hello-python/ so
	// the service must honor the subdir parameter.
	fixtureRoot := t.TempDir()
	target := filepath.Join(fixtureRoot, "plugins", "hello-python")
	if err := copyTree(filepath.Join("..", "..", "testdata", "plugins", "python-hello-plugin"), target); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}

	cloner := &stubGitCloner{fixtureDir: fixtureRoot, commitSHA: "cafef00d"}
	service, _, _ := newGitTestService(t, cloner, []string{"github.com"})

	details, err := service.InstallFromGit(context.Background(), "admin-token", GitInstallInput{
		RepoURL: "https://github.com/owner/repo.git",
		Subdir:  "plugins/hello-python",
	})
	if err != nil {
		t.Fatalf("InstallFromGit with subdir failed: %v", err)
	}
	if details.Installation.RepoSubdir != filepath.Join("plugins", "hello-python") {
		t.Fatalf("RepoSubdir = %q", details.Installation.RepoSubdir)
	}
}

func TestInstallFromGitRejectsSubdirEscape(t *testing.T) {
	t.Parallel()

	cloner := &stubGitCloner{fixtureDir: "ignored"}
	service, _, _ := newGitTestService(t, cloner, []string{"github.com"})

	_, err := service.InstallFromGit(context.Background(), "admin-token", GitInstallInput{
		RepoURL: "https://github.com/owner/repo.git",
		Subdir:  "../../etc",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if cloner.lastURL != "" {
		t.Fatalf("cloner.Clone must not run when subdir is invalid")
	}
}

func TestResolvePluginDirectoryInCloneRejectsEscapingSymlink(t *testing.T) {
	t.Parallel()

	cloneDir := t.TempDir()
	outsideDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsideDir, "ink-plugin.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write outside manifest: %v", err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(cloneDir, "plugin")); err != nil {
		t.Fatalf("create escaping symlink: %v", err)
	}

	_, err := resolvePluginDirectoryInClone(cloneDir, "plugin")
	if !errors.Is(err, ErrInvalidPlugin) {
		t.Fatalf("expected ErrInvalidPlugin, got %v", err)
	}
}

func TestInstallFromGitMissingSubdirManifest(t *testing.T) {
	t.Parallel()

	// Fixture that has no plugin at the requested subdir.
	fixtureRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fixtureRoot, "plugins", "empty"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cloner := &stubGitCloner{fixtureDir: fixtureRoot, commitSHA: "0"}
	service, _, _ := newGitTestService(t, cloner, []string{"github.com"})

	_, err := service.InstallFromGit(context.Background(), "admin-token", GitInstallInput{
		RepoURL: "https://github.com/owner/repo.git",
		Subdir:  "plugins/empty",
	})
	if !errors.Is(err, ErrInvalidPlugin) {
		t.Fatalf("expected ErrInvalidPlugin, got %v", err)
	}
}
