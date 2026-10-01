import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { configureAuthRefresh } from "@/services/http";
import { fetchPrintJob, fetchPrintJobs, fetchPrintJobStatuses } from "@/services/printers";

describe("printer queries", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    configureAuthRefresh(null);
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    fetchMock.mockReset();
  });

  it("requests a bounded summary page and preserves opaque cursors", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ printJobs: [], nextCursor: null })),
    );
    await fetchPrintJobs("token", { status: "active", limit: 20, cursor: "a+/= cursor" });
    const [input, init] = fetchMock.mock.calls[0]!;
    const url = new URL(String(input), "https://ink.example");
    expect(url.pathname).toBe("/api/v1/print-jobs");
    expect(Object.fromEntries(url.searchParams)).toEqual({
      status: "active",
      limit: "20",
      cursor: "a+/= cursor",
    });
    expect(new Headers(init?.headers).get("Authorization")).toBe("Bearer token");
  });

  it("queries statuses with the user's day boundary even when no jobs are loaded", async () => {
    fetchMock.mockResolvedValueOnce(new Response(JSON.stringify({ printJobs: [] })));
    await fetchPrintJobStatuses("token", [], "2026-09-30T16:00:00.000Z");
    const url = new URL(String(fetchMock.mock.calls[0]![0]), "https://ink.example");
    expect(url.pathname).toBe("/api/v1/print-jobs/status");
    expect(url.searchParams.get("since")).toBe("2026-09-30T16:00:00.000Z");
    expect(url.searchParams.has("ids")).toBe(false);
  });

  it("retrieves a full body only from the authenticated detail endpoint", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ printJob: { id: "job-1", content: "full body" } })),
    );
    await expect(fetchPrintJob("token", "job-1")).resolves.toMatchObject({ content: "full body" });
    expect(fetchMock.mock.calls[0]![0]).toBe("/api/v1/print-jobs/job-1");
  });
});
