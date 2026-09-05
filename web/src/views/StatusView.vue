<script setup lang="ts">
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import { RouterLink } from "vue-router";

import AppDialog from "@/components/AppDialog.vue";
import { useWorkspaceStore } from "@/stores/workspace";
import { getDeviceStatusBadgeClass, getPrintStatusBadgeClass } from "@/utils/workspace";

const workspaceStore = useWorkspaceStore();
const { t } = useI18n();
const addDeviceOpen = ref(false);
const deviceName = ref("");
const deviceNote = ref("");
const deviceIdentifier = ref("");
const setAsDefault = ref(false);
const addDeviceError = ref("");

function openAddDeviceDialog() {
  addDeviceOpen.value = true;
  addDeviceError.value = "";
  deviceName.value = "";
  deviceNote.value = "";
  deviceIdentifier.value = "";
  setAsDefault.value = false;
}

function closeAddDeviceDialog() {
  addDeviceOpen.value = false;
}

async function submitAddDevice() {
  addDeviceError.value = "";

  if (!deviceName.value.trim()) {
    addDeviceError.value = t("status.dialog.errors.nameRequired");
    return;
  }

  if (workspaceStore.isAuthenticated && !deviceIdentifier.value.trim()) {
    addDeviceError.value = t("status.dialog.errors.identifierRequired");
    return;
  }

  const created = await workspaceStore.addDevice({
    name: deviceName.value,
    note: deviceNote.value,
    deviceId: deviceIdentifier.value,
    setAsDefault: setAsDefault.value,
  });
  if (!created) {
    addDeviceError.value =
      workspaceStore.flashTone === "error"
        ? workspaceStore.flashMessage
        : t("status.dialog.errors.createFailed");
    return;
  }
  closeAddDeviceDialog();
}
</script>

