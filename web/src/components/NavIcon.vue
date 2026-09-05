<script setup lang="ts">
defineProps<{ name: string }>();
const paths: Record<string, string> = {
  status: "M6 9V3h12v6M6 17H3V9h18v8h-3M6 14h12v7H6zM17 11h1",
  conversations: "M21 11a9 9 0 0 1-9 9H3l2-5a9 9 0 1 1 16-4ZM8 9h8M8 13h5",
  prints: "M7 3h10v4H7zM5 5H3v16h18V5h-2M7 12h10M7 16h6",
  settings: "M4 7h16M4 17h16M8 4v6M16 14v6",
  tutorial: "M4 3h12l4 4v14H4zM8 11h8M8 15h6",
};
</script>

<template>
  <svg
    width="20"
    height="20"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.6"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="paths[name] || paths.status" />
  </svg>
</template>
