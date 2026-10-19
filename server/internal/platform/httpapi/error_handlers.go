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
		writeError(w, requestID, http.StatusUnauthorized, "invalid_credentials", "账号或密码不正确。")
	case errors.Is(err, auth.ErrCurrentPassword):
		writeError(w, requestID, http.StatusUnauthorized, "current_password_incorrect", "当前密码不正确。")
	case errors.Is(err, auth.ErrInvalidRefreshToken), errors.Is(err, auth.ErrInvalidAccessToken):
		writeError(w, requestID, http.StatusUnauthorized, "unauthorized", "登录状态已失效，请重新登录。")
	case errors.Is(err, auth.ErrWeakPassword):
		writeError(w, requestID, http.StatusBadRequest, "invalid_password", "密码至少 8 位。")
	case errors.Is(err, auth.ErrInvalidProfile):
		writeError(w, requestID, http.StatusBadRequest, "invalid_profile", "请输入有效的账号信息。")
	case errors.Is(err, auth.ErrEmailTaken):
		writeError(w, requestID, http.StatusConflict, "email_taken", "该账号已存在。")
	case errors.Is(err, auth.ErrForbidden):
		writeError(w, requestID, http.StatusForbidden, "forbidden", "当前账号没有该操作权限。")
	case errors.Is(err, auth.ErrUserDisabled):
		writeError(w, requestID, http.StatusLocked, "user_disabled", "账号已被禁用。")
	default:
		s.logger.Error("auth handler failed", "request_id", requestID, "error", err)
		writeError(w, requestID, http.StatusInternalServerError, "internal_error", "服务暂时不可用，请稍后重试。")
	}
}

func (s *Server) writeAIError(w http.ResponseWriter, requestID string, err error) {
	switch {
	case errors.Is(err, ai.ErrForbidden):
		writeError(w, requestID, http.StatusForbidden, "forbidden", "当前账号没有该操作权限。")
	case errors.Is(err, ai.ErrNotConfigured):
		writeError(w, requestID, http.StatusPreconditionFailed, "ai_not_configured", "当前还没有配置 AI 服务。")
	case errors.Is(err, ai.ErrMissingSecret):
		writeError(w, requestID, http.StatusServiceUnavailable, "ai_secret_missing", "服务端尚未配置 AI 加密密钥。")
	case errors.Is(err, ai.ErrInvalidConfig):
		writeError(w, requestID, http.StatusBadRequest, "invalid_ai_config", "请输入有效的 AI 服务配置。")
	case errors.Is(err, ai.ErrInvalidInput):
		writeError(w, requestID, http.StatusBadRequest, "invalid_ai_input", "请输入有效的对话内容。")
	case errors.Is(err, ai.ErrProviderUnavailable):
		writeError(w, requestID, http.StatusBadGateway, "ai_provider_unavailable", "AI 服务暂时不可用，请稍后重试。")
	default:
		s.writeAuthError(w, requestID, err)
	}
}

func (s *Server) writePrinterError(w http.ResponseWriter, requestID string, err error) {
	switch {
	case errors.Is(err, printer.ErrSubmissionOutcomeUnknown):
		writeError(w, requestID, http.StatusBadGateway, "print_submission_unknown", "提交结果未知，请先检查设备是否已打印，确认后再决定是否手动创建新任务；请勿直接重复提交。")
	case errors.Is(err, printer.ErrForbidden):
		writeError(w, requestID, http.StatusForbidden, "forbidden", "当前账号没有该操作权限。")
	case errors.Is(err, printer.ErrNotConfigured):
		writeError(w, requestID, http.StatusPreconditionFailed, "printer_not_configured", "当前还没有配置 Memobird 服务。")
	case errors.Is(err, printer.ErrContentTooLarge):
		writeError(w, requestID, http.StatusRequestEntityTooLarge, "print_content_too_large", "内容超过打印限制，请拆分后再预览或打印（标题最多 512 字节、正文最多 64 KiB、合计最多 8000 个字符、图片高度最多 8192 像素）；请避免过多连续零宽字符。")
	case errors.Is(err, printer.ErrNotFound):
		writeError(w, requestID, http.StatusNotFound, "printer_resource_not_found", "指定的设备或打印任务不存在。")
	case errors.Is(err, printer.ErrInvalidInput):
		writeError(w, requestID, http.StatusBadRequest, "invalid_printer_input", "请输入有效的设备或打印信息。")
	case errors.Is(err, printer.ErrUnavailable):
		writeError(w, requestID, http.StatusBadGateway, "printer_unavailable", "咕咕机服务暂时不可用，请稍后重试。")
	default:
		s.writeAuthError(w, requestID, err)
	}
}

func (s *Server) writeFeedbackError(w http.ResponseWriter, requestID string, err error) {
	switch {
	case errors.Is(err, feedback.ErrInvalidInput):
		writeError(w, requestID, http.StatusBadRequest, "invalid_feedback_input", "请输入反馈内容。")
	case errors.Is(err, feedback.ErrNoAdminRecipient):
		writeError(w, requestID, http.StatusPreconditionFailed, "feedback_recipient_missing", "当前还没有可接收反馈的管理员账号。")
	case errors.Is(err, feedback.ErrNoAdminDevice):
		writeError(w, requestID, http.StatusPreconditionFailed, "feedback_printer_missing", "管理员当前还没有可接收反馈的默认咕咕机。")
	case errors.Is(err, printer.ErrSubmissionOutcomeUnknown), errors.Is(err, printer.ErrNotConfigured), errors.Is(err, printer.ErrNotFound), errors.Is(err, printer.ErrInvalidInput), errors.Is(err, printer.ErrContentTooLarge), errors.Is(err, printer.ErrUnavailable), errors.Is(err, printer.ErrForbidden):
		s.writePrinterError(w, requestID, err)
	default:
		s.writeAuthError(w, requestID, err)
	}
}
