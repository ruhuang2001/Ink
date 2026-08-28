import { defineStore } from "pinia";
import { computed, ref, watch } from "vue";

import {
  normalizeLocalePreference,
  resolveLocalePreference,
  setI18nLocale,
  translate,
} from "@/i18n";
import { getLocalizedErrorMessage } from "@/i18n/errors";
import { formatScheduleLabelForLocale } from "@/i18n/formatters";
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
import { generateReplyWithMockService } from "@/services/mockInk";
import {
  createPrintSchedule,
  deletePrintSchedule,
  disablePlugin,
  fetchAdminPlugins,
  fetchPlugins,
  fetchPrintSchedules,
  installPluginFromGit,
  savePluginBinding,
  testPluginBinding,
  togglePrintSchedule,
  updatePrintSchedule,
  uploadPluginZip,
} from "@/services/plugins";
import {
  bindPrinter,
  cancelPrintJob,
  createPrintJob,
  deletePrinter,
  fetchPrintJobs,
  fetchPrinters,
  submitPrintJob,
  updatePrintJobDevice as updatePrintJobDeviceWithApi,
} from "@/services/printers";
import {
  createUserWithApi,
  fetchWorkspaceStateWithApi,
  saveWorkspaceStateWithApi,
} from "@/services/workspace";
import type { PluginDetails, PrintScheduleView } from "@/types/plugins";
import type {
  AuthSession,
  Conversation,
  ConversationMessage,
  Device,
  LocaleCode,
  LocalePreference,
  PersistedWorkspaceState,
  Preferences,
  PrintJob,
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
const REMOTE_SAVE_DEBOUNCE_MS = 750;
const REMOTE_PRINT_STATUS_POLL_MS = 5000;
const REMOTE_PRINT_STATUS_INITIAL_POLL_MS = 1500;
const REMOTE_PRINT_STATUS_MAX_BACKOFF_MS = 120000;

type ActiveSchedule = Schedule & {
  pluginInstallationId: string;
  frequencyType: "daily" | "weekly";
  timezone: string;
  hour: number;
  minute: number;
  weekdays: number[];
  printPolicy: {
    batchSize: number;
  };
  pluginDisplayName: string;
  nextRunAt?: string;
  lastRunAt?: string;
  lastError: string;
};

function getNow() {
  return new Date().toISOString();
}

function getErrorMessage(error: unknown, fallbackKey: string) {
  return getLocalizedErrorMessage(error, fallbackKey);
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

function mapRemoteScheduleToActiveSchedule(
  schedule: PrintScheduleView,
  locale: LocaleCode,
): ActiveSchedule {
  return {
    id: schedule.id,
    title: schedule.title,
    source: schedule.sourceLabel,
    timeLabel: formatScheduleLabelForLocale(
      schedule.frequencyType,
      schedule.hour,
      schedule.minute,
      schedule.weekdays,
      locale,
      schedule.timeLabel,
    ),
    deviceId: schedule.deviceId,
    enabled: schedule.enabled,
    pluginInstallationId: schedule.pluginInstallationId,
    frequencyType: schedule.frequencyType,
    timezone: schedule.timezone,
    hour: schedule.hour,
    minute: schedule.minute,
    weekdays: Array.from(schedule.weekdays),
    printPolicy: {
      batchSize: schedule.printPolicy.batchSize,
    },
    pluginDisplayName: schedule.pluginDisplayName,
    nextRunAt: schedule.nextRunAt,
    lastRunAt: schedule.lastRunAt,
    lastError: schedule.lastError || "",
  };
}

function mapLocalScheduleToActiveSchedule(schedule: Schedule): ActiveSchedule {
  return {
    ...schedule,
    pluginInstallationId: "",
    frequencyType: "daily",
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Shanghai",
    hour: 19,
    minute: 30,
    weekdays: [],
    printPolicy: {
      batchSize: 1,
    },
    pluginDisplayName: schedule.source,
    nextRunAt: undefined,
    lastRunAt: undefined,
    lastError: "",
  };
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
  const schedules: Schedule[] = [
    {
      id: "schedule-morning",
      title: translate("store.seed.schedules.morning.title"),
      source: translate("store.seed.schedules.morning.source"),
      timeLabel: formatScheduleLabelForLocale("daily", 8, 0, []),
      deviceId: "device-desk",
      enabled: true,
    },
    {
      id: "schedule-night",
      title: translate("store.seed.schedules.night.title"),
      source: translate("store.seed.schedules.night.source"),
      timeLabel: formatScheduleLabelForLocale("daily", 22, 0, []),
      deviceId: "device-bedroom",
      enabled: true,
    },
    {
      id: "schedule-weekend",
      title: translate("store.seed.schedules.weekend.title"),
      source: translate("store.seed.schedules.weekend.source"),
      timeLabel: formatScheduleLabelForLocale("weekly", 9, 30, [6]),
      deviceId: "device-desk",
      enabled: false,
    },
  ];
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
  const sources: SourceConnection[] = [
    {
      id: "source-worth",
      name: translate("store.seed.sources.worth.name"),
      type: translate("store.seed.sources.worth.type"),
      note: translate("store.seed.sources.worth.note"),
      status: "connected",
    },
    {
      id: "source-weather",
      name: translate("store.seed.sources.weather.name"),
      type: translate("store.seed.sources.weather.type"),
      note: translate("store.seed.sources.weather.note"),
      status: "connected",
    },
    {
      id: "source-calendar",
      name: translate("store.seed.sources.calendar.name"),
      type: translate("store.seed.sources.calendar.type"),
      note: translate("store.seed.sources.calendar.note"),
      status: "error",
    },
  ];
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

function readAuthSessionFromStorage(storage: Storage) {
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

function writePersistedAuthSession(session: AuthSession | null, persistAcrossRestarts: boolean) {
  if (typeof window === "undefined") {
    return;
  }

  window.sessionStorage.removeItem(AUTH_SESSION_STORAGE_KEY);
  window.localStorage.removeItem(AUTH_SESSION_STORAGE_KEY);

  if (!session) {
    return;
  }

  const storage = persistAcrossRestarts ? window.localStorage : window.sessionStorage;
  storage.setItem(AUTH_SESSION_STORAGE_KEY, JSON.stringify(session));
}

function sortPrintJobsByUpdatedAt(printJobs: PrintJob[]) {
  // The project targets ES2022; use a copied array so the fallback remains non-mutating.
  // oxlint-disable-next-line unicorn/no-array-sort
  return [...printJobs].sort(
    (left, right) => new Date(right.updatedAt).getTime() - new Date(left.updatedAt).getTime(),
  );
}

function buildAIReplyMessages(messages: ConversationMessage[]) {
  return messages.map((message) => ({
    role: message.role,
    content: message.text,
  })) as { role: "user" | "assistant"; content: string }[];
}

function mapPluginToSource(plugin: PluginDetails): SourceConnection {
  const note =
    plugin.binding?.lastFetchError ||
    plugin.binding?.lastError ||
    plugin.installation.lastError ||
    (plugin.installation.sourceType === "git" && plugin.installation.repoUrl
      ? translate("store.labels.pluginGitRepo", { url: plugin.installation.repoUrl })
      : "") ||
    plugin.installation.description ||
    translate("store.labels.pluginDefaultNote");

  return {
    id: plugin.installation.id,
    name: plugin.installation.displayName,
    type: `${translate(
      plugin.installation.runtimeType === "node"
        ? "store.labels.pluginRuntimeNode"
        : "store.labels.pluginRuntimePython",
    )} ${translate(
      plugin.installation.sourceType === "git"
        ? "store.labels.pluginSourceGit"
        : "store.labels.pluginSourceUpload",
    )}`,
    note,
    status:
      plugin.installation.status === "disabled" || !plugin.binding?.enabled
        ? "disconnected"
        : plugin.binding.status === "error"
          ? "error"
          : "connected",
  };
}

export const useWorkspaceStore = defineStore("workspace", () => {
  const persisted = readPersistedWorkspaceState();
  const persistedAuthSession = readPersistedAuthSession();

  const authUser = ref<User | null>(persistedAuthSession ? null : persisted.authUser);
  const authSession = ref<AuthSession | null>(persistedAuthSession);
  const authLoading = ref(false);
  const authError = ref("");
  const authBootstrapping = ref(false);
  const passwordChangeLoading = ref(false);
  const feedbackSubmitting = ref(false);
  const feedbackError = ref("");
  const workspaceLoading = ref(false);
  const workspaceSyncing = ref(false);
  const workspaceSyncError = ref("");
  const accountCreationLoading = ref(false);
  const accountCreationError = ref("");
  const aiConfigLoading = ref(false);
  const aiConfigSaving = ref(false);
  const aiConfigError = ref("");
  const printerSyncError = ref("");
  const pluginLoading = ref(false);
  const pluginSaving = ref(false);
  const pluginUploadLoading = ref(false);
  const pluginGitInstallLoading = ref(false);
  const pluginError = ref("");
  const pluginActionError = ref("");
  const pluginTestingId = ref("");
  const pluginSavingId = ref("");
  const pluginUploadingName = ref("");
  const workspaceOwnerId = ref<string | null>(null);
  const workspaceHydrating = ref(false);

  const devices = ref<Device[]>(persisted.devices);
  const conversations = ref<Conversation[]>(persisted.conversations);
  const activeConversationId = ref(persisted.activeConversationId);
  const printJobs = ref<PrintJob[]>(persisted.printJobs);
  const schedules = ref<Schedule[]>(persisted.schedules);
  const sources = ref<SourceConnection[]>(persisted.sources);
  const availablePlugins = ref<PluginDetails[]>([]);
  const adminPlugins = ref<PluginDetails[]>([]);
  const remoteSchedules = ref<PrintScheduleView[]>([]);

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

  const isGenerating = ref(false);
  const generationError = ref("");
  const selectedConversationMessageIds = ref<string[]>([]);
  const isCreatingPrint = ref(false);
  const flashMessage = ref("");
  const flashTone = ref<"success" | "error" | "info">("info");
  const postLoginTutorialOpen = ref(false);
  let flashTimer = 0;
  let remoteSaveTimer = 0;
  let remoteSavePromise: Promise<boolean> | null = null;
  let remoteSavePending = false;
  let remotePrintStatusTimer = 0;
  let remotePrintStatusPromise: Promise<void> | null = null;
  let remotePrintStatusBackoffMs = REMOTE_PRINT_STATUS_POLL_MS;

  configureAuthRefresh(async () => {
    const current = authSession.value;
    if (!current) {
      return null;
    }
    try {
      const refreshed = await refreshAuthSession(current.refreshToken);
      setAuthState(refreshed.user, refreshed.session);
      return refreshed.session.accessToken;
    } catch {
      clearAuthState();
      return null;
    }
  });

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
  const sortedPrintJobs = computed(() => sortPrintJobsByUpdatedAt(printJobs.value));
  const pendingPrintJobs = computed(() =>
    sortedPrintJobs.value.filter((job) => job.status === "pending" || job.status === "queued"),
  );
  const recentPrintJobs = computed(() => sortedPrintJobs.value.slice(0, 5));
  const activeSchedules = computed(() => {
    const locale = effectiveLocale.value;
    return isAuthenticated.value
      ? remoteSchedules.value.map((schedule) => mapRemoteScheduleToActiveSchedule(schedule, locale))
      : schedules.value.map(mapLocalScheduleToActiveSchedule);
  });
  const activeSources = computed(() =>
    isAuthenticated.value ? availablePlugins.value.map(mapPluginToSource) : sources.value,
  );
  const connectedDevicesCount = computed(
    () => devices.value.filter((device) => device.status === "connected").length,
  );
  const enabledSchedulesCount = computed(
    () => activeSchedules.value.filter((schedule) => schedule.enabled).length,
  );
  const pendingConfirmationCount = computed(
    () => printJobs.value.filter((job) => job.status === "pending").length,
  );
  const todayCompletedCount = computed(() => {
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
  const hasQueuedRemotePrintJobs = computed(
    () => isAuthenticated.value && printJobs.value.some((job) => job.status === "queued"),
  );

  const summaryCards = computed(() => {
    void effectiveLocale.value;
    return [
      {
        label: translate("store.summary.boundDevices"),
        value: translate("store.summary.deviceCount", { count: devices.value.length }),
      },
      {
        label: translate("store.summary.enabledSchedules"),
        value: translate("store.summary.scheduleCount", { count: enabledSchedulesCount.value }),
      },
      {
        label: translate("store.summary.completedToday"),
        value: translate("store.summary.printCount", { count: todayCompletedCount.value }),
      },
    ];
  });

  const workspaceState = computed<WorkspaceState>(() => ({
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

      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(value));
    },
    { deep: true, immediate: true },
  );

  watch(
    [authSession, loginProtectionEnabled],
    ([session, loginProtection]) => {
      writePersistedAuthSession(session, !loginProtection);
    },
    { deep: true, immediate: true },
  );

  function scheduleRemoteWorkspaceSave() {
    if (
      typeof window === "undefined" ||
      !authSession.value ||
      !authUser.value ||
      workspaceOwnerId.value !== authUser.value.id ||
      workspaceHydrating.value
    ) {
      return;
    }
    if (remoteSaveTimer) window.clearTimeout(remoteSaveTimer);
    remoteSaveTimer = window.setTimeout(() => {
      remoteSaveTimer = 0;
      void persistRemoteWorkspace();
    }, REMOTE_SAVE_DEBOUNCE_MS);
  }

  watch(
    hasQueuedRemotePrintJobs,
    (hasQueuedJobs) => {
      if (!hasQueuedJobs) {
        clearRemotePrintStatusSync();
        return;
      }

      scheduleRemotePrintStatusSync(true);
    },
    { immediate: true },
  );

  if (typeof window !== "undefined") {
    window.addEventListener("visibilitychange", () => {
      if (document.visibilityState === "hidden") {
        clearRemotePrintStatusSync();
        return;
      }
      if (hasQueuedRemotePrintJobs.value) {
        scheduleRemotePrintStatusSync(true);
      }
    });
  }

  function applyWorkspaceState(nextState: WorkspaceState) {
    const normalized = normalizeWorkspaceState(nextState);
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
  }

  function upsertPrintJob(nextJob: PrintJob) {
    printJobs.value = [nextJob, ...printJobs.value.filter((job) => job.id !== nextJob.id)];
  }

  function upsertDevice(nextDevice: Device) {
    devices.value = [nextDevice, ...devices.value.filter((device) => device.id !== nextDevice.id)];
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
    if (typeof window === "undefined") {
      return;
    }

    clearRemotePrintStatusSync();

    if (!hasQueuedRemotePrintJobs.value) {
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
    if (remotePrintStatusPromise) {
      return remotePrintStatusPromise;
    }

    const currentSession = authSession.value;
    if (!currentSession || !hasQueuedRemotePrintJobs.value) {
      clearRemotePrintStatusSync();
      return;
    }

    remotePrintStatusPromise = (async () => {
      try {
        const accessToken = currentSession.accessToken;
        const { printJobs: latestPrintJobs } = await fetchPrintJobs(accessToken);

        if (authSession.value?.accessToken !== accessToken) {
          return;
        }

        printJobs.value = latestPrintJobs;
        printerSyncError.value = "";
        remotePrintStatusBackoffMs = REMOTE_PRINT_STATUS_POLL_MS;
      } catch (error) {
        printerSyncError.value = getErrorMessage(error, "store.errors.syncPrintStatus");
        remotePrintStatusBackoffMs = Math.min(
          Math.max(remotePrintStatusBackoffMs * 2, REMOTE_PRINT_STATUS_POLL_MS),
          REMOTE_PRINT_STATUS_MAX_BACKOFF_MS,
        );
      } finally {
        remotePrintStatusPromise = null;

        if (hasQueuedRemotePrintJobs.value) {
          scheduleRemotePrintStatusSync();
        }
      }
    })();

    return remotePrintStatusPromise;
  }

  function restoreAnonymousWorkspace() {
    workspaceOwnerId.value = null;
    clearRemotePrintStatusSync();
    workspaceHydrating.value = true;
    applyWorkspaceState(readPersistedWorkspaceState());
    availablePlugins.value = [];
    adminPlugins.value = [];
    remoteSchedules.value = [];
    pluginError.value = "";
    pluginActionError.value = "";
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
    printerSyncError.value = "";
  }

  async function persistRemoteWorkspace() {
    if (remoteSavePromise) {
      remoteSavePending = true;
      return remoteSavePromise;
    }

    const currentSession = authSession.value;
    const currentUser = authUser.value;

    if (!currentSession || !currentUser || workspaceOwnerId.value !== currentUser.id) {
      return true;
    }

    workspaceSyncing.value = true;
    workspaceSyncError.value = "";
    remoteSavePromise = (async () => {
      try {
        do {
          remoteSavePending = false;
          const snapshot = JSON.parse(JSON.stringify(workspaceState.value)) as WorkspaceState;
          await saveWorkspaceStateWithApi(currentSession.accessToken, snapshot);
        } while (
          remoteSavePending &&
          authSession.value?.accessToken === currentSession.accessToken &&
          authUser.value?.id === currentUser.id &&
          workspaceOwnerId.value === currentUser.id
        );
        return true;
      } catch (error) {
        workspaceSyncError.value = getErrorMessage(error, "store.errors.syncWorkspace");
        return false;
      } finally {
        workspaceSyncing.value = false;
        remoteSavePromise = null;
      }
    })();

    return remoteSavePromise;
  }

  async function flushRemoteWorkspaceSave() {
    if (remoteSaveTimer) {
      window.clearTimeout(remoteSaveTimer);
      remoteSaveTimer = 0;
      return persistRemoteWorkspace();
    }

    return remoteSavePromise ?? true;
  }

  async function loadRemoteWorkspace() {
    const currentSession = authSession.value;
    const currentUser = authUser.value;

    if (!currentSession || !currentUser) {
      return false;
    }

    workspaceLoading.value = true;
    workspaceSyncError.value = "";
    workspaceHydrating.value = true;

    try {
      const state = await fetchWorkspaceStateWithApi(currentSession.accessToken);
      applyWorkspaceState(state);
      await loadLiveIntegrations(currentSession.accessToken);
      workspaceOwnerId.value = currentUser.id;
      return true;
    } catch (error) {
      workspaceSyncError.value = getErrorMessage(error, "store.errors.loadAccountData");
      return false;
    } finally {
      workspaceHydrating.value = false;
      workspaceLoading.value = false;
    }
  }

  async function loadLiveIntegrations(accessToken: string) {
    aiConfigLoading.value = true;
    pluginLoading.value = true;
    printerSyncError.value = "";
    pluginError.value = "";

    try {
      const [aiSummary, printerResponse, printJobResponse, pluginResponse, scheduleResponse] =
        await Promise.all([
          fetchAIConfigSummary(accessToken),
          fetchPrinters(accessToken),
          fetchPrintJobs(accessToken),
          fetchPlugins(accessToken),
          fetchPrintSchedules(accessToken),
        ]);

      applyAIConfig(aiSummary);
      devices.value = printerResponse.devices.filter((device) => device.status !== "offline");
      printJobs.value = printJobResponse.printJobs;
      availablePlugins.value = pluginResponse.plugins;
      remoteSchedules.value = scheduleResponse.schedules;

      if (authUser.value?.role === "admin") {
        const adminResponse = await fetchAdminPlugins(accessToken);
        adminPlugins.value = adminResponse.plugins;
      } else {
        adminPlugins.value = [];
      }
    } catch (error) {
      const message = getErrorMessage(error, "store.errors.loadIntegrations");
      aiConfigError.value = message;
      printerSyncError.value = message;
      pluginError.value = message;
    } finally {
      aiConfigLoading.value = false;
      pluginLoading.value = false;
    }
  }

  function showFlash(message: string, tone: "success" | "error" | "info" = "info") {
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
  }

  function selectConversation(conversationId: string) {
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
    const conversation = createEmptyConversation();

    conversations.value = [conversation, ...conversations.value];
    selectConversation(conversation.id);
    showFlashKey("store.flash.conversationCreated");
  }

  function deleteConversation(conversationId: string) {
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
    const currentConversation = ensureActiveConversation();

    updateConversation(currentConversation.id, (conversation) => ({
      ...conversation,
      draft: value,
      updatedAt: getNow(),
    }));
  }

  function saveCurrentDraft() {
    ensureActiveConversation();

    showFlashKey("store.flash.draftSaved", "success");
  }

  async function sendCurrentDraft() {
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
              })
            ).content
          : await generateReplyWithMockService({ prompt });
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
      generationError.value = getErrorMessage(error, "store.errors.generateReply");
      showFlash(generationError.value, "error");
      return false;
    } finally {
      isGenerating.value = false;
    }
  }

  async function regenerateLatestReply() {
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

    updateConversation(conversation.id, (current) => ({
      ...current,
      messages:
        current.messages.at(-1)?.role === "assistant"
          ? current.messages.slice(0, -1)
          : current.messages,
      updatedAt: getNow(),
    }));

    try {
      const refreshedConversation =
        conversations.value.find((item) => item.id === conversation.id) ?? conversation;
      const reply =
        isAuthenticated.value && authSession.value
          ? (
              await generateAIReply(authSession.value.accessToken, {
                messages: buildAIReplyMessages(refreshedConversation.messages),
              })
            ).content
          : await generateReplyWithMockService({ prompt: latestUserMessage.text });
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
      showFlashKey("store.flash.replyRegenerated", "success");
      return true;
    } catch (error) {
      generationError.value = getErrorMessage(error, "store.errors.regenerateReply");
      showFlash(generationError.value, "error");
      return false;
    } finally {
      isGenerating.value = false;
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
    window.setTimeout(() => {
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
    }, 500);
  }

  async function addPrintJob(title: string, content: string, source: string) {
    if (isCreatingPrint.value) {
      return null;
    }

    isCreatingPrint.value = true;
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
        upsertPrintJob(job);
        showFlashKey("store.flash.printQueuedDirectly", "success");
        return job;
      }

      const job = buildPrintJob(title, content, source);
      printJobs.value = [job, ...printJobs.value];

      showFlashKey("store.flash.printQueuedDirectly", "success");
      await maybeCompleteQueuedJob(job.id);

      return job;
    } catch (error) {
      const message = getErrorMessage(error, "store.errors.createPrint");
      showFlash(message, "error");
      return null;
    } finally {
      isCreatingPrint.value = false;
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
    const target = printJobs.value.find((job) => job.id === jobId);

    if (!target || target.status !== "pending") {
      return false;
    }

    if (isAuthenticated.value && authSession.value) {
      try {
        const submitted = await submitPrintJob(authSession.value.accessToken, jobId);
        upsertPrintJob(submitted);
        showFlashKey("store.flash.printQueued", "success");
        return true;
      } catch (error) {
        showFlash(getErrorMessage(error, "store.errors.submitPrint"), "error");
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
    showFlashKey("store.flash.printQueued", "success");
    await maybeCompleteQueuedJob(jobId);
    return true;
  }

  async function cancelPrint(jobId: string) {
    const target = printJobs.value.find((job) => job.id === jobId);

    if (!target || (target.status !== "pending" && target.status !== "queued")) {
      return false;
    }

    if (isAuthenticated.value && authSession.value) {
      try {
        const cancelled = await cancelPrintJob(authSession.value.accessToken, jobId);
        upsertPrintJob(cancelled);
        showFlashKey("store.flash.printCancelled", "success");
        return true;
      } catch (error) {
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
    showFlashKey("store.flash.printCancelled", "success");
    return true;
  }

  async function updatePrintDevice(jobId: string, deviceId: string) {
    if (isAuthenticated.value && authSession.value) {
      try {
        const updated = await updatePrintJobDeviceWithApi(authSession.value.accessToken, jobId, {
          printerBindingId: deviceId,
        });
        upsertPrintJob(updated);
        showFlashKey("store.flash.printDeviceUpdated", "success");
        return;
      } catch (error) {
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
    showFlashKey("store.flash.printDeviceUpdated", "success");
  }

  async function addDevice(options?: {
    name?: string;
    note?: string;
    deviceId?: string;
    setAsDefault?: boolean;
  }) {
    if (isAuthenticated.value && authSession.value) {
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
        upsertDevice(device);
        if (options?.setAsDefault || !defaultDeviceId.value) {
          defaultDeviceId.value = device.id;
        }
        showFlashKey("store.flash.deviceBound", "success");
        return device;
      } catch (error) {
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
      try {
        await deletePrinter(authSession.value.accessToken, deviceId);
      } catch (error) {
        showFlash(getErrorMessage(error, "store.errors.deleteDevice"), "error");
        return false;
      }

      const remainingDevices = devices.value.filter((device) => device.id !== deviceId);
      const fallbackDeviceId =
        defaultDeviceId.value === deviceId
          ? (remainingDevices.find((device) => device.status !== "offline")?.id ?? "")
          : defaultDeviceId.value;

      devices.value = remainingDevices;
      printJobs.value = printJobs.value.filter((job) => job.deviceId !== deviceId);
      remoteSchedules.value = remoteSchedules.value.filter(
        (schedule) => schedule.deviceId !== deviceId,
      );
      defaultDeviceId.value = fallbackDeviceId;
      showFlashKey("store.flash.deviceDeleted", "success");
      return true;
    }

    const remainingDevices = devices.value.filter((device) => device.id !== deviceId);
    const fallbackDeviceId =
      defaultDeviceId.value === deviceId ? (remainingDevices[0]?.id ?? "") : defaultDeviceId.value;

    devices.value = remainingDevices;
    defaultDeviceId.value = fallbackDeviceId;
    printJobs.value = printJobs.value.map((job) =>
      job.deviceId === deviceId
        ? {
            ...job,
            deviceId: fallbackDeviceId,
            updatedAt: getNow(),
          }
        : job,
    );
    schedules.value = schedules.value.map((schedule) =>
      schedule.deviceId === deviceId
        ? {
            ...schedule,
            deviceId: fallbackDeviceId,
          }
        : schedule,
    );
    showFlashKey(
      target.status === "pending" ? "store.flash.deviceRemoved" : "store.flash.deviceDeleted",
      "success",
    );
    return true;
  }

  function upsertRemoteSchedule(nextSchedule: PrintScheduleView) {
    remoteSchedules.value = [
      nextSchedule,
      ...remoteSchedules.value.filter((schedule) => schedule.id !== nextSchedule.id),
    ];
  }

  function upsertPlugin(nextPlugin: PluginDetails) {
    availablePlugins.value = [
      nextPlugin,
      ...availablePlugins.value.filter(
        (plugin) => plugin.installation.id !== nextPlugin.installation.id,
      ),
    ];
    if (isAdmin.value) {
      adminPlugins.value = [
        nextPlugin,
        ...adminPlugins.value.filter(
          (plugin) => plugin.installation.id !== nextPlugin.installation.id,
        ),
      ];
    }
  }

  async function toggleSchedule(scheduleId: string) {
    if (isAuthenticated.value && authSession.value) {
      try {
        const updated = await togglePrintSchedule(authSession.value.accessToken, scheduleId);
        upsertRemoteSchedule(updated);
        showFlashKey("store.flash.scheduleUpdated", "success");
        return true;
      } catch (error) {
        showFlash(getErrorMessage(error, "store.errors.updateSchedule"), "error");
        return false;
      }
    }

    schedules.value = schedules.value.map((schedule) =>
      schedule.id === scheduleId ? { ...schedule, enabled: !schedule.enabled } : schedule,
    );
    showFlashKey("store.flash.scheduleUpdated", "success");
    return true;
  }

  async function createSchedule(options?: {
    title?: string;
    source?: string;
    timeLabel?: string;
    deviceId?: string;
    pluginInstallationId?: string;
    frequencyType?: "daily" | "weekly";
    timezone?: string;
    hour?: number;
    minute?: number;
    weekdays?: number[];
    batchSize?: number;
  }) {
    const nextDeviceId = options?.deviceId || defaultDeviceId.value;
    const targetDevice = devices.value.find((device) => device.id === nextDeviceId);

    if (!targetDevice || targetDevice.status === "offline") {
      showFlashKey("store.errors.scheduleDeviceRequired", "error");
      return null;
    }

    if (isAuthenticated.value && authSession.value) {
      const pluginInstallationId = options?.pluginInstallationId?.trim() ?? "";
      if (!pluginInstallationId) {
        showFlashKey("store.errors.pluginSourceRequired", "error");
        return null;
      }

      try {
        const schedule = await createPrintSchedule(authSession.value.accessToken, {
          title: options?.title?.trim() || translate("store.labels.scheduleTitle"),
          pluginInstallationId,
          frequencyType: options?.frequencyType || "daily",
          timezone: options?.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone,
          hour: options?.hour ?? 19,
          minute: options?.minute ?? 30,
          weekdays: options?.weekdays ?? [],
          printPolicy: {
            batchSize: options?.batchSize ?? 1,
          },
          deviceId: nextDeviceId,
          enabled: true,
        });
        upsertRemoteSchedule(schedule);
        showFlashKey("store.flash.scheduleCreated", "success");
        return schedule;
      } catch (error) {
        showFlash(getErrorMessage(error, "store.errors.createSchedule"), "error");
        return null;
      }
    }

    const schedule: Schedule = {
      id: createId("schedule"),
      title: options?.title?.trim() || translate("store.labels.scheduleTitle"),
      source: options?.source?.trim() || translate("store.labels.scheduleManualSource"),
      timeLabel: options?.timeLabel?.trim() || translate("store.labels.scheduleTimeFallback"),
      deviceId: nextDeviceId,
      enabled: true,
    };

    schedules.value = [schedule, ...schedules.value];
    showFlashKey("store.flash.scheduleCreated", "success");
    return schedule;
  }

  async function updateScheduleDevice(scheduleId: string, deviceId: string) {
    const targetDevice = devices.value.find((device) => device.id === deviceId);

    if (!targetDevice || targetDevice.status === "offline") {
      showFlashKey("store.errors.scheduleDeviceRequired", "error");
      return;
    }

    if (isAuthenticated.value && authSession.value) {
      const current = remoteSchedules.value.find((schedule) => schedule.id === scheduleId);
      if (!current) {
        return;
      }

      try {
        const updated = await updatePrintSchedule(authSession.value.accessToken, scheduleId, {
          title: current.title,
          pluginInstallationId: current.pluginInstallationId,
          frequencyType: current.frequencyType,
          timezone: current.timezone,
          hour: current.hour,
          minute: current.minute,
          weekdays: current.weekdays,
          printPolicy: {
            batchSize: current.printPolicy.batchSize,
          },
          deviceId,
          enabled: current.enabled,
        });
        upsertRemoteSchedule(updated);
        showFlashKey("store.flash.scheduleDeviceUpdated", "success");
        return;
      } catch (error) {
        showFlash(getErrorMessage(error, "store.errors.updateSchedule"), "error");
        return;
      }
    }

    schedules.value = schedules.value.map((schedule) =>
      schedule.id === scheduleId ? { ...schedule, deviceId } : schedule,
    );
    showFlashKey("store.flash.scheduleDeviceUpdated", "success");
  }

  async function deleteSchedule(scheduleId: string) {
    if (isAuthenticated.value && authSession.value) {
      try {
        await deletePrintSchedule(authSession.value.accessToken, scheduleId);
        remoteSchedules.value = remoteSchedules.value.filter(
          (schedule) => schedule.id !== scheduleId,
        );
        showFlashKey("store.flash.scheduleDeleted", "success");
        return true;
      } catch (error) {
        showFlash(getErrorMessage(error, "store.errors.deleteSchedule"), "error");
        return false;
      }
    }

    schedules.value = schedules.value.filter((schedule) => schedule.id !== scheduleId);
    showFlashKey("store.flash.scheduleDeleted", "success");
    return true;
  }

  function toggleSourceConnection(sourceId: string) {
    const target = sources.value.find((source) => source.id === sourceId);

    if (!target) {
      return false;
    }

    const nextStatus = target.status === "connected" ? "disconnected" : "connected";
    sources.value = sources.value.map((source) =>
      source.id === sourceId
        ? {
            ...source,
            status: nextStatus,
            note:
              nextStatus === "connected"
                ? translate("store.labels.sourceConnectedNote")
                : translate("store.labels.sourceDisconnectedNote"),
          }
        : source,
    );
    showFlashKey(
      nextStatus === "connected" ? "store.flash.sourceConnected" : "store.flash.sourceDisconnected",
      "success",
    );
    return true;
  }

  async function uploadPlugin(file: File) {
    const current = authSession.value;

    if (!current) {
      pluginActionError.value = translate("store.errors.authRequired");
      return null;
    }

    pluginUploadLoading.value = true;
    pluginActionError.value = "";
    pluginUploadingName.value = file.name;

    try {
      const uploaded = await uploadPluginZip(current.accessToken, file);
      upsertPlugin(uploaded);
      if (isAdmin.value) {
        const adminResponse = await fetchAdminPlugins(current.accessToken);
        adminPlugins.value = adminResponse.plugins;
      }
      showFlashKey("store.flash.pluginUploaded", "success");
      return uploaded;
    } catch (error) {
      pluginActionError.value = getErrorMessage(error, "store.errors.uploadPlugin");
      showFlash(pluginActionError.value, "error");
      return null;
    } finally {
      pluginUploadLoading.value = false;
      pluginUploadingName.value = "";
    }
  }

  async function installPluginRepository(options: {
    repoUrl: string;
    repoRef?: string;
    repoSubdir?: string;
  }) {
    const current = authSession.value;

    if (!current) {
      pluginActionError.value = translate("store.errors.authRequired");
      return null;
    }

    pluginGitInstallLoading.value = true;
    pluginActionError.value = "";

    try {
      const installed = await installPluginFromGit(current.accessToken, {
        repoUrl: options.repoUrl.trim(),
        repoRef: options.repoRef?.trim() || undefined,
        repoSubdir: options.repoSubdir?.trim() || undefined,
      });
      upsertPlugin(installed);
      if (isAdmin.value) {
        const adminResponse = await fetchAdminPlugins(current.accessToken);
        adminPlugins.value = adminResponse.plugins;
      }
      showFlashKey("store.flash.pluginInstalledFromGit", "success");
      return installed;
    } catch (error) {
      pluginActionError.value = getErrorMessage(error, "store.errors.installPluginFromGit");
      showFlash(pluginActionError.value, "error");
      return null;
    } finally {
      pluginGitInstallLoading.value = false;
    }
  }

  async function disablePluginInstallation(installationId: string) {
    const current = authSession.value;

    if (!current) {
      pluginActionError.value = translate("store.errors.authRequired");
      return null;
    }

    pluginSaving.value = true;
    pluginSavingId.value = installationId;
    pluginActionError.value = "";

    try {
      const updated = await disablePlugin(current.accessToken, installationId);
      upsertPlugin(updated);
      adminPlugins.value = adminPlugins.value.map((plugin) =>
        plugin.installation.id === installationId ? updated : plugin,
      );
      availablePlugins.value = availablePlugins.value.map((plugin) =>
        plugin.installation.id === installationId
          ? {
              ...plugin,
              installation: updated.installation,
            }
          : plugin,
      );
      showFlashKey("store.flash.pluginDisabled", "success");
      return updated;
    } catch (error) {
      pluginActionError.value = getErrorMessage(error, "store.errors.disablePlugin");
      showFlash(pluginActionError.value, "error");
      return null;
    } finally {
      pluginSaving.value = false;
      pluginSavingId.value = "";
    }
  }

  async function testPluginConfiguration(
    installationId: string,
    config: Record<string, unknown>,
    secrets: Record<string, string>,
    enabled = true,
  ) {
    const current = authSession.value;

    if (!current) {
      pluginActionError.value = translate("store.errors.authRequired");
      return null;
    }

    pluginTestingId.value = installationId;
    pluginActionError.value = "";

    try {
      const result = await testPluginBinding(current.accessToken, installationId, {
        enabled,
        config,
        secrets,
      });
      showFlashKey(
        result.valid ? "store.flash.pluginTestPassed" : "store.flash.pluginTestFailed",
        result.valid ? "success" : "error",
      );
      return result;
    } catch (error) {
      pluginActionError.value = getErrorMessage(error, "store.errors.testPlugin");
      showFlash(pluginActionError.value, "error");
      return null;
    } finally {
      pluginTestingId.value = "";
    }
  }

  async function savePluginConfiguration(
    installationId: string,
    config: Record<string, unknown>,
    secrets: Record<string, string>,
    enabled = true,
  ) {
    const current = authSession.value;

    if (!current) {
      pluginActionError.value = translate("store.errors.authRequired");
      return null;
    }

    pluginSaving.value = true;
    pluginSavingId.value = installationId;
    pluginActionError.value = "";

    try {
      const updated = await savePluginBinding(current.accessToken, installationId, {
        enabled,
        config,
        secrets,
      });
      upsertPlugin(updated);
      showFlashKey(
        enabled ? "store.flash.pluginConfigEnabled" : "store.flash.pluginConfigSaved",
        "success",
      );
      return updated;
    } catch (error) {
      pluginActionError.value = getErrorMessage(error, "store.errors.savePluginConfiguration");
      showFlash(pluginActionError.value, "error");
      return null;
    } finally {
      pluginSaving.value = false;
      pluginSavingId.value = "";
    }
  }

  function setDefaultDevice(deviceId: string) {
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
    selectedTheme.value = theme;
    scheduleRemoteWorkspaceSave();
    showFlashKey("store.flash.themeUpdated");
  }

  function setLocale(nextLocale: LocalePreference) {
    localePreference.value = normalizeLocalePreference(nextLocale);
    scheduleRemoteWorkspaceSave();
    showFlash(translate("store.flash.localeUpdated"));
  }

  function setSendConfirmation(enabled: boolean) {
    sendConfirmationEnabled.value = false;
    scheduleRemoteWorkspaceSave();
    if (enabled) {
      showFlashKey("store.flash.sendConfirmationEnabled");
    }
  }

  function setTutorialTabEnabled(enabled: boolean) {
    tutorialTabEnabled.value = enabled;
    scheduleRemoteWorkspaceSave();
    showFlashKey(enabled ? "store.flash.tutorialTabShown" : "store.flash.tutorialTabHidden");
  }

  function setLoginProtection(enabled: boolean) {
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
    const current = authSession.value;

    if (!current) {
      aiConfigError.value = translate("store.errors.authRequired");
      return false;
    }

    aiConfigSaving.value = true;
    aiConfigError.value = "";

    try {
      const summary = await saveAIConfig(current.accessToken, config);
      applyAIConfig(summary);
      showFlashKey("store.flash.aiConfigSaved", "success");
      return true;
    } catch (error) {
      aiConfigError.value = getErrorMessage(error, "store.errors.saveAIConfig");
      showFlash(aiConfigError.value, "error");
      return false;
    } finally {
      aiConfigSaving.value = false;
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
    if (remoteSaveTimer) {
      window.clearTimeout(remoteSaveTimer);
      remoteSaveTimer = 0;
    }

    authUser.value = null;
    authSession.value = null;
    authError.value = "";
    feedbackError.value = "";
    accountCreationError.value = "";
    aiConfigError.value = "";
    pluginError.value = "";
    pluginActionError.value = "";
    workspaceOwnerId.value = null;
  }

  async function refreshSessionIfNeeded() {
    const current = authSession.value;

    if (!current) {
      return false;
    }

    const expiresAt = new Date(current.accessTokenExpiresAt).getTime();

    if (Number.isNaN(expiresAt)) {
      clearAuthState();
      return false;
    }

    if (expiresAt > Date.now() + 30_000) {
      return true;
    }

    try {
      const refreshed = await refreshAuthSession(current.refreshToken);
      setAuthState(refreshed.user, refreshed.session);
      return true;
    } catch (error) {
      clearAuthState();

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

    try {
      const sessionReady = await refreshSessionIfNeeded();

      if (!sessionReady || !authSession.value) {
        return false;
      }

      if (!authUser.value) {
        const user = await fetchCurrentUser(authSession.value.accessToken);
        authUser.value = user;
      }

      const loaded = await loadRemoteWorkspace();
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
      clearAuthState();
      restoreAnonymousWorkspace();
      authError.value = getErrorMessage(error, "store.errors.authRequired");
      return false;
    } finally {
      authBootstrapping.value = false;
    }
  }

  async function login(email: string, password: string) {
    authLoading.value = true;
    authError.value = "";

    try {
      const result = await loginWithApi({ email, password });
      setAuthState(result.user, result.session);
      const loaded = await loadRemoteWorkspace();

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
      authError.value = getErrorMessage(error, "store.errors.login");
      postLoginTutorialOpen.value = false;
      showFlash(authError.value, "error");
      return false;
    } finally {
      authLoading.value = false;
    }
  }

  async function changePassword(currentPassword: string, newPassword: string) {
    const current = authSession.value;

    if (!current) {
      authError.value = translate("store.errors.authRequired");
      return false;
    }

    passwordChangeLoading.value = true;
    authError.value = "";

    try {
      await changePasswordWithApi({
        accessToken: current.accessToken,
        currentPassword,
        newPassword,
      });
      clearAuthState();
      restoreAnonymousWorkspace();
      showFlashKey("store.flash.passwordUpdated", "success");
      return true;
    } catch (error) {
      authError.value = getErrorMessage(error, "store.errors.changePassword");
      showFlash(authError.value, "error");
      return false;
    } finally {
      passwordChangeLoading.value = false;
    }
  }

  async function submitFeedback(content: string) {
    const current = authSession.value;
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
      showFlashKey("store.flash.feedbackSubmitted", "success");
      return true;
    } catch (error) {
      feedbackError.value = getErrorMessage(error, "store.errors.submitFeedback");
      showFlash(feedbackError.value, "error");
      return false;
    } finally {
      feedbackSubmitting.value = false;
    }
  }

  async function logout() {
    const current = authSession.value;

    await flushRemoteWorkspaceSave();

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

    clearAuthState();
    restoreAnonymousWorkspace();
    postLoginTutorialOpen.value = false;
    showFlashKey("store.flash.loggedOut");
  }

  function closePostLoginTutorial() {
    postLoginTutorialOpen.value = false;
  }

  async function createAccount(email: string, name: string, password: string) {
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
      showFlashKey("store.flash.accountCreated", "success");
      return true;
    } catch (error) {
      accountCreationError.value = getErrorMessage(error, "store.errors.createAccount");
      showFlash(accountCreationError.value, "error");
      return false;
    } finally {
      accountCreationLoading.value = false;
    }
  }

  function formatPrintTime(iso: string) {
    void effectiveLocale.value;
    return formatRelativeTimestamp(iso);
  }

  function getDeviceName(deviceId: string) {
    void effectiveLocale.value;
    return deviceMap.value[deviceId]?.name ?? translate("common.labels.notSet");
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
    accountCreationLoading,
    accountCreationError,
    aiConfigSummary,
    aiConfigLoading,
    aiConfigSaving,
    aiConfigError,
    printerSyncError,
    pluginLoading,
    pluginSaving,
    pluginUploadLoading,
    pluginGitInstallLoading,
    pluginError,
    pluginActionError,
    pluginTestingId,
    pluginSavingId,
    pluginUploadingName,
    devices,
    conversations,
    activeConversationId,
    printJobs,
    schedules,
    sources,
    availablePlugins,
    adminPlugins,
    remoteSchedules,
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
    flashMessage,
    flashTone,
    postLoginTutorialOpen,
    activeConversation,
    conversationMessages,
    selectedConversationMessages,
    defaultDevice,
    pendingPrintJobs,
    recentPrintJobs,
    activeSchedules,
    activeSources,
    connectedDevicesCount,
    enabledSchedulesCount,
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
    confirmPrint,
    cancelPrint,
    updatePrintDevice,
    addDevice,
    removeDevice,
    toggleSchedule,
    createSchedule,
    updateScheduleDevice,
    deleteSchedule,
    toggleSourceConnection,
    uploadPlugin,
    installPluginRepository,
    disablePluginInstallation,
    testPluginConfiguration,
    savePluginConfiguration,
    setDefaultDevice,
    setTheme,
    setLocale,
    showFlashKey,
    setSendConfirmation,
    setTutorialTabEnabled,
    setLoginProtection,
    saveAIServiceConfig,
    initializeAuth,
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
