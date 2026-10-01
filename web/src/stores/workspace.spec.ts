import { createPinia, disposePinia, getActivePinia, setActivePinia } from "pinia";
import { afterEach, vi } from "vitest";

import type { fetchAIConfigSummary, generateAIReply, saveAIConfig } from "@/services/ai";
import type {
  changePasswordWithApi,
  fetchCurrentUser,
  loginWithApi,
  logoutWithApi,
  refreshAuthSession,
} from "@/services/auth";
import type { submitFeedbackToAdmin } from "@/services/feedback";
import type {
  bindPrinter,
  cancelPrintJob,
  createPrintJob,
  deletePrinter,
  fetchPrinters,
  submitPrintJob,
  updatePrintJobDevice,
} from "@/services/printers";
import { fetchPrintJob, fetchPrintJobs, fetchPrintJobStatuses } from "@/services/printers";
import * as printerService from "@/services/printers";
import type { createUserWithApi, fetchWorkspaceStateWithApi } from "@/services/workspace";
import { saveWorkspaceStateWithApi } from "@/services/workspace";
import { useWorkspaceStore } from "@/stores/workspace";
import type { PrintJob, PrintJobCounts, PrintJobSummary } from "@/types/workspace";

vi.mock("@/services/auth", () => ({
  changePasswordWithApi: vi.fn<typeof changePasswordWithApi>(async () => undefined),
  fetchCurrentUser: vi.fn<typeof fetchCurrentUser>(),
  loginWithApi: vi.fn<typeof loginWithApi>(async ({ email }: { email: string }) => ({
    user: {
      id: "user-1",
      email,
      name: "Ink User",
      role: "member",
    },
    session: {
      accessToken: "access-token",
      refreshToken: "refresh-token",
      accessTokenExpiresAt: new Date(Date.now() + 900_000).toISOString(),
    },
  })),
  logoutWithApi: vi.fn<typeof logoutWithApi>(async () => undefined),
  refreshAuthSession: vi.fn<typeof refreshAuthSession>(),
  AuthApiError: class AuthApiError extends Error {},
}));

vi.mock("@/services/workspace", () => ({
  createUserWithApi: vi.fn<typeof createUserWithApi>(async () => ({
    id: "user-2",
    email: "new-user",
    name: "New User",
    role: "member",
  })),
  fetchWorkspaceStateWithApi: vi.fn<typeof fetchWorkspaceStateWithApi>(async () => ({
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
  })),
  saveWorkspaceStateWithApi: vi.fn<typeof saveWorkspaceStateWithApi>(
    async (_accessToken, state) => state,
  ),
}));

vi.mock("@/services/ai", () => ({
  fetchAIConfigSummary: vi.fn<typeof fetchAIConfigSummary>(async () => ({
    bound: false,
    providerName: "OpenAI Compatible",
    providerType: "openai-compatible",
    baseUrl: "",
    model: "gpt-4.1-mini",
    keyConfigured: false,
  })),
  generateAIReply: vi.fn<typeof generateAIReply>(async () => ({
    content: "这是来自真实 AI 服务的回复。",
    model: "gpt-4.1-mini",
    providerName: "OpenAI Compatible",
  })),
  saveAIConfig: vi.fn<typeof saveAIConfig>(async (_accessToken, payload) => ({
    bound: true,
    providerName: payload.providerName,
    providerType: payload.providerType,
    baseUrl: payload.baseUrl,
    model: payload.model,
    keyConfigured: true,
  })),
}));

vi.mock("@/services/plugins", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/services/plugins")>()),
  fetchPlugins: vi.fn<typeof import("@/services/plugins").fetchPlugins>(async () => ({
    plugins: [],
  })),
  fetchPrintSchedules: vi.fn<typeof import("@/services/plugins").fetchPrintSchedules>(async () => ({
    schedules: [],
  })),
  fetchAdminPlugins: vi.fn<typeof import("@/services/plugins").fetchAdminPlugins>(async () => ({
    plugins: [],
  })),
}));

vi.mock("@/services/feedback", () => ({
  submitFeedbackToAdmin: vi.fn<typeof submitFeedbackToAdmin>(async () => undefined),
}));

