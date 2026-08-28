export type PluginInstallationStatus = "installing" | "ready" | "failed" | "disabled";
export type PluginBindingStatus = "connected" | "disconnected" | "error";
export type PluginNetworkPermissionMode = "none" | "declared_hosts" | "all";
export type PluginFieldType =
  | "text"
  | "secret"
  | "textarea"
  | "url"
  | "number"
  | "select"
  | "checkbox";
export type FrequencyType = "daily" | "weekly";

export interface PluginFieldOption {
  label: string;
  value: string;
}

export interface PluginFieldSpec {
  key: string;
  label: string;
  type: PluginFieldType;
  required: boolean;
  description?: string;
  defaultValue?: string | number | boolean;
  options?: PluginFieldOption[];
}

export interface PluginPermissions {
  network?: {
    mode: PluginNetworkPermissionMode;
    hosts?: string[];
  };
  filesystem?: {
    temp?: boolean;
    cache?: boolean;
  };
  installScripts?: boolean;
}

export interface PluginManifest {
  schemaVersion: number;
  kind: "source";
  pluginKey: string;
  name: string;
  version: string;
  description: string;
  runtime: {
    type: "node" | "python";
  };
  fetchPolicy: {
    type: "fixed_interval";
    minutes: number;
  };
  entrypoints: {
    validate: {
      command: string[];
    };
    fetch: {
      command: string[];
    };
  };
  permissions?: PluginPermissions;
  workspaceConfigSchema: PluginFieldSpec[];
}

export type PluginBlockType =
  | "heading"
  | "paragraph"
  | "image"
  | "link"
  | "divider"
  | "list"
  | "quote";

export interface ContentBlock {
  type: PluginBlockType;
  level?: number;
  text?: string;
  url?: string;
  alt?: string;
  items?: string[];
  style?: "bullet" | "ordered";
}

export interface PluginInstallationSummary {
  id: string;
  pluginKey: string;
  sourceType: "upload" | "git";
  displayName: string;
  version: string;
  runtimeType: "node" | "python";
  status: PluginInstallationStatus;
  lastError?: string;
  description?: string;
  repoUrl?: string;
  repoRef?: string;
  repoCommitSha?: string;
  repoSubdir?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface PluginBindingSummary {
  id: string;
  enabled: boolean;
  status: PluginBindingStatus;
  config: Record<string, unknown>;
  lastValidatedAt?: string;
  lastError?: string;
  nextFetchAt?: string;
  lastFetchAt?: string;
  lastFetchError?: string;
}

export interface PluginDetails {
  installation: PluginInstallationSummary;
  manifest: PluginManifest;
  binding?: PluginBindingSummary;
}

export interface PluginValidationError {
  field: string;
  message: string;
}

export interface PluginValidationResult {
  valid: boolean;
  errors?: PluginValidationError[];
}

export interface PrintScheduleView {
  id: string;
  title: string;
  pluginInstallationId: string;
  pluginBindingId: string;
  pluginDisplayName: string;
  frequencyType: FrequencyType;
  timezone: string;
  hour: number;
  minute: number;
  weekdays: number[];
  printPolicy: {
    batchSize: number;
  };
  deviceId: string;
  enabled: boolean;
  nextRunAt?: string;
  lastRunAt?: string;
  lastError?: string;
  timeLabel: string;
  sourceLabel: string;
}
