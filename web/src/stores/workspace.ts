import { defineStore } from "pinia";
import { computed, onScopeDispose, ref, watch } from "vue";

import {
  normalizeLocalePreference,
  resolveLocalePreference,
  setI18nLocale,
  translate,
} from "@/i18n";
import { getLocalizedErrorMessage } from "@/i18n/errors";
import type { AIConfigSummary } from "@/services/ai";
import { fetchAIConfigSummary, generateAIReply, saveAIConfig } from "@/services/ai";
import {
  AuthApiError,
  changePasswordWithApi,
  fetchCurrentUser,
  loginWithApi,
  logoutWithApi,
  refreshAuthSession,
} from "@/services/auth";
import { submitFeedbackToAdmin } from "@/services/feedback";
import { configureAuthRefresh } from "@/services/http";
import { getMemorySettings, updateMemorySettings } from "@/services/memory";
import { generateReplyWithMockService } from "@/services/mockInk";
import {
  bindPrinter,
  cancelPrintJob,
  createPrintJob,
  deletePrinter,
  fetchPrintJob,
  fetchPrintJobs,
  fetchPrintJobStatuses,
  fetchPrinters,
  submitPrintJob,
  updatePrintJobDevice as updatePrintJobDeviceWithApi,
} from "@/services/printers";
import {
  createUserWithApi,
  fetchWorkspaceStateWithApi,
  saveWorkspaceStateWithApi,
} from "@/services/workspace";
import type { MemorySettings } from "@/types/memory";
import type {
  AuthSession,
  Conversation,
  ConversationMessage,
  Device,
  LocalePreference,
  PersistedWorkspaceState,
  Preferences,
  PrintJob,
  PrintJobCounts,
  PrintJobSummary,
  Schedule,
  ServiceBinding,
  SourceConnection,
  ThemeMode,
  User,
  WorkspaceState,
} from "@/types/workspace";
import {
  createId,
  formatRelativeTimestamp,
  getDeviceStatusLabel,
  getPrintStatusLabel,
  getSourceStatusLabel,
  normalizeThemeMode,
} from "@/utils/workspace";

const STORAGE_KEY = "ink.workspace.v1";
const AUTH_SESSION_STORAGE_KEY = "ink.auth.session.v1";
const ACCOUNT_DRAFT_STORAGE_PREFIX = "ink.workspace.draft.v1.";
const PRINT_OUTCOME_STORAGE_PREFIX = "ink.print.outcome.v1.";
const FEEDBACK_OUTCOME_STORAGE_PREFIX = "ink.feedback.outcome.v1.";
const REMOTE_SAVE_DEBOUNCE_MS = 750;
const REMOTE_PRINT_STATUS_POLL_MS = 5000;
const REMOTE_PRINT_STATUS_INITIAL_POLL_MS = 1500;
const REMOTE_PRINT_STATUS_IDLE_POLL_MS = 30000;
const REMOTE_PRINT_STATUS_MAX_BACKOFF_MS = 120000;

function draftContent(state: WorkspaceState) {
  const { revision: _revision, ...content } = state;
  return JSON.stringify(content);
}

function accountDraftPrefix(userId: string) {
  return `${ACCOUNT_DRAFT_STORAGE_PREFIX}${encodeURIComponent(userId)}.`;
}

function downloadState(state: WorkspaceState) {
  downloadDraftText(JSON.stringify(state, null, 2));
}

function downloadDraftText(text: string) {
  const url = URL.createObjectURL(new Blob([text], { type: "application/json" }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = "ink-workspace-draft.json";
  anchor.click();
  URL.revokeObjectURL(url);
}

function isRecoverableDraft(value: unknown): value is WorkspaceState {
  if (!value || typeof value !== "object") return false;
  const state = value as WorkspaceState;
  const preferences = state.preferences;
  return (
    Array.isArray(state.conversations) &&
    typeof state.activeConversationId === "string" &&
    state.conversations.every(
      (conversation) =>
        conversation &&
        typeof conversation.id === "string" &&
        typeof conversation.title === "string" &&
        typeof conversation.preview === "string" &&
        typeof conversation.updatedAt === "string" &&
        typeof conversation.draft === "string" &&
        Array.isArray(conversation.messages) &&
        conversation.messages.every(
          (message) =>
            message &&
            typeof message.id === "string" &&
            (message.role === "user" || message.role === "assistant") &&
            typeof message.text === "string" &&
            typeof message.createdAt === "string",
        ),
    ) &&
    !!preferences &&
    typeof preferences.defaultDeviceId === "string" &&
    typeof preferences.loginProtectionEnabled === "boolean" &&
    typeof preferences.sendConfirmationEnabled === "boolean" &&
    typeof preferences.tutorialTabEnabled === "boolean" &&
    ["light", "dark", "system"].includes(preferences.theme) &&
    ["zh-CN", "en-US", "system"].includes(preferences.locale)
  );
}

function readOutcomeMarkers(key: string, pending?: string) {
  const markers = new Set<string>();
  if (pending) markers.add(pending);
  let failed = false;
  for (const name of ["localStorage", "sessionStorage"] as const) {
    try {
      const marker = window[name].getItem(key);
      if (marker) markers.add(marker);
    } catch {
      failed = true;
    }
  }
  // Sorting a new array preserves the project's ES2022 target.
  // oxlint-disable-next-line unicorn/no-array-sort
  const values = Array.from(markers).sort();
  return { markers: values, signature: JSON.stringify(values), failed };
}

function getNow() {
  return new Date().toISOString();
}

function getErrorMessage(error: unknown, fallbackKey: string) {
  return getLocalizedErrorMessage(error, fallbackKey);
}

function isPrintSubmissionUnknown(error: unknown) {
  return (
    error instanceof AuthApiError &&
    (error.code === "print_submission_unknown" ||
      error.status === 0 ||
      error.code === "network_error")
  );
}

function createEmptyConversation(): Conversation {
  return {
    id: createId("conversation"),
    title: translate("store.seed.newConversation.title"),
    preview: translate("store.seed.newConversation.preview"),
    updatedAt: getNow(),
    draft: "",
    messages: [],
  };
}

function normalizeConversations(conversations: Conversation[]): Conversation[] {
  return conversations.length > 0 ? conversations : [createEmptyConversation()];
}

function resolveActiveConversationId(
  activeConversationId: string | undefined,
  conversations: Conversation[],
) {
  if (
    activeConversationId &&
    conversations.some((conversation) => conversation.id === activeConversationId)
  ) {
    return activeConversationId;
  }

  return conversations[0]?.id ?? "";
}

function createInitialMessages(): ConversationMessage[] {
  const createdAt = new Date(Date.now() - 1000 * 60 * 10).toISOString();

  return [
    {
      id: createId("message"),
      role: "user",
      text: translate("store.seed.initialConversation.user"),
      createdAt,
    },
    {
      id: createId("message"),
      role: "assistant",
      text: translate("store.seed.initialConversation.assistant"),
      createdAt: new Date(Date.now() - 1000 * 60 * 9).toISOString(),
    },
  ];
}

function createSeedState(): PersistedWorkspaceState {
  const now = Date.now();
  const primaryMessages = createInitialMessages();
  const conversationList: Conversation[] = [
    {
      id: "conv-today",
      title: translate("store.seed.conversations.today.title"),
      preview: translate("store.seed.conversations.today.preview"),
      updatedAt: new Date(now - 1000 * 60 * 2).toISOString(),
      draft: "",
      messages: primaryMessages,
    },
    {
      id: "conv-birthday",
      title: translate("store.seed.conversations.birthday.title"),
      preview: translate("store.seed.conversations.birthday.preview"),
      updatedAt: new Date(now - 1000 * 60 * 10).toISOString(),
      draft: "",
      messages: [
        {
          id: createId("message"),
          role: "user",
          text: translate("store.seed.conversations.birthday.user"),
          createdAt: new Date(now - 1000 * 60 * 12).toISOString(),
        },
        {
          id: createId("message"),
          role: "assistant",
          text: translate("store.seed.conversations.birthday.assistant"),
          createdAt: new Date(now - 1000 * 60 * 11).toISOString(),
        },
      ],
    },
    {
      id: "conv-shopping",
      title: translate("store.seed.conversations.shopping.title"),
      preview: translate("store.seed.conversations.shopping.preview"),
      updatedAt: new Date(now - 1000 * 60 * 60 * 18).toISOString(),
      draft: translate("store.seed.conversations.shopping.draft"),
      messages: [
        {
          id: createId("message"),
          role: "user",
          text: translate("store.seed.conversations.shopping.user"),
          createdAt: new Date(now - 1000 * 60 * 60 * 18).toISOString(),
        },
        {
          id: createId("message"),
          role: "assistant",
          text: translate("store.seed.conversations.shopping.assistant"),
          createdAt: new Date(now - 1000 * 60 * 60 * 17).toISOString(),
        },
      ],
    },
  ];
  const devices: Device[] = [
    {
      id: "device-desk",
      name: translate("store.seed.devices.desk.name"),
      status: "connected",
      note: translate("store.seed.devices.desk.note"),
    },
    {
      id: "device-bedroom",
      name: translate("store.seed.devices.bedroom.name"),
      status: "pending",
      note: translate("store.seed.devices.bedroom.note"),
    },
  ];
  const schedules: Schedule[] = [];
  const printJobs: PrintJob[] = [
    {
      id: "print-pending-message",
      title: translate("store.seed.printJobs.pending.title"),
      source: translate("store.seed.printJobs.pending.source"),
      deviceId: "device-bedroom",
      status: "pending",
      createdAt: new Date(now - 1000 * 60 * 30).toISOString(),
      updatedAt: new Date(now - 1000 * 60 * 30).toISOString(),
      content: translate("store.seed.printJobs.pending.content"),
    },
    {
      id: "print-queued-report",
      title: translate("store.seed.printJobs.queued.title"),
      source: translate("store.seed.printJobs.queued.source"),
      deviceId: "device-desk",
      status: "queued",
      createdAt: new Date(now - 1000 * 60 * 25).toISOString(),
      updatedAt: new Date(now - 1000 * 60 * 25).toISOString(),
      content: translate("store.seed.printJobs.queued.content"),
    },
    {
      id: "print-done-todo",
      title: translate("store.seed.printJobs.completedTodo.title"),
      source: translate("store.seed.printJobs.completedTodo.source"),
      deviceId: "device-desk",
      status: "completed",
      createdAt: new Date(now - 1000 * 60 * 70).toISOString(),
      updatedAt: new Date(now - 1000 * 60 * 68).toISOString(),
      content: translate("store.seed.printJobs.completedTodo.content"),
    },
    {
      id: "print-done-shopping",
      title: translate("store.seed.printJobs.completedShopping.title"),
      source: translate("store.seed.printJobs.completedShopping.source"),
      deviceId: "device-desk",
      status: "completed",
      createdAt: new Date(now - 1000 * 60 * 95).toISOString(),
      updatedAt: new Date(now - 1000 * 60 * 93).toISOString(),
      content: translate("store.seed.printJobs.completedShopping.content"),
    },
  ];
  const sources: SourceConnection[] = [];
  const preferences: Preferences = {
    loginProtectionEnabled: false,
    sendConfirmationEnabled: false,
    tutorialTabEnabled: true,
    theme: "light",
    defaultDeviceId: "device-desk",
    locale: "system",
  };
  const serviceBinding: ServiceBinding = {
    providerName: null,
    modelName: "Ink AI",
    bound: false,
  };

  return {
    authUser: null,
    devices,
    conversations: conversationList,
    activeConversationId: "conv-today",
    printJobs,
    schedules,
    sources,
    preferences,
    serviceBinding,
  };
}

function normalizeWorkspaceState(state: Partial<WorkspaceState>): WorkspaceState {
  const seed = createSeedState();
  const conversations =
    state.conversations === undefined
      ? seed.conversations
      : normalizeConversations(state.conversations);

  return {
    revision: state.revision,
    devices: state.devices ?? seed.devices,
    conversations,
    activeConversationId: resolveActiveConversationId(state.activeConversationId, conversations),
    printJobs: state.printJobs ?? seed.printJobs,
    schedules: state.schedules ?? seed.schedules,
    sources: state.sources ?? seed.sources,
    preferences: {
      loginProtectionEnabled:
        state.preferences?.loginProtectionEnabled ?? seed.preferences.loginProtectionEnabled,
      sendConfirmationEnabled: false,
      tutorialTabEnabled:
        state.preferences?.tutorialTabEnabled ?? seed.preferences.tutorialTabEnabled,
      theme: normalizeThemeMode(state.preferences?.theme ?? seed.preferences.theme),
      defaultDeviceId: state.preferences?.defaultDeviceId ?? seed.preferences.defaultDeviceId,
      locale: normalizeLocalePreference(state.preferences?.locale ?? seed.preferences.locale),
    },
    serviceBinding: {
      providerName: state.serviceBinding?.providerName ?? seed.serviceBinding.providerName,
      modelName: state.serviceBinding?.modelName ?? seed.serviceBinding.modelName,
      bound: state.serviceBinding?.bound ?? seed.serviceBinding.bound,
    },
  };
}

function readPersistedWorkspaceState() {
  if (typeof window === "undefined") {
    return createSeedState();
  }

  const raw = window.localStorage.getItem(STORAGE_KEY);

  if (!raw) {
    return createSeedState();
  }

  try {
    const parsed = JSON.parse(raw) as Partial<PersistedWorkspaceState>;

    return {
      authUser: parsed.authUser ?? null,
      ...normalizeWorkspaceState(parsed),
    };
  } catch {
    return createSeedState();
  }
}

function isPersistedAuthSession(value: unknown): value is AuthSession {
  if (!value || typeof value !== "object") {
    return false;
  }

  const candidate = value as Record<string, unknown>;
  return (
    typeof candidate.accessToken === "string" &&
    typeof candidate.refreshToken === "string" &&
    typeof candidate.accessTokenExpiresAt === "string"
  );
}

type StoredAuthSession = AuthSession & { userId?: string };

function accessTokenUserId(accessToken: string): string | undefined {
  try {
    const payload = accessToken.split(".")[1];
    if (!payload) return undefined;
    const claims = JSON.parse(atob(payload.replace(/-/g, "+").replace(/_/g, "/")));
    return typeof claims.sub === "string" ? claims.sub : undefined;
  } catch {
    return undefined;
  }
}

function readAuthSessionFromStorage(storage: Storage): StoredAuthSession | null {
  const raw = storage.getItem(AUTH_SESSION_STORAGE_KEY);
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw);
    return isPersistedAuthSession(parsed) ? parsed : null;
  } catch {
    return null;
  }
}

