import { request } from "@/services/http";
import type { User, WorkspaceState } from "@/types/workspace";

export interface CreateUserPayload {
  email: string;
  name: string;
  password: string;
}

export async function fetchWorkspaceStateWithApi(accessToken: string) {
  return request<WorkspaceState>("/api/v1/workspace", {
    method: "GET",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export async function saveWorkspaceStateWithApi(accessToken: string, state: WorkspaceState) {
  return request<WorkspaceState>("/api/v1/workspace", {
    method: "PUT",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(state),
  });
}

export async function createUserWithApi(accessToken: string, payload: CreateUserPayload) {
  const response = await request<{ user: User }>("/api/v1/admin/users", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify(payload),
  });

  return response.user;
}
