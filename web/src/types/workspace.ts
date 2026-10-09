export type DeviceStatus = "connected" | "pending" | "offline";
export type PrintStatus = "pending" | "queued" | "completed" | "failed" | "cancelled";
export type ConversationMessageRole = "user" | "assistant";
export type ThemeMode = "light" | "dark" | "system";
export type SourceConnectionStatus = "connected" | "disconnected" | "error";
export type UserRole = "admin" | "member";
export type LocaleCode = "zh-CN" | "en-US";
export type LocalePreference = "system" | LocaleCode;

export interface User {
  id: string;
  email: string;
  name: string;
  role: UserRole;
}

export interface AuthSession {
  accessToken: string;
  refreshToken: string;
  accessTokenExpiresAt: string;
}

export interface Device {
  id: string;
  name: string;
  status: DeviceStatus;
  note: string;
}

export interface ConversationMessage {
  id: string;
  role: ConversationMessageRole;
  text: string;
  createdAt: string;
}

export interface Conversation {
  id: string;
  title: string;
  preview: string;
  updatedAt: string;
  draft: string;
  messages: ConversationMessage[];
}

export interface PrintJob {
  id: string;
  title: string;
  source: string;
  deviceId: string;
  status: PrintStatus;
  createdAt: string;
  updatedAt: string;
  content: string;
  errorMessage?: string;
}

export type PrintJobSummary = Omit<PrintJob, "content">;

export interface PrintJobCounts {
  pending: number;
  queued: number;
  completed: number;
  failed: number;
  cancelled: number;
  todayCompleted: number;
}

export interface Schedule {
  id: string;
  title: string;
  source: string;
  timeLabel: string;
  deviceId: string;
  enabled: boolean;
}

export interface SourceConnection {
  id: string;
  name: string;
  type: string;
  note: string;
  status: SourceConnectionStatus;
}

export interface Preferences {
  loginProtectionEnabled: boolean;
  sendConfirmationEnabled: boolean;
  tutorialTabEnabled: boolean;
  theme: ThemeMode;
  defaultDeviceId: string;
  locale: LocalePreference;
}

export interface ServiceBinding {
  providerName: string | null;
  modelName: string;
  bound: boolean;
}

export interface WorkspaceState {
  revision?: number;
  devices: Device[];
  conversations: Conversation[];
  activeConversationId: string;
  printJobs: PrintJob[];
  schedules: Schedule[];
  sources: SourceConnection[];
  preferences: Preferences;
  serviceBinding: ServiceBinding;
}

export interface PersistedWorkspaceState extends WorkspaceState {
  authUser: User | null;
}
