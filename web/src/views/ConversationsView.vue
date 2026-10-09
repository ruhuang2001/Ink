<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { RouterLink } from "vue-router";

import { useWorkspaceStore } from "@/stores/workspace";

const workspaceStore = useWorkspaceStore();
const { t } = useI18n();

const hasMessages = computed(() => (workspaceStore.activeConversation?.messages.length ?? 0) > 0);
const hasConversationHistory = computed(() =>
  workspaceStore.conversations.some((conversation) => conversation.messages.length > 0),
);
const emptyStateHint = computed(() =>
  hasConversationHistory.value
    ? t("conversations.emptyState.withHistory")
    : t("conversations.emptyState.firstConversation"),
);
function handleDraftInput(event: Event) {
  const target = event.target as HTMLTextAreaElement | null;
  workspaceStore.updateCurrentDraft(target?.value ?? "");
}

function handleConversationChange(event: Event) {
  const target = event.target as HTMLSelectElement | null;
  if (target?.value) workspaceStore.selectConversation(target.value);
}

function handleDeleteCurrentConversation() {
  const current = workspaceStore.activeConversation;

  if (!current) {
    return;
  }

  const hasContent = current.messages.length > 0 || current.draft.trim().length > 0;
  if (hasContent && typeof window !== "undefined") {
    const confirmed = window.confirm(t("conversations.confirmDelete", { title: current.title }));
    if (!confirmed) {
      return;
    }
  }

  workspaceStore.deleteConversation(current.id);
}
</script>

