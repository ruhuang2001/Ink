<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

import InkIcon from "@/components/InkIcon.vue";

const props = defineProps<{
  modelValue: string;
  generating: boolean;
  saving: boolean;
  saveStatus: string;
  saveError: boolean;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: string];
  send: [];
  save: [];
}>();
const { t } = useI18n();
const textarea = ref<HTMLTextAreaElement | null>(null);
const composing = ref(false);
let observer: ResizeObserver | undefined;
let lastWidth = 0;

function resizeInput() {
  if (!textarea.value) return;
  textarea.value.style.height = "auto";
  textarea.value.style.height = Math.min(textarea.value.scrollHeight, 160) + "px";
}

function send() {
  if (!props.generating && props.modelValue.trim()) emit("send");
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key !== "Enter" || (!event.ctrlKey && !event.metaKey)) return;
  if (composing.value || event.isComposing || event.keyCode === 229 || event.repeat) return;
  event.preventDefault();
  send();
}

watch(
  () => props.modelValue,
  () => nextTick(resizeInput),
);
onMounted(() => {
  resizeInput();
  if (typeof ResizeObserver === "undefined" || !textarea.value) return;
  observer = new ResizeObserver(([entry]) => {
    if (entry && entry.contentRect.width !== lastWidth) {
      lastWidth = entry.contentRect.width;
      resizeInput();
    }
  });
  observer.observe(textarea.value);
});
onBeforeUnmount(() => observer?.disconnect());
</script>

<template>
  <div class="conversation-composer">
    <label for="conversation-draft" class="sr-only">{{
      t("conversations.draftPlaceholder")
    }}</label>
    <textarea
      id="conversation-draft"
      ref="textarea"
      :value="modelValue"
      rows="2"
      :placeholder="t('conversations.draftPlaceholder')"
      aria-describedby="conversation-shortcut"
      @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
      @keydown="handleKeydown"
      @compositionstart="composing = true"
      @compositionend="composing = false"
    />
    <div class="conversation-composer-controls">
      <button
        type="button"
        class="conversation-quiet-button conversation-save"
        :class="{ 'is-error': saveError }"
        :disabled="saving"
        :title="t('conversations.actions.saveDraft')"
        :aria-label="t('conversations.actions.saveDraft') + ': ' + saveStatus"
        @click="emit('save')"
      >
        <InkIcon name="save" />
        <span role="status">{{ saveStatus }}</span>
      </button>
      <button
        type="button"
        class="conversation-send"
        :disabled="generating || !modelValue.trim()"
        @click="send"
      >
        {{ t(generating ? "conversations.sending" : "conversations.actions.send") }}
        <InkIcon name="arrow" />
      </button>
    </div>
    <p id="conversation-shortcut" class="conversation-shortcut">
      {{ t("conversations.shortcut") }}
    </p>
  </div>
</template>
