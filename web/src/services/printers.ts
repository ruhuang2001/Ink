import { request } from "@/services/http";
import type { ContentBlock } from "@/types/plugins";
import type { Device, PrintJob } from "@/types/workspace";

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

export async function fetchPrintJobs(accessToken: string) {
  return request<{ printJobs: PrintJob[] }>("/api/v1/print-jobs", {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
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