function readPersistedAuthSession() {
  if (typeof window === "undefined") {
    return null;
  }

  return (
    readAuthSessionFromStorage(window.sessionStorage) ??
    readAuthSessionFromStorage(window.localStorage)
  );
}

function writePersistedAuthSession(
  session: AuthSession | null,
  persistAcrossRestarts: boolean,
  userId?: string,
) {
  if (typeof window === "undefined") {
    return;
  }

  if (!session) {
    window.sessionStorage.removeItem(AUTH_SESSION_STORAGE_KEY);
    window.localStorage.removeItem(AUTH_SESSION_STORAGE_KEY);
    return;
  }

  const storage = persistAcrossRestarts ? window.localStorage : window.sessionStorage;
  const otherStorage = persistAcrossRestarts ? window.sessionStorage : window.localStorage;
  storage.setItem(AUTH_SESSION_STORAGE_KEY, JSON.stringify({ ...session, userId }));
  otherStorage.removeItem(AUTH_SESSION_STORAGE_KEY);
}

function sortPrintJobsByUpdatedAt<T extends PrintJobSummary>(printJobs: T[]) {
  // The project targets ES2022; use a copied array so the fallback remains non-mutating.
  // oxlint-disable-next-line unicorn/no-array-sort
  return [...printJobs].sort(
    (left, right) => new Date(right.updatedAt).getTime() - new Date(left.updatedAt).getTime(),
  );
}

function comparePrintJobTimestamp(left: string, right: string) {
  const milliseconds = Date.parse(left) - Date.parse(right);
  if (milliseconds) return milliseconds;
  // RFC3339Nano can distinguish mutations within one millisecond. Date.parse alone cannot.
  return printJobSubmillisecondFraction(left) - printJobSubmillisecondFraction(right);
}

function printJobSubmillisecondFraction(value: string) {
  return Number((value.match(/\.(\d+)(?:Z|[+-])/)?.[1] ?? "").padEnd(9, "0").slice(3, 9));
}

function buildAIReplyMessages(messages: ConversationMessage[]) {
  return messages.map((message) => ({
    role: message.role,
    content: message.text,
  })) as { role: "user" | "assistant"; content: string }[];
}

