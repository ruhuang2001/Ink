package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ruhuang/ink/server/internal/printer"
)

func TestUnknownPrintSubmissionUsesSafePublicError(t *testing.T) {
	for _, path := range []string{"/api/v1/print-jobs", "/api/v1/print-jobs/job-1/submit", "/api/v1/feedback/print"} {
		t.Run(path, func(t *testing.T) {
			err := errors.Join(printer.ErrSubmissionOutcomeUnknown, errors.New("synthetic-provider-secret"))
			server := newTestServer(fakeAuthService{}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{err: err}, fakeFeedbackService{err: err})
			request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"title":"Receipt","content":"Synthetic","printerBindingId":"device-1","submitImmediately":true}`))
			if path == "/api/v1/feedback/print" {
				request = httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"content":"Synthetic"}`))
			}
			request.Header.Set("Authorization", "Bearer synthetic-token")
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, request)
			var body errorEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusBadGateway || body.Code != "print_submission_unknown" || !strings.Contains(body.Message, "请先检查设备") || !strings.Contains(body.Message, "请勿直接重复提交") {
				t.Fatalf("unsafe unknown error: status=%d body=%+v", response.Code, body)
			}
			if strings.Contains(body.Message, "重试") || strings.Contains(response.Body.String(), "synthetic-provider-secret") {
				t.Fatal("unknown result suggested retry or exposed provider details")
			}
		})
	}
}

func TestKnownPrintFailureRetainsUnavailableError(t *testing.T) {
	server := newTestServer(fakeAuthService{}, fakeWorkspaceService{}, fakeAIService{}, fakePrinterService{}, fakeFeedbackService{})
	response := httptest.NewRecorder()
	server.writePrinterError(response, "synthetic-request", printer.ErrUnavailable)
	var body errorEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "printer_unavailable" || response.Code != http.StatusBadGateway {
		t.Fatalf("known failure error changed: %+v", body)
	}
}
