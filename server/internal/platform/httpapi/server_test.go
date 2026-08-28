package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"log/slog"

	"github.com/ruhuang/ink/server/internal/ai"
	"github.com/ruhuang/ink/server/internal/auth"
	"github.com/ruhuang/ink/server/internal/feedback"
	"github.com/ruhuang/ink/server/internal/pluginfetch"
	"github.com/ruhuang/ink/server/internal/plugins"
	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/schedule"
	"github.com/ruhuang/ink/server/internal/workspace"
)

func newTestServer(
	authService fakeAuthService,
	workspaceService fakeWorkspaceService,
	aiService fakeAIService,
	printerService fakePrinterService,
	feedbackService fakeFeedbackService,
	pluginService fakePluginService,
	pluginRunService fakePluginRunService,
	scheduleService fakeScheduleService,
) *Server {
	return NewServer(
		authService,
		workspaceService,
		aiService,
		printerService,
		feedbackService,
		pluginService,
		pluginRunService,
		scheduleService,
		slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil)),
		time.Minute,
		5,
		100,
		nil,
		"",
		32<<20,
	)
}

func TestLoginHandlerReturnsTokens(t *testing.T) {
	server := newTestServer(fakeAuthService{
		loginResult: auth.AuthResult{
			User: auth.UserDTO{
				ID:    "user-1",
				Email: "name@example.com",
				Name:  "Ink User",
				Role:  "member",
			},
			Token: auth.TokenPair{
				AccessToken:          "access-token",
				RefreshToken:         "refresh-token",
				AccessTokenExpiresAt: time.Now().UTC().Add(15 * time.Minute),
			},
		},
	}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{}, fakePluginService{}, fakePluginRunService{}, fakeScheduleService{})

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"name@example.com","password":"demo-password"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload["accessToken"] != "access-token" {
		t.Fatalf("expected access token in response")
	}
}

func TestJSONRequestBoundaries(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{name: "unknown field", body: `{"email":"name@example.com","password":"secret","extra":true}`, want: http.StatusBadRequest},
		{name: "trailing JSON", body: `{"email":"name@example.com","password":"secret"} {}`, want: http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(fakeAuthService{}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{}, fakePluginService{}, fakePluginRunService{}, fakeScheduleService{})
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(test.body))
			response := httptest.NewRecorder()

			server.Handler().ServeHTTP(response, request)

			if response.Code != test.want {
				t.Fatalf("expected %d, got %d", test.want, response.Code)
			}
		})
	}
}

func TestOversizedJSONDoesNotCallService(t *testing.T) {
	calls := 0
	server := newTestServer(fakeAuthService{loginCalls: &calls}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{}, fakePluginService{}, fakePluginRunService{}, fakeScheduleService{})
	body := `{"email":"name@example.com","password":"` + string(bytes.Repeat([]byte("x"), int(defaultJSONMaxBytes))) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", response.Code)
	}
	if calls != 0 {
		t.Fatalf("expected auth service not to be called, got %d calls", calls)
	}
	var payload errorEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code != "request_too_large" {
		t.Fatalf("expected request_too_large, got %q", payload.Code)
	}
}

func TestManualPluginRunAllowsEmptyBody(t *testing.T) {
	calls := 0
	server := newTestServer(fakeAuthService{}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{}, fakePluginService{}, fakePluginRunService{calls: &calls}, fakeScheduleService{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/plugin-1/run", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if calls != 1 {
		t.Fatalf("expected one plugin run, got %d", calls)
	}
}

func TestMeRequiresBearerToken(t *testing.T) {
	server := newTestServer(fakeAuthService{}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{}, fakePluginService{}, fakePluginRunService{}, fakeScheduleService{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestLoginRateLimit(t *testing.T) {
	server := NewServer(fakeAuthService{
		loginResult: auth.AuthResult{
			User: auth.UserDTO{
				ID:    "user-1",
				Email: "name@example.com",
				Name:  "Ink User",
				Role:  "member",
			},
			Token: auth.TokenPair{
				AccessToken:          "access-token",
				RefreshToken:         "refresh-token",
				AccessTokenExpiresAt: time.Now().UTC().Add(15 * time.Minute),
			},
		},
	}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{}, fakePluginService{}, fakePluginRunService{}, fakeScheduleService{}, slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil)), time.Minute, 1, 100, nil, "", 32<<20)

	first := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"name@example.com","password":"demo-password"}`))
	first.RemoteAddr = "127.0.0.1:1234"
	second := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"name@example.com","password":"demo-password"}`))
	second.RemoteAddr = "127.0.0.1:1234"
	firstResponse := httptest.NewRecorder()
	secondResponse := httptest.NewRecorder()

	server.Handler().ServeHTTP(firstResponse, first)
	server.Handler().ServeHTTP(secondResponse, second)

	if firstResponse.Code != http.StatusOK {
		t.Fatalf("expected first login to succeed, got %d", firstResponse.Code)
	}
	if secondResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second login to be rate limited, got %d", secondResponse.Code)
	}
}

