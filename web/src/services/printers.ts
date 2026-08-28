import { AuthApiError } from "@/services/auth";
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

interface ApiErrorResponse {
  code?: string;
  message?: string;
  requestId?: string;
}

async function request<T>(input: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Content-Type", "application/json");

  let response: Response;
  try {
    response = await fetch(input, {
      ...init,
      headers,
    });
  } catch (error) {
    throw new AuthApiError(
      0,
      "network_error",
      error instanceof Error
        ? `网络异常，请检查连接后重试。${error.message ? ` (${error.message})` : ""}`
        : "网络异常，请检查连接后重试。",
    );
  }

  if (!response.ok) {
    let errorPayload: ApiErrorResponse | null = null;

    try {
      errorPayload = (await response.json()) as ApiErrorResponse;
    } catch {
      errorPayload = null;
    }

    throw new AuthApiError(
      response.status,
      errorPayload?.code ?? "request_failed",
      errorPayload?.message ?? "请求失败，请稍后重试。",
      errorPayload?.requestId,
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
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
