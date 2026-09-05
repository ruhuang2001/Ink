import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, vi } from "vitest";
import { nextTick } from "vue";

import AppRoot from "@/app/AppRoot.vue";
import { createTestRouter } from "@/router";
import { useWorkspaceStore } from "@/stores/workspace";

type MatchMediaEventHandler = (
  type: string,
  listener: EventListenerOrEventListenerObject | null,
) => void;
type LegacyMediaQueryListenerHandler = (
  listener: ((event: MediaQueryListEvent) => void) | null,
) => void;
type MatchMediaDispatchEventHandler = (event: Event) => boolean;
type MatchMediaFactory = (query: string) => MediaQueryList;

function createMatchMediaMock(initialMatches = false) {
  let matches = initialMatches;
  const listeners = new Set<EventListenerOrEventListenerObject>();
  const notify = (event: MediaQueryListEvent) => {
    for (const listener of listeners) {
      if (typeof listener === "function") {
        listener.call(mediaQueryList, event);
        continue;
      }

      listener.handleEvent(event);
    }
  };
  const mediaQueryList = {
    get matches() {
      return matches;
    },
    media: "(prefers-color-scheme: dark)",
    onchange: null,
    addEventListener: vi.fn<MatchMediaEventHandler>(
      (_type: string, listener: EventListenerOrEventListenerObject | null) => {
        if (!listener) {
          return;
        }

        listeners.add(listener);
      },
    ),
    removeEventListener: vi.fn<MatchMediaEventHandler>(
      (_type: string, listener: EventListenerOrEventListenerObject | null) => {
        if (!listener) {
          return;
        }

        listeners.delete(listener);
      },
    ),
    addListener: vi.fn<LegacyMediaQueryListenerHandler>(
      (listener: ((event: MediaQueryListEvent) => void) | null) => {
        if (!listener) {
          return;
        }

        listeners.add(listener as unknown as EventListener);
      },
    ),
    removeListener: vi.fn<LegacyMediaQueryListenerHandler>(
      (listener: ((event: MediaQueryListEvent) => void) | null) => {
        if (!listener) {
          return;
        }

        listeners.delete(listener as unknown as EventListener);
      },
    ),
    dispatch(nextMatches: boolean) {
      matches = nextMatches;
      const event = { matches: nextMatches, media: mediaQueryList.media } as MediaQueryListEvent;
      notify(event);
    },
    dispatchEvent: vi.fn<MatchMediaDispatchEventHandler>(() => true),
  } satisfies MediaQueryList & { dispatch(nextMatches: boolean): void };

  return mediaQueryList;
}

let matchMediaMock = createMatchMediaMock();

beforeEach(() => {
  matchMediaMock = createMatchMediaMock();
  vi.stubGlobal(
    "matchMedia",
    vi.fn<MatchMediaFactory>(() => matchMediaMock),
  );
});

async function mountAt(path: string, authenticated = true) {
  const pinia = createPinia();
  setActivePinia(pinia);
  const store = useWorkspaceStore();

  if (authenticated) {
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
  }

  const router = createTestRouter(pinia);

  router.push(path);
  await router.isReady();

  const wrapper = mount(AppRoot, {
    global: {
      plugins: [pinia, router],
    },
  });

  return { wrapper, router, store };
}

describe("AppRoot", () => {
  it.each([
    ["/status", "设备"],
    ["/conversations", "对话"],
    ["/prints", "打印"],
    ["/settings", "设置"],
    ["/tutorial", "教程"],
  ])("renders the workspace shell for %s when authenticated", async (path, heading) => {
    const { wrapper } = await mountAt(path, true);

    expect(wrapper.find("header").exists()).toBe(true);
    expect(wrapper.find("nav.fixed").exists()).toBe(true);
    expect(wrapper.text()).toContain(heading);
  });

  it("renders the workspace shell for anonymous visitors on the public status page", async () => {
    const { wrapper, router } = await mountAt("/status", false);

    expect(router.currentRoute.value.fullPath).toBe("/status");
    expect(wrapper.find("header nav").exists()).toBe(true);
    expect(wrapper.find("nav.fixed").exists()).toBe(true);
    expect(wrapper.text()).toContain("登录");
    expect(wrapper.text()).toContain("演示工作区 · 数据仅供体验");
    expect(wrapper.text()).not.toContain("退出");
  });

  it("renders the login view outside the workspace shell for anonymous settings access", async () => {
    const { wrapper, router } = await mountAt("/settings", false);

    expect(router.currentRoute.value.fullPath).toBe("/login?redirect=/settings");
    expect(wrapper.text()).toContain("登录账号");
    expect(wrapper.find("header nav").exists()).toBe(false);
    expect(wrapper.find("nav.fixed").exists()).toBe(false);
  });

  it("syncs the selected theme to the document and theme-color meta", async () => {
    let themeMeta = document.querySelector('meta[name="theme-color"]');
    if (!themeMeta) {
      themeMeta = document.createElement("meta");
      themeMeta.setAttribute("name", "theme-color");
      document.head.appendChild(themeMeta);
    }
    themeMeta.setAttribute("content", "#000000");

    const { store } = await mountAt("/status");
    store.selectedTheme = "dark";
    await nextTick();

    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(document.documentElement.dataset.colorMode).toBe("dark");
    expect(document.documentElement.style.colorScheme).toBe("dark");
    expect(themeMeta.getAttribute("content")).toBe("#14110f");
  });

  it("reacts to system color-scheme changes when the theme follows the system", async () => {
    const { store } = await mountAt("/status");
    store.selectedTheme = "system";
    await nextTick();

    expect(document.documentElement.dataset.theme).toBe("system");
    expect(document.documentElement.dataset.colorMode).toBe("light");

    matchMediaMock.dispatch(true);
    await nextTick();

    expect(document.documentElement.dataset.colorMode).toBe("dark");
    expect(document.documentElement.style.colorScheme).toBe("dark");
  });
});
