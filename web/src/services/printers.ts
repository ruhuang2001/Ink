import { request } from "@/services/http";
import type { ContentBlock } from "@/types/plugins";
import type { Device, PrintJob, PrintJobCounts, PrintJobSummary } from "@/types/workspace";

export interface BindPrinterPayload {
  name: string;
  note: string;
  deviceId: string;
}

export interface CreatePrintJobPayload {
  title: string;
  source: string;
  content: string;
  printerBindingId: string;
  submitImmediately: boolean;
}

interface UpdatePrintJobDevicePayload {
  printerBindingId: string;
}

export async function fetchPrinters(accessToken: string) {
  return request<{ devices: Device[] }>("/api/v1/printers", {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export async function bindPrinter(accessToken: string, payload: BindPrinterPayload) {
  const response = await request<{ device: Device }>("/api/v1/printers/bind", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(payload),
  });

  return response.device;
}

export async function deletePrinter(accessToken: string, printerId: string) {
  await request<void>(`/api/v1/printers/${printerId}`, {
    method: "DELETE",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export interface PrintJobPage {
  printJobs: PrintJobSummary[];
  nextCursor: string | null;
}

export type PrintJobListStatus = "active" | "history" | "all";

export async function fetchPrintJobs(
  accessToken: string,
  options: { status?: PrintJobListStatus; limit?: number; cursor?: string } = {},
) {
  const query = new URLSearchParams({
    status: options.status ?? "all",
    limit: String(options.limit ?? 20),
  });
  if (options.cursor) query.set("cursor", options.cursor);
  return request<PrintJobPage>(`/api/v1/print-jobs?${query}`, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export async function fetchPrintJob(accessToken: string, jobId: string) {
  const response = await request<{ printJob: PrintJob }>(
    `/api/v1/print-jobs/${encodeURIComponent(jobId)}`,
    { headers: { Authorization: `Bearer ${accessToken}` } },
  );
  return response.printJob;
}

export async function fetchPrintJobStatuses(accessToken: string, ids: string[], since: string) {
  const query = new URLSearchParams({ since });
  if (ids.length) query.set("ids", ids.join(","));
  return request<{
    printJobs: Pick<PrintJobSummary, "id" | "status" | "updatedAt" | "deviceId">[];
    counts: PrintJobCounts;
    latestJobId: string | null;
  }>(`/api/v1/print-jobs/status?${query}`, {
    headers: { Authorization: `Bearer ${accessToken}` },
  });
}

export async function createPrintJob(accessToken: string, payload: CreatePrintJobPayload) {
  const response = await request<{ printJob: PrintJob }>("/api/v1/print-jobs", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(payload),
    timeoutMs: 120000,
  });

  return response.printJob;
}

export async function renderPrintPreview(
  accessToken: string,
  payload: Pick<CreatePrintJobPayload, "title" | "content">,
) {
  const response = await request<{ image: string }>("/api/v1/print-preview", {
    method: "POST",
    headers: { Authorization: `Bearer ${accessToken}` },
    body: JSON.stringify(payload),
  });
  return response.image;
}

export async function renderBlocksPrintPreview(
  accessToken: string,
  payload: { title: string; blocks: ContentBlock[] },
) {
  const response = await request<{ image: string }>("/api/v1/print-preview", {
    method: "POST",
    headers: { Authorization: `Bearer ${accessToken}` },
    body: JSON.stringify(payload),
  });
  return response.image;
}

export async function submitPrintJob(accessToken: string, jobId: string) {
  const response = await request<{ printJob: PrintJob }>(`/api/v1/print-jobs/${jobId}/submit`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify({}),
    timeoutMs: 120000,
  });

  return response.printJob;
}

export async function cancelPrintJob(accessToken: string, jobId: string) {
  const response = await request<{ printJob: PrintJob }>(`/api/v1/print-jobs/${jobId}/cancel`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify({}),
  });

  return response.printJob;
}

export async function updatePrintJobDevice(
  accessToken: string,
  jobId: string,
  payload: UpdatePrintJobDevicePayload,
) {
  const response = await request<{ printJob: PrintJob }>(`/api/v1/print-jobs/${jobId}/device`, {
    method: "PUT",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(payload),
  });

  return response.printJob;
}
