<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { RouterLink } from "vue-router";

import ConversationComposer from "@/components/ConversationComposer.vue";
import ConversationMessages from "@/components/ConversationMessages.vue";
import InkIcon from "@/components/InkIcon.vue";
import { useWorkspaceStore } from "@/stores/workspace";

const workspaceStore = useWorkspaceStore();
const { t } = useI18n();
const selecting = ref(false);
const actions = ref<HTMLDetailsElement | null>(null);
const messages = ref<InstanceType<typeof ConversationMessages> | null>(null);
const saving = ref(false);
const saveResult = ref<"idle" | "saved" | "error">("idle");
let saveVersion = 0;
const hasMessages = computed(() => (workspaceStore.activeConversation?.messages.length ?? 0) > 0);
const selectedCount = computed(() => workspaceStore.selectedConversationMessageIds.length);
const emptyStateHint = computed(() =>
  workspaceStore.conversations.some((conversation) => conversation.messages.length > 0)
    ? t("conversations.emptyState.withHistory")
    : t("conversations.emptyState.firstConversation"),
);
const saveError = computed(
  () => saveResult.value === "error" || !!workspaceStore.workspaceSyncError,
);
const saveStatus = computed(() => {
  if (saveError.value) return t("conversations.save.failed");
  if (saving.value || workspaceStore.workspaceSyncing) return t("conversations.save.saving");
  if (saveResult.value === "saved") return t("conversations.save.saved");
  return t("conversations.actions.saveDraft");
});

function closeActions() {
  if (actions.value) actions.value.open = false;
}
function closeActionsWithFocus() {
  closeActions();
  actions.value?.querySelector("summary")?.focus();
}
function closeOutside(event: PointerEvent) {
  if (event.target instanceof Node && !actions.value?.contains(event.target)) closeActions();
}
function exitSelection() {
  selecting.value = false;
  workspaceStore.selectedConversationMessageIds = [];
}
function toggleSelection() {
  if (selecting.value) exitSelection();
  else selecting.value = true;
}
function selectConversation(event: Event) {
  workspaceStore.selectConversation((event.target as HTMLSelectElement).value);
}
function deleteConversation() {
  closeActions();
  const current = workspaceStore.activeConversation;
  if (!current) return;
  if (
    (current.messages.length || current.draft.trim()) &&
    !window.confirm(t("conversations.confirmDelete", { title: current.title }))
  )
    return;
  workspaceStore.deleteConversation(current.id);
}
async function send() {
  void nextTick(() => messages.value?.scrollToLatest());
  await workspaceStore.sendCurrentDraft();
}
async function saveDraft() {
  if (saving.value) return;
  const version = ++saveVersion;
  saving.value = true;
  const saved = await workspaceStore.saveCurrentDraft();
  if (version !== saveVersion) return;
  saving.value = false;
  saveResult.value = saved ? "saved" : "error";
}
function printConversation() {
  closeActions();
  void workspaceStore.createPrintFromConversation();
}
function regenerate() {
  closeActions();
  void workspaceStore.regenerateLatestReply();
}
watch(
  () => [workspaceStore.activeConversationId, workspaceStore.authUser?.id],
  () => {
    exitSelection();
    closeActions();
    saveVersion++;
    saving.value = false;
    saveResult.value = "idle";
  },
);
watch(
  () => workspaceStore.activeConversation?.draft,
  () => {
    saveVersion++;
    saving.value = false;
    saveResult.value = "idle";
  },
);
onMounted(() => document.addEventListener("pointerdown", closeOutside));
onBeforeUnmount(() => {
  saveVersion++;
  document.removeEventListener("pointerdown", closeOutside);
});
</script>

