package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ruhuang/ink/server/internal/ai"
	"github.com/ruhuang/ink/server/internal/auth"
	"github.com/ruhuang/ink/server/internal/feedback"
	"github.com/ruhuang/ink/server/internal/pluginfetch"
	"github.com/ruhuang/ink/server/internal/plugins"
	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/schedule"
	"github.com/ruhuang/ink/server/internal/session"
	"github.com/ruhuang/ink/server/internal/workspace"
)

type PluginService interface {
	ListAdminInstallations(ctx context.Context, accessToken string) ([]plugins.PluginDetails, error)
	UploadPlugin(ctx context.Context, accessToken string, filename string, source io.Reader) (plugins.PluginDetails, error)
	InstallFromGit(ctx context.Context, accessToken string, input plugins.GitInstallInput) (plugins.PluginDetails, error)
	DisableInstallation(ctx context.Context, accessToken string, installationID string) (plugins.PluginDetails, error)
	ListUserPlugins(ctx context.Context, accessToken string) ([]plugins.PluginDetails, error)
	GetUserPlugin(ctx context.Context, accessToken string, installationID string) (plugins.PluginDetails, error)
	SaveBinding(ctx context.Context, accessToken string, installationID string, input plugins.BindingInput) (plugins.PluginDetails, error)
	TestBinding(ctx context.Context, accessToken string, installationID string, input plugins.BindingInput) (plugins.ValidationResult, error)
}

type PluginRunService interface {
	RunManual(ctx context.Context, accessToken string, installationID string) (pluginfetch.ManualRunResult, error)
}

type ScheduleService interface {
	List(ctx context.Context, accessToken string) ([]schedule.ScheduleView, error)
	Create(ctx context.Context, accessToken string, input schedule.UpsertInput) (schedule.ScheduleView, error)
	Update(ctx context.Context, accessToken string, scheduleID string, input schedule.UpsertInput) (schedule.ScheduleView, error)
	Toggle(ctx context.Context, accessToken string, scheduleID string) (schedule.ScheduleView, error)
	Delete(ctx context.Context, accessToken string, scheduleID string) error
	RunNow(ctx context.Context, accessToken string, scheduleID string) (schedule.ManualPrintResult, error)
}

type FeedbackService interface {
	Submit(ctx context.Context, accessToken string, input feedback.SubmitInput) error
}

const (
	defaultJSONMaxBytes   int64 = 1 << 20
	workspaceJSONMaxBytes int64 = 4 << 20

	pluginUploadMultipartMemory int64 = 32 << 10
)

// Server exposes the HTTP handlers for authentication endpoints.
type Server struct {
	auth                 auth.AuthService
	workspace            workspace.WorkspaceService
	ai                   ai.AIService
	printer              printer.PrinterService
	feedback             FeedbackService
	plugins              PluginService
	pluginRuns           PluginRunService
	schedules            ScheduleService
	logger               *slog.Logger
	rateLimiter          *LoginRateLimiter
	trustedProxyCIDRs    []netip.Prefix
	trustedProxyHeader   string
	pluginUploadMaxBytes int64
}

// NewServer wires the auth service, logger, and login rate limiter into an HTTP server.
func NewServer(
	authService auth.AuthService,
	workspaceService workspace.WorkspaceService,
	aiService ai.AIService,
	printerService printer.PrinterService,
	feedbackService FeedbackService,
	pluginService PluginService,
	pluginRunService PluginRunService,
	scheduleService ScheduleService,
	logger *slog.Logger,
	rateWindow time.Duration,
	rateMax int,
	rateMaxEntries int,
	trustedProxyCIDRs []netip.Prefix,
	trustedProxyHeader string,
	pluginUploadMaxBytes int64,
) *Server {
	return &Server{
		auth:                 authService,
		workspace:            workspaceService,
		ai:                   aiService,
		printer:              printerService,
		feedback:             feedbackService,
		plugins:              pluginService,
		pluginRuns:           pluginRunService,
		schedules:            scheduleService,
		logger:               logger,
		rateLimiter:          NewLoginRateLimiter(rateWindow, rateMax, rateMaxEntries),
		trustedProxyCIDRs:    append([]netip.Prefix(nil), trustedProxyCIDRs...),
		trustedProxyHeader:   trustedProxyHeader,
		pluginUploadMaxBytes: pluginUploadMaxBytes,
	}
}

