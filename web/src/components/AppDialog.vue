<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";

const props = defineProps<{
  open: boolean;
  title: string;
  description?: string;
  size?: "compact" | "wide";
}>();

const emit = defineEmits<{
  close: [];
}>();

const { t } = useI18n();
const dialogRef = ref<HTMLDialogElement | null>(null);

// A modal dialog opens in the browser's top layer. That layer is not clipped by
// any ancestor transform, so the dialog no longer needs a portal to escape the
// animated page, and focus trapping, background inertness, Escape, and focus
// restoration all come from the platform.
function syncOpen(open: boolean) {
  const dialog = dialogRef.value;
  if (!dialog) {
    return;
  }

  if (open) {
    if (!dialog.open) {
      dialog.showModal();
    }
    return;
  }

  if (dialog.open) {
    dialog.close();
  }
}

// The browser closes a modal on Escape by itself. Closing through the owner
// instead keeps its state and the DOM in step, so the dialog cannot be left
// closed while the app still believes it is open.
function handleCancel(event: Event) {
  event.preventDefault();
  emit("close");
}

// ::backdrop is not a node, so a click outside the panel lands on the dialog
// element itself. Anything inside the panel targets the panel or its children.
function handlePointerDown(event: PointerEvent) {
  if (event.target === dialogRef.value) {
    emit("close");
  }
}

watch(() => props.open, syncOpen, { flush: "post" });

// A watcher cannot apply the initial state: a dialog that mounts already open
// has no element to call showModal() on until mount finishes.
onMounted(() => syncOpen(props.open));

onBeforeUnmount(() => {
  if (dialogRef.value?.open) {
    dialogRef.value.close();
  }
});
</script>

<template>
  <dialog
    ref="dialogRef"
    class="app-dialog"
    :aria-label="title"
    @cancel="handleCancel"
    @pointerdown="handlePointerDown"
  >
    <section
      data-app-dialog-panel
      data-impeccable-ignore="hard-offset-shadow"
      class="max-h-[calc(100dvh-env(safe-area-inset-top)-0.5rem)] w-full overflow-y-auto rounded-t-[1.75rem] border border-stone-200 bg-[var(--app-surface)] px-5 pt-5 pb-[calc(env(safe-area-inset-bottom)+1.25rem)] shadow-2xl shadow-stone-900/10 sm:max-h-[min(42rem,calc(100dvh-3rem))] sm:rounded-3xl"
      :class="size === 'wide' ? 'preview-dialog sm:max-w-xl' : 'sm:max-w-md sm:p-6'"
    >
      <button
        v-if="size === 'wide'"
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
          <p v-if="description" class="mt-1 text-sm text-stone-600">{{ description }}</p>
        </div>
        <button
          type="button"
          class="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-stone-200 bg-white text-stone-600 transition-colors hover:border-stone-300 hover:text-stone-900"
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
  </dialog>
</template>

<style scoped>
.app-dialog {
  /* The dialog is a full-screen frame; the panel inside it is the card. */
  box-sizing: border-box;
  width: 100%;
  max-width: none;
  height: 100%;
  max-height: none;
  margin: 0;
  padding: 0;
  overflow: hidden;
  border: 0;
  background: transparent;
  color: inherit;
  opacity: 0;
  transition:
    opacity 180ms var(--ease-out-quart),
    display 180ms allow-discrete,
    overlay 180ms allow-discrete;
}

.app-dialog[open] {
  display: flex;
  align-items: flex-end;
  justify-content: center;
  opacity: 1;
}

/* Starting from transparent lets the entry fade in even though the dialog is
   only displayed once it is open. */
@starting-style {
  .app-dialog[open] {
    opacity: 0;
  }
}

.app-dialog::backdrop {
  background: var(--app-overlay);
  backdrop-filter: blur(4px);
  opacity: 0;
  transition:
    opacity 180ms var(--ease-out-quart),
    display 180ms allow-discrete,
    overlay 180ms allow-discrete;
}

.app-dialog[open]::backdrop {
  opacity: 1;
}

@starting-style {
  .app-dialog[open]::backdrop {
    opacity: 0;
  }
}

@media (min-width: 40rem) {
  .app-dialog[open] {
    align-items: center;
    padding: 1.5rem 1rem;
  }
}

.app-dialog > section {
  margin-inline: auto;
}

@media (prefers-reduced-motion: reduce) {
  .app-dialog,
  .app-dialog::backdrop {
    transition-duration: 0.01ms;
  }
}

.preview-dialog {
  position: relative;
  padding: 0;
  overflow-y: auto;
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
