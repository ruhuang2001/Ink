package memory

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/auth"
)

type fakeRepo struct {
	mu       sync.Mutex
	settings *StoredSettings
	saved    []StoredSettings
}

func (r *fakeRepo) GetMemorySettings(context.Context) (*StoredSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.settings == nil {
		return nil, nil
	}
	copied := *r.settings
	return &copied, nil
}

func (r *fakeRepo) SaveMemorySettings(_ context.Context, settings StoredSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saved = append(r.saved, settings)
	saved := settings
	r.settings = &saved
	return nil
}

type fakeAuth struct {
	user auth.UserDTO
	err  error
}

func (a fakeAuth) GetCurrentUser(context.Context, string) (auth.UserDTO, error) {
	if a.err != nil {
		return auth.UserDTO{}, a.err
	}
	return a.user, nil
}

type fakeEncryptor struct {
	decryptErr error
}

func (fakeEncryptor) Encrypt(plaintext string) ([]byte, []byte, error) {
	return []byte("cipher:" + plaintext), []byte("nonce"), nil
}

func (e fakeEncryptor) Decrypt(ciphertext []byte, _ []byte) (string, error) {
	if e.decryptErr != nil {
		return "", e.decryptErr
	}
	return strings.TrimPrefix(string(ciphertext), "cipher:"), nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func adminUser() auth.UserDTO {
	return auth.UserDTO{ID: "user_admin", Role: "admin"}
}

func memberUser() auth.UserDTO {
	return auth.UserDTO{ID: "user_member", Role: "member"}
}

func TestGetSettingsHidesBaseURLFromNonAdmins(t *testing.T) {
	repo := &fakeRepo{settings: &StoredSettings{
		Enabled:    false,
		ZepBaseURL: "http://zep:8000",
		UpdatedAt:  time.Now(),
	}}
	service := NewService(repo, fakeAuth{user: memberUser()}, fakeEncryptor{}, fixedClock{now: time.Now()}, false)

	summary, err := service.GetSettings(context.Background(), "token")
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if summary.ZepBaseURL != "" {
		t.Fatalf("expected base URL to be redacted, got %q", summary.ZepBaseURL)
	}
}

func TestGetSettingsReturnsBaseURLToAdmins(t *testing.T) {
	repo := &fakeRepo{settings: &StoredSettings{
		ZepBaseURL: "https://zep.example.com",
		UpdatedAt:  time.Now(),
	}}
	service := NewService(repo, fakeAuth{user: adminUser()}, fakeEncryptor{}, fixedClock{now: time.Now()}, false)

	summary, err := service.GetSettings(context.Background(), "token")
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if summary.ZepBaseURL != "https://zep.example.com" {
		t.Fatalf("expected base URL for admin, got %q", summary.ZepBaseURL)
	}
}

func TestUpdateSettingsRequiresAdmin(t *testing.T) {
	service := NewService(&fakeRepo{}, fakeAuth{user: memberUser()}, fakeEncryptor{}, fixedClock{now: time.Now()}, false)

	_, err := service.UpdateSettings(context.Background(), "token", UpdateSettingsInput{Enabled: true})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestUpdateSettingsRejectsPrivateZepURLByDefault(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo, fakeAuth{user: adminUser()}, fakeEncryptor{}, fixedClock{now: time.Now()}, false)

	_, err := service.UpdateSettings(context.Background(), "token", UpdateSettingsInput{
		Enabled:    true,
		ZepBaseURL: "http://169.254.169.254/",
	})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
	if len(repo.saved) != 0 {
		t.Fatalf("rejected config must not be persisted, saved=%d", len(repo.saved))
	}
}

func TestUpdateSettingsAllowsPrivateZepURLWhenOptedIn(t *testing.T) {
	repo := &fakeRepo{}
	service := NewService(repo, fakeAuth{user: adminUser()}, fakeEncryptor{}, fixedClock{now: time.Now()}, true)

	if _, err := service.UpdateSettings(context.Background(), "token", UpdateSettingsInput{
		Enabled:    true,
		ZepBaseURL: "http://zep:8000/",
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if len(repo.saved) != 1 || repo.saved[0].ZepBaseURL != "http://zep:8000" {
		t.Fatalf("expected normalized base URL persisted, got %+v", repo.saved)
	}
}

func TestUpdateSettingsReportsUndecryptableKey(t *testing.T) {
	repo := &fakeRepo{settings: &StoredSettings{
		Ciphertext: []byte("cipher:old-key"),
		Nonce:      []byte("nonce"),
		UpdatedAt:  time.Now(),
	}}
	service := NewService(repo, fakeAuth{user: adminUser()}, fakeEncryptor{decryptErr: errors.New("bad key")}, fixedClock{now: time.Now()}, true)

	_, err := service.UpdateSettings(context.Background(), "token", UpdateSettingsInput{
		Enabled:    true,
		ZepBaseURL: "http://zep:8000",
	})
	if !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("expected ErrDecryptFailed, got %v", err)
	}
	if len(repo.saved) != 0 {
		t.Fatal("undecryptable key must not be persisted")
	}
}

func TestKeyConfiguredReflectsDecryptability(t *testing.T) {
	settings := StoredSettings{Ciphertext: []byte("cipher:key"), Nonce: []byte("nonce")}

	good := NewService(&fakeRepo{}, fakeAuth{}, fakeEncryptor{}, fixedClock{now: time.Now()}, false)
	if !good.keyConfigured(settings) {
		t.Fatal("expected key to be reported as configured")
	}

	broken := NewService(&fakeRepo{}, fakeAuth{}, fakeEncryptor{decryptErr: errors.New("bad key")}, fixedClock{now: time.Now()}, false)
	if broken.keyConfigured(settings) {
		t.Fatal("undecryptable key must not be reported as configured")
	}
}

type stubClient struct {
	mu        sync.Mutex
	probes    int
	available bool
}

func (c *stubClient) EnsureSession(context.Context, string, string) error { return nil }
func (c *stubClient) AddMessage(context.Context, string, Message) error   { return nil }
func (c *stubClient) GetSessionSummary(context.Context, string) (*SessionSummary, error) {
	return nil, nil
}
func (c *stubClient) SearchMemories(context.Context, string, string, int) ([]Memory, error) {
	return nil, nil
}
func (c *stubClient) IsAvailable(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.probes++
	if c.available {
		return nil
	}
	return ErrUnavailable
}

func TestAvailabilityProbeIsCached(t *testing.T) {
	client := &stubClient{available: true}
	now := time.Now()
	service := NewService(&fakeRepo{}, fakeAuth{}, fakeEncryptor{}, fixedClock{now: now}, false)
	service.setClient(client)

	for i := 0; i < 5; i++ {
		if !service.availability(context.Background()) {
			t.Fatal("expected available")
		}
	}

	client.mu.Lock()
	probes := client.probes
	client.mu.Unlock()
	if probes != 1 {
		t.Fatalf("expected a single probe within the TTL, got %d", probes)
	}
}

func TestClientUpdateIsRaceFree(t *testing.T) {
	service := NewService(&fakeRepo{}, fakeAuth{user: adminUser()}, fakeEncryptor{}, fixedClock{now: time.Now()}, false)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			service.setClient(&stubClient{})
		}()
		go func() {
			defer wg.Done()
			_ = service.GetClient()
		}()
	}
	wg.Wait()
}

func TestZepClientDecodesCreatedResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"relevant_facts": []map[string]string{{"fact": "earlier context"}},
		})
	}))
	defer server.Close()

	client := NewZepClient(server.URL, "", time.Second)
	summary, err := client.GetSessionSummary(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("GetSessionSummary: %v", err)
	}
	if summary == nil || summary.Summary != "earlier context" {
		t.Fatalf("expected decoded summary, got %+v", summary)
	}
}

func TestZepEnsureSessionSendsUserAssociation(t *testing.T) {
	var payload map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/api/v2/users" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		if r.URL.Path != "/api/v2/sessions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewZepClient(server.URL, "secret", time.Second)
	if err := client.EnsureSession(context.Background(), "session-1", "user-1"); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	if payload["user_id"] != "user-1" || payload["session_id"] != "session-1" {
		t.Fatalf("unexpected payload %+v", payload)
	}
}

func TestZepEnsureSessionToleratesExistingSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"session_id": "session-1", "user_id": "user-1"})
	}))
	defer server.Close()

	client := NewZepClient(server.URL, "", time.Second)
	if err := client.EnsureSession(context.Background(), "session-1", "user-1"); err != nil {
		t.Fatalf("expected conflict to be tolerated, got %v", err)
	}
}

func TestEnhancerStoreConversationCreatesSession(t *testing.T) {
	client := &recordingClient{}
	enhancer := NewEnhancer(func() Client { return client })

	err := enhancer.StoreConversation(context.Background(), "user-1", "session-1",
		[]ChatMessage{{Role: "user", Content: "hello"}}, "hi there")
	if err != nil {
		t.Fatalf("StoreConversation: %v", err)
	}
	if client.sessionUserID != "user-1" || client.sessionID != sessionID("user-1", "session-1") {
		t.Fatalf("expected session created for user, got %q/%q", client.sessionID, client.sessionUserID)
	}
	if len(client.messages) != 2 {
		t.Fatalf("expected user and assistant messages, got %d", len(client.messages))
	}
}

type recordingClient struct {
	sessionID     string
	sessionUserID string
	messages      []Message
}

func (c *recordingClient) EnsureSession(_ context.Context, sessionID, userID string) error {
	c.sessionID = sessionID
	c.sessionUserID = userID
	return nil
}
func (c *recordingClient) AddMessage(_ context.Context, _ string, message Message) error {
	c.messages = append(c.messages, message)
	return nil
}
func (c *recordingClient) GetSessionSummary(context.Context, string) (*SessionSummary, error) {
	return nil, nil
}
func (c *recordingClient) SearchMemories(context.Context, string, string, int) ([]Memory, error) {
	return nil, nil
}
func (c *recordingClient) IsAvailable(context.Context) error { return nil }
