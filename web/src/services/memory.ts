import { request } from "@/services/http";
import type { MemorySettings, UpdateMemorySettingsInput } from "@/types/memory";

export async function getMemorySettings(accessToken: string): Promise<MemorySettings> {
  return await request<MemorySettings>("/api/v1/memory/settings", {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export async function updateMemorySettings(
  accessToken: string,
  input: UpdateMemorySettingsInput,
): Promise<MemorySettings> {
  return await request<MemorySettings>("/api/v1/admin/memory/settings", {
    method: "PUT",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(input),
  });
}
