import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";

import { translate } from "@/i18n";
import AppShell from "@/layouts/AppShell.vue";
import { createTestRouter, navigationItems } from "@/router";
import { useWorkspaceStore } from "@/stores/workspace";

async function mountShellAt(path: string, authenticated = true) {
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

  const wrapper = mount(AppShell, {
    global: {
      plugins: [pinia, router],
    },
  });

  return { wrapper, router, store };
}

describe("AppShell", () => {
  it("renders desktop and mobile navigation from router metadata", async () => {
    const { wrapper } = await mountShellAt("/status");

    const desktopNavLinks = wrapper.findAll("header nav a");
    const mobileNavLinks = wrapper.findAll("nav.fixed a");

    expect(desktopNavLinks).toHaveLength(navigationItems.length);
    expect(mobileNavLinks).toHaveLength(navigationItems.length);
    expect(desktopNavLinks.map((link) => link.text().replace(/\d+/g, ""))).toEqual(
      navigationItems.map((item) => translate(item.labelKey)),
    );
    expect(mobileNavLinks.map((link) => link.text().replace(/\s*·\s*\d+/g, ""))).toEqual(
      navigationItems.map((item) => translate(item.labelKey)),
    );
  });

  it("shows the pending print badge and authenticated account controls", async () => {
    const { wrapper } = await mountShellAt("/status");

    expect(wrapper.text()).toContain("打印1");
    expect(wrapper.find("button.ink-account").attributes("title")).toBe("name@example.com");
    expect(wrapper.text()).toContain("退出");
  });

  it("switches visual styles and persists the choice across mounts", async () => {
    const { wrapper } = await mountShellAt("/status");
    await wrapper.find("select").setValue("studio");
    expect(document.documentElement.dataset.visualStyle).toBe("studio");
    expect(localStorage.getItem("ink.visual-style")).toBe("studio");
    wrapper.unmount();
    const { wrapper: restored } = await mountShellAt("/prints");
    expect((restored.find("select").element as HTMLSelectElement).value).toBe("studio");
    await restored.find("select").setValue("paper");
    expect(document.documentElement.dataset.visualStyle).toBe("paper");
  });

  it("hides account controls for anonymous visitors", async () => {
    const { wrapper } = await mountShellAt("/status", false);

    expect(wrapper.text()).toContain("登录");
    expect(wrapper.text()).toContain("演示工作区 · 数据仅供体验");
    expect(wrapper.text()).not.toContain("name@example.com");
    expect(wrapper.text()).not.toContain("退出");
  });

  it("shows the demo banner only on the first three tabs for anonymous visitors", async () => {
    const { wrapper: statusWrapper } = await mountShellAt("/status", false);
    const { wrapper: tutorialWrapper } = await mountShellAt("/tutorial", false);

    expect(statusWrapper.text()).toContain("演示工作区 · 数据仅供体验");
    expect(tutorialWrapper.text()).not.toContain("演示工作区 · 数据仅供体验");
  });

  it("routes anonymous visitors to login from the header action", async () => {
    const { wrapper, router } = await mountShellAt("/status", false);
    const loginLink = wrapper.find("header a.ink-account");

    expect(loginLink?.exists()).toBe(true);

    await loginLink?.trigger("click");
    await flushPromises();
    await vi.waitFor(() => {
      expect(router.currentRoute.value.fullPath).toBe("/login?redirect=/status");
    });
  });

  it("logs out and returns to conversations when the header logout action is used", async () => {
    const { wrapper, router, store } = await mountShellAt("/prints");
    const logoutButton = wrapper.findAll("button").find((button) => button.text() === "退出");

    expect(logoutButton?.exists()).toBe(true);

    await logoutButton?.trigger("click");
    await flushPromises();

    expect(store.isAuthenticated).toBe(false);
    await vi.waitFor(() => {
      expect(router.currentRoute.value.fullPath).toBe("/conversations");
    });
  });

  it("shows and dismisses the post-login binding tutorial dialog", async () => {
    const { wrapper, store } = await mountShellAt("/conversations");

    store.postLoginTutorialOpen = true;
    await flushPromises();

    expect(wrapper.text()).toContain("登录成功后先绑定设备");
    expect(wrapper.text()).toContain("双击开机键，先打印状态纸条");

    await wrapper
      .findAll("button")
      .find((button) => button.text() === "稍后再看")
      ?.trigger("click");
    await flushPromises();

    expect(store.postLoginTutorialOpen).toBe(false);
  });

  it("hides the tutorial tab when the preference is disabled", async () => {
    const { wrapper, store } = await mountShellAt("/conversations");

    store.tutorialTabEnabled = false;
    await flushPromises();

    expect(wrapper.findAll("header nav a").some((link) => link.text().includes("教程"))).toBe(
      false,
    );
    expect(wrapper.findAll("nav.fixed a").some((link) => link.text().includes("教程"))).toBe(false);
  });
});