<template>
  <section class="mx-auto max-w-6xl space-y-8 pt-2 sm:space-y-10">
    <div class="ink-rule flex items-center justify-between gap-4 border-b pb-5 sm:pb-6">
      <h1 class="text-3xl font-semibold tracking-tight text-stone-950 sm:text-[2.7rem]">
        {{ t("navigation.conversations.label") }}
      </h1>
      <button
        type="button"
        class="ui-btn-secondary shrink-0 lg:hidden"
        @click="workspaceStore.createConversation"
      >
        {{ t("common.actions.new") }}
      </button>
    </div>

    <label class="block lg:hidden">
      <span class="mb-2 block text-sm font-medium text-stone-600">
        {{ t("conversations.recent") }}
      </span>
      <select
        :value="workspaceStore.activeConversationId"
        class="min-h-11 w-full rounded-md border border-stone-300 bg-white px-3 text-base text-stone-900"
        @change="handleConversationChange"
      >
        <option v-for="chat in workspaceStore.conversations" :key="chat.id" :value="chat.id">
          {{ chat.title }} · {{ workspaceStore.formatPrintTime(chat.updatedAt) }}
        </option>
      </select>
    </label>

    <div class="grid gap-8 lg:grid-cols-[280px_minmax(0,1fr)] lg:gap-12">
      <aside class="hidden min-w-0 space-y-4 lg:block">
        <div class="flex items-center justify-between">
          <div>
            <h3 class="text-base leading-6 font-semibold text-stone-900">
              {{ t("conversations.recent") }}
            </h3>
          </div>
          <button
            class="ui-btn-secondary px-3 py-1.5 text-sm"
            @click="workspaceStore.createConversation"
          >
            {{ t("common.actions.new") }}
          </button>
        </div>

        <div
          v-if="workspaceStore.conversations.length"
          class="ui-list-card max-h-[calc(100dvh-19rem)] overflow-y-auto"
        >
          <button
            v-for="chat in workspaceStore.conversations"
            :key="chat.id"
            type="button"
            class="ui-list-row block w-full cursor-pointer text-left"
            :class="{ 'is-active': workspaceStore.activeConversationId === chat.id }"
            @click="workspaceStore.selectConversation(chat.id)"
          >
            <div class="flex items-start justify-between gap-2">
              <p
                class="text-sm font-medium"
                :class="
                  workspaceStore.activeConversationId === chat.id
                    ? 'text-stone-900'
                    : 'text-stone-700'
                "
              >
                {{ chat.title }}
              </p>
              <span
                class="text-xs"
                :class="
                  workspaceStore.activeConversationId === chat.id
                    ? 'text-stone-500'
                    : 'text-stone-400'
                "
              >
                {{ workspaceStore.formatPrintTime(chat.updatedAt) }}
              </span>
            </div>
            <p class="mt-1 line-clamp-2 text-sm text-stone-500">
              {{ chat.preview }}
            </p>
          </button>
        </div>
      </aside>

      <div
        class="ink-paper flex min-h-[24rem] min-w-0 flex-col p-4 sm:min-h-[28rem] sm:p-6 lg:h-[calc(100dvh-17rem)] lg:min-h-[500px]"
      >
        <div
          class="mb-4 flex shrink-0 flex-col gap-3 border-b border-stone-200 pb-4 sm:flex-row sm:items-center sm:justify-between"
        >
          <div>
            <h3 class="text-base leading-6 font-semibold text-stone-900">
              {{
                workspaceStore.activeConversation?.title ?? t("conversations.currentConversation")
              }}
            </h3>
            <p class="mt-1 text-sm text-stone-500">
              {{
                t("conversations.defaultDevice", {
                  value: workspaceStore.activeDeviceLabel || t("common.labels.notSet"),
                })
              }}
            </p>
          </div>
          <div class="flex w-full sm:w-auto">
            <button
              type="button"
              class="ui-btn-secondary w-full px-3 py-1.5 text-sm sm:w-auto"
              @click="handleDeleteCurrentConversation"
            >
              {{ t("conversations.actions.deleteConversation") }}
            </button>
          </div>
        </div>

        <div
          v-if="!hasMessages"
          class="flex flex-1 items-center justify-center border-y border-stone-200 px-6 text-center"
        >
          <div class="space-y-2">
            <h4 class="text-base font-semibold text-stone-900">
              {{ t("conversations.emptyState.title") }}
            </h4>
            <p class="text-sm leading-6 text-stone-500">
              {{ emptyStateHint }}
            </p>
          </div>
        </div>

        <div v-else class="space-y-4 lg:flex-1 lg:overflow-y-auto lg:pr-2">
          <article
            v-for="message in workspaceStore.activeConversation?.messages"
            :key="message.id"
            class="space-y-2"
          >
            <div
              class="flex items-center gap-3"
              :class="message.role === 'user' ? 'justify-end' : 'justify-start'"
            >
              <button
                type="button"
                class="block max-w-[88%] rounded-xl border px-5 py-3.5 text-left text-[15px] leading-relaxed transition-all"
                :class="
                  message.role === 'user'
                    ? workspaceStore.selectedConversationMessageIds.includes(message.id)
                      ? 'rounded-br-sm border-stone-700 bg-stone-700 text-white ring-2 ring-stone-400 ring-offset-2'
                      : 'rounded-br-sm border-stone-800 bg-stone-800 text-white'
                    : workspaceStore.selectedConversationMessageIds.includes(message.id)
                      ? 'rounded-bl-sm border-stone-500 bg-stone-100 text-stone-950 ring-2 ring-stone-300 ring-offset-2'
                      : 'rounded-bl-sm border-stone-200 bg-white text-stone-900'
                "
                :aria-label="
                  workspaceStore.selectedConversationMessageIds.includes(message.id)
                    ? t('conversations.selection.deselect')
                    : t('conversations.selection.select')
                "
                :aria-pressed="workspaceStore.selectedConversationMessageIds.includes(message.id)"
                @click="workspaceStore.toggleConversationMessageSelection(message.id)"
              >
                {{ message.text }}
              </button>
            </div>
          </article>

          <article
            v-if="workspaceStore.isGenerating"
            class="max-w-[85%] rounded-xl rounded-bl-sm border border-stone-200 bg-white px-5 py-3.5 text-[15px] leading-relaxed text-stone-500"
          >
            {{ t("conversations.generating") }}
          </article>
        </div>

        <div class="mt-6 shrink-0 space-y-4 border-t border-stone-200 pt-4">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="flex flex-wrap gap-2">
              <button
                class="ui-btn-secondary whitespace-nowrap"
                @click="workspaceStore.createPrintFromSelectedMessages"
              >
                {{ t("conversations.actions.printSelected") }}
              </button>
              <button
                class="ui-btn-secondary whitespace-nowrap"
                @click="workspaceStore.createPrintFromConversation"
              >
                {{ t("conversations.actions.printConversation") }}
              </button>
            </div>
            <div class="flex items-center gap-2">
              <button
                class="ui-btn-secondary whitespace-nowrap"
                @click="workspaceStore.saveCurrentDraft"
              >
                {{ t("conversations.actions.saveDraft") }}
              </button>
              <RouterLink to="/prints" class="ui-btn-secondary whitespace-nowrap">
                {{ t("conversations.actions.viewPrintQueue") }}
              </RouterLink>
            </div>
          </div>

          <p
            v-if="workspaceStore.generationError"
            class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700"
          >
            {{ workspaceStore.generationError }}
          </p>

          <div class="ink-composer relative rounded-md transition-all">
            <textarea
              :value="workspaceStore.activeConversation?.draft ?? ''"
              rows="4"
              :placeholder="t('conversations.draftPlaceholder')"
              class="w-full resize-none border-0 bg-transparent p-4 text-[15px] leading-relaxed text-stone-900 placeholder:text-stone-400 focus:ring-0 focus:outline-none"
              @input="handleDraftInput"
            />
            <div
              class="flex flex-col gap-3 rounded-b-md border-t border-stone-200 bg-stone-50/50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
            >
              <div class="flex flex-wrap items-center gap-2 text-xs text-stone-500">
                <span>
                  {{
                    t("conversations.selectedCount", {
                      count: workspaceStore.selectedConversationMessageIds.length,
                    })
                  }}
                </span>
              </div>
              <div class="flex gap-2">
                <button
                  class="rounded-md p-1.5 text-stone-500 transition-colors hover:bg-stone-200/50 hover:text-stone-900"
                  :title="t('conversations.actions.regenerate')"
                  :disabled="workspaceStore.isGenerating"
                  @click="workspaceStore.regenerateLatestReply"
                >
                  <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                    />
                  </svg>
                </button>
                <button
                  class="ui-btn-primary px-4 py-1.5"
                  :disabled="workspaceStore.isGenerating"
                  @click="workspaceStore.sendCurrentDraft"
                >
                  {{
                    workspaceStore.isGenerating
                      ? t("conversations.sending")
                      : t("conversations.actions.send")
                  }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
