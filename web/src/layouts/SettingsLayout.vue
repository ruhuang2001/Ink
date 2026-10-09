<script setup lang="ts">
import { useI18n } from "vue-i18n";
import { RouterLink, RouterView, useRoute } from "vue-router";

const { t } = useI18n();
const route = useRoute();

const sections = [
  { name: "settings", path: "/settings", labelKey: "settings.navigation.preferences" },
  {
    name: "settings-devices",
    path: "/settings/devices",
    labelKey: "settings.navigation.devices",
  },
  { name: "settings-guide", path: "/settings/guide", labelKey: "settings.navigation.guide" },
] as const;

function isSectionActive(name: string) {
  return route.name === name;
}
</script>

<template>
  <section class="mx-auto max-w-6xl pt-3 sm:pt-5">
    <header>
      <h1 class="text-3xl font-semibold tracking-tight text-stone-950 sm:text-4xl">
        {{ t("navigation.settings.label") }}
      </h1>
    </header>

    <nav
      class="settings-section-nav mt-8 overflow-x-auto overflow-y-hidden border-b border-stone-200"
      aria-label="Settings"
    >
      <div class="flex min-w-max gap-7">
        <RouterLink
          v-for="section in sections"
          :key="section.name"
          :to="section.path"
          class="relative pb-3 text-sm font-medium text-stone-500 transition-colors hover:text-stone-900"
          :class="{
            'text-stone-950 after:absolute after:inset-x-0 after:-bottom-px after:h-0.5 after:bg-amber-600':
              isSectionActive(section.name),
          }"
        >
          {{ t(section.labelKey) }}
        </RouterLink>
      </div>
    </nav>

    <RouterView v-slot="{ Component, route }">
      <Transition name="page-swap" mode="out-in">
        <component :is="Component" :key="route.name" />
      </Transition>
    </RouterView>
  </section>
</template>

<style scoped>
.settings-section-nav {
  scrollbar-width: none;
}

.settings-section-nav::-webkit-scrollbar {
  display: none;
}
</style>
