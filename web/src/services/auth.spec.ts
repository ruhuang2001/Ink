import { afterEach, beforeEach, expect, it, vi } from "vitest";

import {
  AuthApiError,
  changePasswordWithApi,
  fetchCurrentUser,
  loginWithApi,
  logoutWithApi,
  refreshAuthSession,
} from "@/services/auth";

const fetchMock = vi.fn<typeof fetch>();

describe("auth service", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    fetchMock.mockReset();
  });

  it("maps login responses into user and session payloads", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          user: {
            id: "user-1",
            email: "name@example.com",
            name: "Ink User",
            role: "member",
          },
          accessToken: "access-token",
          refreshToken: "refresh-token",
          expiresIn: 900,
        }),
        { status: 200 },
      ),
    );

    await expect(
      loginWithApi({
        email: "name@example.com",
        password: "demo-password",
      }),
    ).resolves.toMatchObject({
      user: {
        email: "name@example.com",
      },
      session: {
        accessToken: "access-token",
        refreshToken: "refresh-token",
      },
    });
  });

  it("refreshes sessions and fetches the current user", async () => {
    fetchMock
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            user: {
              id: "user-1",
              email: "name@example.com",
              name: "Ink User",
              role: "member",
            },
            accessToken: "next-access",
            refreshToken: "next-refresh",
            expiresIn: 900,
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            user: {
              id: "user-1",
              email: "name@example.com",
              name: "Ink User",
              role: "member",
            },
          }),
          { status: 200 },
        ),
      );

    await expect(refreshAuthSession("old-refresh")).resolves.toMatchObject({
      session: {
        accessToken: "next-access",
        refreshToken: "next-refresh",
      },
    });
    await expect(fetchCurrentUser("next-access")).resolves.toMatchObject({
      id: "user-1",
    });
  });

  it("sends logout and change password requests, and surfaces api errors", async () => {
    fetchMock
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            code: "invalid_credentials",
            message: "Invalid email or password.",
            requestId: "req-1",
          }),
          { status: 401 },
        ),
      );

    await expect(
      logoutWithApi({
        accessToken: "access-token",
        refreshToken: "refresh-token",
      }),
    ).resolves.toBeUndefined();
    await expect(
      changePasswordWithApi({
        accessToken: "access-token",
        currentPassword: "demo-password",
        newPassword: "next-password",
      }),
    ).resolves.toBeUndefined();
    await expect(
      loginWithApi({
        email: "name@example.com",
        password: "wrong",
      }),
    ).rejects.toEqual(
      new AuthApiError(401, "invalid_credentials", "Invalid email or password.", "req-1"),
    );

    fetchMock.mockRejectedValueOnce(new Error("network error"));

    await expect(
      loginWithApi({
        email: "name@example.com",
        password: "wrong",
      }),
    ).rejects.toMatchObject({
      status: 0,
      code: "network_error",
    });
  });
});
