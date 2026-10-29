package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ZepClient implements the Zep Community Edition v1.0.2 HTTP API.
type ZepClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewZepClient creates a new Zep client
func NewZepClient(baseURL, apiKey string, timeout time.Duration) *ZepClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &ZepClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout:       timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

type zepMessage struct {
	RoleType string         `json:"role_type"`
	Role     string         `json:"role"`
	Content  string         `json:"content"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type zepAddMessageRequest struct {
	Messages []zepMessage `json:"messages"`
}

type zepFact struct {
	Fact      string    `json:"fact"`
	CreatedAt time.Time `json:"created_at"`
}

type zepMemoryResponse struct {
	RelevantFacts []zepFact `json:"relevant_facts"`
}

type zepSearchResult struct {
	Fact *zepFact `json:"fact"`
}

type zepSearchResponse struct {
	Results []zepSearchResult `json:"results"`
}

func (z *ZepClient) AddMessage(ctx context.Context, sessionID string, message Message) error {
	url := fmt.Sprintf("%s/api/v2/sessions/%s/memory", z.baseURL, url.PathEscape(sessionID))

	req := zepAddMessageRequest{
		Messages: []zepMessage{
			{
				Role:     message.Role,
				RoleType: message.Role,
				Content:  message.Content,
				Metadata: message.Metadata,
			},
		},
	}

	return z.doRequest(ctx, "POST", url, req, nil)
}

type zepCreateSessionRequest struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
}

// EnsureSession provisions the user before creating a CE v1.0.2 session.
// Duplicate creates return 400; only a successful reread confirms a race.
func (z *ZepClient) EnsureSession(ctx context.Context, sessionID, userID string) error {
	if userID == "" || sessionID == "" {
		return fmt.Errorf("missing zep user or session id")
	}
	endpoint := fmt.Sprintf("%s/api/v2/sessions/%s", z.baseURL, url.PathEscape(sessionID))
	var session zepCreateSessionRequest
	err := z.doRequest(ctx, http.MethodGet, endpoint, nil, &session)
	if err == nil {
		return verifySession(session, sessionID, userID)
	}
	if !isZepStatus(err, http.StatusNotFound) {
		return err
	}
	if err := z.ensureUser(ctx, userID); err != nil {
		return err
	}
	err = z.doRequest(ctx, http.MethodPost, z.baseURL+"/api/v2/sessions", zepCreateSessionRequest{SessionID: sessionID, UserID: userID}, nil)
	if err == nil {
		return nil
	}
	if !isZepStatus(err, http.StatusBadRequest) && !isZepStatus(err, http.StatusConflict) {
		return err
	}
	if readErr := z.doRequest(ctx, http.MethodGet, endpoint, nil, &session); readErr != nil {
		return err
	}
	return verifySession(session, sessionID, userID)
}

func verifySession(session zepCreateSessionRequest, sessionID, userID string) error {
	if session.SessionID != sessionID || session.UserID != userID {
		return fmt.Errorf("zep session does not belong to the requested user")
	}
	return nil
}

type zepUser struct {
	UserID string `json:"user_id"`
}

func (z *ZepClient) ensureUser(ctx context.Context, userID string) error {
	endpoint := fmt.Sprintf("%s/api/v2/users/%s", z.baseURL, url.PathEscape(userID))
	var user zepUser
	err := z.doRequest(ctx, http.MethodGet, endpoint, nil, &user)
	if err == nil {
		if user.UserID != userID {
			return fmt.Errorf("zep returned an unexpected user")
		}
		return nil
	}
	if !isZepStatus(err, http.StatusNotFound) {
		return err
	}
	err = z.doRequest(ctx, http.MethodPost, z.baseURL+"/api/v2/users", zepUser{UserID: userID}, nil)
	if err == nil {
		return nil
	}
	if !isZepStatus(err, http.StatusBadRequest) && !isZepStatus(err, http.StatusConflict) {
		return err
	}
	if readErr := z.doRequest(ctx, http.MethodGet, endpoint, nil, &user); readErr != nil {
		return err
	}
	if user.UserID != userID {
		return fmt.Errorf("zep returned an unexpected user")
	}
	return nil
}

func isZepStatus(err error, code int) bool {
	statusErr, ok := errors.AsType[*zepStatusError](err)
	return ok && statusErr.StatusCode == code
}

// GetSessionSummary adapts CE v1.0.2 relevant facts to the context interface.
// This API does not return a prose conversation summary.
func (z *ZepClient) GetSessionSummary(ctx context.Context, sessionID string) (*SessionSummary, error) {
	url := fmt.Sprintf("%s/api/v2/sessions/%s/memory", z.baseURL, url.PathEscape(sessionID))

	var resp zepMemoryResponse
	if err := z.doRequest(ctx, "GET", url, nil, &resp); err != nil {
		return nil, err
	}

	facts := make([]string, 0, len(resp.RelevantFacts))
	for _, fact := range resp.RelevantFacts {
		if content := strings.TrimSpace(fact.Fact); content != "" {
			facts = append(facts, content)
		}
	}
	if len(facts) == 0 {
		return nil, nil
	}
	return &SessionSummary{Summary: strings.Join(facts, "\n"), UpdatedAt: time.Now()}, nil
}

func (z *ZepClient) SearchMemories(ctx context.Context, userID, query string, limit int) ([]Memory, error) {
	if userID == "" || limit <= 0 {
		return nil, nil
	}
	url := fmt.Sprintf("%s/api/v2/sessions/search?limit=%d", z.baseURL, limit)

	reqBody := map[string]any{
		"text":    query,
		"user_id": userID,
	}

	var resp zepSearchResponse
	if err := z.doRequest(ctx, "POST", url, reqBody, &resp); err != nil {
		return nil, err
	}

	memories := make([]Memory, 0, len(resp.Results))
	for _, r := range resp.Results {
		if r.Fact == nil || strings.TrimSpace(r.Fact.Fact) == "" {
			continue
		}
		memories = append(memories, Memory{
			Content:   r.Fact.Fact,
			CreatedAt: r.Fact.CreatedAt,
		})
		if len(memories) == limit {
			break
		}
	}

	return memories, nil
}

// IsAvailable probes liveness only; CE healthz does not validate API credentials.
func (z *ZepClient) IsAvailable(ctx context.Context) error {
	url := fmt.Sprintf("%s/healthz", z.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("create health check request: %w", err)
	}

	resp, err := z.httpClient.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ErrUnavailable
	}

	return nil
}

func (z *ZepClient) doRequest(ctx context.Context, method, url string, reqBody, respBody any) error {
	var bodyReader io.Reader
	if reqBody != nil {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if z.apiKey != "" {
		req.Header.Set("Authorization", "Api-Key "+z.apiKey)
	}

	resp, err := z.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &zepStatusError{StatusCode: resp.StatusCode}
	}

	// Zep answers message appends and session creation with 201, so accept any
	// 2xx rather than only 200. Failures below are reported before a body can
	// decode as a zero value, which would look like "no memories".
	if respBody != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(respBody); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

type zepStatusError struct {
	StatusCode int
}

func (e *zepStatusError) Error() string {
	return fmt.Sprintf("zep request failed: status %d", e.StatusCode)
}
