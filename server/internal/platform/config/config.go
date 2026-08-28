package config

import (
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config contains the runtime settings required by the auth service.
type Config struct {
	AppName                     string
	Port                        int
	DatabaseURL                 string
	JWTSecret                   string
	AccessTokenTTL              time.Duration
	RefreshTokenTTL             time.Duration
	RateLimitWindow             time.Duration
	RateLimitMax                int
	RateLimitMaxEntries         int
	TrustedProxyCIDRs           []netip.Prefix
	TrustedProxyHeader          string
	AIConfigEncryptionKey       string
	AIAllowInsecurePrivateURL   bool
	AIProviderTimeout           time.Duration
	MemobirdAccessKey           string
	MemobirdBaseURL             string
	MemobirdTimeout             time.Duration
	PluginRoot                  string
	PluginExecTimeout           time.Duration
	PluginInstallTimeout        time.Duration
	PluginUploadMaxBytes        int64
	PluginOutputMaxBytes        int64
	PluginFetchMaxItems         int
	PluginFetchMaxBlocksPerItem int
	PluginFetchMaxTextBytes     int
	PluginFetchMaxURLBytes      int
	PluginEnvAllowlist          []string
	PluginGitAllowedHosts       []string
	SchedulerPollInterval       time.Duration
	InboxJanitorInterval        time.Duration
	InboxRetention              time.Duration
}

// Load reads application configuration from the current environment.
func Load() (Config, error) {
	port, err := envInt("PORT", 8080)
	if err != nil {
		return Config{}, err
	}

	accessTokenTTL, err := envDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}

	refreshTokenTTL, err := envDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	rateLimitWindow, err := envDuration("LOGIN_RATE_LIMIT_WINDOW", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}

	rateLimitMax, err := envInt("LOGIN_RATE_LIMIT_MAX", 10)
	if err != nil {
		return Config{}, err
	}

	rateLimitMaxEntries, err := envInt("LOGIN_RATE_LIMIT_MAX_ENTRIES", 20000)
	if err != nil {
		return Config{}, err
	}

	trustedProxyCIDRs, err := envPrefixList("TRUSTED_PROXY_CIDRS")
	if err != nil {
		return Config{}, err
	}
	trustedProxyHeader, err := trustedProxyHeaderValue(os.Getenv("TRUSTED_PROXY_HEADER"))
	if err != nil {
		return Config{}, err
	}

	aiProviderTimeout, err := envDuration("AI_PROVIDER_TIMEOUT", 45*time.Second)
	if err != nil {
		return Config{}, err
	}

	memobirdTimeout, err := envDuration("MEMOBIRD_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}

	pluginExecTimeout, err := envDuration("PLUGIN_EXEC_TIMEOUT", 20*time.Second)
	if err != nil {
		return Config{}, err
	}

	pluginInstallTimeout, err := envDuration("PLUGIN_INSTALL_TIMEOUT", 2*time.Minute)
	if err != nil {
		return Config{}, err
	}

	pluginUploadMaxBytes, err := envInt64("PLUGIN_UPLOAD_MAX_BYTES", 32<<20)
	if err != nil {
		return Config{}, err
	}

	pluginOutputMaxBytes, err := envInt64("PLUGIN_OUTPUT_MAX_BYTES", 1<<20)
	if err != nil {
		return Config{}, err
	}

	pluginFetchMaxItems, err := envInt("PLUGIN_FETCH_MAX_ITEMS", 100)
	if err != nil {
		return Config{}, err
	}

	pluginFetchMaxBlocksPerItem, err := envInt("PLUGIN_FETCH_MAX_BLOCKS_PER_ITEM", 80)
	if err != nil {
		return Config{}, err
	}

	pluginFetchMaxTextBytes, err := envInt("PLUGIN_FETCH_MAX_TEXT_BYTES", 32768)
	if err != nil {
		return Config{}, err
	}

	pluginFetchMaxURLBytes, err := envInt("PLUGIN_FETCH_MAX_URL_BYTES", 2048)
	if err != nil {
		return Config{}, err
	}

	schedulerPollInterval, err := envDuration("SCHEDULER_POLL_INTERVAL", 30*time.Second)
	if err != nil {
		return Config{}, err
	}

	inboxJanitorInterval, err := envDuration("INBOX_JANITOR_INTERVAL", 6*time.Hour)
	if err != nil {
		return Config{}, err
	}

	inboxRetention, err := envDuration("INBOX_RETENTION", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppName:                     envString("APP_NAME", "ink-auth"),
		Port:                        port,
		DatabaseURL:                 os.Getenv("DATABASE_URL"),
		JWTSecret:                   os.Getenv("JWT_SECRET"),
		AccessTokenTTL:              accessTokenTTL,
		RefreshTokenTTL:             refreshTokenTTL,
		RateLimitWindow:             rateLimitWindow,
		RateLimitMax:                rateLimitMax,
		RateLimitMaxEntries:         rateLimitMaxEntries,
		TrustedProxyCIDRs:           trustedProxyCIDRs,
		TrustedProxyHeader:          trustedProxyHeader,
		AIConfigEncryptionKey:       os.Getenv("AI_CONFIG_ENCRYPTION_KEY"),
		AIAllowInsecurePrivateURL:   envBool("AI_ALLOW_INSECURE_PRIVATE_URL", false),
		AIProviderTimeout:           aiProviderTimeout,
		MemobirdAccessKey:           os.Getenv("MEMOBIRD_ACCESS_KEY"),
		MemobirdBaseURL:             os.Getenv("MEMOBIRD_BASE_URL"),
		MemobirdTimeout:             memobirdTimeout,
		PluginRoot:                  envString("PLUGIN_ROOT", ".plugins"),
		PluginExecTimeout:           pluginExecTimeout,
		PluginInstallTimeout:        pluginInstallTimeout,
		PluginUploadMaxBytes:        pluginUploadMaxBytes,
		PluginOutputMaxBytes:        pluginOutputMaxBytes,
		PluginFetchMaxItems:         pluginFetchMaxItems,
		PluginFetchMaxBlocksPerItem: pluginFetchMaxBlocksPerItem,
		PluginFetchMaxTextBytes:     pluginFetchMaxTextBytes,
		PluginFetchMaxURLBytes:      pluginFetchMaxURLBytes,
		PluginEnvAllowlist:          envStringList("PLUGIN_ENV_ALLOWLIST", []string{}),
		PluginGitAllowedHosts:       envStringList("PLUGIN_GIT_ALLOWED_HOSTS", []string{"github.com", "gitee.com", "gitlab.com"}),
		SchedulerPollInterval:       schedulerPollInterval,
		InboxJanitorInterval:        inboxJanitorInterval,
		InboxRetention:              inboxRetention,
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	if cfg.Port <= 0 {
		return Config{}, fmt.Errorf("PORT must be positive")
	}

	if cfg.AccessTokenTTL <= 0 {
		return Config{}, fmt.Errorf("ACCESS_TOKEN_TTL must be positive")
	}

	if cfg.RefreshTokenTTL <= 0 {
		return Config{}, fmt.Errorf("REFRESH_TOKEN_TTL must be positive")
	}

	if cfg.RateLimitWindow <= 0 {
		return Config{}, fmt.Errorf("LOGIN_RATE_LIMIT_WINDOW must be positive")
	}

	if cfg.RateLimitMax <= 0 {
		return Config{}, fmt.Errorf("LOGIN_RATE_LIMIT_MAX must be positive")
	}
	if cfg.RateLimitMaxEntries < 2 {
		return Config{}, fmt.Errorf("LOGIN_RATE_LIMIT_MAX_ENTRIES must be at least 2")
	}
	if (len(cfg.TrustedProxyCIDRs) == 0) != (cfg.TrustedProxyHeader == "") {
		return Config{}, fmt.Errorf("TRUSTED_PROXY_CIDRS and TRUSTED_PROXY_HEADER must be configured together")
	}
	if cfg.AIProviderTimeout <= 0 {
		return Config{}, fmt.Errorf("AI_PROVIDER_TIMEOUT must be positive")
	}
	if cfg.MemobirdTimeout <= 0 {
		return Config{}, fmt.Errorf("MEMOBIRD_TIMEOUT must be positive")
	}
	if cfg.PluginExecTimeout <= 0 {
		return Config{}, fmt.Errorf("PLUGIN_EXEC_TIMEOUT must be positive")
	}
	if cfg.PluginInstallTimeout <= 0 {
		return Config{}, fmt.Errorf("PLUGIN_INSTALL_TIMEOUT must be positive")
	}
	if cfg.PluginUploadMaxBytes <= 0 {
		return Config{}, fmt.Errorf("PLUGIN_UPLOAD_MAX_BYTES must be positive")
	}
	if cfg.PluginOutputMaxBytes <= 0 {
		return Config{}, fmt.Errorf("PLUGIN_OUTPUT_MAX_BYTES must be positive")
	}
	if cfg.PluginFetchMaxItems <= 0 {
		return Config{}, fmt.Errorf("PLUGIN_FETCH_MAX_ITEMS must be positive")
	}
	if cfg.PluginFetchMaxBlocksPerItem <= 0 {
		return Config{}, fmt.Errorf("PLUGIN_FETCH_MAX_BLOCKS_PER_ITEM must be positive")
	}
	if cfg.PluginFetchMaxTextBytes <= 0 {
		return Config{}, fmt.Errorf("PLUGIN_FETCH_MAX_TEXT_BYTES must be positive")
	}
	if cfg.PluginFetchMaxURLBytes <= 0 {
		return Config{}, fmt.Errorf("PLUGIN_FETCH_MAX_URL_BYTES must be positive")
	}
	if cfg.SchedulerPollInterval <= 0 {
		return Config{}, fmt.Errorf("SCHEDULER_POLL_INTERVAL must be positive")
	}
	if cfg.InboxJanitorInterval <= 0 {
		return Config{}, fmt.Errorf("INBOX_JANITOR_INTERVAL must be positive")
	}
	if cfg.InboxRetention <= 0 {
		return Config{}, fmt.Errorf("INBOX_RETENTION must be positive")
	}

	return cfg, nil
}

// LoadDotEnv loads the first local dotenv file that exists.
func LoadDotEnv() error {
	candidates := []string{
		".env",
		filepath.Join("server", ".env"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return godotenv.Load(candidate)
		}
	}

	return nil
}

// ResolveProjectPath resolves a path from either the repo root or server directory.
func ResolveProjectPath(path string) string {
	candidates := []string{
		path,
		filepath.Join("server", path),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return path
}

func envString(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func envInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
	}

	return parsed, nil
}

func envInt64(key string, fallback int64) (int64, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid integer: %w", key, err)
	}

	return parsed, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}

	return parsed, nil
}

func envPrefixList(key string) ([]netip.Prefix, error) {
	values := envStringList(key, nil)
	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("%s contains invalid CIDR %q: %w", key, value, err)
		}
		if prefix.Addr().Is4In6() {
			return nil, fmt.Errorf("%s contains IPv4-mapped CIDR %q; use its IPv4 CIDR equivalent", key, value)
		}
		result = append(result, prefix.Masked())
	}
	return result, nil
}

func trustedProxyHeaderValue(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "", "forwarded", "x-forwarded-for":
		return value, nil
	default:
		return "", fmt.Errorf("TRUSTED_PROXY_HEADER must be forwarded or x-forwarded-for")
	}
}

// envStringList returns a comma-separated env var as a trimmed non-empty
// string slice, falling back to the provided default when the var is unset
// or empty.
func envStringList(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	case "0", "false", "FALSE", "no", "NO", "off", "OFF":
		return false
	default:
		return fallback
	}
}
