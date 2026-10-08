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
    store.remotePrintCounts = {
      pending: 1,
      queued: 0,
      completed: 0,
      failed: 0,
      cancelled: 0,
      todayCompleted: 0,
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
    const { wrapper } = await mountShellAt("/settings/devices");

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
    const { wrapper } = await mountShellAt("/settings/devices");

    expect(wrapper.text()).toContain("打印1");
    expect(wrapper.text()).toContain("name@example.com");
    expect(wrapper.text()).toContain("退出");
  });

  it("shows the author credit link next to the product name", async () => {
    const { wrapper } = await mountShellAt("/settings/devices");

    const creditLink = wrapper
      .findAll("a")
      .find((link) => link.text().includes("Powered by ruhuang2001"));

    expect(creditLink?.attributes("href")).toBe("https://github.com/ruhuang2001");
  });

  it("keeps synchronization failures visible until the affected data recovers", async () => {
    const { wrapper, store } = await mountShellAt("/settings/devices");

    store.workspaceSyncError = "草稿保存失败";
    store.printSyncError = "打印状态加载失败";
    store.aiConfigError = "AI 配置加载失败";
    store.pluginError = "插件加载失败";
    await flushPromises();

    const alert = wrapper.get('[role="alert"]');
    expect(alert.text()).toContain("工作区：草稿保存失败");
    expect(alert.text()).toContain("设备与打印：打印状态加载失败");
    expect(alert.text()).toContain("AI 服务：AI 配置加载失败");
    expect(alert.text()).toContain("扩展功能：插件加载失败");

    store.printSyncError = "";
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).not.toContain("打印状态加载失败");
    expect(wrapper.get('[role="alert"]').text()).toContain("草稿保存失败");

    store.workspaceSyncError = "";
    store.aiConfigError = "";
    store.pluginError = "";
    await flushPromises();
    expect(wrapper.find('[role="alert"]').exists()).toBe(false);
  });

  it("does not expose account synchronization errors in the anonymous demo", async () => {
    const { wrapper, store } = await mountShellAt("/conversations", false);

    store.workspaceSyncError = "账号数据保存失败";
    await flushPromises();

    expect(wrapper.find('[role="alert"]').exists()).toBe(false);
    expect(wrapper.text()).not.toContain("账号数据保存失败");
  });

  it("retries synchronization from the error message", async () => {
    const { wrapper, store } = await mountShellAt("/settings/devices");
    const retry = vi.spyOn(store, "retrySynchronization").mockResolvedValue(true);
    store.workspaceSyncError = "保存失败";
    await flushPromises();

    await wrapper.get('[role="alert"] button').trigger("click");
    expect(retry).toHaveBeenCalledOnce();
  });

  it("requires an explicit choice before discarding conflicted local edits", async () => {
    const { wrapper, store } = await mountShellAt("/settings/devices");
    const reload = vi.spyOn(store, "reloadConflictedWorkspace").mockResolvedValue(true);
    const confirm = vi
      .spyOn(window, "confirm")
      .mockReturnValueOnce(false)
      .mockReturnValueOnce(true);
    store.workspaceSyncError = "其他位置已更新";
    store.workspaceConflict = true;
    await flushPromises();

    const action = wrapper.get('[role="alert"] button');
    expect(action.text()).toBe("重新加载工作区");
    await action.trigger("click");
    expect(confirm).toHaveBeenCalledWith(expect.stringContaining("丢弃当前未保存的改动"));
    expect(reload).not.toHaveBeenCalled();
    await action.trigger("click");
    expect(reload).toHaveBeenCalledOnce();
    confirm.mockRestore();
  });

  it("hides account controls for anonymous visitors", async () => {
    const { wrapper } = await mountShellAt("/conversations", false);

    expect(wrapper.text()).toContain("登录");
    expect(wrapper.text()).toContain("当前设备、对话、打印页均为演示内容");
    expect(wrapper.text()).not.toContain("name@example.com");
    expect(wrapper.text()).not.toContain("退出");
  });

  it("disables page controls during reload and exposes a local draft download", async () => {
    const { wrapper, store } = await mountShellAt("/conversations");
    const download = vi.spyOn(store, "downloadWorkspaceDraft").mockImplementation(() => undefined);
    store.workspaceSyncError = "工作区冲突";
    store.workspaceConflict = true;
    await flushPromises();
    const action = wrapper
      .findAll('[role="alert"] button')
      .find((button) => button.text() === "下载本地草稿 JSON")!;
    await action.trigger("click");
    expect(download).toHaveBeenCalledOnce();
    store.workspaceLoading = true;
    await flushPromises();
    expect(wrapper.get("main fieldset").attributes("disabled")).toBeDefined();
    expect(wrapper.get("main fieldset").attributes("inert")).toBeDefined();
    expect(action.attributes("disabled")).toBeDefined();
    store.workspaceLoading = false;
    store.workspaceConflict = false;
    await flushPromises();
    expect(wrapper.get("main fieldset").attributes("disabled")).toBeUndefined();
    expect(wrapper.text()).not.toContain("下载本地草稿 JSON");
  });

  it("shows the demo banner on public workspace pages", async () => {
    const { wrapper } = await mountShellAt("/conversations", false);

    expect(wrapper.text()).toContain("具体使用请登录后继续");
  });

  it("routes anonymous visitors to login from the header action", async () => {
    const { wrapper, router } = await mountShellAt("/conversations", false);
    const loginLink = wrapper.findAll("a").find((link) => link.text() === "登录");

    expect(loginLink?.exists()).toBe(true);

    await loginLink?.trigger("click");
    await flushPromises();
    await vi.waitFor(() => {
      expect(router.currentRoute.value.fullPath).toBe("/login");
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

  it("keeps device and guide pages inside the settings navigation", async () => {
    const { wrapper } = await mountShellAt("/settings/guide");

    expect(wrapper.findAll("header nav a").map((link) => link.text())).toEqual([
      "对话",
      "打印1",
      "设置",
    ]);
    expect(wrapper.text()).toContain("偏好");
    expect(wrapper.text()).toContain("设备");
    expect(wrapper.text()).toContain("使用指南");
  });
});