func TestLoginRateLimitUsesAccountAndIPDimensions(t *testing.T) {
	newServer := func() *Server {
		server := newTestServer(fakeAuthService{
			loginResult: auth.AuthResult{
				Token: auth.TokenPair{AccessTokenExpiresAt: time.Now().UTC().Add(time.Minute)},
			},
		}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{}, fakePluginService{}, fakePluginRunService{}, fakeScheduleService{})
		server.rateLimiter = NewLoginRateLimiter(time.Minute, 1, 100)
		return server
	}
	login := func(server *Server, address string, email string) int {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"email":"`+email+`","password":"demo-password"}`))
		request.RemoteAddr = address
		response := httptest.NewRecorder()
		server.Handler().ServeHTTP(response, request)
		return response.Code
	}

	accountServer := newServer()
	if got := login(accountServer, "192.0.2.1:1001", "same@example.com"); got != http.StatusOK {
		t.Fatalf("first account attempt status = %d, want 200", got)
	}
	if got := login(accountServer, "192.0.2.2:1002", "same@example.com"); got != http.StatusTooManyRequests {
		t.Fatalf("same account from another IP status = %d, want 429", got)
	}

	ipServer := newServer()
	if got := login(ipServer, "192.0.2.3:1003", "first@example.com"); got != http.StatusOK {
		t.Fatalf("first IP attempt status = %d, want 200", got)
	}
	if got := login(ipServer, "192.0.2.3:1004", "second@example.com"); got != http.StatusTooManyRequests {
		t.Fatalf("same IP with another account status = %d, want 429", got)
	}
}

func TestRequestIPOnlyTrustsConfiguredProxyChain(t *testing.T) {
	trusted := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("2001:db8:ffff::/48"),
	}
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		xForwarded string
		headerType string
		want       string
	}{
		{
			name:       "untrusted peer cannot spoof X-Forwarded-For",
			remoteAddr: "198.51.100.10:443",
			xForwarded: "203.0.113.10",
			headerType: "x-forwarded-for",
			want:       "198.51.100.10",
		},
		{
			name:       "trusted proxy uses first untrusted hop from the right",
			remoteAddr: "10.0.0.4:443",
			xForwarded: "203.0.113.20, 10.0.0.3",
			headerType: "x-forwarded-for",
			want:       "203.0.113.20",
		},
		{
			name:       "trusted proxy parses standard Forwarded IPv6",
			remoteAddr: "[2001:db8:ffff::2]:443",
			forwarded:  `for="[2001:db8:1::20]:1234";proto=https`,
			headerType: "forwarded",
			want:       "2001:db8:1::20",
		},
		{
			name:       "malformed trusted header falls back to peer",
			remoteAddr: "10.0.0.4:443",
			xForwarded: "not-an-ip",
			headerType: "x-forwarded-for",
			want:       "10.0.0.4",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = test.remoteAddr
			request.Header.Set("Forwarded", test.forwarded)
			request.Header.Set("X-Forwarded-For", test.xForwarded)
			if got := requestIP(request, trusted, test.headerType); got != test.want {
				t.Fatalf("requestIP() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRequestIPUsesOnlyConfiguredHeaderAndAllFieldLines(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.4:443"
	request.Header.Set("Forwarded", "for=192.0.2.99")
	request.Header.Add("X-Forwarded-For", "203.0.113.30")
	request.Header.Add("X-Forwarded-For", "10.0.0.3")

	if got := requestIP(request, trusted, "x-forwarded-for"); got != "203.0.113.30" {
		t.Fatalf("requestIP() = %q, want configured X-Forwarded-For client", got)
	}
}

func TestMalformedForwardedHeaderFallsBackToPeer(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	tests := []string{
		"for=203.0.113.20;for=192.0.2.1",
		"for=203.0.113.20;proto",
		`for="203.0.113.20`,
		`for="203.0.113.20:invalid"`,
		`for="203.0.113.20"trailing`,
	}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = "10.0.0.4:443"
			request.Header.Set("Forwarded", value)
			if got := requestIP(request, trusted, "forwarded"); got != "10.0.0.4" {
				t.Fatalf("requestIP() = %q, want direct peer for malformed Forwarded", got)
			}
		})
	}
}

