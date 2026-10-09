<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watchEffect } from "vue";
import { useRoute } from "vue-router";

import { translate } from "@/i18n";
import { retryRouteLoad, routeLoadFailure } from "@/router/chunkRecovery";
import { useWorkspaceStore } from "@/stores/workspace";
import { resolveThemeMode } from "@/utils/workspace";

const route = useRoute();
const workspaceStore = useWorkspaceStore();
const themeColors = {
  light: "#f3f3f0",
  dark: "#14110f",
} as const;
const prefersDark = ref(false);
let colorSchemeMediaQuery: MediaQueryList | null = null;

function syncSystemColorScheme() {
  prefersDark.value = colorSchemeMediaQuery?.matches ?? false;
}

function handleSystemColorSchemeChange(event: MediaQueryListEvent) {
  prefersDark.value = event.matches;
}

onMounted(() => {
  if (workspaceStore.authSession && !workspaceStore.authUser && !workspaceStore.authBootstrapping) {
    void workspaceStore.initializeAuth();
  }

  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return;
  }

  colorSchemeMediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
  syncSystemColorScheme();

  if (typeof colorSchemeMediaQuery.addEventListener === "function") {
    colorSchemeMediaQuery.addEventListener("change", handleSystemColorSchemeChange);
    return;
  }

  colorSchemeMediaQuery.addListener(handleSystemColorSchemeChange);
});

onBeforeUnmount(() => {
  if (!colorSchemeMediaQuery) {
    return;
  }

  if (typeof colorSchemeMediaQuery.removeEventListener === "function") {
    colorSchemeMediaQuery.removeEventListener("change", handleSystemColorSchemeChange);
  } else {
    colorSchemeMediaQuery.removeListener(handleSystemColorSchemeChange);
  }
});

watchEffect(() => {
  if (typeof document === "undefined") {
    return;
  }

  document.documentElement.lang = workspaceStore.effectiveLocale;

  const resolvedTheme = resolveThemeMode(workspaceStore.selectedTheme, prefersDark.value);
  const root = document.documentElement;

  root.dataset.theme = workspaceStore.selectedTheme;
  root.dataset.colorMode = resolvedTheme;
  root.style.colorScheme = resolvedTheme;
  document.title = route.meta.titleKey
    ? `${translate("app.name")} · ${translate(route.meta.titleKey)}`
    : translate("app.name");
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute("content", themeColors[resolvedTheme] ?? themeColors.light);
});
</script>

<template>
  <div v-if="routeLoadFailure" role="alert" class="mx-auto max-w-xl p-8 text-stone-900">
    <p>{{ translate("shell.routeLoadError") }}</p>
    <button type="button" class="ui-btn-primary mt-4 px-4 py-2" @click="retryRouteLoad">
      {{ translate("shell.syncErrors.retry") }}
    </button>
  </div>
  <RouterView />
</template>
