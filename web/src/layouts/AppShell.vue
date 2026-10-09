<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";

import AppDialog from "@/components/AppDialog.vue";
import { navigationItems } from "@/router";
import { DEFAULT_LOGIN_REDIRECT } from "@/router/authRedirect";
import { useWorkspaceStore } from "@/stores/workspace";

const route = useRoute();
const router = useRouter();
const workspaceStore = useWorkspaceStore();
const { t } = useI18n();
const postLoginTutorialSteps = computed(
  () =>
    [
      {
        title: t("shell.postLoginTutorial.steps.powerOn.title"),
        detail: t("shell.postLoginTutorial.steps.powerOn.detail"),
      },
      {
        title: t("shell.postLoginTutorial.steps.bind.title"),
        detail: t("shell.postLoginTutorial.steps.bind.detail"),
      },
      {
        title: t("shell.postLoginTutorial.steps.default.title"),
        detail: t("shell.postLoginTutorial.steps.default.detail"),
      },
    ] as const,
);

const pendingBadge = computed(() =>
  workspaceStore.pendingConfirmationCount > 0 ? workspaceStore.pendingConfirmationCount : "",
);
const syncErrors = computed(() =>
  [
    { key: "workspace", message: workspaceStore.workspaceSyncError },
    { key: "printers", message: workspaceStore.printerSyncError },
    { key: "ai", message: workspaceStore.aiConfigError },
    { key: "extensions", message: workspaceStore.pluginError },
  ].filter((error) => error.message),
);
const loginTarget = computed(() => ({
  path: "/login",
  query: route.fullPath === DEFAULT_LOGIN_REDIRECT ? undefined : { redirect: route.fullPath },
}));
const visibleNavigationItems = computed(() =>
  navigationItems.map((item) => ({
    ...item,
    label: t(item.labelKey),
    navHint: t(item.navHintKey),
  })),
);
const designs = ["paper", "studio", "quiet"] as const;
const design = ref(
  designs.includes(route.query.design as (typeof designs)[number])
    ? (route.query.design as (typeof designs)[number])
    : "paper",
);

watch(
  () => route.query.design,
  (value) => {
    if (designs.includes(value as (typeof designs)[number])) {
      design.value = value as (typeof designs)[number];
    }
  },
);

function isNavigationItemActive(path: string) {
  return path === "/settings" ? route.path.startsWith("/settings") : route.path === path;
}

function closePostLoginTutorial() {
  workspaceStore.closePostLoginTutorial();
}

async function handlePostLoginTutorialNavigate(path: string) {
  closePostLoginTutorial();
  await router.push(path);
}

async function handleLogout() {
  await workspaceStore.logout();
  await router.replace(DEFAULT_LOGIN_REDIRECT);
}

async function recoverSynchronization() {
  if (workspaceStore.workspaceConflict) {
    if (!window.confirm(t("shell.syncErrors.confirmReload"))) return;
    await workspaceStore.reloadConflictedWorkspace();
    return;
  }
  await workspaceStore.retrySynchronization();
}
</script>