export const useWorkspaceStore = defineStore("workspace", () => {
  const pendingPrintOutcomes = new Map<string, string>();
  const pendingFeedbackOutcomes = new Map<string, string>();
  const persisted = readPersistedWorkspaceState();
  const persistedAuthSession = readPersistedAuthSession();

  const authUser = ref<User | null>(persistedAuthSession ? null : persisted.authUser);
  const authSession = ref<StoredAuthSession | null>(persistedAuthSession);
  const authLoading = ref(false);
  const authError = ref("");
  const authBootstrapping = ref(false);
  const passwordChangeLoading = ref(false);
  const feedbackSubmitting = ref(false);
  const feedbackError = ref("");
  const workspaceLoading = ref(false);
  const workspaceSyncing = ref(false);
  const workspaceSyncError = ref("");
  const workspaceConflict = ref(false);
  const recoveredWorkspaceDraft = ref<WorkspaceState | null>(null);
  const damagedWorkspaceDrafts = ref<{ key: string; raw: string }[]>([]);
  const workspaceDraftStorageError = ref("");
  const workspaceRevision = ref(0);
  const accountCreationLoading = ref(false);
  const accountCreationError = ref("");
  const aiConfigLoading = ref(false);
  const aiConfigSaving = ref(false);
  const aiConfigError = ref("");
  const aiConfigSaveError = ref("");
  const memoryConfigSaving = ref(false);
  const memoryConfigError = ref("");
  const memoryConfigSaveError = ref("");
  const deviceSyncError = ref("");
  const printSyncError = ref("");
  const printerSyncError = computed(() => deviceSyncError.value || printSyncError.value);
  const workspaceOwnerId = ref<string | null>(null);
  const workspaceHydrating = ref(false);

  const devices = ref<Device[]>(persisted.devices);
  const conversations = ref<Conversation[]>(persisted.conversations);
  const activeConversationId = ref(persisted.activeConversationId);
  const printJobs = ref<PrintJob[]>(persisted.printJobs);
  const remotePrintJobs = ref<PrintJobSummary[]>([]);
  const remotePrintCounts = ref<PrintJobCounts | null>(null);
  const remotePrintJobsReady = ref(false);
  const activePrintJobsCursor = ref<string | null>(null);
  const historyPrintJobsCursor = ref<string | null>(null);
  const printJobsLoading = ref(false);
  const activePrintJobsLoading = ref(false);
  const historyPrintJobsLoading = ref(false);
  const schedules = ref<Schedule[]>(persisted.schedules);
  const sources = ref<SourceConnection[]>(persisted.sources);

  const loginProtectionEnabled = ref(persisted.preferences.loginProtectionEnabled);
  const sendConfirmationEnabled = ref(persisted.preferences.sendConfirmationEnabled);
  const tutorialTabEnabled = ref(persisted.preferences.tutorialTabEnabled);
  const selectedTheme = ref<ThemeMode>(persisted.preferences.theme);
  const defaultDeviceId = ref(persisted.preferences.defaultDeviceId);
  const localePreference = ref<LocalePreference>(persisted.preferences.locale);
  const effectiveLocale = computed(() => resolveLocalePreference(localePreference.value));

  const serviceBinding = ref<ServiceBinding>(persisted.serviceBinding);
  const aiConfigSummary = ref<AIConfigSummary>({
    bound: persisted.serviceBinding.bound,
    providerName: persisted.serviceBinding.providerName ?? "OpenAI Compatible",
    providerType: "openai-compatible",
    baseUrl: "",
    model: persisted.serviceBinding.modelName,
    keyConfigured: false,
  });

  const memorySettings = ref<MemorySettings>({
    enabled: false,
    zepBaseUrl: "",
    keyConfigured: false,
    available: false,
  });

  const isGenerating = ref(false);
  const generationError = ref("");
  const selectedConversationMessageIds = ref<string[]>([]);
  const isCreatingPrint = ref(false);
  const printSubmissionUnknown = ref(false);
  const printOutcomeStorageError = ref("");
  const feedbackSubmissionUnknown = ref(false);
  const feedbackOutcomeStorageError = ref("");
  const flashMessage = ref("");
  const flashTone = ref<"success" | "error" | "info">("info");
  const postLoginTutorialOpen = ref(false);
  let flashTimer = 0;
  let remoteSaveTimer = 0;
  let remoteSavePromise: Promise<boolean> | null = null;
  let remoteSavePending = false;
  let remoteWorkspaceDirty = false;
  let recoveredDraftStorageKey: string | null = null;
  let recoveredDraftStoredValue: string | null = null;
  const draftTabId = createId("tab");
  let remotePrintStatusTimer = 0;
  let remotePrintStatusPromise: Promise<void> | null = null;
  let remotePrintStatusBackoffMs = REMOTE_PRINT_STATUS_POLL_MS;
  let disposed = false;
  let displayedPrintOutcomeMarker: string | null = null;
  let displayedFeedbackOutcomeMarker: string | null = null;
  const anonymousPrintCompletionTimers = new Set<number>();
  let remotePrintAccountVersion = 0;
  let remotePrintPageVersion = 0;
  let remotePrintMutationVersion = 0;
  let latestRemotePrintJobId: string | null = null;
  let remotePrintPagesStale = false;
  let printJobsRefreshPromise: Promise<void> | null = null;
  const printPagePromises = new Map<"active" | "history", Promise<void>>();
  const printDetailPromises = new Map<string, Promise<PrintJob | null>>();
  const removedRemotePrintJobIds = new Set<string>();

  let sessionRefresh: { accessToken: string; promise: Promise<string | null> } | null = null;

  function refreshCurrentAuthSession(accessToken: string): Promise<string | null> {
    if (sessionRefresh?.accessToken === accessToken) return sessionRefresh.promise;
    const promise = rotateAuthSession(accessToken).finally(() => {
      if (sessionRefresh?.promise === promise) sessionRefresh = null;
    });
    sessionRefresh = { accessToken, promise };
    return promise;
  }

  async function rotateAuthSession(accessToken: string) {
    if (disposed) return null;
    const current = authSession.value;
    if (!current || current.accessToken !== accessToken) {
      return null;
    }
    const shared =
      typeof window !== "undefined" &&
      !window.sessionStorage.getItem(AUTH_SESSION_STORAGE_KEY) &&
      !!window.localStorage.getItem(AUTH_SESSION_STORAGE_KEY);
    const userId = authUser.value?.id ?? current.userId ?? accessTokenUserId(current.accessToken);
    const rotate = async () => {
      if (disposed || authSession.value?.accessToken !== accessToken) return null;
      if (shared) {
        const stored = readAuthSessionFromStorage(window.localStorage);
        if (!stored) return null;
        if (stored.accessToken !== accessToken) {
          // Another tab may have rotated or switched accounts while we waited.
          if (!userId || stored.userId !== userId) return null;
          authSession.value = stored;
          return stored.accessToken;
        }
      }
      try {
        const refreshed = await refreshAuthSession(current.refreshToken);
        if (disposed || authSession.value?.accessToken !== accessToken) return null;
        if (shared && readAuthSessionFromStorage(window.localStorage)?.accessToken !== accessToken)
          return null;
        setAuthState(refreshed.user, refreshed.session);
        // Persist before releasing the cross-tab lock so the next tab reuses
        // the rotated token instead of submitting the consumed refresh token.
        writePersistedAuthSession(
          refreshed.session,
          !loginProtectionEnabled.value,
          refreshed.user.id,
        );
        return refreshed.session.accessToken;
      } catch (error) {
        if (disposed || authSession.value?.accessToken !== accessToken) return null;
        if (error instanceof AuthApiError && error.status === 401) {
          if (
            !shared ||
            readAuthSessionFromStorage(window.localStorage)?.accessToken === accessToken
          ) {
            clearAuthState();
            restoreAnonymousWorkspace();
          }
          return null;
        }
        throw error;
      }
    };
    if (shared && typeof navigator !== "undefined" && navigator.locks) {
      return navigator.locks.request("ink.auth.refresh", rotate);
    }
    return rotate();
  }

  configureAuthRefresh(refreshCurrentAuthSession);

  const deviceMap = computed(() =>
    devices.value.reduce<Record<string, Device>>((accumulator, device) => {
      accumulator[device.id] = device;
      return accumulator;
    }, {}),
  );

  const activeConversation = computed(
    () =>
      conversations.value.find((conversation) => conversation.id === activeConversationId.value) ??
      conversations.value[0] ??
      null,
  );
  const conversationMessages = computed(() => activeConversation.value?.messages ?? []);
  const selectedConversationMessages = computed(() =>
    conversationMessages.value.filter((message) =>
      selectedConversationMessageIds.value.includes(message.id),
    ),
  );
  const defaultDevice = computed(
    () => devices.value.find((device) => device.id === defaultDeviceId.value) ?? null,
  );
  const sortedPrintJobs = computed(() =>
    sortPrintJobsByUpdatedAt<PrintJobSummary & { content?: string }>(
      isAuthenticated.value ? remotePrintJobs.value : printJobs.value,
    ),
  );
  const pendingPrintJobs = computed(() =>
    sortedPrintJobs.value.filter((job) => job.status === "pending" || job.status === "queued"),
  );
  const recentPrintJobs = computed(() => sortedPrintJobs.value.slice(0, 5));
  const printHistoryJobs = computed(() =>
    isAuthenticated.value
      ? sortedPrintJobs.value.filter((job) => job.status !== "pending" && job.status !== "queued")
      : recentPrintJobs.value,
  );
  const connectedDevicesCount = computed(
    () => devices.value.filter((device) => device.status === "connected").length,
  );
  const pendingConfirmationCount = computed(() =>
    isAuthenticated.value
      ? (remotePrintCounts.value?.pending ?? 0)
      : printJobs.value.filter((job) => job.status === "pending").length,
  );
  const todayCompletedCount = computed(() => {
    if (isAuthenticated.value) return remotePrintCounts.value?.todayCompleted ?? 0;
    const today = new Date();

    return printJobs.value.filter(
      (job) =>
        job.status === "completed" &&
        new Date(job.updatedAt).toDateString() === today.toDateString(),
    ).length;
  });

  const activeDeviceLabel = computed(() => defaultDevice.value?.name ?? "");
  const activeModelLabel = computed(
    () => aiConfigSummary.value.model || serviceBinding.value.modelName,
  );
  const todayPrintCount = computed(() => todayCompletedCount.value);
  const welcomeLabel = computed(() => {
    void effectiveLocale.value;
    return translate("store.summary.welcomeLabel");
  });
  const isConfigured = computed(() => activeDeviceLabel.value !== "");
  const isAuthenticated = computed(() => authUser.value !== null && authSession.value !== null);
  const isAdmin = computed(() => authUser.value?.role === "admin");
  const shouldPollRemotePrintJobs = computed(
    () => isAuthenticated.value && remotePrintJobsReady.value,
  );

  const summaryCards = computed(() => {
    void effectiveLocale.value;
    return [
      {
        label: translate("store.summary.boundDevices"),
        value: translate("store.summary.deviceCount", { count: devices.value.length }),
      },
      {
        label: translate("store.summary.completedToday"),
        value: translate("store.summary.printCount", { count: todayCompletedCount.value }),
      },
    ];
  });

  const workspaceState = computed<WorkspaceState>(() => ({
    revision: workspaceRevision.value || undefined,
    devices: devices.value,
    conversations: conversations.value,
    activeConversationId: activeConversationId.value,
    printJobs: printJobs.value,
    schedules: schedules.value,
    sources: sources.value,
    preferences: {
      loginProtectionEnabled: loginProtectionEnabled.value,
      sendConfirmationEnabled: sendConfirmationEnabled.value,
      tutorialTabEnabled: tutorialTabEnabled.value,
      theme: selectedTheme.value,
      defaultDeviceId: defaultDeviceId.value,
      locale: localePreference.value,
    },
    serviceBinding: serviceBinding.value,
  }));

  const persistableState = computed<PersistedWorkspaceState>(() => ({
    authUser: loginProtectionEnabled.value || !authSession.value ? null : authUser.value,
    ...workspaceState.value,
  }));

  watch(
    effectiveLocale,
    (locale) => {
      setI18nLocale(locale);
    },
    { immediate: true },
  );

  watch(
    persistableState,
    (value) => {
      if (typeof window === "undefined" || authSession.value) {
        return;
      }

      try {
        window.localStorage.setItem(STORAGE_KEY, JSON.stringify(value));
      } catch {
        workspaceDraftStorageError.value = translate("store.errors.localDraftStorage");
      }
    },
    { deep: true, immediate: true },
  );

  watch(
    [authSession, loginProtectionEnabled, () => authUser.value?.id],
    ([session, loginProtection]) => {
      try {
        writePersistedAuthSession(session, !loginProtection, authUser.value?.id ?? session?.userId);
      } catch {
        workspaceDraftStorageError.value = translate("store.errors.localDraftStorage");
      }
    },
    { deep: true, immediate: true },
  );

  function isCurrentAccount(accountVersion: number) {
    return !disposed && accountVersion === remotePrintAccountVersion;
  }

  function accountDraftKey(userId: string) {
    return `${accountDraftPrefix(userId)}${draftTabId}`;
  }

  function loadPrintOutcome() {
    const userId = authUser.value?.id;
    printSubmissionUnknown.value = false;
    printOutcomeStorageError.value = "";
    displayedPrintOutcomeMarker = null;
    if (!userId || !authSession.value) return;
    const key = `${PRINT_OUTCOME_STORAGE_PREFIX}${encodeURIComponent(userId)}`;
    const result = readOutcomeMarkers(key, pendingPrintOutcomes.get(userId));
    displayedPrintOutcomeMarker = result.signature;
    printSubmissionUnknown.value = result.failed || result.markers.length > 0;
    if (result.failed || pendingPrintOutcomes.has(userId))
      printOutcomeStorageError.value = translate("store.errors.printOutcomeStorage");
  }

  function markPrintOutcomeUnknown(userId: string | undefined) {
    if (!userId) return;
    const marker = createId("outcome");
    pendingPrintOutcomes.set(userId, marker);
    const key = `${PRINT_OUTCOME_STORAGE_PREFIX}${encodeURIComponent(userId)}`;
    try {
      window.localStorage.setItem(key, marker);
      pendingPrintOutcomes.delete(userId);
      window.sessionStorage.removeItem(key);
    } catch {
      try {
        window.sessionStorage.setItem(key, marker);
      } catch {
        /* Keep the current-session latch. */
      }
    }
    if (!disposed && authUser.value?.id === userId && authSession.value) loadPrintOutcome();
  }

  function acknowledgePrintOutcome() {
    const userId = authUser.value?.id;
    if (!userId || !authSession.value) return false;
    const key = `${PRINT_OUTCOME_STORAGE_PREFIX}${encodeURIComponent(userId)}`;
    try {
      const latest = readOutcomeMarkers(key, pendingPrintOutcomes.get(userId));
      if (latest.failed || latest.signature !== displayedPrintOutcomeMarker) {
        loadPrintOutcome();
        return false;
      }
      window.localStorage.removeItem(key);
      window.sessionStorage.removeItem(key);
      pendingPrintOutcomes.delete(userId);
      displayedPrintOutcomeMarker = null;
      printSubmissionUnknown.value = false;
      printOutcomeStorageError.value = "";
      return true;
    } catch {
      printOutcomeStorageError.value = translate("store.errors.printOutcomeStorage");
      return false;
    }
  }

  function loadFeedbackOutcome() {
    const userId = authUser.value?.id;
    feedbackSubmissionUnknown.value = false;
    feedbackOutcomeStorageError.value = "";
    displayedFeedbackOutcomeMarker = null;
    if (!userId || !authSession.value) return;
    const key = `${FEEDBACK_OUTCOME_STORAGE_PREFIX}${encodeURIComponent(userId)}`;
    const result = readOutcomeMarkers(key, pendingFeedbackOutcomes.get(userId));
    displayedFeedbackOutcomeMarker = result.signature;
    feedbackSubmissionUnknown.value = result.failed || result.markers.length > 0;
    if (result.failed || pendingFeedbackOutcomes.has(userId))
      feedbackOutcomeStorageError.value = translate("store.errors.feedbackOutcomeStorage");
  }

  function markFeedbackOutcomeUnknown(userId: string | undefined) {
    if (!userId) return;
    const marker = createId("outcome");
    pendingFeedbackOutcomes.set(userId, marker);
    const key = `${FEEDBACK_OUTCOME_STORAGE_PREFIX}${encodeURIComponent(userId)}`;
    try {
      window.localStorage.setItem(key, marker);
      pendingFeedbackOutcomes.delete(userId);
      window.sessionStorage.removeItem(key);
    } catch {
      try {
        window.sessionStorage.setItem(key, marker);
      } catch {
        /* Keep the current-session latch. */
      }
    }
    if (!disposed && authUser.value?.id === userId && authSession.value) loadFeedbackOutcome();
  }

  function acknowledgeFeedbackOutcome() {
    const userId = authUser.value?.id;
    if (!userId || !authSession.value) return false;
    const key = `${FEEDBACK_OUTCOME_STORAGE_PREFIX}${encodeURIComponent(userId)}`;
    try {
      const latest = readOutcomeMarkers(key, pendingFeedbackOutcomes.get(userId));
      if (latest.failed || latest.signature !== displayedFeedbackOutcomeMarker) {
        loadFeedbackOutcome();
        return false;
      }
      window.localStorage.removeItem(key);
      window.sessionStorage.removeItem(key);
      pendingFeedbackOutcomes.delete(userId);
      displayedFeedbackOutcomeMarker = null;
      feedbackSubmissionUnknown.value = false;
      feedbackOutcomeStorageError.value = "";
      return true;
    } catch {
      feedbackOutcomeStorageError.value = translate("store.errors.feedbackOutcomeStorage");
      return false;
    }
  }

  function preservePendingWorkspaceDraft() {
    const userId = authUser.value?.id;
    if (!userId || workspaceOwnerId.value !== userId || !remoteWorkspaceDirty) return true;
    try {
      window.localStorage.setItem(accountDraftKey(userId), JSON.stringify(workspaceState.value));
      workspaceDraftStorageError.value = "";
      return true;
    } catch {
      workspaceDraftStorageError.value = translate("store.errors.localDraftStorage");
      return false;
    }
  }

  function loadRecoveredWorkspaceDraft(userId: string, excludeCurrentTab = false) {
    recoveredWorkspaceDraft.value = null;
    damagedWorkspaceDrafts.value = [];
    recoveredDraftStorageKey = null;
    recoveredDraftStoredValue = null;
    try {
      const prefix = accountDraftPrefix(userId);
      const keys = Array.from({ length: window.localStorage.length }, (_, index) =>
        window.localStorage.key(index),
      );
      for (const key of keys) {
        if (!key?.startsWith(prefix) || (excludeCurrentTab && key === accountDraftKey(userId)))
          continue;
        const raw = window.localStorage.getItem(key);
        if (!raw) continue;
        try {
          const parsed: unknown = JSON.parse(raw);
          if (!isRecoverableDraft(parsed)) throw new Error("Invalid draft shape");
          if (!recoveredWorkspaceDraft.value) {
            recoveredWorkspaceDraft.value = normalizeWorkspaceState(parsed);
            recoveredDraftStorageKey = key;
            recoveredDraftStoredValue = raw;
          }
        } catch {
          damagedWorkspaceDrafts.value.push({ key, raw });
        }
      }
    } catch {
      workspaceDraftStorageError.value = translate("store.errors.localDraftStorage");
    }
  }

  function downloadDamagedWorkspaceDraft(key: string) {
    if (authUser.value?.id !== workspaceOwnerId.value) return;
    const draft = damagedWorkspaceDrafts.value.find((item) => item.key === key);
    if (draft) downloadDraftText(draft.raw);
  }

  function discardDamagedWorkspaceDraft(key: string) {
    const userId = authUser.value?.id;
    const draft = damagedWorkspaceDrafts.value.find((item) => item.key === key);
    if (!userId || workspaceOwnerId.value !== userId || !draft) return false;
    try {
      if (window.localStorage.getItem(key) === draft.raw) window.localStorage.removeItem(key);
      loadRecoveredWorkspaceDraft(userId, true);
      return true;
    } catch {
      workspaceDraftStorageError.value = translate("store.errors.localDraftStorage");
      return false;
    }
  }

  function downloadRecoveredWorkspaceDraft() {
    if (recoveredWorkspaceDraft.value && authUser.value?.id === workspaceOwnerId.value) {
      downloadState(recoveredWorkspaceDraft.value);
    }
  }

  function discardRecoveredWorkspaceDraft() {
    const userId = authUser.value?.id;
    if (!userId || workspaceOwnerId.value !== userId) return false;
    try {
      if (
        recoveredDraftStorageKey &&
        window.localStorage.getItem(recoveredDraftStorageKey) === recoveredDraftStoredValue
      )
        window.localStorage.removeItem(recoveredDraftStorageKey);
      loadRecoveredWorkspaceDraft(userId, true);
      workspaceDraftStorageError.value = "";
      return true;
    } catch {
      workspaceDraftStorageError.value = translate("store.errors.localDraftStorage");
      return false;
    }
  }

  function connectedDevice(id: string) {
    return devices.value.some((device) => device.id === id && device.status === "connected");
  }

  function restoreRecoveredWorkspaceDraft() {
    const recovered = recoveredWorkspaceDraft.value;
    if (
      !recovered ||
      workspaceLoading.value ||
      workspaceSyncing.value ||
      isGenerating.value ||
      !authUser.value ||
      workspaceOwnerId.value !== authUser.value.id
    )
      return false;
    // Integration records remain authoritative; recovery restores the user's
    // conversations and preferences against the latest loaded revision.
    const recoveredDefault = recovered.preferences.defaultDeviceId;
    const liveDefault = connectedDevice(recoveredDefault)
      ? recoveredDefault
      : connectedDevice(defaultDeviceId.value)
        ? defaultDeviceId.value
        : "";
    applyWorkspaceState({
      ...workspaceState.value,
      conversations: recovered.conversations,
      activeConversationId: recovered.activeConversationId,
      preferences: { ...recovered.preferences, defaultDeviceId: liveDefault },
    });
    const recoveredKey = recoveredDraftStorageKey;
    const recoveredValue = recoveredDraftStoredValue;
    recoveredWorkspaceDraft.value = null;
    recoveredDraftStorageKey = null;
    recoveredDraftStoredValue = null;
    scheduleRemoteWorkspaceSave();
    if (!workspaceDraftStorageError.value) {
      try {
        if (
          recoveredKey &&
          recoveredKey !== accountDraftKey(authUser.value.id) &&
          window.localStorage.getItem(recoveredKey) === recoveredValue
        )
          window.localStorage.removeItem(recoveredKey);
        loadRecoveredWorkspaceDraft(authUser.value.id, true);
      } catch {
        workspaceDraftStorageError.value = translate("store.errors.localDraftStorage");
      }
    }
    return true;
  }

  function scheduleRemoteWorkspaceSave() {
    if (
      disposed ||
      typeof window === "undefined" ||
      !authSession.value ||
      !authUser.value ||
      workspaceOwnerId.value !== authUser.value.id ||
      workspaceHydrating.value
    ) {
      return;
    }
    remoteWorkspaceDirty = true;
    preservePendingWorkspaceDraft();
    if (workspaceConflict.value) return;
    if (remoteSaveTimer) window.clearTimeout(remoteSaveTimer);
    remoteSaveTimer = window.setTimeout(() => {
      remoteSaveTimer = 0;
      void persistRemoteWorkspace();
    }, REMOTE_SAVE_DEBOUNCE_MS);
  }

  watch(
    [() => authUser.value?.id, () => !!authSession.value],
    () => {
      remotePrintAccountVersion++;
      remoteSavePromise = null;
      remoteSavePending = false;
      remoteWorkspaceDirty = false;
      recoveredWorkspaceDraft.value = null;
      damagedWorkspaceDrafts.value = [];
      workspaceDraftStorageError.value = "";
      isGenerating.value = false;
      isCreatingPrint.value = false;
      loadPrintOutcome();
      loadFeedbackOutcome();
      passwordChangeLoading.value = false;
      feedbackSubmitting.value = false;
      accountCreationLoading.value = false;
      aiConfigSaving.value = false;
      workspaceSyncing.value = false;
      memoryConfigSaving.value = false;
      memoryConfigError.value = "";
      memoryConfigSaveError.value = "";
      memorySettings.value = {
        enabled: false,
        zepBaseUrl: "",
        keyConfigured: false,
        available: false,
      };
      workspaceConflict.value = false;
      workspaceLoading.value = false;
      workspaceHydrating.value = false;
      remotePrintPageVersion++;
      remotePrintMutationVersion++;
      clearRemotePrintStatusSync();
      remotePrintStatusPromise = null;
      printJobsRefreshPromise = null;
      printPagePromises.clear();
      printDetailPromises.clear();
      removedRemotePrintJobIds.clear();
      remotePrintJobs.value = [];
      remotePrintCounts.value = null;
      remotePrintJobsReady.value = false;
      activePrintJobsCursor.value = null;
      historyPrintJobsCursor.value = null;
      latestRemotePrintJobId = null;
      remotePrintPagesStale = false;
      printJobsLoading.value = false;
      activePrintJobsLoading.value = false;
      historyPrintJobsLoading.value = false;
      remotePrintStatusBackoffMs = REMOTE_PRINT_STATUS_POLL_MS;
    },
    { flush: "sync" },
  );

  watch(
    shouldPollRemotePrintJobs,
    (shouldPoll) => {
      if (!shouldPoll) {
        clearRemotePrintStatusSync();
        return;
      }

      scheduleRemotePrintStatusSync(true);
    },
    { immediate: true },
  );

  if (typeof window !== "undefined") {
    const handlePrintVisibilityChange = () => {
      if (document.visibilityState === "hidden") {
        clearRemotePrintStatusSync();
        return;
      }
      if (shouldPollRemotePrintJobs.value) {
        scheduleRemotePrintStatusSync(true);
      }
    };
    document.addEventListener("visibilitychange", handlePrintVisibilityChange);
    onScopeDispose(() => {
      document.removeEventListener("visibilitychange", handlePrintVisibilityChange);
    });
  }

  onScopeDispose(() => {
    disposed = true;
    remotePrintAccountVersion++;
    clearRemotePrintStatusSync();
    if (typeof window !== "undefined") {
      window.clearTimeout(remoteSaveTimer);
      window.clearTimeout(flashTimer);
      for (const timer of anonymousPrintCompletionTimers) window.clearTimeout(timer);
    }
    remoteSaveTimer = 0;
    flashTimer = 0;
    remoteSavePending = false;
    anonymousPrintCompletionTimers.clear();
  });

  function applyWorkspaceState(nextState: WorkspaceState) {
    const normalized = normalizeWorkspaceState(nextState);
    workspaceRevision.value = normalized.revision ?? 0;
    devices.value = normalized.devices;
    conversations.value = normalized.conversations;
    activeConversationId.value = normalized.activeConversationId;
    printJobs.value = normalized.printJobs;
    schedules.value = normalized.schedules;
    sources.value = normalized.sources;
    loginProtectionEnabled.value = normalized.preferences.loginProtectionEnabled;
    sendConfirmationEnabled.value = normalized.preferences.sendConfirmationEnabled;
    tutorialTabEnabled.value = normalized.preferences.tutorialTabEnabled;
    selectedTheme.value = normalized.preferences.theme;
    defaultDeviceId.value = normalized.preferences.defaultDeviceId;
    localePreference.value = normalized.preferences.locale;
    serviceBinding.value = normalized.serviceBinding;
    selectedConversationMessageIds.value = [];
    generationError.value = "";
  }

  function applyAIConfig(nextConfig: AIConfigSummary) {
    aiConfigSummary.value = nextConfig;
    serviceBinding.value = {
      providerName: nextConfig.bound ? nextConfig.providerName : null,
      modelName: nextConfig.model || "Ink AI",
      bound: nextConfig.bound,
    };
    aiConfigError.value = "";
    aiConfigSaveError.value = "";
    scheduleRemoteWorkspaceSave();
  }

  function upsertPrintJob(nextJob: PrintJob) {
    remotePrintMutationVersion++;
    mergeRemotePrintJobs([nextJob]);
    remotePrintJobsReady.value = true;
    scheduleRemotePrintStatusSync(true);
  }

  function mergeRemotePrintJobs(nextJobs: PrintJobSummary[], preserveExisting = false) {
    const jobs = new Map(remotePrintJobs.value.map((job) => [job.id, job]));
    for (const nextJob of nextJobs) {
      if (removedRemotePrintJobIds.has(nextJob.id)) continue;
      const previous = jobs.get(nextJob.id);
      if (previous && preserveExisting) continue;
      if (!previous || comparePrintJobTimestamp(nextJob.updatedAt, previous.updatedAt) >= 0) {
        const { id, title, source, deviceId, status, createdAt, updatedAt, errorMessage } = nextJob;
        jobs.set(id, { id, title, source, deviceId, status, createdAt, updatedAt, errorMessage });
      }
    }
    remotePrintJobs.value = [...jobs.values()];
  }

  async function refreshRemotePrintJobs() {
    if (disposed) return;
    if (printJobsRefreshPromise) return printJobsRefreshPromise;
    const session = authSession.value;
    if (!session || !isAuthenticated.value) return;
    const accountVersion = remotePrintAccountVersion;
    const mutationVersion = remotePrintMutationVersion;
    remotePrintPageVersion++;
    printJobsLoading.value = true;
    let promise!: Promise<void>;
    promise = Promise.resolve().then(async () => {
      try {
        const midnight = new Date();
        midnight.setHours(0, 0, 0, 0);
        const [active, history, metadata] = await Promise.all([
          fetchPrintJobs(session.accessToken, { status: "active" }),
          fetchPrintJobs(session.accessToken, { status: "history" }),
          remotePrintCounts.value
            ? Promise.resolve(null)
            : fetchPrintJobStatuses(session.accessToken, [], midnight.toISOString()),
        ]);
        if (!isCurrentAccount(accountVersion)) return;
        mergeRemotePrintJobs(
          [...active.printJobs, ...history.printJobs],
          mutationVersion !== remotePrintMutationVersion,
        );
        // Restart cursors after membership changes. Existing rows are retained and deduplicated,
        // so a formerly active job cannot be skipped when it enters an already paged history.
        activePrintJobsCursor.value = active.nextCursor;
        historyPrintJobsCursor.value = history.nextCursor;
        if (metadata?.counts && mutationVersion === remotePrintMutationVersion) {
          remotePrintCounts.value = metadata.counts;
          latestRemotePrintJobId = metadata.latestJobId;
        }
        remotePrintJobsReady.value = true;
        remotePrintPagesStale = false;
        printSyncError.value = "";
      } catch (error) {
        if (isCurrentAccount(accountVersion)) {
          remotePrintPagesStale = true;
          printSyncError.value = getErrorMessage(error, "store.errors.loadIntegrations");
        }
        throw error;
      } finally {
        if (printJobsRefreshPromise === promise) {
          printJobsRefreshPromise = null;
          printJobsLoading.value = false;
        }
      }
    });
    printJobsRefreshPromise = promise;
    return promise;
  }

  async function loadMorePrintJobs(status: "active" | "history") {
    if (disposed) return;
    const expectedAccountVersion = remotePrintAccountVersion;
    if (printJobsRefreshPromise) {
      try {
        await printJobsRefreshPromise;
      } catch {
        return;
      }
    }
    if (expectedAccountVersion !== remotePrintAccountVersion) return;
    if (printPagePromises.has(status)) return printPagePromises.get(status);
    const cursor = status === "active" ? activePrintJobsCursor : historyPrintJobsCursor;
    const loading = status === "active" ? activePrintJobsLoading : historyPrintJobsLoading;
    const session = authSession.value;
    if (!session || !isAuthenticated.value || !cursor.value) return;
    const nextCursor = cursor.value;
    const accountVersion = remotePrintAccountVersion;
    const pageVersion = remotePrintPageVersion;
    const mutationVersion = remotePrintMutationVersion;
    loading.value = true;
    let promise!: Promise<void>;
    promise = Promise.resolve().then(async () => {
      try {
        const page = await fetchPrintJobs(session.accessToken, { status, cursor: nextCursor });
        if (!isCurrentAccount(accountVersion)) return;
        mergeRemotePrintJobs(page.printJobs, mutationVersion !== remotePrintMutationVersion);
        if (pageVersion === remotePrintPageVersion) cursor.value = page.nextCursor;
        printSyncError.value = "";
      } catch (error) {
        if (isCurrentAccount(accountVersion)) {
          printSyncError.value = getErrorMessage(error, "store.errors.loadIntegrations");
        }
      } finally {
        if (printPagePromises.get(status) === promise) {
          printPagePromises.delete(status);
          loading.value = false;
        }
      }
    });
    printPagePromises.set(status, promise);
    return promise;
  }

  async function loadPrintJobDetail(jobId: string) {
    if (disposed) return null;
    if (!isAuthenticated.value) return printJobs.value.find((job) => job.id === jobId) ?? null;
    if (printDetailPromises.has(jobId)) return printDetailPromises.get(jobId)!;
    const session = authSession.value;
    if (!session) return null;
    const accountVersion = remotePrintAccountVersion;
    const promise = fetchPrintJob(session.accessToken, jobId)
      .then((job) => (accountVersion === remotePrintAccountVersion ? job : null))
      .catch((error: unknown) => {
        if (
          accountVersion === remotePrintAccountVersion &&
          error instanceof AuthApiError &&
          error.status === 404
        ) {
          removedRemotePrintJobIds.add(jobId);
          remotePrintJobs.value = remotePrintJobs.value.filter((job) => job.id !== jobId);
        }
        throw error;
      })
      .finally(() => {
        if (printDetailPromises.get(jobId) === promise) printDetailPromises.delete(jobId);
      });
    printDetailPromises.set(jobId, promise);
    return promise;
  }

  function upsertDevice(nextDevice: Device) {
    devices.value = [nextDevice, ...devices.value.filter((device) => device.id !== nextDevice.id)];
    scheduleRemoteWorkspaceSave();
  }

  function clearRemotePrintStatusSync() {
    if (typeof window === "undefined") {
      return;
    }

    if (remotePrintStatusTimer) {
      window.clearTimeout(remotePrintStatusTimer);
      remotePrintStatusTimer = 0;
    }
  }

  function scheduleRemotePrintStatusSync(immediate = false) {
    if (disposed || typeof window === "undefined") {
      return;
    }

    clearRemotePrintStatusSync();

    if (!shouldPollRemotePrintJobs.value) {
      return;
    }

    if (document.visibilityState === "hidden") {
      return;
    }

    remotePrintStatusTimer = window.setTimeout(
      () => {
        remotePrintStatusTimer = 0;
        void syncRemotePrintStatus();
      },
      immediate ? REMOTE_PRINT_STATUS_INITIAL_POLL_MS : remotePrintStatusBackoffMs,
    );
  }

  async function syncRemotePrintStatus() {
    if (disposed) return;
    if (remotePrintStatusPromise) return remotePrintStatusPromise;
    const currentSession = authSession.value;
    if (!currentSession || !shouldPollRemotePrintJobs.value) {
      clearRemotePrintStatusSync();
      return;
    }
    const accountVersion = remotePrintAccountVersion;
    const mutationVersion = remotePrintMutationVersion;
    let promise!: Promise<void>;
    promise = Promise.resolve().then(async () => {
      let statusesLoaded = false;
      try {
        const midnight = new Date();
        midnight.setHours(0, 0, 0, 0);
        const ids = remotePrintJobs.value
          .filter((job) => job.status === "pending" || job.status === "queued")
          .map((job) => job.id);
        // Keep each status response bounded even after the user loads many active pages.
        const chunks = Array.from({ length: Math.max(1, Math.ceil(ids.length / 100)) }, (_, i) =>
          ids.slice(i * 100, (i + 1) * 100),
        );
        let metadata: Awaited<ReturnType<typeof fetchPrintJobStatuses>> | null = null;
        for (const [index, chunk] of chunks.entries()) {
          const session = authSession.value;
          if (!session || accountVersion !== remotePrintAccountVersion) return;
          const response = await fetchPrintJobStatuses(
            session.accessToken,
            chunk,
            midnight.toISOString(),
            index === 0,
          );
          if (!isCurrentAccount(accountVersion)) return;
          if (response.counts) metadata = response;
          const updates = new Map(response.printJobs.map((job) => [job.id, job]));
          if (mutationVersion === remotePrintMutationVersion) {
            for (const id of chunk) {
              if (!updates.has(id)) removedRemotePrintJobIds.add(id);
            }
            remotePrintJobs.value = remotePrintJobs.value.filter(
              (job) => !removedRemotePrintJobIds.has(job.id),
            );
          }
          mergeRemotePrintJobs(
            remotePrintJobs.value.map((job) => {
              const update = updates.get(job.id);
              return update ? { ...job, ...update } : job;
            }),
            mutationVersion !== remotePrintMutationVersion,
          );
        }
        statusesLoaded = true;
        if (metadata?.counts && mutationVersion === remotePrintMutationVersion) {
          const changed =
            remotePrintPagesStale ||
            latestRemotePrintJobId !== metadata.latestJobId ||
            JSON.stringify(remotePrintCounts.value) !== JSON.stringify(metadata.counts);
          // Keep healthy status metadata even if a subsequent page fetch fails.
          remotePrintCounts.value = metadata.counts;
          latestRemotePrintJobId = metadata.latestJobId;
          if (changed) await refreshRemotePrintJobs();
          if (!isCurrentAccount(accountVersion)) return;
          if (mutationVersion === remotePrintMutationVersion) {
            remotePrintCounts.value = metadata.counts;
            latestRemotePrintJobId = metadata.latestJobId;
          }
        }
        printSyncError.value = "";
        remotePrintStatusBackoffMs =
          (remotePrintCounts.value?.queued ?? 0) > 0
            ? REMOTE_PRINT_STATUS_POLL_MS
            : REMOTE_PRINT_STATUS_IDLE_POLL_MS;
      } catch (error) {
        if (!isCurrentAccount(accountVersion)) return;
        printSyncError.value = getErrorMessage(error, "store.errors.syncPrintStatus");
        remotePrintStatusBackoffMs = statusesLoaded
          ? (remotePrintCounts.value?.queued ?? 0) > 0
            ? REMOTE_PRINT_STATUS_POLL_MS
            : REMOTE_PRINT_STATUS_IDLE_POLL_MS
          : Math.min(
              Math.max(remotePrintStatusBackoffMs * 2, REMOTE_PRINT_STATUS_POLL_MS),
              REMOTE_PRINT_STATUS_MAX_BACKOFF_MS,
            );
      } finally {
        if (remotePrintStatusPromise === promise) {
          remotePrintStatusPromise = null;
          if (shouldPollRemotePrintJobs.value) scheduleRemotePrintStatusSync();
        }
      }
    });
    remotePrintStatusPromise = promise;
    return promise;
  }

  function restoreAnonymousWorkspace() {
    workspaceOwnerId.value = null;
    clearRemotePrintStatusSync();
    workspaceHydrating.value = true;
    applyWorkspaceState(readPersistedWorkspaceState());
    applyAIConfig({
      bound: false,
      providerName: "OpenAI Compatible",
      providerType: "openai-compatible",
      baseUrl: "",
      model: "Ink AI",
      keyConfigured: false,
    });
    workspaceHydrating.value = false;
    workspaceSyncError.value = "";
    workspaceConflict.value = false;
    deviceSyncError.value = "";
    printSyncError.value = "";
  }

  async function persistRemoteWorkspace() {
    if (disposed) return true;
    if (workspaceConflict.value) return false;
    if (remoteSavePromise) {
      remoteSavePending = true;
      return remoteSavePromise;
    }

    const currentSession = authSession.value;
    const currentUser = authUser.value;
    const accountVersion = remotePrintAccountVersion;

    if (!currentSession || !currentUser || workspaceOwnerId.value !== currentUser.id) {
      return true;
    }

    workspaceSyncing.value = true;
    workspaceSyncError.value = "";
    let promise!: Promise<boolean>;
    promise = (async () => {
      try {
        do {
          remoteSavePending = false;
          const saveSession = authSession.value;
          if (
            disposed ||
            accountVersion !== remotePrintAccountVersion ||
            !saveSession ||
            authUser.value?.id !== currentUser.id ||
            workspaceOwnerId.value !== currentUser.id
          ) {
            return true;
          }
          const snapshot = JSON.parse(JSON.stringify(workspaceState.value)) as WorkspaceState;
          let saved: WorkspaceState;
          try {
            saved = await saveWorkspaceStateWithApi(saveSession.accessToken, snapshot);
          } catch (error) {
            const latestSession = authSession.value;
            if (
              !(error instanceof AuthApiError) ||
              error.status !== 401 ||
              !latestSession ||
              latestSession.accessToken === saveSession.accessToken ||
              disposed ||
              accountVersion !== remotePrintAccountVersion ||
              authUser.value?.id !== currentUser.id ||
              workspaceOwnerId.value !== currentUser.id
            )
              throw error;
            // Retry the unchanged revision only after this same account's
            // token was rotated by another request.
            saved = await saveWorkspaceStateWithApi(latestSession.accessToken, snapshot);
          }
          if (!isCurrentAccount(accountVersion)) return false;
          workspaceRevision.value = saved.revision ?? workspaceRevision.value;
          if (draftContent(snapshot) === draftContent(workspaceState.value)) {
            remoteWorkspaceDirty = false;
            try {
              window.localStorage.removeItem(accountDraftKey(currentUser.id));
              workspaceDraftStorageError.value = "";
            } catch {
              workspaceDraftStorageError.value = translate("store.errors.localDraftStorage");
            }
          } else {
            preservePendingWorkspaceDraft();
            remoteSavePending = true;
          }
        } while (
          remoteSavePending &&
          authUser.value?.id === currentUser.id &&
          workspaceOwnerId.value === currentUser.id
        );
        return true;
      } catch (error) {
        if (!isCurrentAccount(accountVersion)) return false;
        workspaceConflict.value =
          error instanceof AuthApiError &&
          (error.code === "workspace_conflict" || error.code === "workspace_revision_required");
        remoteSavePending = false;
        workspaceSyncError.value = getErrorMessage(error, "store.errors.syncWorkspace");
        return false;
      } finally {
        if (remoteSavePromise === promise) {
          workspaceSyncing.value = false;
          remoteSavePromise = null;
        }
      }
    })();
    remoteSavePromise = promise;

    return promise;
  }

  async function flushRemoteWorkspaceSave() {
    if (remoteSaveTimer) {
      window.clearTimeout(remoteSaveTimer);
      remoteSaveTimer = 0;
      return persistRemoteWorkspace();
    }

    return remoteSavePromise ?? (remoteWorkspaceDirty ? persistRemoteWorkspace() : true);
  }

  async function loadRemoteWorkspace(recoverDraft = true) {
    if (disposed) return false;
    const currentSession = authSession.value;
    const currentUser = authUser.value;

    if (!currentSession || !currentUser) {
      return false;
    }
    const accountVersion = remotePrintAccountVersion;

    workspaceLoading.value = true;
    workspaceSyncError.value = "";
    workspaceHydrating.value = true;

    try {
      const state = await fetchWorkspaceStateWithApi(currentSession.accessToken);
      if (!isCurrentAccount(accountVersion)) return false;
      applyWorkspaceState(state);
      await loadLiveIntegrations(currentSession.accessToken);
      if (!isCurrentAccount(accountVersion)) return false;
      workspaceOwnerId.value = currentUser.id;
      if (recoverDraft) {
        loadRecoveredWorkspaceDraft(currentUser.id);
      } else {
        remoteWorkspaceDirty = false;
        try {
          window.localStorage.removeItem(accountDraftKey(currentUser.id));
          if (
            recoveredDraftStorageKey &&
            window.localStorage.getItem(recoveredDraftStorageKey) === recoveredDraftStoredValue
          )
            window.localStorage.removeItem(recoveredDraftStorageKey);
          recoveredDraftStorageKey = null;
          recoveredWorkspaceDraft.value = null;
        } catch {
          workspaceDraftStorageError.value = translate("store.errors.localDraftStorage");
        }
      }
      workspaceConflict.value = false;
      return true;
    } catch (error) {
      if (!isCurrentAccount(accountVersion)) return false;
      workspaceSyncError.value = getErrorMessage(error, "store.errors.loadAccountData");
      return false;
    } finally {
      if (isCurrentAccount(accountVersion)) {
        workspaceHydrating.value = false;
        workspaceLoading.value = false;
      }
    }
  }

  async function loadLiveIntegrations(accessToken: string) {
    if (disposed) return;
    aiConfigLoading.value = true;
    aiConfigError.value = "";
    const accountVersion = remotePrintAccountVersion;

    try {
      const [aiSummary, memorySummary, printerResponse, printsResponse] = await Promise.allSettled([
        fetchAIConfigSummary(accessToken),
        getMemorySettings(accessToken),
        fetchPrinters(accessToken),
        refreshRemotePrintJobs(),
      ]);

      if (!isCurrentAccount(accountVersion)) return;
      if (aiSummary.status === "fulfilled") {
        if (!aiConfigSaving.value && !aiConfigSaveError.value) applyAIConfig(aiSummary.value);
      } else {
        aiConfigError.value = getErrorMessage(aiSummary.reason, "store.errors.loadIntegrations");
      }
      if (memorySummary.status === "fulfilled") {
        memorySettings.value = memorySummary.value;
      }
      // Memory config failure is non-critical, don't show error
      if (printerResponse.status === "fulfilled") {
        deviceSyncError.value = "";
        devices.value = printerResponse.value.devices.filter(
          (device) => device.status !== "offline",
        );
      } else {
        deviceSyncError.value = getErrorMessage(
          printerResponse.reason,
          "store.errors.loadIntegrations",
        );
      }
      if (printsResponse.status === "rejected") {
        printSyncError.value = getErrorMessage(
          printsResponse.reason,
          "store.errors.loadIntegrations",
        );
      }
    } finally {
      if (isCurrentAccount(accountVersion)) {
        aiConfigLoading.value = false;
      }
    }
  }

  async function retrySynchronization() {
    const accountVersion = remotePrintAccountVersion;
    if (!isAuthenticated.value || workspaceLoading.value || workspaceSyncing.value) return false;
    if (workspaceSyncError.value && !workspaceConflict.value) await persistRemoteWorkspace();
    if (!isCurrentAccount(accountVersion)) return false;
    const session = authSession.value;
    if (session && (printerSyncError.value || aiConfigError.value)) {
      await loadLiveIntegrations(session.accessToken);
      if (!isCurrentAccount(accountVersion)) return false;
    }
    return !workspaceSyncError.value && !printerSyncError.value && !aiConfigError.value;
  }

  async function reloadConflictedWorkspace() {
    if (
      workspaceLoading.value ||
      recoveredWorkspaceDraft.value ||
      isGenerating.value ||
      isCreatingPrint.value
    )
      return false;
    if (remoteSaveTimer) {
      window.clearTimeout(remoteSaveTimer);
      remoteSaveTimer = 0;
    }
    return loadRemoteWorkspace(false);
  }

  function downloadWorkspaceDraft() {
    downloadState(workspaceState.value);
  }

  function showFlash(message: string, tone: "success" | "error" | "info" = "info") {
    if (disposed) return;
    flashMessage.value = message;
    flashTone.value = tone;

    if (flashTimer) {
      window.clearTimeout(flashTimer);
    }

    if (typeof window !== "undefined") {
      flashTimer = window.setTimeout(() => {
        flashMessage.value = "";
      }, 2600);
    }
  }

  function dismissFlash() {
    if (flashTimer) {
      window.clearTimeout(flashTimer);
      flashTimer = 0;
    }
    flashMessage.value = "";
  }

  function showFlashKey(
    key: string,
    tone: "success" | "error" | "info" = "info",
    values?: Record<string, unknown>,
  ) {
    showFlash(translate(key, values), tone);
  }

  function updateConversation(
    conversationId: string,
    updater: (conversation: Conversation) => Conversation,
  ) {
    conversations.value = conversations.value.map((conversation) =>
      conversation.id === conversationId ? updater(conversation) : conversation,
    );
    scheduleRemoteWorkspaceSave();
  }

  function touchConversation(conversationId: string) {
    const conversation = conversations.value.find((item) => item.id === conversationId);

    if (!conversation) {
      return;
    }

    conversations.value = [
      {
        ...conversation,
        updatedAt: getNow(),
      },
      ...conversations.value.filter((item) => item.id !== conversationId),
    ];
    scheduleRemoteWorkspaceSave();
  }

  function selectConversation(conversationId: string) {
    if (workspaceLoading.value || recoveredWorkspaceDraft.value) return;
    activeConversationId.value = conversationId;
    generationError.value = "";
    selectedConversationMessageIds.value = [];
    scheduleRemoteWorkspaceSave();
  }

  function ensureActiveConversation() {
    if (activeConversation.value) {
      return activeConversation.value;
    }

    const existingConversation = conversations.value[0];
    if (existingConversation) {
      selectConversation(existingConversation.id);
      return existingConversation;
    }

    const conversation = createEmptyConversation();
    conversations.value = [conversation];
    selectConversation(conversation.id);
    return conversation;
  }

  function createConversation() {
    if (workspaceLoading.value || recoveredWorkspaceDraft.value) return;
    const conversation = createEmptyConversation();

    conversations.value = [conversation, ...conversations.value];
    selectConversation(conversation.id);
    showFlashKey("store.flash.conversationCreated");
  }

  function deleteConversation(conversationId: string) {
    if (workspaceLoading.value || recoveredWorkspaceDraft.value) return false;
    const remaining = conversations.value.filter(
      (conversation) => conversation.id !== conversationId,
    );

    if (remaining.length === conversations.value.length) {
      return false;
    }

    if (remaining.length === 0) {
      const replacement = createEmptyConversation();

      conversations.value = [replacement];
      selectConversation(replacement.id);
      showFlashKey("store.flash.conversationDeleted", "success");
      scheduleRemoteWorkspaceSave();
      return true;
    }

    conversations.value = remaining;

    if (activeConversationId.value === conversationId) {
      activeConversationId.value = remaining[0].id;
    }

    selectedConversationMessageIds.value = [];
    generationError.value = "";
    showFlashKey("store.flash.conversationDeleted", "success");
    scheduleRemoteWorkspaceSave();
    return true;
  }

  function updateCurrentDraft(value: string) {
    if (workspaceLoading.value || recoveredWorkspaceDraft.value) return;
    const currentConversation = ensureActiveConversation();

    updateConversation(currentConversation.id, (conversation) => ({
      ...conversation,
      draft: value,
      updatedAt: getNow(),
    }));
  }

  async function saveCurrentDraft() {
    if (workspaceLoading.value || recoveredWorkspaceDraft.value) return false;
    ensureActiveConversation();
    const accountVersion = remotePrintAccountVersion;
    if (isAuthenticated.value) {
      const saved = await flushRemoteWorkspaceSave();
      if (!isCurrentAccount(accountVersion)) return false;
      if (!saved) {
        showFlash(workspaceSyncError.value || translate("store.errors.syncWorkspace"), "error");
        return false;
      }
    } else {
      try {
        window.localStorage.setItem(STORAGE_KEY, JSON.stringify(persistableState.value));
      } catch {
        showFlashKey("store.errors.localDraftStorage", "error");
        return false;
      }
    }
    showFlashKey("store.flash.draftSaved", "success");
    return true;
  }

  async function sendCurrentDraft() {
    const accountVersion = remotePrintAccountVersion;
    if (workspaceLoading.value || recoveredWorkspaceDraft.value) return false;
    const conversation = ensureActiveConversation();

    if (isGenerating.value) {
      return false;
    }

    const prompt = conversation.draft.trim();

    if (!prompt) {
      generationError.value = translate("store.errors.promptRequired");
      return false;
    }

    const userMessage: ConversationMessage = {
      id: createId("message"),
      role: "user",
      text: prompt,
      createdAt: getNow(),
    };

    generationError.value = "";
    isGenerating.value = true;

    updateConversation(conversation.id, (current) => ({
      ...current,
      title:
        current.messages.length === 0
          ? prompt.slice(0, 8) || translate("store.labels.conversationUntitled")
          : current.title,
      preview: prompt.slice(0, 22),
      updatedAt: getNow(),
      draft: "",
      messages: [...current.messages, userMessage],
    }));

    touchConversation(conversation.id);

    try {
      const reply =
        isAuthenticated.value && authSession.value
          ? (
              await generateAIReply(authSession.value.accessToken, {
                messages: buildAIReplyMessages([...conversation.messages, userMessage]),
                conversationId: conversation.id,
              })
            ).content
          : await generateReplyWithMockService({ prompt });
      if (!isCurrentAccount(accountVersion)) return false;
      const assistantMessage: ConversationMessage = {
        id: createId("message"),
        role: "assistant",
        text: reply,
        createdAt: getNow(),
      };

      updateConversation(conversation.id, (current) => ({
        ...current,
        preview: reply.slice(0, 22),
        updatedAt: getNow(),
        messages: [...current.messages, assistantMessage],
      }));
      touchConversation(conversation.id);
      showFlashKey("store.flash.replyGenerated", "success");
      return true;
    } catch (error) {
      if (!isCurrentAccount(accountVersion)) return false;
      generationError.value = getErrorMessage(error, "store.errors.generateReply");
      showFlash(generationError.value, "error");
      return false;
    } finally {
      if (isCurrentAccount(accountVersion)) {
        isGenerating.value = false;
      }
    }
  }

  async function regenerateLatestReply() {
    const accountVersion = remotePrintAccountVersion;
    if (workspaceLoading.value || recoveredWorkspaceDraft.value) return false;
    const conversation = activeConversation.value;
    let latestUserMessage: ConversationMessage | undefined;

    for (let index = (conversation?.messages.length ?? 0) - 1; index >= 0; index -= 1) {
      const candidate = conversation?.messages[index];

      if (candidate?.role === "user") {
        latestUserMessage = candidate;
        break;
      }
    }

    if (!conversation || !latestUserMessage || isGenerating.value) {
      generationError.value = translate("store.errors.regenerateUnavailable");
      return false;
    }

    isGenerating.value = true;
    generationError.value = "";

    const messages =
      conversation.messages.at(-1)?.role === "assistant"
        ? conversation.messages.slice(0, -1)
        : conversation.messages;

    try {
      const reply =
        isAuthenticated.value && authSession.value
          ? (
              await generateAIReply(authSession.value.accessToken, {
                messages: buildAIReplyMessages(messages),
                conversationId: conversation.id,
              })
            ).content
          : await generateReplyWithMockService({ prompt: latestUserMessage.text });
      if (!isCurrentAccount(accountVersion)) return false;
      const assistantMessage: ConversationMessage = {
        id: createId("message"),
        role: "assistant",
        text: reply,
        createdAt: getNow(),
      };

      updateConversation(conversation.id, (current) => ({
        ...current,
        preview: reply.slice(0, 22),
        updatedAt: getNow(),
        messages: [...messages, assistantMessage],
      }));
      touchConversation(conversation.id);
      showFlashKey("store.flash.replyRegenerated", "success");
      return true;
    } catch (error) {
      if (!isCurrentAccount(accountVersion)) return false;
      generationError.value = getErrorMessage(error, "store.errors.regenerateReply");
      showFlash(generationError.value, "error");
      return false;
    } finally {
      if (isCurrentAccount(accountVersion)) {
        isGenerating.value = false;
      }
    }
  }

  function buildPrintJob(title: string, content: string, source: string): PrintJob {
    const now = getNow();

    return {
      id: createId("print"),
      title,
      source,
      deviceId: defaultDeviceId.value,
      status: "queued",
      createdAt: now,
      updatedAt: now,
      content,
    };
  }

  async function maybeCompleteQueuedJob(jobId: string) {
    if (disposed) return;
    const accountVersion = remotePrintAccountVersion;
    const timer = window.setTimeout(() => {
      anonymousPrintCompletionTimers.delete(timer);
      if (!isCurrentAccount(accountVersion)) return;
      const target = printJobs.value.find((job) => job.id === jobId);

      if (!target || target.status !== "queued") {
        return;
      }

      printJobs.value = printJobs.value.map((job) =>
        job.id === jobId
          ? {
              ...job,
              status: "completed",
              updatedAt: getNow(),
            }
          : job,
      );
      scheduleRemoteWorkspaceSave();
    }, 500);
    anonymousPrintCompletionTimers.add(timer);
  }

  async function addPrintJob(title: string, content: string, source: string) {
    if (isAuthenticated.value) loadPrintOutcome();
    if (printSubmissionUnknown.value) {
      showFlashKey("errors.api.print_submission_unknown", "error");
      return null;
    }
    if (isCreatingPrint.value) {
      return null;
    }

    isCreatingPrint.value = true;
    const accountVersion = remotePrintAccountVersion;
    const originUserId = authUser.value?.id;
    try {
      if (isAuthenticated.value && authSession.value) {
        if (!defaultDeviceId.value) {
          showFlashKey("store.errors.defaultDeviceRequired", "error");
          return null;
        }

        const job = await createPrintJob(authSession.value.accessToken, {
          title,
          source,
          content,
          printerBindingId: defaultDeviceId.value,
          submitImmediately: true,
        });
        if (!isCurrentAccount(accountVersion)) return null;
        upsertPrintJob(job);
        showFlashKey("store.flash.printQueuedDirectly", "success");
        return job;
      }

      const job = buildPrintJob(title, content, source);
      printJobs.value = [job, ...printJobs.value];
      scheduleRemoteWorkspaceSave();

      showFlashKey("store.flash.printQueuedDirectly", "success");
      await maybeCompleteQueuedJob(job.id);

      return job;
    } catch (error) {
      if (isPrintSubmissionUnknown(error)) markPrintOutcomeUnknown(originUserId);
      if (!isCurrentAccount(accountVersion)) return null;
      if (isPrintSubmissionUnknown(error)) {
        try {
          await refreshRemotePrintJobs();
        } catch {
          /* Keep the safety warning even if history cannot load. */
        }
        if (!isCurrentAccount(accountVersion)) return null;
      }
      const message = isPrintSubmissionUnknown(error)
        ? translate("errors.api.print_submission_unknown")
        : getErrorMessage(error, "store.errors.createPrint");
      showFlash(message, "error");
      return null;
    } finally {
      if (isCurrentAccount(accountVersion)) isCreatingPrint.value = false;
    }
  }

  async function createPrintFromSelectedMessages() {
    if (selectedConversationMessages.value.length === 0) {
      showFlashKey("store.errors.selectMessagesRequired", "error");
      return null;
    }

    const content = selectedConversationMessages.value
      .map(
        (message) =>
          `${translate(
            message.role === "user"
              ? "store.labels.messageAuthorUser"
              : "store.labels.messageAuthorAssistant",
          )}：${message.text}`,
      )
      .join("\n\n");

    return addPrintJob(
      activeConversation.value?.title ?? translate("store.labels.selectedMessagesTitle"),
      content,
      translate("store.labels.selectedMessagesSource"),
    );
  }

  async function createPrintFromConversation() {
    const conversation = activeConversation.value;

    if (!conversation || conversation.messages.length === 0) {
      showFlashKey("store.errors.conversationEmpty", "error");
      return null;
    }

    const content = conversation.messages
      .map(
        (message) =>
          `${translate(
            message.role === "user"
              ? "store.labels.messageAuthorUser"
              : "store.labels.messageAuthorAssistant",
          )}：${message.text}`,
      )
      .join("\n\n");
    return addPrintJob(
      conversation.title,
      content,
      translate("store.labels.currentConversationSource"),
    );
  }

  async function createManualPrint(options?: { title?: string; content?: string }) {
    return addPrintJob(
      options?.title?.trim() || translate("store.labels.manualPrintTitle"),
      options?.content?.trim() || translate("store.labels.manualPrintContent"),
      translate("store.labels.manualPrintSource"),
    );
  }

  async function confirmPrint(jobId: string) {
    if (isAuthenticated.value) loadPrintOutcome();
    if (printSubmissionUnknown.value) {
      showFlashKey("errors.api.print_submission_unknown", "error");
      return false;
    }
    const target = (isAuthenticated.value ? remotePrintJobs.value : printJobs.value).find(
      (job) => job.id === jobId,
    );

    if (!target || target.status !== "pending") {
      return false;
    }

    if (isAuthenticated.value && authSession.value) {
      const accountVersion = remotePrintAccountVersion;
      const originUserId = authUser.value?.id;
      try {
        const submitted = await submitPrintJob(authSession.value.accessToken, jobId);
        if (!isCurrentAccount(accountVersion)) return false;
        upsertPrintJob(submitted);
        showFlashKey("store.flash.printQueued", "success");
        return true;
      } catch (error) {
        if (isPrintSubmissionUnknown(error)) markPrintOutcomeUnknown(originUserId);
        if (!isCurrentAccount(accountVersion)) return false;
        if (isPrintSubmissionUnknown(error)) {
          try {
            await refreshRemotePrintJobs();
          } catch {
            /* Preserve the safety warning. */
          }
          if (!isCurrentAccount(accountVersion)) return false;
        }
        showFlash(
          isPrintSubmissionUnknown(error)
            ? translate("errors.api.print_submission_unknown")
            : getErrorMessage(error, "store.errors.submitPrint"),
          "error",
        );
        return false;
      }
    }

    printJobs.value = printJobs.value.map((job) =>
      job.id === jobId
        ? {
            ...job,
            status: "queued",
            updatedAt: getNow(),
          }
        : job,
    );
    scheduleRemoteWorkspaceSave();
    showFlashKey("store.flash.printQueued", "success");
    await maybeCompleteQueuedJob(jobId);
    return true;
  }

  async function cancelPrint(jobId: string) {
    const target = (isAuthenticated.value ? remotePrintJobs.value : printJobs.value).find(
      (job) => job.id === jobId,
    );

    if (!target || (target.status !== "pending" && target.status !== "queued")) {
      return false;
    }

    if (isAuthenticated.value && authSession.value) {
      const accountVersion = remotePrintAccountVersion;
      try {
        const cancelled = await cancelPrintJob(authSession.value.accessToken, jobId);
        if (!isCurrentAccount(accountVersion)) return false;
        upsertPrintJob(cancelled);
        showFlashKey("store.flash.printCancelled", "success");
        return true;
      } catch (error) {
        if (!isCurrentAccount(accountVersion)) return false;
        showFlash(getErrorMessage(error, "store.errors.cancelPrint"), "error");
        return false;
      }
    }

    printJobs.value = printJobs.value.map((job) =>
      job.id === jobId
        ? {
            ...job,
            status: "cancelled",
            updatedAt: getNow(),
          }
        : job,
    );
    scheduleRemoteWorkspaceSave();
    showFlashKey("store.flash.printCancelled", "success");
    return true;
  }

  async function updatePrintDevice(jobId: string, deviceId: string) {
    if (isAuthenticated.value && authSession.value) {
      const accountVersion = remotePrintAccountVersion;
      try {
        const updated = await updatePrintJobDeviceWithApi(authSession.value.accessToken, jobId, {
          printerBindingId: deviceId,
        });
        if (!isCurrentAccount(accountVersion)) return;
        upsertPrintJob(updated);
        showFlashKey("store.flash.printDeviceUpdated", "success");
        return;
      } catch (error) {
        if (!isCurrentAccount(accountVersion)) return;
        showFlash(getErrorMessage(error, "store.errors.updatePrintDevice"), "error");
        return;
      }
    }

    printJobs.value = printJobs.value.map((job) =>
      job.id === jobId
        ? {
            ...job,
            deviceId,
            updatedAt: getNow(),
          }
        : job,
    );
    scheduleRemoteWorkspaceSave();
    showFlashKey("store.flash.printDeviceUpdated", "success");
  }

  async function addDevice(options?: {
    name?: string;
    note?: string;
    deviceId?: string;
    setAsDefault?: boolean;
  }) {
    if (isAuthenticated.value && authSession.value) {
      const accountVersion = remotePrintAccountVersion;
      try {
        const deviceIdentifier = options?.deviceId?.trim() ?? "";
        if (!deviceIdentifier) {
          showFlashKey("store.errors.deviceIdRequired", "error");
          return null;
        }

        const device = await bindPrinter(authSession.value.accessToken, {
          name:
            options?.name?.trim() ||
            translate("store.labels.deviceName", { count: devices.value.length + 1 }),
          note: options?.note?.trim() || "",
          deviceId: deviceIdentifier,
        });
        if (!isCurrentAccount(accountVersion)) return null;
        upsertDevice(device);
        if (options?.setAsDefault || !defaultDeviceId.value) {
          defaultDeviceId.value = device.id;
          scheduleRemoteWorkspaceSave();
        }
        showFlashKey("store.flash.deviceBound", "success");
        return device;
      } catch (error) {
        if (!isCurrentAccount(accountVersion)) return null;
        showFlash(getErrorMessage(error, "store.errors.bindDevice"), "error");
        return null;
      }
    }

    const nextIndex = devices.value.length + 1;
    const device: Device = {
      id: createId("device"),
      name: options?.name?.trim() || translate("store.labels.deviceName", { count: nextIndex }),
      status: "pending",
      note: options?.note?.trim() || translate("store.labels.devicePendingNote"),
    };

    devices.value = [...devices.value, device];
    if (options?.setAsDefault) {
      defaultDeviceId.value = device.id;
      scheduleRemoteWorkspaceSave();
    }
    showFlashKey("store.flash.deviceAdded", "success");
    return device;
  }

  async function removeDevice(deviceId: string) {
    const target = devices.value.find((device) => device.id === deviceId);

    if (!target) {
      return false;
    }

    if (isAuthenticated.value && authSession.value) {
      const accountVersion = remotePrintAccountVersion;
      try {
        await deletePrinter(authSession.value.accessToken, deviceId);
        if (!isCurrentAccount(accountVersion)) return false;
      } catch (error) {
        if (!isCurrentAccount(accountVersion)) return false;
        showFlash(getErrorMessage(error, "store.errors.deleteDevice"), "error");
        return false;
      }

      const remainingDevices = devices.value.filter((device) => device.id !== deviceId);
      const fallbackDeviceId =
        defaultDeviceId.value === deviceId
          ? (remainingDevices.find((device) => device.status !== "offline")?.id ?? "")
          : defaultDeviceId.value;

      devices.value = remainingDevices;
      remotePrintMutationVersion++;
      scheduleRemotePrintStatusSync(true);
      defaultDeviceId.value = fallbackDeviceId;
      scheduleRemoteWorkspaceSave();
      showFlashKey("store.flash.deviceDeleted", "success");
      return true;
    }

    const remainingDevices = devices.value.filter((device) => device.id !== deviceId);
    const fallbackDeviceId =
      defaultDeviceId.value === deviceId ? (remainingDevices[0]?.id ?? "") : defaultDeviceId.value;

    devices.value = remainingDevices;
    defaultDeviceId.value = fallbackDeviceId;
    scheduleRemoteWorkspaceSave();
    printJobs.value = printJobs.value.map((job) =>
      job.deviceId === deviceId
        ? {
            ...job,
            deviceId: fallbackDeviceId,
            updatedAt: getNow(),
          }
        : job,
    );
    showFlashKey(
      target.status === "pending" ? "store.flash.deviceRemoved" : "store.flash.deviceDeleted",
      "success",
    );
    return true;
  }

  function setDefaultDevice(deviceId: string) {
    if (workspaceLoading.value || recoveredWorkspaceDraft.value) return;
    const targetDevice = devices.value.find((device) => device.id === deviceId);

    if (!targetDevice || targetDevice.status === "offline") {
      showFlashKey("store.errors.defaultDeviceOffline", "error");
      return;
    }

    defaultDeviceId.value = deviceId;
    scheduleRemoteWorkspaceSave();
    showFlashKey("store.flash.defaultDeviceUpdated", "success");
  }

  function setTheme(theme: ThemeMode) {
    if (workspaceLoading.value || recoveredWorkspaceDraft.value) return;
    selectedTheme.value = theme;
    scheduleRemoteWorkspaceSave();
    showFlashKey("store.flash.themeUpdated");
  }

  function setLocale(nextLocale: LocalePreference) {
    if (workspaceLoading.value || recoveredWorkspaceDraft.value) return;
    localePreference.value = normalizeLocalePreference(nextLocale);
    scheduleRemoteWorkspaceSave();
    showFlash(translate("store.flash.localeUpdated"));
  }

  function setSendConfirmation(enabled: boolean) {
    if (workspaceLoading.value || recoveredWorkspaceDraft.value) return;
    sendConfirmationEnabled.value = false;
    scheduleRemoteWorkspaceSave();
    if (enabled) {
      showFlashKey("store.flash.sendConfirmationEnabled");
    }
  }

  function setTutorialTabEnabled(enabled: boolean) {
    if (workspaceLoading.value || recoveredWorkspaceDraft.value) return;
    tutorialTabEnabled.value = enabled;
    scheduleRemoteWorkspaceSave();
    showFlashKey(enabled ? "store.flash.tutorialTabShown" : "store.flash.tutorialTabHidden");
  }

  function setLoginProtection(enabled: boolean) {
    if (workspaceLoading.value || recoveredWorkspaceDraft.value) return;
    loginProtectionEnabled.value = enabled;
    scheduleRemoteWorkspaceSave();
    showFlashKey(
      enabled ? "store.flash.loginProtectionEnabled" : "store.flash.loginProtectionDisabled",
    );
  }

  async function saveAIServiceConfig(config: {
    providerName: string;
    providerType: string;
    baseUrl: string;
    model: string;
    apiKey: string;
  }) {
    const accountVersion = remotePrintAccountVersion;
    const current = authSession.value;

    if (!current) {
      aiConfigSaveError.value = translate("store.errors.authRequired");
      return false;
    }

    aiConfigSaving.value = true;
    aiConfigSaveError.value = "";

    try {
      const summary = await saveAIConfig(current.accessToken, config);
      if (!isCurrentAccount(accountVersion)) return false;
      applyAIConfig(summary);
      showFlashKey("store.flash.aiConfigSaved", "success");
      return true;
    } catch (error) {
      if (!isCurrentAccount(accountVersion)) return false;
      aiConfigSaveError.value = getErrorMessage(error, "store.errors.saveAIConfig");
      showFlash(aiConfigSaveError.value, "error");
      return false;
    } finally {
      if (isCurrentAccount(accountVersion)) {
        aiConfigSaving.value = false;
      }
    }
  }

  async function saveMemoryConfig(config: {
    enabled: boolean;
    zepBaseUrl: string;
    zepApiKey?: string;
  }) {
    const accountVersion = remotePrintAccountVersion;
    const current = authSession.value;

    if (!current) {
      memoryConfigSaveError.value = translate("store.errors.authRequired");
      return false;
    }

    memoryConfigSaving.value = true;
    memoryConfigSaveError.value = "";

    try {
      const summary = await updateMemorySettings(current.accessToken, config);
      if (!isCurrentAccount(accountVersion)) return false;
      memorySettings.value = summary;
      showFlashKey("store.flash.memoryConfigSaved", "success");
      return true;
    } catch (error) {
      if (!isCurrentAccount(accountVersion)) return false;
      memoryConfigSaveError.value = getErrorMessage(error, "store.errors.saveMemoryConfig");
      showFlash(memoryConfigSaveError.value, "error");
      return false;
    } finally {
      if (isCurrentAccount(accountVersion)) {
        memoryConfigSaving.value = false;
      }
    }
  }

  function setAuthState(user: User, session: AuthSession) {
    authUser.value = user;
    authSession.value = session;
    authError.value = "";
    feedbackError.value = "";
    accountCreationError.value = "";
    aiConfigError.value = "";
  }

  function clearAuthState() {
    if (!preservePendingWorkspaceDraft() && remoteWorkspaceDirty) {
      // A safe sign-out must remain possible even when browser storage fails.
      // Try an explicit file backup before clearing private in-memory state.
      try {
        downloadState(workspaceState.value);
      } catch {
        /* Keep sign-out available. */
      }
    }
    if (remoteSaveTimer) {
      window.clearTimeout(remoteSaveTimer);
      remoteSaveTimer = 0;
    }

    authUser.value = null;
    authSession.value = null;
    authLoading.value = false;
    authBootstrapping.value = false;
    authError.value = "";
    feedbackError.value = "";
    accountCreationError.value = "";
    aiConfigError.value = "";
    aiConfigSaveError.value = "";
    workspaceOwnerId.value = null;
  }

  async function refreshSessionIfNeeded() {
    const current = authSession.value;

    if (disposed || !current) {
      return false;
    }

    const expiresAt = new Date(current.accessTokenExpiresAt).getTime();

    if (Number.isNaN(expiresAt)) {
      clearAuthState();
      restoreAnonymousWorkspace();
      return false;
    }

    if (expiresAt > Date.now() + 30_000) {
      return true;
    }

    try {
      return !!(await refreshCurrentAuthSession(current.accessToken));
    } catch (error) {
      if (disposed || authSession.value?.accessToken !== current.accessToken) return false;
      if (error instanceof AuthApiError) {
        authError.value = error.message;
      }

      return false;
    }
  }

  async function initializeAuth() {
    if (!authSession.value) {
      clearAuthState();
      return false;
    }

    authBootstrapping.value = true;
    let accountVersion = remotePrintAccountVersion;

    try {
      const sessionReady = await refreshSessionIfNeeded();

      if (!sessionReady || !authSession.value) {
        return false;
      }
      if (disposed) return false;
      // A successful refresh may populate authUser for a restored session.
      // refreshSessionIfNeeded already rejects responses from an older session.
      accountVersion = remotePrintAccountVersion;

      if (!authUser.value) {
        const user = await fetchCurrentUser(authSession.value.accessToken);
        if (disposed || accountVersion !== remotePrintAccountVersion) return false;
        authUser.value = user;
        accountVersion = remotePrintAccountVersion;
      }

      const loaded = await loadRemoteWorkspace();
      if (disposed || accountVersion !== remotePrintAccountVersion) return false;
      if (!loaded) {
        clearAuthState();
        authError.value =
          workspaceSyncError.value || translate("store.errors.loadAccountDataRelogin");
        restoreAnonymousWorkspace();
        return false;
      }

      authError.value = "";
      return true;
    } catch (error) {
      if (disposed || accountVersion !== remotePrintAccountVersion) return false;
      clearAuthState();
      restoreAnonymousWorkspace();
      authError.value = getErrorMessage(error, "store.errors.authRequired");
      return false;
    } finally {
      if (isCurrentAccount(accountVersion)) authBootstrapping.value = false;
    }
  }

  async function login(email: string, password: string) {
    authLoading.value = true;
    authError.value = "";
    let accountVersion = remotePrintAccountVersion;

    try {
      const result = await loginWithApi({ email, password });
      if (disposed || accountVersion !== remotePrintAccountVersion) return false;
      setAuthState(result.user, result.session);
      accountVersion = remotePrintAccountVersion;
      const loaded = await loadRemoteWorkspace();
      if (disposed || accountVersion !== remotePrintAccountVersion) return false;

      if (!loaded) {
        const workspaceError =
          workspaceSyncError.value || translate("store.errors.loadAccountData");
        clearAuthState();
        restoreAnonymousWorkspace();
        authError.value = workspaceError;
        showFlash(workspaceError, "error");
        return false;
      }

      showFlashKey("store.flash.loginSuccess", "success");
      postLoginTutorialOpen.value = devices.value.length === 0;
      return true;
    } catch (error) {
      if (disposed || accountVersion !== remotePrintAccountVersion) return false;
      authError.value = getErrorMessage(error, "store.errors.login");
      postLoginTutorialOpen.value = false;
      showFlash(authError.value, "error");
      return false;
    } finally {
      if (isCurrentAccount(accountVersion)) authLoading.value = false;
    }
  }

  async function changePassword(currentPassword: string, newPassword: string) {
    const accountVersion = remotePrintAccountVersion;
    const current = authSession.value;

    if (!current) {
      authError.value = translate("store.errors.authRequired");
      return false;
    }

    passwordChangeLoading.value = true;
    authError.value = "";

    try {
      await flushRemoteWorkspaceSave();
      if (!isCurrentAccount(accountVersion)) return false;
      await changePasswordWithApi({
        accessToken: authSession.value?.accessToken ?? current.accessToken,
        currentPassword,
        newPassword,
      });
      if (!isCurrentAccount(accountVersion)) return false;
      clearAuthState();
      restoreAnonymousWorkspace();
      showFlashKey("store.flash.passwordUpdated", "success");
      return true;
    } catch (error) {
      if (!isCurrentAccount(accountVersion)) return false;
      authError.value = getErrorMessage(error, "store.errors.changePassword");
      showFlash(authError.value, "error");
      return false;
    } finally {
      if (isCurrentAccount(accountVersion)) {
        passwordChangeLoading.value = false;
      }
    }
  }

  async function submitFeedback(content: string) {
    const accountVersion = remotePrintAccountVersion;
    const originUserId = authUser.value?.id;
    const current = authSession.value;
    loadFeedbackOutcome();
    if (feedbackSubmissionUnknown.value) {
      feedbackError.value = translate("store.errors.feedbackSubmissionUnknown");
      return false;
    }
    if (feedbackSubmitting.value) return false;
    const normalizedContent = content.trim();

    if (!current) {
      feedbackError.value = translate("store.errors.feedbackLoginRequired");
      showFlash(feedbackError.value, "error");
      return false;
    }

    if (!normalizedContent) {
      feedbackError.value = translate("feedback.errors.required");
      return false;
    }

    feedbackSubmitting.value = true;
    feedbackError.value = "";

    try {
      await submitFeedbackToAdmin(current.accessToken, normalizedContent);
      if (!isCurrentAccount(accountVersion)) return false;
      showFlashKey("store.flash.feedbackSubmitted", "success");
      return true;
    } catch (error) {
      if (isPrintSubmissionUnknown(error)) markFeedbackOutcomeUnknown(originUserId);
      if (!isCurrentAccount(accountVersion)) return false;
      feedbackError.value = isPrintSubmissionUnknown(error)
        ? translate("store.errors.feedbackSubmissionUnknown")
        : getErrorMessage(error, "store.errors.submitFeedback");
      showFlash(feedbackError.value, "error");
      return false;
    } finally {
      if (isCurrentAccount(accountVersion)) {
        feedbackSubmitting.value = false;
      }
    }
  }

  async function logout() {
    const accountVersion = remotePrintAccountVersion;

    await flushRemoteWorkspaceSave();
    if (!isCurrentAccount(accountVersion)) return;
    const current = authSession.value;

    if (current) {
      try {
        await logoutWithApi({
          accessToken: current.accessToken,
          refreshToken: current.refreshToken,
        });
      } catch {
        // Local sign-out should still succeed if the backend session already expired.
      }
    }

    if (!isCurrentAccount(accountVersion)) return;
    clearAuthState();
    restoreAnonymousWorkspace();
    postLoginTutorialOpen.value = false;
    showFlashKey("store.flash.loggedOut");
  }

  function closePostLoginTutorial() {
    postLoginTutorialOpen.value = false;
  }

  async function createAccount(email: string, name: string, password: string) {
    const accountVersion = remotePrintAccountVersion;
    const current = authSession.value;

    if (!current) {
      accountCreationError.value = translate("store.errors.authRequired");
      return false;
    }

    accountCreationLoading.value = true;
    accountCreationError.value = "";

    try {
      await createUserWithApi(current.accessToken, {
        email,
        name,
        password,
      });
      if (!isCurrentAccount(accountVersion)) return false;
      showFlashKey("store.flash.accountCreated", "success");
      return true;
    } catch (error) {
      if (!isCurrentAccount(accountVersion)) return false;
      accountCreationError.value = getErrorMessage(error, "store.errors.createAccount");
      showFlash(accountCreationError.value, "error");
      return false;
    } finally {
      if (isCurrentAccount(accountVersion)) {
        accountCreationLoading.value = false;
      }
    }
  }

  function formatPrintTime(iso: string) {
    void effectiveLocale.value;
    return formatRelativeTimestamp(iso);
  }

  function getDeviceName(deviceId: string) {
    void effectiveLocale.value;
    return (
      deviceMap.value[deviceId]?.name ??
      translate(deviceId ? "store.labels.removedDevice" : "common.labels.notSet")
    );
  }

  function toggleConversationMessageSelection(messageId: string) {
    selectedConversationMessageIds.value = selectedConversationMessageIds.value.includes(messageId)
      ? selectedConversationMessageIds.value.filter((current) => current !== messageId)
      : [...selectedConversationMessageIds.value, messageId];
  }

  return {
    authUser,
    authSession,
    authLoading,
    authError,
    authBootstrapping,
    passwordChangeLoading,
    feedbackSubmitting,
    feedbackError,
    workspaceLoading,
    workspaceSyncing,
    workspaceSyncError,
    workspaceConflict,
    recoveredWorkspaceDraft,
    damagedWorkspaceDrafts,
    downloadDamagedWorkspaceDraft,
    discardDamagedWorkspaceDraft,
    workspaceDraftStorageError,
    downloadRecoveredWorkspaceDraft,
    discardRecoveredWorkspaceDraft,
    restoreRecoveredWorkspaceDraft,
    accountCreationLoading,
    accountCreationError,
    aiConfigSummary,
    aiConfigLoading,
    aiConfigSaving,
    aiConfigError,
    aiConfigSaveError,
    memorySettings,
    memoryConfigSaving,
    memoryConfigError,
    memoryConfigSaveError,
    printerSyncError,
    deviceSyncError,
    printSyncError,
    printJobsLoading,
    activePrintJobsLoading,
    historyPrintJobsLoading,
    activePrintJobsCursor,
    historyPrintJobsCursor,
    remotePrintCounts,
    devices,
    conversations,
    activeConversationId,
    printJobs,
    remotePrintJobs,
    schedules,
    sources,
    loginProtectionEnabled,
    sendConfirmationEnabled,
    tutorialTabEnabled,
    selectedTheme,
    localePreference,
    effectiveLocale,
    defaultDeviceId,
    serviceBinding,
    isGenerating,
    generationError,
    selectedConversationMessageIds,
    isCreatingPrint,
    printSubmissionUnknown,
    printOutcomeStorageError,
    feedbackSubmissionUnknown,
    feedbackOutcomeStorageError,
    acknowledgeFeedbackOutcome,
    acknowledgePrintOutcome,
    flashMessage,
    flashTone,
    dismissFlash,
    postLoginTutorialOpen,
    activeConversation,
    conversationMessages,
    selectedConversationMessages,
    defaultDevice,
    pendingPrintJobs,
    recentPrintJobs,
    printHistoryJobs,
    connectedDevicesCount,
    pendingConfirmationCount,
    summaryCards,
    activeDeviceLabel,
    activeModelLabel,
    todayPrintCount,
    welcomeLabel,
    isConfigured,
    isAuthenticated,
    isAdmin,
    selectConversation,
    createConversation,
    deleteConversation,
    updateCurrentDraft,
    saveCurrentDraft,
    sendCurrentDraft,
    regenerateLatestReply,
    createPrintFromSelectedMessages,
    createPrintFromConversation,
    createManualPrint,
    loadMorePrintJobs,
    loadPrintJobDetail,
    confirmPrint,
    cancelPrint,
    updatePrintDevice,
    addDevice,
    removeDevice,
    setDefaultDevice,
    setTheme,
    setLocale,
    showFlashKey,
    setSendConfirmation,
    setTutorialTabEnabled,
    setLoginProtection,
    saveAIServiceConfig,
    saveMemoryConfig,
    initializeAuth,
    retrySynchronization,
    reloadConflictedWorkspace,
    downloadWorkspaceDraft,
    refreshSessionIfNeeded,
    changePassword,
    submitFeedback,
    createAccount,
    login,
    logout,
    closePostLoginTutorial,
    formatPrintTime,
    getDeviceName,
    getDeviceStatusLabel,
    getPrintStatusLabel,
    getSourceStatusLabel,
    toggleConversationMessageSelection,
  };
});
