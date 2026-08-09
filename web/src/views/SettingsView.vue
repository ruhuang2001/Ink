<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

import AppDialog from "@/components/AppDialog.vue";
import { DEFAULT_LOGIN_REDIRECT } from "@/router/authRedirect";
import { storeToRefs } from "@/stores/pinia";
import { useWorkspaceStore } from "@/stores/workspace";
import type { PluginDetails, PluginFieldSpec } from "@/types/plugins";
import { getPluginFieldDefaultValue } from "@/utils/plugins";
import {
  getPluginBindingStatusBadgeClass,
  getPluginBindingStatusLabel,
  getPluginInstallationStatusBadgeClass,
  getPluginInstallationStatusLabel,
  getServiceBindingStatusBadgeClass,
  getThemeDescription,
  getUserRoleBadgeClass,
  getUserRoleLabel,
} from "@/utils/workspace";

const router = useRouter();
const workspaceStore = useWorkspaceStore();
const { adminPlugins, aiConfigSummary, availablePlugins } = storeToRefs(workspaceStore);
const { t } = useI18n();

const themes = computed(
  () =>
    [
      { label: t("theme.light"), value: "light" },
      { label: t("theme.dark"), value: "dark" },
      { label: t("theme.system"), value: "system" },
    ] as const,
);
const localeOptions = computed(
  () =>
    [
      { label: t("settings.language.options.system"), value: "system" },
      { label: t("settings.language.options.zhCN"), value: "zh-CN" },
      { label: t("settings.language.options.enUS"), value: "en-US" },
    ] as const,
);

const currentPassword = ref("");
const newPassword = ref("");
const confirmPassword = ref("");
const passwordFormError = ref("");
const currentPasswordVisible = ref(false);
const newPasswordVisible = ref(false);
const confirmPasswordVisible = ref(false);
const newAccountEmail = ref("");
const newAccountName = ref("");
const newAccountPassword = ref("");
const newAccountFormError = ref("");
const newAccountPasswordVisible = ref(false);
const aiProviderName = ref("OpenAI Compatible");
const aiBaseUrl = ref("");
const aiModel = ref("gpt-4.1-mini");
const aiApiKey = ref("");
const aiFormError = ref("");
const passwordDialogOpen = ref(false);
const accountCreationDialogOpen = ref(false);
const aiConfigDialogOpen = ref(false);
const pluginUploadError = ref("");
const pluginGitInstallError = ref("");
const pluginGitRepoUrl = ref("");
const pluginGitRepoRef = ref("");
const pluginGitRepoSubdir = ref("");
const pluginAddDialogOpen = ref(false);
const pluginAddMode = ref<"zip" | "github">("zip");
const activePluginConfigId = ref<string | null>(null);
const pluginEnabledDrafts = ref<Record<string, boolean>>({});
const pluginDrafts = ref<Record<string, Record<string, unknown>>>({});
const pluginSecretDrafts = ref<Record<string, Record<string, string>>>({});
const pluginTestMessages = ref<Record<string, string>>({});

const AI_PROVIDER_TYPE = "openai-compatible";
const AI_PROVIDER_NAME_FALLBACK = "OpenAI Compatible";
const AI_MODEL_FALLBACK = "gpt-4.1-mini";

const aiConfigErrorMessage = computed(() => aiFormError.value || workspaceStore.aiConfigError);
const pluginInstallations = computed(() =>
  workspaceStore.isAdmin ? adminPlugins.value : availablePlugins.value,
);
const activePluginConfig = computed(() => {
  const installationId = activePluginConfigId.value;
  const plugins = availablePlugins.value;
  for (const plugin of plugins) {
    if (plugin.installation.id === installationId) {
      return plugin;
    }
  }
  return null;
});

void AppDialog;
void pluginInstallations.value;
void activePluginConfig.value;

watch(
  aiConfigSummary,
  (summary) => {
    aiProviderName.value = summary.providerName || AI_PROVIDER_NAME_FALLBACK;
    aiBaseUrl.value = summary.baseUrl;
    aiModel.value = summary.model || AI_MODEL_FALLBACK;
  },
  { deep: true, immediate: true },
);

function handleDefaultDeviceChange(event: Event) {
  const target = event.target as HTMLSelectElement | null;
  workspaceStore.setDefaultDevice(target?.value ?? workspaceStore.defaultDeviceId);
}

function createPluginDraftState(plugin: PluginDetails) {
  const config: Record<string, unknown> = {};
  const secrets: Record<string, string> = {};

  for (const field of plugin.manifest.workspaceConfigSchema) {
    if (field.type === "secret") {
      secrets[field.key] = "";
      continue;
    }

    config[field.key] = plugin.binding?.config?.[field.key] ?? getPluginFieldDefaultValue(field);
  }

  return {
    enabled: plugin.binding?.enabled ?? false,
    config,
    secrets,
  };
}

function resetPluginDraft(plugin: PluginDetails) {
  const nextState = createPluginDraftState(plugin);
  pluginEnabledDrafts.value[plugin.installation.id] = nextState.enabled;
  pluginDrafts.value[plugin.installation.id] = nextState.config;
  pluginSecretDrafts.value[plugin.installation.id] = nextState.secrets;
}

function ensurePluginDraft(plugin: PluginDetails) {
  pluginEnabledDrafts.value[plugin.installation.id] =
    pluginEnabledDrafts.value[plugin.installation.id] ?? plugin.binding?.enabled ?? false;

  const nextDraft = { ...pluginDrafts.value[plugin.installation.id] };
  for (const field of plugin.manifest.workspaceConfigSchema) {
    if (field.type === "secret" || field.key in nextDraft) {
      continue;
    }
    nextDraft[field.key] = plugin.binding?.config?.[field.key] ?? getPluginFieldDefaultValue(field);
  }
  pluginDrafts.value[plugin.installation.id] = nextDraft;

  const nextSecrets = { ...pluginSecretDrafts.value[plugin.installation.id] };
  for (const field of plugin.manifest.workspaceConfigSchema) {
    if (field.type !== "secret" || field.key in nextSecrets) {
      continue;
    }
    nextSecrets[field.key] = "";
  }
  pluginSecretDrafts.value[plugin.installation.id] = nextSecrets;
}

watch(
  () => workspaceStore.availablePlugins,
  (plugins) => {
    const activeIDs = new Set(plugins.map((plugin) => plugin.installation.id));
    for (const drafts of [
      pluginEnabledDrafts.value,
      pluginDrafts.value,
      pluginSecretDrafts.value,
    ]) {
      for (const installationID of Object.keys(drafts)) {
        if (!activeIDs.has(installationID)) {
          delete drafts[installationID];
        }
      }
    }
    for (const plugin of plugins) {
      resetPluginDraft(plugin);
    }
  },
  { deep: true, immediate: true },
);

function pluginDraftValue(plugin: PluginDetails, field: PluginFieldSpec) {
  return field.type === "secret"
    ? (pluginSecretDrafts.value[plugin.installation.id]?.[field.key] ?? "")
    : (pluginDrafts.value[plugin.installation.id]?.[field.key] ??
        getPluginFieldDefaultValue(field));
}

