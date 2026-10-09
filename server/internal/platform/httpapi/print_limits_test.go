package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ruhuang/ink/server/internal/plugins"
	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/workspace"
)

func TestPrintRoutesRejectOversizedContent(t *testing.T) {
	for _, test := range []struct {
		name    string
		title   string
		content string
	}{
		{"title bytes", strings.Repeat("a", 513), "Content"},
		{"content bytes", "Title", strings.Repeat("a", (64<<10)+1)},
		{"image height", "Title", strings.Repeat("line\n", 1000)},
	} {
		for _, route := range []string{"/api/v1/print-preview", "/api/v1/print-jobs"} {
			t.Run(test.name+route, func(t *testing.T) {
				server := newTestServer(fakeAuthService{}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{}, fakePluginService{}, fakePluginRunService{}, fakeScheduleService{})
				server.printer = limitCheckingPrinter{service: printer.NewService(nil, fakeAuthService{}, nil, nil, "", "", time.Second)}
				payload := map[string]any{"title": test.title, "content": test.content}
				if route == "/api/v1/print-jobs" {
					payload["printerBindingId"] = "device-1"
				}
				assertPrintContentTooLarge(t, server, route, payload)
			})
		}
	}
}

func TestFeedbackPreservesPrintLimitAndRequestBodyErrors(t *testing.T) {
	server := newTestServer(fakeAuthService{}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{err: printer.ErrContentTooLarge}, fakePluginService{}, fakePluginRunService{}, fakeScheduleService{})
	assertPrintContentTooLarge(t, server, "/api/v1/feedback/print", map[string]string{"content": "oversized feedback"})
	body, err := json.Marshal(map[string]string{"content": strings.Repeat("a", (1<<20)+1)})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/feedback/print", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), `"code":"request_too_large"`) {
		t.Fatalf("expected body limit error, got %d: %s", response.Code, response.Body.String())
	}
}

func TestBlocksPreviewRejectsOversizedContent(t *testing.T) {
	server := newTestServer(fakeAuthService{}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{}, fakePluginService{}, fakePluginRunService{}, fakeScheduleService{})
	server.printer = limitCheckingPrinter{service: printer.NewService(nil, fakeAuthService{}, nil, nil, "", "", time.Second)}
	assertPrintContentTooLarge(t, server, "/api/v1/print-preview", map[string]any{
		"title":  "Title",
		"blocks": []plugins.ContentBlock{{Type: plugins.BlockParagraph, Text: strings.Repeat("a", (64<<10)+1)}},
	})
}

func assertPrintContentTooLarge(t *testing.T, server *Server, route string, payload any) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, route, bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), `"code":"print_content_too_large"`) {
		t.Fatalf("expected print content 413, got %d: %s", response.Code, response.Body.String())
	}
}

type limitCheckingPrinter struct {
	fakePrinterService
	service *printer.Service
}

func (p limitCheckingPrinter) RenderPreview(ctx context.Context, title, content string) (string, error) {
	return p.service.RenderPreview(ctx, title, content)
}

func (p limitCheckingPrinter) RenderBlocksPreview(ctx context.Context, title string, blocks []plugins.ContentBlock) (string, error) {
	return p.service.RenderBlocksPreview(ctx, title, blocks)
}

func (p limitCheckingPrinter) CreatePrintJob(ctx context.Context, token string, input printer.CreateJobInput) (workspace.PrintJob, error) {
	return p.service.CreatePrintJob(ctx, token, input)
}
