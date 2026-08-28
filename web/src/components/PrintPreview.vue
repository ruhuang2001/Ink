<script setup lang="ts">
defineProps<{
  title: string;
  content: string;
  image?: string;
}>();
</script>

<template>
  <div class="print-preview-shell">
    <div class="print-preview-paper" role="img" :aria-label="`打印预览：${title}`">
      <img
        v-if="image"
        class="print-preview-image"
        :src="`data:image/png;base64,${image}`"
        :alt="`打印预览：${title}`"
      />
      <template v-else>
        <h4 class="print-preview-title">{{ title }}</h4>
        <div class="print-preview-content">{{ content }}</div>
      </template>
      <div class="print-preview-tear" aria-hidden="true" />
    </div>
  </div>
</template>

<style scoped>
.print-preview-shell {
  display: flex;
  justify-content: center;
  padding: 1rem;
  border-radius: 1rem;
  background: #e7e5e4;
}

.print-preview-paper {
  width: 384px;
  max-width: 100%;
  min-height: 180px;
  padding: 18px;
  color: #1c1917;
  background: #fff;
  box-shadow: 0 8px 24px rgb(28 25 23 / 12%);
  font-family: "PingFang SC", "Microsoft YaHei", "Noto Sans SC", system-ui, sans-serif;
  font-size: 22px;
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.print-preview-title {
  margin: 0 0 1.25rem;
  font-size: 22px;
  font-weight: 600;
}

.print-preview-image {
  display: block;
  width: 100%;
  height: auto;
  image-rendering: auto;
}

.print-preview-content {
  white-space: pre-wrap;
}

.print-preview-tear {
  height: 8px;
  margin: 1.25rem -18px -18px;
  background: repeating-linear-gradient(135deg, transparent 0 8px, #e7e5e4 8px 12px);
}
</style>
