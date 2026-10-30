import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { configureAuthRefresh } from "@/services/http";
import { getMemorySettings, updateMemorySettings } from "@/services/memory";

describe("memory settings service", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    configureAuthRefresh(null);
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    fetchMock.mockReset();
  });

  it("sends the access token when reading memory settings", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          enabled: false,
          zepBaseUrl: "",
          keyConfigured: false,
          available: false,
        }),
      ),
    );

    await getMemorySettings("token");

    const [input, init] = fetchMock.mock.calls[0]!;
    expect(new URL(String(input), "https://ink.example").pathname).toBe("/api/v1/memory/settings");
    expect(init?.method).toBe("GET");
    expect(new Headers(init?.headers).get("Authorization")).toBe("Bearer token");
  });

  it("sends the access token when saving memory settings", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          enabled: true,
          zepBaseUrl: "https://zep.example.com",
          keyConfigured: true,
          available: true,
        }),
      ),
    );

    await updateMemorySettings("token", {
      enabled: true,
      zepBaseUrl: "https://zep.example.com",
      zepApiKey: "secret",
    });

    const [input, init] = fetchMock.mock.calls[0]!;
    expect(new URL(String(input), "https://ink.example").pathname).toBe(
      "/api/v1/admin/memory/settings",
    );
    expect(init?.method).toBe("PUT");
    expect(new Headers(init?.headers).get("Authorization")).toBe("Bearer token");
    expect(JSON.parse(String(init?.body))).toEqual({
      enabled: true,
      zepBaseUrl: "https://zep.example.com",
      zepApiKey: "secret",
    });
  });
});
