import { createPinia, setActivePinia } from "pinia";
import { afterEach, vi } from "vitest";
import { createMemoryHistory } from "vue-router";

import { createAppRouter, navigationItems, routes } from "@/router";
import { useWorkspaceStore } from "@/stores/workspace";

afterEach(() => {
  vi.restoreAllMocks();
});

function createAuthenticatedRouter() {
  const pinia = createPinia();
  setActivePinia(pinia);
  const store = useWorkspaceStore();
  store.authUser = {
    id: "user-1",
    email: "name@example.com",
    name: "Ink User",
    role: "member",
  };
  store.authSession = {
    accessToken: "access-token",
    refreshToken: "refresh-token",
    accessTokenExpiresAt: new Date(Date.now() + 60_000).toISOString(),
  };

  return createAppRouter(createMemoryHistory(), pinia);
}

describe("router configuration", () => {
  it("keeps navigation items in sync with workspace routes", () => {
    const workspaceRoute = routes.find((route) => route.path === "/");
    const shellChildren = (workspaceRoute?.children ?? []).filter(
      (route) => route.meta?.showInNav !== false,
    );

    expect(navigationItems).toHaveLength(shellChildren.length);
    expect(navigationItems.map((item) => item.path)).toEqual([
      "/conversations",
      "/prints",
      "/settings",
    ]);
  });

  it("redirects anonymous visitors from the root route to the public conversations page", async () => {
    const pinia = createPinia();
    const router = createAppRouter(createMemoryHistory(), pinia);

    router.push("/");
    await router.isReady();

    expect(router.currentRoute.value.fullPath).toBe("/conversations");
  });

  it.each(["/conversations", "/prints"])("allows anonymous visitors to reach %s", async (path) => {
    const pinia = createPinia();
    const router = createAppRouter(createMemoryHistory(), pinia);

    router.push(path);
    await router.isReady();

    expect(router.currentRoute.value.fullPath).toBe(path);
  });

  it.each([
    ["/status", "/settings/devices"],
    ["/tutorial", "/settings/guide"],
  ])("keeps the legacy %s route as a redirect to %s", async (source, destination) => {
    const router = createAuthenticatedRouter();

    router.push(source);
    await router.isReady();

    expect(router.currentRoute.value.fullPath).toBe(destination);
  });

  it.each(["/settings", "/settings/devices", "/settings/guide"])(
    "requires authentication for %s",
    async (path) => {
      const pinia = createPinia();
      const router = createAppRouter(createMemoryHistory(), pinia);

      router.push(path);
      await router.isReady();

      expect(router.currentRoute.value.fullPath).toBe(`/login?redirect=${path}`);
    },
  );

  it("redirects the retired connections route to /prints for authenticated visitors", async () => {
    const router = createAuthenticatedRouter();

    router.push("/connections");
    await router.isReady();

    expect(router.currentRoute.value.fullPath).toBe("/prints");
  });

  it("restores a persisted session before entering settings", async () => {
    vi.spyOn(globalThis, "fetch")
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
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            devices: [],
            conversations: [],
            activeConversationId: "",
            printJobs: [],
            schedules: [],
            sources: [],
            preferences: {
              loginProtectionEnabled: false,
              sendConfirmationEnabled: false,
              tutorialTabEnabled: true,
              theme: "light",
              defaultDeviceId: "",
              locale: "system",
            },
            serviceBinding: {
              providerName: null,
              modelName: "Ink AI",
              bound: false,
            },
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            bound: false,
            providerName: "OpenAI Compatible",
            providerType: "openai-compatible",
            baseUrl: "",
            model: "gpt-4.1-mini",
            keyConfigured: false,
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            devices: [],
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            printJobs: [],
          }),
          { status: 200 },
        ),
      );

    const pinia = createPinia();
    setActivePinia(pinia);
    window.localStorage.setItem(
      "ink.auth.session.v1",
      JSON.stringify({
        accessToken: "access-token",
        refreshToken: "refresh-token",
        accessTokenExpiresAt: new Date(Date.now() + 60_000).toISOString(),
      }),
    );
    const store = useWorkspaceStore();

    const router = createAppRouter(createMemoryHistory(), pinia);
    router.push("/settings");
    await router.isReady();

    expect(router.currentRoute.value.fullPath).toBe("/settings");
    expect(store.authUser?.email).toBe("name@example.com");
  });

  it.each([
    ["/login", "/conversations"],
    ["/login?redirect=/settings", "/settings"],
    ["/login?redirect=/missing", "/conversations"],
    ["/login?redirect=/login", "/conversations"],
  ])("redirects authenticated visitors from %s to %s", async (source, destination) => {
    const router = createAuthenticatedRouter();

    router.push(source);
    await router.isReady();

    expect(router.currentRoute.value.fullPath).toBe(destination);
  });

  it("updates the document title from route metadata", async () => {
    const router = createAuthenticatedRouter();

    router.push("/settings");
    await router.isReady();

    expect(document.title).toBe("Ink · 偏好设置");
  });
});
