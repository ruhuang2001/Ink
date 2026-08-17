package plugins

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// safeGitRefPattern restricts the branch/tag name to characters that cannot
// be misinterpreted as arguments or shell metacharacters and that are valid
// inside a git refname.
var safeGitRefPattern = regexp.MustCompile(`^[A-Za-z0-9._\-/]+$`)

// GitCloner performs a shallow clone of a remote git repository into destDir
// and returns the resolved commit SHA. Implementations must never prompt for
// credentials interactively; network transports other than HTTPS should be
// rejected before Clone is called.
type GitCloner interface {
	Clone(ctx context.Context, repoURL, ref, destDir string) (commitSHA string, err error)
}

// GoGitCloner performs clones through the pure-Go `go-git` library. It
// intentionally does NOT shell out to the `git` binary, which means an
// attacker cannot smuggle arguments into a subprocess even if upstream
// validation is ever relaxed.
type GoGitCloner struct{}

// Clone performs a shallow single-branch clone of repoURL into destDir and
// returns the resolved commit SHA. An empty ref lets the remote default
// branch be cloned.
func (GoGitCloner) Clone(ctx context.Context, repoURL, ref, destDir string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref != "" && !safeGitRefPattern.MatchString(ref) {
		return "", fmt.Errorf("%w: git ref contains unsupported characters", ErrInvalidInput)
	}
	repo, err := cloneRepo(ctx, repoURL, ref, destDir)
	if err != nil {
		return "", fmt.Errorf("git clone: %w", err)
	}
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("git head: %w", err)
	}
	return head.Hash().String(), nil
}

// cloneRepo performs the PlainCloneContext call, falling back from a branch
// reference to a tag reference when ref is non-empty and the branch lookup
// fails. Extracted to keep GoGitCloner.Clone under the Codacy complexity
// threshold.
func cloneRepo(ctx context.Context, repoURL, ref, destDir string) (*gogit.Repository, error) {
	opts := &gogit.CloneOptions{
		URL:          repoURL,
		Depth:        1,
		SingleBranch: true,
		Tags:         gogit.NoTags,
	}
	if ref != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(ref)
	}
	repo, err := gogit.PlainCloneContext(ctx, destDir, false, opts)
	if err == nil || ref == "" || !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return repo, err
	}
	opts.ReferenceName = plumbing.NewTagReferenceName(ref)
	return gogit.PlainCloneContext(ctx, destDir, false, opts)
}

// GitInstallInput describes a request to install a plugin from a remote git
// repository. Ref and Subdir are both optional.
type GitInstallInput struct {
	RepoURL string `json:"repoUrl"`
	Ref     string `json:"ref"`
	Subdir  string `json:"subdir"`
}

// DefaultGitAllowedHosts enumerates the hosts the service accepts when the
// operator has not customized PLUGIN_GIT_ALLOWED_HOSTS. The list is
// intentionally small — SSRF-style probing against internal services is the
// biggest risk on this path.
var DefaultGitAllowedHosts = []string{
	"github.com",
	"gitee.com",
	"gitlab.com",
}

// validateGitURL parses and sanity-checks the provided repository URL against
// the allowlist. It returns a normalized URL (scheme + host + path) that can
// be handed to git clone.
func validateGitURL(rawURL string, allowedHosts []string) (string, string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", "", fmt.Errorf("%w: repository URL is required", ErrInvalidInput)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("%w: invalid repository URL", ErrInvalidInput)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", "", fmt.Errorf("%w: repository URL must use https", ErrInvalidInput)
	}
	if parsed.User != nil {
		return "", "", fmt.Errorf("%w: credentials must not be embedded in the URL", ErrInvalidInput)
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", "", fmt.Errorf("%w: repository URL is missing a host", ErrInvalidInput)
	}
	if !isHostAllowed(host, allowedHosts) {
		return "", "", fmt.Errorf("%w: repository host %q is not allowed", ErrInvalidInput, host)
	}

	// Reconstruct the URL without query/fragment so callers can't smuggle in
	// credentials or auth params via the request.
	normalizedHost := host
	if port := parsed.Port(); port != "" {
		normalizedHost = normalizedHost + ":" + port
	}
	normalized := &url.URL{
		Scheme: strings.ToLower(parsed.Scheme),
		Host:   normalizedHost,
		Path:   parsed.Path,
	}
	return normalized.String(), host, nil
}

// isHostAllowed reports whether host matches any entry in allowed. Entries
// may be exact hostnames ("github.com") or a single-level wildcard
// ("*.example.com") to permit sub-domains.
func isHostAllowed(host string, allowed []string) bool {
	for _, entry := range allowed {
		entry = strings.TrimSpace(strings.ToLower(entry))
		if entry == "" {
			continue
		}
		if entry == host {
			return true
		}
		if strings.HasPrefix(entry, "*.") {
			suffix := entry[1:] // ".example.com"
			if strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
				return true
			}
		}
	}
	return false
}

// sanitizeSubdir validates that subdir is a well-formed relative path that
// stays inside the clone directory.
func sanitizeSubdir(subdir string) (string, error) {
	subdir = strings.TrimSpace(subdir)
	if subdir == "" {
		return "", nil
	}
	if !filepath.IsLocal(subdir) {
		return "", fmt.Errorf("%w: subdir must be a relative path", ErrInvalidInput)
	}
	cleaned := filepath.Clean(subdir)
	return cleaned, nil
}

// resolvePluginDirectoryInClone returns the directory inside cloneDir that
// contains ink-plugin.json. If subdir is set it is applied as a relative
// path; otherwise the clone root (or a single top-level subdirectory) is
// searched.
func resolvePluginDirectoryInClone(cloneDir string, subdir string) (string, error) {
	if subdir != "" {
		if !filepath.IsLocal(subdir) {
			return "", fmt.Errorf("%w: subdir escapes the repository", ErrInvalidInput)
		}
		pluginDir, err := securejoin.SecureJoin(cloneDir, subdir)
		if err != nil {
			return "", fmt.Errorf("%w: invalid repository subdir", ErrInvalidInput)
		}
		info, err := os.Stat(pluginDir)
		if err != nil || !info.IsDir() {
			return "", fmt.Errorf("%w: subdir %q not found in repository", ErrInvalidPlugin, subdir)
		}
		if _, err := os.Stat(filepath.Join(pluginDir, "ink-plugin.json")); err != nil {
			return "", fmt.Errorf("%w: ink-plugin.json missing in subdir %q", ErrInvalidPlugin, subdir)
		}
		return pluginDir, nil
	}
	return resolvePluginDirectory(cloneDir)
}

// ErrGitInstallDisabled is returned by InstallFromGit when the service was
// constructed without a GitCloner.
var ErrGitInstallDisabled = errors.New("git install is not enabled on this server")
