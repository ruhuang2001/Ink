package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ruhuang/ink/server/internal/ai"
	"github.com/ruhuang/ink/server/internal/auth"
)

var (
	ErrForbidden     = errors.New("forbidden")
	ErrMissingSecret = errors.New("memory encryption secret missing")
	ErrDecryptFailed = errors.New("memory encryption key cannot decrypt the stored api key")
	ErrInvalidConfig = errors.New("invalid memory config")
)

const (
	zepRequestTimeout    = 10 * time.Second
	availabilityTimeout  = 3 * time.Second
	availabilityCacheTTL = 30 * time.Second
)

type Encryptor interface {
	Encrypt(plaintext string) ([]byte, []byte, error)
	Decrypt(ciphertext []byte, nonce []byte) (string, error)
}

type Repository interface {
	GetMemorySettings(ctx context.Context) (*StoredSettings, error)
	SaveMemorySettings(ctx context.Context, settings StoredSettings) error
}

type Authenticator interface {
	GetCurrentUser(ctx context.Context, accessToken string) (auth.UserDTO, error)
}

type Clock interface {
	Now() time.Time
}

type StoredSettings struct {
	Enabled       bool
	ZepBaseURL    string
	Ciphertext    []byte
	Nonce         []byte
	UpdatedByUser *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SettingsSummary struct {
	Enabled       bool       `json:"enabled"`
	ZepBaseURL    string     `json:"zepBaseUrl"`
	KeyConfigured bool       `json:"keyConfigured"`
	Available     bool       `json:"available"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
}

type UpdateSettingsInput struct {
	Enabled    bool   `json:"enabled"`
	ZepBaseURL string `json:"zepBaseUrl"`
	ZepAPIKey  string `json:"zepApiKey"` // optional, empty means keep existing
}

type Service struct {
	repo      Repository
	auth      Authenticator
	encryptor Encryptor
	clock     Clock

	// allowInsecurePrivateURL mirrors the AI provider guard: Zep commonly runs
	// on a private network, so operators must opt in explicitly.
	allowInsecurePrivateURL bool

	mu     sync.RWMutex
	client Client

	// Availability probes hit the network, so cache the result instead of
	// blocking every workspace load while Zep is unreachable.
	availMu        sync.Mutex
	availOK        bool
	availCheckedAt time.Time
}

type MemoryService interface {
	GetSettings(ctx context.Context, accessToken string) (SettingsSummary, error)
	UpdateSettings(ctx context.Context, accessToken string, input UpdateSettingsInput) (SettingsSummary, error)
	GetClient() Client
}

func NewService(
	repo Repository,
	authenticator Authenticator,
	encryptor Encryptor,
	clock Clock,
	allowInsecurePrivateURL bool,
) *Service {
	return &Service{
		repo:                    repo,
		auth:                    authenticator,
		encryptor:               encryptor,
		clock:                   clock,
		allowInsecurePrivateURL: allowInsecurePrivateURL,
		client:                  NewNullClient(),
	}
}

func (s *Service) GetSettings(ctx context.Context, accessToken string) (SettingsSummary, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return SettingsSummary{}, err
	}

	settings, err := s.repo.GetMemorySettings(ctx)
	if err != nil {
		return SettingsSummary{}, err
	}
	if settings == nil {
		return SettingsSummary{
			Enabled:   false,
			Available: false,
		}, nil
	}

	summary := s.summaryFromStored(ctx, *settings)
	// Only administrators manage the Zep connection; other members still need
	// the enabled/available flags but must not learn the internal endpoint.
	if currentUser.Role != "admin" {
		summary.ZepBaseURL = ""
	}

	return summary, nil
}

func (s *Service) UpdateSettings(ctx context.Context, accessToken string, input UpdateSettingsInput) (SettingsSummary, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return SettingsSummary{}, err
	}
	if currentUser.Role != "admin" {
		return SettingsSummary{}, ErrForbidden
	}

	baseURL := strings.TrimRight(strings.TrimSpace(input.ZepBaseURL), "/")
	if input.Enabled {
		if baseURL == "" {
			return SettingsSummary{}, ErrInvalidConfig
		}
		if err := ai.ValidateBaseURL(baseURL, s.allowInsecurePrivateURL); err != nil {
			return SettingsSummary{}, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
		}
	}

	existing, err := s.repo.GetMemorySettings(ctx)
	if err != nil {
		return SettingsSummary{}, err
	}

	// Handle API key
	var ciphertext, nonce []byte
	if input.ZepAPIKey != "" {
		if s.encryptor == nil {
			return SettingsSummary{}, ErrMissingSecret
		}
		ciphertext, nonce, err = s.encryptor.Encrypt(input.ZepAPIKey)
		if err != nil {
			return SettingsSummary{}, err
		}
	} else if existing != nil {
		// Keep existing key
		ciphertext = existing.Ciphertext
		nonce = existing.Nonce
	}

	now := s.clock.Now()
	settings := StoredSettings{
		Enabled:       input.Enabled,
		ZepBaseURL:    baseURL,
		Ciphertext:    ciphertext,
		Nonce:         nonce,
		UpdatedByUser: &currentUser.ID,
		UpdatedAt:     now,
	}
	if existing != nil {
		settings.CreatedAt = existing.CreatedAt
	} else {
		settings.CreatedAt = now
	}

	// Build the client before persisting so an unusable key or endpoint is
	// reported to the administrator instead of being stored silently.
	client, err := s.buildClient(settings)
	if err != nil {
		return SettingsSummary{}, err
	}

	if err := s.repo.SaveMemorySettings(ctx, settings); err != nil {
		return SettingsSummary{}, err
	}

	s.setClient(client)

	return s.summaryFromStored(ctx, settings), nil
}

func (s *Service) GetClient() Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.client == nil {
		return NewNullClient()
	}
	return s.client
}

func (s *Service) InitializeClient(ctx context.Context) error {
	settings, err := s.repo.GetMemorySettings(ctx)
	if err != nil {
		return err
	}
	if settings == nil || !settings.Enabled {
		s.setClient(NewNullClient())
		return nil
	}

	client, err := s.buildClient(*settings)
	if err != nil {
		s.setClient(NewNullClient())
		return err
	}

	s.setClient(client)
	return nil
}

func (s *Service) setClient(client Client) {
	s.mu.Lock()
	s.client = client
	s.mu.Unlock()

	// A new client invalidates any cached availability result.
	s.availMu.Lock()
	s.availCheckedAt = time.Time{}
	s.availOK = false
	s.availMu.Unlock()
}

func (s *Service) buildClient(settings StoredSettings) (Client, error) {
	if !settings.Enabled || settings.ZepBaseURL == "" {
		return NewNullClient(), nil
	}
	if err := ai.ValidateBaseURL(settings.ZepBaseURL, s.allowInsecurePrivateURL); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	var apiKey string
	if len(settings.Ciphertext) > 0 && len(settings.Nonce) > 0 {
		if s.encryptor == nil {
			return nil, ErrMissingSecret
		}
		decrypted, err := s.encryptor.Decrypt(settings.Ciphertext, settings.Nonce)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDecryptFailed, err)
		}
		apiKey = decrypted
	}

	return NewZepClient(settings.ZepBaseURL, apiKey, zepRequestTimeout), nil
}

// keyConfigured reports whether a stored key exists and can actually be
// decrypted with the current encryption key.
func (s *Service) keyConfigured(settings StoredSettings) bool {
	if len(settings.Ciphertext) == 0 || len(settings.Nonce) == 0 || s.encryptor == nil {
		return false
	}
	_, err := s.encryptor.Decrypt(settings.Ciphertext, settings.Nonce)
	return err == nil
}

func (s *Service) summaryFromStored(ctx context.Context, settings StoredSettings) SettingsSummary {
	summary := SettingsSummary{
		Enabled:       settings.Enabled,
		ZepBaseURL:    settings.ZepBaseURL,
		KeyConfigured: s.keyConfigured(settings),
		UpdatedAt:     &settings.UpdatedAt,
	}

	if settings.Enabled {
		summary.Available = s.availability(ctx)
	}

	return summary
}

func (s *Service) availability(ctx context.Context) bool {
	s.availMu.Lock()
	defer s.availMu.Unlock()

	if !s.availCheckedAt.IsZero() && s.clock.Now().Sub(s.availCheckedAt) < availabilityCacheTTL {
		return s.availOK
	}

	checkCtx, cancel := context.WithTimeout(ctx, availabilityTimeout)
	defer cancel()

	s.availOK = s.GetClient().IsAvailable(checkCtx) == nil
	s.availCheckedAt = s.clock.Now()
	return s.availOK
}

func (s *Service) String() string {
	return "memory.Service"
}

var _ MemoryService = (*Service)(nil)
