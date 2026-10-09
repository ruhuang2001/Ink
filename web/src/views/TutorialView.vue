<script setup lang="ts">
import { ref } from "vue";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { RouterLink } from "vue-router";

import AppDialog from "@/components/AppDialog.vue";
import { useWorkspaceStore } from "@/stores/workspace";

const workspaceStore = useWorkspaceStore();
const { t } = useI18n();

const steps = computed(
  () =>
    [
      {
        number: "01",
        title: t("tutorial.steps.chat.title"),
        body: t("tutorial.steps.chat.body"),
        note: t("tutorial.steps.chat.note"),
      },
      {
        number: "02",
        title: t("tutorial.steps.print.title"),
        body: t("tutorial.steps.print.body"),
        note: t("tutorial.steps.print.note"),
      },
      {
        number: "03",
        title: t("tutorial.steps.schedule.title"),
        body: t("tutorial.steps.schedule.body"),
        note: t("tutorial.steps.schedule.note"),
      },
    ] as const,
);

const faqs = computed(
  () =>
    [
      {
        question: t("tutorial.faqs.chatOrPrint.question"),
        answer: t("tutorial.faqs.chatOrPrint.answer"),
      },
      {
        question: t("tutorial.faqs.scheduleUse.question"),
        answer: t("tutorial.faqs.scheduleUse.answer"),
      },
      {
        question: t("tutorial.faqs.contentTooLong.question"),
        answer: t("tutorial.faqs.contentTooLong.answer"),
      },
      {
        question: t("tutorial.faqs.notPrinted.question"),
        answer: t("tutorial.faqs.notPrinted.answer"),
      },
    ] as const,
);

const feedbackOpen = ref(false);
const feedbackDraft = ref("");
const feedbackFormError = ref("");

function openFeedbackDialog() {
  feedbackOpen.value = true;
  feedbackDraft.value = "";
  feedbackFormError.value = "";
}

function closeFeedbackDialog() {
  feedbackOpen.value = false;
  feedbackFormError.value = "";
}

async function handleFeedbackSubmit() {
  feedbackFormError.value = "";

  if (!feedbackDraft.value.trim()) {
    feedbackFormError.value = t("feedback.errors.required");
    return;
  }

  const success = await workspaceStore.submitFeedback(feedbackDraft.value);
  if (!success) {
    feedbackFormError.value = workspaceStore.feedbackError;
    return;
  }

  feedbackDraft.value = "";
  closeFeedbackDialog();
}
</script>