function pluginPermissionLabels(plugin: PluginDetails) {
  const permissions = plugin.manifest.permissions;
  const labels: string[] = [];
  const network = permissions?.network;

  if (!network || network.mode === "none") {
    labels.push(t("settings.plugins.permissions.networkNone"));
  } else if (network.mode === "all") {
    labels.push(t("settings.plugins.permissions.networkAll"));
  } else {
    const hosts = network.hosts?.join(", ") || t("settings.plugins.permissions.unspecifiedHosts");
    labels.push(t("settings.plugins.permissions.networkHosts", { hosts }));
  }

  if (permissions?.filesystem?.cache) {
    labels.push(t("settings.plugins.permissions.cache"));
  }
  if (permissions?.installScripts) {
    labels.push(t("settings.plugins.permissions.installScripts"));
  }

  return labels;
}

function updatePluginDraft(plugin: PluginDetails, field: PluginFieldSpec, value: unknown) {
  ensurePluginDraft(plugin);

  if (field.type === "secret") {
    pluginSecretDrafts.value[plugin.installation.id] = {
      ...pluginSecretDrafts.value[plugin.installation.id],
      [field.key]: String(value ?? ""),
    };
    return;
  }

  pluginDrafts.value[plugin.installation.id] = {
    ...pluginDrafts.value[plugin.installation.id],
    [field.key]:
      field.type === "number" && value !== ""
        ? Number(value)
        : field.type === "checkbox"
          ? Boolean(value)
          : value,
  };
}

function setPluginEnabled(plugin: PluginDetails, enabled: boolean) {
  ensurePluginDraft(plugin);
  pluginEnabledDrafts.value[plugin.installation.id] = enabled;
}

function handlePluginEnabledChange(plugin: PluginDetails, event: Event) {
  const target = event.target as HTMLInputElement | null;
  setPluginEnabled(plugin, target?.checked ?? false);
}

function handlePluginInput(plugin: PluginDetails, field: PluginFieldSpec, event: Event) {
  const target = event.target as HTMLInputElement | HTMLTextAreaElement | null;
  updatePluginDraft(plugin, field, target?.value ?? "");
}

function handlePluginSelect(plugin: PluginDetails, field: PluginFieldSpec, event: Event) {
  const target = event.target as HTMLSelectElement | null;
  updatePluginDraft(plugin, field, target?.value ?? "");
}

function handlePluginCheckbox(plugin: PluginDetails, field: PluginFieldSpec, event: Event) {
  const target = event.target as HTMLInputElement | null;
  updatePluginDraft(plugin, field, target?.checked ?? false);
}

async function handlePluginUpload(event: Event) {
  pluginUploadError.value = "";
  const target = event.target as HTMLInputElement | null;
  const file = target?.files?.[0];

  if (!file) {
    return;
  }

  const uploaded = await workspaceStore.uploadPlugin(file);
  if (!uploaded) {
    pluginUploadError.value = workspaceStore.pluginActionError;
    return;
  }

  pluginAddDialogOpen.value = false;

  if (target) {
    target.value = "";
  }
}

async function handlePluginInstallFromGit() {
  pluginGitInstallError.value = "";

  const repoUrl = pluginGitRepoUrl.value.trim();
  const repoRef = pluginGitRepoRef.value.trim();
  const repoSubdir = pluginGitRepoSubdir.value.trim();

  if (!repoUrl) {
    pluginGitInstallError.value = t("settings.plugins.addDialog.errors.repoUrlRequired");
    return;
  }

  const installed = await workspaceStore.installPluginRepository({
    repoUrl,
    repoRef,
    repoSubdir,
  });
  if (!installed) {
    pluginGitInstallError.value = workspaceStore.pluginActionError;
    return;
  }

  pluginGitRepoUrl.value = "";
  pluginGitRepoRef.value = "";
  pluginGitRepoSubdir.value = "";
  pluginAddDialogOpen.value = false;
}

async function handlePluginDisable(installationId: string) {
  await workspaceStore.disablePluginInstallation(installationId);
}

function openPluginAddDialog(mode: "zip" | "github" = "zip") {
  pluginUploadError.value = "";
  pluginGitInstallError.value = "";
  pluginAddMode.value = mode;
  pluginAddDialogOpen.value = true;
}

function closePluginAddDialog() {
  pluginAddDialogOpen.value = false;
}

function openPluginConfigDialog(plugin: PluginDetails) {
  ensurePluginDraft(plugin);
  activePluginConfigId.value = plugin.installation.id;
}

function closePluginConfigDialog() {
  activePluginConfigId.value = null;
}

async function handlePluginTest(plugin: PluginDetails) {
  ensurePluginDraft(plugin);
  const result = await workspaceStore.testPluginConfiguration(
    plugin.installation.id,
    pluginDrafts.value[plugin.installation.id] ?? {},
    pluginSecretDrafts.value[plugin.installation.id] ?? {},
    pluginEnabledDrafts.value[plugin.installation.id] ?? false,
  );

  pluginTestMessages.value[plugin.installation.id] = result?.valid
    ? t("settings.plugins.testPassed")
    : result?.errors?.map((error) => error.message).join("；") || workspaceStore.pluginActionError;
}

async function handlePluginSave(plugin: PluginDetails) {
  ensurePluginDraft(plugin);
  const saved = await workspaceStore.savePluginConfiguration(
    plugin.installation.id,
    pluginDrafts.value[plugin.installation.id] ?? {},
    pluginSecretDrafts.value[plugin.installation.id] ?? {},
    pluginEnabledDrafts.value[plugin.installation.id] ?? false,
  );

  if (saved) {
    pluginSecretDrafts.value[plugin.installation.id] = createPluginDraftState(plugin).secrets;
    closePluginConfigDialog();
  }
}

async function handleLogout() {
  await workspaceStore.logout();
  await router.replace(DEFAULT_LOGIN_REDIRECT);
}

async function handlePasswordSubmit() {
  passwordFormError.value = "";
  const nextPassword = newPassword.value;
  const confirmNextPassword = confirmPassword.value;

  if (!currentPassword.value.trim()) {
    passwordFormError.value = t("settings.account.passwordDialog.errors.currentPasswordRequired");
    return;
  }

  if (nextPassword.length < 8) {
    passwordFormError.value = t("settings.account.passwordDialog.errors.passwordTooShort");
    return;
  }

  if (nextPassword !== confirmNextPassword) {
    passwordFormError.value = t("settings.account.passwordDialog.errors.passwordMismatch");
    return;
  }

  const success = await workspaceStore.changePassword(currentPassword.value, nextPassword);
  if (!success) {
    passwordFormError.value = workspaceStore.authError;
    return;
  }

  currentPassword.value = "";
  newPassword.value = "";
  confirmPassword.value = "";
  passwordDialogOpen.value = false;
  await router.replace({
    path: "/login",
    query: {
      notice: "password-updated",
    },
  });
}

async function handleCreateAccountSubmit() {
  newAccountFormError.value = "";

  if (!newAccountEmail.value.trim()) {
    newAccountFormError.value = t("settings.account.createAccountDialog.errors.accountRequired");
    return;
  }

  if (newAccountPassword.value.trim().length < 8) {
    newAccountFormError.value = t("settings.account.createAccountDialog.errors.passwordTooShort");
    return;
  }

  const success = await workspaceStore.createAccount(
    newAccountEmail.value.trim(),
    newAccountName.value.trim(),
    newAccountPassword.value,
  );

  if (!success) {
    newAccountFormError.value = workspaceStore.accountCreationError;
    return;
  }

  newAccountEmail.value = "";
  newAccountName.value = "";
  newAccountPassword.value = "";
  accountCreationDialogOpen.value = false;
}