<template>
  <section class="conversation-workspace">
    <h1 class="sr-only">{{ t("navigation.conversations.label") }}</h1>
    <aside class="conversation-sidebar">
      <div class="conversation-sidebar-heading">
        <h2>{{ t("conversations.recent") }}</h2>
        <button
          type="button"
          class="conversation-quiet-button"
          @click="workspaceStore.createConversation"
        >
          <InkIcon name="plus" />{{ t("common.actions.new") }}
        </button>
      </div>
      <div class="conversation-list">
        <button
          v-for="chat in workspaceStore.conversations"
          :key="chat.id"
          type="button"
          class="conversation-list-item"
          :class="{ 'is-active': workspaceStore.activeConversationId === chat.id }"
          :aria-current="workspaceStore.activeConversationId === chat.id ? 'true' : undefined"
          @click="workspaceStore.selectConversation(chat.id)"
        >
          <span class="conversation-list-title">{{ chat.title }}</span>
          <span class="conversation-list-preview">{{ chat.preview }}</span>
          <span class="conversation-list-time">{{
            workspaceStore.formatPrintTime(chat.updatedAt)
          }}</span>
        </button>
      </div>
    </aside>

    <div class="conversation-panel">
      <header class="conversation-heading">
        <div class="conversation-heading-copy">
          <h2 class="conversation-desktop-title">
            {{ workspaceStore.activeConversation?.title ?? t("conversations.currentConversation") }}
          </h2>
          <label for="recent-conversation" class="sr-only">{{ t("conversations.recent") }}</label>
          <select
            id="recent-conversation"
            class="conversation-mobile-select"
            :value="workspaceStore.activeConversationId"
            @change="selectConversation"
          >
            <option v-for="chat in workspaceStore.conversations" :key="chat.id" :value="chat.id">
              {{ chat.title }}
            </option>
          </select>
          <p class="conversation-device">
            <InkIcon name="print" />{{
              workspaceStore.activeDeviceLabel || t("common.labels.notSet")
            }}
          </p>
        </div>
        <div class="conversation-heading-actions">
          <button
            type="button"
            class="conversation-quiet-button conversation-mobile-new"
            :aria-label="t('common.actions.new')"
            @click="workspaceStore.createConversation"
          >
            <InkIcon name="plus" />
          </button>
          <button
            type="button"
            class="conversation-quiet-button"
            :aria-pressed="selecting"
            :disabled="!hasMessages"
            @click="toggleSelection"
          >
            <InkIcon name="print" />{{
              t(
                selecting
                  ? "conversations.actions.finishSelection"
                  : "conversations.actions.selectForPrint",
              )
            }}
          </button>
          <details
            ref="actions"
            class="conversation-actions"
            @keydown.esc.prevent="closeActionsWithFocus"
          >
            <summary
              class="conversation-quiet-button"
              :aria-label="t('conversations.actions.more')"
            >
              •••
            </summary>
            <div class="conversation-action-popover">
              <button
                type="button"
                :disabled="!hasMessages || workspaceStore.isCreatingPrint"
                @click="printConversation"
              >
                {{ t("conversations.actions.printConversation") }}
              </button>
              <RouterLink to="/prints">{{ t("conversations.actions.viewPrintQueue") }}</RouterLink>
              <button
                type="button"
                :disabled="!hasMessages || workspaceStore.isGenerating"
                @click="regenerate"
              >
                {{ t("conversations.actions.regenerate") }}
              </button>
              <button type="button" class="is-danger" @click="deleteConversation">
                {{ t("conversations.actions.deleteConversation") }}
              </button>
            </div>
          </details>
        </div>
      </header>

      <ConversationMessages
        ref="messages"
        :conversation-id="workspaceStore.activeConversationId"
        :messages="workspaceStore.activeConversation?.messages ?? []"
        :generating="workspaceStore.isGenerating"
        :selecting="selecting"
        :selected-ids="workspaceStore.selectedConversationMessageIds"
        :empty-hint="emptyStateHint"
        @toggle="workspaceStore.toggleConversationMessageSelection"
      />

      <div class="conversation-bottom">
        <div v-if="selecting" class="conversation-selection-bar">
          <span role="status">{{
            t("conversations.selectedCount", { count: selectedCount })
          }}</span>
          <button
            type="button"
            class="conversation-print-selection"
            :disabled="!selectedCount || workspaceStore.isCreatingPrint"
            @click="workspaceStore.createPrintFromSelectedMessages"
          >
            <InkIcon name="print" />{{ t("conversations.actions.printSelected") }}
          </button>
        </div>
        <p v-if="workspaceStore.generationError" role="alert" class="conversation-error">
          {{ workspaceStore.generationError }}
        </p>
        <ConversationComposer
          :model-value="workspaceStore.activeConversation?.draft ?? ''"
          :generating="workspaceStore.isGenerating"
          :saving="saving || workspaceStore.workspaceSyncing"
          :save-status="saveStatus"
          :save-error="saveError"
          @update:model-value="workspaceStore.updateCurrentDraft"
          @send="send"
          @save="saveDraft"
        />
      </div>
    </div>
  </section>
</template>
