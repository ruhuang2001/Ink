package plugins

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
	defer func() { _ = os.RemoveAll(tempDir) }()
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

func (b *limitedBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *limitedBuffer) Exceeded() bool { return b.exceeded }

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
		if prefix, _, found := strings.Cut(entry, "="); found {
			key = prefix
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
