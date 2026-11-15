import { describe, expect, it } from "vitest";

import { setI18nLocale, translate } from "@/i18n";
import {
  LOCALIZED_ERROR_CODES,
  getLocalizedErrorMessage,
  getLocalizedPrintJobErrorKey,
} from "@/i18n/errors";
import { AuthApiError } from "@/services/http";

describe("workspace and print errors", () => {
  it.each([
    ["zh-CN", "workspace_conflict", "草稿"],
    ["en-US", "workspace_conflict", "local draft"],
    ["zh-CN", "workspace_revision_required", "重新加载"],
    ["en-US", "workspace_revision_required", "reload"],
    ["zh-CN", "print_content_too_large", "8192"],
    ["en-US", "print_content_too_large", "8,192"],
  ] as const)("localizes %s %s", (locale, code, expected) => {
    setI18nLocale(locale);
    const message = getLocalizedErrorMessage(new AuthApiError(409, code, "Server fallback"));
    expect(message).toBe(translate(`errors.api.${code}`));
    expect(message).toContain(expected);
    expect(message).not.toBe("Server fallback");
  });
  it("localizes every code the API can send", () => {
    // The API returns codes with no prose, so a code without a message becomes a
    // generic fallback in the interface. Keep the two lists in step.
    for (const locale of ["zh-CN", "en-US"] as const) {
      setI18nLocale(locale);
      for (const code of LOCALIZED_ERROR_CODES) {
        expect(translate(`errors.api.${code}`), `${locale} ${code}`).not.toContain("errors.api.");
      }
    }
  });
  it("replaces an unrecognized API code with the localized fallback", () => {
    setI18nLocale("en-US");
    // The API no longer sends prose, so an unknown code must not surface as its
    // raw identifier in the interface.
    const message = getLocalizedErrorMessage(
      new AuthApiError(400, "some_future_code", "some_future_code"),
    );
    expect(message).toBe(translate("errors.api.request_failed"));
    expect(message).not.toContain("some_future_code");
  });
});

describe("print job failure codes", () => {
  it.each([
    ["zh-CN", "print_submission_unknown", "避免重复发送"],
    ["en-US", "print_submission_unknown", "duplicate prints"],
    ["zh-CN", "print_provider_print_id_unknown", "新建一次打印"],
    ["en-US", "print_provider_print_id_unknown", "create a new print"],
  ] as const)("localizes %s %s", (locale, code, expected) => {
    setI18nLocale(locale);
    const key = getLocalizedPrintJobErrorKey(code);
    expect(key).toBe(`prints.errors.${code}`);
    expect(translate(key)).toContain(expected);
  });

  it("hides a message that is not a known code", () => {
    // Provider errors can carry request URLs and credentials, so an unknown
    // value must never reach the page.
    expect(getLocalizedPrintJobErrorKey("https://provider.example?ak=secret")).toBe("");
    expect(getLocalizedPrintJobErrorKey("")).toBe("");
    expect(getLocalizedPrintJobErrorKey(undefined)).toBe("");
  });
});