async function handleAIConfigSubmit() {
  aiFormError.value = "";

  if (!aiBaseUrl.value.trim()) {
    aiFormError.value = t("settings.ai.dialog.errors.baseUrlRequired");
    return;
  }

  if (!aiModel.value.trim()) {
    aiFormError.value = t("settings.ai.dialog.errors.modelRequired");
    return;
  }

  if (!aiApiKey.value.trim() && !workspaceStore.aiConfigSummary.keyConfigured) {
    aiFormError.value = t("settings.ai.dialog.errors.apiKeyRequired");
    return;
  }

  const success = await workspaceStore.saveAIServiceConfig({
    providerName: aiProviderName.value.trim() || AI_PROVIDER_NAME_FALLBACK,
    providerType: AI_PROVIDER_TYPE,
    baseUrl: aiBaseUrl.value.trim(),
    model: aiModel.value.trim() || AI_MODEL_FALLBACK,
    apiKey: aiApiKey.value.trim(),
  });

  if (!success) {
    aiFormError.value = workspaceStore.aiConfigError;
    return;
  }

  aiApiKey.value = "";
  aiConfigDialogOpen.value = false;
}

function openAIConfigDialog() {
  aiFormError.value = "";
  aiConfigDialogOpen.value = true;
}

function closeAIConfigDialog() {
  aiConfigDialogOpen.value = false;
}

function resetPasswordForm() {
  currentPassword.value = "";
  newPassword.value = "";
  confirmPassword.value = "";
  passwordFormError.value = "";
  currentPasswordVisible.value = false;
  newPasswordVisible.value = false;
  confirmPasswordVisible.value = false;
}

function openPasswordDialog() {
  passwordFormError.value = "";
  passwordDialogOpen.value = true;
}

function closePasswordDialog() {
  passwordDialogOpen.value = false;
  resetPasswordForm();
}

function resetAccountCreationForm() {
  newAccountEmail.value = "";
  newAccountName.value = "";
  newAccountPassword.value = "";
  newAccountFormError.value = "";
  newAccountPasswordVisible.value = false;
}

function openAccountCreationDialog() {
  newAccountFormError.value = "";
  accountCreationDialogOpen.value = true;
}

function closeAccountCreationDialog() {
  accountCreationDialogOpen.value = false;
  resetAccountCreationForm();
}

void openPluginAddDialog;
void closePluginAddDialog;
void openPluginConfigDialog;
void openAIConfigDialog;
void closeAIConfigDialog;
void openPasswordDialog;
void closePasswordDialog;
void openAccountCreationDialog;
void closeAccountCreationDialog;
</script>