<template>
  <section class="mx-auto max-w-5xl space-y-6 pt-4 sm:space-y-8">
    <div>
      <h2 class="text-2xl font-semibold tracking-tight text-stone-900">
        {{ t("navigation.status.label") }}
      </h2>
    </div>

    <div class="ink-summary">
      <article v-for="item in workspaceStore.summaryCards" :key="item.label" class="ink-stat">
        <p class="text-sm text-stone-500">{{ item.label }}</p>
        <p class="text-sm font-medium text-stone-900">{{ item.value }}</p>
      </article>
    </div>

    <div class="grid gap-8 lg:grid-cols-[minmax(0,1fr)_320px]">
      <div class="space-y-8">
        <section>
          <div
            v-if="workspaceStore.printerSyncError"
            class="mb-4 rounded-2xl border border-amber-200 bg-amber-50 px-5 py-4"
          >
            <p class="text-sm font-medium text-amber-900">{{ t("status.syncErrorTitle") }}</p>
            <p class="mt-1 text-sm text-amber-700">{{ workspaceStore.printerSyncError }}</p>
          </div>

          <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 class="text-base leading-6 font-semibold text-stone-900">
                {{ t("status.boundDevices") }}
              </h3>
            </div>
            <div class="flex items-center gap-2">
              <button
                type="button"
                class="ui-btn-secondary px-3 py-1.5 text-sm"
                @click="openAddDeviceDialog"
              >
                {{ t("status.actions.addDevice") }}
              </button>
            </div>
          </div>

          <div
            v-if="workspaceStore.devices.length === 0"
            class="rounded-2xl border border-dashed border-stone-200 bg-stone-50 px-6 py-10 text-center"
          >
            <h4 class="text-base font-semibold text-stone-900">
              {{ t("status.emptyDevices.title") }}
            </h4>
            <p class="mt-2 text-sm text-stone-500">
              {{
                workspaceStore.isAuthenticated
                  ? t("status.emptyDevices.authenticated")
                  : t("status.emptyDevices.anonymous")
              }}
            </p>
          </div>

          <div v-else class="ui-list-card">
            <article
              v-for="device in workspaceStore.devices"
              :key="device.id"
              class="ui-list-row flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between"
            >
              <div class="flex min-w-0 items-center gap-3">
                <div
                  class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-stone-100 text-sm font-semibold text-stone-700"
                >
                  {{ device.name.slice(0, 1) }}
                </div>
                <div class="min-w-0">
                  <p class="truncate text-sm font-medium text-stone-900">{{ device.name }}</p>
                  <p class="mt-0.5 text-sm text-stone-500">
                    {{
                      device.id === workspaceStore.defaultDeviceId
                        ? t("status.defaultDevicePrefix")
                        : ""
                    }}
                    {{ device.note }}
                  </p>
                </div>
              </div>

              <div class="flex w-full flex-wrap items-center gap-2 sm:w-auto sm:justify-end">
                <span class="ui-status-badge" :class="getDeviceStatusBadgeClass(device.status)">
                  {{ workspaceStore.getDeviceStatusLabel(device.status) }}
                </span>
                <button
                  v-if="device.id !== workspaceStore.defaultDeviceId && device.status !== 'offline'"
                  type="button"
                  class="ui-btn-secondary px-3 py-1.5 text-sm"
                  @click="workspaceStore.setDefaultDevice(device.id)"
                >
                  {{ t("status.actions.setDefault") }}
                </button>
                <button
                  v-if="device.status !== 'offline'"
                  type="button"
                  class="ui-btn-secondary px-3 py-1.5 text-sm"
                  @click="workspaceStore.removeDevice(device.id)"
                >
                  {{
                    device.status === "pending"
                      ? t("status.actions.remove")
                      : workspaceStore.isAuthenticated
                        ? t("common.actions.delete")
                        : t("status.actions.unbind")
                  }}
                </button>
              </div>
            </article>
          </div>
        </section>

        <section>
          <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 class="text-base leading-6 font-semibold text-stone-900">
                {{ t("status.autoPrint") }}
              </h3>
            </div>
            <RouterLink to="/prints" class="ui-btn-secondary px-3 py-1.5 text-sm">
              {{ t("status.actions.goToPrints") }}
            </RouterLink>
          </div>

          <div
            v-if="workspaceStore.activeSchedules.length === 0"
            class="rounded-2xl border border-dashed border-stone-200 bg-stone-50 px-6 py-10 text-center"
          >
            <h4 class="text-base font-semibold text-stone-900">
              {{ t("status.emptySchedules") }}
            </h4>
          </div>

          <div v-else class="ui-list-card">
            <article
              v-for="task in workspaceStore.activeSchedules"
              :key="task.id"
              class="ui-list-row flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between"
            >
              <div class="min-w-0">
                <p class="text-sm font-medium text-stone-900">{{ task.title }}</p>
                <p class="mt-0.5 text-sm text-stone-500">
                  {{ task.source }} · {{ task.timeLabel }} ·
                  {{ workspaceStore.getDeviceName(task.deviceId) }}
                </p>
              </div>

              <button
                type="button"
                class="ui-toggle"
                :class="{ 'is-on': task.enabled }"
                :aria-label="
                  t(task.enabled ? 'status.actions.disableTask' : 'status.actions.enableTask', {
                    title: task.title,
                  })
                "
                :aria-pressed="task.enabled"
                @click="workspaceStore.toggleSchedule(task.id)"
              >
                <span class="ui-toggle-thumb" />
              </button>
            </article>
          </div>
        </section>
      </div>

      <aside>
        <div class="mb-4">
          <div>
            <h3 class="text-base leading-6 font-semibold text-stone-900">
              {{ t("status.recentPrints") }}
            </h3>
          </div>
        </div>

        <div class="ui-list-card p-4">
          <div class="ui-timeline">
            <article
              v-for="item in workspaceStore.recentPrintJobs"
              :key="item.id"
              class="ui-timeline-item"
            >
              <div class="ui-timeline-row">
                <div class="ui-timeline-copy">
                  <p class="truncate text-sm font-medium text-stone-900">{{ item.title }}</p>
                  <p class="mt-0.5 text-sm text-stone-500">
                    {{ workspaceStore.getDeviceName(item.deviceId) }} ·
                    {{ workspaceStore.formatPrintTime(item.updatedAt) }}
                  </p>
                </div>
                <span
                  class="ui-status-badge self-center"
                  :class="getPrintStatusBadgeClass(item.status)"
                >
                  {{ workspaceStore.getPrintStatusLabel(item.status) }}
                </span>
              </div>
            </article>
          </div>
        </div>
      </aside>
    </div>

    <AppDialog
      :open="addDeviceOpen"
      :title="t('status.dialog.title')"
      :description="
        workspaceStore.isAuthenticated
          ? t('status.dialog.description.authenticated')
          : t('status.dialog.description.anonymous')
      "
      @close="closeAddDeviceDialog"
    >
      <form class="space-y-4" @submit.prevent="submitAddDevice">
        <label class="block">
          <span class="mb-2 block text-sm font-medium text-stone-900">
            {{ t("status.dialog.fields.name") }}
          </span>
          <input
            v-model="deviceName"
            type="text"
            :placeholder="t('status.dialog.placeholders.name')"
            class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 placeholder:text-stone-400 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
          />
        </label>

        <label class="block">
          <span class="mb-2 block text-sm font-medium text-stone-900">
            {{ t("status.dialog.fields.note") }}
          </span>
          <input
            v-model="deviceNote"
            type="text"
            :placeholder="t('status.dialog.placeholders.note')"
            class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 placeholder:text-stone-400 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
          />
        </label>

        <label v-if="workspaceStore.isAuthenticated" class="block">
          <span class="mb-2 block text-sm font-medium text-stone-900">
            {{ t("status.dialog.fields.identifier") }}
          </span>
          <input
            v-model="deviceIdentifier"
            type="text"
            :placeholder="t('status.dialog.placeholders.identifier')"
            class="w-full rounded-xl border border-stone-200 bg-white px-4 py-2.5 text-sm text-stone-900 placeholder:text-stone-400 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
          />
        </label>

        <label
          class="flex items-center gap-3 rounded-2xl border border-stone-200 bg-stone-50 px-4 py-3"
        >
          <input
            v-model="setAsDefault"
            type="checkbox"
            class="h-4 w-4 rounded border-stone-300 text-stone-900 focus:ring-stone-900"
          />
          <span class="text-sm text-stone-900">{{ t("status.dialog.fields.setDefault") }}</span>
        </label>

        <p v-if="addDeviceError" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700">
          {{ addDeviceError }}
        </p>

        <div class="flex justify-end gap-3">
          <button
            type="button"
            class="ui-btn-secondary px-4 py-2 text-sm"
            @click="closeAddDeviceDialog"
          >
            {{ t("common.actions.cancel") }}
          </button>
          <button type="submit" class="ui-btn-primary px-4 py-2 text-sm">
            {{ t("status.actions.addDevice") }}
          </button>
        </div>
      </form>
    </AppDialog>
  </section>
</template>
