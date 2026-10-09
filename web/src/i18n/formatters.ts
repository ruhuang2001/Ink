import type { FrequencyType } from "@/types/plugins";
import type {
  DeviceStatus,
  LocaleCode,
  PrintStatus,
  SourceConnectionStatus,
  UserRole,
} from "@/types/workspace";

import { getI18nLocale, translate } from "./index";

function createTimeFormatter(locale: LocaleCode) {
  return new Intl.DateTimeFormat(locale, {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function createDateTimeFormatter(locale: LocaleCode) {
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function isSameDay(left: Date, right: Date) {
  return (
    left.getFullYear() === right.getFullYear() &&
    left.getMonth() === right.getMonth() &&
    left.getDate() === right.getDate()
  );
}

export function formatRelativeTimestampForLocale(
  iso: string,
  locale: LocaleCode = getI18nLocale(),
) {
  const target = new Date(iso);
  const now = new Date();
  const diffMs = now.getTime() - target.getTime();
  const diffMinutes = Math.max(0, Math.floor(diffMs / 60_000));
  const timeFormatter = createTimeFormatter(locale);

  if (diffMinutes < 1) {
    return translate("time.justNow");
  }

  if (diffMinutes < 60) {
    return translate("time.minutesAgo", { count: diffMinutes });
  }

  if (isSameDay(target, now)) {
    return translate("time.todayAt", { time: timeFormatter.format(target) });
  }

  const yesterday = new Date(now);
  yesterday.setDate(yesterday.getDate() - 1);

  if (isSameDay(target, yesterday)) {
    return translate("time.yesterdayAt", { time: timeFormatter.format(target) });
  }

  return createDateTimeFormatter(locale).format(target);
}

export function formatWeekdayList(weekdays: number[], locale: LocaleCode = getI18nLocale()) {
  const separator = translate("schedule.listSeparator");
  return weekdays
    .map((weekday) => translate(`weekdays.short.${weekday}`, { locale }))
    .join(separator);
}

export function formatScheduleLabelForLocale(
  frequencyType: FrequencyType,
  hour: number,
  minute: number,
  weekdays: number[],
  locale: LocaleCode = getI18nLocale(),
  fallback = "",
) {
  const now = new Date();
  const sample = new Date(now.getFullYear(), now.getMonth(), now.getDate(), hour, minute, 0, 0);
  const time = createTimeFormatter(locale).format(sample);

  if (frequencyType === "daily") {
    return translate("schedule.everyDay", { time });
  }

  if (frequencyType === "weekly" && weekdays.length > 0) {
    return translate("schedule.everyWeek", {
      days: formatWeekdayList(weekdays, locale),
      time,
    });
  }

  return fallback || time;
}

export function getDeviceStatusLabelForLocale(status: DeviceStatus) {
  return translate(`statuses.device.${status}`);
}

export function getPrintStatusLabelForLocale(status: PrintStatus) {
  return translate(`statuses.print.${status}`);
}

export function getSourceStatusLabelForLocale(status: SourceConnectionStatus) {
  return translate(`statuses.source.${status}`);
}

export function getPluginInstallationStatusLabelForLocale(status: string) {
  return translate(`statuses.pluginInstallation.${status}`) || status;
}

export function getUserRoleLabelForLocale(role: UserRole) {
  return translate(`statuses.userRole.${role}`);
}