vi.mock("@/services/printers", () => ({
  bindPrinter: vi.fn<typeof bindPrinter>(async (_accessToken, payload) => ({
    id: "device-api-1",
    name: payload.name,
    status: "connected",
    note: payload.note,
  })),
  cancelPrintJob: vi.fn<typeof cancelPrintJob>(async (_accessToken, jobId) => ({
    id: jobId,
    title: "服务端任务",
    source: "手动打印",
    deviceId: "device-api-1",
    status: "cancelled",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    content: "内容",
  })),
  createPrintJob: vi.fn<typeof createPrintJob>(async (_accessToken, payload) => ({
    id: "print-api-1",
    title: payload.title,
    source: payload.source,
    deviceId: payload.printerBindingId,
    status: payload.submitImmediately ? "queued" : "pending",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    content: payload.content,
  })),
  deletePrinter: vi.fn<typeof deletePrinter>(async () => undefined),
  fetchPrintJobs: vi.fn<typeof fetchPrintJobs>(async () => ({
    printJobs: [],
    nextCursor: null,
  })),
  fetchPrintJob: vi.fn<typeof fetchPrintJob>(),
  fetchPrintJobStatuses: vi.fn<typeof fetchPrintJobStatuses>(async (_token, ids) => ({
    printJobs: ids.map((id) => ({
      id,
      status: "queued",
      updatedAt: new Date().toISOString(),
      deviceId: "device-api-1",
    })),
    counts: { pending: 0, queued: 0, completed: 0, failed: 0, cancelled: 0, todayCompleted: 0 },
    latestJobId: null,
  })),
  fetchPrinters: vi.fn<typeof fetchPrinters>(async () => ({
    devices: [],
  })),
  submitPrintJob: vi.fn<typeof submitPrintJob>(async (_accessToken, jobId) => ({
    id: jobId,
    title: "服务端任务",
    source: "手动打印",
    deviceId: "device-api-1",
    status: "queued",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    content: "内容",
  })),
  updatePrintJobDevice: vi.fn<typeof updatePrintJobDevice>(
    async (_accessToken, jobId, payload) => ({
      id: jobId,
      title: "服务端任务",
      source: "手动打印",
      deviceId: payload.printerBindingId,
      status: "pending",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      content: "内容",
    }),
  ),
}));

function authenticateStore(role: "admin" | "member" = "member") {
  const store = useWorkspaceStore();
  store.authUser = {
    id: "user-1",
    email: role === "admin" ? "admin" : "name@example.com",
    name: role === "admin" ? "Administrator" : "Ink User",
    role,
  };
  store.authSession = {
    accessToken: "access-token",
    refreshToken: "refresh-token",
    accessTokenExpiresAt: new Date(Date.now() + 60_000).toISOString(),
  };

  return store;
}

function printSummary(id: string, status: PrintJobSummary["status"] = "pending"): PrintJobSummary {
  return {
    id,
    title: id,
    source: "manual",
    deviceId: "device-api-1",
    status,
    createdAt: "2026-04-10T00:00:00.123400Z",
    updatedAt: "2026-04-10T00:00:00.123400Z",
  };
}

