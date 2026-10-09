<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { RouterLink } from "vue-router";

import AppDialog from "@/components/AppDialog.vue";
import PrintPreview from "@/components/PrintPreview.vue";
import { renderPrintPreview } from "@/services/printers";
import { useWorkspaceStore } from "@/stores/workspace";
import { getPrintStatusBadgeClass } from "@/utils/workspace";

const workspaceStore = useWorkspaceStore();
const { t } = useI18n();

const weekdayOptions = computed(
  () =>
    [
      { label: t("weekdays.short.0"), value: 0 },
      { label: t("weekdays.short.1"), value: 1 },
      { label: t("weekdays.short.2"), value: 2 },
      { label: t("weekdays.short.3"), value: 3 },
      { label: t("weekdays.short.4"), value: 4 },
      { label: t("weekdays.short.5"), value: 5 },
      { label: t("weekdays.short.6"), value: 6 },
    ] as const,
);

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
const scheduleDialogOpen = ref(false);
const printTitle = ref("");
const printContent = ref("");
const printError = ref("");
const scheduleTitle = ref("");
const scheduleSource = ref("");
const scheduleTime = ref(t("prints.scheduleDialog.placeholders.time"));
const schedulePluginInstallationId = ref("");
const scheduleFrequencyType = ref<"daily" | "weekly">("daily");
const scheduleTimezone = ref(Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Shanghai");
const scheduleHour = ref(19);
const scheduleMinute = ref(30);
const scheduleWeekdays = ref<number[]>([]);
const scheduleBatchSize = ref(1);
const scheduleDeviceId = ref("");
const scheduleError = ref("");
const previewJob = ref<{ title: string; content: string; image?: string } | null>(null);
const previewLoading = ref(false);
const previewError = ref("");
let previewRequestVersion = 0;

watch(() => workspaceStore.authUser?.id, closePreview);
onBeforeUnmount(closePreview);

function getInvalidBatchSizeMessage() {
  return t("prints.errors.invalidBatchSize");
}

const connectedPlugins = computed(() =>
  workspaceStore.availablePlugins.filter(
    (plugin) =>
      plugin.installation.status === "ready" &&
      plugin.binding?.enabled &&
      plugin.binding.status === "connected",
  ),
);

const selectedSchedulePlugin = computed(
  () =>
    connectedPlugins.value.find(
      (plugin) => plugin.installation.id === schedulePluginInstallationId.value,
    ) ?? null,
);

function handlePrintDeviceChange(jobId: string, event: Event) {
  const target = event.target as HTMLSelectElement | null;
  workspaceStore.updatePrintDevice(jobId, target?.value ?? workspaceStore.defaultDeviceId);
}

function handleScheduleDeviceChange(scheduleId: string, event: Event) {
  const target = event.target as HTMLSelectElement | null;
  workspaceStore.updateScheduleDevice(scheduleId, target?.value ?? workspaceStore.defaultDeviceId);
}

async function handleScheduleDelete(scheduleId: string) {
  if (typeof window !== "undefined" && !window.confirm(t("prints.confirmDeleteSchedule"))) {
    return;
  }

  await workspaceStore.deleteSchedule(scheduleId);
}

function toggleWeekday(weekday: number) {
  scheduleWeekdays.value = scheduleWeekdays.value.includes(weekday)
    ? scheduleWeekdays.value.filter((value) => value !== weekday)
    : weekdayOptions.value
        .map((option) => option.value)
        .filter((value) => [...scheduleWeekdays.value, weekday].includes(value));
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
  previewLoading.value = !!item.id && workspaceStore.isAuthenticated;
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
    printError.value =
      workspaceStore.flashTone === "error"
        ? workspaceStore.flashMessage
        : t("prints.printDialog.errors.createFailed");
    return;
  }

  closePrintDialog();
}

