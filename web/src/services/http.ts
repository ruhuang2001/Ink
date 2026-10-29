export interface ApiErrorResponse {
  code?: string;
  message?: string;
  requestId?: string;
}

export class AuthApiError extends Error {
  status: number;
  code: string;
  requestId?: string;

  constructor(status: number, code: string, message: string, requestId?: string) {
    super(message);
    this.name = "AuthApiError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }
}

const DEFAULT_TIMEOUT_MS = 15000;

export interface HttpRequestInit extends RequestInit {
  timeoutMs?: number;
  skipAuthRefresh?: boolean;
}

type AuthRefreshHandler = (accessToken: string) => Promise<string | null>;
let authRefreshHandler: AuthRefreshHandler | null = null;
let authRefreshOperation: { accessToken: string; promise: Promise<string | null> } | null = null;

export function configureAuthRefresh(handler: AuthRefreshHandler | null) {
  authRefreshHandler = handler;
  if (!handler) {
    authRefreshOperation = null;
  }
}

export async function request<T>(input: string, init: HttpRequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (
    !headers.has("Content-Type") &&
    !(typeof FormData !== "undefined" && init.body instanceof FormData)
  ) {
    headers.set("Content-Type", "application/json");
  }
  if (!headers.has("X-Request-ID")) {
    headers.set("X-Request-ID", createRequestId());
  }

  const canRefresh = !init.skipAuthRefresh && authRefreshHandler && headers.has("Authorization");
  let attempt = await fetchWithTimeout(input, init, headers);
  let response = attempt.response;

  try {
    if (response.status === 401 && canRefresh) {
      await response.body?.cancel().catch(() => undefined);
      attempt.cleanup();
      const accessToken = headers.get("Authorization")?.replace(/^Bearer\s+/i, "") ?? "";
      let nextAccessToken: string | null;
      try {
        nextAccessToken = await refreshAccessToken(accessToken);
      } catch {
        throw new AuthApiError(
          401,
          "auth_refresh_failed",
          "Session refresh failed. The original request was not sent. Please try again or sign in again.",
        );
      }
      if (init.signal?.aborted) {
        throw new AuthApiError(
          401,
          "auth_refresh_failed",
          "The request was canceled during session refresh and was not sent.",
        );
      }
      if (nextAccessToken) {
        headers.set("Authorization", `Bearer ${nextAccessToken}`);
        attempt = await fetchWithTimeout(input, init, headers);
        response = attempt.response;
      }
    }
    if (!response.ok) {
      let errorPayload: ApiErrorResponse | null = null;
      try {
        errorPayload = (await attempt.readJSON()) as ApiErrorResponse;
      } catch (error) {
        // A 5xx can follow an accepted mutation; an unreadable error body
        // cannot establish whether it completed. Preserve that uncertainty.
        // Explicit 4xx rejection statuses remain authoritative.
        if (error instanceof AuthApiError && (response.status >= 500 || error.status !== 0))
          throw error;
        errorPayload = null;
      }

      throw new AuthApiError(
        response.status,
        errorPayload?.code ?? "request_failed",
        errorPayload?.message ?? "Request failed. Please try again later.",
        errorPayload?.requestId ?? response.headers.get("X-Request-ID") ?? undefined,
      );
    }

    if (response.status === 204) {
      return undefined as T;
    }

    return (await attempt.readJSON()) as T;
  } finally {
    attempt.cleanup();
  }
}

async function fetchWithTimeout(input: string, init: HttpRequestInit, headers: Headers) {
  const controller = new AbortController();
  const timeoutMs = init.timeoutMs ?? DEFAULT_TIMEOUT_MS;
  let timedOut = false;
  const timeoutHandler = () => {
    timedOut = true;
    controller.abort();
  };
  const timeoutId = globalThis.setTimeout(timeoutHandler, timeoutMs);
  const onAbort = () => controller.abort();
  if (init.signal?.aborted) {
    controller.abort();
  } else {
    init.signal?.addEventListener("abort", onAbort, { once: true });
  }

  try {
    const response = await fetch(input, { ...init, headers, signal: controller.signal });
    return {
      response,
      readJSON: async (): Promise<unknown> => {
        try {
          return await response.json();
        } catch (error) {
          throw networkError(error, timedOut);
        }
      },
      cleanup: () => {
        globalThis.clearTimeout(timeoutId);
        init.signal?.removeEventListener("abort", onAbort);
      },
    };
  } catch (error) {
    globalThis.clearTimeout(timeoutId);
    init.signal?.removeEventListener("abort", onAbort);
    throw networkError(error, timedOut);
  }
}

function networkError(error: unknown, timedOut: boolean) {
  const message =
    error instanceof DOMException && error.name === "AbortError" && timedOut
      ? "Request timed out. Please try again later."
      : error instanceof DOMException && error.name === "AbortError" && !timedOut
        ? "Request was canceled."
        : error instanceof Error
          ? `A network error occurred. Check your connection and try again.${error.message ? ` (${error.message})` : ""}`
          : "A network error occurred. Check your connection and try again.";
  return new AuthApiError(0, "network_error", message);
}

function createRequestId() {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `ink-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

async function refreshAccessToken(authorization: string) {
  if (!authRefreshHandler) {
    return null;
  }
  if (!authRefreshOperation || authRefreshOperation.accessToken !== authorization) {
    const promise = authRefreshHandler(authorization).finally(() => {
      if (authRefreshOperation?.promise === promise) {
        authRefreshOperation = null;
      }
    });
    authRefreshOperation = { accessToken: authorization, promise };
  }
  return authRefreshOperation.promise;
}
