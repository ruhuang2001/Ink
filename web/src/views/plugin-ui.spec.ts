import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { nextTick } from "vue";

import { createTestRouter } from "@/router";
import { useWorkspaceStore } from "@/stores/workspace";
import PrintsView from "@/views/PrintsView.vue";
import SettingsView from "@/views/SettingsView.vue";

function buildExampleUrl(path: string) {
  var normalizedPath = path;
  if (normalizedPath.charCodeAt(0) === 47) {
    normalizedPath = normalizedPath.slice(1);
  }

  return ["https://example.test/", normalizedPath].join("");
}

function buildFixtureRepoUrl() {
  return "https://github.com/example/ink-plugin.git";
}

function buildToken(kind: string) {
  return [kind, "session", "token"].join("-");
}

function char(code: number) {
  return String.fromCharCode(code);
}

function textContains(source: string, query: string) {
  return source.split(query).length > 1;
}

function getSecretPlaceholder() {
  return (
    char(30041) +
    char(31354) +
    char(21017) +
    char(20445) +
    char(25345) +
    char(24403) +
    char(21069) +
    char(23494) +
    char(38053)
  );
}

function findFormByText(wrapper: ReturnType<typeof mount>, text: string) {
  var forms = wrapper.findAll("form");
  var form = forms.shift();

  while (form !== undefined) {
    if (textContains(form.text(), text)) {
      return form;
    }

    form = forms.shift();
  }

  return wrapper.find("form[data-missing='true']");
}

function findInputByPlaceholder(wrapper: ReturnType<typeof mount>, placeholder: string) {
  var inputs = wrapper.findAll("input");
  var input = inputs.shift();

  while (input !== undefined) {
    if (input.attributes("placeholder") === placeholder) {
      return input;
    }

    input = inputs.shift();
  }

  return inputs.shift();
}

function missingTestElement(label: string): Error {
  return new Error(["missing test element: ", label].join(""));
}

function setNumberInputValue(wrapper: ReturnType<typeof mount>, inputIndex: number, value: number) {
  var inputs = wrapper.findAll("input");
  var input = inputs.shift();
  var remainingIndex = inputIndex;
  var targetInput = wrapper.find("input[data-missing='true']");

  while (input !== undefined) {
    if (input.attributes("type") === "number") {
      if (remainingIndex === 0) {
        targetInput = input;
        break;
      }

      remainingIndex -= 1;
    }

    input = inputs.shift();
  }

  if (!targetInput.exists()) {
    throw missingTestElement("number input");
  }

  return targetInput.setValue(value.toString());
}

function createPluginDetails() {
  return {
    installation: {
      id: "plugin-installation-1",
      pluginKey: "demo-source",
      sourceType: "git" as const,
      displayName: "Demo Source",
      version: "1.0.0",
      runtimeType: "node" as const,
      status: "ready" as const,
      repoUrl: buildExampleUrl("example/demo-source.git"),
      repoRef: "main",
    },
    manifest: {
      schemaVersion: 2,
      kind: "source" as const,
      pluginKey: "demo-source",
      name: "Demo Source",
      version: "1.0.0",
      description: "A demo source plugin.",
      runtime: {
        type: "node" as const,
      },
      fetchPolicy: { type: "fixed_interval" as const, minutes: 15 },
      entrypoints: {
        validate: {
          command: ["pnpm", "validate"],
        },
        fetch: {
          command: ["pnpm", "fetch"],
        },
      },
      workspaceConfigSchema: [
        {
          key: "feedUrl",
          label: "Feed URL",
          type: "url" as const,
          required: true,
          description: buildExampleUrl("feed"),
        },
        {
          key: "apiKey",
          label: "API Key",
          type: "secret" as const,
          required: false,
        },
      ],
    },
    binding: {
      id: "binding-1",
      enabled: true,
      status: "connected" as const,
      config: {
        feedUrl: buildExampleUrl("feed"),
      },
      lastFetchAt: new Date("2026-04-10T00:20:00.000Z").toISOString(),
      nextFetchAt: new Date("2026-04-10T00:35:00.000Z").toISOString(),
    },
  };
}

function createUpdatedPluginDetails() {
  return {
    ...createPluginDetails(),
    manifest: {
      ...createPluginDetails().manifest,
      workspaceConfigSchema: [
        {
          key: "endpoint",
          label: "Endpoint",
          type: "url" as const,
          required: true,
          description: buildExampleUrl("api"),
        },
        {
          key: "apiKey",
          label: "API Key",
          type: "secret" as const,
          required: false,
        },
      ],
    },
    binding: {
      ...createPluginDetails().binding,
      config: {
        endpoint: buildExampleUrl("api"),
      },
    },
  };
}

