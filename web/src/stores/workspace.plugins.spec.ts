import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { fetchAIConfigSummary, generateAIReply, saveAIConfig } from "@/services/ai";
import * as aiService from "@/services/ai";
import type {
  changePasswordWithApi,
  fetchCurrentUser,
  loginWithApi,
  logoutWithApi,
  refreshAuthSession,
} from "@/services/auth";
import * as authService from "@/services/auth";
import type {
  createPrintSchedule,
  deletePrintSchedule,
  disablePlugin,
  fetchAdminPlugins,
  fetchPlugin,
  fetchPlugins,
  fetchPrintSchedules,
  installPluginFromGit,
  savePluginBinding,
  testPluginBinding,
  togglePrintSchedule,
  updatePrintSchedule,
  uploadPluginZip,
} from "@/services/plugins";
import * as pluginService from "@/services/plugins";
import type {
  bindPrinter,
  cancelPrintJob,
  createPrintJob,
  deletePrinter,
  fetchPrintJobs,
  fetchPrinters,
  submitPrintJob,
  updatePrintJobDevice,
} from "@/services/printers";
import * as printerService from "@/services/printers";
import type {
  createUserWithApi,
  fetchWorkspaceStateWithApi,
  saveWorkspaceStateWithApi,
} from "@/services/workspace";
import * as workspaceService from "@/services/workspace";
import { useWorkspaceStore } from "@/stores/workspace";

vi.mock("@/services/auth", () => ({
  changePasswordWithApi: vi.fn<typeof changePasswordWithApi>(async () => undefined),
  fetchCurrentUser: vi.fn<typeof fetchCurrentUser>(),
  loginWithApi: vi.fn<typeof loginWithApi>(),
  logoutWithApi: vi.fn<typeof logoutWithApi>(async () => undefined),
  refreshAuthSession: vi.fn<typeof refreshAuthSession>(),
  AuthApiError: class AuthApiError extends Error {},
}));

vi.mock("@/services/workspace", () => ({
  createUserWithApi: vi.fn<typeof createUserWithApi>(),
  fetchWorkspaceStateWithApi: vi.fn<typeof fetchWorkspaceStateWithApi>(),
  saveWorkspaceStateWithApi: vi.fn<typeof saveWorkspaceStateWithApi>(
    async (_accessToken, state) => state,
  ),
}));

vi.mock("@/services/ai", () => ({
  fetchAIConfigSummary: vi.fn<typeof fetchAIConfigSummary>(),
  generateAIReply: vi.fn<typeof generateAIReply>(),
  saveAIConfig: vi.fn<typeof saveAIConfig>(),
}));

vi.mock("@/services/printers", () => ({
  bindPrinter: vi.fn<typeof bindPrinter>(),
  cancelPrintJob: vi.fn<typeof cancelPrintJob>(),
  createPrintJob: vi.fn<typeof createPrintJob>(),
  deletePrinter: vi.fn<typeof deletePrinter>(),
  fetchPrintJobs: vi.fn<typeof fetchPrintJobs>(),
  fetchPrinters: vi.fn<typeof fetchPrinters>(),
  submitPrintJob: vi.fn<typeof submitPrintJob>(),
  updatePrintJobDevice: vi.fn<typeof updatePrintJobDevice>(),
}));

vi.mock("@/services/plugins", () => ({
  createPrintSchedule: vi.fn<typeof createPrintSchedule>(),
  deletePrintSchedule: vi.fn<typeof deletePrintSchedule>(),
  disablePlugin: vi.fn<typeof disablePlugin>(),
  fetchAdminPlugins: vi.fn<typeof fetchAdminPlugins>(),
  fetchPlugin: vi.fn<typeof fetchPlugin>(),
  fetchPlugins: vi.fn<typeof fetchPlugins>(),
  fetchPrintSchedules: vi.fn<typeof fetchPrintSchedules>(),
  installPluginFromGit: vi.fn<typeof installPluginFromGit>(),
  savePluginBinding: vi.fn<typeof savePluginBinding>(),
  testPluginBinding: vi.fn<typeof testPluginBinding>(),
  togglePrintSchedule: vi.fn<typeof togglePrintSchedule>(),
  updatePrintSchedule: vi.fn<typeof updatePrintSchedule>(),
  uploadPluginZip: vi.fn<typeof uploadPluginZip>(),
}));

