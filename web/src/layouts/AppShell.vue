<script setup lang="ts">
import { computed } from "vue";
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
const anonymousDemoRouteNames = new Set(["conversations", "prints"]);
const loginTarget = computed(() => ({
  path: "/login",
  query: route.fullPath === DEFAULT_LOGIN_REDIRECT ? undefined : { redirect: route.fullPath },
}));
const showAnonymousDemoBanner = computed(
  () => !workspaceStore.isAuthenticated && anonymousDemoRouteNames.has(String(route.name ?? "")),
);
const visibleNavigationItems = computed(() =>
  navigationItems.map((item) => ({
    ...item,
    label: t(item.labelKey),
    navHint: t(item.navHintKey),
  })),
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
</script>

<template>
  <div class="flex min-h-[100dvh] flex-col bg-stone-50 text-stone-900">
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
      class="sticky top-0 z-40 border-b border-stone-200 bg-stone-50/95 px-4 pt-[calc(env(safe-area-inset-top)+0.75rem)] pb-3 backdrop-blur sm:px-5 lg:px-8 lg:py-3"
    >
      <div class="mx-auto flex max-w-7xl items-center justify-between gap-4 lg:hidden">
        <div class="flex items-center gap-3">
          <img src="/icon.jpg" alt="Ink Icon" class="h-8 w-8 rounded-lg object-contain" />
          <div class="flex items-center gap-2">
            <p class="text-sm font-semibold text-stone-950">Ink</p>
            <a
              href="https://github.com/ruhuang2001"
              target="_blank"
              rel="noreferrer"
              class="text-xs text-stone-400 transition-colors hover:text-stone-700"
            >
              Powered by ruhuang2001
            </a>
          </div>
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
            <img src="/icon.jpg" alt="Ink Icon" class="h-8 w-8 rounded-lg object-contain" />
            <p class="text-sm font-semibold text-stone-950">Ink</p>
            <a
              href="https://github.com/ruhuang2001"
              target="_blank"
              rel="noreferrer"
              class="text-xs text-stone-400 transition-colors hover:text-stone-700"
            >
              Powered by ruhuang2001
            </a>
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
      v-if="showAnonymousDemoBanner"
      class="border-b border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 lg:px-8"
    >
      <div
        class="mx-auto flex max-w-7xl flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"
      >
        <p class="leading-6">{{ t("shell.demoBanner.body") }}</p>
        <RouterLink
          :to="loginTarget"
          class="inline-flex items-center justify-center rounded-lg border border-amber-300 bg-white px-3 py-1.5 text-sm font-medium text-amber-900 transition-colors hover:border-amber-400 hover:bg-amber-100"
        >
          {{ t("shell.demoBanner.action") }}
        </RouterLink>
      </div>
    </div>

    <main
      class="mx-auto w-full max-w-7xl flex-1 px-4 pt-5 pb-[calc(5.75rem+env(safe-area-inset-bottom))] sm:px-5 sm:pt-7 lg:px-8 lg:py-10"
    >
      <RouterView v-slot="{ Component, route: currentRoute }">
        <Transition name="page-swap" mode="out-in">
          <component :is="Component" :key="currentRoute.fullPath" />
        </Transition>
      </RouterView>
    </main>

    <nav
      class="fixed inset-x-0 bottom-0 z-30 border-t border-stone-200 bg-stone-50/95 px-3 pt-1.5 pb-[calc(env(safe-area-inset-bottom)+0.35rem)] backdrop-blur lg:hidden"
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
