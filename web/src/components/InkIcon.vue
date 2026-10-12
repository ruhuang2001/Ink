<script setup lang="ts">
withDefaults(
  defineProps<{ name?: "print" | "settings" | "chat" | "spark" | "plus" | "save" | "arrow" }>(),
  { name: "spark" },
);
</script>
<template>
  <svg
    class="ink-icon"
    viewBox="0 0 24 24"
    aria-hidden="true"
    fill="none"
    stroke="currentColor"
    stroke-width="1.8"
    stroke-linecap="square"
    stroke-linejoin="miter"
  >
    <template v-if="name === 'print'">
      <path
        d="M6 9V3h12v6M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2"
      />
      <path d="M6 14h12v7H6zM18 12h.01" />
    </template>
    <template v-else-if="name === 'settings'">
      <path
        d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M18.4 5.6l-2.1 2.1M7.7 16.3l-2.1 2.1"
      />
      <circle cx="12" cy="12" r="3.5" />
    </template>
    <template v-else-if="name === 'chat'">
      <path d="M4 5h16v11H8l-4 4z" />
      <path d="M8 9h8M8 12h5" />
    </template>
    <template v-else-if="name === 'plus'"><path d="M12 5v14M5 12h14" /></template>
    <template v-else-if="name === 'save'">
      <path d="M5 4h12l2 2v14H5zM8 4v6h8V4M8 20v-5h8v5" />
    </template>
    <template v-else-if="name === 'arrow'"><path d="M5 12h13M13 6l6 6-6 6" /></template>
    <template v-else>
      <path
        d="m12 3 1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8zM19 16l.8 2.2L22 19l-2.2.8L19 22l-.8-2.2L16 19l2.2-.8z"
      />
    </template>
  </svg>
</template>