function printCounts(overrides: Partial<PrintJobCounts> = {}): PrintJobCounts {
  return {
    pending: 0,
    queued: 0,
    completed: 0,
    failed: 0,
    cancelled: 0,
    todayCompleted: 0,
    ...overrides,
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((fulfill) => {
    resolve = fulfill;
  });
  return { promise, resolve };
}

describe("workspace store", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    window.localStorage.clear();
    window.sessionStorage.clear();
    vi.clearAllMocks();
    vi.mocked(fetchPrintJobs).mockReset().mockResolvedValue({ printJobs: [], nextCursor: null });
    vi.mocked(fetchPrintJobStatuses)
      .mockReset()
      .mockImplementation(async (_token, ids) => ({
        printJobs: ids.map((id) => ({
          id,
          status: "queued",
          updatedAt: new Date().toISOString(),
          deviceId: "device-api-1",
        })),
        counts: { pending: 0, queued: 0, completed: 0, failed: 0, cancelled: 0, todayCompleted: 0 },
        latestJobId: null,
      }));
  });

  afterEach(() => {
    const pinia = getActivePinia();
    if (pinia) disposePinia(pinia);
    vi.restoreAllMocks();
  });

  it("exposes stable defaults and derived summaries", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 3, 7, 12, 0, 0));

    try {
      const store = useWorkspaceStore();

      expect(store.activeDeviceLabel).toBe("书桌咕咕机");
      expect(store.activeModelLabel).toBe("Ink AI");
      expect(store.todayPrintCount).toBe(2);
      expect(store.welcomeLabel).toBe("整理内容，准备打印");
      expect(store.isConfigured).toBe(true);
      expect(store.loginProtectionEnabled).toBe(false);
      expect(store.sendConfirmationEnabled).toBe(false);
      expect(store.tutorialTabEnabled).toBe(true);
      expect(store.pendingConfirmationCount).toBe(1);
      expect(store.enabledSchedulesCount).toBe(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it("updates configuration from settings actions instead of local hard-coded values", () => {
    const store = useWorkspaceStore();

    store.setDefaultDevice("device-bedroom");
    store.setTutorialTabEnabled(false);

    expect(store.activeDeviceLabel).toBe("卧室咕咕机");
    expect(store.isConfigured).toBe(true);
    expect(store.activeModelLabel).toBe("Ink AI");
    expect(store.welcomeLabel).toBe("整理内容，准备打印");
    expect(store.tutorialTabEnabled).toBe(false);
  });

  it("adds devices and removes the default device in anonymous mode", async () => {
    const store = useWorkspaceStore();
    const previousLength = store.devices.length;

    const newDevice = await store.addDevice({
      name: "客厅咕咕机",
      note: "窗边打印机",
      setAsDefault: true,
    });

    expect(store.devices).toHaveLength(previousLength + 1);
    expect(store.devices.at(-1)?.id).toBe(newDevice?.id);
    expect(store.defaultDeviceId).toBe(newDevice?.id);

    await expect(store.removeDevice(store.defaultDeviceId)).resolves.toBe(true);
    expect(store.devices.some((device) => device.id === newDevice?.id)).toBe(false);
  });

  it("removes non-default devices and reassigns related items to the default device", async () => {
    const store = useWorkspaceStore();

    await expect(store.removeDevice("device-bedroom")).resolves.toBe(true);
    expect(store.devices.some((device) => device.id === "device-bedroom")).toBe(false);
    expect(store.printJobs.some((job) => job.deviceId === "device-bedroom")).toBe(false);
    expect(store.schedules.some((schedule) => schedule.deviceId === "device-bedroom")).toBe(false);
    expect(store.printJobs.some((job) => job.deviceId === store.defaultDeviceId)).toBe(true);
    expect(store.schedules.some((schedule) => schedule.deviceId === store.defaultDeviceId)).toBe(
      true,
    );
  });

  it("allows removing all devices and clears the default device", async () => {
    const store = useWorkspaceStore();

    await expect(store.removeDevice("device-bedroom")).resolves.toBe(true);
    await expect(store.removeDevice("device-desk")).resolves.toBe(true);
    expect(store.devices).toHaveLength(0);
    expect(store.defaultDeviceId).toBe("");
    expect(
      store.printJobs.every(
        (job) => job.deviceId === "" || job.status === "cancelled" || job.status === "completed",
      ),
    ).toBe(true);
  });

  it("can generate a reply and create a queued print by default from the active conversation", async () => {
    const store = useWorkspaceStore();
    const previousCount = store.pendingPrintJobs.length;

    store.updateCurrentDraft("请帮我整理一句适合明早看的提醒");
    await store.sendCurrentDraft();
    store.toggleConversationMessageSelection(store.activeConversation!.messages.at(-1)!.id);
    const printJob = await store.createPrintFromSelectedMessages();

    expect(store.activeConversation?.messages.at(-1)?.role).toBe("assistant");
    expect(store.selectedConversationMessages.at(-1)?.text).toContain(
      "请帮我整理一句适合明早看的提醒",
    );
    expect(store.pendingPrintJobs).toHaveLength(previousCount + 1);
    expect(printJob?.status).toBe("queued");
    expect(printJob?.source).toBe("对话选中问答");
  });

  it("supports multi-select printing in conversation order", async () => {
    const store = useWorkspaceStore();
    const first = store.activeConversation!.messages[0];
    const second = store.activeConversation!.messages[1];

    store.toggleConversationMessageSelection(second.id);
    store.toggleConversationMessageSelection(first.id);

    const printJob = await store.createPrintFromSelectedMessages();

    expect(store.selectedConversationMessages.map((message) => message.id)).toEqual([
      first.id,
      second.id,
    ]);
    expect(printJob?.content).toContain(`我：${first.text}`);
    expect(printJob?.content).toContain(`Ink：${second.text}`);
  });

  it("cancels pending and queued print jobs without removing history", async () => {
    const store = useWorkspaceStore();
    const pendingJob = store.printJobs.find((job) => job.status === "pending");

    expect(pendingJob).toBeTruthy();
    await expect(store.cancelPrint(pendingJob!.id)).resolves.toBe(true);
    expect(store.printJobs.find((job) => job.id === pendingJob!.id)?.status).toBe("cancelled");

    store.updateCurrentDraft("请整理一条准备直接排队的纸条");
    await store.sendCurrentDraft();
    store.toggleConversationMessageSelection(store.activeConversation!.messages.at(-1)!.id);
    const queuedJob = await store.createPrintFromSelectedMessages();

    expect(queuedJob?.status).toBe("queued");
    await expect(store.cancelPrint(queuedJob!.id)).resolves.toBe(true);
    expect(store.printJobs.find((job) => job.id === queuedJob!.id)?.status).toBe("cancelled");
  });

  it("can delete the active conversation and keep the workspace usable", () => {
    const store = useWorkspaceStore();
    const currentId = store.activeConversationId;
    const previousLength = store.conversations.length;

    expect(store.deleteConversation(currentId)).toBe(true);
    expect(store.conversations).toHaveLength(previousLength - 1);
    expect(store.activeConversation).not.toBeNull();
    expect(store.activeConversationId).not.toBe(currentId);
  });

  it("supports print queue updates and source state cycling", async () => {
    const store = useWorkspaceStore();
    const pendingJob = store.pendingPrintJobs.find((job) => job.status === "pending");
    const schedule = store.schedules[0];
    const source = store.sources[0];

    expect(pendingJob).toBeTruthy();

    await store.confirmPrint(pendingJob!.id);
    store.updateScheduleDevice(schedule.id, "device-bedroom");
    store.toggleSourceConnection(source.id);
    store.setTheme("dark");
    store.setLoginProtection(false);
    await store.logout();

    expect(store.printJobs.find((job) => job.id === pendingJob!.id)?.status).toBe("queued");
    expect(store.schedules.find((item) => item.id === schedule.id)?.deviceId).toBe(
      "device-bedroom",
    );
    expect(store.sources.find((item) => item.id === source.id)?.status).toBe("disconnected");
    expect(store.selectedTheme).toBe("dark");
    expect(store.loginProtectionEnabled).toBe(false);
    expect(store.isAuthenticated).toBe(false);
  });

  it("maps the legacy soft theme to light when hydrating persisted state", () => {
    window.localStorage.setItem(
      "ink.workspace.v1",
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
          theme: "soft",
          defaultDeviceId: "",
          locale: "system",
        },
        serviceBinding: {
          providerName: null,
          modelName: "Ink AI",
          bound: false,
        },
      }),
    );

    const store = useWorkspaceStore();

    expect(store.selectedTheme).toBe("light");
  });

  it("clears the local auth state after a successful password change", async () => {
    const store = authenticateStore();

    await expect(store.changePassword("demo-password", "next-password")).resolves.toBe(true);
    expect(store.isAuthenticated).toBe(false);
    expect(store.flashMessage).toBe("密码已更新，请重新登录。");
  });

  it("loads remote workspace data after login", async () => {
    const store = useWorkspaceStore();

    await expect(store.login("admin", "demo-password")).resolves.toBe(true);

    expect(store.isAuthenticated).toBe(true);
    expect(store.devices).toEqual([]);
    expect(store.conversations).toHaveLength(1);
    expect(store.activeConversation?.title).toBe("新对话");
    expect(store.activeConversation?.messages).toEqual([]);
    expect(store.aiConfigSummary.bound).toBe(false);
    expect(store.sendConfirmationEnabled).toBe(false);
    expect(store.postLoginTutorialOpen).toBe(true);
  });

  it("persists changes made while a remote workspace save is in flight", async () => {
    vi.useFakeTimers();
    let finishFirstSave!: () => void;
    vi.mocked(saveWorkspaceStateWithApi)
      .mockImplementationOnce(
        async (_accessToken, state) =>
          new Promise((resolve) => {
            finishFirstSave = () => resolve(state);
          }),
      )
      .mockImplementation(async (_accessToken, state) => state);

    try {
      const store = useWorkspaceStore();
      await store.login("name@example.com", "demo-password");
      vi.mocked(saveWorkspaceStateWithApi).mockClear();

      store.setTheme("dark");
      await vi.advanceTimersByTimeAsync(750);
      expect(saveWorkspaceStateWithApi).toHaveBeenCalledTimes(1);

      store.setTheme("light");
      await vi.advanceTimersByTimeAsync(750);
      finishFirstSave();
      await vi.waitFor(() => {
        expect(saveWorkspaceStateWithApi).toHaveBeenCalledTimes(2);
      });

      expect(vi.mocked(saveWorkspaceStateWithApi).mock.calls[1]?.[1].preferences.theme).toBe(
        "light",
      );
    } finally {
      vi.useRealTimers();
    }
  });

  it("can send the first message after loading an empty remote workspace", async () => {
    const store = useWorkspaceStore();

    await expect(store.login("admin", "demo-password")).resolves.toBe(true);
    store.updateCurrentDraft("帮我整理一句登录后的第一条消息");

    await expect(store.sendCurrentDraft()).resolves.toBe(true);

    expect(store.activeConversation?.messages[0]?.role).toBe("user");
    expect(store.activeConversation?.messages[0]?.text).toBe("帮我整理一句登录后的第一条消息");
    expect(store.activeConversation?.messages.at(-1)?.role).toBe("assistant");
  });

  it("uses the real AI reply endpoint when the user is authenticated", async () => {
    const store = authenticateStore();

    store.updateCurrentDraft("帮我整理一句真实回复");
    await expect(store.sendCurrentDraft()).resolves.toBe(true);

    expect(store.activeConversation?.messages.at(-1)?.text).toBe("这是来自真实 AI 服务的回复。");
  });

  it("allows admins to save the server-side AI provider config", async () => {
    const store = authenticateStore("admin");

    await expect(
      store.saveAIServiceConfig({
        providerName: "Acme AI",
        providerType: "openai-compatible",
        baseUrl: "https://example.com/v1",
        model: "gpt-4.1-mini",
        apiKey: "secret-key",
      }),
    ).resolves.toBe(true);

    expect(store.aiConfigSummary.bound).toBe(true);
    expect(store.aiConfigSummary.providerName).toBe("Acme AI");
    expect(store.aiConfigSummary.keyConfigured).toBe(true);
  });

  it("binds devices and creates print jobs through the authenticated printer API", async () => {
    const store = authenticateStore();

    const device = await store.addDevice({
      name: "我的咕咕机",
      note: "书房",
      deviceId: "m1-123456",
      setAsDefault: true,
    });
    const job = await store.createManualPrint({
      title: "真实打印",
      content: "这是一条真实打印任务。",
    });

    expect(device?.status).toBe("connected");
    expect(store.defaultDeviceId).toBe(device?.id);
    expect(job?.deviceId).toBe(device?.id);
    expect(store.remotePrintJobs.find((item) => item.id === job!.id)?.status).toBe("queued");
  });

  it("refreshes queued authenticated print jobs until they complete", async () => {
    vi.useFakeTimers();

    try {
      const store = useWorkspaceStore();
      store.remotePrintJobs = store.remotePrintJobs.filter((job) => job.status !== "queued");
      authenticateStore();

      const job = await store.createManualPrint({
        title: "真实打印",
        content: "这是一条真实打印任务。",
      });

      vi.mocked(fetchPrintJobStatuses).mockResolvedValueOnce({
        printJobs: store.remotePrintJobs.map((item) =>
          item.id === job!.id
            ? {
                ...item,
                status: "completed",
                updatedAt: new Date(Date.now() + 1000).toISOString(),
              }
            : item,
        ),
        counts: { pending: 0, queued: 0, completed: 1, failed: 0, cancelled: 0, todayCompleted: 1 },
        latestJobId: job!.id,
      });

      expect(store.remotePrintJobs.find((item) => item.id === job!.id)?.status).toBe("queued");

      await vi.advanceTimersByTimeAsync(1500);

      expect(store.remotePrintJobs.find((item) => item.id === job!.id)?.status).toBe("completed");
    } finally {
      vi.useRealTimers();
    }
  });

  it("allows admins to create member accounts", async () => {
    const store = authenticateStore("admin");

    await expect(store.createAccount("new-user", "New User", "demo-password")).resolves.toBe(true);
    expect(store.flashMessage).toBe("新账号已创建。");
  });

  it("submits feedback to the administrator printer for authenticated users", async () => {
    const store = authenticateStore();

    await expect(store.submitFeedback("希望补一个反馈按钮")).resolves.toBe(true);

    expect(store.feedbackError).toBe("");
    expect(store.flashMessage).toBe("反馈已发送，作者会直接收到纸条。");
  });

  it("requires login before submitting feedback", async () => {
    const store = useWorkspaceStore();

    await expect(store.submitFeedback("hello")).resolves.toBe(false);

    expect(store.feedbackError).toBe("请先登录后再反馈。");
  });

  it("keeps auth tokens out of the workspace snapshot and persists them when login protection is off", async () => {
    authenticateStore();

    await Promise.resolve();

    expect(window.localStorage.getItem("ink.auth.session.v1")).toContain("refresh-token");
    expect(window.sessionStorage.getItem("ink.auth.session.v1")).toBeNull();
    expect(window.localStorage.getItem("ink.workspace.v1") ?? "").not.toContain("refresh-token");
    expect(window.localStorage.getItem("ink.workspace.v1") ?? "").not.toContain("access-token");
  });

  it("stores auth tokens only for the current tab when login protection is on", async () => {
    const store = authenticateStore();

    store.setLoginProtection(true);
    await Promise.resolve();

    expect(window.sessionStorage.getItem("ink.auth.session.v1")).toContain("refresh-token");
    expect(window.localStorage.getItem("ink.auth.session.v1")).toBeNull();
    expect(window.localStorage.getItem("ink.workspace.v1") ?? "").not.toContain("refresh-token");
    expect(window.localStorage.getItem("ink.workspace.v1") ?? "").not.toContain("access-token");
  });
  it("keeps global counts accurate while paging summaries and loads bodies only on demand", async () => {
    vi.useFakeTimers();
    try {
      const first = printSummary("pending-1");
      vi.mocked(fetchPrintJobs)
        .mockResolvedValueOnce({ printJobs: [first], nextCursor: "active-first" })
        .mockResolvedValueOnce({
          printJobs: [printSummary("history-1", "completed")],
          nextCursor: "history-first",
        });
      vi.mocked(fetchPrintJobStatuses).mockResolvedValueOnce({
        printJobs: [],
        counts: printCounts({ pending: 120, queued: 30, completed: 300, todayCompleted: 9 }),
        latestJobId: first.id,
      });
      const store = useWorkspaceStore();
      await store.login("name@example.com", "secret");
      expect(fetchPrintJobs).toHaveBeenCalledWith("access-token", { status: "active" });
      expect(fetchPrintJobs).toHaveBeenCalledWith("access-token", { status: "history" });
      expect(store.pendingConfirmationCount).toBe(120);
      expect(store.todayPrintCount).toBe(9);
      expect(store.printJobs).toEqual([]);

      const page = deferred<Awaited<ReturnType<typeof fetchPrintJobs>>>();
      vi.mocked(fetchPrintJobs).mockReturnValueOnce(page.promise);
      const loads = [store.loadMorePrintJobs("active"), store.loadMorePrintJobs("active")];
      await Promise.resolve();
      expect(fetchPrintJobs).toHaveBeenCalledTimes(3);
      page.resolve({ printJobs: [first, printSummary("pending-2")], nextCursor: null });
      await Promise.all(loads);
      expect(store.pendingPrintJobs.map((job) => job.id)).toEqual(["pending-1", "pending-2"]);
      expect(store.activePrintJobsCursor).toBeNull();
      expect(store.pendingConfirmationCount).toBe(120);

      const detail = deferred<PrintJob>();
      vi.mocked(fetchPrintJob).mockReturnValueOnce(detail.promise);
      const details = [store.loadPrintJobDetail(first.id), store.loadPrintJobDetail(first.id)];
      expect(fetchPrintJob).toHaveBeenCalledTimes(1);
      detail.resolve({ ...first, content: "the full printable body" });
      expect((await Promise.all(details))[0]?.content).toBe("the full printable body");
      expect(store.remotePrintJobs[0]).not.toHaveProperty("content");
      expect(saveWorkspaceStateWithApi).not.toHaveBeenCalled();
      store.setTheme("dark");
      await vi.advanceTimersByTimeAsync(750);
      expect(vi.mocked(saveWorkspaceStateWithApi).mock.calls.at(-1)?.[1].printJobs).toEqual([]);
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("uses only statuses on steady polls and discovers scheduled jobs after the queue becomes empty", async () => {
    vi.useFakeTimers();
    try {
      const counts = printCounts({ completed: 1, todayCompleted: 1 });
      vi.mocked(fetchPrintJobs)
        .mockResolvedValueOnce({ printJobs: [], nextCursor: null })
        .mockResolvedValueOnce({
          printJobs: [printSummary("history-old", "completed")],
          nextCursor: "old-history",
        });
      vi.mocked(fetchPrintJobStatuses)
        .mockResolvedValueOnce({ printJobs: [], counts, latestJobId: "history-old" })
        .mockResolvedValueOnce({ printJobs: [], counts, latestJobId: "history-old" })
        .mockResolvedValueOnce({
          printJobs: [],
          counts: printCounts({ queued: 1, completed: 1, todayCompleted: 1 }),
          latestJobId: "scheduled-new",
        });
      const store = useWorkspaceStore();
      await store.login("name@example.com", "secret");
      await vi.advanceTimersByTimeAsync(1500);
      expect(fetchPrintJobs).toHaveBeenCalledTimes(2);
      expect(fetchPrintJobStatuses).toHaveBeenCalledTimes(2);
      vi.mocked(fetchPrintJobs)
        .mockResolvedValueOnce({
          printJobs: [printSummary("scheduled-new", "queued")],
          nextCursor: "new-active",
        })
        .mockResolvedValueOnce({
          printJobs: [printSummary("history-new", "completed")],
          nextCursor: "restarted-history",
        });
      await vi.advanceTimersByTimeAsync(30000);
      expect(store.pendingPrintJobs.map((job) => job.id)).toEqual(["scheduled-new"]);
      expect(store.printHistoryJobs.map((job) => job.id)).toContain("history-old");
      expect(store.historyPrintJobsCursor).toBe("restarted-history");
      expect(saveWorkspaceStateWithApi).not.toHaveBeenCalled();
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("preserves newer mutations when status and page requests finish late within the same millisecond", async () => {
    vi.useFakeTimers();
    try {
      const first = printSummary("pending-1");
      vi.mocked(fetchPrintJobs)
        .mockResolvedValueOnce({ printJobs: [first], nextCursor: "more-active" })
        .mockResolvedValueOnce({ printJobs: [], nextCursor: null });
      vi.mocked(fetchPrintJobStatuses).mockResolvedValueOnce({
        printJobs: [],
        counts: printCounts({ pending: 1 }),
        latestJobId: first.id,
      });
      const store = useWorkspaceStore();
      await store.login("name@example.com", "secret");
      const page = deferred<Awaited<ReturnType<typeof fetchPrintJobs>>>();
      const statuses = deferred<Awaited<ReturnType<typeof fetchPrintJobStatuses>>>();
      vi.mocked(fetchPrintJobs).mockReturnValueOnce(page.promise);
      vi.mocked(fetchPrintJobStatuses).mockReturnValueOnce(statuses.promise);
      const loading = store.loadMorePrintJobs("active");
      await vi.advanceTimersByTimeAsync(1500);
      vi.mocked(printerService.cancelPrintJob).mockResolvedValueOnce({
        ...first,
        status: "cancelled",
        updatedAt: "2026-04-10T00:00:00.123456Z",
        content: "body",
      });
      await expect(store.cancelPrint(first.id)).resolves.toBe(true);
      page.resolve({
        printJobs: [{ ...first, updatedAt: "2026-04-10T00:00:00.123455Z" }],
        nextCursor: null,
      });
      statuses.resolve({
        printJobs: [{ ...first, updatedAt: "2026-04-10T00:00:00.123455Z" }],
        counts: printCounts({ pending: 1 }),
        latestJobId: first.id,
      });
      await loading;
      await Promise.resolve();
      expect(store.remotePrintJobs[0]?.status).toBe("cancelled");
      expect(store.remotePrintJobs[0]?.updatedAt).toBe("2026-04-10T00:00:00.123456Z");
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("discards old-account pages, details and status responses after switching accounts", async () => {
    vi.useFakeTimers();
    try {
      const first = printSummary("account-a-job");
      vi.mocked(fetchPrintJobs)
        .mockResolvedValueOnce({ printJobs: [first], nextCursor: "a-cursor" })
        .mockResolvedValueOnce({ printJobs: [], nextCursor: null });
      vi.mocked(fetchPrintJobStatuses).mockResolvedValueOnce({
        printJobs: [],
        counts: printCounts({ pending: 1 }),
        latestJobId: first.id,
      });
      const store = useWorkspaceStore();
      await store.login("a@example.com", "secret");
      const page = deferred<Awaited<ReturnType<typeof fetchPrintJobs>>>();
      const detail = deferred<PrintJob>();
      const statuses = deferred<Awaited<ReturnType<typeof fetchPrintJobStatuses>>>();
      vi.mocked(fetchPrintJobs).mockReturnValueOnce(page.promise);
      vi.mocked(fetchPrintJob).mockReturnValueOnce(detail.promise);
      vi.mocked(fetchPrintJobStatuses).mockReturnValueOnce(statuses.promise);
      const loading = store.loadMorePrintJobs("active");
      const preview = store.loadPrintJobDetail(first.id);
      await vi.advanceTimersByTimeAsync(1500);
      store.authUser = { ...store.authUser!, id: "user-b" };
      store.authSession = { ...store.authSession!, accessToken: "b-token" };
      store.remotePrintJobs = [printSummary("account-b-job")];
      store.activePrintJobsCursor = "b-cursor";
      page.resolve({ printJobs: [first], nextCursor: "a-next" });
      detail.resolve({ ...first, content: "a private body" });
      statuses.resolve({
        printJobs: [first],
        counts: printCounts({ pending: 9 }),
        latestJobId: first.id,
      });
      await loading;
      await expect(preview).resolves.toBeNull();
      await Promise.resolve();
      expect(store.remotePrintJobs.map((job) => job.id)).toEqual(["account-b-job"]);
      expect(store.activePrintJobsCursor).toBe("b-cursor");
      expect(store.remotePrintCounts).toBeNull();
      expect(store.printerSyncError).toBe("");
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("bounds status batches and pauses them while the page is hidden", async () => {
    vi.useFakeTimers();
    const visibility = vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
    try {
      const store = useWorkspaceStore();
      await store.login("name@example.com", "secret");
      store.remotePrintJobs = Array.from({ length: 205 }, (_, index) =>
        printSummary(`job-${index}`, "queued"),
      );
      vi.mocked(fetchPrintJobStatuses).mockClear();
      await vi.advanceTimersByTimeAsync(1500);
      expect(vi.mocked(fetchPrintJobStatuses).mock.calls.map((call) => call[1].length)).toEqual([
        100, 100, 5,
      ]);
      visibility.mockReturnValue("hidden");
      document.dispatchEvent(new Event("visibilitychange"));
      await vi.advanceTimersByTimeAsync(120000);
      expect(fetchPrintJobStatuses).toHaveBeenCalledTimes(3);
      visibility.mockReturnValue("visible");
      document.dispatchEvent(new Event("visibilitychange"));
      await vi.advanceTimersByTimeAsync(1500);
      expect(fetchPrintJobStatuses).toHaveBeenCalledTimes(6);
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
      visibility.mockRestore();
    }
  });
});
