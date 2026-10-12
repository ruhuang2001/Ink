<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps<{
  open: boolean;
  title: string;
  description?: string;
  size?: "compact" | "wide";
}>();

const { t } = useI18n();
// Keep dialogs outside the animated page, which creates a containing block for fixed elements.
const portalTarget = ref<Element | null>(null);
onMounted(() => {
  portalTarget.value = document.querySelector(".ink-page");
});

const emit = defineEmits<{
  close: [];
}>();

const dialogRef = ref<HTMLElement | null>(null);
const closeButtonRef = ref<HTMLButtonElement | null>(null);
let previousActiveElement: HTMLElement | null = null;

function getFocusableElements() {
  if (!dialogRef.value) {
    return [];
  }

  return Array.from(
    dialogRef.value.querySelectorAll<HTMLElement>(
      'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    ),
  ).filter((element) => !element.hasAttribute("hidden") && element.tabIndex !== -1);
}

function restoreFocus() {
  previousActiveElement?.focus();
  previousActiveElement = null;
}

function trapFocus(event: KeyboardEvent) {
  const focusableElements = getFocusableElements();
  if (focusableElements.length === 0) {
    event.preventDefault();
    dialogRef.value?.focus();
    return;
  }

  const first = focusableElements[0];
  const last = focusableElements.at(-1);
  if (!last) {
    return;
  }

  const activeElement = document.activeElement;
  if (event.shiftKey && activeElement === first) {
    event.preventDefault();
    last.focus();
    return;
  }

  if (!event.shiftKey && activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (!props.open) {
    return;
  }

  const dialogs = document.querySelectorAll('[role="dialog"][aria-modal="true"]');
  const topDialog = dialogs.item(dialogs.length - 1);
  if (topDialog && topDialog !== dialogRef.value) return;

  if (event.key === "Escape") {
    event.preventDefault();
    emit("close");
    return;
  }

  if (event.key === "Tab") {
    trapFocus(event);
  }
}

watch(
  () => props.open,
  async (open, wasOpen) => {
    if (typeof document === "undefined") {
      return;
    }

    if (open) {
      previousActiveElement =
        document.activeElement instanceof HTMLElement ? document.activeElement : null;
      document.addEventListener("keydown", handleKeydown);
      await nextTick();
      closeButtonRef.value?.focus();
      return;
    }

    document.removeEventListener("keydown", handleKeydown);
    if (wasOpen) {
      restoreFocus();
    }
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  if (typeof document !== "undefined") {
    document.removeEventListener("keydown", handleKeydown);
  }

  restoreFocus();
});
</script>

<template>
  <Teleport :to="portalTarget ?? 'body'" :disabled="!portalTarget">
    <Transition name="dialog-fade">
      <div
        v-if="open"
        class="fixed inset-0 z-50 flex items-end justify-center bg-stone-950/35 px-0 py-0 backdrop-blur-sm sm:px-4 sm:py-6"
        :class="{ 'sm:items-center': size === 'wide' }"
        @click.self="emit('close')"
      >
        <section
          ref="dialogRef"
          class="max-h-[calc(100dvh-env(safe-area-inset-top)-0.5rem)] w-full overflow-y-auto rounded-t-[1.75rem] border border-stone-200 bg-[var(--app-surface)] px-5 pt-5 pb-[calc(env(safe-area-inset-bottom)+1.25rem)] shadow-2xl shadow-stone-900/10 sm:max-h-[min(42rem,calc(100dvh-3rem))] sm:rounded-3xl"
          :class="size === 'wide' ? 'preview-dialog sm:max-w-xl' : 'sm:max-w-md sm:p-6'"
          role="dialog"
          aria-modal="true"
          :aria-label="title"
          tabindex="-1"
        >
          <button
            v-if="size === 'wide'"
            ref="closeButtonRef"
            type="button"
            class="preview-dialog-close"
            :aria-label="t('common.actions.closeWindow')"
            @click="emit('close')"
          >
            ×
          </button>
          <div v-else class="flex items-start justify-between gap-4">
            <div class="min-w-0">
              <h3 class="text-lg font-semibold text-stone-900">{{ title }}</h3>
              <p v-if="description" class="mt-1 text-sm text-stone-500">{{ description }}</p>
            </div>
            <button
              ref="closeButtonRef"
              type="button"
              class="inline-flex h-9 w-9 items-center justify-center rounded-full border border-stone-200 bg-white text-stone-500 transition-colors hover:border-stone-300 hover:text-stone-900"
              :aria-label="t('common.actions.closeWindow')"
              @click="emit('close')"
            >
              ×
            </button>
          </div>

          <div :class="size === 'wide' ? 'preview-dialog-body' : 'mt-5'">
            <slot />
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.preview-dialog {
  position: relative;
  padding: 0;
  overflow: hidden;
  border-radius: 20px;
  background: var(--app-surface-soft);
}

.preview-dialog-close {
  position: absolute;
  top: 12px;
  right: 12px;
  z-index: 1;
  display: inline-flex;
  width: 32px;
  height: 32px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--app-border);
  border-radius: 999px;
  background: var(--app-surface-raised);
  color: var(--app-muted);
  font-size: 20px;
  font-weight: 300;
  line-height: 1;
  transition:
    color 150ms,
    border-color 150ms;
}

.preview-dialog-close:hover {
  border-color: var(--app-border-strong);
  color: var(--app-text-strong);
}

.preview-dialog-body {
  padding-bottom: env(safe-area-inset-bottom);
}
</style>
