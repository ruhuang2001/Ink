import { request } from "@/services/http";
import type { PluginDetails, PluginValidationResult, PrintScheduleView } from "@/types/plugins";

export interface PluginBindingPayload {
  enabled: boolean;
  config: Record<string, unknown>;
  secrets: Record<string, string>;
}

export interface GitPluginInstallPayload {
  repoUrl: string;
  repoRef?: string;
  repoSubdir?: string;
}

export interface PrintSchedulePayload {
  title: string;
  pluginInstallationId: string;
  frequencyType: "daily" | "weekly";
  timezone: string;
  hour: number;
  minute: number;
  weekdays: number[];
  printPolicy: {
    batchSize: number;
  };
  deviceId: string;
  enabled: boolean;
}

export interface ManualPluginFetchResult {
  fetchedCount: number;
  ingestedCount: number;
  inboxItemIds: string[];
  cursorAdvanced: boolean;
}

export interface ManualPrintScheduleRunResult {
  printedCount: number;
  failedCount: number;
  skippedCount: number;
  printJobIds: string[];
}

export async function fetchAdminPlugins(accessToken: string) {
  return request<{ plugins: PluginDetails[] }>("/api/v1/admin/plugins", {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export async function uploadPluginZip(accessToken: string, file: File) {
  const formData = new FormData();
  formData.set("file", file);

  const response = await request<{ plugin: PluginDetails }>("/api/v1/admin/plugins/upload", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    body: formData,
    timeoutMs: 180000,
  });

  return response.plugin;
}

export async function installPluginFromGit(accessToken: string, payload: GitPluginInstallPayload) {
  const response = await request<{ plugin: PluginDetails }>(
    "/api/v1/admin/plugins/install-from-git",
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify(payload),
      timeoutMs: 180000,
    },
  );

  return response.plugin;
}

export async function disablePlugin(accessToken: string, installationId: string) {
  const response = await request<{ plugin: PluginDetails }>(
    `/api/v1/admin/plugins/${installationId}/disable`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify({}),
      timeoutMs: 120000,
    },
  );

  return response.plugin;
}

export async function fetchPlugins(accessToken: string) {
  return request<{ plugins: PluginDetails[] }>("/api/v1/plugins", {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export async function fetchPlugin(accessToken: string, installationId: string) {
  const response = await request<{ plugin: PluginDetails }>(`/api/v1/plugins/${installationId}`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });

  return response.plugin;
}

export async function savePluginBinding(
  accessToken: string,
  installationId: string,
  payload: PluginBindingPayload,
) {
  const response = await request<{ plugin: PluginDetails }>(
    `/api/v1/plugins/${installationId}/binding`,
    {
      method: "PUT",
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify(payload),
      timeoutMs: 60000,
    },
  );

  return response.plugin;
}

export async function testPluginBinding(
  accessToken: string,
  installationId: string,
  payload: PluginBindingPayload,
) {
  const response = await request<{ result: PluginValidationResult }>(
    `/api/v1/plugins/${installationId}/test`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify(payload),
      timeoutMs: 60000,
    },
  );

  return response.result;
}

export async function fetchPrintSchedules(accessToken: string) {
  return request<{ schedules: PrintScheduleView[] }>("/api/v1/print-schedules", {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export async function createPrintSchedule(accessToken: string, payload: PrintSchedulePayload) {
  const response = await request<{ schedule: PrintScheduleView }>("/api/v1/print-schedules", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(payload),
  });

  return response.schedule;
}

export async function updatePrintSchedule(
  accessToken: string,
  scheduleId: string,
  payload: PrintSchedulePayload,
) {
  const response = await request<{ schedule: PrintScheduleView }>(
    `/api/v1/print-schedules/${scheduleId}`,
    {
      method: "PUT",
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify(payload),
    },
  );

  return response.schedule;
}

export async function togglePrintSchedule(accessToken: string, scheduleId: string) {
  const response = await request<{ schedule: PrintScheduleView }>(
    `/api/v1/print-schedules/${scheduleId}/toggle`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify({}),
    },
  );

  return response.schedule;
}

export async function deletePrintSchedule(accessToken: string, scheduleId: string) {
  return request<void>(`/api/v1/print-schedules/${scheduleId}`, {
    method: "DELETE",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export async function runPluginFetch(accessToken: string, installationId: string) {
  const response = await request<{ result: ManualPluginFetchResult }>(
    `/api/v1/plugins/${installationId}/run`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify({}),
      timeoutMs: 60000,
    },
  );

  return response.result;
}

export async function runPrintSchedule(accessToken: string, scheduleId: string) {
  const response = await request<{ result: ManualPrintScheduleRunResult }>(
    `/api/v1/print-schedules/${scheduleId}/run`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
      body: JSON.stringify({}),
      timeoutMs: 120000,
    },
  );

  return response.result;
}
