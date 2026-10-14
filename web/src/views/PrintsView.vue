<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { RouterLink } from "vue-router";

import AppDialog from "@/components/AppDialog.vue";
import InkIcon from "@/components/InkIcon.vue";
import PrintPreview from "@/components/PrintPreview.vue";
import { renderPrintPreview } from "@/services/printers";
import { useWorkspaceStore } from "@/stores/workspace";
import { getPrintStatusBadgeClass } from "@/utils/workspace";

const workspaceStore = useWorkspaceStore();
const { t } = useI18n();

const activePrintGroups = computed(() =>
  [
    {
      status: "pending",
      title: t("prints.pending.awaitingConfirmation"),
      jobs: workspaceStore.pendingPrintJobs.filter((job) => job.status === "pending"),
    },
    {
      status: "queued",
      title: t("prints.pending.queued"),
      jobs: workspaceStore.pendingPrintJobs.filter((job) => job.status === "queued"),
    },
  ].filter((group) => group.jobs.length > 0),
);
const historyPrintJobs = computed(() =>
  workspaceStore.printHistoryJobs.filter(
    (job) => job.status !== "pending" && job.status !== "queued",
  ),
);

const printDialogOpen = ref(false);
const printTitle = ref("");
const printContent = ref("");
const printError = ref("");
const previewJob = ref<{ title: string; content: string; image?: string } | null>(null);
const previewLoading = ref(false);
const previewError = ref("");
let previewRequestVersion = 0;

watch(() => workspaceStore.authUser?.id, closePreview);
onBeforeUnmount(closePreview);

function handlePrintDeviceChange(jobId: string, event: Event) {
  const target = event.target as HTMLSelectElement | null;
  workspaceStore.updatePrintDevice(jobId, target?.value ?? workspaceStore.defaultDeviceId);
}

function openPrintDialog() {
  printDialogOpen.value = true;
  printTitle.value = "";
  printContent.value = "";
  printError.value = "";
}

function closePrintDialog() {
  printDialogOpen.value = false;
}

async function openPreview(item: { id?: string; title: string; content?: string }) {
  const requestVersion = ++previewRequestVersion;
  previewJob.value = { title: item.title, content: item.content ?? "" };
  previewError.value = "";
  previewLoading.value = workspaceStore.isAuthenticated;
  try {
    const detail = item.id ? await workspaceStore.loadPrintJobDetail(item.id) : item;
    if (requestVersion !== previewRequestVersion || !detail) return;
    const payload = { title: detail.title, content: detail.content ?? "" };
    previewJob.value = payload;
    previewLoading.value = false;
    const token = workspaceStore.authSession?.accessToken;
    if (!token) return;
    try {
      const image = await renderPrintPreview(token, payload);
      if (requestVersion === previewRequestVersion) previewJob.value = { ...payload, image };
    } catch {
      // Keep the CSS fallback visible when the preview endpoint is unavailable.
    }
  } catch {
    if (requestVersion === previewRequestVersion) previewError.value = t("prints.preview.error");
  } finally {
    if (requestVersion === previewRequestVersion) previewLoading.value = false;
  }
}

function closePreview() {
  previewRequestVersion++;
  previewJob.value = null;
  previewLoading.value = false;
  previewError.value = "";
}

function previewManualPrint() {
  printError.value = "";
  if (!printTitle.value.trim()) {
    printError.value = t("prints.printDialog.errors.titleRequired");
    return;
  }
  if (!printContent.value.trim()) {
    printError.value = t("prints.printDialog.errors.contentRequired");
    return;
  }
  void openPreview({ title: printTitle.value, content: printContent.value });
}

async function submitPrintDialog() {
  printError.value = "";

  if (!printTitle.value.trim()) {
    printError.value = t("prints.printDialog.errors.titleRequired");
    return;
  }

  if (!printContent.value.trim()) {
    printError.value = t("prints.printDialog.errors.contentRequired");
    return;
  }

  const created = await workspaceStore.createManualPrint({
    title: printTitle.value,
    content: printContent.value,
  });

  if (!created) {
    if (workspaceStore.printSubmissionUnknown) {
      closePrintDialog();
      closePreview();
      return;
    }
    printError.value =
      workspaceStore.flashTone === "error"
        ? workspaceStore.flashMessage
        : t("prints.printDialog.errors.createFailed");
    return;
  }

  closePrintDialog();
}
</script>