function openScheduleDialog() {
  scheduleDialogOpen.value = true;
  scheduleTitle.value = "";
  scheduleSource.value = "";
  scheduleTime.value = t("prints.scheduleDialog.placeholders.time");
  schedulePluginInstallationId.value = connectedPlugins.value[0]?.installation.id ?? "";
  scheduleFrequencyType.value = "daily";
  scheduleTimezone.value = Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Shanghai";
  scheduleHour.value = 19;
  scheduleMinute.value = 30;
  scheduleWeekdays.value = [];
  scheduleBatchSize.value = 1;
  scheduleDeviceId.value = workspaceStore.defaultDeviceId;
  scheduleError.value = "";
}

function closeScheduleDialog() {
  scheduleDialogOpen.value = false;
}

async function submitScheduleDialog() {
  scheduleError.value = "";

  if (!scheduleTitle.value.trim()) {
    scheduleError.value = t("prints.scheduleDialog.errors.titleRequired");
    return;
  }

  if (!(scheduleDeviceId.value || workspaceStore.defaultDeviceId)) {
    scheduleError.value = t("prints.scheduleDialog.errors.deviceRequired");
    return;
  }

  if (workspaceStore.isAuthenticated) {
    if (!schedulePluginInstallationId.value) {
      scheduleError.value = t("prints.scheduleDialog.errors.pluginRequired");
      return;
    }

    if (scheduleFrequencyType.value === "weekly" && scheduleWeekdays.value.length === 0) {
      scheduleError.value = t("prints.scheduleDialog.errors.weekdayRequired");
      return;
    }

    if (!Number.isInteger(scheduleBatchSize.value) || scheduleBatchSize.value <= 0) {
      scheduleError.value = getInvalidBatchSizeMessage();
      return;
    }

    const created = await workspaceStore.createSchedule({
      title: scheduleTitle.value,
      deviceId: scheduleDeviceId.value || workspaceStore.defaultDeviceId,
      pluginInstallationId: schedulePluginInstallationId.value,
      frequencyType: scheduleFrequencyType.value,
      timezone: scheduleTimezone.value,
      hour: scheduleHour.value,
      minute: scheduleMinute.value,
      weekdays: scheduleWeekdays.value,
      batchSize: scheduleBatchSize.value,
    });

    if (!created) {
      scheduleError.value =
        workspaceStore.flashTone === "error"
          ? workspaceStore.flashMessage
          : t("prints.scheduleDialog.errors.createFailed");
      return;
    }

    closeScheduleDialog();
    return;
  }

  if (!scheduleTime.value.trim()) {
    scheduleError.value = t("prints.scheduleDialog.errors.timeRequired");
    return;
  }

  await workspaceStore.createSchedule({
    title: scheduleTitle.value,
    source: scheduleSource.value || t("prints.scheduleDialog.manualSourceFallback"),
    timeLabel: scheduleTime.value,
    deviceId: scheduleDeviceId.value || workspaceStore.defaultDeviceId,
  });
  closeScheduleDialog();
}
</script>