<template>
  <div :data-design="design" class="ink-page flex min-h-[100dvh] flex-col text-stone-900">
    <AppDialog
      :open="workspaceStore.postLoginTutorialOpen"
      :title="t('shell.postLoginTutorial.title')"
      :description="t('shell.postLoginTutorial.description')"
      @close="closePostLoginTutorial"
    >
      <div class="space-y-4">
        <article
          v-for="(step, index) in postLoginTutorialSteps"
          :key="step.title"
          class="border-b border-stone-200 px-1 py-4 last:border-b-0"
        >
          <p class="text-xs font-medium tracking-[0.12em] text-stone-500 uppercase">
            {{ t("shell.postLoginTutorial.stepLabel", { index: index + 1 }) }}
          </p>
          <p class="mt-1 text-sm font-medium text-stone-900">{{ step.title }}</p>
          <p class="mt-1 text-sm leading-6 text-stone-600">{{ step.detail }}</p>
        </article>

        <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            type="button"
            class="ui-btn-secondary px-4 py-2 text-sm"
            @click="closePostLoginTutorial"
          >
            {{ t("shell.postLoginTutorial.actions.later") }}
          </button>
          <button
            type="button"
            class="ui-btn-primary px-4 py-2 text-sm"
            @click="handlePostLoginTutorialNavigate('/settings/guide')"
          >
            {{ t("shell.postLoginTutorial.actions.viewTutorial") }}
          </button>
        </div>
      </div>
    </AppDialog>

    <header
      class="ink-rule sticky top-0 z-40 border-b bg-[color:var(--app-surface-floating)] px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-3 backdrop-blur sm:px-5 lg:px-8 lg:py-3"
    >
      <div class="mx-auto flex max-w-7xl items-center justify-between gap-4 lg:hidden">
        <div class="flex items-center gap-3">
          <img src="/icon.png" alt="" class="h-8 w-8 rounded-md object-contain" />
          <p class="text-sm font-semibold text-stone-950">Ink</p>
          <div class="hidden sm:block">
            <p class="text-xs text-stone-500">
              {{
                route.meta.navHintKey
                  ? t(route.meta.navHintKey)
                  : route.meta.titleKey
                    ? t(route.meta.titleKey)
                    : t("common.labels.workspace")
              }}
            </p>
          </div>
        </div>

        <button
          v-if="workspaceStore.isAuthenticated"
          type="button"
          class="text-sm font-medium text-stone-600 hover:text-stone-900"
          @click="handleLogout"
        >
          {{ t("common.actions.logout") }}
        </button>
        <RouterLink
          v-else
          :to="loginTarget"
          class="text-sm font-medium text-stone-600 hover:text-stone-900"
        >
          {{ t("common.actions.login") }}
        </RouterLink>
      </div>

      <div class="mx-auto hidden max-w-7xl items-center justify-between lg:flex">
        <div class="flex items-center gap-8">
          <div class="flex items-center gap-3">
            <img src="/icon.png" alt="" class="h-8 w-8 rounded-md object-contain" />
            <p class="text-sm font-semibold text-stone-950">Ink</p>
          </div>

          <nav class="flex items-center gap-1">
            <RouterLink
              v-for="item in visibleNavigationItems"
              :key="item.name"
              :to="item.path"
              class="relative px-2 py-2 text-sm font-medium transition-colors after:absolute after:inset-x-2 after:-bottom-[0.8rem] after:h-0.5 after:origin-center after:scale-x-0 after:bg-amber-600 after:transition-transform"
              :class="
                isNavigationItemActive(item.path)
                  ? 'text-stone-950 after:scale-x-100'
                  : 'text-stone-500 hover:text-stone-900'
              "
            >
              <span>{{ item.label }}</span>
              <span
                v-if="item.name === 'prints' && pendingBadge"
                class="ml-2 inline-flex min-w-5 shrink-0 items-center justify-center rounded-full bg-stone-900 px-1.5 py-0.5 text-[11px] whitespace-nowrap text-white"
              >
                {{ pendingBadge }}
              </span>
            </RouterLink>
          </nav>
        </div>

        <div class="flex items-center gap-4">
          <template v-if="workspaceStore.isAuthenticated">
            <p class="text-sm text-stone-500">{{ workspaceStore.authUser?.email }}</p>
            <button
              type="button"
              class="text-sm font-medium text-stone-600 hover:text-stone-900"
              @click="handleLogout"
            >
              {{ t("common.actions.logout") }}
            </button>
          </template>
          <RouterLink
            v-else
            :to="loginTarget"
            class="text-sm font-medium text-stone-600 hover:text-stone-900"
          >
            {{ t("common.actions.login") }}
          </RouterLink>
        </div>
      </div>
    </header>

    <div
      v-if="workspaceStore.flashMessage"
      class="border-b px-4 py-3 text-sm lg:px-8"
      :class="
        workspaceStore.flashTone === 'success'
          ? 'border-emerald-100 bg-emerald-50 text-emerald-700'
          : workspaceStore.flashTone === 'error'
            ? 'border-rose-100 bg-rose-50 text-rose-700'
            : 'border-stone-200 bg-stone-50 text-stone-700'
      "
    >
      <div class="mx-auto max-w-7xl">
        {{ workspaceStore.flashMessage }}
      </div>
    </div>

    <div
      v-if="workspaceStore.isAuthenticated && syncErrors.length"
      role="alert"
      class="border-b border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 lg:px-8"
    >
      <div class="mx-auto max-w-7xl space-y-1">
        <p class="font-medium">{{ t("shell.syncErrors.title") }}</p>
        <ul class="space-y-1">
          <li v-for="error in syncErrors" :key="error.key">
            {{ t(`shell.syncErrors.${error.key}`, { message: error.message }) }}
          </li>
        </ul>
        <button
          type="button"
          class="ui-btn-secondary mt-2 px-3 py-1.5 text-sm"
          :disabled="
            workspaceStore.workspaceLoading ||
            workspaceStore.workspaceSyncing ||
            workspaceStore.isGenerating ||
            workspaceStore.isCreatingPrint
          "
          @click="recoverSynchronization"
        >
          {{
            t(
              workspaceStore.workspaceConflict
                ? "shell.syncErrors.reload"
                : "shell.syncErrors.retry",
            )
          }}
        </button>
        <button
          v-if="workspaceStore.workspaceConflict"
          type="button"
          class="ui-btn-secondary mt-2 ml-2 px-3 py-1.5 text-sm"
          :disabled="workspaceStore.workspaceLoading"
          @click="workspaceStore.downloadWorkspaceDraft"
        >
          {{ t("shell.syncErrors.download") }}
        </button>
      </div>
    </div>

    <main
      class="mx-auto w-full max-w-7xl flex-1 px-4 pt-6 pb-[calc(5.75rem+env(safe-area-inset-bottom))] sm:px-5 sm:pt-8 lg:px-8 lg:py-12"
    >
      <fieldset
        class="m-0 min-w-0 border-0 p-0"
        :disabled="workspaceStore.workspaceLoading"
        :inert="workspaceStore.workspaceLoading"
      >
        <RouterView v-slot="{ Component, route: currentRoute }">
          <Transition name="page-swap" mode="out-in">
            <component :is="Component" :key="currentRoute.fullPath" />
          </Transition>
        </RouterView>
      </fieldset>
    </main>

    <nav
      class="ink-rule fixed inset-x-0 bottom-0 z-30 border-t bg-[color:var(--app-surface-floating)] px-3 pt-1.5 pb-[calc(env(safe-area-inset-bottom)+0.35rem)] backdrop-blur lg:hidden"
    >
      <div
        class="mx-auto grid max-w-lg gap-1"
        :style="{ gridTemplateColumns: `repeat(${visibleNavigationItems.length}, minmax(0, 1fr))` }"
      >
        <RouterLink
          v-for="item in visibleNavigationItems"
          :key="item.name"
          :to="item.path"
          class="relative flex min-h-12 flex-col items-center justify-center px-2 py-2.5 text-center transition-colors after:absolute after:top-0 after:left-1/2 after:h-0.5 after:w-8 after:-translate-x-1/2 after:scale-x-0 after:bg-amber-600 after:transition-transform"
          :class="
            isNavigationItemActive(item.path)
              ? 'text-stone-950 after:scale-x-100'
              : 'text-stone-500 hover:text-stone-900'
          "
        >
          <span class="block text-xs leading-tight font-medium">
            {{ item.label }}
            <span v-if="item.name === 'prints' && pendingBadge">· {{ pendingBadge }}</span>
          </span>
        </RouterLink>
      </div>
    </nav>
  </div>
</template>