func TestForwardedParserHandlesQuotedSeparators(t *testing.T) {
	chain, ok := parseForwardedHeader(`for=203.0.113.20;host="example.com,8080";by="proxy;edge"`)
	if !ok || len(chain) != 1 || chain[0].String() != "203.0.113.20" {
		t.Fatalf("parseForwardedHeader() = (%v, %v), want one valid client", chain, ok)
	}
}

func TestLoginRateLimiterExpiresAndBoundsEntries(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	limiter := NewLoginRateLimiter(time.Minute, 1, 4)
	limiter.now = func() time.Time { return now }

	if !limiter.Allow("ip:1", "account:1") {
		t.Fatal("first attempt should be allowed")
	}
	if limiter.Allow("ip:1", "account:1") {
		t.Fatal("attempt inside the window should be denied")
	}
	if !limiter.Allow("ip:2", "account:2") {
		t.Fatal("independent dimensions should be allowed")
	}
	if limiter.Allow("ip:3", "account:3") {
		t.Fatal("new dimensions should be denied when active storage is full")
	}
	if len(limiter.hits) > limiter.maxEntries {
		t.Fatalf("entry count = %d, exceeds max %d", len(limiter.hits), limiter.maxEntries)
	}
	if limiter.Allow("ip:1", "account:1") {
		t.Fatal("capacity pressure must not evict an active saturated account")
	}

	now = now.Add(time.Minute + time.Second)
	if !limiter.Allow("ip:3", "account:3") {
		t.Fatal("attempt should be allowed after the window expires")
	}
	if len(limiter.hits) != 2 {
		t.Fatalf("expired entries were not cleaned up: got %d, want 2", len(limiter.hits))
	}
}

func TestChangePasswordReturnsNoContent(t *testing.T) {
	server := newTestServer(fakeAuthService{}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{}, fakePluginService{}, fakePluginRunService{}, fakeScheduleService{})
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/change-password",
		bytes.NewBufferString(`{"currentPassword":"demo-password","newPassword":"next-password"}`),
	)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", response.Code)
	}
}

func TestCreateUserRequiresAdminAuthorization(t *testing.T) {
	server := newTestServer(
		fakeAuthService{
			createUserResult: auth.UserDTO{
				ID:    "user-2",
				Email: "new-user",
				Name:  "New User",
				Role:  "member",
			},
		},
		fakeWorkspaceService{},
		fakeAIService{},
		fakePrinterService{},
		fakeFeedbackService{},
		fakePluginService{},
		fakePluginRunService{},
		fakeScheduleService{},
	)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/users",
		bytes.NewBufferString(`{"email":"new-user","name":"New User","password":"demo-password"}`),
	)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", response.Code)
	}
}

func TestWorkspaceHandlersRequireAuthorization(t *testing.T) {
	server := newTestServer(fakeAuthService{}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{}, fakePluginService{}, fakePluginRunService{}, fakeScheduleService{})

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/workspace", nil)
	getResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(getResponse, getRequest)

	if getResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected get workspace to require auth, got %d", getResponse.Code)
	}

	putRequest := httptest.NewRequest(http.MethodPut, "/api/v1/workspace", bytes.NewBufferString(`{}`))
	putResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(putResponse, putRequest)

	if putResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected save workspace to require auth, got %d", putResponse.Code)
	}
}

