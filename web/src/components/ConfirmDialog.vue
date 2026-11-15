<script setup lang="ts">
import { useI18n } from "vue-i18n";

import AppDialog from "@/components/AppDialog.vue";
import { activeConfirm, settleConfirm } from "@/services/confirm";

const { t } = useI18n();
</script>

<template>
  <AppDialog
    :open="!!activeConfirm"
    :title="t('common.labels.confirmTitle')"
    @close="settleConfirm(false)"
  >
    <p class="text-sm leading-6 text-stone-700">{{ activeConfirm?.message }}</p>
    <div class="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
      <button
        type="button"
        class="ui-btn-secondary px-4 py-2 text-sm"
        @click="settleConfirm(false)"
      >
        {{ t("common.actions.cancel") }}
      </button>
      <button
        type="button"
        class="px-4 py-2 text-sm"
        :class="activeConfirm?.tone === 'danger' ? 'ui-btn-danger' : 'ui-btn-primary'"
        @click="settleConfirm(true)"
      >
        {{ activeConfirm?.confirmLabel ?? t("common.actions.confirm") }}
      </button>
    </div>
  </AppDialog>
</template>
