import { describe, expect, it } from "vitest";

import { setI18nLocale, translate } from "@/i18n";
import { getLocalizedErrorMessage } from "@/i18n/errors";
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
});
