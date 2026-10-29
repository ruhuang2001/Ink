<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps<{
  title: string;
  content: string;
  image?: string;
}>();

const { t } = useI18n();
const stage = ref<HTMLElement | null>(null);
const availableWidth = ref(384);
const imageFailed = ref(false);
const hasImage = computed(() => !!props.image && !imageFailed.value);
const scale = computed(() => Math.min(1, availableWidth.value / 384));
let resizeObserver: ResizeObserver | undefined;

watch(
  () => props.image,
  () => {
    imageFailed.value = false;
  },
);

onMounted(() => {
  if (!stage.value) return;
  const updateWidth = () => {
    availableWidth.value = Math.max(1, stage.value!.clientWidth - 48);
  };
  updateWidth();
  if (typeof ResizeObserver !== "undefined") {
    resizeObserver = new ResizeObserver(updateWidth);
    resizeObserver.observe(stage.value);
  }
});

onBeforeUnmount(() => resizeObserver?.disconnect());
</script>

<template>
  <div
    ref="stage"
    class="print-preview"
    tabindex="0"
    role="region"
    :aria-label="t('prints.preview.canvas')"
  >
    <div class="print-preview-sheet" :style="{ zoom: scale }">
      <div class="print-preview-paper" :class="{ 'print-preview-paper-fallback': !hasImage }">
        <img
          v-if="hasImage"
          class="print-preview-image"
          :src="`data:image/png;base64,${image}`"
          :alt="t('prints.preview.imageAlt', { title })"
          width="384"
          decoding="async"
          @error="imageFailed = true"
        />
        <div v-else role="img" :aria-label="t('prints.preview.imageAlt', { title })">
          <h4 class="print-preview-title">{{ title }}</h4>
          <div class="print-preview-content">{{ content }}</div>
        </div>
      </div>
      <div class="print-preview-tear" aria-hidden="true" />
    </div>
  </div>
</template>

<style scoped>
.print-preview {
  max-height: min(70dvh, 680px);
  overflow: auto;
  padding: 28px 24px 36px;
  overscroll-behavior: contain;
  scrollbar-width: thin;
  scrollbar-color: var(--app-border-strong) transparent;
}

.print-preview:focus-visible {
  outline: 2px solid var(--app-ring);
  outline-offset: -2px;
}

.print-preview-sheet {
  width: 384px;
  margin-inline: auto;
  filter: drop-shadow(0 2px 3px rgb(28 25 23 / 8%)) drop-shadow(0 18px 32px rgb(28 25 23 / 14%));
}

.print-preview-paper {
  color: #1c1917;
  background: #fff;
  color-scheme: light;
}

/* The rendered PNG already includes the printer's margins. */
.print-preview-paper-fallback {
  min-height: 180px;
  padding: 18px;
  font-family: var(--font-cjk);
  font-size: 22px;
  line-height: 1.55;
  overflow-wrap: anywhere;
}

.print-preview-title {
  margin: 0 0 20px;
  padding-bottom: 14px;
  border-bottom: 1px dashed #d6d3d1;
  font-size: 22px;
  font-weight: 600;
  white-space: pre-wrap;
}

.print-preview-image {
  display: block;
  width: 384px;
  height: auto;
}

.print-preview-content {
  white-space: pre-wrap;
}

.print-preview-tear {
  height: 6px;
  background: #fff;
  mask: conic-gradient(from -45deg at 50% 100%, #000 90deg, transparent 0) 0 0 / 10px 6px;
}
</style>
