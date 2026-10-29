<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

import InkIcon from "@/components/InkIcon.vue";
import type { ConversationMessage } from "@/types/workspace";

const props = defineProps<{
  conversationId: string;
  messages: ConversationMessage[];
  generating: boolean;
  selecting: boolean;
  selectedIds: string[];
  emptyHint: string;
}>();
const emit = defineEmits<{ toggle: [id: string] }>();
const { t } = useI18n();
const viewport = ref<HTMLElement | null>(null);
const content = ref<HTMLElement | null>(null);
const pinned = ref(true);
const showJump = ref(false);
let observer: ResizeObserver | undefined;

function distanceFromBottom() {
  const el = viewport.value;
  return el ? el.scrollHeight - el.scrollTop - el.clientHeight : 0;
}
function handleScroll() {
  const distance = distanceFromBottom();
  pinned.value = distance <= 48;
  showJump.value = distance > 120;
}
function scrollToLatest() {
  pinned.value = true;
  showJump.value = false;
  if (viewport.value) viewport.value.scrollTop = viewport.value.scrollHeight;
}
function followContent() {
  if (pinned.value) scrollToLatest();
  else showJump.value = distanceFromBottom() > 120;
}
watch(
  () => [
    props.conversationId,
    props.messages.length,
    props.messages.at(-1)?.text,
    props.generating,
  ],
  (value, previous) => {
    if (value[0] !== previous[0]) scrollToLatest();
    else followContent();
  },
  { flush: "post" },
);
onMounted(async () => {
  await nextTick();
  scrollToLatest();
  if (typeof ResizeObserver === "undefined") return;
  observer = new ResizeObserver(followContent);
  if (content.value) observer.observe(content.value);
  if (viewport.value) observer.observe(viewport.value);
});
onBeforeUnmount(() => observer?.disconnect());
defineExpose({ scrollToLatest });
</script>

<template>
  <div class="conversation-message-area">
    <div
      ref="viewport"
      class="conversation-message-viewport"
      tabindex="0"
      role="region"
      :aria-label="t('conversations.messageRegion')"
      :aria-busy="generating"
      @scroll="handleScroll"
    >
      <div ref="content" class="conversation-message-content">
        <div v-if="!messages.length" class="conversation-empty">
          <span class="conversation-empty-icon"><InkIcon name="chat" /></span>
          <h3>{{ t("conversations.emptyState.title") }}</h3>
          <p>{{ emptyHint }}</p>
        </div>
        <article
          v-for="(message, index) in messages"
          :key="message.id"
          class="conversation-message"
          :class="[
            'conversation-message-' + message.role,
            { 'is-selected': selectedIds.includes(message.id), 'is-selecting': selecting },
          ]"
        >
          <label v-if="selecting" class="conversation-message-select">
            <input
              type="checkbox"
              :checked="selectedIds.includes(message.id)"
              :aria-label="t('conversations.selection.message', { index: index + 1 })"
              @change="emit('toggle', message.id)"
            />
          </label>
          <div class="conversation-message-body">
            <p class="conversation-message-author">
              {{
                t(
                  message.role === "user"
                    ? "store.labels.messageAuthorUser"
                    : "store.labels.messageAuthorAssistant",
                )
              }}
            </p>
            <div class="conversation-message-text">{{ message.text }}</div>
          </div>
        </article>
        <div v-if="generating" class="conversation-generating" role="status">
          <span aria-hidden="true" class="conversation-activity-dot" />
          {{ t("conversations.generating") }}
        </div>
      </div>
    </div>
    <button v-if="showJump" type="button" class="conversation-jump" @click="scrollToLatest">
      ↓ {{ t("conversations.actions.jumpToLatest") }}
    </button>
  </div>
</template>