<template>
  <section class="print-page mx-auto max-w-6xl space-y-8 pt-2 sm:space-y-10">
    <header
      class="ink-rule flex flex-col gap-5 border-b pb-6 sm:flex-row sm:items-end sm:justify-between sm:pb-7"
    >
      <div>
        <h1 class="text-3xl font-semibold tracking-tight text-stone-950 sm:text-[2.7rem]">
          {{ t("navigation.prints.label") }}
        </h1>
        <p class="mt-2 text-sm text-stone-500">
          {{ t("prints.defaultSettings.defaultDevice") }} ·
          {{ workspaceStore.activeDeviceLabel || t("prints.defaultSettings.notSet") }}
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2 sm:justify-end">
        <button class="ui-btn-primary flex-1 px-4 sm:flex-none" @click="openPrintDialog">
          {{ t("prints.actions.newPrint") }}
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
          class="border-y border-stone-200 px-6 py-10 text-center"
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
        <div class="mb-4 flex items-center justify-between gap-4">
          <h2 class="text-xl font-semibold tracking-tight text-stone-900">
            {{ t("prints.schedules.title") }}
          </h2>
          <button
            class="text-sm font-medium text-stone-600 hover:text-stone-950"
            @click="openScheduleDialog"
          >
            {{ t("prints.actions.newSchedule") }}
          </button>
        </div>

        <div
          v-if="workspaceStore.activeSchedules.length === 0"
          class="border-y border-stone-200 px-6 py-10 text-center"
        >
          <h4 class="text-base font-semibold text-stone-900">
            {{ t("prints.schedules.emptyTitle") }}
          </h4>
        </div>

        <div v-else class="ui-list-card">
          <article
            v-for="task in workspaceStore.activeSchedules"
            :key="task.id"
            class="ui-list-row flex flex-col gap-4"
          >
            <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div class="min-w-0">
                <p class="text-sm font-medium text-stone-900">{{ task.title }}</p>
                <p class="mt-1 text-sm text-stone-500">
                  {{ task.source }} · {{ task.timeLabel }} ·
                  {{
                    t("prints.schedules.sendToDevice", {
                      device: workspaceStore.getDeviceName(task.deviceId),
                    })
                  }}
                </p>
                <p v-if="task.nextRunAt" class="mt-1 text-xs text-stone-500">
                  {{
                    t("prints.schedules.nextRunAt", {
                      time: workspaceStore.formatPrintTime(task.nextRunAt),
                    })
                  }}
                </p>
                <p
                  v-if="workspaceStore.isAuthenticated && task.printPolicy?.batchSize"
                  class="mt-1 text-xs text-stone-500"
                >
                  {{
                    t("prints.schedules.batchSizeHint", {
                      count: task.printPolicy.batchSize,
                    })
                  }}
                </p>
                <p
                  v-if="task.lastError"
                  class="mt-3 rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700"
                >
                  {{ task.lastError }}
                </p>
              </div>

              <div class="flex items-center gap-2">
                <button
                  type="button"
                  class="px-2 py-2 text-sm text-stone-400 transition-colors hover:text-rose-700"
                  @click="handleScheduleDelete(task.id)"
                >
                  {{ t("common.actions.delete") }}
                </button>
                <button
                  type="button"
                  class="ui-toggle"
                  :class="{ 'is-on': task.enabled }"
                  :aria-label="
                    t(
                      task.enabled ? 'prints.schedules.disableTask' : 'prints.schedules.enableTask',
                      { title: task.title },
                    )
                  "
                  :aria-pressed="task.enabled"
                  @click="workspaceStore.toggleSchedule(task.id)"
                >
                  <span class="ui-toggle-thumb" />
                </button>
              </div>
            </div>

            <div class="flex flex-col gap-2 md:flex-row md:items-center">
              <label class="text-sm font-medium text-stone-700">
                {{ t("prints.schedules.deviceLabel") }}
              </label>
              <select
                :value="task.deviceId"
                class="w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-900 md:w-auto"
                @change="handleScheduleDeviceChange(task.id, $event)"
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
            </div>
          </article>
        </div>
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
      :open="scheduleDialogOpen"
      :title="t('prints.scheduleDialog.title')"
      @close="closeScheduleDialog"
    >
      <form class="space-y-4" @submit.prevent="submitScheduleDialog">
        <label class="block">
          <span class="mb-2 block text-sm font-medium text-stone-900">
            {{ t("prints.scheduleDialog.fields.title") }}
          </span>
          <input
            v-model="scheduleTitle"
            type="text"
            :placeholder="t('prints.scheduleDialog.placeholders.title')"
            class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 placeholder:text-stone-400 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
          />
        </label>

        <template v-if="workspaceStore.isAuthenticated">
          <label class="block">
            <span class="mb-2 block text-sm font-medium text-stone-900">
              {{ t("prints.scheduleDialog.fields.plugin") }}
            </span>
            <select
              v-model="schedulePluginInstallationId"
              class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
            >
              <option value="">{{ t("prints.scheduleDialog.placeholders.plugin") }}</option>
              <option
                v-for="plugin in connectedPlugins"
                :key="plugin.installation.id"
                :value="plugin.installation.id"
              >
                {{ plugin.installation.displayName }}
              </option>
            </select>
          </label>

          <div
            v-if="connectedPlugins.length === 0"
            class="rounded-lg bg-stone-50 px-4 py-3 text-sm text-stone-500"
          >
            {{ t("prints.scheduleDialog.emptyPlugins") }}
          </div>

          <div class="grid gap-4 sm:grid-cols-2">
            <label class="block">
              <span class="mb-2 block text-sm font-medium text-stone-900">
                {{ t("prints.scheduleDialog.fields.frequency") }}
              </span>
              <select
                v-model="scheduleFrequencyType"
                class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
              >
                <option value="daily">{{ t("prints.scheduleDialog.frequency.daily") }}</option>
                <option value="weekly">{{ t("prints.scheduleDialog.frequency.weekly") }}</option>
              </select>
            </label>

            <label class="block">
              <span class="mb-2 block text-sm font-medium text-stone-900">
                {{ t("prints.scheduleDialog.fields.timezone") }}
              </span>
              <input
                v-model="scheduleTimezone"
                type="text"
                :placeholder="t('prints.scheduleDialog.placeholders.timezone')"
                class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 placeholder:text-stone-400 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
              />
            </label>
          </div>

          <div class="grid gap-4 sm:grid-cols-2">
            <label class="block">
              <span class="mb-2 block text-sm font-medium text-stone-900">
                {{ t("prints.scheduleDialog.fields.hour") }}
              </span>
              <input
                v-model.number="scheduleHour"
                type="number"
                min="0"
                max="23"
                class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
              />
            </label>

            <label class="block">
              <span class="mb-2 block text-sm font-medium text-stone-900">
                {{ t("prints.scheduleDialog.fields.minute") }}
              </span>
              <input
                v-model.number="scheduleMinute"
                type="number"
                min="0"
                max="59"
                class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
              />
            </label>
          </div>

          <div v-if="scheduleFrequencyType === 'weekly'" class="block">
            <span class="mb-2 block text-sm font-medium text-stone-900">
              {{ t("prints.scheduleDialog.fields.weekdays") }}
            </span>
            <div class="flex flex-wrap gap-2">
              <button
                v-for="weekday in weekdayOptions"
                :key="weekday.value"
                type="button"
                class="rounded-full px-3 py-1.5 text-sm ring-1 transition ring-inset"
                :class="
                  scheduleWeekdays.includes(weekday.value)
                    ? 'bg-stone-900 text-white ring-stone-900'
                    : 'bg-white text-stone-700 ring-stone-200'
                "
                @click="toggleWeekday(weekday.value)"
              >
                {{ weekday.label }}
              </button>
            </div>
          </div>

          <div
            v-if="selectedSchedulePlugin"
            class="rounded-xl border border-stone-200 bg-stone-50 px-4 py-4"
          >
            <div class="flex items-start justify-between gap-3">
              <div>
                <p class="text-sm font-medium text-stone-900">
                  {{ selectedSchedulePlugin.installation.displayName }}
                </p>
                <p class="mt-1 text-sm text-stone-500">
                  {{ selectedSchedulePlugin.manifest.description }}
                </p>
              </div>
              <span class="text-xs text-stone-500">
                {{
                  selectedSchedulePlugin.installation.runtimeType === "node"
                    ? t("store.labels.pluginRuntimeNode")
                    : t("store.labels.pluginRuntimePython")
                }}
              </span>
            </div>

            <div class="mt-4 grid gap-4 md:grid-cols-2">
              <div class="rounded-lg border border-dashed border-stone-200 bg-white px-4 py-3">
                <p class="text-xs font-medium tracking-[0.12em] text-stone-500 uppercase">
                  {{ t("prints.scheduleDialog.pluginDetails.fetchFrequency") }}
                </p>
                <p class="mt-1 text-sm text-stone-900">
                  {{
                    t("prints.scheduleDialog.pluginDetails.fetchEveryMinutes", {
                      minutes: selectedSchedulePlugin.manifest.fetchPolicy.minutes,
                    })
                  }}
                </p>
                <p class="mt-1 text-xs text-stone-500">
                  {{ t("prints.scheduleDialog.pluginDetails.fetchHint") }}
                </p>
              </div>

              <div class="rounded-lg border border-dashed border-stone-200 bg-white px-4 py-3">
                <p class="text-xs font-medium tracking-[0.12em] text-stone-500 uppercase">
                  {{ t("prints.scheduleDialog.pluginDetails.fetchStatus") }}
                </p>
                <p class="mt-1 text-sm text-stone-900">
                  {{
                    selectedSchedulePlugin.binding?.lastFetchAt
                      ? t("prints.scheduleDialog.pluginDetails.lastFetchedAt", {
                          time: workspaceStore.formatPrintTime(
                            selectedSchedulePlugin.binding.lastFetchAt,
                          ),
                        })
                      : t("prints.scheduleDialog.pluginDetails.neverFetched")
                  }}
                </p>
                <p class="mt-1 text-xs text-stone-500">
                  {{
                    selectedSchedulePlugin.binding?.nextFetchAt
                      ? t("prints.scheduleDialog.pluginDetails.nextFetchAt", {
                          time: workspaceStore.formatPrintTime(
                            selectedSchedulePlugin.binding.nextFetchAt,
                          ),
                        })
                      : t("prints.scheduleDialog.pluginDetails.noNextFetch")
                  }}
                </p>
                <p
                  v-if="selectedSchedulePlugin.binding?.lastFetchError"
                  class="mt-2 text-xs text-rose-700"
                >
                  {{ selectedSchedulePlugin.binding.lastFetchError }}
                </p>
              </div>
            </div>

            <label class="mt-4 block">
              <span class="mb-2 block text-sm font-medium text-stone-900">
                {{ t("prints.scheduleDialog.fields.batchSize") }}
              </span>
              <input
                v-model.number="scheduleBatchSize"
                type="number"
                min="1"
                class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
              />
              <span class="mt-2 block text-xs text-stone-500">
                {{ t("prints.scheduleDialog.pluginDetails.batchSizeHint") }}
              </span>
            </label>
          </div>

          <div
            v-else
            class="rounded-xl border border-dashed border-stone-200 bg-stone-50 px-4 py-3 text-sm text-stone-500"
          >
            {{ t("prints.scheduleDialog.emptyPluginSelection") }}
          </div>
        </template>

        <template v-else>
          <label class="block">
            <span class="mb-2 block text-sm font-medium text-stone-900">
              {{ t("prints.scheduleDialog.fields.source") }}
            </span>
            <input
              v-model="scheduleSource"
              type="text"
              :placeholder="t('prints.scheduleDialog.placeholders.source')"
              class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 placeholder:text-stone-400 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
            />
          </label>

          <label class="block">
            <span class="mb-2 block text-sm font-medium text-stone-900">
              {{ t("prints.scheduleDialog.fields.time") }}
            </span>
            <input
              v-model="scheduleTime"
              type="text"
              :placeholder="t('prints.scheduleDialog.placeholders.time')"
              class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 placeholder:text-stone-400 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
            />
          </label>
        </template>

        <label class="block">
          <span class="mb-2 block text-sm font-medium text-stone-900">
            {{ t("prints.scheduleDialog.fields.device") }}
          </span>
          <select
            v-model="scheduleDeviceId"
            class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
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

        <p v-if="scheduleError" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700">
          {{ scheduleError }}
        </p>

        <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <button
            type="button"
            class="ui-btn-secondary px-4 py-2 text-sm"
            @click="closeScheduleDialog"
          >
            {{ t("common.actions.cancel") }}
          </button>
          <button
            type="submit"
            class="ui-btn-primary px-4 py-2 text-sm"
            :disabled="workspaceStore.isAuthenticated && connectedPlugins.length === 0"
          >
            {{ t("prints.scheduleDialog.submit") }}
          </button>
        </div>
      </form>
    </AppDialog>

    <AppDialog
      v-if="previewJob"
      :open="true"
      :title="t('prints.preview.title')"
      @close="closePreview"
    >
      <p class="mb-4 text-sm text-stone-500">{{ t("prints.preview.hint") }}</p>
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