<template>
  <section class="print-page mx-auto max-w-6xl space-y-8 pt-2 sm:space-y-10">
    <header
      class="ink-rule flex flex-col gap-4 border-b pb-5 sm:flex-row sm:items-center sm:justify-between"
    >
      <div>
        <h1 class="sr-only">
          {{ t("navigation.prints.label") }}
        </h1>
        <p class="text-sm text-stone-500">
          {{ t("prints.defaultSettings.defaultDevice") }} ·
          {{ workspaceStore.activeDeviceLabel || t("prints.defaultSettings.notSet") }}
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2 sm:justify-end">
        <button class="ui-btn-primary flex-1 px-4 sm:flex-none" @click="openPrintDialog">
          <InkIcon name="plus" />{{ t("prints.actions.newPrint") }}
        </button>
        <RouterLink
          to="/settings/guide"
          class="min-h-11 w-full px-1 py-2 text-center text-sm font-medium text-stone-500 transition-colors hover:text-stone-900 sm:min-h-0 sm:w-auto sm:text-left"
        >
          {{ t("prints.actions.bindingTutorial") }}
        </RouterLink>
      </div>
    </header>

    <div class="space-y-14">
      <section class="print-pending">
        <div class="mb-4 flex items-center justify-between gap-4">
          <h2 class="text-xl font-semibold tracking-tight text-stone-900">
            {{ t("prints.pending.title") }}
          </h2>
          <span class="text-sm text-stone-500 tabular-nums">
            {{ workspaceStore.pendingPrintJobs.length }}
          </span>
        </div>

        <div
          v-if="workspaceStore.pendingPrintJobs.length === 0"
          class="ink-print-empty border-y border-stone-200 px-6 py-10 text-center"
        >
          <h4 class="text-base font-semibold text-stone-900">
            {{ t("prints.pending.emptyTitle") }}
          </h4>
          <p class="mt-2 text-sm text-stone-500">
            {{
              workspaceStore.isAuthenticated
                ? t("prints.pending.emptyAuthenticated")
                : t("prints.pending.emptyAnonymous")
            }}
          </p>
        </div>

        <div v-else class="space-y-8">
          <section v-for="group in activePrintGroups" :key="group.status">
            <h3 class="mb-2 text-sm font-medium text-stone-500">
              {{ group.title }} · {{ group.jobs.length }}
            </h3>
            <div class="print-queue">
              <article v-for="item in group.jobs" :key="item.id" class="print-ticket">
                <div class="min-w-0 flex-1">
                  <p class="text-base font-medium text-stone-900">{{ item.title }}</p>
                  <p class="mt-1 text-sm text-stone-500">
                    {{ item.source }} · {{ workspaceStore.getDeviceName(item.deviceId) }} ·
                    {{ workspaceStore.formatPrintTime(item.updatedAt) }}
                  </p>
                  <p
                    v-if="!workspaceStore.isAuthenticated && 'content' in item"
                    class="mt-3 max-w-2xl text-sm leading-6 text-stone-700"
                  >
                    “{{ item.content }}”
                  </p>

                  <div
                    class="mt-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"
                  >
                    <label class="flex min-w-0 items-center gap-2 text-sm text-stone-600">
                      <span class="shrink-0">{{ t("prints.pending.targetDevice") }}</span>
                      <select
                        :value="item.deviceId"
                        :disabled="workspaceStore.isAuthenticated && item.status === 'queued'"
                        class="min-w-0 rounded-md border border-stone-200 bg-white px-3 py-2 text-sm text-stone-900"
                        @change="handlePrintDeviceChange(item.id, $event)"
                      >
                        <option
                          v-for="device in workspaceStore.devices.filter(
                            (device) => device.status !== 'offline',
                          )"
                          :key="device.id"
                          :value="device.id"
                        >
                          {{ device.name }}
                        </option>
                      </select>
                    </label>

                    <div class="print-ticket-actions flex items-center justify-end gap-4">
                      <button
                        class="text-sm font-medium text-stone-600 transition-colors hover:text-stone-950"
                        @click="openPreview(item)"
                      >
                        {{ t("prints.actions.preview") }}
                      </button>
                      <button
                        v-if="!workspaceStore.isAuthenticated || item.status === 'pending'"
                        class="text-sm text-stone-500 transition-colors hover:text-rose-700"
                        @click="workspaceStore.cancelPrint(item.id)"
                      >
                        {{ t("prints.actions.cancelPrint") }}
                      </button>
                      <button
                        v-if="item.status === 'pending'"
                        class="ui-btn-primary px-4"
                        @click="workspaceStore.confirmPrint(item.id)"
                      >
                        {{ t("prints.actions.confirmPrint") }}
                      </button>
                    </div>
                  </div>
                  <p
                    v-if="workspaceStore.isAuthenticated && item.status === 'queued'"
                    class="mt-3 text-xs leading-5 text-stone-400"
                  >
                    {{ t("prints.pending.queuedHint") }}
                  </p>
                </div>
              </article>
            </div>
          </section>
        </div>
        <button
          v-if="workspaceStore.isAuthenticated && workspaceStore.activePrintJobsCursor"
          class="ui-btn-secondary mt-4 px-3 py-1.5 text-sm"
          :disabled="workspaceStore.activePrintJobsLoading || workspaceStore.printJobsLoading"
          @click="workspaceStore.loadMorePrintJobs('active')"
        >
          {{ t(workspaceStore.activePrintJobsLoading ? "prints.loading" : "prints.loadMore") }}
        </button>
      </section>

      <section>
        <div class="mb-4">
          <h2 class="text-xl font-semibold tracking-tight text-stone-900">
            {{ t("prints.recentPrints") }}
          </h2>
        </div>

        <div class="ui-list-card p-4">
          <div class="ui-timeline">
            <article v-for="item in historyPrintJobs" :key="item.id" class="ui-timeline-item">
              <div class="ui-timeline-row">
                <div class="ui-timeline-copy">
                  <p class="truncate text-sm font-medium text-stone-900">{{ item.title }}</p>
                  <p v-if="item.errorMessage" class="mt-1 text-sm text-red-700" role="alert">
                    {{ item.errorMessage }}
                  </p>
                  <p class="mt-0.5 text-sm text-stone-500">
                    {{ workspaceStore.getDeviceName(item.deviceId) }} ·
                    {{ workspaceStore.formatPrintTime(item.updatedAt) }}
                  </p>
                  <button
                    class="mt-2 text-sm text-stone-600 hover:text-stone-900"
                    @click="openPreview(item)"
                  >
                    {{ t("prints.actions.preview") }}
                  </button>
                </div>
                <span
                  class="ui-status-badge sm:self-center"
                  :class="getPrintStatusBadgeClass(item.status)"
                >
                  {{ workspaceStore.getPrintStatusLabel(item.status) }}
                </span>
              </div>
            </article>
          </div>
        </div>
        <button
          v-if="workspaceStore.isAuthenticated && workspaceStore.historyPrintJobsCursor"
          class="ui-btn-secondary mt-4 px-3 py-1.5 text-sm"
          :disabled="workspaceStore.historyPrintJobsLoading || workspaceStore.printJobsLoading"
          @click="workspaceStore.loadMorePrintJobs('history')"
        >
          {{ t(workspaceStore.historyPrintJobsLoading ? "prints.loading" : "prints.loadMore") }}
        </button>
      </section>
    </div>

    <AppDialog
      :open="printDialogOpen"
      :title="t('prints.printDialog.title')"
      @close="closePrintDialog"
    >
      <form class="space-y-4" @submit.prevent="submitPrintDialog">
        <label class="block">
          <span class="mb-2 block text-sm font-medium text-stone-900">
            {{ t("prints.printDialog.fields.title") }}
          </span>
          <input
            v-model="printTitle"
            type="text"
            :placeholder="t('prints.printDialog.placeholders.title')"
            class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 placeholder:text-stone-400 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
          />
        </label>

        <label class="block">
          <span class="mb-2 block text-sm font-medium text-stone-900">
            {{ t("prints.printDialog.fields.content") }}
          </span>
          <textarea
            v-model="printContent"
            rows="4"
            :placeholder="t('prints.printDialog.placeholders.content')"
            class="w-full rounded-xl border border-stone-200 bg-white px-4 py-3 text-sm text-stone-900 placeholder:text-stone-400 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
          />
        </label>

        <p v-if="printError" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700">
          {{ printError }}
        </p>

        <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <button
            type="button"
            class="ui-btn-secondary px-4 py-2 text-sm"
            @click="closePrintDialog"
          >
            {{ t("common.actions.cancel") }}
          </button>
          <button
            type="button"
            class="ui-btn-secondary px-4 py-2 text-sm"
            @click="previewManualPrint"
          >
            {{ t("prints.actions.preview") }}
          </button>
          <button type="submit" class="ui-btn-primary px-4 py-2 text-sm">
            {{ t("prints.printDialog.submit") }}
          </button>
        </div>
      </form>
    </AppDialog>

    <AppDialog
      v-if="previewJob"
      :open="true"
      :title="t('prints.preview.title')"
      size="wide"
      @close="closePreview"
    >
      <p class="px-6 pt-14 text-sm text-stone-500">{{ t("prints.preview.hint") }}</p>
      <p v-if="previewLoading" role="status" class="py-8 text-center text-sm text-stone-500">
        {{ t("prints.loading") }}
      </p>
      <p v-else-if="previewError" role="alert" class="text-sm text-rose-700">{{ previewError }}</p>
      <PrintPreview
        v-else
        :title="previewJob.title"
        :content="previewJob.content"
        :image="previewJob.image"
      />
    </AppDialog>
  </section>
</template>
