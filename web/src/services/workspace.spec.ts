import { afterEach, beforeEach, expect, it, vi } from "vitest";

import {
  createUserWithApi,
  fetchWorkspaceStateWithApi,
  saveWorkspaceStateWithApi,
} from "@/services/workspace";

const fetchMock = vi.fn<typeof fetch>();

describe("workspace service", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    fetchMock.mockReset();
  });

  it("loads and saves workspace state through authenticated endpoints", async () => {
    fetchMock
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            revision: 7,
            devices: [],
            conversations: [],
            activeConversationId: "",
            printJobs: [],
            schedules: [],
            sources: [],
            preferences: {
              loginProtectionEnabled: false,
              sendConfirmationEnabled: true,
              tutorialTabEnabled: true,
              theme: "light",
              defaultDeviceId: "",
              locale: "system",
            },
            serviceBinding: {
              providerName: null,
              modelName: "Ink AI",
              bound: false,
            },
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            revision: 8,
            devices: [],
            conversations: [],
            activeConversationId: "",
            printJobs: [],
            schedules: [],
            sources: [],
            preferences: {
              loginProtectionEnabled: false,
              sendConfirmationEnabled: true,
              tutorialTabEnabled: true,
              theme: "light",
              defaultDeviceId: "",
              locale: "system",
            },
            serviceBinding: {
              providerName: null,
              modelName: "Ink AI",
              bound: false,
            },
          }),
          { status: 200 },
        ),
      );

    await expect(fetchWorkspaceStateWithApi("access-token")).resolves.toMatchObject({
      revision: 7,
      preferences: {
        theme: "light",
      },
    });

    await expect(
      saveWorkspaceStateWithApi("access-token", {
        revision: 7,
        devices: [],
        conversations: [],
        activeConversationId: "",
        printJobs: [],
        schedules: [],
        sources: [],
        preferences: {
          loginProtectionEnabled: false,
          sendConfirmationEnabled: true,
          tutorialTabEnabled: true,
          theme: "light",
          defaultDeviceId: "",
          locale: "system",
        },
        serviceBinding: {
          providerName: null,
          modelName: "Ink AI",
          bound: false,
        },
      }),
    ).resolves.toMatchObject({
      revision: 8,
      serviceBinding: {
        modelName: "Ink AI",
      },
    });
    expect(JSON.parse(fetchMock.mock.calls[1]?.[1]?.body as string).revision).toBe(7);
  });

  it("creates users through the admin endpoint", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          user: {
            id: "user-2",
            email: "new-user",
            name: "New User",
            role: "member",
          },
        }),
        { status: 201 },
      ),
    );

    await expect(
      createUserWithApi("access-token", {
        email: "new-user",
        name: "New User",
        password: "demo-password",
      }),
    ).resolves.toMatchObject({
      email: "new-user",
      role: "member",
    });
  });
});
