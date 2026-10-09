import { translate } from "@/i18n";
import {
  formatRelativeTimestampForLocale,
  getDeviceStatusLabelForLocale,
  getPluginInstallationStatusLabelForLocale,
  getPrintStatusLabelForLocale,
  getSourceStatusLabelForLocale,
  getUserRoleLabelForLocale,
} from "@/i18n/formatters";
import type { PluginDetails, PluginInstallationStatus } from "@/types/plugins";
import type {
  DeviceStatus,
  PrintStatus,
  SourceConnectionStatus,
  ThemeMode,
  UserRole,
} from "@/types/workspace";

export type ResolvedThemeMode = Exclude<ThemeMode, "system">;
type BadgeTone = "success" | "warning" | "danger" | "neutral";
type SummaryTone = "green" | "amber" | "stone" | "neutral";

function getInsetBadgeClass(tone: BadgeTone) {
  switch (tone) {
    case "success":
      return "bg-emerald-50 text-emerald-700 ring-1 ring-emerald-600/20 ring-inset";
    case "warning":
      return "bg-amber-50 text-amber-700 ring-1 ring-amber-600/20 ring-inset";
    case "danger":
      return "bg-rose-50 text-rose-700 ring-1 ring-rose-600/20 ring-inset";
    default:
      return "bg-stone-100 text-stone-700 ring-1 ring-stone-500/10 ring-inset";
  }
}

function getRoleBadgeClass(tone: "admin" | "member") {
  if (tone === "admin") {
    return "bg-amber-100 text-amber-800";
  }

  return "bg-stone-100 text-stone-800";
}

export function createId(prefix: string) {
  return `${prefix}-${Math.random().toString(36).slice(2, 10)}`;
}

export function formatRelativeTimestamp(iso: string) {
  return formatRelativeTimestampForLocale(iso);
}

export function isSameDay(left: Date, right: Date) {
  return (
    left.getFullYear() === right.getFullYear() &&
    left.getMonth() === right.getMonth() &&
    left.getDate() === right.getDate()
  );
}

export function getDeviceStatusLabel(status: DeviceStatus) {
  return getDeviceStatusLabelForLocale(status);
}

export function getDeviceStatusBadgeClass(status: DeviceStatus) {
  if (status === "connected") {
    return getInsetBadgeClass("success");
  }

  if (status === "pending") {
    return getInsetBadgeClass("warning");
  }

  return getInsetBadgeClass("neutral");
}

export function getPrintStatusLabel(status: PrintStatus) {
  return getPrintStatusLabelForLocale(status);
}

export function getPrintStatusBadgeClass(status: PrintStatus) {
  if (status === "completed") {
    return getInsetBadgeClass("success");
  }

  if (status === "queued") {
    return getInsetBadgeClass("warning");
  }

  if (status === "failed") {
    return getInsetBadgeClass("danger");
  }

  return getInsetBadgeClass("neutral");
}

export function getSourceStatusLabel(status: SourceConnectionStatus) {
  return getSourceStatusLabelForLocale(status);
}

export function getSourceStatusBadgeClass(status: SourceConnectionStatus) {
  if (status === "connected") {
    return getInsetBadgeClass("success");
  }

  if (status === "error") {
    return getInsetBadgeClass("danger");
  }

  return getInsetBadgeClass("neutral");
}

export function getPluginInstallationStatusLabel(status: PluginInstallationStatus) {
  return getPluginInstallationStatusLabelForLocale(status) || status;
}

export function getPluginInstallationStatusBadgeClass(status: PluginInstallationStatus) {
  switch (status) {
    case "ready":
      return getInsetBadgeClass("success");
    case "failed":
      return getInsetBadgeClass("danger");
    case "disabled":
      return getInsetBadgeClass("neutral");
    default:
      return getInsetBadgeClass("warning");
  }
}

export function getPluginBindingStatusLabel(plugin: PluginDetails) {
  if (plugin.installation.status === "disabled") {
    return translate("statuses.pluginBinding.disabled");
  }

  if (!plugin.binding?.enabled) {
    return translate("statuses.pluginBinding.disconnected");
  }

  return getSourceStatusLabel(plugin.binding.status);
}

export function getPluginBindingStatusBadgeClass(plugin: PluginDetails) {
  if (plugin.installation.status === "disabled" || !plugin.binding?.enabled) {
    return getInsetBadgeClass("neutral");
  }

  return plugin.binding.status === "error"
    ? getInsetBadgeClass("danger")
    : getInsetBadgeClass("success");
}

export function getUserRoleLabel(role: UserRole) {
  return getUserRoleLabelForLocale(role);
}

export function getUserRoleBadgeClass(role: UserRole) {
  return getRoleBadgeClass(role);
}

export function getServiceBindingStatusBadgeClass(bound: boolean) {
  return getSourceStatusBadgeClass(bound ? "connected" : "disconnected");
}

export function getSummaryProgressClass(tone: SummaryTone | string) {
  switch (tone) {
    case "green":
      return "bg-emerald-500";
    case "amber":
      return "bg-amber-500";
    case "stone":
      return "bg-stone-400";
    default:
      return "bg-stone-800";
  }
}

export function normalizeThemeMode(theme: unknown): ThemeMode {
  if (theme === "dark" || theme === "light" || theme === "system") {
    return theme;
  }

  if (theme === "soft") {
    return "light";
  }

  return "light";
}

export function resolveThemeMode(theme: ThemeMode, prefersDark: boolean): ResolvedThemeMode {
  if (theme === "system") {
    return prefersDark ? "dark" : "light";
  }

  return theme;
}