<template>
  <section class="mx-auto max-w-6xl space-y-6 pt-8 sm:space-y-8 lg:space-y-10">
    <AppDialog
      :open="feedbackOpen"
      :title="t('feedback.dialog.title')"
      :description="t('feedback.dialog.description')"
      @close="closeFeedbackDialog"
    >
      <form class="space-y-4" @submit.prevent="handleFeedbackSubmit">
        <label class="block">
          <span class="mb-2 block text-sm font-medium text-stone-900">
            {{ t("feedback.dialog.contentLabel") }}
          </span>
          <textarea
            v-model="feedbackDraft"
            rows="6"
            :placeholder="t('feedback.dialog.placeholder')"
            class="w-full rounded-xl border border-stone-200 bg-white px-4 py-3 text-sm leading-7 text-stone-900 placeholder:text-stone-400 focus:border-stone-900 focus:ring-1 focus:ring-stone-900 focus:outline-none"
          />
        </label>

        <p v-if="feedbackFormError" class="rounded-lg bg-rose-50 px-3 py-2 text-sm text-rose-700">
          {{ feedbackFormError }}
        </p>

        <div class="flex flex-col gap-3 sm:flex-row sm:justify-end">
          <button
            type="button"
            class="ui-btn-secondary px-4 py-2.5 text-sm"
            @click="closeFeedbackDialog"
          >
            {{ t("common.actions.cancel") }}
          </button>
          <button
            class="ui-btn-primary px-4 py-2.5 text-sm"
            :disabled="workspaceStore.feedbackSubmitting"
          >
            {{
              workspaceStore.feedbackSubmitting
                ? t("feedback.dialog.submitting")
                : t("feedback.dialog.submit")
            }}
          </button>
        </div>
      </form>
    </AppDialog>

    <div class="border-b border-stone-200 pb-8">
      <div class="space-y-7">
        <div class="space-y-4">
          <p class="text-sm font-medium tracking-[0.2em] text-stone-500 uppercase">
            {{ t("tutorial.hero.eyebrow") }}
          </p>
          <div class="space-y-3">
            <h2
              class="max-w-4xl text-[clamp(2rem,4.8vw,3.5rem)] font-semibold tracking-tight text-stone-900"
            >
              {{ t("tutorial.hero.title") }}
              <span class="block text-stone-600">{{ t("tutorial.hero.subtitle") }}</span>
            </h2>
          </div>
        </div>

        <div class="grid border-y border-stone-200 sm:grid-cols-3 sm:divide-x sm:divide-stone-200">
          <RouterLink
            to="/conversations"
            class="border-b border-stone-200 px-4 py-5 transition-colors hover:bg-stone-50 sm:border-b-0"
          >
            <p class="text-sm font-semibold text-stone-900">
              {{ t("tutorial.features.chat.title") }}
            </p>
            <p class="mt-2 text-sm leading-6 text-stone-600">
              {{ t("tutorial.features.chat.body") }}
            </p>
          </RouterLink>
          <RouterLink
            to="/prints"
            class="border-b border-stone-200 px-4 py-5 transition-colors hover:bg-stone-50 sm:border-b-0"
          >
            <p class="text-sm font-semibold text-stone-900">
              {{ t("tutorial.features.print.title") }}
            </p>
            <p class="mt-2 text-sm leading-6 text-stone-600">
              {{ t("tutorial.features.print.body") }}
            </p>
          </RouterLink>
          <RouterLink to="/prints" class="px-4 py-5 transition-colors hover:bg-stone-50">
            <p class="text-sm font-semibold text-stone-900">
              {{ t("tutorial.features.schedule.title") }}
            </p>
            <p class="mt-2 text-sm leading-6 text-stone-600">
              {{ t("tutorial.features.schedule.body") }}
            </p>
          </RouterLink>
        </div>

        <div class="flex flex-col gap-3 sm:flex-row">
          <RouterLink to="/conversations" class="ui-btn-primary px-4 py-2.5 text-center text-sm">
            {{ t("tutorial.actions.goToConversations") }}
          </RouterLink>
          <RouterLink to="/prints" class="ui-btn-secondary px-4 py-2.5 text-center text-sm">
            {{ t("tutorial.actions.goToPrints") }}
          </RouterLink>
        </div>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-[minmax(0,1.15fr)_0.85fr]">
      <article class="border-t border-stone-200 py-6 sm:py-7">
        <div class="max-w-2xl">
          <p class="text-sm font-medium tracking-[0.18em] text-stone-500 uppercase">
            {{ t("tutorial.stepsSection.eyebrow") }}
          </p>
          <h3 class="mt-3 text-2xl font-semibold tracking-tight text-stone-900">
            {{ t("tutorial.stepsSection.title") }}
          </h3>
        </div>

        <div class="mt-6 border-t border-stone-200">
          <article
            v-for="(step, index) in steps"
            :key="step.number"
            class="border-b border-stone-200 px-1 py-6"
          >
            <div class="grid gap-4 md:grid-cols-[auto_minmax(0,1fr)] md:items-start">
              <div
                class="inline-flex h-10 w-10 items-center justify-center rounded-full border text-sm font-semibold"
                :class="
                  index === 0
                    ? 'border-amber-500 text-amber-800'
                    : 'border-stone-300 text-stone-700'
                "
              >
                {{ step.number }}
              </div>
              <div>
                <h4 class="text-lg font-semibold text-stone-900">{{ step.title }}</h4>
                <p class="mt-2 text-sm leading-7 text-stone-600">{{ step.body }}</p>
                <p
                  class="mt-4 border-l-2 px-4 py-1 text-sm leading-6"
                  :class="
                    index === 0
                      ? 'border-amber-500 text-amber-900'
                      : 'border-stone-300 text-stone-600'
                  "
                >
                  {{ step.note }}
                </p>
              </div>
            </div>
          </article>
        </div>
      </article>

      <div class="space-y-6">
        <article class="border-y border-stone-200 py-6 sm:py-7">
          <p class="text-sm font-medium tracking-[0.18em] text-stone-500 uppercase">
            {{ t("tutorial.mobile.eyebrow") }}
          </p>
          <h3 class="mt-3 text-2xl font-semibold tracking-tight text-stone-900">
            {{ t("tutorial.mobile.title") }}
          </h3>
          <p class="mt-3 text-sm leading-7 text-stone-600">
            {{ t("tutorial.mobile.body") }}
          </p>
        </article>

        <article class="border-y border-stone-200 py-6 sm:py-7">
          <h3 class="text-2xl font-semibold tracking-tight text-stone-900">
            {{ t("tutorial.faqSection.title") }}
          </h3>
          <div class="mt-5 border-t border-stone-200">
            <article
              v-for="item in faqs"
              :key="item.question"
              class="border-b border-stone-200 px-1 py-4"
            >
              <p class="text-sm font-semibold text-stone-900">{{ item.question }}</p>
              <p class="mt-2 text-sm leading-7 text-stone-600">{{ item.answer }}</p>
            </article>
          </div>

          <div class="mt-6 border-l-2 border-amber-600 py-1 pl-4">
            <p class="text-sm font-semibold text-stone-900">
              {{ t("tutorial.faqSection.missingQuestionTitle") }}
            </p>
            <p class="mt-2 text-sm leading-7 text-stone-600">
              {{ t("tutorial.faqSection.missingQuestionBody") }}
            </p>
            <button
              type="button"
              class="ui-btn-secondary mt-4 w-full px-4 py-2.5 text-sm sm:w-auto"
              @click="openFeedbackDialog"
            >
              {{ t("feedback.card.action") }}
            </button>
          </div>
        </article>

        <aside class="border-y border-stone-200 py-6 sm:py-7">
          <h3 class="text-xl font-semibold text-stone-900">
            {{ t("tutorial.start.title") }}
          </h3>
          <div class="mt-5 space-y-3">
            <RouterLink
              to="/conversations"
              class="ui-btn-primary block w-full px-4 py-2.5 text-center text-sm"
            >
              {{ t("tutorial.actions.goToConversations") }}
            </RouterLink>
            <RouterLink
              to="/prints"
              class="ui-btn-secondary block w-full px-4 py-2.5 text-center text-sm"
            >
              {{ t("tutorial.actions.goToPrints") }}
            </RouterLink>
          </div>
        </aside>
      </div>
    </div>
  </section>
</template>
