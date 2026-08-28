import { request } from "@/services/http";

export async function submitFeedbackToAdmin(accessToken: string, content: string) {
  await request<void>("/api/v1/feedback/print", {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    body: JSON.stringify({ content }),
  });
}
