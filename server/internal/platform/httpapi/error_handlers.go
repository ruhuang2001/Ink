package httpapi

import (
	"errors"
	"net/http"

	"github.com/ruhuang/ink/server/internal/ai"
	"github.com/ruhuang/ink/server/internal/auth"
	"github.com/ruhuang/ink/server/internal/feedback"
	"github.com/ruhuang/ink/server/internal/plugins"
	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/schedule"
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
	case errors.Is(err, printer.ErrForbidden):
		writeError(w, requestID, http.StatusForbidden, "forbidden", "当前账号没有该操作权限。")
	case errors.Is(err, printer.ErrNotConfigured):
		writeError(w, requestID, http.StatusPreconditionFailed, "printer_not_configured", "当前还没有配置 Memobird 服务。")
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
	case errors.Is(err, printer.ErrNotConfigured), errors.Is(err, printer.ErrNotFound), errors.Is(err, printer.ErrInvalidInput), errors.Is(err, printer.ErrUnavailable), errors.Is(err, printer.ErrForbidden):
		s.writePrinterError(w, requestID, err)
	default:
		s.writeAuthError(w, requestID, err)
	}
}

func (s *Server) writePluginError(w http.ResponseWriter, requestID string, err error) {
	var validationFailure plugins.ValidationFailure
	switch {
	case errors.Is(err, plugins.ErrForbidden):
		writeError(w, requestID, http.StatusForbidden, "forbidden", "当前账号没有该操作权限。")
	case errors.Is(err, plugins.ErrNotFound):
		writeError(w, requestID, http.StatusNotFound, "plugin_not_found", "指定插件不存在。")
	case errors.Is(err, plugins.ErrMissingSecret):
		writeError(w, requestID, http.StatusServiceUnavailable, "plugin_secret_missing", "服务端尚未配置插件加密密钥。")
	case errors.As(err, &validationFailure):
		writeError(w, requestID, http.StatusBadRequest, "invalid_plugin_config", validationFailure.Error())
	case errors.Is(err, plugins.ErrInvalidInput):
		writeError(w, requestID, http.StatusBadRequest, "invalid_plugin_input", "请输入有效的插件配置。")
	case errors.Is(err, plugins.ErrInvalidPlugin):
		writeError(w, requestID, http.StatusBadRequest, "invalid_plugin_package", err.Error())
	case errors.Is(err, plugins.ErrGitInstallDisabled):
		writeError(w, requestID, http.StatusServiceUnavailable, "plugin_git_install_disabled", "服务端未启用从 Git 仓库安装插件。")
	case errors.Is(err, plugins.ErrExecutionFailed):
		writeError(w, requestID, http.StatusBadGateway, "plugin_execution_failed", err.Error())
	default:
		s.writeAuthError(w, requestID, err)
	}
}

func (s *Server) writeScheduleError(w http.ResponseWriter, requestID string, err error) {
	switch {
	case errors.Is(err, schedule.ErrNotFound):
		writeError(w, requestID, http.StatusNotFound, "schedule_not_found", "指定定时任务不存在。")
	case errors.Is(err, schedule.ErrInvalidInput):
		writeError(w, requestID, http.StatusBadRequest, "invalid_schedule_input", err.Error())
	default:
		var validationFailure plugins.ValidationFailure
		if errors.As(err, &validationFailure) {
			writeError(w, requestID, http.StatusBadRequest, "invalid_schedule_config", validationFailure.Error())
			return
		}
		s.writePluginError(w, requestID, err)
	}
}