func TestAIConfigRequiresAuthorization(t *testing.T) {
	server := newTestServer(fakeAuthService{}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{}, fakePluginService{}, fakePluginRunService{}, fakeScheduleService{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/ai/config", nil)
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestListPrintersReturnsDevices(t *testing.T) {
	server := newTestServer(
		fakeAuthService{},
		fakeWorkspaceService{},
		fakeAIService{},
		fakePrinterService{
			devices: []workspace.Device{
				{ID: "device-1", Name: "书桌咕咕机", Status: "connected", Note: "默认设备"},
			},
		},
		fakeFeedbackService{},
		fakePluginService{},
		fakePluginRunService{},
		fakeScheduleService{},
	)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/printers", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
}

type fakeAuthService struct {
	loginResult       auth.AuthResult
	loginErr          error
	loginCalls        *int
	changePasswordErr error
	createUserResult  auth.UserDTO
	createUserErr     error
}

func (f fakeAuthService) Login(_ context.Context, _ auth.LoginInput) (auth.AuthResult, error) {
	if f.loginCalls != nil {
		*f.loginCalls++
	}
	return f.loginResult, f.loginErr
}

func (f fakeAuthService) Refresh(_ context.Context, _ string, _ auth.ClientMeta) (auth.AuthResult, error) {
	return auth.AuthResult{}, nil
}

func (f fakeAuthService) Logout(_ context.Context, _ string, _ string) error {
	return nil
}

func (f fakeAuthService) GetCurrentUser(_ context.Context, _ string) (auth.UserDTO, error) {
	return auth.UserDTO{
		ID:    "user-1",
		Email: "name@example.com",
		Name:  "Ink User",
		Role:  "admin",
	}, nil
}

func (f fakeAuthService) ChangePassword(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	_ auth.ClientMeta,
) error {
	return f.changePasswordErr
}

func (f fakeAuthService) CreateUser(
	_ context.Context,
	_ string,
	_ auth.CreateUserInput,
) (auth.UserDTO, error) {
	return f.createUserResult, f.createUserErr
}

type fakeWorkspaceService struct {
	state workspace.State
	err   error
}

func (f fakeWorkspaceService) GetState(_ context.Context, _ string) (workspace.State, error) {
	return f.state, f.err
}

func (f fakeWorkspaceService) SaveState(
	_ context.Context,
	_ string,
	state workspace.State,
) (workspace.State, error) {
	if f.err != nil {
		return workspace.State{}, f.err
	}
	return state, nil
}

var _ auth.AuthService = fakeAuthService{}
var _ workspace.WorkspaceService = fakeWorkspaceService{}

type fakeAIService struct {
	summary ai.ConfigSummary
	reply   ai.ReplyResult
	err     error
}

func (f fakeAIService) GetConfigSummary(_ context.Context, _ string) (ai.ConfigSummary, error) {
	return f.summary, f.err
}

func (f fakeAIService) UpdateSystemConfig(_ context.Context, _ string, _ ai.UpdateConfigInput) (ai.ConfigSummary, error) {
	return f.summary, f.err
}

func (f fakeAIService) GenerateReply(_ context.Context, _ string, _ ai.ReplyInput) (ai.ReplyResult, error) {
	return f.reply, f.err
}

type fakePrinterService struct {
	devices   []workspace.Device
	printJobs []workspace.PrintJob
	err       error
}

func (f fakePrinterService) RenderPreview(_ context.Context, _ string, _ string) (string, error) {
	return "iVBORw0KGgo=", f.err
}

func (f fakePrinterService) RenderBlocksPreview(_ context.Context, _ string, _ []plugins.ContentBlock) (string, error) {
	return "iVBORw0KGgo=", f.err
}

func (f fakePrinterService) ListDevices(_ context.Context, _ string) ([]workspace.Device, error) {
	return f.devices, f.err
}

func (f fakePrinterService) BindDevice(_ context.Context, _ string, _ printer.BindInput) (workspace.Device, error) {
	if f.err != nil {
		return workspace.Device{}, f.err
	}
	if len(f.devices) > 0 {
		return f.devices[0], nil
	}
	return workspace.Device{}, nil
}

func (f fakePrinterService) DeleteDevice(_ context.Context, _ string, _ string) error {
	return f.err
}

func (f fakePrinterService) ListPrintJobs(_ context.Context, _ string) ([]workspace.PrintJob, error) {
	return f.printJobs, f.err
}

func (f fakePrinterService) CreatePrintJob(_ context.Context, _ string, _ printer.CreateJobInput) (workspace.PrintJob, error) {
	if f.err != nil {
		return workspace.PrintJob{}, f.err
	}
	if len(f.printJobs) > 0 {
		return f.printJobs[0], nil
	}
	return workspace.PrintJob{}, nil
}

func (f fakePrinterService) SubmitPrintJob(_ context.Context, _ string, _ string) (workspace.PrintJob, error) {
	if f.err != nil {
		return workspace.PrintJob{}, f.err
	}
	if len(f.printJobs) > 0 {
		return f.printJobs[0], nil
	}
	return workspace.PrintJob{}, nil
}

func (f fakePrinterService) CancelPrintJob(_ context.Context, _ string, _ string) (workspace.PrintJob, error) {
	if f.err != nil {
		return workspace.PrintJob{}, f.err
	}
	if len(f.printJobs) > 0 {
		return f.printJobs[0], nil
	}
	return workspace.PrintJob{}, nil
}

func (f fakePrinterService) UpdatePrintJobDevice(_ context.Context, _ string, _ string, _ printer.UpdateJobDeviceInput) (workspace.PrintJob, error) {
	if f.err != nil {
		return workspace.PrintJob{}, f.err
	}
	if len(f.printJobs) > 0 {
		return f.printJobs[0], nil
	}
	return workspace.PrintJob{}, nil
}

var _ ai.AIService = fakeAIService{}
var _ printer.PrinterService = fakePrinterService{}

type fakeFeedbackService struct {
	err error
}

func (f fakeFeedbackService) Submit(_ context.Context, _ string, _ feedback.SubmitInput) error {
	return f.err
}

var _ FeedbackService = fakeFeedbackService{}

type fakePluginService struct {
	items  []plugins.PluginDetails
	result plugins.PluginDetails
	test   plugins.ValidationResult
	err    error
}

func (f fakePluginService) ListAdminInstallations(_ context.Context, _ string) ([]plugins.PluginDetails, error) {
	return f.items, f.err
}

func (f fakePluginService) UploadPlugin(_ context.Context, _ string, _ string, _ io.Reader) (plugins.PluginDetails, error) {
	return f.result, f.err
}

func (f fakePluginService) InstallFromGit(_ context.Context, _ string, _ plugins.GitInstallInput) (plugins.PluginDetails, error) {
	return f.result, f.err
}

func (f fakePluginService) DisableInstallation(_ context.Context, _ string, _ string) (plugins.PluginDetails, error) {
	return f.result, f.err
}

func (f fakePluginService) ListUserPlugins(_ context.Context, _ string) ([]plugins.PluginDetails, error) {
	return f.items, f.err
}

func (f fakePluginService) GetUserPlugin(_ context.Context, _ string, _ string) (plugins.PluginDetails, error) {
	return f.result, f.err
}

func (f fakePluginService) SaveBinding(_ context.Context, _ string, _ string, _ plugins.BindingInput) (plugins.PluginDetails, error) {
	return f.result, f.err
}

func (f fakePluginService) TestBinding(_ context.Context, _ string, _ string, _ plugins.BindingInput) (plugins.ValidationResult, error) {
	return f.test, f.err
}

type fakePluginRunService struct {
	result pluginfetch.ManualRunResult
	err    error
	calls  *int
}

func (f fakePluginRunService) RunManual(_ context.Context, _ string, _ string) (pluginfetch.ManualRunResult, error) {
	if f.calls != nil {
		*f.calls++
	}
	return f.result, f.err
}

type fakeScheduleService struct {
	items []schedule.ScheduleView
	item  schedule.ScheduleView
	err   error
}

func (f fakeScheduleService) List(_ context.Context, _ string) ([]schedule.ScheduleView, error) {
	return f.items, f.err
}

func (f fakeScheduleService) Create(_ context.Context, _ string, _ schedule.UpsertInput) (schedule.ScheduleView, error) {
	return f.item, f.err
}

func (f fakeScheduleService) Update(_ context.Context, _ string, _ string, _ schedule.UpsertInput) (schedule.ScheduleView, error) {
	return f.item, f.err
}

func (f fakeScheduleService) Toggle(_ context.Context, _ string, _ string) (schedule.ScheduleView, error) {
	return f.item, f.err
}

func (f fakeScheduleService) RunNow(_ context.Context, _ string, _ string) (schedule.ManualPrintResult, error) {
	return schedule.ManualPrintResult{}, nil
}

func (f fakeScheduleService) Delete(_ context.Context, _ string, _ string) error {
	return f.err
}

var _ PluginService = fakePluginService{}
var _ PluginRunService = fakePluginRunService{}
var _ ScheduleService = fakeScheduleService{}

func TestSubmitFeedbackRequiresAuthorization(t *testing.T) {
	server := newTestServer(fakeAuthService{}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{}, fakePluginService{}, fakePluginRunService{}, fakeScheduleService{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/feedback/print", bytes.NewBufferString(`{"content":"hello"}`))
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestSubmitFeedbackReturnsNoContent(t *testing.T) {
	server := newTestServer(
		fakeAuthService{},
		fakeWorkspaceService{},
		fakeAIService{},
		fakePrinterService{},
		fakeFeedbackService{},
		fakePluginService{},
		fakePluginRunService{},
		fakeScheduleService{},
	)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/feedback/print", bytes.NewBufferString(`{"content":"hello"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()

	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", response.Code)
	}
}
