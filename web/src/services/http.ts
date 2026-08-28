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
let authRefreshPromise: Promise<string | null> | null = null;

export function configureAuthRefresh(handler: AuthRefreshHandler | null) {
  authRefreshHandler = handler;
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

  const canRefresh =
    !init.skipAuthRefresh &&
    authRefreshHandler &&
    headers.has("Authorization") &&
    !input.includes("/auth/refresh");
  let response = await fetchWithTimeout(input, init, headers);

  if (response.status === 401 && canRefresh) {
    const nextAccessToken = await refreshAccessToken(headers.get("Authorization") ?? "");
    if (nextAccessToken) {
      headers.set("Authorization", `Bearer ${nextAccessToken}`);
      response = await fetchWithTimeout(input, init, headers);
    }
  }

  if (!response.ok) {
    let errorPayload: ApiErrorResponse | null = null;
    try {
      errorPayload = (await response.json()) as ApiErrorResponse;
    } catch {
      errorPayload = null;
    }

    throw new AuthApiError(
      response.status,
      errorPayload?.code ?? "request_failed",
      errorPayload?.message ?? "请求失败，请稍后重试。",
      errorPayload?.requestId ?? response.headers.get("X-Request-ID") ?? undefined,
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
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
    return await fetch(input, { ...init, headers, signal: controller.signal });
  } catch (error) {
    const message =
      error instanceof DOMException && error.name === "AbortError" && timedOut
        ? "请求超时，请稍后重试。"
        : error instanceof DOMException && error.name === "AbortError" && !timedOut
          ? "请求已取消。"
          : error instanceof Error
            ? `网络异常，请检查连接后重试。${error.message ? ` (${error.message})` : ""}`
            : "网络异常，请检查连接后重试。";
    throw new AuthApiError(0, "network_error", message);
  } finally {
    globalThis.clearTimeout(timeoutId);
    init.signal?.removeEventListener("abort", onAbort);
  }
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
  if (!authRefreshPromise) {
    authRefreshPromise = authRefreshHandler(authorization).finally(() => {
      authRefreshPromise = null;
    });
  }
  return authRefreshPromise;
}
