import { AuthApiError, request } from "@/services/http";
import type { AuthSession, User } from "@/types/workspace";

export { AuthApiError } from "@/services/http";

export interface LoginPayload {
  email: string;
  password: string;
}

export interface LogoutPayload {
  accessToken: string;
  refreshToken: string;
}

export interface ChangePasswordPayload {
  accessToken: string;
  currentPassword: string;
  newPassword: string;
}

interface AuthResponse {
  user: User;
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

interface MeResponse {
  user: User;
}

function buildSession(response: AuthResponse): AuthSession {
  const expiresInMs = Number(response.expiresIn) * 1000;
  if (!Number.isFinite(expiresInMs) || expiresInMs <= 0) {
    throw new AuthApiError(500, "invalid_session_payload", "登录会话无效，请重新登录。");
  }

  return {
    accessToken: response.accessToken,
    refreshToken: response.refreshToken,
    accessTokenExpiresAt: new Date(Date.now() + expiresInMs).toISOString(),
  };
}

export async function loginWithApi(payload: LoginPayload) {
  const response = await request<AuthResponse>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify(payload),
  });

  return {
    user: response.user,
    session: buildSession(response),
  };
}

export async function refreshAuthSession(refreshToken: string) {
  const response = await request<AuthResponse>("/api/v1/auth/refresh", {
    method: "POST",
    body: JSON.stringify({ refreshToken }),
  });

  return {
    user: response.user,
    session: buildSession(response),
  };
}

export async function fetchCurrentUser(accessToken: string) {
  const response = await request<MeResponse>("/api/v1/auth/me", {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });

  return response.user;
}

export async function logoutWithApi(payload: LogoutPayload) {
  await request<void>("/api/v1/auth/logout", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${payload.accessToken}`,
    },
    body: JSON.stringify({
      refreshToken: payload.refreshToken,
    }),
  });
}

export async function changePasswordWithApi(payload: ChangePasswordPayload) {
  await request<void>("/api/v1/auth/change-password", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${payload.accessToken}`,
    },
    body: JSON.stringify({
      currentPassword: payload.currentPassword,
      newPassword: payload.newPassword,
    }),
  });
}
