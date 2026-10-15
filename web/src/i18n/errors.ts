import { AuthApiError } from "@/services/auth";

import { translate } from "./index";

const LOCALIZED_ERROR_CODES = new Set([
  "auth_refresh_failed",
  "ai_not_configured",
  "ai_provider_unavailable",
  "ai_secret_missing",
  "feedback_printer_missing",
  "feedback_recipient_missing",
  "forbidden",
  "invalid_ai_config",
  "invalid_ai_input",
  "invalid_credentials",
  "invalid_feedback_input",
  "invalid_printer_input",
  "invalid_session_payload",
  "network_error",
  "printer_not_configured",
  "printer_resource_not_found",
  "printer_unavailable",
  "print_content_too_large",
  "print_submission_unknown",
  "request_failed",
  "workspace_conflict",
  "workspace_revision_required",
]);

export function getLocalizedErrorMessage(
  error: unknown,
  fallbackKey = "errors.api.request_failed",
) {
  if (error instanceof AuthApiError && LOCALIZED_ERROR_CODES.has(error.code)) {
    return translate(`errors.api.${error.code}`);
  }

  if (error instanceof Error && error.message) {
    return error.message;
  }

  return translate(fallbackKey);
}
