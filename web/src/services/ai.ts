import { request } from "@/services/http";

export interface AIConfigSummary {
  bound: boolean;
  providerName: string;
  providerType: string;
  baseUrl: string;
  model: string;
  keyConfigured: boolean;
  updatedAt?: string;
}

export interface SaveAIConfigPayload {
  providerName: string;
  providerType: string;
  baseUrl: string;
  model: string;
  apiKey: string;
}

export interface AIReplyMessage {
  role: "system" | "user" | "assistant";
  content: string;
}

export interface AIReplyResult {
  content: string;
  model: string;
  providerName: string;
}

export async function fetchAIConfigSummary(accessToken: string) {
  return request<AIConfigSummary>("/api/v1/ai/config", {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export async function saveAIConfig(accessToken: string, payload: SaveAIConfigPayload) {
  return request<AIConfigSummary>("/api/v1/admin/ai/config", {
    method: "PUT",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(payload),
  });
}

export async function generateAIReply(
  accessToken: string,
  payload: { messages: AIReplyMessage[] },
) {
  return request<AIReplyResult>("/api/v1/ai/reply", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(payload),
    timeoutMs: 60000,
  });
}
