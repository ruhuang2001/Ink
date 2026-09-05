<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";

import AppDialog from "@/components/AppDialog.vue";
import NavIcon from "@/components/NavIcon.vue";
import StylePicker from "@/components/StylePicker.vue";
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
const anonymousDemoRouteNames = new Set(["status", "conversations", "prints"]);
const loginTarget = computed(() => ({
  path: "/login",
  query: route.fullPath === DEFAULT_LOGIN_REDIRECT ? undefined : { redirect: route.fullPath },
}));
const showAnonymousDemoBanner = computed(
  () => !workspaceStore.isAuthenticated && anonymousDemoRouteNames.has(String(route.name ?? "")),
);
const visibleNavigationItems = computed(() =>
  navigationItems
    .filter((item) => item.name !== "tutorial" || workspaceStore.tutorialTabEnabled)
    .map((item) => ({
      ...item,
      label: t(item.labelKey),
      navHint: t(item.navHintKey),
    })),
);

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
  <div class="ink-shell flex min-h-[100dvh] flex-col text-stone-900">
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
          class="rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3"
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
            @click="handlePostLoginTutorialNavigate('/tutorial')"
          >
            {{ t("shell.postLoginTutorial.actions.viewTutorial") }}
          </button>
        </div>
      </div>
    </AppDialog>

    <header class="ink-header">
      <RouterLink to="/status" class="ink-brand" aria-label="Ink">
        <span class="ink-brand-mark" aria-hidden="true">i.</span><span>Ink</span>
      </RouterLink>
      <nav class="ink-desktop-nav" :aria-label="t('common.labels.workspace')">
        <RouterLink
          v-for="item in visibleNavigationItems"
          :key="item.name"
          :to="item.path"
          class="ink-nav-link"
          :class="{ 'is-current': route.name === item.name }"
          :aria-current="route.name === item.name ? 'page' : undefined"
        >
          <NavIcon :name="item.name" /><span>{{ item.label }}</span>
          <span v-if="item.name === 'prints' && pendingBadge" class="ink-count">{{
            pendingBadge
          }}</span>
        </RouterLink>
      </nav>
      <div class="ink-header-actions">
        <StylePicker />
        <button
          v-if="workspaceStore.isAuthenticated"
          type="button"
          class="ink-account"
          :title="workspaceStore.authUser?.email"
          @click="handleLogout"
        >
          {{ t("common.actions.logout") }}
        </button>
        <RouterLink v-else :to="loginTarget" class="ink-account"
          >{{ t("common.actions.login") }} <span aria-hidden="true">↗</span></RouterLink
        >
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

    <div v-if="showAnonymousDemoBanner" class="ink-demo">
      <div class="ink-demo-inner">
        <p class="leading-6">{{ t("shell.demoBanner.body") }}</p>
        <RouterLink :to="loginTarget" class="ink-demo-link">
          {{ t("shell.demoBanner.action") }}
        </RouterLink>
      </div>
    </div>

    <main
      class="ink-main mx-auto w-full max-w-7xl flex-1 px-4 pt-4 pb-[calc(5.75rem+env(safe-area-inset-bottom))] sm:px-5 sm:pt-5 lg:px-8 lg:py-8"
    >
      <RouterView v-slot="{ Component, route: currentRoute }">
        <Transition name="page-swap" mode="out-in">
          <component :is="Component" :key="currentRoute.fullPath" />
        </Transition>
      </RouterView>
    </main>

    <nav
      class="ink-mobile-nav fixed inset-x-0 bottom-0 z-30 border-t border-stone-200 bg-white/95 px-3 pt-2.5 pb-[calc(env(safe-area-inset-bottom)+0.5rem)] backdrop-blur lg:hidden"
    >
      <div
        class="mx-auto grid max-w-lg gap-1"
        :style="{ gridTemplateColumns: `repeat(${visibleNavigationItems.length}, minmax(0, 1fr))` }"
      >
        <RouterLink
          v-for="item in visibleNavigationItems"
          :key="item.name"
          :to="item.path"
          class="flex min-h-12 flex-col items-center justify-center rounded-xl px-2 py-2.5 text-center transition-colors"
          :class="
            route.name === item.name
              ? 'bg-stone-900 text-white shadow-sm'
              : 'text-stone-500 hover:bg-stone-50 hover:text-stone-900'
          "
        >
          <NavIcon :name="item.name" />
          <span class="block text-xs leading-tight font-medium">
            {{ item.label }}
            <span v-if="item.name === 'prints' && pendingBadge">· {{ pendingBadge }}</span>
          </span>
        </RouterLink>
      </div>
    </nav>
  </div>
</template>