function createPermissionedPluginDetails() {
  const plugin = createPluginDetails();

  return {
    ...plugin,
    manifest: {
      ...plugin.manifest,
      permissions: {
        network: {
          mode: "declared_hosts" as const,
          hosts: ["api.example.com", "*.example.org"],
        },
        filesystem: {
          temp: true,
          cache: true,
        },
        installScripts: true,
      },
    },
  };
}

function createSchedule() {
  return {
    id: "schedule-1",
    title: "Morning Digest",
    source: "Demo Source",
    timeLabel: "每天 08:30",
    deviceId: "device-1",
    enabled: true,
    pluginInstallationId: "plugin-installation-1",
    pluginBindingId: "binding-1",
    frequencyType: "daily" as const,
    timezone: "Asia/Shanghai",
    hour: 8,
    minute: 30,
    weekdays: [],
    printPolicy: { batchSize: 1 },
    pluginDisplayName: "Demo Source",
    nextRunAt: new Date("2026-04-11T00:30:00.000Z").toISOString(),
    lastRunAt: new Date("2026-04-10T00:30:00.000Z").toISOString(),
    lastError: "",
    sourceLabel: "Demo Source",
  };
}

async function createWorkspaceContext(path: string, role: "admin" | "member" = "member") {
  const pinia = createPinia();
  setActivePinia(pinia);
  const store = useWorkspaceStore();

  store.authUser = {
    id: "user-1",
    email: role === "admin" ? "admin" : "member",
    name: role === "admin" ? "Administrator" : "Ink User",
    role,
  };
  store.authSession = {
    accessToken: buildToken("access"),
    refreshToken: buildToken("refresh"),
    accessTokenExpiresAt: new Date(Date.now() + 60_000).toISOString(),
  };

  const router = createTestRouter(pinia);
  router.push(path);
  await router.isReady();

  return { pinia, router, store };
}

async function submitGitInstallForm(
  wrapper: ReturnType<typeof mount>,
  repoUrl: string,
  repoRef: string,
  repoSubdir: string,
) {
  await wrapper
    .findAll("button")
    .find((button) => button.text() === "添加插件")
    ?.trigger("click");
  await nextTick();
  await wrapper
    .findAll("button")
    .find((button) => button.text() === "GitHub 导入")
    ?.trigger("click");
  await nextTick();

  const gitInstallForm = findFormByText(wrapper, "导入插件");
  const gitInputs = gitInstallForm?.findAll("input") ?? [];
  await gitInputs[0]?.setValue(repoUrl);
  await gitInputs[1]?.setValue(repoRef);
  await gitInputs[2]?.setValue(repoSubdir);
  await gitInstallForm?.trigger("submit");
}

async function openPluginConfigDialog(wrapper: ReturnType<typeof mount>, pluginName: string) {
  const card = wrapper
    .findAll("div.rounded-2xl")
    .find((element) => element.text().includes(pluginName));
  await card
    ?.findAll("button")
    .find((button) => button.text() === "配置插件")
    ?.trigger("click");
  await nextTick();
}