// Handler builds the HTTP handler tree for the auth API.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /api/v1/auth/login", s.wrap(s.handleLogin))
	mux.HandleFunc("POST /api/v1/auth/refresh", s.wrap(s.handleRefresh))
	mux.HandleFunc("POST /api/v1/auth/logout", s.wrap(s.handleLogout))
	mux.HandleFunc("POST /api/v1/auth/change-password", s.wrap(s.handleChangePassword))
	mux.HandleFunc("GET /api/v1/auth/me", s.wrap(s.handleMe))
	mux.HandleFunc("POST /api/v1/admin/users", s.wrap(s.handleCreateUser))
	mux.HandleFunc("GET /api/v1/workspace", s.wrap(s.handleGetWorkspace))
	mux.HandleFunc("PUT /api/v1/workspace", s.wrap(s.handleSaveWorkspace))
	mux.HandleFunc("GET /api/v1/ai/config", s.wrap(s.handleGetAIConfig))
	mux.HandleFunc("PUT /api/v1/admin/ai/config", s.wrap(s.handleSaveAIConfig))
	mux.HandleFunc("POST /api/v1/ai/reply", s.wrap(s.handleGenerateAIReply))
	mux.HandleFunc("GET /api/v1/admin/plugins", s.wrap(s.handleListAdminPlugins))
	mux.HandleFunc("POST /api/v1/admin/plugins/upload", s.wrap(s.handleUploadPlugin))
	mux.HandleFunc("POST /api/v1/admin/plugins/install-from-git", s.wrap(s.handleInstallPluginFromGit))
	mux.HandleFunc("POST /api/v1/admin/plugins/{installationID}/disable", s.wrap(s.handleDisablePlugin))
	mux.HandleFunc("GET /api/v1/plugins", s.wrap(s.handleListPlugins))
	mux.HandleFunc("GET /api/v1/plugins/{installationID}", s.wrap(s.handleGetPlugin))
	mux.HandleFunc("PUT /api/v1/plugins/{installationID}/binding", s.wrap(s.handleSavePluginBinding))
	mux.HandleFunc("POST /api/v1/plugins/{installationID}/test", s.wrap(s.handleTestPluginBinding))
	mux.HandleFunc("POST /api/v1/plugins/{installationID}/run", s.wrap(s.handleRunPlugin))
	mux.HandleFunc("GET /api/v1/printers", s.wrap(s.handleListPrinters))
	mux.HandleFunc("POST /api/v1/printers/bind", s.wrap(s.handleBindPrinter))
	mux.HandleFunc("DELETE /api/v1/printers/{printerID}", s.wrap(s.handleDeletePrinter))
	mux.HandleFunc("POST /api/v1/feedback/print", s.wrap(s.handleSubmitFeedback))
	mux.HandleFunc("GET /api/v1/print-jobs", s.wrap(s.handleListPrintJobs))
	mux.HandleFunc("POST /api/v1/print-jobs", s.wrap(s.handleCreatePrintJob))
	mux.HandleFunc("POST /api/v1/print-preview", s.wrap(s.handlePrintPreview))
	mux.HandleFunc("POST /api/v1/print-jobs/{jobID}/submit", s.wrap(s.handleSubmitPrintJob))
	mux.HandleFunc("POST /api/v1/print-jobs/{jobID}/cancel", s.wrap(s.handleCancelPrintJob))
	mux.HandleFunc("PUT /api/v1/print-jobs/{jobID}/device", s.wrap(s.handleUpdatePrintJobDevice))
	mux.HandleFunc("GET /api/v1/print-schedules", s.wrap(s.handleListPrintSchedules))
	mux.HandleFunc("POST /api/v1/print-schedules", s.wrap(s.handleCreatePrintSchedule))
	mux.HandleFunc("PUT /api/v1/print-schedules/{scheduleID}", s.wrap(s.handleUpdatePrintSchedule))
	mux.HandleFunc("POST /api/v1/print-schedules/{scheduleID}/run", s.wrap(s.handleRunPrintSchedule))
	mux.HandleFunc("POST /api/v1/print-schedules/{scheduleID}/toggle", s.wrap(s.handleTogglePrintSchedule))
	mux.HandleFunc("DELETE /api/v1/print-schedules/{scheduleID}", s.wrap(s.handleDeletePrintSchedule))
	return mux
}