<template>
  <section class="mx-auto max-w-5xl space-y-8 pt-4">
    <div class="max-w-2xl">
      <h2 class="text-2xl font-semibold tracking-tight text-stone-900">
        {{ t("navigation.settings.label") }}
      </h2>
    </div>

    <div class="space-y-12">
      <article
        class="grid grid-cols-1 items-start gap-x-10 gap-y-5 md:grid-cols-[minmax(0,13rem)_1fr]"
      >
        <div>
          <h3 class="text-base leading-6 font-semibold text-stone-900">
            {{ t("settings.account.title") }}
          </h3>
        </div>
        <div class="min-w-0">
          <div class="ui-settings-group">
            <div class="ui-settings-row">
              <div class="ui-settings-copy">
                <p class="text-sm font-medium text-stone-900">
                  {{ t("settings.account.currentAccount") }}
                </p>
                <p class="mt-0.5 text-sm text-stone-500">
                  {{ workspaceStore.authUser?.email ?? t("settings.account.signedOut") }}
                </p>
              </div>
              <div class="flex flex-wrap items-center gap-3">
                <span
                  class="ui-status-badge"
                  :class="getUserRoleBadgeClass(workspaceStore.isAdmin ? 'admin' : 'member')"
                >
                  {{ getUserRoleLabel(workspaceStore.isAdmin ? "admin" : "member") }}
                </span>
                <button
                  type="button"
                  class="ui-btn-secondary px-3 py-1.5 text-sm"
                  @click="handleLogout"
                >
                  {{ t("common.actions.logout") }}
                </button>
              </div>
            </div>
            <div class="ui-settings-row">
              <div class="ui-settings-copy">
                <p class="text-sm font-medium text-stone-900">
                  {{ t("settings.account.loginProtection") }}
                </p>
                <p class="mt-0.5 text-sm text-stone-500">
                  {{
                    workspaceStore.loginProtectionEnabled
                      ? t("settings.account.loginProtectionEnabled")
                      : t("settings.account.loginProtectionDisabled")
                  }}
                </p>
              </div>
              <button
                type="button"
                class="ui-toggle"
                :class="{ 'is-on': workspaceStore.loginProtectionEnabled }"
                :aria-label="
                  workspaceStore.loginProtectionEnabled
                    ? t('settings.account.toggleAria.disableLoginProtection')
                    : t('settings.account.toggleAria.enableLoginProtection')
                "
                :aria-pressed="workspaceStore.loginProtectionEnabled"
                @click="workspaceStore.setLoginProtection(!workspaceStore.loginProtectionEnabled)"
              >
                <span class="ui-toggle-thumb" />
              </button>
            </div>
            <div class="rounded-xl border border-stone-200 bg-white p-4">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <p class="text-sm font-medium text-stone-900">
                    {{ t("settings.account.passwordCard.title") }}
                  </p>
                  <p class="mt-1 text-sm text-stone-500">
                    {{ t("settings.account.passwordCard.description") }}
                  </p>
                </div>
                <button
                  type="button"
                  class="ui-btn-primary px-4 py-2 text-sm"
                  @click="openPasswordDialog"
                >
                  {{ t("settings.account.passwordCard.action") }}
                </button>
              </div>
              <div class="mt-4 grid gap-4 md:grid-cols-2">
                <div class="rounded-xl border border-stone-200 bg-stone-50 px-3 py-3">
                  <p class="text-xs font-medium tracking-[0.12em] text-stone-500 uppercase">
                    {{ t("settings.account.passwordCard.securityRule") }}
                  </p>
                  <p class="mt-1 text-sm text-stone-900">
                    {{ t("settings.account.passwordCard.securityRuleValue") }}
                  </p>
                </div>
                <div class="rounded-xl border border-stone-200 bg-stone-50 px-3 py-3">
                  <p class="text-xs font-medium tracking-[0.12em] text-stone-500 uppercase">
                    {{ t("settings.account.passwordCard.result") }}
                  </p>
                  <p class="mt-1 text-sm text-stone-900">
                    {{ t("settings.account.passwordCard.resultValue") }}
                  </p>
                </div>
              </div>
            </div>
            <div
              v-if="workspaceStore.isAdmin"
              class="rounded-xl border border-stone-200 bg-white p-4"
            >
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <p class="text-sm font-medium text-stone-900">
                    {{ t("settings.account.createAccountCard.title") }}
                  </p>
                  <p class="mt-1 text-sm text-stone-500">
                    {{ t("settings.account.createAccountCard.description") }}
                  </p>
                </div>
                <div class="flex items-center gap-3">
                  <span class="ui-status-badge self-start" :class="getUserRoleBadgeClass('admin')">
                    {{ getUserRoleLabel("admin") }}
                  </span>
                  <button
                    type="button"
                    class="ui-btn-primary px-4 py-2 text-sm"
                    @click="openAccountCreationDialog"
                  >
                    {{ t("settings.account.createAccountCard.action") }}
                  </button>
                </div>
              </div>
              <div class="mt-4 grid gap-4 md:grid-cols-2">
                <div class="rounded-xl border border-stone-200 bg-stone-50 px-3 py-3">
                  <p class="text-xs font-medium tracking-[0.12em] text-stone-500 uppercase">
                    {{ t("settings.account.createAccountCard.accountType") }}
                  </p>
                  <p class="mt-1 text-sm text-stone-900">
                    {{ t("settings.account.createAccountCard.accountTypeValue") }}
                  </p>
                </div>
                <div class="rounded-xl border border-stone-200 bg-stone-50 px-3 py-3">
                  <p class="text-xs font-medium tracking-[0.12em] text-stone-500 uppercase">
                    {{ t("settings.account.createAccountCard.initialRole") }}
                  </p>
                  <p class="mt-1 text-sm text-stone-900">
                    {{ t("settings.account.createAccountCard.initialRoleValue") }}
                  </p>
                </div>
              </div>
            </div>

            <AppDialog
              :open="passwordDialogOpen"
              :title="t('settings.account.passwordDialog.title')"
              :description="t('settings.account.passwordDialog.description')"
              @close="closePasswordDialog"
            >
              <form class="space-y-4" @submit.prevent="handlePasswordSubmit">
                <div class="grid gap-4 md:grid-cols-3">
                  <label class="block">
                    <span class="mb-2 block text-sm font-medium text-stone-900">
                      {{ t("settings.account.passwordDialog.currentPassword") }}
                    </span>
                    <div
                      class="flex items-center gap-2 rounded-lg border border-stone-200 bg-white px-3"
                    >
                      <input
                        v-model="currentPassword"
                        :type="currentPasswordVisible ? 'text' : 'password'"
                        autocomplete="current-password"
                        class="min-w-0 flex-1 bg-transparent py-2 text-sm text-stone-900 focus:outline-none"
                      />
                      <button
                        type="button"
                        class="shrink-0 text-xs font-medium text-stone-500 hover:text-stone-900"
                        @click="currentPasswordVisible = !currentPasswordVisible"
                      >
                        {{
                          currentPasswordVisible
                            ? t("common.actions.hide")
                            : t("common.actions.show")
                        }}
                      </button>
                    </div>
                  </label>
                  <label class="block">
                    <span class="mb-2 block text-sm font-medium text-stone-900">
                      {{ t("settings.account.passwordDialog.newPassword") }}
                    </span>
                    <div
                      class="flex items-center gap-2 rounded-lg border border-stone-200 bg-white px-3"
                    >
                      <input
                        v-model="newPassword"
                        :type="newPasswordVisible ? 'text' : 'password'"
                        autocomplete="new-password"
                        class="min-w-0 flex-1 bg-transparent py-2 text-sm text-stone-900 focus:outline-none"
                      />
                      <button
                        type="button"
                        class="shrink-0 text-xs font-medium text-stone-500 hover:text-stone-900"
                        @click="newPasswordVisible = !newPasswordVisible"
                      >
                        {{
                          newPasswordVisible ? t("common.actions.hide") : t("common.actions.show")
                        }}
                      </button>
                    </div>
                  </label>
                  <label class="block">
                    <span class="mb-2 block text-sm font-medium text-stone-900">
                      {{ t("settings.account.passwordDialog.confirmPassword") }}
                    </span>
                    <div
                      class="flex items-center gap-2 rounded-lg border border-stone-200 bg-white px-3"
                    >
                      <input
                        v-model="confirmPassword"
                        :type="confirmPasswordVisible ? 'text' : 'password'"
                        autocomplete="new-password"
                        class="min-w-0 flex-1 bg-transparent py-2 text-sm text-stone-900 focus:outline-none"
                      />
                      <button
                        type="button"
                        class="shrink-0 text-xs font-medium text-stone-500 hover:text-stone-900"
                        @click="confirmPasswordVisible = !confirmPasswordVisible"
                      >
                        {{
                          confirmPasswordVisible
                            ? t("common.actions.hide")
                            : t("common.actions.show")
                        }}
                      </button>
                    </div>
                  </label>
                </div>
                <p
                  v-if="passwordFormError"
                  class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700"
                >
                  {{ passwordFormError }}
                </p>
                <div class="flex justify-end gap-3">
                  <button
                    type="button"
                    class="ui-btn-secondary px-4 py-2 text-sm"
                    @click="closePasswordDialog"
                  >
                    {{ t("common.actions.cancel") }}
                  </button>
                  <button
                    type="submit"
                    class="ui-btn-primary px-4 py-2 text-sm"
                    :disabled="workspaceStore.passwordChangeLoading"
                  >
                    {{
                      workspaceStore.passwordChangeLoading
                        ? t("settings.account.passwordDialog.submitting")
                        : t("settings.account.passwordDialog.submit")
                    }}
                  </button>
                </div>
              </form>
            </AppDialog>

            <AppDialog
              :open="accountCreationDialogOpen"
              :title="t('settings.account.createAccountDialog.title')"
              :description="t('settings.account.createAccountDialog.description')"
              @close="closeAccountCreationDialog"
            >
              <form class="space-y-4" @submit.prevent="handleCreateAccountSubmit">
                <div class="grid gap-4 md:grid-cols-3">
                  <label class="block">
                    <span class="mb-2 block text-sm font-medium text-stone-900">
                      {{ t("settings.account.createAccountDialog.account") }}
                    </span>
                    <input
                      v-model="newAccountEmail"
                      type="text"
                      autocomplete="username"
                      :placeholder="t('settings.account.createAccountDialog.placeholders.account')"
                      class="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
                    />
                  </label>
                  <label class="block">
                    <span class="mb-2 block text-sm font-medium text-stone-900">
                      {{ t("settings.account.createAccountDialog.displayName") }}
                    </span>
                    <input
                      v-model="newAccountName"
                      type="text"
                      :placeholder="
                        t('settings.account.createAccountDialog.placeholders.displayName')
                      "
                      class="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
                    />
                  </label>
                  <label class="block">
                    <span class="mb-2 block text-sm font-medium text-stone-900">
                      {{ t("settings.account.createAccountDialog.initialPassword") }}
                    </span>
                    <div
                      class="flex items-center gap-2 rounded-lg border border-stone-200 bg-white px-3"
                    >
                      <input
                        v-model="newAccountPassword"
                        :type="newAccountPasswordVisible ? 'text' : 'password'"
                        autocomplete="new-password"
                        class="min-w-0 flex-1 bg-transparent py-2 text-sm text-stone-900 focus:outline-none"
                      />
                      <button
                        type="button"
                        class="shrink-0 text-xs font-medium text-stone-500 hover:text-stone-900"
                        @click="newAccountPasswordVisible = !newAccountPasswordVisible"
                      >
                        {{
                          newAccountPasswordVisible
                            ? t("common.actions.hide")
                            : t("common.actions.show")
                        }}
                      </button>
                    </div>
                  </label>
                </div>
                <p
                  v-if="newAccountFormError"
                  class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700"
                >
                  {{ newAccountFormError }}
                </p>
                <div class="flex justify-end gap-3">
                  <button
                    type="button"
                    class="ui-btn-secondary px-4 py-2 text-sm"
                    @click="closeAccountCreationDialog"
                  >
                    {{ t("common.actions.cancel") }}
                  </button>
                  <button
                    type="submit"
                    class="ui-btn-primary px-4 py-2 text-sm"
                    :disabled="workspaceStore.accountCreationLoading"
                  >
                    {{
                      workspaceStore.accountCreationLoading
                        ? t("settings.account.createAccountDialog.submitting")
                        : t("settings.account.createAccountDialog.submit")
                    }}
                  </button>
                </div>
              </form>
            </AppDialog>
          </div>
        </div>
      </article>

      <article
        class="grid grid-cols-1 items-start gap-x-10 gap-y-5 md:grid-cols-[minmax(0,13rem)_1fr]"
      >
        <div>
          <h3 class="text-base leading-6 font-semibold text-stone-900">
            {{ t("settings.printing.title") }}
          </h3>
        </div>
        <div class="min-w-0">
          <div class="ui-settings-group">
            <div v-if="workspaceStore.workspaceSyncError" class="rounded-xl bg-amber-50 p-4">
              <p class="text-sm font-medium text-amber-900">
                {{ t("settings.printing.syncErrorTitle") }}
              </p>
              <p class="mt-1 text-sm text-amber-700">
                {{ workspaceStore.workspaceSyncError }}
              </p>
            </div>
            <div class="ui-settings-row">
              <div class="ui-settings-copy">
                <p class="text-sm font-medium text-stone-900">
                  {{ t("settings.printing.defaultDevice") }}
                </p>
              </div>
              <select
                :value="workspaceStore.defaultDeviceId"
                :disabled="workspaceStore.devices.every((device) => device.status === 'offline')"
                class="rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-900"
                @change="handleDefaultDeviceChange"
              >
                <option
                  v-if="workspaceStore.devices.every((device) => device.status === 'offline')"
                  value=""
                >
                  {{ t("settings.printing.noDefaultDevice") }}
                </option>
                <option
                  v-for="device in workspaceStore.devices.filter(
                    (device) => device.status !== 'offline',
                  )"
                  :key="device.id"
                  :value="device.id"
                >
                  {{ device.name }}
                </option>
              </select>
            </div>
            <div class="ui-settings-row">
              <div class="ui-settings-copy">
                <p class="text-sm font-medium text-stone-900">
                  {{ t("settings.printing.tutorialTab") }}
                </p>
                <p class="mt-0.5 text-sm text-stone-500">
                  {{
                    workspaceStore.tutorialTabEnabled
                      ? t("settings.printing.tutorialTabShown")
                      : t("settings.printing.tutorialTabHidden")
                  }}
                </p>
              </div>
              <button
                type="button"
                class="ui-toggle"
                :class="{ 'is-on': workspaceStore.tutorialTabEnabled }"
                :aria-label="
                  workspaceStore.tutorialTabEnabled
                    ? t('settings.printing.toggleAria.disableTutorialTab')
                    : t('settings.printing.toggleAria.enableTutorialTab')
                "
                :aria-pressed="workspaceStore.tutorialTabEnabled"
                @click="workspaceStore.setTutorialTabEnabled(!workspaceStore.tutorialTabEnabled)"
              >
                <span class="ui-toggle-thumb" />
              </button>
            </div>
          </div>
        </div>
      </article>

      <article
        class="grid grid-cols-1 items-start gap-x-10 gap-y-5 md:grid-cols-[minmax(0,13rem)_1fr]"
      >
        <div>
          <h3 class="text-base leading-6 font-semibold text-stone-900">
            {{ t("settings.appearance.title") }}
          </h3>
        </div>
        <div class="min-w-0">
          <div class="ui-settings-choice-grid">
            <button
              v-for="theme in themes"
              :key="theme.value"
              type="button"
              class="ui-btn-secondary justify-center py-2 text-sm"
              :class="
                theme.value === workspaceStore.selectedTheme
                  ? 'border-stone-300 bg-white text-stone-900 ring-1 ring-stone-200/70'
                  : 'border-transparent bg-transparent text-stone-600 shadow-none hover:border-stone-200 hover:bg-white'
              "
              :aria-pressed="theme.value === workspaceStore.selectedTheme"
              @click="workspaceStore.setTheme(theme.value)"
            >
              {{ theme.label }}
            </button>
          </div>
          <p class="mt-2 text-sm text-stone-500">
            {{
              t("settings.appearance.currentTheme", {
                value: getThemeDescription(workspaceStore.selectedTheme),
              })
            }}
          </p>
          <p class="mt-1 text-sm text-stone-500">
            {{ t("settings.appearance.description") }}
          </p>
        </div>
      </article>

      <article
        class="grid grid-cols-1 items-start gap-x-10 gap-y-5 md:grid-cols-[minmax(0,13rem)_1fr]"
      >
        <div>
          <h3 class="text-base leading-6 font-semibold text-stone-900">
            {{ t("settings.language.title") }}
          </h3>
        </div>
        <div class="min-w-0">
          <div class="ui-settings-choice-grid">
            <button
              v-for="option in localeOptions"
              :key="option.value"
              type="button"
              class="ui-btn-secondary justify-center py-2 text-sm"
              :class="
                option.value === workspaceStore.localePreference
                  ? 'border-stone-300 bg-white text-stone-900 ring-1 ring-stone-200/70'
                  : 'border-transparent bg-transparent text-stone-600 shadow-none hover:border-stone-200 hover:bg-white'
              "
              :aria-pressed="option.value === workspaceStore.localePreference"
              @click="workspaceStore.setLocale(option.value)"
            >
              {{ option.label }}
            </button>
          </div>
          <p class="mt-2 text-sm text-stone-500">
            {{
              t("settings.language.current", {
                value:
                  localeOptions.find((option) => option.value === workspaceStore.localePreference)
                    ?.label ?? workspaceStore.localePreference,
              })
            }}
          </p>
          <p class="mt-1 text-sm text-stone-500">
            {{ t("settings.language.description") }}
          </p>
        </div>
      </article>

      <article
        class="grid grid-cols-1 items-start gap-x-10 gap-y-5 md:grid-cols-[minmax(0,13rem)_1fr]"
      >
        <div>
          <h3 class="text-base leading-6 font-semibold text-stone-900">
            {{ t("settings.ai.title") }}
          </h3>
        </div>
        <div class="min-w-0">
          <div class="ui-settings-group">
            <div
              v-if="workspaceStore.aiConfigLoading"
              class="rounded-xl border border-stone-200 bg-stone-50 px-4 py-3"
            >
              <p class="text-sm text-stone-600">{{ t("settings.ai.loading") }}</p>
            </div>

            <div class="rounded-xl border border-stone-200 bg-stone-50 p-4">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <span
                  class="ui-status-badge"
                  :class="getServiceBindingStatusBadgeClass(workspaceStore.aiConfigSummary.bound)"
                >
                  {{
                    workspaceStore.aiConfigSummary.bound
                      ? t("settings.ai.configured")
                      : t("settings.ai.notConfigured")
                  }}
                </span>
                <button
                  v-if="workspaceStore.isAdmin"
                  type="button"
                  class="ui-btn-primary px-4 py-2 text-sm"
                  @click="openAIConfigDialog"
                >
                  {{ t("settings.ai.edit") }}
                </button>
              </div>

              <div class="mt-4 grid gap-4 md:grid-cols-2">
                <div>
                  <p class="text-xs font-medium tracking-[0.12em] text-stone-500 uppercase">
                    {{ t("settings.ai.provider") }}
                  </p>
                  <p class="mt-1 text-sm text-stone-900">
                    {{
                      workspaceStore.aiConfigSummary.bound
                        ? workspaceStore.aiConfigSummary.providerName || AI_PROVIDER_NAME_FALLBACK
                        : t("settings.ai.notConfigured")
                    }}
                  </p>
                </div>
                <div>
                  <p class="text-xs font-medium tracking-[0.12em] text-stone-500 uppercase">
                    {{ t("settings.ai.model") }}
                  </p>
                  <p class="mt-1 text-sm text-stone-900">
                    {{
                      workspaceStore.aiConfigSummary.bound
                        ? workspaceStore.aiConfigSummary.model || AI_MODEL_FALLBACK
                        : t("settings.ai.notConfigured")
                    }}
                  </p>
                </div>
              </div>
            </div>

            <AppDialog
              :open="aiConfigDialogOpen"
              :title="t('settings.ai.dialog.title')"
              @close="closeAIConfigDialog"
            >
              <form class="space-y-4" @submit.prevent="handleAIConfigSubmit">
                <div class="grid gap-4 md:grid-cols-2">
                  <label class="block">
                    <span class="mb-2 block text-sm font-medium text-stone-900">
                      {{ t("settings.ai.dialog.providerName") }}
                    </span>
                    <input
                      v-model="aiProviderName"
                      type="text"
                      :placeholder="t('settings.ai.dialog.placeholders.providerName')"
                      class="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
                    />
                  </label>
                  <label class="block">
                    <span class="mb-2 block text-sm font-medium text-stone-900">
                      {{ t("settings.ai.dialog.apiUrl") }}
                    </span>
                    <input
                      v-model="aiBaseUrl"
                      type="url"
                      :placeholder="t('settings.ai.dialog.placeholders.apiUrl')"
                      class="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
                    />
                  </label>
                  <label class="block">
                    <span class="mb-2 block text-sm font-medium text-stone-900">
                      {{ t("settings.ai.dialog.defaultModel") }}
                    </span>
                    <input
                      v-model="aiModel"
                      type="text"
                      :placeholder="t('settings.ai.dialog.placeholders.defaultModel')"
                      class="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
                    />
                  </label>
                  <label class="block">
                    <span class="mb-2 block text-sm font-medium text-stone-900">
                      {{ t("settings.ai.dialog.apiKey") }}
                    </span>
                    <input
                      v-model="aiApiKey"
                      :placeholder="
                        workspaceStore.aiConfigSummary.keyConfigured
                          ? t('settings.ai.dialog.apiKeyPlaceholderConfigured')
                          : t('settings.ai.dialog.apiKeyPlaceholderEmpty')
                      "
                      type="password"
                      autocomplete="off"
                      class="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
                    />
                  </label>
                </div>

                <p
                  v-if="aiConfigErrorMessage"
                  class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700"
                >
                  {{ aiConfigErrorMessage }}
                </p>

                <div class="flex justify-end gap-3">
                  <button
                    type="button"
                    class="ui-btn-secondary px-4 py-2 text-sm"
                    @click="closeAIConfigDialog"
                  >
                    {{ t("common.actions.cancel") }}
                  </button>
                  <button
                    type="submit"
                    class="ui-btn-primary px-4 py-2 text-sm"
                    :disabled="workspaceStore.aiConfigSaving || workspaceStore.aiConfigLoading"
                  >
                    {{
                      workspaceStore.aiConfigSaving
                        ? t("settings.ai.dialog.saving")
                        : t("settings.ai.dialog.submit")
                    }}
                  </button>
                </div>
              </form>
            </AppDialog>
          </div>
        </div>
      </article>

      <article
        class="grid grid-cols-1 items-start gap-x-10 gap-y-5 md:grid-cols-[minmax(0,13rem)_1fr]"
      >
        <div>
          <h3 class="text-base leading-6 font-semibold text-stone-900">
            {{ t("settings.plugins.title") }}
          </h3>
        </div>
        <div class="min-w-0 space-y-4">
          <template v-if="workspaceStore.isAuthenticated">
            <section class="ui-settings-group">
              <div
                class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-stone-200 bg-stone-50 px-5 py-4"
              >
                <p class="text-sm font-medium text-stone-900">
                  {{ t("settings.plugins.installed") }}
                </p>
                <button
                  v-if="workspaceStore.isAdmin"
                  type="button"
                  class="ui-btn-primary px-4 py-2 text-sm"
                  @click="openPluginAddDialog()"
                >
                  {{ t("settings.plugins.add") }}
                </button>
              </div>

              <div v-if="pluginInstallations.length === 0" class="ui-settings-row !items-start">
                <div class="ui-settings-copy">
                  <p class="text-sm font-medium text-stone-900">
                    {{ t("settings.plugins.empty") }}
                  </p>
                </div>
              </div>

              <div
                v-for="plugin in pluginInstallations"
                :key="plugin.installation.id"
                class="rounded-2xl border border-stone-200 bg-white px-5 py-5 shadow-xs"
              >
                <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                  <div class="min-w-0 flex-1">
                    <div class="flex flex-wrap items-center gap-2">
                      <p class="text-sm font-medium text-stone-900">
                        {{ plugin.installation.displayName }}
                      </p>
                      <span
                        class="ui-status-badge"
                        :class="getPluginInstallationStatusBadgeClass(plugin.installation.status)"
                      >
                        {{ getPluginInstallationStatusLabel(plugin.installation.status) }}
                      </span>
                      <span
                        v-if="plugin.binding"
                        class="ui-status-badge"
                        :class="getPluginBindingStatusBadgeClass(plugin)"
                      >
                        {{ getPluginBindingStatusLabel(plugin) }}
                      </span>
                    </div>
                    <p class="mt-2 text-xs text-stone-500">
                      {{
                        plugin.installation.sourceType === "git"
                          ? t("settings.plugins.sourceTypeGit")
                          : t("settings.plugins.sourceTypeZip")
                      }}
                      · v{{ plugin.installation.version }}
                    </p>
                    <div class="mt-3 flex flex-wrap gap-2">
                      <span
                        v-for="label in pluginPermissionLabels(plugin)"
                        :key="label"
                        class="ui-status-badge border-stone-200 bg-stone-50 text-stone-600"
                      >
                        {{ label }}
                      </span>
                    </div>
                    <p
                      v-if="plugin.binding?.lastError || plugin.installation.lastError"
                      class="mt-4 rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700"
                    >
                      {{ plugin.binding?.lastError || plugin.installation.lastError }}
                    </p>
                  </div>

                  <div class="flex shrink-0 flex-wrap items-center gap-3">
                    <button
                      v-if="
                        workspaceStore.availablePlugins.some(
                          (item) => item.installation.id === plugin.installation.id,
                        )
                      "
                      type="button"
                      class="ui-btn-primary px-4 py-2 text-sm"
                      @click="openPluginConfigDialog(plugin)"
                    >
                      {{ t("settings.plugins.configure") }}
                    </button>
                    <button
                      v-if="plugin.installation.status !== 'disabled' && workspaceStore.isAdmin"
                      type="button"
                      class="ui-btn-secondary px-3 py-1.5 text-sm"
                      :disabled="
                        workspaceStore.pluginSaving &&
                        workspaceStore.pluginSavingId === plugin.installation.id
                      "
                      @click="handlePluginDisable(plugin.installation.id)"
                    >
                      {{
                        workspaceStore.pluginSaving &&
                        workspaceStore.pluginSavingId === plugin.installation.id
                          ? t("settings.plugins.disabling")
                          : t("settings.plugins.disable")
                      }}
                    </button>
                  </div>
                </div>
              </div>
            </section>

            <AppDialog
              :open="pluginAddDialogOpen"
              :title="t('settings.plugins.addDialog.title')"
              @close="closePluginAddDialog"
            >
              <div class="space-y-4">
                <aside
                  role="note"
                  class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-950"
                >
                  <p class="font-medium">{{ t("settings.plugins.addDialog.securityTitle") }}</p>
                  <p class="mt-1 leading-6">
                    {{ t("settings.plugins.addDialog.securityWarning") }}
                  </p>
                  <p class="mt-1 leading-6">
                    {{ t("settings.plugins.addDialog.permissionsWarning") }}
                  </p>
                </aside>

                <div class="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    class="ui-btn-secondary justify-center py-2 text-sm"
                    :class="
                      pluginAddMode === 'zip'
                        ? 'border-stone-300 bg-white text-stone-900 ring-1 ring-stone-200/70'
                        : 'border-transparent bg-transparent text-stone-600 shadow-none hover:border-stone-200 hover:bg-white'
                    "
                    :aria-pressed="pluginAddMode === 'zip'"
                    @click="pluginAddMode = 'zip'"
                  >
                    {{ t("settings.plugins.addDialog.zipUpload") }}
                  </button>
                  <button
                    type="button"
                    class="ui-btn-secondary justify-center py-2 text-sm"
                    :class="
                      pluginAddMode === 'github'
                        ? 'border-stone-300 bg-white text-stone-900 ring-1 ring-stone-200/70'
                        : 'border-transparent bg-transparent text-stone-600 shadow-none hover:border-stone-200 hover:bg-white'
                    "
                    :aria-pressed="pluginAddMode === 'github'"
                    @click="pluginAddMode = 'github'"
                  >
                    {{ t("settings.plugins.addDialog.githubImport") }}
                  </button>
                </div>

                <div v-if="pluginAddMode === 'zip'" class="space-y-4">
                  <label
                    class="ui-btn-primary inline-flex w-full cursor-pointer justify-center px-4 py-2 text-sm"
                  >
                    <input
                      type="file"
                      accept=".zip,application/zip"
                      class="hidden"
                      :disabled="workspaceStore.pluginUploadLoading"
                      @change="handlePluginUpload"
                    />
                    {{
                      workspaceStore.pluginUploadLoading
                        ? t("settings.plugins.addDialog.uploading")
                        : t("settings.plugins.addDialog.chooseZip")
                    }}
                  </label>

                  <p
                    v-if="workspaceStore.pluginUploadLoading && workspaceStore.pluginUploadingName"
                    class="text-sm text-stone-500"
                  >
                    {{
                      t("settings.plugins.addDialog.processing", {
                        name: workspaceStore.pluginUploadingName,
                      })
                    }}
                  </p>

                  <p
                    v-if="pluginUploadError"
                    class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700"
                  >
                    {{ pluginUploadError }}
                  </p>
                </div>

                <form v-else class="space-y-4" @submit.prevent="handlePluginInstallFromGit">
                  <div class="grid gap-4">
                    <label class="block">
                      <span class="mb-2 block text-sm font-medium text-stone-900">
                        {{ t("settings.plugins.addDialog.repoUrl") }}
                      </span>
                      <input
                        v-model="pluginGitRepoUrl"
                        type="url"
                        :placeholder="t('settings.plugins.addDialog.placeholders.repoUrl')"
                        class="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
                      />
                    </label>

                    <label class="block">
                      <span class="mb-2 block text-sm font-medium text-stone-900">
                        {{ t("settings.plugins.addDialog.repoRef") }}
                      </span>
                      <input
                        v-model="pluginGitRepoRef"
                        type="text"
                        :placeholder="t('settings.plugins.addDialog.placeholders.repoRef')"
                        class="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
                      />
                    </label>

                    <label class="block">
                      <span class="mb-2 block text-sm font-medium text-stone-900">
                        {{ t("settings.plugins.addDialog.repoSubdir") }}
                      </span>
                      <input
                        v-model="pluginGitRepoSubdir"
                        type="text"
                        :placeholder="t('settings.plugins.addDialog.placeholders.repoSubdir')"
                        class="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
                      />
                    </label>
                  </div>

                  <p
                    v-if="pluginGitInstallError"
                    class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700"
                  >
                    {{ pluginGitInstallError }}
                  </p>

                  <div class="flex justify-end gap-3">
                    <button
                      type="button"
                      class="ui-btn-secondary px-4 py-2 text-sm"
                      @click="closePluginAddDialog"
                    >
                      {{ t("common.actions.cancel") }}
                    </button>
                    <button
                      type="submit"
                      class="ui-btn-primary px-4 py-2 text-sm"
                      :disabled="workspaceStore.pluginGitInstallLoading"
                    >
                      {{
                        workspaceStore.pluginGitInstallLoading
                          ? t("settings.plugins.addDialog.importing")
                          : t("settings.plugins.addDialog.submit")
                      }}
                    </button>
                  </div>
                </form>
              </div>
            </AppDialog>

            <AppDialog
              :open="activePluginConfig !== null"
              :title="
                activePluginConfig?.installation.displayName ??
                t('settings.plugins.configDialog.fallbackTitle')
              "
              :description="activePluginConfig?.manifest.description"
              @close="closePluginConfigDialog"
            >
              <template v-if="activePluginConfig">
                <div class="grid gap-3 md:grid-cols-2">
                  <div class="rounded-xl border border-stone-200 bg-stone-50 px-3 py-3">
                    <p class="text-xs font-medium tracking-[0.12em] text-stone-500 uppercase">
                      {{ t("settings.plugins.configDialog.installStatus") }}
                    </p>
                    <div class="mt-2 flex flex-wrap items-center gap-2">
                      <span
                        class="ui-status-badge"
                        :class="
                          getPluginInstallationStatusBadgeClass(
                            activePluginConfig.installation.status,
                          )
                        "
                      >
                        {{
                          getPluginInstallationStatusLabel(activePluginConfig.installation.status)
                        }}
                      </span>
                      <span
                        class="ui-status-badge"
                        :class="getPluginBindingStatusBadgeClass(activePluginConfig)"
                      >
                        {{ getPluginBindingStatusLabel(activePluginConfig) }}
                      </span>
                    </div>
                  </div>
                  <div class="rounded-xl border border-stone-200 bg-stone-50 px-3 py-3">
                    <p class="text-xs font-medium tracking-[0.12em] text-stone-500 uppercase">
                      {{ t("settings.plugins.configDialog.fetchPolicy") }}
                    </p>
                    <p class="mt-2 text-sm text-stone-900">
                      {{
                        t("settings.plugins.configDialog.fetchEveryMinutes", {
                          minutes: activePluginConfig.manifest.fetchPolicy.minutes,
                        })
                      }}
                    </p>
                    <p class="mt-1 text-xs text-stone-500">
                      {{
                        activePluginConfig.binding?.nextFetchAt
                          ? t("settings.plugins.configDialog.nextFetchAt", {
                              time: new Date(activePluginConfig.binding.nextFetchAt).toLocaleString(
                                workspaceStore.effectiveLocale,
                              ),
                            })
                          : t("settings.plugins.configDialog.noFetchScheduled")
                      }}
                    </p>
                  </div>
                </div>

                <form class="mt-5 space-y-4" @submit.prevent="handlePluginSave(activePluginConfig)">
                  <label
                    class="flex items-center justify-between gap-4 rounded-xl border border-stone-200 bg-stone-50 px-4 py-3"
                  >
                    <div>
                      <span class="block text-sm font-medium text-stone-900">
                        {{ t("settings.plugins.configDialog.enableWorkspaceBinding") }}
                      </span>
                      <span class="mt-1 block text-sm text-stone-500">
                        {{ t("settings.plugins.configDialog.enableWorkspaceBindingHint") }}
                      </span>
                    </div>
                    <input
                      :checked="
                        pluginEnabledDrafts[activePluginConfig.installation.id] ??
                        activePluginConfig.binding?.enabled ??
                        false
                      "
                      type="checkbox"
                      class="h-4 w-4 rounded border-stone-300 text-stone-900 focus:ring-stone-900"
                      @change="handlePluginEnabledChange(activePluginConfig, $event)"
                    />
                  </label>

                  <div v-if="activePluginConfig.manifest.workspaceConfigSchema.length === 0">
                    <p
                      class="rounded-xl border border-dashed border-stone-200 px-4 py-3 text-sm text-stone-500"
                    >
                      {{ t("settings.plugins.configDialog.noWorkspaceConfig") }}
                    </p>
                  </div>

                  <div v-else class="grid gap-4 md:grid-cols-2">
                    <div
                      v-for="field in activePluginConfig.manifest.workspaceConfigSchema"
                      :key="field.key"
                      class="block"
                      :class="{ 'md:col-span-2': field.type === 'textarea' }"
                    >
                      <span class="mb-2 block text-sm font-medium text-stone-900">
                        {{ field.label }}
                        <span v-if="field.required" class="text-rose-500">*</span>
                      </span>

                      <textarea
                        v-if="field.type === 'textarea'"
                        :value="String(pluginDraftValue(activePluginConfig, field) ?? '')"
                        rows="4"
                        :placeholder="field.description || ''"
                        class="w-full rounded-xl border border-stone-200 bg-white px-4 py-3 text-sm text-stone-900 placeholder:text-stone-400 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
                        @input="handlePluginInput(activePluginConfig, field, $event)"
                      />

                      <select
                        v-else-if="field.type === 'select'"
                        :value="String(pluginDraftValue(activePluginConfig, field) ?? '')"
                        class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
                        @change="handlePluginSelect(activePluginConfig, field, $event)"
                      >
                        <option
                          v-for="option in field.options ?? []"
                          :key="option.value"
                          :value="option.value"
                        >
                          {{ option.label }}
                        </option>
                      </select>

                      <label
                        v-else-if="field.type === 'checkbox'"
                        class="flex items-center gap-3 rounded-xl border border-stone-200 bg-white px-4 py-3"
                      >
                        <input
                          :checked="Boolean(pluginDraftValue(activePluginConfig, field))"
                          type="checkbox"
                          class="h-4 w-4 rounded border-stone-300 text-stone-900 focus:ring-stone-900"
                          @change="handlePluginCheckbox(activePluginConfig, field, $event)"
                        />
                        <span class="text-sm text-stone-700">
                          {{
                            field.description || t("settings.plugins.configDialog.checkboxFallback")
                          }}
                        </span>
                      </label>

                      <input
                        v-else
                        :value="pluginDraftValue(activePluginConfig, field)"
                        :type="
                          field.type === 'secret'
                            ? 'password'
                            : field.type === 'number'
                              ? 'number'
                              : field.type === 'url'
                                ? 'url'
                                : 'text'
                        "
                        :placeholder="
                          field.type === 'secret'
                            ? t('settings.plugins.configDialog.secretPlaceholder')
                            : field.description || ''
                        "
                        autocomplete="off"
                        class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 placeholder:text-stone-400 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
                        @input="handlePluginInput(activePluginConfig, field, $event)"
                      />

                      <span v-if="field.description" class="mt-2 block text-xs text-stone-500">
                        {{ field.description }}
                      </span>
                    </div>
                  </div>

                  <p
                    v-if="pluginTestMessages[activePluginConfig.installation.id]"
                    class="rounded-lg bg-stone-100 px-3 py-2 text-sm text-stone-700"
                  >
                    {{ pluginTestMessages[activePluginConfig.installation.id] }}
                  </p>

                  <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
                    <button
                      type="button"
                      class="ui-btn-secondary px-4 py-2 text-sm"
                      :disabled="
                        workspaceStore.pluginTestingId === activePluginConfig.installation.id
                      "
                      @click="handlePluginTest(activePluginConfig)"
                    >
                      {{
                        workspaceStore.pluginTestingId === activePluginConfig.installation.id
                          ? t("settings.plugins.configDialog.testing")
                          : t("settings.plugins.configDialog.test")
                      }}
                    </button>
                    <button
                      type="submit"
                      class="ui-btn-primary px-4 py-2 text-sm"
                      :disabled="
                        workspaceStore.pluginSavingId === activePluginConfig.installation.id
                      "
                    >
                      {{
                        workspaceStore.pluginSavingId === activePluginConfig.installation.id
                          ? t("settings.plugins.configDialog.saving")
                          : t("settings.plugins.configDialog.save")
                      }}
                    </button>
                  </div>
                </form>
              </template>
            </AppDialog>
          </template>

          <div v-else class="ui-settings-group">
            <div
              v-for="source in workspaceStore.activeSources"
              :key="source.id"
              class="ui-settings-row"
            >
              <div class="ui-settings-copy">
                <p class="text-sm font-medium text-stone-900">{{ source.name }}</p>
                <p class="mt-0.5 text-sm text-stone-500">
                  {{ source.type }} · {{ source.note }} ·
                  {{ workspaceStore.getSourceStatusLabel(source.status) }}
                </p>
              </div>
              <button
                type="button"
                class="ui-btn-secondary px-3 py-1.5 text-sm"
                @click="workspaceStore.toggleSourceConnection(source.id)"
              >
                {{
                  source.status === "connected"
                    ? t("settings.plugins.anonymousActions.disconnect")
                    : t("settings.plugins.anonymousActions.connect")
                }}
              </button>
            </div>
          </div>
        </div>
      </article>
    </div>
  </section>
</template>
