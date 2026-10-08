package postgres

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ruhuang/ink/server/internal/ai"
	"github.com/ruhuang/ink/server/internal/auth"
	"github.com/ruhuang/ink/server/internal/platform/clock"
	"github.com/ruhuang/ink/server/internal/platform/idgen"
	"github.com/ruhuang/ink/server/internal/platform/migrate"
	"github.com/ruhuang/ink/server/internal/platform/secret"
	"github.com/ruhuang/ink/server/internal/plugins"
)

type recoveryAuthenticator struct{}

func (recoveryAuthenticator) GetCurrentUser(context.Context, string) (auth.UserDTO, error) {
	return auth.UserDTO{ID: "recovery-user", Role: "admin"}, nil
}

type recoveryCompletion struct {
	expected ai.RuntimeConfig
	calls    int
}

func (c *recoveryCompletion) CreateReply(_ context.Context, config ai.RuntimeConfig, _ []ai.ChatMessage) (ai.ReplyResult, error) {
	c.calls++
	if config != c.expected {
		return ai.ReplyResult{}, errors.New("recovered AI configuration differs")
	}
	return ai.ReplyResult{Content: "synthetic reply", Model: config.Model}, nil
}

// This opt-in test owns its database and never reads DATABASE_URL, .env, or
// AI_CONFIG_ENCRYPTION_KEY. The random recovery keys exist only in memory.
func TestBackupRestoresEncryptedSecretsAndPluginFiles(t *testing.T) {
	if os.Getenv("INK_TEST_BACKUP_RECOVERY") != "1" {
		t.Skip("set INK_TEST_BACKUP_RECOVERY=1 to run isolated Docker recovery test")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Fatal("python3 is required for the trusted fixture")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	container := "ink-secret-recovery-" + hex.EncodeToString(recoveryRandom(t, 8))
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := exec.CommandContext(cleanupCtx, "docker", "stop", container).Run(); err != nil {
			t.Error("temporary recovery container cleanup failed")
		}
	})
	recoveryDocker(t, ctx, nil, "run", "--rm", "--detach", "--name", container,
		"--env", "POSTGRES_PASSWORD=postgres", "--publish", "127.0.0.1::5432", "postgres:16")
	ready := false
	for range 30 {
		if exec.CommandContext(ctx, "docker", "exec", container, "pg_isready", "-U", "postgres").Run() == nil {
			ready = true
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("temporary PostgreSQL readiness timed out")
		case <-time.After(time.Second):
		}
	}
	if !ready {
		t.Fatal("temporary PostgreSQL was not ready")
	}
	address := strings.TrimSpace(string(recoveryDocker(t, ctx, nil, "port", container, "5432/tcp")))
	_, port, ok := strings.Cut(address, ":")
	if !ok {
		t.Fatal("temporary PostgreSQL port unavailable")
	}
	for _, name := range []string{"recovery_source", "recovery_restored"} {
		recoveryDocker(t, ctx, nil, "exec", container, "createdb", "-U", "postgres", name)
	}
	sourceDB := recoveryPool(t, ctx, port, "recovery_source")
	if _, err := migrate.NewRunner(sourceDB).Up(ctx, "../../../../migrations"); err != nil {
		t.Fatal("apply synthetic recovery migrations")
	}
	if _, err := sourceDB.Exec(ctx, `insert into users (id,email,password_hash,display_name,role,status) values ('recovery-user','recovery@example.com','synthetic-hash','Recovery','admin','active')`); err != nil {
		t.Fatal("create synthetic recovery user")
	}
	box := recoveryBox(t)
	apiKey := "synthetic-ai-" + hex.EncodeToString(recoveryRandom(t, 16))
	pluginSecret := "synthetic-plugin-" + hex.EncodeToString(recoveryRandom(t, 16))
	completion := &recoveryCompletion{expected: ai.RuntimeConfig{ProviderName: "Recovery fixture", ProviderType: ai.DefaultProviderType, BaseURL: "https://example.com/v1", Model: "synthetic-model", APIKey: apiKey}}
	newAI := func(store *Store, encryptor ai.Encryptor) *ai.Service {
		return ai.NewService(store, recoveryAuthenticator{}, completion, encryptor, clock.SystemClock{}, false)
	}
	source := New(sourceDB)
	if _, err := newAI(source, box).UpdateSystemConfig(ctx, "synthetic-token", ai.UpdateConfigInput{ProviderName: completion.expected.ProviderName, ProviderType: completion.expected.ProviderType, BaseURL: completion.expected.BaseURL, Model: completion.expected.Model, APIKey: apiKey}); err != nil {
		t.Fatal("store encrypted synthetic AI configuration")
	}
	root := t.TempDir()
	installed := filepath.Join(root, "installed")
	if err := os.CopyFS(installed, os.DirFS("../../../../testdata/plugins/python-hello-plugin")); err != nil {
		t.Fatal("copy trusted source plugin fixture")
	}
	manifest, err := os.ReadFile(filepath.Join(installed, "ink-plugin.json"))
	if err != nil {
		t.Fatal("read trusted fixture manifest")
	}
	now := time.Now().UTC()
	installation := plugins.Installation{ID: "recovery-plugin", PluginKey: "python-hello-source", SourceType: plugins.SourceTypeUpload, DisplayName: "Python Hello Source", Version: "1.0.0", RuntimeType: "python", ManifestJSON: manifest, CurrentPath: installed, Status: plugins.InstallationStatusReady, CreatedAt: now, UpdatedAt: now}
	if err := source.SaveInstallation(ctx, installation); err != nil {
		t.Fatal("store synthetic plugin installation")
	}
	newPlugin := func(store *Store, encryptor plugins.Encryptor) *plugins.Service {
		return plugins.NewService(store, recoveryAuthenticator{}, encryptor, idgen.Generator{}, clock.SystemClock{}, nil, root, 5*time.Second, 5*time.Second, plugins.RuntimeLimits{}, nil, nil)
	}
	if _, err := newPlugin(source, box).SaveBinding(ctx, "synthetic-token", installation.ID, plugins.BindingInput{Enabled: true, Config: map[string]any{"sourceName": "Recovery source", "message": "Recovered fixture content", "uppercase": true}, Secrets: map[string]string{"apiToken": pluginSecret}}); err != nil {
		t.Fatal("store encrypted plugin binding and validate trusted fixture")
	}
	trigger := plugins.FetchTrigger{Kind: plugins.TriggerKind("manual"), TriggeredAt: now.Format(time.RFC3339), Timezone: "UTC"}
	sourceBinding, sourceSecrets, err := newPlugin(source, box).GetBindingForUser(ctx, installation.ID, "recovery-user")
	if err != nil {
		t.Fatal("load source plugin binding")
	}
	expectedFetch, err := newPlugin(source, box).ExecuteFetch(ctx, installation, sourceBinding, sourceSecrets, trigger)
	if err != nil || len(expectedFetch.Items) != 1 {
		t.Fatal("source trusted fixture did not fetch one item")
	}
	pluginBackup := filepath.Join(root, "plugin-backup")
	if err := os.CopyFS(pluginBackup, os.DirFS(installed)); err != nil {
		t.Fatal("back up trusted plugin files")
	}
	dump := recoveryDocker(t, ctx, nil, "exec", container, "pg_dump", "-U", "postgres", "--format=custom", "--compress=0", "--no-owner", "--no-acl", "recovery_source")
	for _, plaintext := range []string{apiKey, pluginSecret} {
		if bytes.Contains(dump, []byte(plaintext)) {
			t.Fatal("synthetic plaintext present in database backup")
		}
	}
	recoveryDocker(t, ctx, dump, "exec", "-i", container, "pg_restore", "-U", "postgres", "--dbname=recovery_restored", "--no-owner", "--no-acl", "--exit-on-error")
	if err := os.RemoveAll(installed); err != nil {
		t.Fatal("remove source fixture files before recovery")
	}
	if err := os.CopyFS(installed, os.DirFS(pluginBackup)); err != nil {
		t.Fatal("restore trusted plugin files to recorded path")
	}
	restored := New(recoveryPool(t, ctx, port, "recovery_restored"))
	restoredConfig, err := restored.GetSystemConfig(ctx)
	if err != nil || restoredConfig == nil {
		t.Fatal("load restored encrypted AI configuration")
	}
	sourceConfig, err := source.GetSystemConfig(ctx)
	if err != nil || !bytes.Equal(sourceConfig.Ciphertext, restoredConfig.Ciphertext) || !bytes.Equal(sourceConfig.Nonce, restoredConfig.Nonce) {
		t.Fatal("AI encrypted bytes changed during recovery")
	}
	restoredPlugin := newPlugin(restored, box)
	binding, recoveredSecrets, err := restoredPlugin.GetBindingForUser(ctx, installation.ID, "recovery-user")
	if err != nil || recoveredSecrets["apiToken"] != pluginSecret || !bytes.Equal(sourceBinding.Ciphertext, binding.Ciphertext) || !bytes.Equal(sourceBinding.Nonce, binding.Nonce) {
		t.Fatal("plugin secret changed during recovery")
	}
	recoveredInstallation, _, err := restoredPlugin.GetInstallation(ctx, installation.ID)
	if err != nil || recoveredInstallation.CurrentPath != installed {
		t.Fatal("plugin installation reference changed during recovery")
	}
	recoveredFetch, err := restoredPlugin.ExecuteFetch(ctx, recoveredInstallation, binding, recoveredSecrets, trigger)
	if err != nil || !reflect.DeepEqual(expectedFetch, recoveredFetch) {
		t.Fatal("restored trusted fixture output changed")
	}
	input := ai.ReplyInput{Messages: []ai.ChatMessage{{Role: "user", Content: "Synthetic request"}}}
	if _, err := newAI(restored, box).GenerateReply(ctx, "synthetic-token", input); err != nil || completion.calls != 1 {
		t.Fatal("restored AI key did not reach the local test client")
	}
	for _, test := range []struct {
		name      string
		encryptor *secret.Box
	}{
		{name: "wrong key", encryptor: recoveryBox(t)},
		{name: "missing key"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var aiEncryptor ai.Encryptor
			var pluginEncryptor plugins.Encryptor
			if test.encryptor != nil {
				aiEncryptor, pluginEncryptor = test.encryptor, test.encryptor
			}
			reply, err := newAI(restored, aiEncryptor).GenerateReply(ctx, "synthetic-token", input)
			if !errors.Is(err, ai.ErrMissingSecret) || reply.Content != "" || completion.calls != 1 {
				t.Fatal("invalid AI recovery key did not fail closed")
			}
			recoveryNoLeak(t, err.Error(), apiKey, pluginSecret)
			_, values, err := newPlugin(restored, pluginEncryptor).GetBindingForUser(ctx, installation.ID, "recovery-user")
			if err == nil || values != nil {
				t.Fatal("invalid plugin recovery key produced secret values")
			}
			recoveryNoLeak(t, err.Error(), apiKey, pluginSecret)
		})
	}
	summary, err := newAI(restored, box).GetConfigSummary(ctx, "synthetic-token")
	if err != nil {
		t.Fatal("load recovered AI summary")
	}
	details, err := restoredPlugin.GetUserPlugin(ctx, "synthetic-token", installation.ID)
	if err != nil {
		t.Fatal("load recovered plugin summary")
	}
	public, err := json.Marshal([]any{summary, details})
	if err != nil {
		t.Fatal("encode recovered public summaries")
	}
	recoveryNoLeak(t, string(public), apiKey, pluginSecret)
	if _, err := restored.db.Exec(ctx, `update ai_provider_settings set api_key_nonce = decode('00','hex'); update plugin_bindings set secret_nonce = decode('00','hex')`); err != nil {
		t.Fatal("create synthetic damaged nonce")
	}
	if reply, err := newAI(restored, box).GenerateReply(ctx, "synthetic-token", input); !errors.Is(err, ai.ErrMissingSecret) || reply.Content != "" {
		t.Fatal("damaged AI nonce did not fail without plaintext")
	}
	if _, values, err := restoredPlugin.GetBindingForUser(ctx, installation.ID, "recovery-user"); err == nil || values != nil {
		t.Fatal("damaged plugin nonce did not fail without plaintext")
	} else {
		recoveryNoLeak(t, err.Error(), apiKey, pluginSecret)
	}
	t.Log("Restored encrypted AI/plugin values, rejected wrong/missing keys and damaged nonces, ran restored trusted plugin files; no external providers contacted.")
}

