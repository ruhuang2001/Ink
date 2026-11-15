import { AuthApiError } from "@/services/auth";

import { translate } from "./index";

export const LOCALIZED_ERROR_CODES = new Set([
  "auth_refresh_failed",
  "ai_not_configured",
  "ai_provider_unavailable",
  "ai_secret_missing",
  "current_password_incorrect",
  "email_taken",
  "feedback_printer_missing",
  "feedback_recipient_missing",
  "forbidden",
  "internal_error",
  "invalid_ai_config",
  "invalid_ai_input",
  "invalid_credentials",
  "invalid_feedback_input",
  "invalid_password",
  "invalid_printer_input",
  "invalid_profile",
  "invalid_request",
  "invalid_session_payload",
  "network_error",
  "printer_not_configured",
  "printer_resource_not_found",
  "printer_unavailable",
  "print_content_too_large",
  "print_submission_unknown",
  "rate_limited",
  "request_failed",
  "request_too_large",
  "unauthorized",
  "user_disabled",
  "workspace_conflict",
  "workspace_revision_required",
]);

// Print-job failures are stored as codes, not prose, so they carry no locale.
// Only these codes are rendered; an unrecognized value stays hidden rather than
// showing a raw internal message.
const LOCALIZED_PRINT_JOB_ERRORS = new Set([
  "print_submission_unknown",
  "print_provider_print_id_unknown",
]);

export function getLocalizedErrorMessage(
  error: unknown,
  fallbackKey = "errors.api.request_failed",
) {
  if (error instanceof AuthApiError) {
    // The API sends codes, not prose, so an unrecognized code falls back to a
    // generic localized message instead of showing the raw identifier.
    return LOCALIZED_ERROR_CODES.has(error.code)
      ? translate(`errors.api.${error.code}`)
      : translate(fallbackKey);
  }

  if (error instanceof Error && error.message) {
    return error.message;
  }

  return translate(fallbackKey);
}

export function getLocalizedPrintJobErrorKey(errorMessage?: string) {
  if (!errorMessage || !LOCALIZED_PRINT_JOB_ERRORS.has(errorMessage)) return "";
  return `prints.errors.${errorMessage}`;
}