type responseEnvelope struct {
	User         auth.UserDTO `json:"user"`
	AccessToken  string       `json:"accessToken"`
	RefreshToken string       `json:"refreshToken"`
	ExpiresIn    int64        `json:"expiresIn"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type createUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type aiConfigRequest struct {
	ProviderName string `json:"providerName"`
	ProviderType string `json:"providerType"`
	BaseURL      string `json:"baseUrl"`
	Model        string `json:"model"`
	APIKey       string `json:"apiKey"`
}

type aiReplyRequest struct {
	Messages []ai.ChatMessage `json:"messages"`
}

type bindPrinterRequest struct {
	Name     string `json:"name"`
	Note     string `json:"note"`
	DeviceID string `json:"deviceId"`
}

type createPrintJobRequest struct {
	Title             string `json:"title"`
	Source            string `json:"source"`
	Content           string `json:"content"`
	PrinterBindingID  string `json:"printerBindingId"`
	SubmitImmediately bool   `json:"submitImmediately"`
}

type printPreviewRequest struct {
	Title   string                 `json:"title"`
	Content string                 `json:"content"`
	Blocks  []plugins.ContentBlock `json:"blocks,omitempty"`
}

type updatePrintJobDeviceRequest struct {
	PrinterBindingID string `json:"printerBindingId"`
}

type submitFeedbackRequest struct {
	Content string `json:"content"`
}

type pluginBindingRequest struct {
	Enabled bool              `json:"enabled"`
	Config  map[string]any    `json:"config"`
	Secrets map[string]string `json:"secrets"`
}

type printScheduleRequest struct {
	Title                string                 `json:"title"`
	PluginInstallationID string                 `json:"pluginInstallationId"`
	FrequencyType        schedule.FrequencyType `json:"frequencyType"`
	Timezone             string                 `json:"timezone"`
	Hour                 int                    `json:"hour"`
	Minute               int                    `json:"minute"`
	Weekdays             []int                  `json:"weekdays"`
	PrintPolicy          schedule.PrintPolicy   `json:"printPolicy"`
	DeviceID             string                 `json:"deviceId"`
	Enabled              bool                   `json:"enabled"`
}

type errorEnvelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

func (s *Server) wrap(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("Content-Type", "application/json")
		next(w, r, requestID)
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, requestID string, dst any, limit int64, allowEmpty bool) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		if allowEmpty && errors.Is(err, io.EOF) {
			return true
		}
		writeJSONDecodeError(w, requestID, err)
		return false
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeJSONDecodeError(w, requestID, err)
		return false
	}
	return true
}

func writeJSONDecodeError(w http.ResponseWriter, requestID string, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeError(w, requestID, http.StatusRequestEntityTooLarge, "request_too_large", "请求体过大。")
		return
	}
	writeError(w, requestID, http.StatusBadRequest, "invalid_request", "请求格式不正确。")
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request, requestID string) {
	var payload loginRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	meta := s.clientMetaFromRequest(r)
	if !s.rateLimiter.Allow("ip:"+meta.IPAddress, "account:"+auth.NormalizeEmail(payload.Email)) {
		writeError(w, requestID, http.StatusTooManyRequests, "rate_limited", "登录尝试过于频繁，请稍后再试。")
		return
	}

	result, err := s.auth.Login(r.Context(), auth.LoginInput{
		Email:    payload.Email,
		Password: payload.Password,
		Meta:     meta,
	})
	if err != nil {
		s.writeAuthError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, responseEnvelope{
		User:         result.User,
		AccessToken:  result.Token.AccessToken,
		RefreshToken: result.Token.RefreshToken,
		ExpiresIn:    int64(time.Until(result.Token.AccessTokenExpiresAt).Seconds()),
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request, requestID string) {
	var payload refreshRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	result, err := s.auth.Refresh(r.Context(), payload.RefreshToken, s.clientMetaFromRequest(r))
	if err != nil {
		s.writeAuthError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, responseEnvelope{
		User:         result.User,
		AccessToken:  result.Token.AccessToken,
		RefreshToken: result.Token.RefreshToken,
		ExpiresIn:    int64(time.Until(result.Token.AccessTokenExpiresAt).Seconds()),
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	account, err := s.auth.GetCurrentUser(r.Context(), accessToken)
	if err != nil {
		s.writeAuthError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]auth.UserDTO{"user": account})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request, requestID string) {
	var payload logoutRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, true) {
		return
	}

	if err := s.auth.Logout(r.Context(), bearerToken(r.Header.Get("Authorization")), payload.RefreshToken); err != nil {
		s.logger.Warn("logout failed", "request_id", requestID, "error", err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload changePasswordRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	if err := s.auth.ChangePassword(
		r.Context(),
		accessToken,
		payload.CurrentPassword,
		payload.NewPassword,
		s.clientMetaFromRequest(r),
	); err != nil {
		s.writeAuthError(w, requestID, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload createUserRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	created, err := s.auth.CreateUser(r.Context(), accessToken, auth.CreateUserInput{
		Email:    payload.Email,
		Name:     payload.Name,
		Password: payload.Password,
	})
	if err != nil {
		s.writeAuthError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]auth.UserDTO{"user": created})
}

func (s *Server) handleGetWorkspace(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	state, err := s.workspace.GetState(r.Context(), accessToken)
	if err != nil {
		s.writeAuthError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleSaveWorkspace(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var state workspace.State
	if !decodeJSON(w, r, requestID, &state, workspaceJSONMaxBytes, false) {
		return
	}

	saved, err := s.workspace.SaveState(r.Context(), accessToken, state)
	if err != nil {
		s.writeAuthError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleGetAIConfig(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	summary, err := s.ai.GetConfigSummary(r.Context(), accessToken)
	if err != nil {
		s.writeAIError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleSaveAIConfig(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload aiConfigRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	summary, err := s.ai.UpdateSystemConfig(r.Context(), accessToken, ai.UpdateConfigInput{
		ProviderName: payload.ProviderName,
		ProviderType: payload.ProviderType,
		BaseURL:      payload.BaseURL,
		Model:        payload.Model,
		APIKey:       payload.APIKey,
	})
	if err != nil {
		s.writeAIError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleGenerateAIReply(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload aiReplyRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	reply, err := s.ai.GenerateReply(r.Context(), accessToken, ai.ReplyInput{Messages: payload.Messages})
	if err != nil {
		s.writeAIError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, reply)
}

func (s *Server) handleListPrinters(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	devices, err := s.printer.ListDevices(r.Context(), accessToken)
	if err != nil {
		s.writePrinterError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string][]workspace.Device{"devices": devices})
}

func (s *Server) handleBindPrinter(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload bindPrinterRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	device, err := s.printer.BindDevice(r.Context(), accessToken, printer.BindInput{
		Name:     payload.Name,
		Note:     payload.Note,
		DeviceID: payload.DeviceID,
	})
	if err != nil {
		s.writePrinterError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]workspace.Device{"device": device})
}

func (s *Server) handleDeletePrinter(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	if err := s.printer.DeleteDevice(r.Context(), accessToken, r.PathValue("printerID")); err != nil {
		s.writePrinterError(w, requestID, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSubmitFeedback(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload submitFeedbackRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	if err := s.feedback.Submit(r.Context(), accessToken, feedback.SubmitInput{
		Content: payload.Content,
	}); err != nil {
		s.writeFeedbackError(w, requestID, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListPrintJobs(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	jobs, err := s.printer.ListPrintJobs(r.Context(), accessToken)
	if err != nil {
		s.writePrinterError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string][]workspace.PrintJob{"printJobs": jobs})
}

func (s *Server) handleCreatePrintJob(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload createPrintJobRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	job, err := s.printer.CreatePrintJob(r.Context(), accessToken, printer.CreateJobInput{
		Title:             payload.Title,
		Source:            payload.Source,
		Content:           payload.Content,
		PrinterBindingID:  payload.PrinterBindingID,
		SubmitImmediately: payload.SubmitImmediately,
	})
	if err != nil {
		s.writePrinterError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]workspace.PrintJob{"printJob": job})
}

func (s *Server) handlePrintPreview(w http.ResponseWriter, r *http.Request, requestID string) {
	if bearerToken(r.Header.Get("Authorization")) == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}
	var payload printPreviewRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}
	var image string
	var err error
	if len(payload.Blocks) > 0 {
		image, err = s.printer.RenderBlocksPreview(r.Context(), payload.Title, payload.Blocks)
	} else {
		image, err = s.printer.RenderPreview(r.Context(), payload.Title, payload.Content)
	}
	if err != nil {
		s.writePrinterError(w, requestID, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"image": image})
}

func (s *Server) handleSubmitPrintJob(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	job, err := s.printer.SubmitPrintJob(r.Context(), accessToken, r.PathValue("jobID"))
	if err != nil {
		s.writePrinterError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]workspace.PrintJob{"printJob": job})
}

func (s *Server) handleCancelPrintJob(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	job, err := s.printer.CancelPrintJob(r.Context(), accessToken, r.PathValue("jobID"))
	if err != nil {
		s.writePrinterError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]workspace.PrintJob{"printJob": job})
}

func (s *Server) handleUpdatePrintJobDevice(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload updatePrintJobDeviceRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	job, err := s.printer.UpdatePrintJobDevice(r.Context(), accessToken, r.PathValue("jobID"), printer.UpdateJobDeviceInput{
		PrinterBindingID: payload.PrinterBindingID,
	})
	if err != nil {
		s.writePrinterError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]workspace.PrintJob{"printJob": job})
}

func (s *Server) handleListPrintSchedules(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	items, err := s.schedules.List(r.Context(), accessToken)
	if err != nil {
		s.writeScheduleError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string][]schedule.ScheduleView{"schedules": items})
}

func (s *Server) handleCreatePrintSchedule(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload printScheduleRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	item, err := s.schedules.Create(r.Context(), accessToken, schedule.UpsertInput{
		Title:                payload.Title,
		PluginInstallationID: payload.PluginInstallationID,
		FrequencyType:        payload.FrequencyType,
		Timezone:             payload.Timezone,
		Hour:                 payload.Hour,
		Minute:               payload.Minute,
		Weekdays:             payload.Weekdays,
		PrintPolicy:          payload.PrintPolicy,
		DeviceID:             payload.DeviceID,
		Enabled:              payload.Enabled,
	})
	if err != nil {
		s.writeScheduleError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]schedule.ScheduleView{"schedule": item})
}

func (s *Server) handleUpdatePrintSchedule(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload printScheduleRequest
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, false) {
		return
	}

	item, err := s.schedules.Update(r.Context(), accessToken, r.PathValue("scheduleID"), schedule.UpsertInput{
		Title:                payload.Title,
		PluginInstallationID: payload.PluginInstallationID,
		FrequencyType:        payload.FrequencyType,
		Timezone:             payload.Timezone,
		Hour:                 payload.Hour,
		Minute:               payload.Minute,
		Weekdays:             payload.Weekdays,
		PrintPolicy:          payload.PrintPolicy,
		DeviceID:             payload.DeviceID,
		Enabled:              payload.Enabled,
	})
	if err != nil {
		s.writeScheduleError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]schedule.ScheduleView{"schedule": item})
}

func (s *Server) handleRunPrintSchedule(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	var payload struct{}
	if !decodeJSON(w, r, requestID, &payload, defaultJSONMaxBytes, true) {
		return
	}

	result, err := s.schedules.RunNow(r.Context(), accessToken, r.PathValue("scheduleID"))
	if err != nil {
		s.writeScheduleError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]schedule.ManualPrintResult{"result": result})
}

func (s *Server) handleTogglePrintSchedule(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	item, err := s.schedules.Toggle(r.Context(), accessToken, r.PathValue("scheduleID"))
	if err != nil {
		s.writeScheduleError(w, requestID, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]schedule.ScheduleView{"schedule": item})
}

func (s *Server) handleDeletePrintSchedule(w http.ResponseWriter, r *http.Request, requestID string) {
	accessToken := bearerToken(r.Header.Get("Authorization"))
	if accessToken == "" {
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "请先登录。")
		return
	}

	if err := s.schedules.Delete(r.Context(), accessToken, r.PathValue("scheduleID")); err != nil {
		s.writeScheduleError(w, requestID, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, requestID string, status int, code string, message string) {
	writeJSON(w, status, errorEnvelope{
		Code:      code,
		Message:   message,
		RequestID: requestID,
	})
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}

func (s *Server) clientMetaFromRequest(r *http.Request) auth.ClientMeta {
	return auth.ClientMeta{
		ClientType: session.ClientTypeWeb,
		UserAgent:  r.UserAgent(),
		IPAddress:  requestIP(r, s.trustedProxyCIDRs, s.trustedProxyHeader),
	}
}

func requestIP(r *http.Request, trustedProxyCIDRs []netip.Prefix, trustedProxyHeader string) string {
	peer, ok := parseForwardedAddress(strings.TrimSpace(r.RemoteAddr))
	if !ok {
		return strings.TrimSpace(r.RemoteAddr)
	}
	if !addressInPrefixes(peer, trustedProxyCIDRs) {
		return peer.String()
	}

	chain, ok := forwardedChain(r.Header, trustedProxyHeader)
	if !ok || len(chain) == 0 {
		return peer.String()
	}
	current := peer
	for index := len(chain) - 1; index >= 0; index-- {
		if !addressInPrefixes(current, trustedProxyCIDRs) {
			return current.String()
		}
		current = chain[index]
	}
	return current.String()
}

func forwardedChain(header http.Header, trustedProxyHeader string) ([]netip.Addr, bool) {
	if trustedProxyHeader == "" {
		return nil, true
	}
	values := header.Values(http.CanonicalHeaderKey(trustedProxyHeader))
	if len(values) == 0 {
		return nil, true
	}
	if trustedProxyHeader == "forwarded" {
		return parseForwardedHeader(strings.Join(values, ","))
	}
	parts := strings.Split(strings.Join(values, ","), ",")
	result := make([]netip.Addr, 0, len(parts))
	for _, part := range parts {
		address, ok := parseForwardedAddress(strings.TrimSpace(part))
		if !ok {
			return nil, false
		}
		result = append(result, address)
	}
	return result, true
}

func parseForwardedHeader(value string) ([]netip.Addr, bool) {
	elements, ok := splitForwardedValue(value, ',')
	if !ok {
		return nil, false
	}
	result := make([]netip.Addr, 0, len(elements))
	for _, element := range elements {
		var rawAddress string
		seenParameters := make(map[string]struct{})
		parameters, ok := splitForwardedValue(element, ';')
		if !ok || len(parameters) == 0 {
			return nil, false
		}
		for _, parameter := range parameters {
			key, value, found := strings.Cut(strings.TrimSpace(parameter), "=")
			key = strings.ToLower(strings.TrimSpace(key))
			if !found || !isHTTPToken(key) {
				return nil, false
			}
			if _, duplicate := seenParameters[key]; duplicate {
				return nil, false
			}
			seenParameters[key] = struct{}{}
			parsedValue, ok := parseForwardedParameter(value)
			if !ok {
				return nil, false
			}
			if key == "for" {
				rawAddress = parsedValue
			}
		}
		address, ok := parseForwardedAddress(rawAddress)
		if !ok {
			return nil, false
		}
		result = append(result, address)
	}
	return result, true
}

func splitForwardedValue(value string, separator byte) ([]string, bool) {
	result := []string{}
	start := 0
	quoted := false
	escaped := false
	for index := 0; index < len(value); index++ {
		current := value[index]
		if escaped {
			escaped = false
			continue
		}
		if quoted && current == '\\' {
			escaped = true
			continue
		}
		if current == '"' {
			quoted = !quoted
			continue
		}
		if current == separator && !quoted {
			result = append(result, value[start:index])
			start = index + 1
		}
	}
	if quoted || escaped {
		return nil, false
	}
	return append(result, value[start:]), true
}

func parseForwardedParameter(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, `"`) {
		unquoted, err := strconv.Unquote(value)
		if err != nil || hasControlCharacter(unquoted) {
			return "", false
		}
		return unquoted, true
	}
	if !isHTTPToken(value) {
		return "", false
	}
	return value, true
}

func isHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	const separators = `()<>@,;:\"/[]?={} `
	for index := 0; index < len(value); index++ {
		current := value[index]
		if current <= 0x20 || current >= 0x7f || strings.ContainsRune(separators, rune(current)) {
			return false
		}
	}
	return true
}

func hasControlCharacter(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] == 0x7f {
			return true
		}
	}
	return false
}

func parseForwardedAddress(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "unknown") || strings.HasPrefix(value, "_") {
		return netip.Addr{}, false
	}
	if addressPort, err := netip.ParseAddrPort(value); err == nil {
		return addressPort.Addr().Unmap(), true
	}
	address, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}

func addressInPrefixes(address netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

// LoginRateLimiter limits repeated login attempts within a fixed time window.
type LoginRateLimiter struct {
	mu          sync.Mutex
	window      time.Duration
	max         int
	maxEntries  int
	hits        map[string][]time.Time
	lastSeen    map[string]time.Time
	lastCleanup time.Time
	now         func() time.Time
}

// NewLoginRateLimiter creates a rate limiter for login attempts.
func NewLoginRateLimiter(window time.Duration, max int, maxEntries int) *LoginRateLimiter {
	return &LoginRateLimiter{
		window:     window,
		max:        max,
		maxEntries: maxEntries,
		hits:       make(map[string][]time.Time),
		lastSeen:   make(map[string]time.Time),
		now:        time.Now,
	}
}

// Allow atomically records one login attempt against every supplied dimension.
func (l *LoginRateLimiter) Allow(keys ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)
	if l.lastCleanup.IsZero() || now.Sub(l.lastCleanup) >= l.window {
		l.cleanup(cutoff)
		l.lastCleanup = now
	}
	if len(l.hits)+missingKeyCount(l.hits, keys) > l.maxEntries {
		return false
	}

	windowHits := make(map[string][]time.Time, len(keys))
	allowed := true
	for _, key := range keys {
		current := l.hits[key][:0]
		for _, hit := range l.hits[key] {
			if hit.After(cutoff) {
				current = append(current, hit)
			}
		}
		windowHits[key] = current
		if len(current) >= l.max {
			allowed = false
		}
	}

	for key, current := range windowHits {
		if len(current) < l.max {
			current = append(current, now)
		}
		l.hits[key] = current
		l.lastSeen[key] = now
	}
	return allowed
}

func (l *LoginRateLimiter) cleanup(cutoff time.Time) {
	for key, seen := range l.lastSeen {
		if !seen.After(cutoff) {
			delete(l.hits, key)
			delete(l.lastSeen, key)
		}
	}
}

func missingKeyCount(existing map[string][]time.Time, keys []string) int {
	missing := 0
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		if _, ok := existing[key]; !ok {
			missing++
		}
	}
	return missing
}

type contextKey string

const requestIDKey contextKey = "request_id"

// WithRequestID stores the request identifier on a context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}