describe("plugin ui flows", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
    window.sessionStorage.clear();
    vi.stubGlobal(
      "confirm",
      vi.fn(() => true),
    );
  });

  it("renders admin plugin controls and submits plugin upload/test/save actions", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/settings", "admin");
    const plugin = createPluginDetails();
    store.availablePlugins = [plugin];
    store.adminPlugins = [plugin];

    const uploadSpy = vi.spyOn(store, "uploadPlugin").mockResolvedValue(plugin);
    const installGitSpy = vi.spyOn(store, "installPluginRepository").mockResolvedValue(plugin);
    const testSpy = vi
      .spyOn(store, "testPluginConfiguration")
      .mockResolvedValue({ valid: true, errors: [] });
    const saveSpy = vi.spyOn(store, "savePluginConfiguration").mockResolvedValue(plugin);

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    expect(wrapper.text()).toContain("添加插件");
    expect(wrapper.text()).toContain("已安装插件");
    expect(wrapper.text()).toContain("Demo Source");

    await wrapper
      .findAll("button")
      .find((button) => button.text() === "添加插件")
      ?.trigger("click");
    await nextTick();

    const securityNote = wrapper.find("[role='note']");
    expect(securityNote.exists()).toBe(true);
    expect(securityNote.text()).toContain("插件及其依赖会在服务端执行");
    expect(securityNote.text()).toContain("仅是插件声明");
    expect(securityNote.text()).toContain("并不代表由沙箱强制执行的授权");

    const fileInput = wrapper.find("input[type='file']");
    Object.defineProperty(fileInput.element, "files", {
      value: [new File(["zip-content"], "demo-source.zip", { type: "application/zip" })],
    });
    await fileInput.trigger("change");
    await submitGitInstallForm(wrapper, buildFixtureRepoUrl(), "main", "plugins/hello-node");
    await openPluginConfigDialog(wrapper, "Demo Source");

    const feedInput = findInputByPlaceholder(wrapper, buildExampleUrl("feed"));
    if (feedInput === undefined) {
      throw missingTestElement("feed input");
    }
    await feedInput.setValue(buildExampleUrl("updated"));

    await wrapper
      .findAll("button")
      .find((button) => button.text() === "测试插件")
      ?.trigger("click");
    await findFormByText(wrapper, "保存配置")?.trigger("submit");

    expect(uploadSpy).toHaveBeenCalledTimes(1);
    expect(installGitSpy).toHaveBeenCalledWith({
      repoUrl: buildFixtureRepoUrl(),
      repoRef: "main",
      repoSubdir: "plugins/hello-node",
    });
    expect(testSpy).toHaveBeenCalledWith(
      "plugin-installation-1",
      {
        feedUrl: buildExampleUrl("updated"),
      },
      {
        apiKey: "",
      },
      true,
    );
    expect(saveSpy).toHaveBeenCalledWith(
      "plugin-installation-1",
      {
        feedUrl: buildExampleUrl("updated"),
      },
      {
        apiKey: "",
      },
      true,
    );
  });

  it("shows plugin permission declarations in settings", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/settings", "admin");
    const plugin = createPermissionedPluginDetails();
    store.availablePlugins = [plugin];
    store.adminPlugins = [plugin];

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    expect(wrapper.text()).toContain("网络：api.example.com, *.example.org");
    expect(wrapper.text()).toContain("持久缓存");
    expect(wrapper.text()).toContain("安装脚本");
  });

  it("refreshes plugin drafts after plugin updates and clears secret inputs after save", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/settings", "admin");
    const plugin = createPluginDetails();
    const updatedPlugin = createUpdatedPluginDetails();
    store.availablePlugins = [plugin];
    store.adminPlugins = [plugin];

    vi.spyOn(store, "savePluginConfiguration").mockImplementation(async () => {
      store.availablePlugins = [updatedPlugin];
      store.adminPlugins = [updatedPlugin];
      return updatedPlugin;
    });

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    await openPluginConfigDialog(wrapper, "Demo Source");

    const secretInput = findInputByPlaceholder(wrapper, getSecretPlaceholder());
    if (secretInput === undefined) {
      throw missingTestElement("secret input");
    }
    await secretInput.setValue(buildToken("draft"));
    await findFormByText(wrapper, "保存配置")?.trigger("submit");
    await nextTick();

    expect(findInputByPlaceholder(wrapper, buildExampleUrl("feed"))).toBeUndefined();
    expect(findInputByPlaceholder(wrapper, buildExampleUrl("api"))).toBeDefined();
    const savedSecretInput = findInputByPlaceholder(wrapper, getSecretPlaceholder());
    if (savedSecretInput === undefined) {
      throw missingTestElement("saved secret input");
    }
    expect((savedSecretInput.element as HTMLInputElement).value).toBe("");
  });

  it("creates and deletes authenticated plugin schedules from the prints view", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/prints");
    const plugin = createPluginDetails();
    const expectedTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Shanghai";
    store.availablePlugins = [plugin];
    store.remoteSchedules = [createSchedule()];
    store.devices = [
      {
        id: "device-1",
        name: "Desk Printer",
        status: "connected",
        note: "Primary device",
      },
    ];
    store.defaultDeviceId = "device-1";

    const createSpy = vi.spyOn(store, "createSchedule").mockResolvedValue(createSchedule());
    const deleteSpy = vi.spyOn(store, "deleteSchedule").mockResolvedValue(true);

    const wrapper = mount(PrintsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    expect(wrapper.text()).toContain("Demo Source");

    await wrapper
      .findAll("button")
      .find((button) => button.text() === "新建定时任务")
      ?.trigger("click");

    await wrapper.find("input[placeholder='例如：晨间提醒']").setValue("Weekly Digest");
    const frequencySelect = wrapper
      .findAll("select")
      .find((select) => select.text().includes("每天") && select.text().includes("每周"));
    await frequencySelect?.setValue("weekly");
    await wrapper
      .findAll("button")
      .find((button) => button.text() === "周一")
      ?.trigger("click");
    await setNumberInputValue(wrapper, 2, 3);
    await wrapper.findAll("form").at(-1)?.trigger("submit");

    expect(createSpy).toHaveBeenCalledWith({
      title: "Weekly Digest",
      deviceId: "device-1",
      pluginInstallationId: "plugin-installation-1",
      frequencyType: "weekly",
      timezone: expectedTimezone,
      hour: 19,
      minute: 30,
      weekdays: [1],
      batchSize: 3,
    });

    await wrapper
      .findAll("button")
      .find((button) => button.text() === "删除")
      ?.trigger("click");

    expect(deleteSpy).toHaveBeenCalledWith("schedule-1");
  });
});