function createPluginDetails(overrides?: Record<string, unknown>) {
  return {
    installation: {
      id: "plugin-installation-1",
      pluginKey: "demo-source",
      sourceType: "git" as const,
      displayName: "Demo Source",
      version: "1.0.0",
      runtimeType: "node" as const,
      status: "ready" as const,
      repoUrl: "https://github.com/example/demo-source.git",
      repoRef: "main",
      repoCommitSha: "abc123",
      ...(overrides?.installation as Record<string, unknown> | undefined),
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
      fetchPolicy: {
        type: "fixed_interval" as const,
        minutes: 15,
      },
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
        },
      ],
      ...(overrides?.manifest as Record<string, unknown> | undefined),
    },
    binding: {
      id: "binding-1",
      enabled: true,
      status: "connected" as const,
      config: {
        feedUrl: "https://example.com/feed",
      },
      lastFetchAt: new Date("2026-04-10T00:20:00.000Z").toISOString(),
      nextFetchAt: new Date("2026-04-10T00:35:00.000Z").toISOString(),
      ...(overrides?.binding as Record<string, unknown> | undefined),
    },
  };
}

function createScheduleView(overrides?: Record<string, unknown>) {
  return {
    id: "schedule-1",
    title: "Morning Digest",
    pluginInstallationId: "plugin-installation-1",
    pluginBindingId: "binding-1",
    pluginDisplayName: "Demo Source",
    frequencyType: "daily" as const,
    timezone: "Asia/Shanghai",
    hour: 8,
    minute: 30,
    weekdays: [],
    printPolicy: {
      batchSize: 1,
    },
    deviceId: "device-1",
    enabled: true,
    nextRunAt: new Date("2026-04-11T00:30:00.000Z").toISOString(),
    lastRunAt: new Date("2026-04-10T00:30:00.000Z").toISOString(),
    lastError: "",
    timeLabel: "每天 08:30",
    sourceLabel: "Demo Source",
    ...overrides,
  };
}

function createWorkspaceState() {
  return {
    devices: [],
    conversations: [],
    activeConversationId: "",
    printJobs: [],
    schedules: [],
    sources: [],
    preferences: {
      loginProtectionEnabled: false,
      sendConfirmationEnabled: true,
      tutorialTabEnabled: true,
      theme: "light" as const,
      defaultDeviceId: "device-1",
      locale: "system" as const,
    },
    serviceBinding: {
      providerName: null,
      modelName: "Ink AI",
      bound: false,
    },
  };
}

function authenticateStore(role: "admin" | "member" = "member") {
  const store = useWorkspaceStore();
  store.authUser = {
    id: "user-1",
    email: role === "admin" ? "admin" : "member",
    name: role === "admin" ? "Administrator" : "Ink User",
    role,
  };
  store.authSession = {
    accessToken: "access-token",
    refreshToken: "refresh-token",
    accessTokenExpiresAt: new Date(Date.now() + 900_000).toISOString(),
  };
  return store;
}