func recoveryRandom(t *testing.T, size int) []byte {
	t.Helper()
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		t.Fatal("generate synthetic recovery entropy")
	}
	return value
}

func recoveryBox(t *testing.T) *secret.Box {
	t.Helper()
	box, err := secret.NewBox(base64.StdEncoding.EncodeToString(recoveryRandom(t, 32)))
	if err != nil {
		t.Fatal("create synthetic recovery encryptor")
	}
	return box
}

func recoveryDocker(t *testing.T, ctx context.Context, stdin []byte, args ...string) []byte {
	t.Helper()
	command := exec.CommandContext(ctx, "docker", args...)
	command.Stdin = bytes.NewReader(stdin)
	output, err := command.Output()
	if err != nil {
		t.Fatal("isolated recovery Docker operation failed")
	}
	return output
}

func recoveryPool(t *testing.T, ctx context.Context, port string, database string) *pgxpool.Pool {
	t.Helper()
	db, err := pgxpool.New(ctx, fmt.Sprintf("postgres://postgres:postgres@127.0.0.1:%s/%s?sslmode=disable", port, database))
	if err != nil {
		t.Fatal("connect isolated recovery database")
	}
	t.Cleanup(db.Close)
	return db
}

func recoveryNoLeak(t *testing.T, value string, plaintexts ...string) {
	t.Helper()
	if slices.ContainsFunc(plaintexts, func(plaintext string) bool { return strings.Contains(value, plaintext) }) {
		t.Fatal("synthetic secret leaked into a public response")
	}
}
