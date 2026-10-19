import { createPinia, setActivePinia } from "pinia";
import { afterEach, vi } from "vitest";
import { nextTick } from "vue";

import { translate } from "@/i18n";
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
import type { submitFeedbackToAdmin } from "@/services/feedback";
import { AuthApiError, request } from "@/services/http";
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
import type { createUserWithApi } from "@/services/workspace";
import { fetchWorkspaceStateWithApi, saveWorkspaceStateWithApi } from "@/services/workspace";
import { useWorkspaceStore } from "@/stores/workspace";
import type {
  PersistedWorkspaceState,
  PrintJob,
  PrintJobCounts,
  PrintJobSummary,
} from "@/types/workspace";

vi.mock("@/services/auth", async (importOriginal) => ({
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
  AuthApiError: (await importOriginal<typeof import("@/services/auth")>()).AuthApiError,
}));

vi.mock("@/services/workspace", () => ({
  createUserWithApi: vi.fn<typeof createUserWithApi>(async () => ({
    id: "user-2",
    email: "new-user",
    name: "New User",
    role: "member",
  })),
  fetchWorkspaceStateWithApi: vi.fn<typeof fetchWorkspaceStateWithApi>(async () => ({
    revision: 1,
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

const backupValues = () =>
  Array.from({ length: localStorage.length }, (_, i) => localStorage.key(i))
    .filter((key) => key?.startsWith("ink.workspace.draft.v1.user-1."))
    .map((key) => localStorage.getItem(key!));
function blockedStorage(): never {
  throw new DOMException("blocked", "SecurityError");
}

function accountBackup(userId: string) {
  const prefix = `ink.workspace.draft.v1.${encodeURIComponent(userId)}.`;
  const key = Array.from({ length: localStorage.length }, (_, index) =>
    localStorage.key(index),
  ).find((candidate) => candidate?.startsWith(prefix));
  return key ? localStorage.getItem(key) : null;
}

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
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((fulfill, fail) => {
    resolve = fulfill;
    reject = fail;
  });
  return { promise, resolve, reject };
}

function trackWorkspaceTimers() {
  const scheduled = vi.spyOn(window, "setTimeout");
  const cleared = vi.spyOn(window, "clearTimeout");
  const applicationTimers = (start = 0) =>
    scheduled.mock.calls
      .slice(start)
      .map(([, delay], index) => ({
        delay,
        handle: scheduled.mock.results[start + index]?.value,
      }))
      // jsdom queues zero-delay storage events when local/session storage changes.
      // Those belong to the browser environment, rather than the store's timers.
      .filter(({ delay }) => typeof delay === "number" && delay > 0);

  return { scheduled, cleared, applicationTimers };
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
    vi.restoreAllMocks();
  });

  it("waits for the workspace PUT before announcing a saved draft", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    const pending = deferred<Awaited<ReturnType<typeof saveWorkspaceStateWithApi>>>();
    vi.mocked(saveWorkspaceStateWithApi).mockReturnValueOnce(pending.promise);
    store.updateCurrentDraft("save acknowledgement draft");
    const saving = store.saveCurrentDraft();
    expect(store.flashMessage).not.toBe(translate("store.flash.draftSaved"));
    const snapshot = vi.mocked(saveWorkspaceStateWithApi).mock.calls.at(-1)![1];
    pending.resolve({ ...snapshot, revision: 2 });
    await expect(saving).resolves.toBe(true);
    expect(store.flashMessage).toBe(translate("store.flash.draftSaved"));
    expect(accountBackup("user-1")).toBeNull();
    store.$dispose();
  });

  it("reports failure and keeps an account-only draft through safe logout", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    store.updateCurrentDraft("private unsynced draft");
    vi.mocked(saveWorkspaceStateWithApi).mockRejectedValue(new Error("Synthetic outage"));
    await expect(store.saveCurrentDraft()).resolves.toBe(false);
    expect(store.flashTone).toBe("error");
    await store.logout();
    expect(store.isAuthenticated).toBe(false);
    expect(store.conversations.some((c) => c.draft === "private unsynced draft")).toBe(false);
    const backup = accountBackup("user-1")!;
    expect(backup).toContain("private unsynced draft");
    expect(backup).not.toContain("access-token");
    expect(backup).not.toContain("refresh-token");
    expect(localStorage.getItem("ink.workspace.v1")).not.toContain("private unsynced draft");
    vi.mocked(saveWorkspaceStateWithApi).mockImplementation(async (_token, state) => ({
      ...state,
      revision: 2,
    }));
    await store.login("a@example.invalid", "synthetic-password");
    expect(store.recoveredWorkspaceDraft?.conversations[0]?.draft).toBe("private unsynced draft");
    expect(store.activeConversation?.draft).not.toBe("private unsynced draft");
    store.updateCurrentDraft("new edit does not discard recovery");
    expect(store.activeConversation?.draft).not.toBe("new edit does not discard recovery");
    await expect(store.saveCurrentDraft()).resolves.toBe(false);
    expect(store.recoveredWorkspaceDraft?.conversations[0]?.draft).toBe("private unsynced draft");
    expect(store.restoreRecoveredWorkspaceDraft()).toBe(true);
    expect(store.activeConversation?.draft).toBe("private unsynced draft");
    await store.saveCurrentDraft();
    expect(accountBackup("user-1")).toBeNull();
    store.$dispose();
  });

  it("keeps failed-save recovery private from a different account", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    store.updateCurrentDraft("A private backup");
    vi.mocked(saveWorkspaceStateWithApi).mockRejectedValueOnce(new Error("Synthetic outage"));
    await store.logout();
    vi.mocked(authService.loginWithApi).mockResolvedValueOnce({
      user: { id: "user-b", email: "b@example.invalid", name: "B", role: "member" },
      session: {
        accessToken: "b-token",
        refreshToken: "b-refresh",
        accessTokenExpiresAt: new Date(Date.now() + 900000).toISOString(),
      },
    });
    await store.login("b@example.invalid", "synthetic-password");
    expect(store.recoveredWorkspaceDraft).toBeNull();
    expect(store.conversations.some((c) => c.draft === "A private backup")).toBe(false);
    expect(store.restoreRecoveredWorkspaceDraft()).toBe(false);
    store.$dispose();
  });

  it("preserves unsynced edits when refresh invalidates the session", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    store.updateCurrentDraft("expired-session draft");
    store.authSession = { ...store.authSession!, accessTokenExpiresAt: new Date(0).toISOString() };
    vi.mocked(authService.refreshAuthSession).mockRejectedValueOnce(
      new AuthApiError(401, "unauthorized", "expired"),
    );
    await expect(store.refreshSessionIfNeeded()).resolves.toBe(false);
    expect(store.isAuthenticated).toBe(false);
    expect(accountBackup("user-1")).toContain("expired-session draft");
    expect(store.conversations.some((c) => c.draft === "expired-session draft")).toBe(false);
    store.$dispose();
  });

  it("backs up a failed-save draft before password change invalidates the account", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    store.updateCurrentDraft("password-change draft");
    vi.mocked(saveWorkspaceStateWithApi).mockRejectedValueOnce(new Error("Synthetic outage"));
    await expect(store.changePassword("synthetic-old", "synthetic-new")).resolves.toBe(true);
    expect(store.isAuthenticated).toBe(false);
    expect(accountBackup("user-1")).toContain("password-change draft");
    store.$dispose();
  });

  it("retains the previous reply on failure and replaces it only on regeneration success", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    store.updateCurrentDraft("question");
    await store.sendCurrentDraft();
    const original = store.activeConversation!.messages.at(-1)!;
    vi.mocked(aiService.generateAIReply).mockRejectedValueOnce(
      new Error("Synthetic provider failure"),
    );
    await expect(store.regenerateLatestReply()).resolves.toBe(false);
    expect(store.activeConversation?.messages.at(-1)).toEqual(original);
    vi.mocked(aiService.generateAIReply).mockResolvedValueOnce({
      content: "replacement",
      model: "synthetic",
      providerName: "synthetic",
    });
    await expect(store.regenerateLatestReply()).resolves.toBe(true);
    expect(store.activeConversation?.messages).toHaveLength(2);
    expect(store.activeConversation?.messages.at(-1)?.text).toBe("replacement");
    store.$dispose();
  });

  it("ignores an old AI response without clearing a new account generation", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    const old = deferred<Awaited<ReturnType<typeof aiService.generateAIReply>>>();
    vi.mocked(aiService.generateAIReply).mockReturnValueOnce(old.promise);
    store.updateCurrentDraft("old question");
    const oldGeneration = store.sendCurrentDraft();
    await store.logout();
    await store.login("a@example.invalid", "synthetic-password");
    const next = deferred<Awaited<ReturnType<typeof aiService.generateAIReply>>>();
    vi.mocked(aiService.generateAIReply).mockReturnValueOnce(next.promise);
    store.updateCurrentDraft("new question");
    const newGeneration = store.sendCurrentDraft();
    old.resolve({ content: "old private reply", model: "m", providerName: "p" });
    await expect(oldGeneration).resolves.toBe(false);
    expect(store.isGenerating).toBe(true);
    expect(store.activeConversation?.messages.some((m) => m.text === "old private reply")).toBe(
      false,
    );
    next.resolve({ content: "new reply", model: "m", providerName: "p" });
    await expect(newGeneration).resolves.toBe(true);
    store.$dispose();
  });

  it("round-trips legacy sources and schedules without exposing their operations", async () => {
    const state = await fetchWorkspaceStateWithApi("synthetic");
    const schedule = {
      id: "old-schedule",
      title: "Archived",
      source: "Old source",
      timeLabel: "old",
      deviceId: "removed",
      enabled: true,
    };
    const source = {
      id: "old-source",
      name: "Archived",
      type: "legacy",
      note: "keep",
      status: "connected" as const,
    };
    vi.mocked(fetchWorkspaceStateWithApi).mockResolvedValueOnce({
      ...state,
      schedules: [schedule],
      sources: [source],
    });
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    store.updateCurrentDraft("current 1.0 edit");
    await store.saveCurrentDraft();
    const saved = vi.mocked(saveWorkspaceStateWithApi).mock.calls.at(-1)![1];
    expect(saved.schedules).toEqual([schedule]);
    expect(saved.sources).toEqual([source]);
    expect(store).not.toHaveProperty("createSchedule");
    expect(store).not.toHaveProperty("savePluginConfiguration");
    store.$dispose();
  });

  it("discards an old AI configuration response and preserves the new account loading state", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    const old = deferred<Awaited<ReturnType<typeof aiService.saveAIConfig>>>();
    vi.mocked(aiService.saveAIConfig).mockReturnValueOnce(old.promise);
    const payload = {
      providerName: "old",
      providerType: "openai-compatible",
      baseUrl: "https://old.example.invalid",
      model: "old",
      apiKey: "synthetic",
    };
    const oldSaving = store.saveAIServiceConfig(payload);
    await store.logout();
    await store.login("a@example.invalid", "synthetic-password");
    const next = deferred<Awaited<ReturnType<typeof aiService.saveAIConfig>>>();
    vi.mocked(aiService.saveAIConfig).mockReturnValueOnce(next.promise);
    const nextSaving = store.saveAIServiceConfig({ ...payload, model: "new" });
    old.resolve({
      bound: true,
      providerName: "old",
      providerType: "openai-compatible",
      baseUrl: payload.baseUrl,
      model: "old",
      keyConfigured: true,
    });
    await expect(oldSaving).resolves.toBe(false);
    expect(store.aiConfigSaving).toBe(true);
    expect(store.aiConfigSummary.model).not.toBe("old");
    next.resolve({
      bound: true,
      providerName: "new",
      providerType: "openai-compatible",
      baseUrl: payload.baseUrl,
      model: "new",
      keyConfigured: true,
    });
    await expect(nextSaving).resolves.toBe(true);
    expect(store.aiConfigSummary.model).toBe("new");
    store.$dispose();
  });

  it("offers an export when account backup storage fails and still signs out safely", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    const setItem = Storage.prototype.setItem;
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(function (this: Storage, key, value) {
      if (key.startsWith("ink.workspace.draft.v1."))
        throw new DOMException("full", "QuotaExceededError");
      return setItem.call(this, key, value);
    });
    const createURL = vi.fn<() => string>(() => "blob:backup");
    vi.stubGlobal("URL", {
      createObjectURL: createURL,
      revokeObjectURL: vi.fn<(url: string) => void>(),
    });
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);
    try {
      store.updateCurrentDraft("backup storage failed draft");
      expect(store.workspaceDraftStorageError).toBe(translate("store.errors.localDraftStorage"));
      vi.mocked(saveWorkspaceStateWithApi).mockRejectedValueOnce(new Error("synthetic outage"));
      await store.logout();
      expect(click).toHaveBeenCalledOnce();
      expect(createURL).toHaveBeenCalledOnce();
      expect(store.isAuthenticated).toBe(false);
      expect(store.conversations.some((c) => c.draft === "backup storage failed draft")).toBe(
        false,
      );
    } finally {
      vi.unstubAllGlobals();
      store.$dispose();
    }
  });

  it("retains other tabs' backups and edits made while an older snapshot is saving", async () => {
    const first = useWorkspaceStore();
    await first.login("a@example.invalid", "synthetic-password");
    const second = useWorkspaceStore(createPinia());
    await second.login("a@example.invalid", "synthetic-password");
    first.updateCurrentDraft("first tab draft");
    second.updateCurrentDraft("second tab draft");
    const pending = deferred<Awaited<ReturnType<typeof saveWorkspaceStateWithApi>>>();
    vi.mocked(saveWorkspaceStateWithApi).mockReturnValueOnce(pending.promise);
    const saving = first.saveCurrentDraft();
    const oldSnapshot = vi.mocked(saveWorkspaceStateWithApi).mock.calls.at(-1)![1];
    first.updateCurrentDraft("first tab newer edit");
    const latest = deferred<Awaited<ReturnType<typeof saveWorkspaceStateWithApi>>>();
    vi.mocked(saveWorkspaceStateWithApi).mockReturnValueOnce(latest.promise);
    pending.resolve({ ...oldSnapshot, revision: 2 });
    await vi.waitFor(() => expect(saveWorkspaceStateWithApi).toHaveBeenCalledTimes(2));
    expect(backupValues().some((raw) => raw?.includes("first tab newer edit"))).toBe(true);
    expect(backupValues().some((raw) => raw?.includes("second tab draft"))).toBe(true);
    latest.resolve({ ...vi.mocked(saveWorkspaceStateWithApi).mock.calls.at(-1)![1], revision: 3 });
    await saving;
    expect(backupValues()).toHaveLength(1);
    expect(backupValues()[0]).toContain("second tab draft");
    first.$dispose();
    second.$dispose();
  });

  it("does not delete a recovery copy that another tab has updated", async () => {
    const state = await fetchWorkspaceStateWithApi("synthetic");
    const key = "ink.workspace.draft.v1.user-1.other-tab";
    localStorage.setItem(
      key,
      JSON.stringify({
        ...state,
        conversations: [
          {
            id: "recovery",
            title: "Recovery",
            preview: "",
            updatedAt: new Date().toISOString(),
            draft: "old draft",
            messages: [],
          },
        ],
      }),
    );
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    expect(store.recoveredWorkspaceDraft?.conversations[0]?.draft).toBe("old draft");
    const updated = JSON.parse(localStorage.getItem(key)!);
    updated.conversations[0].draft = "new draft from another tab";
    localStorage.setItem(key, JSON.stringify(updated));
    expect(store.discardRecoveredWorkspaceDraft()).toBe(true);
    expect(localStorage.getItem(key)).toContain("new draft from another tab");
    expect(store.recoveredWorkspaceDraft?.conversations[0]?.draft).toBe(
      "new draft from another tab",
    );
    await store.logout();
    expect(store.isAuthenticated).toBe(false);
    store.$dispose();
  });

  it("refreshes saved print history after an unknown submission and prevents a blind retry", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    const printer = await store.addDevice({ deviceId: "synthetic-device", setAsDefault: true });
    expect(printer).not.toBeNull();
    vi.mocked(printerService.createPrintJob).mockRejectedValueOnce(
      new AuthApiError(502, "print_submission_unknown", "unsafe generic retry"),
    );
    vi.mocked(fetchPrintJobs)
      .mockResolvedValueOnce({ printJobs: [], nextCursor: null })
      .mockResolvedValueOnce({
        printJobs: [printSummary("saved-unknown", "failed")],
        nextCursor: null,
      });
    await expect(
      store.createManualPrint({ title: "Keep me", content: "Original body" }),
    ).resolves.toBeNull();
    expect(store.printSubmissionUnknown).toBe(true);
    expect(store.flashMessage).toBe(translate("errors.api.print_submission_unknown"));
    expect(store.printHistoryJobs.some((job) => job.id === "saved-unknown")).toBe(true);
    await store.createManualPrint({ title: "Keep me", content: "Original body" });
    expect(printerService.createPrintJob).toHaveBeenCalledTimes(1);
    store.$dispose();
  });

  it("treats a print submission network interruption as unknown without encouraging a duplicate", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    await store.addDevice({ deviceId: "synthetic-device", setAsDefault: true });
    vi.mocked(printerService.createPrintJob).mockRejectedValueOnce(
      new AuthApiError(0, "network_error", "timeout; retry"),
    );
    const before = vi.mocked(fetchPrintJobs).mock.calls.length;
    await expect(
      store.createManualPrint({ title: "Network interrupted", content: "Keep the body" }),
    ).resolves.toBeNull();
    expect(store.printSubmissionUnknown).toBe(true);
    expect(store.flashMessage).toBe(translate("errors.api.print_submission_unknown"));
    expect(fetchPrintJobs).toHaveBeenCalledTimes(before + 2);
    await store.createManualPrint({ title: "Network interrupted", content: "Keep the body" });
    expect(printerService.createPrintJob).toHaveBeenCalledTimes(1);
    store.$dispose();
  });

  it("uses a fixed safety message for feedback with an unknown print result", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic-password");
    const feedbackService = await import("@/services/feedback");
    vi.mocked(feedbackService.submitFeedbackToAdmin).mockRejectedValueOnce(
      new AuthApiError(0, "network_error", "timeout; retry"),
    );
    await expect(store.submitFeedback("synthetic feedback")).resolves.toBe(false);
    expect(store.feedbackError).toBe(translate("store.errors.feedbackSubmissionUnknown"));
    store.$dispose();
  });

  it.each([201, 502])(
    "does not resend a print whose HTTP %s response body disconnects",
    async (status) => {
      const store = useWorkspaceStore();
      await store.login("a@example.invalid", "synthetic-password");
      await store.addDevice({ deviceId: "synthetic-device", setAsDefault: true });
      const response = new Response(null, { status });
      vi.spyOn(response, "json").mockRejectedValueOnce(
        new TypeError("connection lost after headers"),
      );
      const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(response);
      vi.mocked(printerService.createPrintJob).mockImplementationOnce(async () =>
        request("/api/v1/print-jobs", { method: "POST", body: "{}" }),
      );
      await expect(
        store.createManualPrint({
          title: "Accepted before disconnect",
          content: "Original content",
        }),
      ).resolves.toBeNull();
      expect(store.printSubmissionUnknown).toBe(true);
      expect(store.flashMessage).toBe(translate("errors.api.print_submission_unknown"));
      await store.createManualPrint({
        title: "Accepted before disconnect",
        content: "Original content",
      });
      expect(fetchMock).toHaveBeenCalledOnce();
      expect(printerService.createPrintJob).toHaveBeenCalledOnce();
      store.$dispose();
    },
  );

  it("persists an account print latch across relogin and requires a fresh acknowledgement for new outcomes", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic");
    await store.addDevice({ deviceId: "synthetic-device", setAsDefault: true });
    vi.mocked(printerService.createPrintJob).mockRejectedValueOnce(
      new AuthApiError(0, "network_error", "unknown"),
    );
    await store.createManualPrint({ title: "unknown", content: "body" });
    await store.logout();
    await store.login("a@example.invalid", "synthetic");
    expect(store.printSubmissionUnknown).toBe(true);
    const reloaded = useWorkspaceStore(createPinia());
    await reloaded.login("a@example.invalid", "synthetic");
    expect(reloaded.printSubmissionUnknown).toBe(true);
    expect(reloaded.acknowledgePrintOutcome()).toBe(true);
    await store.addDevice({ deviceId: "synthetic-device", setAsDefault: true });
    vi.mocked(printerService.createPrintJob).mockRejectedValueOnce(
      new AuthApiError(0, "network_error", "new unknown"),
    );
    await store.createManualPrint({ title: "new unknown", content: "body" });
    expect(store.printSubmissionUnknown).toBe(true);
    expect(reloaded.acknowledgePrintOutcome()).toBe(false);
    expect(reloaded.printSubmissionUnknown).toBe(true);
    const calls = vi.mocked(printerService.createPrintJob).mock.calls.length;
    await reloaded.createManualPrint({ title: "blind retry", content: "body" });
    expect(printerService.createPrintJob).toHaveBeenCalledTimes(calls);
    await store.logout();
    vi.mocked(authService.loginWithApi).mockResolvedValueOnce({
      user: { id: "user-b", email: "b@example.invalid", name: "B", role: "member" },
      session: {
        accessToken: "b",
        refreshToken: "b-refresh",
        accessTokenExpiresAt: new Date(Date.now() + 900000).toISOString(),
      },
    });
    await store.login("b@example.invalid", "synthetic");
    expect(store.printSubmissionUnknown).toBe(false);
    store.$dispose();
    reloaded.$dispose();
  });

  it("keeps the current live default when a recovered draft points at a removed device", async () => {
    const state = await fetchWorkspaceStateWithApi("synthetic");
    localStorage.setItem(
      "ink.workspace.draft.v1.user-1.old-tab",
      JSON.stringify({
        ...state,
        preferences: { ...state.preferences, defaultDeviceId: "removed" },
      }),
    );
    vi.mocked(fetchWorkspaceStateWithApi).mockResolvedValueOnce({
      ...state,
      preferences: { ...state.preferences, defaultDeviceId: "live" },
    });
    vi.mocked(printerService.fetchPrinters).mockResolvedValueOnce({
      devices: [{ id: "live", name: "Live device", note: "", status: "connected" }],
    });
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic");
    expect(store.restoreRecoveredWorkspaceDraft()).toBe(true);
    expect(store.defaultDeviceId).toBe("live");
    expect(store.devices.map((device) => device.id)).toEqual(["live"]);
    store.$dispose();
  });

  it("retains an in-session unknown print latch when browser storage is unavailable", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic");
    await store.addDevice({ deviceId: "synthetic-device", setAsDefault: true });
    const setItem = Storage.prototype.setItem;
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(function (this: Storage, key, value) {
      if (key.startsWith("ink.print.outcome.v1."))
        throw new DOMException("full", "QuotaExceededError");
      return setItem.call(this, key, value);
    });
    vi.mocked(printerService.createPrintJob).mockRejectedValueOnce(
      new AuthApiError(0, "network_error", "unknown"),
    );
    await store.createManualPrint({ title: "unknown", content: "body" });
    expect(store.printOutcomeStorageError).toBe(translate("store.errors.printOutcomeStorage"));
    await store.createManualPrint({ title: "blind retry", content: "body" });
    expect(printerService.createPrintJob).toHaveBeenCalledOnce();
    expect(store.printOutcomeStorageError).toBe(translate("store.errors.printOutcomeStorage"));
    await store.logout();
    await store.login("a@example.invalid", "synthetic");
    expect(store.printSubmissionUnknown).toBe(true);
    store.$dispose();
  });

  it("keeps malformed backups downloadable without blocking a later valid draft", async () => {
    const state = await fetchWorkspaceStateWithApi("synthetic");
    const broken = "ink.workspace.draft.v1.user-1.broken";
    const badShape = "ink.workspace.draft.v1.user-1.bad-shape";
    const valid = "ink.workspace.draft.v1.user-1.valid";
    localStorage.setItem(broken, "{broken");
    localStorage.setItem(badShape, JSON.stringify({ ...state, conversations: "invalid" }));
    localStorage.setItem(valid, JSON.stringify(state));
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic");
    expect(store.damagedWorkspaceDrafts.map((draft) => draft.key)).toEqual([broken, badShape]);
    expect(store.recoveredWorkspaceDraft).not.toBeNull();
    expect(localStorage.getItem(broken)).toBe("{broken");
    const createURL = vi.fn<(blob: Blob) => string>(() => "blob:damaged");
    vi.stubGlobal("URL", {
      createObjectURL: createURL,
      revokeObjectURL: vi.fn<(url: string) => void>(),
    });
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
    try {
      store.downloadDamagedWorkspaceDraft(broken);
      expect(createURL).toHaveBeenCalledOnce();
      expect(store.discardDamagedWorkspaceDraft(broken)).toBe(true);
      expect(localStorage.getItem(broken)).toBeNull();
      expect(localStorage.getItem(badShape)).not.toBeNull();
      expect(store.recoveredWorkspaceDraft).not.toBeNull();
    } finally {
      vi.unstubAllGlobals();
      store.$dispose();
    }
  });

  it("persists feedback verification separately and blocks repeated physical feedback prints", async () => {
    const service = await import("@/services/feedback");
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic");
    vi.mocked(service.submitFeedbackToAdmin).mockRejectedValueOnce(
      new AuthApiError(0, "network_error", "unknown"),
    );
    await store.submitFeedback("synthetic feedback");
    expect(store.feedbackSubmissionUnknown).toBe(true);
    expect(store.printSubmissionUnknown).toBe(false);
    await store.submitFeedback("synthetic feedback");
    expect(service.submitFeedbackToAdmin).toHaveBeenCalledOnce();
    await store.logout();
    await store.login("a@example.invalid", "synthetic");
    expect(store.feedbackSubmissionUnknown).toBe(true);
    expect(store.acknowledgeFeedbackOutcome()).toBe(true);
    await expect(store.submitFeedback("confirmed feedback")).resolves.toBe(true);
    expect(service.submitFeedbackToAdmin).toHaveBeenCalledTimes(2);
    store.$dispose();
  });

  it.each(["print", "feedback"] as const)(
    "records a late unknown %s result for the origin account without changing account B",
    async (kind) => {
      const service = await import("@/services/feedback");
      const store = useWorkspaceStore();
      await store.login("a@example.invalid", "synthetic");
      await store.addDevice({ deviceId: "synthetic-device", setAsDefault: true });
      const pending = deferred<never>();
      if (kind === "print")
        vi.mocked(printerService.createPrintJob).mockReturnValueOnce(pending.promise);
      else vi.mocked(service.submitFeedbackToAdmin).mockReturnValueOnce(pending.promise);
      const sending =
        kind === "print"
          ? store.createManualPrint({ title: "old", content: "body" })
          : store.submitFeedback("old feedback");
      await store.logout();
      vi.mocked(authService.loginWithApi).mockResolvedValueOnce({
        user: { id: "user-b", email: "b@example.invalid", name: "B", role: "member" },
        session: {
          accessToken: "b",
          refreshToken: "b-refresh",
          accessTokenExpiresAt: new Date(Date.now() + 900000).toISOString(),
        },
      });
      await store.login("b@example.invalid", "synthetic");
      pending.reject(new AuthApiError(0, "network_error", "late unknown"));
      await sending;
      expect(store.printSubmissionUnknown).toBe(false);
      expect(store.feedbackSubmissionUnknown).toBe(false);
      expect(localStorage.getItem(`ink.${kind}.outcome.v1.user-1`)).not.toBeNull();
      await store.logout();
      await store.login("a@example.invalid", "synthetic");
      expect(
        kind === "print" ? store.printSubmissionUnknown : store.feedbackSubmissionUnknown,
      ).toBe(true);
      store.$dispose();
    },
  );

  it.each(["print", "feedback"] as const)(
    "does not let an old acknowledgement clear a newer session %s marker",
    async (kind) => {
      const key = `ink.${kind}.outcome.v1.user-1`;
      localStorage.setItem(key, "old-local");
      const store = useWorkspaceStore();
      await store.login("a@example.invalid", "synthetic");
      sessionStorage.setItem(key, "new-session");
      const acknowledge =
        kind === "print" ? store.acknowledgePrintOutcome : store.acknowledgeFeedbackOutcome;
      expect(acknowledge()).toBe(false);
      expect(localStorage.getItem(key)).toBe("old-local");
      expect(sessionStorage.getItem(key)).toBe("new-session");
      expect(acknowledge()).toBe(true);
      store.$dispose();
    },
  );

  it("does not latch a rejected print when only authentication refresh fails", async () => {
    const store = useWorkspaceStore();
    await store.login("a@example.invalid", "synthetic");
    await store.addDevice({ deviceId: "synthetic-device", setAsDefault: true });
    vi.mocked(authService.refreshAuthSession).mockRejectedValueOnce(
      new AuthApiError(0, "network_error", "refresh timeout"),
    );
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ code: "unauthorized" }), { status: 401 }),
      );
    vi.mocked(printerService.createPrintJob).mockImplementationOnce(async () =>
      request("/api/v1/print-jobs", {
        method: "POST",
        headers: { Authorization: "Bearer access-token" },
        body: "{}",
      }),
    );
    await store.createManualPrint({ title: "rejected", content: "body" });
    expect(store.printSubmissionUnknown).toBe(false);
    expect(store.flashMessage).toBe(translate("errors.api.auth_refresh_failed"));
    expect(fetchMock).toHaveBeenCalledOnce();
    store.$dispose();
  });

  it.each(["print", "feedback"] as const)(
    "keeps the current %s latch when both browser storage getters are blocked",
    async (kind) => {
      const service = await import("@/services/feedback");
      const store = useWorkspaceStore();
      await store.login("a@example.invalid", "synthetic");
      await store.addDevice({ deviceId: "synthetic-device", setAsDefault: true });
      if (kind === "print")
        vi.mocked(printerService.createPrintJob).mockRejectedValueOnce(
          new AuthApiError(0, "network_error", "unknown"),
        );
      else
        vi.mocked(service.submitFeedbackToAdmin).mockRejectedValueOnce(
          new AuthApiError(0, "network_error", "unknown"),
        );
      const send = () =>
        kind === "print"
          ? store.createManualPrint({ title: "unknown", content: "body" })
          : store.submitFeedback("unknown feedback");
      await send();
      const local = vi.spyOn(window, "localStorage", "get").mockImplementation(blockedStorage);
      const session = vi.spyOn(window, "sessionStorage", "get").mockImplementation(blockedStorage);
      try {
        await send();
        expect(
          kind === "print" ? store.printSubmissionUnknown : store.feedbackSubmissionUnknown,
        ).toBe(true);
        expect(
          kind === "print" ? printerService.createPrintJob : service.submitFeedbackToAdmin,
        ).toHaveBeenCalledOnce();
        expect(
          kind === "print" ? store.acknowledgePrintOutcome() : store.acknowledgeFeedbackOutcome(),
        ).toBe(false);
      } finally {
        local.mockRestore();
        session.mockRestore();
        store.$dispose();
      }
    },
  );

  it.each(["print", "feedback"] as const)(
    "blocks another %s when a previously persisted outcome cannot be read",
    async (kind) => {
      const service = await import("@/services/feedback");
      const store = useWorkspaceStore();
      await store.login("a@example.invalid", "synthetic");
      await store.addDevice({ deviceId: "synthetic-device", setAsDefault: true });
      const submission =
        kind === "print" ? printerService.createPrintJob : service.submitFeedbackToAdmin;
      vi.mocked(submission).mockRejectedValueOnce(new AuthApiError(0, "network_error", "unknown"));
      const send = () =>
        kind === "print"
          ? store.createManualPrint({ title: "unknown", content: "body" })
          : store.submitFeedback("unknown feedback");
      await send();
      const key = `ink.${kind}.outcome.v1.user-1`;
      expect(localStorage.getItem(key)).not.toBeNull();
      expect(sessionStorage.getItem(key)).toBeNull();
      const originalStorage = window.localStorage;
      const read = vi.spyOn(window, "localStorage", "get").mockReturnValue({
        get length() {
          return originalStorage.length;
        },
        clear: originalStorage.clear.bind(originalStorage),
        key: originalStorage.key.bind(originalStorage),
        removeItem: originalStorage.removeItem.bind(originalStorage),
        setItem: originalStorage.setItem.bind(originalStorage),
        getItem(itemKey: string) {
          if (itemKey === key) throw new DOMException("blocked", "SecurityError");
          return originalStorage.getItem(itemKey);
        },
      });
      try {
        await send();
        expect(submission).toHaveBeenCalledOnce();
        expect(
          kind === "print" ? store.printSubmissionUnknown : store.feedbackSubmissionUnknown,
        ).toBe(true);
        expect(
          kind === "print" ? store.printOutcomeStorageError : store.feedbackOutcomeStorageError,
        ).not.toBe("");
        expect(
          kind === "print" ? store.acknowledgePrintOutcome() : store.acknowledgeFeedbackOutcome(),
        ).toBe(false);
      } finally {
        read.mockRestore();
        store.$dispose();
      }
    },
  );

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

  it("supports print queue updates and preferences without extension actions", async () => {
    const store = useWorkspaceStore();
    const pendingJob = store.pendingPrintJobs.find((job) => job.status === "pending");
    await store.confirmPrint(pendingJob!.id);
    store.setTheme("dark");
    store.setLoginProtection(false);
    await store.logout();
    expect(store.printJobs.find((job) => job.id === pendingJob!.id)?.status).toBe("queued");
    expect(store.selectedTheme).toBe("dark");
    expect(store.isAuthenticated).toBe(false);
    expect(store).not.toHaveProperty("createSchedule");
    expect(store).not.toHaveProperty("uploadPlugin");
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

  it("retains core printer and print data when AI loading fails", async () => {
    const error = new Error("AI unavailable");
    const printer = {
      id: "device-api-1",
      name: "My printer",
      status: "connected" as const,
      note: "",
    };
    const job = printSummary("existing-print", "completed");
    vi.mocked(printerService.fetchPrinters).mockResolvedValueOnce({ devices: [printer] });
    vi.mocked(fetchPrintJobs)
      .mockResolvedValueOnce({ printJobs: [], nextCursor: null })
      .mockResolvedValueOnce({ printJobs: [job], nextCursor: null });
    vi.mocked(aiService.fetchAIConfigSummary).mockRejectedValueOnce(error);
    const store = useWorkspaceStore();
    await expect(store.login("name@example.com", "secret")).resolves.toBe(true);
    expect(store.devices).toEqual([printer]);
    expect(store.remotePrintJobs).toEqual([job]);
    expect(store.printerSyncError).toBe("");
    expect(store.aiConfigError).toBe(error.message);
    expect(store.aiConfigSummary.model).toBe("Ink AI");
  });

  it("discards a device binding response after signing in to another account", async () => {
    vi.useFakeTimers();
    try {
      const store = useWorkspaceStore();
      await store.login("a@example.com", "secret");
      const binding = deferred<Awaited<ReturnType<typeof printerService.bindPrinter>>>();
      vi.mocked(printerService.bindPrinter).mockReturnValueOnce(binding.promise);
      const adding = store.addDevice({
        name: "A printer",
        deviceId: "a-device",
        setAsDefault: true,
      });

      await store.logout();
      vi.mocked(authService.loginWithApi).mockResolvedValueOnce({
        user: { id: "user-b", email: "b@example.com", name: "B", role: "member" },
        session: {
          accessToken: "b-token",
          refreshToken: "b-refresh",
          accessTokenExpiresAt: new Date(Date.now() + 900_000).toISOString(),
        },
      });
      await store.login("b@example.com", "secret");
      vi.mocked(saveWorkspaceStateWithApi).mockClear();
      const flash = store.flashMessage;
      binding.resolve({ id: "a-binding", name: "A printer", status: "connected", note: "" });

      await expect(adding).resolves.toBeNull();
      await vi.advanceTimersByTimeAsync(750);
      expect(store.authUser?.id).toBe("user-b");
      expect(store.devices).toEqual([]);
      expect(store.defaultDeviceId).toBe("");
      expect(store.flashMessage).toBe(flash);
      expect(saveWorkspaceStateWithApi).not.toHaveBeenCalled();
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("preserves a new login when an older unauthorized request fails to refresh", async () => {
    const store = authenticateStore();
    const refreshed = deferred<Awaited<ReturnType<typeof authService.refreshAuthSession>>>();
    const refreshStarted = deferred<void>();
    vi.mocked(authService.refreshAuthSession).mockImplementationOnce(() => {
      refreshStarted.resolve();
      return refreshed.promise;
    });
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ message: "expired" }), { status: 401 }));
    const response = request("/api/v1/account-check", {
      headers: { Authorization: "Bearer access-token" },
    }).catch((error: unknown) => error);
    await refreshStarted.promise;

    await store.logout();
    vi.mocked(authService.loginWithApi).mockResolvedValueOnce({
      user: { id: "user-b", email: "b@example.com", name: "B", role: "member" },
      session: {
        accessToken: "b-token",
        refreshToken: "b-refresh",
        accessTokenExpiresAt: new Date(Date.now() + 900_000).toISOString(),
      },
    });
    await store.login("b@example.com", "secret");
    refreshed.reject(new Error("Old refresh token revoked"));

    await expect(response).resolves.toMatchObject({ status: 401 });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(store.isAuthenticated).toBe(true);
    expect(store.authUser?.id).toBe("user-b");
    expect(store.authSession?.accessToken).toBe("b-token");
  });

  it.each(["unauthorized", "expired", "invalidExpiry"] as const)(
    "restores guest data without persisting account content when the session is %s",
    async (failure) => {
      const store = useWorkspaceStore();
      store.updateCurrentDraft("guest draft to retain");
      await nextTick();
      const guest = JSON.parse(
        window.localStorage.getItem("ink.workspace.v1")!,
      ) as PersistedWorkspaceState;
      await store.login("name@example.com", "secret");
      store.updateCurrentDraft("account content that must stay private");
      await nextTick();
      const storage = window.localStorage;
      // jsdom Storage owns its methods on the prototype; the test setup's
      // fallback storage exposes them directly on the instance.
      const storageMethods: Storage = Object.hasOwn(storage, "setItem")
        ? storage
        : Object.getPrototypeOf(storage);
      const writes = vi.spyOn(storageMethods, "setItem");

      if (failure === "unauthorized") {
        vi.mocked(authService.refreshAuthSession).mockRejectedValueOnce(
          new AuthApiError(401, "unauthorized", "Session expired"),
        );
        vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
          new Response(JSON.stringify({ message: "expired" }), { status: 401 }),
        );
        await request("/api/v1/expiry-check", {
          headers: { Authorization: "Bearer access-token" },
        }).catch(() => undefined);
      } else {
        store.authSession!.accessTokenExpiresAt =
          failure === "expired" ? new Date(Date.now() - 1000).toISOString() : "invalid";
        if (failure === "expired") {
          vi.mocked(authService.refreshAuthSession).mockRejectedValueOnce(
            new AuthApiError(401, "unauthorized", "Session expired"),
          );
        }
        await store.refreshSessionIfNeeded();
      }
      await nextTick();

      expect(store.isAuthenticated).toBe(false);
      expect(store.conversations).toEqual(guest.conversations);
      expect(store.devices).toEqual(guest.devices);
      const saved = window.localStorage.getItem("ink.workspace.v1")!;
      expect(saved).toContain("guest draft to retain");
      expect(saved).not.toContain("account content that must stay private");
      expect(writes).toHaveBeenCalled();
      expect(
        writes.mock.calls
          .filter(([key]) => key === "ink.workspace.v1")
          .every(([, value]) => !value.includes("account content that must stay private")),
      ).toBe(true);
    },
  );

  it.each(["resolved", "rejected"] as const)(
    "preserves a new login after an older expiry refresh is %s",
    async (outcome) => {
      const store = authenticateStore();
      store.authSession!.accessTokenExpiresAt = new Date(Date.now() - 1000).toISOString();
      const oldUser = store.authUser!;
      const oldSession = { ...store.authSession! };
      const refreshed = deferred<Awaited<ReturnType<typeof authService.refreshAuthSession>>>();
      vi.mocked(authService.refreshAuthSession).mockReturnValueOnce(refreshed.promise);
      const refreshing = store.refreshSessionIfNeeded();

      await store.logout();
      vi.mocked(authService.loginWithApi).mockResolvedValueOnce({
        user: { id: "user-b", email: "b@example.com", name: "B", role: "member" },
        session: {
          accessToken: "b-token",
          refreshToken: "b-refresh",
          accessTokenExpiresAt: new Date(Date.now() + 900_000).toISOString(),
        },
      });
      await store.login("b@example.com", "secret");
      if (outcome === "resolved") {
        refreshed.resolve({
          user: oldUser,
          session: { ...oldSession, accessToken: "a-refreshed" },
        });
      } else {
        refreshed.reject(new Error("Old refresh token revoked"));
      }

      await expect(refreshing).resolves.toBe(false);
      expect(store.isAuthenticated).toBe(true);
      expect(store.authUser?.id).toBe("user-b");
      expect(store.authSession?.accessToken).toBe("b-token");
    },
  );

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

  it("retries an in-flight workspace save after same-account token rotation", async () => {
    vi.useFakeTimers();
    try {
      const store = useWorkspaceStore();
      await store.login("name@example.com", "secret");
      vi.mocked(saveWorkspaceStateWithApi).mockImplementationOnce(async (_token, _state) => {
        store.authSession = { ...store.authSession!, accessToken: "rotated-token" };
        throw new AuthApiError(401, "unauthorized", "expired old token");
      });
      store.updateCurrentDraft("pending draft survives token rotation");
      await vi.advanceTimersByTimeAsync(750);
      expect(vi.mocked(saveWorkspaceStateWithApi).mock.calls.map(([token]) => token)).toEqual([
        "access-token",
        "rotated-token",
      ]);
      expect(vi.mocked(saveWorkspaceStateWithApi).mock.calls[1]?.[1]).toEqual(
        vi.mocked(saveWorkspaceStateWithApi).mock.calls[0]?.[1],
      );
      expect(store.workspaceSyncError).toBe("");
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("sends each saved revision with the next workspace change", async () => {
    vi.useFakeTimers();
    try {
      vi.mocked(saveWorkspaceStateWithApi).mockImplementationOnce(async (_token, state) => ({
        ...state,
        revision: 2,
      }));
      const store = useWorkspaceStore();
      await store.login("name@example.com", "secret");
      store.updateCurrentDraft("first change");
      await vi.advanceTimersByTimeAsync(750);
      store.updateCurrentDraft("second change");
      await vi.advanceTimersByTimeAsync(750);

      expect(
        vi.mocked(saveWorkspaceStateWithApi).mock.calls.map(([, state]) => state.revision),
      ).toEqual([1, 2]);
      expect(store.activeConversation?.draft).toBe("second change");
      await vi.advanceTimersByTimeAsync(60000);
      expect(saveWorkspaceStateWithApi).toHaveBeenCalledTimes(2);
      expect(fetchPrintJobStatuses).toHaveBeenCalledTimes(4);
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it.each([
    [409, "workspace_conflict"],
    [428, "workspace_revision_required"],
  ] as const)(
    "preserves local edits after a %s save conflict until an explicit reload",
    async (status, code) => {
      vi.useFakeTimers();
      try {
        const store = useWorkspaceStore();
        await store.login("name@example.com", "secret");
        vi.mocked(saveWorkspaceStateWithApi).mockRejectedValueOnce(
          new AuthApiError(status, code, "Workspace changed elsewhere"),
        );
        store.updateCurrentDraft("local draft to retain");
        await vi.advanceTimersByTimeAsync(750);

        expect(store.workspaceConflict).toBe(true);
        expect(store.workspaceSyncError).toBe(translate(`errors.api.${code}`));
        expect(store.activeConversation?.draft).toBe("local draft to retain");
        store.updateCurrentDraft("further local edit");
        await vi.advanceTimersByTimeAsync(750);
        await expect(store.retrySynchronization()).resolves.toBe(false);
        expect(saveWorkspaceStateWithApi).toHaveBeenCalledTimes(1);
        expect(store.activeConversation?.draft).toBe("further local edit");

        const latest = await fetchWorkspaceStateWithApi("access-token");
        vi.mocked(fetchWorkspaceStateWithApi).mockResolvedValueOnce({ ...latest, revision: 3 });
        await expect(store.reloadConflictedWorkspace()).resolves.toBe(true);
        expect(store.workspaceConflict).toBe(false);
        expect(store.workspaceSyncError).toBe("");
        store.updateCurrentDraft("after reload");
        await vi.advanceTimersByTimeAsync(750);
        expect(vi.mocked(saveWorkspaceStateWithApi).mock.calls.at(-1)?.[1].revision).toBe(3);
      } finally {
        vi.clearAllTimers();
        vi.useRealTimers();
      }
    },
  );

  it("retries a failed save without replacing its current draft", async () => {
    vi.useFakeTimers();
    try {
      const store = useWorkspaceStore();
      await store.login("name@example.com", "secret");
      vi.mocked(saveWorkspaceStateWithApi).mockRejectedValueOnce(new Error("Connection lost"));
      store.updateCurrentDraft("draft remains local");
      await vi.advanceTimersByTimeAsync(750);
      expect(store.workspaceSyncError).toBe("Connection lost");
      expect(store.workspaceConflict).toBe(false);

      await expect(store.retrySynchronization()).resolves.toBe(true);
      expect(store.workspaceSyncError).toBe("");
      expect(store.activeConversation?.draft).toBe("draft remains local");
      expect(saveWorkspaceStateWithApi).toHaveBeenCalledTimes(2);
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("saves a new account edit while an old account save remains in flight", async () => {
    vi.useFakeTimers();
    try {
      const store = useWorkspaceStore();
      await store.login("a@example.com", "secret");
      const oldSave = deferred<Awaited<ReturnType<typeof saveWorkspaceStateWithApi>>>();
      vi.mocked(saveWorkspaceStateWithApi).mockReturnValueOnce(oldSave.promise);
      store.updateCurrentDraft("A private draft");
      await vi.advanceTimersByTimeAsync(750);
      const oldSnapshot = vi.mocked(saveWorkspaceStateWithApi).mock.calls[0]![1];
      store.authSession = null;
      store.authUser = null;
      vi.mocked(authService.loginWithApi).mockResolvedValueOnce({
        user: { id: "user-b", email: "b@example.com", name: "B", role: "member" },
        session: {
          accessToken: "b-token",
          refreshToken: "b-refresh",
          accessTokenExpiresAt: new Date(Date.now() + 900_000).toISOString(),
        },
      });
      await store.login("b@example.com", "secret");
      store.updateCurrentDraft("B single edit");
      await vi.advanceTimersByTimeAsync(750);
      expect(saveWorkspaceStateWithApi).toHaveBeenCalledTimes(2);
      expect(vi.mocked(saveWorkspaceStateWithApi).mock.calls[1]?.[0]).toBe("b-token");
      expect(vi.mocked(saveWorkspaceStateWithApi).mock.calls[1]?.[1].conversations[0]?.draft).toBe(
        "B single edit",
      );
      oldSave.resolve({ ...oldSnapshot, revision: 99 });
      await vi.advanceTimersByTimeAsync(0);
      expect(store.activeConversation?.draft).toBe("B single edit");
      expect(store.workspaceSyncError).toBe("");
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("blocks conversation edits while a confirmed reload is in flight", async () => {
    const store = useWorkspaceStore();
    await store.login("name@example.com", "secret");
    store.updateCurrentDraft("before reload");
    const latest = await fetchWorkspaceStateWithApi("access-token");
    const loading = deferred<Awaited<ReturnType<typeof fetchWorkspaceStateWithApi>>>();
    vi.mocked(fetchWorkspaceStateWithApi).mockReturnValueOnce(loading.promise);
    const reload = store.reloadConflictedWorkspace();
    const conversation = store.activeConversation!;
    store.updateCurrentDraft("must not disappear");
    store.createConversation();
    expect(store.deleteConversation(conversation.id)).toBe(false);
    await expect(store.sendCurrentDraft()).resolves.toBe(false);
    expect(store.activeConversation?.draft).toBe("before reload");
    expect(store.conversations).toHaveLength(1);
    const preferences = {
      theme: store.selectedTheme,
      locale: store.localePreference,
      loginProtection: store.loginProtectionEnabled,
      tutorial: store.tutorialTabEnabled,
      defaultDevice: store.defaultDeviceId,
    };
    store.setTheme("dark");
    store.setLocale("en-US");
    store.setLoginProtection(!preferences.loginProtection);
    store.setTutorialTabEnabled(!preferences.tutorial);
    store.setDefaultDevice("missing-device");
    expect({
      theme: store.selectedTheme,
      locale: store.localePreference,
      loginProtection: store.loginProtectionEnabled,
      tutorial: store.tutorialTabEnabled,
      defaultDevice: store.defaultDeviceId,
    }).toEqual(preferences);
    loading.resolve({ ...latest, revision: 3 });
    await expect(reload).resolves.toBe(true);
    store.updateCurrentDraft("after reload");
    expect(store.activeConversation?.draft).toBe("after reload");
  });

  it("preserves the session after a temporary refresh failure and permits a later retry", async () => {
    const store = useWorkspaceStore();
    await store.login("name@example.com", "secret");
    vi.mocked(authService.refreshAuthSession).mockRejectedValueOnce(
      new AuthApiError(0, "network_error", "offline"),
    );
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (_input, init) =>
        new Headers(init?.headers).get("Authorization") === "Bearer fresh-token"
          ? new Response(JSON.stringify({ ok: true }), { status: 200 })
          : new Response("expired", { status: 401 }),
      );
    await expect(
      request("/test", { headers: { Authorization: "Bearer access-token" } }),
    ).rejects.toMatchObject({ code: "auth_refresh_failed", status: 401 });
    expect(store.isAuthenticated).toBe(true);
    expect(store.authSession?.refreshToken).toBe("refresh-token");
    expect(window.localStorage.getItem("ink.auth.session.v1")).toContain("refresh-token");
    vi.mocked(authService.refreshAuthSession).mockResolvedValueOnce({
      user: store.authUser!,
      session: { ...store.authSession!, accessToken: "fresh-token", refreshToken: "fresh-refresh" },
    });
    await expect(
      request("/test", { headers: { Authorization: "Bearer access-token" } }),
    ).resolves.toEqual({ ok: true });
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it("uses another tab's rotated session under a browser lock", async () => {
    const store = useWorkspaceStore();
    await store.login("name@example.com", "secret");
    const fresh = {
      ...store.authSession!,
      userId: store.authUser!.id,
      accessToken: "shared-fresh",
      refreshToken: "shared-refresh",
    };
    const lock = vi.fn<(name: string, callback: () => Promise<unknown>) => Promise<unknown>>(
      async (_name, callback) => {
        window.localStorage.setItem("ink.auth.session.v1", JSON.stringify(fresh));
        return callback();
      },
    );
    Object.defineProperty(navigator, "locks", { configurable: true, value: { request: lock } });
    try {
      vi.spyOn(globalThis, "fetch").mockImplementation(async (_input, init) =>
        new Headers(init?.headers).get("Authorization") === "Bearer shared-fresh"
          ? new Response(JSON.stringify({ ok: true }), { status: 200 })
          : new Response("expired", { status: 401 }),
      );
      await expect(
        request("/test", { headers: { Authorization: "Bearer access-token" } }),
      ).resolves.toEqual({ ok: true });
      expect(lock).toHaveBeenCalledWith("ink.auth.refresh", expect.any(Function));
      expect(authService.refreshAuthSession).not.toHaveBeenCalled();
      expect(store.authSession?.accessToken).toBe("shared-fresh");
    } finally {
      Reflect.deleteProperty(navigator, "locks");
    }
  });

  it("keeps an AI save failure separate from synchronization retries", async () => {
    const store = useWorkspaceStore();
    await store.login("name@example.com", "secret");
    vi.mocked(aiService.saveAIConfig).mockRejectedValueOnce(new Error("save failed"));
    await expect(
      store.saveAIServiceConfig({
        providerName: "Draft",
        providerType: "openai-compatible",
        baseUrl: "https://draft.example/v1",
        model: "draft",
        apiKey: "",
      }),
    ).resolves.toBe(false);
    expect(store.aiConfigSaveError).toBe("save failed");
    expect(store.aiConfigError).toBe("");
    vi.mocked(aiService.fetchAIConfigSummary).mockClear();
    await store.retrySynchronization();
    expect(aiService.fetchAIConfigSummary).not.toHaveBeenCalled();
    const savedSummary = store.aiConfigSummary;
    store.deviceSyncError = "printers offline";
    await store.retrySynchronization();
    expect(aiService.fetchAIConfigSummary).toHaveBeenCalledTimes(1);
    expect(store.aiConfigSummary).toBe(savedSummary);
    expect(store.aiConfigSaveError).toBe("save failed");
  });

  it("keeps healthy status metadata and retries pages after a page request fails", async () => {
    vi.useFakeTimers();
    try {
      const store = useWorkspaceStore();
      await store.login("name@example.com", "secret");
      vi.mocked(fetchPrintJobs).mockClear().mockRejectedValue(new Error("page offline"));
      vi.mocked(fetchPrintJobStatuses).mockResolvedValue({
        printJobs: [],
        counts: printCounts({ queued: 1 }),
        latestJobId: "new-job",
      });
      await vi.advanceTimersByTimeAsync(1500);
      expect(store.remotePrintCounts?.queued).toBe(1);
      expect(store.printSyncError).toContain("page offline");
      vi.mocked(fetchPrintJobs).mockResolvedValue({
        printJobs: [printSummary("new-job", "queued")],
        nextCursor: null,
      });
      await vi.advanceTimersByTimeAsync(5000);
      expect(fetchPrintJobs).toHaveBeenCalledTimes(4);
      expect(store.printSyncError).toBe("");
      expect(store.remotePrintJobs.some((job) => job.id === "new-job")).toBe(true);
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("reads the refreshed session for later status chunks and fetches counts only once", async () => {
    vi.useFakeTimers();
    try {
      const store = useWorkspaceStore();
      const jobs = Array.from({ length: 101 }, (_, index) =>
        printSummary(`chunk-${index}`, "queued"),
      );
      vi.mocked(fetchPrintJobs).mockImplementation(async (_token, options) => ({
        printJobs: options?.status === "active" ? jobs : [],
        nextCursor: null,
      }));
      await store.login("name@example.com", "secret");
      vi.mocked(fetchPrintJobStatuses).mockClear();
      vi.mocked(fetchPrintJobStatuses).mockImplementation(
        async (_token, ids, _since, includeMetadata) => {
          store.authSession = { ...store.authSession!, accessToken: "fresh-token" };
          return {
            printJobs: ids.map((id) => ({
              id,
              deviceId: "device-api-1",
              status: "queued" as const,
              updatedAt: "2026-04-10T00:00:00.123400Z",
            })),
            ...(includeMetadata ? { counts: printCounts({ queued: 101 }) } : {}),
            latestJobId: null,
          };
        },
      );
      await vi.advanceTimersByTimeAsync(1500);
      expect(fetchPrintJobStatuses).toHaveBeenCalledTimes(2);
      expect(
        vi
          .mocked(fetchPrintJobStatuses)
          .mock.calls.map(([token, ids, _since, metadata]) => [token, ids.length, metadata]),
      ).toEqual([
        ["access-token", 100, true],
        ["fresh-token", 1, false],
      ]);
      expect(store.printSyncError).toBe("");
      expect(store.remotePrintCounts?.queued).toBe(101);
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("unlocks guest controls when signing out during a workspace reload", async () => {
    const store = useWorkspaceStore();
    await store.login("name@example.com", "secret");
    const latest = await fetchWorkspaceStateWithApi("access-token");
    const loading = deferred<Awaited<ReturnType<typeof fetchWorkspaceStateWithApi>>>();
    vi.mocked(fetchWorkspaceStateWithApi).mockReturnValueOnce(loading.promise);
    const reload = store.reloadConflictedWorkspace();
    expect(store.workspaceLoading).toBe(true);
    await store.logout();
    expect(store.workspaceLoading).toBe(false);
    store.updateCurrentDraft("guest can edit after logout");
    expect(store.activeConversation?.draft).toBe("guest can edit after logout");
    loading.resolve(latest);
    await expect(reload).resolves.toBe(false);
    expect(store.workspaceLoading).toBe(false);
    expect(store.activeConversation?.draft).toBe("guest can edit after logout");
  });

  it("does not clear a new account when an older login workspace load finishes", async () => {
    const store = useWorkspaceStore();
    const latest = await fetchWorkspaceStateWithApi("access-token");
    const loading = deferred<Awaited<ReturnType<typeof fetchWorkspaceStateWithApi>>>();
    const started = deferred<void>();
    vi.mocked(fetchWorkspaceStateWithApi).mockImplementationOnce(() => {
      started.resolve();
      return loading.promise;
    });
    const loggingInA = store.login("a@example.com", "secret");
    await started.promise;
    await store.logout();
    vi.mocked(authService.loginWithApi).mockResolvedValueOnce({
      user: { id: "user-b", email: "b@example.com", name: "B", role: "member" },
      session: {
        accessToken: "b-token",
        refreshToken: "b-refresh",
        accessTokenExpiresAt: new Date(Date.now() + 900_000).toISOString(),
      },
    });
    await store.login("b@example.com", "secret");
    store.updateCurrentDraft("B draft stays private");
    loading.resolve(latest);
    await expect(loggingInA).resolves.toBe(false);
    expect(store.authUser?.id).toBe("user-b");
    expect(store.authSession?.accessToken).toBe("b-token");
    expect(store.activeConversation?.draft).toBe("B draft stays private");
  });

  it("bootstraps an expired persisted session after its own successful refresh", async () => {
    window.localStorage.setItem(
      "ink.auth.session.v1",
      JSON.stringify({
        accessToken: "expired-token",
        refreshToken: "refresh-token",
        accessTokenExpiresAt: new Date(Date.now() - 1000).toISOString(),
      }),
    );
    vi.mocked(authService.refreshAuthSession).mockResolvedValueOnce({
      user: { id: "user-1", email: "name@example.com", name: "Ink User", role: "member" },
      session: {
        accessToken: "refreshed-token",
        refreshToken: "next-refresh",
        accessTokenExpiresAt: new Date(Date.now() + 900_000).toISOString(),
      },
    });
    const store = useWorkspaceStore();
    await expect(store.initializeAuth()).resolves.toBe(true);
    expect(store.authSession?.accessToken).toBe("refreshed-token");
    expect(store.authBootstrapping).toBe(false);
    expect(store.conversations).toHaveLength(1);
  });

  it("downloads the local draft without credentials or a network save", async () => {
    const store = useWorkspaceStore();
    await store.login("name@example.com", "secret");
    store.updateCurrentDraft("keep this local draft");
    const createObjectURL = vi.fn<(blob: Blob) => string>(() => "blob:draft");
    const revokeObjectURL = vi.fn<(url: string) => void>();
    const nativeURL = globalThis.URL;
    vi.stubGlobal(
      "URL",
      Object.assign(class extends nativeURL {}, { createObjectURL, revokeObjectURL }),
    );
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);
    try {
      store.downloadWorkspaceDraft();
      const blob = createObjectURL.mock.calls[0]![0];
      const exported = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.addEventListener("load", () => resolve(String(reader.result)), { once: true });
        reader.addEventListener("error", () => reject(reader.error), { once: true });
        reader.readAsText(blob);
      });
      expect(JSON.parse(exported).conversations[0].draft).toBe("keep this local draft");
      expect(exported).not.toContain("access-token");
      expect(exported).not.toContain("refresh-token");
      expect(click).toHaveBeenCalledOnce();
      expect(revokeObjectURL).toHaveBeenCalledWith("blob:draft");
      expect(saveWorkspaceStateWithApi).not.toHaveBeenCalled();
    } finally {
      vi.unstubAllGlobals();
      click.mockRestore();
    }
  });

  it("retains a device loading failure across successful print polls and allows retry", async () => {
    vi.useFakeTimers();
    try {
      vi.mocked(printerService.fetchPrinters).mockRejectedValueOnce(
        new Error("Printer list unavailable"),
      );
      const store = useWorkspaceStore();
      await store.login("name@example.com", "secret");
      expect(store.printerSyncError).toBe("Printer list unavailable");
      await vi.advanceTimersByTimeAsync(1500);
      expect(store.printerSyncError).toBe("Printer list unavailable");

      await expect(store.retrySynchronization()).resolves.toBe(true);
      expect(store.printerSyncError).toBe("");
      expect(printerService.fetchPrinters).toHaveBeenCalledTimes(2);
    } finally {
      vi.clearAllTimers();
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

  it("disposes pending workspace saves, flash timers and visibility listeners", async () => {
    vi.useFakeTimers();
    const timers = trackWorkspaceTimers();
    const addListener = vi.spyOn(document, "addEventListener");
    const removeListener = vi.spyOn(document, "removeEventListener");
    try {
      const store = useWorkspaceStore();
      await store.login("name@example.com", "secret");
      store.updateCurrentDraft("a draft from a disposed store");
      const visibilityListener = addListener.mock.calls.find(
        ([event]) => event === "visibilitychange",
      )?.[1];
      expect(visibilityListener).toBeDefined();
      const pendingTimers = timers
        .applicationTimers()
        .filter(({ handle }) => !timers.cleared.mock.calls.some(([cleared]) => cleared === handle));
      expect(pendingTimers.map(({ delay }) => delay)).toEqual(
        expect.arrayContaining([750, 1500, 2600]),
      );
      const statusCalls = vi.mocked(fetchPrintJobStatuses).mock.calls.length;
      const scheduledBeforeDisposal = timers.scheduled.mock.calls.length;

      store.$dispose();

      expect(removeListener).toHaveBeenCalledWith("visibilitychange", visibilityListener);
      for (const { handle } of pendingTimers) expect(timers.cleared).toHaveBeenCalledWith(handle);
      document.dispatchEvent(new Event("visibilitychange"));
      await vi.advanceTimersByTimeAsync(120000);
      expect(saveWorkspaceStateWithApi).not.toHaveBeenCalled();
      expect(fetchPrintJobStatuses).toHaveBeenCalledTimes(statusCalls);
      expect(timers.applicationTimers(scheduledBeforeDisposal)).toEqual([]);
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("does not restart polling when an in-flight status request finishes after disposal", async () => {
    vi.useFakeTimers();
    const timers = trackWorkspaceTimers();
    try {
      const store = useWorkspaceStore();
      await store.login("name@example.com", "secret");
      const statuses = deferred<Awaited<ReturnType<typeof fetchPrintJobStatuses>>>();
      vi.mocked(fetchPrintJobStatuses).mockReturnValueOnce(statuses.promise);
      await vi.advanceTimersByTimeAsync(1500);
      expect(fetchPrintJobStatuses).toHaveBeenCalledTimes(2);
      const scheduledBeforeDisposal = timers.scheduled.mock.calls.length;

      store.$dispose();
      statuses.resolve({
        printJobs: [],
        counts: printCounts({ queued: 8 }),
        latestJobId: "late-job",
      });
      await vi.advanceTimersByTimeAsync(0);
      expect(timers.applicationTimers(scheduledBeforeDisposal)).toEqual([]);
      await vi.advanceTimersByTimeAsync(120000);
      expect(fetchPrintJobStatuses).toHaveBeenCalledTimes(2);
      expect(fetchPrintJobs).toHaveBeenCalledTimes(2);
      expect(store.remotePrintCounts?.queued).toBe(0);
      expect(saveWorkspaceStateWithApi).not.toHaveBeenCalled();
      expect(timers.applicationTimers(scheduledBeforeDisposal)).toEqual([]);
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("does not send a queued workspace save after an in-flight save finishes on a disposed store", async () => {
    vi.useFakeTimers();
    const timers = trackWorkspaceTimers();
    try {
      const store = useWorkspaceStore();
      await store.login("name@example.com", "secret");
      let finishSave!: () => void;
      vi.mocked(saveWorkspaceStateWithApi).mockImplementationOnce(
        (_accessToken, state) =>
          new Promise((resolve) => {
            finishSave = () => resolve(state);
          }),
      );
      store.setTheme("dark");
      await vi.advanceTimersByTimeAsync(750);
      expect(saveWorkspaceStateWithApi).toHaveBeenCalledTimes(1);
      store.setTheme("light");
      await vi.advanceTimersByTimeAsync(750);
      const scheduledBeforeDisposal = timers.scheduled.mock.calls.length;

      store.$dispose();
      finishSave();
      await vi.advanceTimersByTimeAsync(0);
      await vi.advanceTimersByTimeAsync(120000);

      expect(saveWorkspaceStateWithApi).toHaveBeenCalledTimes(1);
      expect(timers.applicationTimers(scheduledBeforeDisposal)).toEqual([]);
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("cancels anonymous print completion when its store is disposed", async () => {
    vi.useFakeTimers();
    const timers = trackWorkspaceTimers();
    try {
      const store = useWorkspaceStore();
      const job = await store.createManualPrint({ title: "guest print", content: "guest body" });
      expect(job?.status).toBe("queued");
      const pendingTimers = timers.applicationTimers();
      expect(pendingTimers.map(({ delay }) => delay)).toEqual(expect.arrayContaining([500, 2600]));
      const scheduledBeforeDisposal = timers.scheduled.mock.calls.length;

      store.$dispose();
      for (const { handle } of pendingTimers) expect(timers.cleared).toHaveBeenCalledWith(handle);
      await vi.advanceTimersByTimeAsync(120000);

      expect(timers.applicationTimers(scheduledBeforeDisposal)).toEqual([]);
      expect(store.printJobs.find((item) => item.id === job!.id)?.status).toBe("queued");
      expect(saveWorkspaceStateWithApi).not.toHaveBeenCalled();
    } finally {
      vi.clearAllTimers();
      vi.useRealTimers();
    }
  });

  it("does not retry an unauthorized request with a token refreshed after store disposal", async () => {
    const store = authenticateStore();
    const refreshed = deferred<Awaited<ReturnType<typeof authService.refreshAuthSession>>>();
    const refreshStarted = deferred<void>();
    vi.mocked(authService.refreshAuthSession).mockImplementationOnce(() => {
      refreshStarted.resolve();
      return refreshed.promise;
    });
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ message: "expired" }), { status: 401 }));
    const response = request("/api/v1/disposal-check", {
      headers: { Authorization: "Bearer access-token" },
    }).catch((error: unknown) => error);
    await refreshStarted.promise;

    store.$dispose();
    refreshed.resolve({
      user: store.authUser!,
      session: { ...store.authSession!, accessToken: "new-token" },
    });

    await expect(response).resolves.toMatchObject({ status: 401 });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(store.authSession?.accessToken).toBe("access-token");
  });
});