describe("workspace store plugin flows", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    window.localStorage.clear();
    window.sessionStorage.clear();
    vi.clearAllMocks();
  });

  it("hydrates plugin and schedule state during authenticated bootstrap", async () => {
    const store = useWorkspaceStore();
    store.authSession = {
      accessToken: "access-token",
      refreshToken: "refresh-token",
      accessTokenExpiresAt: new Date(Date.now() + 900_000).toISOString(),
    };

    vi.mocked(authService.fetchCurrentUser).mockResolvedValueOnce({
      id: "user-1",
      email: "admin",
      name: "Administrator",
      role: "admin",
    });
    vi.mocked(workspaceService.fetchWorkspaceStateWithApi).mockResolvedValueOnce(
      createWorkspaceState(),
    );
    vi.mocked(aiService.fetchAIConfigSummary).mockResolvedValueOnce({
      bound: true,
      providerName: "Acme AI",
      providerType: "openai-compatible",
      baseUrl: "https://example.com/v1",
      model: "gpt-4.1-mini",
      keyConfigured: true,
    });
    vi.mocked(printerService.fetchPrinters).mockResolvedValueOnce({
      devices: [
        {
          id: "device-1",
          name: "Desk Printer",
          status: "connected",
          note: "Primary device",
        },
      ],
    });
    vi.mocked(printerService.fetchPrintJobs).mockResolvedValueOnce({
      printJobs: [],
    });
    vi.mocked(pluginService.fetchPlugins).mockResolvedValueOnce({
      plugins: [createPluginDetails()],
    });
    vi.mocked(pluginService.fetchPrintSchedules).mockResolvedValueOnce({
      schedules: [createScheduleView()],
    });
    vi.mocked(pluginService.fetchAdminPlugins).mockResolvedValueOnce({
      plugins: [createPluginDetails()],
    });

    await expect(store.initializeAuth()).resolves.toBe(true);

    expect(store.isAuthenticated).toBe(true);
    expect(store.isAdmin).toBe(true);
    expect(store.availablePlugins).toHaveLength(1);
    expect(store.adminPlugins).toHaveLength(1);
    expect(store.activeSources[0]).toMatchObject({
      name: "Demo Source",
      status: "connected",
    });
    expect(store.activeSchedules[0]).toMatchObject({
      title: "Morning Digest",
      pluginInstallationId: "plugin-installation-1",
      timeLabel: "每天 08:30",
    });
  });

  it("supports authenticated plugin upload, validation, save, and disable actions", async () => {
    const store = authenticateStore("admin");

    vi.mocked(pluginService.uploadPluginZip).mockResolvedValueOnce(
      createPluginDetails({
        installation: {
          version: "1.1.0",
        },
      }),
    );
    vi.mocked(pluginService.fetchAdminPlugins).mockResolvedValueOnce({
      plugins: [
        createPluginDetails({
          installation: {
            version: "1.1.0",
          },
        }),
      ],
    });

    const uploaded = await store.uploadPlugin(
      new File(["zip-content"], "demo-source.zip", { type: "application/zip" }),
    );

    expect(uploaded?.installation.version).toBe("1.1.0");
    expect(store.availablePlugins[0]?.installation.version).toBe("1.1.0");

    vi.mocked(pluginService.testPluginBinding).mockResolvedValueOnce({
      valid: false,
      errors: [
        {
          field: "feedUrl",
          message: "请输入 Feed URL。",
        },
      ],
    });

    await expect(
      store.testPluginConfiguration(
        "plugin-installation-1",
        {
          feedUrl: "",
        },
        {},
        true,
      ),
    ).resolves.toEqual({
      valid: false,
      errors: [
        {
          field: "feedUrl",
          message: "请输入 Feed URL。",
        },
      ],
    });
    expect(store.flashTone).toBe("error");

    vi.mocked(pluginService.savePluginBinding).mockResolvedValueOnce(
      createPluginDetails({
        binding: {
          id: "binding-1",
          enabled: true,
          status: "connected",
          config: {
            feedUrl: "https://example.com/updated",
          },
        },
      }),
    );

    await expect(
      store.savePluginConfiguration(
        "plugin-installation-1",
        {
          feedUrl: "https://example.com/updated",
        },
        {
          apiKey: "secret-key",
        },
        true,
      ),
    ).resolves.toMatchObject({
      binding: {
        config: {
          feedUrl: "https://example.com/updated",
        },
      },
    });

    vi.mocked(pluginService.disablePlugin).mockResolvedValueOnce(
      createPluginDetails({
        installation: {
          status: "disabled",
        },
        binding: {
          enabled: false,
          status: "disconnected",
          config: {
            feedUrl: "https://example.com/updated",
          },
        },
      }),
    );

    await expect(store.disablePluginInstallation("plugin-installation-1")).resolves.toMatchObject({
      installation: {
        status: "disabled",
      },
    });
    expect(store.availablePlugins[0]?.installation.status).toBe("disabled");
  });

  it("supports authenticated plugin installation from git", async () => {
    const store = authenticateStore("admin");

    vi.mocked(pluginService.installPluginFromGit).mockResolvedValueOnce(
      createPluginDetails({
        installation: {
          id: "plugin-installation-2",
          repoUrl: "https://github.com/example/ink-plugin.git",
          repoRef: "main",
          repoSubdir: "plugins/hello-node",
        },
      }),
    );
    vi.mocked(pluginService.fetchAdminPlugins).mockResolvedValueOnce({
      plugins: [
        createPluginDetails({
          installation: {
            id: "plugin-installation-2",
            repoUrl: "https://github.com/example/ink-plugin.git",
            repoRef: "main",
            repoSubdir: "plugins/hello-node",
          },
        }),
      ],
    });

    const installed = await store.installPluginRepository({
      repoUrl: "https://github.com/example/ink-plugin.git",
      repoRef: "main",
      repoSubdir: "plugins/hello-node",
    });

    expect(installed?.installation.repoSubdir).toBe("plugins/hello-node");
    expect(pluginService.installPluginFromGit).toHaveBeenCalledWith("access-token", {
      repoUrl: "https://github.com/example/ink-plugin.git",
      repoRef: "main",
      repoSubdir: "plugins/hello-node",
    });
    expect(store.availablePlugins[0]?.installation.id).toBe("plugin-installation-2");
    expect(store.pluginGitInstallLoading).toBe(false);
  });

  it("supports authenticated schedule creation, device updates, toggles, and deletion", async () => {
    const store = authenticateStore();
    store.devices = [
      {
        id: "device-1",
        name: "Desk Printer",
        status: "connected",
        note: "Primary device",
      },
      {
        id: "device-2",
        name: "Bedroom Printer",
        status: "connected",
        note: "Secondary device",
      },
    ];
    store.defaultDeviceId = "device-1";
    store.remoteSchedules = [createScheduleView()];

    vi.mocked(pluginService.createPrintSchedule).mockResolvedValueOnce(
      createScheduleView({
        id: "schedule-2",
        title: "Evening Digest",
        frequencyType: "weekly",
        weekdays: [1, 3, 5],
        timeLabel: "每周一、三、五 20:15",
        hour: 20,
        minute: 15,
      }),
    );

    const created = await store.createSchedule({
      title: "Evening Digest",
      pluginInstallationId: "plugin-installation-1",
      frequencyType: "weekly",
      timezone: "Asia/Shanghai",
      hour: 20,
      minute: 15,
      weekdays: [1, 3, 5],
      batchSize: 2,
      deviceId: "device-1",
    });

    expect(created?.id).toBe("schedule-2");
    expect(store.remoteSchedules.some((schedule) => schedule.id === "schedule-2")).toBe(true);

    vi.mocked(pluginService.updatePrintSchedule).mockResolvedValueOnce(
      createScheduleView({
        id: "schedule-2",
        title: "Evening Digest",
        deviceId: "device-2",
        frequencyType: "weekly",
        weekdays: [1, 3, 5],
        timeLabel: "每周一、三、五 20:15",
        hour: 20,
        minute: 15,
      }),
    );

    await store.updateScheduleDevice("schedule-2", "device-2");
    expect(store.remoteSchedules.find((schedule) => schedule.id === "schedule-2")?.deviceId).toBe(
      "device-2",
    );

    vi.mocked(pluginService.togglePrintSchedule).mockResolvedValueOnce(
      createScheduleView({
        id: "schedule-2",
        title: "Evening Digest",
        enabled: false,
        frequencyType: "weekly",
        weekdays: [1, 3, 5],
        timeLabel: "每周一、三、五 20:15",
        hour: 20,
        minute: 15,
      }),
    );

    await expect(store.toggleSchedule("schedule-2")).resolves.toBe(true);
    expect(store.remoteSchedules.find((schedule) => schedule.id === "schedule-2")?.enabled).toBe(
      false,
    );

    vi.mocked(pluginService.deletePrintSchedule).mockResolvedValueOnce(undefined);

    await expect(store.deleteSchedule("schedule-2")).resolves.toBe(true);
    expect(store.remoteSchedules.some((schedule) => schedule.id === "schedule-2")).toBe(false);
  });

  it("removes authenticated devices together with related jobs and schedules", async () => {
    const store = authenticateStore();
    store.devices = [
      {
        id: "device-1",
        name: "Desk Printer",
        status: "connected",
        note: "Primary device",
      },
      {
        id: "device-2",
        name: "Bedroom Printer",
        status: "connected",
        note: "Secondary device",
      },
    ];
    store.defaultDeviceId = "device-1";
    store.printJobs = [
      {
        id: "print-1",
        title: "Desk Job",
        source: "Manual",
        deviceId: "device-1",
        status: "pending",
        createdAt: new Date("2026-04-10T00:00:00.000Z").toISOString(),
        updatedAt: new Date("2026-04-10T00:00:00.000Z").toISOString(),
        content: "hello",
      },
      {
        id: "print-2",
        title: "Bedroom Job",
        source: "Manual",
        deviceId: "device-2",
        status: "pending",
        createdAt: new Date("2026-04-10T00:00:00.000Z").toISOString(),
        updatedAt: new Date("2026-04-10T00:00:00.000Z").toISOString(),
        content: "world",
      },
    ];
    store.remoteSchedules = [
      createScheduleView({
        id: "schedule-1",
        deviceId: "device-1",
      }),
      createScheduleView({
        id: "schedule-2",
        deviceId: "device-2",
      }),
    ];

    vi.mocked(printerService.deletePrinter).mockResolvedValueOnce(undefined);

    await expect(store.removeDevice("device-1")).resolves.toBe(true);

    expect(store.devices.map((device) => device.id)).toEqual(["device-2"]);
    expect(store.defaultDeviceId).toBe("device-2");
    expect(store.printJobs.map((job) => job.id)).toEqual(["print-2"]);
    expect(store.remoteSchedules.map((schedule) => schedule.id)).toEqual(["schedule-2"]);
  });
});
