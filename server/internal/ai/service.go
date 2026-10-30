package ai

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ruhuang/ink/server/internal/auth"
)

var (
	ErrForbidden           = errors.New("forbidden")
	ErrNotConfigured       = errors.New("ai provider not configured")
	ErrInvalidConfig       = errors.New("invalid ai config")
	ErrProviderUnavailable = errors.New("ai provider unavailable")
	ErrMissingSecret       = errors.New("ai encryption secret missing")
	ErrInvalidInput        = errors.New("invalid ai input")
)

type Encryptor interface {
	Encrypt(plaintext string) ([]byte, []byte, error)
	Decrypt(ciphertext []byte, nonce []byte) (string, error)
}

type Repository interface {
	GetSystemConfig(ctx context.Context) (*StoredConfig, error)
	SaveSystemConfig(ctx context.Context, config StoredConfig) error
}

type Authenticator interface {
	GetCurrentUser(ctx context.Context, accessToken string) (auth.UserDTO, error)
}

type Clock interface {
	Now() time.Time
}

type StoredConfig struct {
	ProviderName  string
	ProviderType  string
	BaseURL       string
	Model         string
	Ciphertext    []byte
	Nonce         []byte
	UpdatedByUser *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type ConfigSummary struct {
	Bound         bool       `json:"bound"`
	ProviderName  string     `json:"providerName"`
	ProviderType  string     `json:"providerType"`
	BaseURL       string     `json:"baseUrl"`
	Model         string     `json:"model"`
	KeyConfigured bool       `json:"keyConfigured"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
}

type UpdateConfigInput struct {
	ProviderName string `json:"providerName"`
	ProviderType string `json:"providerType"`
	BaseURL      string `json:"baseUrl"`
	Model        string `json:"model"`
	APIKey       string `json:"apiKey"`
}

type ReplyInput struct {
	Messages       []ChatMessage `json:"messages"`
	ConversationID string        `json:"conversationId,omitempty"`
}

// MemoryEnhancer optionally augments replies with recalled conversation memory.
// Implementations must degrade silently; memory is never critical to a reply.
type MemoryEnhancer interface {
	EnhanceMessages(ctx context.Context, userID, conversationID string, messages []ChatMessage) ([]ChatMessage, error)
	StoreConversation(ctx context.Context, userID, conversationID string, messages []ChatMessage, assistantReply string) error
}

type Service struct {
	repo                    Repository
	auth                    Authenticator
	client                  CompletionClient
	encryptor               Encryptor
	clock                   Clock
	allowInsecurePrivateURL bool
	memory                  MemoryEnhancer
}

type AIService interface {
	GetConfigSummary(ctx context.Context, accessToken string) (ConfigSummary, error)
	UpdateSystemConfig(ctx context.Context, accessToken string, input UpdateConfigInput) (ConfigSummary, error)
	GenerateReply(ctx context.Context, accessToken string, input ReplyInput) (ReplyResult, error)
}

func NewService(
	repo Repository,
	authenticator Authenticator,
	client CompletionClient,
	encryptor Encryptor,
	clock Clock,
	allowInsecurePrivateURL bool,
) *Service {
	return &Service{
		repo:                    repo,
		auth:                    authenticator,
		client:                  client,
		encryptor:               encryptor,
		clock:                   clock,
		allowInsecurePrivateURL: allowInsecurePrivateURL,
	}
}

func (s *Service) GetConfigSummary(ctx context.Context, accessToken string) (ConfigSummary, error) {
	if _, err := s.auth.GetCurrentUser(ctx, accessToken); err != nil {
		return ConfigSummary{}, err
	}

	config, err := s.repo.GetSystemConfig(ctx)
	if err != nil {
		return ConfigSummary{}, err
	}
	if config == nil {
		return ConfigSummary{
			Bound:        false,
			ProviderName: DefaultProviderName,
			ProviderType: DefaultProviderType,
			Model:        DefaultModel,
		}, nil
	}

	return summaryFromStored(*config), nil
}

func (s *Service) UpdateSystemConfig(ctx context.Context, accessToken string, input UpdateConfigInput) (ConfigSummary, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return ConfigSummary{}, err
	}
	if currentUser.Role != "admin" {
		return ConfigSummary{}, ErrForbidden
	}
	if s.encryptor == nil {
		return ConfigSummary{}, ErrMissingSecret
	}

	existing, err := s.repo.GetSystemConfig(ctx)
	if err != nil {
		return ConfigSummary{}, err
	}

	normalized, err := s.normalizeRuntimeInput(input)
	if err != nil {
		return ConfigSummary{}, err
	}

	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" && existing != nil {
		apiKey, err = s.encryptor.Decrypt(existing.Ciphertext, existing.Nonce)
		if err != nil {
			return ConfigSummary{}, ErrMissingSecret
		}
	}
	if apiKey == "" {
		return ConfigSummary{}, ErrInvalidConfig
	}

	ciphertext, nonce, err := s.encryptor.Encrypt(apiKey)
	if err != nil {
		return ConfigSummary{}, err
	}

	now := s.clock.Now()
	config := StoredConfig{
		ProviderName: normalized.ProviderName,
		ProviderType: normalized.ProviderType,
		BaseURL:      normalized.BaseURL,
		Model:        normalized.Model,
		Ciphertext:   ciphertext,
		Nonce:        nonce,
		UpdatedByUser: func() *string {
			userID := currentUser.ID
			return &userID
		}(),
		UpdatedAt: now,
	}
	if existing != nil {
		config.CreatedAt = existing.CreatedAt
	} else {
		config.CreatedAt = now
	}

	if err := s.repo.SaveSystemConfig(ctx, config); err != nil {
		return ConfigSummary{}, err
	}

	return summaryFromStored(config), nil
}

// SetMemoryEnhancer wires optional conversation memory into replies. It is
// separate from NewService so tests and deployments without memory stay simple.
func (s *Service) SetMemoryEnhancer(enhancer MemoryEnhancer) {
	s.memory = enhancer
}

func (s *Service) GenerateReply(ctx context.Context, accessToken string, input ReplyInput) (ReplyResult, error) {
	currentUser, err := s.auth.GetCurrentUser(ctx, accessToken)
	if err != nil {
		return ReplyResult{}, err
	}

	runtimeConfig, err := s.loadRuntimeConfig(ctx)
	if err != nil {
		return ReplyResult{}, err
	}

	messages, err := normalizeMessages(input.Messages)
	if err != nil {
		return ReplyResult{}, err
	}

	if s.memory != nil && input.ConversationID != "" {
		enhanced, err := s.memory.EnhanceMessages(ctx, currentUser.ID, input.ConversationID, messages)
		if err == nil && len(enhanced) > 0 {
			messages = enhanced
		}
	}

	reply, err := s.client.CreateReply(ctx, runtimeConfig, messages)
	if err != nil {
		return ReplyResult{}, err
	}

	if s.memory != nil && input.ConversationID != "" {
		// Stored history is useful but never worth failing a delivered reply.
		_ = s.memory.StoreConversation(ctx, currentUser.ID, input.ConversationID, messages, reply.Content)
	}

	return reply, nil
}

func (s *Service) loadRuntimeConfig(ctx context.Context) (RuntimeConfig, error) {
	config, err := s.repo.GetSystemConfig(ctx)
	if err != nil {
		return RuntimeConfig{}, err
	}
	if config == nil {
		return RuntimeConfig{}, ErrNotConfigured
	}
	if s.encryptor == nil {
		return RuntimeConfig{}, ErrMissingSecret
	}

	apiKey, err := s.encryptor.Decrypt(config.Ciphertext, config.Nonce)
	if err != nil {
		return RuntimeConfig{}, ErrMissingSecret
	}

	if err := ValidateBaseURL(config.BaseURL, s.allowInsecurePrivateURL); err != nil {
		return RuntimeConfig{}, err
	}

	return RuntimeConfig{
		ProviderName: config.ProviderName,
		ProviderType: config.ProviderType,
		BaseURL:      config.BaseURL,
		Model:        config.Model,
		APIKey:       apiKey,
	}, nil
}

func (s *Service) normalizeRuntimeInput(input UpdateConfigInput) (RuntimeConfig, error) {
	providerType := chooseString(strings.TrimSpace(input.ProviderType), DefaultProviderType)
	if providerType != DefaultProviderType {
		return RuntimeConfig{}, ErrInvalidConfig
	}

	baseURL := strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	if err := ValidateBaseURL(baseURL, s.allowInsecurePrivateURL); err != nil {
		return RuntimeConfig{}, err
	}

	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = DefaultModel
	}

	providerName := chooseString(strings.TrimSpace(input.ProviderName), DefaultProviderName)

	return RuntimeConfig{
		ProviderName: providerName,
		ProviderType: providerType,
		BaseURL:      baseURL,
		Model:        model,
	}, nil
}

func summaryFromStored(config StoredConfig) ConfigSummary {
	return ConfigSummary{
		Bound:         true,
		ProviderName:  chooseString(config.ProviderName, DefaultProviderName),
		ProviderType:  chooseString(config.ProviderType, DefaultProviderType),
		BaseURL:       config.BaseURL,
		Model:         chooseString(config.Model, DefaultModel),
		KeyConfigured: len(config.Ciphertext) > 0 && len(config.Nonce) > 0,
		UpdatedAt:     &config.UpdatedAt,
	}
}

func normalizeMessages(messages []ChatMessage) ([]ChatMessage, error) {
	if len(messages) == 0 {
		return nil, ErrInvalidInput
	}
	if len(messages) > 40 {
		messages = messages[len(messages)-40:]
	}

	totalChars := 0
	normalized := make([]ChatMessage, 0, len(messages))
	for _, message := range messages {
		role := strings.TrimSpace(message.Role)
		content := strings.TrimSpace(message.Content)
		if role != "system" && role != "user" && role != "assistant" {
			return nil, ErrInvalidInput
		}
		if content == "" {
			return nil, ErrInvalidInput
		}

		totalChars += len(content)
		if totalChars > 24000 {
			return nil, ErrInvalidInput
		}

		normalized = append(normalized, ChatMessage{
			Role:    role,
			Content: content,
		})
	}

	return normalized, nil
}

func (s *Service) String() string {
	return "ai.Service"
}

var _ AIService = (*Service)(nil)
