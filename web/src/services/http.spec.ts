import { afterEach, describe, expect, it, vi } from "vitest";

import { AuthApiError, configureAuthRefresh, request } from "@/services/http";

afterEach(() => {
  configureAuthRefresh(null);
  vi.restoreAllMocks();
});

describe("http client", () => {
  it("normalizes JSON requests and sends a request id header", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { "X-Request-ID": "request-1" },
      }),
    );

    await expect(
      request<{ ok: boolean }>("/api/test", { method: "POST", body: "{}" }),
    ).resolves.toEqual({
      ok: true,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/test",
      expect.objectContaining({ headers: expect.any(Headers), signal: expect.any(AbortSignal) }),
    );
    const requestHeaders = fetchMock.mock.calls[0]?.[1]?.headers;
    expect(new Headers(requestHeaders).get("X-Request-ID")).toBeTruthy();
    expect(new Headers(requestHeaders).get("Content-Type")).toBe("application/json");
  });

  it("refreshes once and retries an authorized request after a 401", async () => {
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ code: "expired" }), { status: 401 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true }), { status: 200 }));
    const refresh = vi
      .fn<(authorization: string) => Promise<string | null>>()
      .mockResolvedValue("next-token");
    configureAuthRefresh(refresh);

    await expect(
      request<{ ok: boolean }>("/api/test", {
        headers: { Authorization: "Bearer old-token" },
      }),
    ).resolves.toEqual({ ok: true });
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(refresh).toHaveBeenCalledWith("old-token");
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/test",
      expect.objectContaining({ headers: expect.any(Headers) }),
    );
    const retriedHeaders = fetchMock.mock.calls[1]?.[1]?.headers;
    expect(new Headers(retriedHeaders).get("Authorization")).toBe("Bearer next-token");
  });

  it("does not refresh an old account request after the session changes", async () => {
    let currentToken = "old-token";
    const refresh = vi
      .fn<(authorization: string) => Promise<string | null>>()
      .mockImplementation(async (authorization) =>
        authorization === currentToken ? "next-token" : null,
      );
    configureAuthRefresh(refresh);
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async () => {
      currentToken = "new-token";
      return new Response(JSON.stringify({ code: "expired" }), { status: 401 });
    });

    await expect(
      request("/api/account-a", { headers: { Authorization: "Bearer old-token" } }),
    ).rejects.toMatchObject({ status: 401 });
    expect(refresh).toHaveBeenCalledWith("old-token");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("shares one refresh across concurrent 401 responses", async () => {
    let releaseRefresh!: (token: string) => void;
    const refresh = vi.fn<(authorization: string) => Promise<string | null>>(
      () => new Promise((resolve) => (releaseRefresh = resolve)),
    );
    configureAuthRefresh(refresh);
    vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
      const authorization = new Headers(init?.headers).get("Authorization");
      return authorization === "Bearer next-token"
        ? new Response(JSON.stringify({ ok: true }), { status: 200 })
        : new Response(JSON.stringify({ code: "expired" }), { status: 401 });
    });

    const requests = [
      request<{ ok: boolean }>("/api/one", { headers: { Authorization: "Bearer old-token" } }),
      request<{ ok: boolean }>("/api/two", { headers: { Authorization: "Bearer old-token" } }),
    ];
    await vi.waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
    releaseRefresh("next-token");

    await expect(Promise.all(requests)).resolves.toEqual([{ ok: true }, { ok: true }]);
    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it("raises a timeout as a normalized network error", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(
      (_input, init) =>
        new Promise((_resolve, reject) => {
          init?.signal?.addEventListener(
            "abort",
            () => reject(new DOMException("aborted", "AbortError")),
            {
              once: true,
            },
          );
        }),
    );

    await expect(request("/api/slow", { timeoutMs: 1 })).rejects.toMatchObject({
      code: "network_error",
    } satisfies Partial<AuthApiError>);
  });

  it("normalizes a timeout while reading the response body", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) => {
      const response = new Response(null, { status: 200 });
      vi.spyOn(response, "json").mockImplementation(
        () =>
          new Promise((_resolve, reject) => {
            init?.signal?.addEventListener(
              "abort",
              () => reject(new DOMException("aborted", "AbortError")),
              { once: true },
            );
          }),
      );
      return response;
    });
    await expect(request("/slow-body", { timeoutMs: 1 })).rejects.toMatchObject({
      code: "network_error",
      message: "请求超时，请稍后重试。",
    });
  });

  it("cancels an abandoned 401 body and propagates a transient refresh failure", async () => {
    const response = new Response("expired", { status: 401 });
    const cancel = vi.spyOn(response.body!, "cancel");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(response);
    const failure = new AuthApiError(503, "request_failed", "temporary failure");
    configureAuthRefresh(async () => {
      throw failure;
    });
    await expect(request("/test", { headers: { Authorization: "Bearer token" } })).rejects.toBe(
      failure,
    );
    expect(cancel).toHaveBeenCalledTimes(1);
  });
});
