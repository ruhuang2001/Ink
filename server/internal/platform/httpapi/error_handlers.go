package httpapi

import (
	"errors"
	"net/http"

	"github.com/ruhuang/ink/server/internal/ai"
	"github.com/ruhuang/ink/server/internal/auth"
	"github.com/ruhuang/ink/server/internal/feedback"
	"github.com/ruhuang/ink/server/internal/printer"
)

func (s *Server) writeAuthError(w http.ResponseWriter, requestID string, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, requestID, http.StatusUnauthorized, "invalid_credentials")
	case errors.Is(err, auth.ErrCurrentPassword):
		writeError(w, requestID, http.StatusUnauthorized, "current_password_incorrect")
	case errors.Is(err, auth.ErrInvalidRefreshToken), errors.Is(err, auth.ErrInvalidAccessToken):
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, auth.ErrWeakPassword):
		writeError(w, requestID, http.StatusBadRequest, "invalid_password")
	case errors.Is(err, auth.ErrInvalidProfile):
		writeError(w, requestID, http.StatusBadRequest, "invalid_profile")
	case errors.Is(err, auth.ErrEmailTaken):
		writeError(w, requestID, http.StatusConflict, "email_taken")
	case errors.Is(err, auth.ErrForbidden):
		writeError(w, requestID, http.StatusForbidden, "forbidden")
	case errors.Is(err, auth.ErrUserDisabled):
		writeError(w, requestID, http.StatusLocked, "user_disabled")
	default:
		s.logger.Error("auth handler failed", "request_id", requestID, "error", err)
		writeError(w, requestID, http.StatusInternalServerError, "internal_error")
	}
}

func (s *Server) writeAIError(w http.ResponseWriter, requestID string, err error) {
	switch {
	case errors.Is(err, ai.ErrForbidden):
		writeError(w, requestID, http.StatusForbidden, "forbidden")
	case errors.Is(err, ai.ErrNotConfigured):
		writeError(w, requestID, http.StatusPreconditionFailed, "ai_not_configured")
	case errors.Is(err, ai.ErrMissingSecret):
		writeError(w, requestID, http.StatusServiceUnavailable, "ai_secret_missing")
	case errors.Is(err, ai.ErrInvalidConfig):
		writeError(w, requestID, http.StatusBadRequest, "invalid_ai_config")
	case errors.Is(err, ai.ErrInvalidInput):
		writeError(w, requestID, http.StatusBadRequest, "invalid_ai_input")
	case errors.Is(err, ai.ErrProviderUnavailable):
		writeError(w, requestID, http.StatusBadGateway, "ai_provider_unavailable")
	default:
		s.writeAuthError(w, requestID, err)
	}
}

func (s *Server) writePrinterError(w http.ResponseWriter, requestID string, err error) {
	switch {
	case errors.Is(err, printer.ErrSubmissionOutcomeUnknown):
		writeError(w, requestID, http.StatusBadGateway, "print_submission_unknown")
	case errors.Is(err, printer.ErrForbidden):
		writeError(w, requestID, http.StatusForbidden, "forbidden")
	case errors.Is(err, printer.ErrNotConfigured):
		writeError(w, requestID, http.StatusPreconditionFailed, "printer_not_configured")
	case errors.Is(err, printer.ErrContentTooLarge):
		writeError(w, requestID, http.StatusRequestEntityTooLarge, "print_content_too_large")
	case errors.Is(err, printer.ErrNotFound):
		writeError(w, requestID, http.StatusNotFound, "printer_resource_not_found")
	case errors.Is(err, printer.ErrInvalidInput):
		writeError(w, requestID, http.StatusBadRequest, "invalid_printer_input")
	case errors.Is(err, printer.ErrUnavailable):
		writeError(w, requestID, http.StatusBadGateway, "printer_unavailable")
	default:
		s.writeAuthError(w, requestID, err)
	}
}

func (s *Server) writeFeedbackError(w http.ResponseWriter, requestID string, err error) {
	switch {
	case errors.Is(err, feedback.ErrInvalidInput):
		writeError(w, requestID, http.StatusBadRequest, "invalid_feedback_input")
	case errors.Is(err, feedback.ErrNoAdminRecipient):
		writeError(w, requestID, http.StatusPreconditionFailed, "feedback_recipient_missing")
	case errors.Is(err, feedback.ErrNoAdminDevice):
		writeError(w, requestID, http.StatusPreconditionFailed, "feedback_printer_missing")
	case errors.Is(err, printer.ErrSubmissionOutcomeUnknown), errors.Is(err, printer.ErrNotConfigured), errors.Is(err, printer.ErrNotFound), errors.Is(err, printer.ErrInvalidInput), errors.Is(err, printer.ErrContentTooLarge), errors.Is(err, printer.ErrUnavailable), errors.Is(err, printer.ErrForbidden):
		s.writePrinterError(w, requestID, err)
	default:
		s.writeAuthError(w, requestID, err)
	}
}
