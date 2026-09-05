import { flushPromises, mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { beforeEach, vi } from "vitest";

import { createTestRouter } from "@/router";
import type { fetchAIConfigSummary, generateAIReply, saveAIConfig } from "@/services/ai";
import type {
  changePasswordWithApi,
  fetchCurrentUser,
  loginWithApi,
  logoutWithApi,
  refreshAuthSession,
} from "@/services/auth";
import type { submitFeedbackToAdmin } from "@/services/feedback";
import type {
  bindPrinter,
  cancelPrintJob,
  createPrintJob,
  deletePrinter,
  fetchPrintJobs,
  fetchPrinters,
  submitPrintJob,
  updatePrintJobDevice,
} from "@/services/printers";
import type {
  createUserWithApi,
  fetchWorkspaceStateWithApi,
  saveWorkspaceStateWithApi,
} from "@/services/workspace";
import { useWorkspaceStore } from "@/stores/workspace";
import type { PrintJob } from "@/types/workspace";
import ConversationsView from "@/views/ConversationsView.vue";
import LoginView from "@/views/LoginView.vue";
import PrintsView from "@/views/PrintsView.vue";
import SettingsView from "@/views/SettingsView.vue";
import StatusView from "@/views/StatusView.vue";
import TutorialView from "@/views/TutorialView.vue";

vi.mock("@/services/auth", () => ({
  changePasswordWithApi: vi.fn<typeof changePasswordWithApi>(async () => undefined),
  fetchCurrentUser: vi.fn<typeof fetchCurrentUser>(),
  loginWithApi: vi.fn<typeof loginWithApi>(async ({ email }: { email: string }) => ({
    user: {
      id: "user-1",
      email,
      name: "Ink User",
      role: "member",
    },
    session: {
      accessToken: "access-token",
      refreshToken: "refresh-token",
      accessTokenExpiresAt: new Date(Date.now() + 900_000).toISOString(),
    },
  })),
  logoutWithApi: vi.fn<typeof logoutWithApi>(async () => undefined),
  refreshAuthSession: vi.fn<typeof refreshAuthSession>(),
  AuthApiError: class AuthApiError extends Error {},
}));

vi.mock("@/services/workspace", () => ({
  createUserWithApi: vi.fn<typeof createUserWithApi>(async () => ({
    id: "user-2",
    email: "new-user",
    name: "New User",
    role: "member",
  })),
  fetchWorkspaceStateWithApi: vi.fn<typeof fetchWorkspaceStateWithApi>(async () => ({
    devices: [],
    conversations: [],
    activeConversationId: "",
    printJobs: [],
    schedules: [],
    sources: [],
    preferences: {
      loginProtectionEnabled: false,
      sendConfirmationEnabled: false,
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
  })),
  saveWorkspaceStateWithApi: vi.fn<typeof saveWorkspaceStateWithApi>(
    async (_accessToken, state) => state,
  ),
}));

vi.mock("@/services/ai", () => ({
  fetchAIConfigSummary: vi.fn<typeof fetchAIConfigSummary>(async () => ({
    bound: false,
    providerName: "OpenAI Compatible",
    providerType: "openai-compatible",
    baseUrl: "",
    model: "gpt-4.1-mini",
    keyConfigured: false,
  })),
  generateAIReply: vi.fn<typeof generateAIReply>(async () => ({
    content: "这是来自真实 AI 服务的回复。",
    model: "gpt-4.1-mini",
    providerName: "OpenAI Compatible",
  })),
  saveAIConfig: vi.fn<typeof saveAIConfig>(async (_accessToken, payload) => ({
    bound: true,
    providerName: payload.providerName,
    providerType: payload.providerType,
    baseUrl: payload.baseUrl,
    model: payload.model,
    keyConfigured: true,
  })),
}));

vi.mock("@/services/feedback", () => ({
  submitFeedbackToAdmin: vi.fn<typeof submitFeedbackToAdmin>(async () => undefined),
}));

vi.mock("@/services/printers", () => ({
  bindPrinter: vi.fn<typeof bindPrinter>(async (_accessToken, payload) => ({
    id: "device-api-1",
    name: payload.name,
    status: "connected",
    note: payload.note,
  })),
  cancelPrintJob: vi.fn<typeof cancelPrintJob>(async (_accessToken, jobId) => ({
    id: jobId,
    title: "服务端任务",
    source: "手动打印",
    deviceId: "device-api-1",
    status: "cancelled",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    content: "内容",
  })),
  createPrintJob: vi.fn<typeof createPrintJob>(async (_accessToken, payload) => ({
    id: "print-api-1",
    title: payload.title,
    source: payload.source,
    deviceId: payload.printerBindingId,
    status: payload.submitImmediately ? "queued" : "pending",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    content: payload.content,
  })),
  deletePrinter: vi.fn<typeof deletePrinter>(async () => undefined),
  fetchPrintJobs: vi.fn<typeof fetchPrintJobs>(async () => ({
    printJobs: [],
  })),
  fetchPrinters: vi.fn<typeof fetchPrinters>(async () => ({
    devices: [],
  })),
  submitPrintJob: vi.fn<typeof submitPrintJob>(async (_accessToken, jobId) => ({
    id: jobId,
    title: "服务端任务",
    source: "手动打印",
    deviceId: "device-api-1",
    status: "queued",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    content: "内容",
  })),
  updatePrintJobDevice: vi.fn<typeof updatePrintJobDevice>(
    async (_accessToken, jobId, payload) => ({
      id: jobId,
      title: "服务端任务",
      source: "手动打印",
      deviceId: payload.printerBindingId,
      status: "pending",
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      content: "内容",
    }),
  ),
}));

beforeEach(() => {
  vi.clearAllMocks();
  window.localStorage.clear();
  window.sessionStorage.clear();
  vi.stubGlobal(
    "confirm",
    vi.fn(() => true),
  );
});

async function createWorkspaceContext(path = "/status", authenticated = true) {
  const pinia = createPinia();
  setActivePinia(pinia);
  const store = useWorkspaceStore();

  if (authenticated) {
    store.authUser = {
      id: "user-1",
      email: "name@example.com",
      name: "Ink User",
      role: "member",
    };
    store.authSession = {
      accessToken: "access-token",
      refreshToken: "refresh-token",
      accessTokenExpiresAt: new Date(Date.now() + 60_000).toISOString(),
    };
  }

  const router = createTestRouter(pinia);
  router.push(path);
  await router.isReady();

  return { pinia, router, store };
}

describe("workspace views", () => {
  it("renders the status overview from shared store state", async () => {
    const { pinia, router } = await createWorkspaceContext("/status");
    const wrapper = mount(StatusView, {
      global: {
        plugins: [pinia, router],
      },
    });

    expect(wrapper.text()).toContain("已绑定设备");
    expect(wrapper.text()).toContain("自动打印");
    expect(wrapper.findAll(".ui-list-card article")).not.toHaveLength(0);
  });

  it("manages devices from the status view", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/status");
    const wrapper = mount(StatusView, {
      global: {
        plugins: [pinia, router],
      },
    });

    const previousLength = store.devices.length;
    await wrapper
      .findAll("button")
      .find((button) => button.text() === "添加设备")
      ?.trigger("click");
    await new Promise((resolve) => window.setTimeout(resolve, 0));
    await wrapper.find("input[placeholder='例如：客厅咕咕机']").setValue("客厅咕咕机");
    await wrapper.find("input[placeholder='例如：窗边打印机']").setValue("窗边打印机");
    await wrapper.find("input[placeholder='例如：xxxxxx']").setValue("m1-123456");
    await wrapper.find("input[type='checkbox']").setValue(true);
    await wrapper.find("form").trigger("submit");

    expect(store.devices).toHaveLength(previousLength + 1);
    expect(store.defaultDeviceId).toBe(store.devices.at(0)?.id);

    const deskArticle = wrapper
      .findAll("article")
      .find((article) => article.text().includes("书桌咕咕机"));
    await deskArticle
      ?.findAll("button")
      .find((button) => button.text() === "删除")
      ?.trigger("click");

    expect(store.devices.some((device) => device.id === "device-desk")).toBe(false);
  });

  it("allows sending a message and selecting a reply for printing", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/conversations");
    const wrapper = mount(ConversationsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    await wrapper.find("textarea").setValue("请帮我整理成一句适合贴在电脑旁边的提醒");
    await wrapper.find("button.ui-btn-primary").trigger("click");
    await new Promise((resolve) => window.setTimeout(resolve, 120));

    expect(store.activeConversation?.messages.at(-1)?.role).toBe("assistant");
    expect(wrapper.text()).toContain("打印选中问答");
    expect(store.selectedConversationMessageIds).toEqual([]);

    const assistantSelectionButton = wrapper
      .findAll("button")
      .filter((button) => button.attributes("aria-label") === "选择这条消息")
      .at(-1);
    await assistantSelectionButton?.trigger("click");
    expect(store.selectedConversationMessageIds).toContain(
      store.activeConversation?.messages.at(-1)?.id,
    );

    await wrapper
      .findAll("button.ui-btn-secondary")
      .find((button) => button.text() === "打印选中问答")
      ?.trigger("click");

    expect(store.pendingPrintJobs.at(0)?.source).toBe("对话选中问答");
  });

  it("opens the feedback dialog and submits feedback from the conversations view", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/conversations");
    const wrapper = mount(ConversationsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    expect(wrapper.text()).toContain("问题 / 建议 / 吐槽反馈");

    await wrapper
      .findAll("button")
      .find((button) => button.text() === "反馈给作者")
      ?.trigger("click");
    await new Promise((resolve) => window.setTimeout(resolve, 0));

    const feedbackInput = wrapper.find("textarea[placeholder='反馈（功能 / 建议 / 吐槽）']");
    expect(feedbackInput.exists()).toBe(true);

    await feedbackInput.setValue("建议把反馈入口放到顶部");
    await wrapper.findAll("form").at(-1)?.trigger("submit");
    await flushPromises();

    expect(store.flashMessage).toBe("反馈已发送，作者会直接收到纸条。");
  });

  it("guides and supports the first message when there is no conversation history", async () => {
    window.localStorage.setItem(
      "ink.workspace.v1",
      JSON.stringify({
        authUser: null,
        devices: [],
        conversations: [],
        activeConversationId: "",
        printJobs: [],
        schedules: [],
        sources: [],
        preferences: {
          loginProtectionEnabled: false,
          sendConfirmationEnabled: false,
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
    );

    const { pinia, router, store } = await createWorkspaceContext("/conversations", false);
    const wrapper = mount(ConversationsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    expect(wrapper.text()).toContain("还没有历史对话，直接输入第一条消息即可开始。");
    expect(store.conversations).toHaveLength(1);

    await wrapper.find("textarea").setValue("这是第一条消息");
    await wrapper.find("button.ui-btn-primary").trigger("click");
    await new Promise((resolve) => window.setTimeout(resolve, 120));

    expect(store.activeConversation?.messages[0]?.text).toBe("这是第一条消息");
    expect(store.activeConversation?.messages.at(-1)?.role).toBe("assistant");
  });

  it("allows selecting and printing a user message", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/conversations");
    const wrapper = mount(ConversationsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    const userMessageButton = wrapper
      .findAll("button")
      .find((button) => button.text().includes("帮我整理一张温柔一点的今日提醒"));

    await userMessageButton?.trigger("click");
    expect(store.selectedConversationMessages.at(0)?.role).toBe("user");

    await wrapper
      .findAll("button.ui-btn-secondary")
      .find((button) => button.text() === "打印选中问答")
      ?.trigger("click");

    expect(store.pendingPrintJobs.at(0)?.content).toContain("帮我整理一张温柔一点的今日提醒");
  });

  it("deletes the current conversation from the conversations view", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/conversations");
    const wrapper = mount(ConversationsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    const previousLength = store.conversations.length;
    await wrapper
      .findAll("button")
      .find((button) => button.text() === "删除对话")
      ?.trigger("click");

    expect(store.conversations).toHaveLength(previousLength - 1);
    expect(store.activeConversation).not.toBeNull();
  });

  it("validates and submits the login form", async () => {
    const { pinia, router } = await createWorkspaceContext("/login", false);
    const wrapper = mount(LoginView, {
      global: {
        plugins: [pinia, router],
      },
    });

    await wrapper.find("input[type='text']").setValue("");
    await wrapper.find("input[type='password']").setValue("demo");
    await wrapper.find("form").trigger("submit");

    expect(wrapper.text()).toContain("请输入账号和密码。");

    await wrapper.find("input[type='text']").setValue("admin");
    await wrapper.find("input[type='password']").setValue("demo-password");
    await wrapper.find("form").trigger("submit");
    await new Promise((resolve) => window.setTimeout(resolve, 120));

    expect(router.currentRoute.value.fullPath).toBe("/conversations");
  });

  it("renders the login title as two lines without welcome-back copy", async () => {
    const { pinia, router } = await createWorkspaceContext("/login", false);
    const wrapper = mount(LoginView, {
      global: {
        plugins: [pinia, router],
      },
    });

    const titleLines = wrapper.findAll("h1 span");

    expect(titleLines).toHaveLength(2);
    expect(titleLines.map((line) => line.text())).toEqual(["打开 Ink", "继续你的纸条灵感"]);
    expect(wrapper.text()).not.toContain("欢迎回来");
    expect(wrapper.text()).not.toContain("打开 Ink，继续你的纸条灵感。");
  });

  it("shows the password-updated notice and supports password visibility toggle on login", async () => {
    const { pinia, router } = await createWorkspaceContext("/login?notice=password-updated", false);
    const wrapper = mount(LoginView, {
      global: {
        plugins: [pinia, router],
      },
    });

    expect(wrapper.text()).toContain("密码已更新，请使用新密码重新登录。");
    const passwordInput = wrapper.find("input[type='password']");
    expect(passwordInput.exists()).toBe(true);

    const toggleButton = wrapper.findAll("button").find((button) => button.text() === "显示");
    await toggleButton?.trigger("click");

    expect(wrapper.find("input[type='text']").exists()).toBe(true);
  });

  it("returns to settings after login when the redirect is valid", async () => {
    const { pinia, router } = await createWorkspaceContext("/login?redirect=/settings", false);
    const wrapper = mount(LoginView, {
      global: {
        plugins: [pinia, router],
      },
    });

    await wrapper.find("input[type='text']").setValue("admin");
    await wrapper.find("input[type='password']").setValue("demo-password");
    await wrapper.find("form").trigger("submit");
    await new Promise((resolve) => window.setTimeout(resolve, 120));

    expect(router.currentRoute.value.fullPath).toBe("/settings");
  });

  it("renders the tutorial page with conversation and print guidance", async () => {
    const { pinia, router } = await createWorkspaceContext("/tutorial", false);
    const wrapper = mount(TutorialView, {
      global: {
        plugins: [pinia, router],
      },
    });

    expect(router.currentRoute.value.fullPath).toBe("/tutorial");
    expect(wrapper.text()).toContain("Ink 里最常用的三种打印方式");
    expect(wrapper.text()).toContain("三种常用使用方式");
    expect(wrapper.text()).toContain("添加到 iPhone 主屏幕");
    expect(wrapper.text()).toContain("对话打印、直接打印、定时打印");
    expect(wrapper.text()).toContain("没找到问题？");
    expect(wrapper.text()).toContain("反馈给作者");
  });

  it("confirms pending prints and reflects shared defaults", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/prints");
    const wrapper = mount(PrintsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    const pendingButton = wrapper.findAll("button").find((button) => button.text() === "确认打印");

    await pendingButton?.trigger("click");

    expect(store.pendingPrintJobs.some((job: PrintJob) => job.status === "queued")).toBe(true);
    expect(wrapper.text()).toContain("默认打印设置");
    expect(wrapper.text()).toContain("书桌咕咕机");
    expect(wrapper.text()).toContain("绑定教程");
  });

  it("cancels a pending print job from the prints view", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/prints");
    const wrapper = mount(PrintsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    const pendingArticle = wrapper
      .findAll("article")
      .find((article) => article.text().includes("待确认"));
    await pendingArticle
      ?.findAll("button")
      .find((button) => button.text() === "取消打印")
      ?.trigger("click");

    expect(store.pendingPrintJobs.some((job: PrintJob) => job.status === "pending")).toBe(false);
    expect(store.printJobs.some((job: PrintJob) => job.status === "cancelled")).toBe(true);
  });

  it("prevents cancelling or rebinding queued print jobs for authenticated users", async () => {
    const { pinia, router } = await createWorkspaceContext("/prints");
    const wrapper = mount(PrintsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    const queuedArticle = wrapper
      .findAll("article")
      .find((article) => article.text().includes("明日早报"));

    expect(queuedArticle?.text()).toContain("已提交到咕咕机后不能再取消或改绑设备。");
    expect(queuedArticle?.findAll("button").some((button) => button.text() === "取消打印")).toBe(
      false,
    );
    expect(queuedArticle?.find("select").attributes("disabled")).toBeDefined();
  });

  it("updates shared settings state through the settings panel", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/settings");
    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    const selects = wrapper.findAll("select");
    await selects[0].setValue("device-bedroom");

    expect(store.activeDeviceLabel).toBe("卧室咕咕机");
    expect(selects).toHaveLength(1);
    expect(wrapper.text()).toContain("AI 服务");
    expect(wrapper.text()).toContain("未配置");
    expect(wrapper.text()).toContain("服务商");
  });

  it("toggles the tutorial tab from settings", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/settings");
    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    const toggle = wrapper
      .findAll("button")
      .find((button) => button.attributes("aria-label") === "关闭教程标签");

    await toggle?.trigger("click");

    expect(store.tutorialTabEnabled).toBe(false);
  });

  it("lets administrators submit the real AI config form", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/settings");
    store.authUser = {
      id: "user-1",
      email: "admin",
      name: "Administrator",
      role: "admin",
    };

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    await wrapper
      .findAll("button")
      .find((button) => button.text() === "编辑 AI 配置")
      ?.trigger("click");
    await flushPromises();
    await wrapper.find("input[placeholder='例如：OpenAI Compatible']").setValue("Acme AI");
    await wrapper
      .find("input[placeholder='例如：https://api.openai.com/v1']")
      .setValue("https://example.com/v1");
    await wrapper.find("input[placeholder='例如：gpt-4.1-mini']").setValue("gpt-4.1-mini");
    await wrapper.find("input[placeholder='输入新的服务端密钥']").setValue("secret-key");

    const aiForm = wrapper.findAll("form").find((form) => form.text().includes("保存 AI 配置"));
    await aiForm?.trigger("submit");
    await flushPromises();

    expect(store.aiConfigSummary.bound).toBe(true);
    expect(store.aiConfigSummary.providerName).toBe("Acme AI");
  });

  it("shows AI summary-only copy for non-admin settings users", async () => {
    const { pinia, router } = await createWorkspaceContext("/settings");
    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    expect(wrapper.text()).toContain("AI 服务");
    expect(wrapper.text()).toContain("未配置");
    expect(wrapper.text()).not.toContain("编辑 AI 配置");
  });

  it("submits the password change form and returns to login", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/settings");
    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    await wrapper
      .findAll("button")
      .find((button) => button.text() === "修改密码")
      ?.trigger("click");
    await flushPromises();

    const passwordInputs = wrapper.findAll("input[type='password']");
    await passwordInputs[0].setValue("demo-password");
    await passwordInputs[1].setValue("next-password");
    await passwordInputs[2].setValue("next-password");
    const showButtons = wrapper.findAll("button").filter((button) => button.text() === "显示");
    await showButtons[0]?.trigger("click");
    expect(passwordInputs[0].attributes("type")).toBe("text");

    await wrapper.findAll("form")[0]?.trigger("submit");
    await new Promise((resolve) => window.setTimeout(resolve, 10));

    expect(store.isAuthenticated).toBe(false);
    expect(router.currentRoute.value.fullPath).toBe("/login?notice=password-updated");
  });

  it("shows admin account creation controls in settings for administrators", async () => {
    const { pinia, router, store } = await createWorkspaceContext("/settings");
    store.authUser = {
      id: "user-1",
      email: "admin",
      name: "Administrator",
      role: "admin",
    };

    const wrapper = mount(SettingsView, {
      global: {
        plugins: [pinia, router],
      },
    });

    expect(wrapper.text()).toContain("创建新账号");
    expect(wrapper.text()).toContain("管理员");
    expect(wrapper.text()).toContain("编辑 AI 配置");

    await wrapper
      .findAll("button")
      .find((button) => button.text() === "创建账号")
      ?.trigger("click");
    await flushPromises();

    expect(wrapper.text()).toContain("为成员创建独立登录账号");
    expect(wrapper.find("input[placeholder='例如：alice']").exists()).toBe(true);
  });
});
