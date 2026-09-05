<script setup lang="ts">
import { ref, watch } from "vue";
import { useI18n } from "vue-i18n";

const { t } = useI18n();
const styles = ["mono", "paper", "studio"] as const;
type VisualStyle = (typeof styles)[number];
let initial: VisualStyle = "mono";
try {
  const saved = localStorage.getItem("ink.visual-style");
  if (styles.includes(saved as VisualStyle)) initial = saved as VisualStyle;
} catch {
  /* Storage may be unavailable in private contexts. */
}
const selected = ref<VisualStyle>(initial);
watch(
  selected,
  (value) => {
    document.documentElement.dataset.visualStyle = value;
    try {
      localStorage.setItem("ink.visual-style", value);
    } catch {
      /* Keep the in-memory choice. */
    }
  },
  { immediate: true },
);
</script>

<template>
  <label class="ink-style-picker">
    <span class="ink-style-dot" aria-hidden="true" />
    <select v-model="selected" :aria-label="t('visualStyle.label')">
      <option v-for="style in styles" :key="style" :value="style">
        {{ t(`visualStyle.${style}`) }}
      </option>
    </select>
  </label>
</template>
