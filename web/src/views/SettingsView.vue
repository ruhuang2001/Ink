<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRouter } from "vue-router";

import AppDialog from "@/components/AppDialog.vue";
import { DEFAULT_LOGIN_REDIRECT } from "@/router/authRedirect";
import { useDesignStore } from "@/stores/design";
import { storeToRefs } from "@/stores/pinia";
import { useWorkspaceStore } from "@/stores/workspace";
import {
  getServiceBindingStatusBadgeClass,
  getUserRoleBadgeClass,
  getUserRoleLabel,
} from "@/utils/workspace";

const designStore = useDesignStore();
const router = useRouter();
function selectDesign(design: "paper" | "structured") {
  designStore.setDesign(design);
  const { design: _preview, ...query } = router.currentRoute.value.query;
  if (_preview !== undefined) void router.replace({ query });
}

const workspaceStore = useWorkspaceStore();
const { aiConfigSummary } = storeToRefs(workspaceStore);
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
const AI_PROVIDER_TYPE = "openai-compatible";
const AI_PROVIDER_NAME_FALLBACK = "OpenAI Compatible";
const AI_MODEL_FALLBACK = "gpt-4.1-mini";

const aiConfigErrorMessage = computed(
  () => aiFormError.value || workspaceStore.aiConfigSaveError || workspaceStore.aiConfigError,
);
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
    aiFormError.value = workspaceStore.aiConfigSaveError;
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

void openAIConfigDialog;
void closeAIConfigDialog;
void openPasswordDialog;
void closePasswordDialog;
void openAccountCreationDialog;
void closeAccountCreationDialog;
</script>

<template>
  <section class="mx-auto max-w-5xl space-y-8 pt-8">
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
                <p class="mt-0.5 text-sm text-stone-600">
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
                <p class="mt-0.5 text-sm text-stone-600">
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
            <div class="ui-settings-row">
              <p class="text-sm font-medium text-stone-900">
                {{ t("settings.account.passwordCard.title") }}
              </p>
              <button
                type="button"
                class="ui-btn-secondary px-3 py-1.5 text-sm"
                @click="openPasswordDialog"
              >
                {{ t("settings.account.passwordCard.action") }}
              </button>
            </div>
            <div v-if="workspaceStore.isAdmin" class="ui-settings-row">
              <p class="text-sm font-medium text-stone-900">
                {{ t("settings.account.createAccountCard.title") }}
              </p>
              <button
                type="button"
                class="ui-btn-secondary px-3 py-1.5 text-sm"
                @click="openAccountCreationDialog"
              >
                {{ t("settings.account.createAccountCard.action") }}
              </button>
            </div>

            <AppDialog
              :open="passwordDialogOpen"
              :title="t('settings.account.passwordDialog.title')"
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
                        class="shrink-0 text-xs font-medium text-stone-600 hover:text-stone-900"
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
                        class="shrink-0 text-xs font-medium text-stone-600 hover:text-stone-900"
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
                        class="shrink-0 text-xs font-medium text-stone-600 hover:text-stone-900"
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
                        class="shrink-0 text-xs font-medium text-stone-600 hover:text-stone-900"
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
          <div class="mb-4 grid grid-cols-2 gap-2" role="group" :aria-label="t('design.label')">
            <button
              v-for="design in ['paper', 'structured'] as const"
              :key="design"
              type="button"
              class="ui-btn-secondary"
              :aria-pressed="designStore.selectedDesign === design"
              :class="{ 'ring-2 ring-stone-400': designStore.selectedDesign === design }"
              @click="selectDesign(design)"
            >
              {{ t(`design.${design}`) }}
            </button>
          </div>
          <div v-if="designStore.selectedDesign === 'paper'" class="ui-settings-choice-grid">
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
            <div v-if="workspaceStore.aiConfigLoading" class="border-b border-stone-200 px-1 py-4">
              <p class="text-sm text-stone-600">{{ t("settings.ai.loading") }}</p>
            </div>

            <div class="border-b border-stone-200 px-1 py-4">
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
                  <p class="text-xs font-medium tracking-[0.12em] text-stone-600 uppercase">
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
                  <p class="text-xs font-medium tracking-[0.12em] text-stone-600 uppercase">
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
    </div>
  </section>
</template>
