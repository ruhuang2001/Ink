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
		writeError(w, requestID, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password.")
	case errors.Is(err, auth.ErrCurrentPassword):
		writeError(w, requestID, http.StatusUnauthorized, "current_password_incorrect", "The current password is incorrect.")
	case errors.Is(err, auth.ErrInvalidRefreshToken), errors.Is(err, auth.ErrInvalidAccessToken):
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "Your session has expired. Please sign in again.")
	case errors.Is(err, auth.ErrWeakPassword):
		writeError(w, requestID, http.StatusBadRequest, "invalid_password", "The password must be at least 8 characters long.")
	case errors.Is(err, auth.ErrInvalidProfile):
		writeError(w, requestID, http.StatusBadRequest, "invalid_profile", "Enter valid account information.")
	case errors.Is(err, auth.ErrEmailTaken):
		writeError(w, requestID, http.StatusConflict, "email_taken", "This account already exists.")
	case errors.Is(err, auth.ErrForbidden):
		writeError(w, requestID, http.StatusForbidden, "forbidden", "You do not have permission to perform this action.")
	case errors.Is(err, auth.ErrUserDisabled):
		writeError(w, requestID, http.StatusLocked, "user_disabled", "This account has been disabled.")
	default:
		s.logger.Error("auth handler failed", "request_id", requestID, "error", err)
		writeError(w, requestID, http.StatusInternalServerError, "internal_error", "The service is temporarily unavailable. Please try again later.")
	}
}

func (s *Server) writeAIError(w http.ResponseWriter, requestID string, err error) {
	switch {
	case errors.Is(err, ai.ErrForbidden):
		writeError(w, requestID, http.StatusForbidden, "forbidden", "You do not have permission to perform this action.")
	case errors.Is(err, ai.ErrNotConfigured):
		writeError(w, requestID, http.StatusPreconditionFailed, "ai_not_configured", "The AI service has not been configured.")
	case errors.Is(err, ai.ErrMissingSecret):
		writeError(w, requestID, http.StatusServiceUnavailable, "ai_secret_missing", "The server has not configured an AI encryption key.")
	case errors.Is(err, ai.ErrInvalidConfig):
		writeError(w, requestID, http.StatusBadRequest, "invalid_ai_config", "Enter valid AI service settings.")
	case errors.Is(err, ai.ErrInvalidInput):
		writeError(w, requestID, http.StatusBadRequest, "invalid_ai_input", "Enter valid conversation content.")
	case errors.Is(err, ai.ErrProviderUnavailable):
		writeError(w, requestID, http.StatusBadGateway, "ai_provider_unavailable", "The AI service is temporarily unavailable. Please try again later.")
	default:
		s.writeAuthError(w, requestID, err)
	}
}

func (s *Server) writePrinterError(w http.ResponseWriter, requestID string, err error) {
	switch {
	case errors.Is(err, printer.ErrSubmissionOutcomeUnknown):
		writeError(w, requestID, http.StatusBadGateway, "print_submission_unknown", "The submission result is unknown. Check whether the device printed the job before creating another one; do not resubmit without checking.")
	case errors.Is(err, printer.ErrForbidden):
		writeError(w, requestID, http.StatusForbidden, "forbidden", "You do not have permission to perform this action.")
	case errors.Is(err, printer.ErrNotConfigured):
		writeError(w, requestID, http.StatusPreconditionFailed, "printer_not_configured", "The Memobird service has not been configured.")
	case errors.Is(err, printer.ErrContentTooLarge):
		writeError(w, requestID, http.StatusRequestEntityTooLarge, "print_content_too_large", "Content exceeds the print limits. Split it before previewing or printing (title: 512 bytes max; body: 64 KiB max; total: 8,000 characters max; image height: 8,192 pixels max). Avoid long runs of zero-width characters.")
	case errors.Is(err, printer.ErrNotFound):
		writeError(w, requestID, http.StatusNotFound, "printer_resource_not_found", "The specified device or print job does not exist.")
	case errors.Is(err, printer.ErrInvalidInput):
		writeError(w, requestID, http.StatusBadRequest, "invalid_printer_input", "Enter valid device or print-job information.")
	case errors.Is(err, printer.ErrUnavailable):
		writeError(w, requestID, http.StatusBadGateway, "printer_unavailable", "The Memobird service is temporarily unavailable. Please try again later.")
	default:
		s.writeAuthError(w, requestID, err)
	}
}

func (s *Server) writeFeedbackError(w http.ResponseWriter, requestID string, err error) {
	switch {
	case errors.Is(err, feedback.ErrInvalidInput):
		writeError(w, requestID, http.StatusBadRequest, "invalid_feedback_input", "Enter feedback.")
	case errors.Is(err, feedback.ErrNoAdminRecipient):
		writeError(w, requestID, http.StatusPreconditionFailed, "feedback_recipient_missing", "There is no administrator account available to receive feedback.")
	case errors.Is(err, feedback.ErrNoAdminDevice):
		writeError(w, requestID, http.StatusPreconditionFailed, "feedback_printer_missing", "The administrator has not configured a default Memobird to receive feedback.")
	case errors.Is(err, printer.ErrSubmissionOutcomeUnknown), errors.Is(err, printer.ErrNotConfigured), errors.Is(err, printer.ErrNotFound), errors.Is(err, printer.ErrInvalidInput), errors.Is(err, printer.ErrContentTooLarge), errors.Is(err, printer.ErrUnavailable), errors.Is(err, printer.ErrForbidden):
		s.writePrinterError(w, requestID, err)
	default:
		s.writeAuthError(w, requestID, err)
	}
}
