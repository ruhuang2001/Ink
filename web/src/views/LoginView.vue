<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";

import { DEFAULT_LOGIN_REDIRECT, resolveLoginRedirect } from "@/router/authRedirect";
import { useWorkspaceStore } from "@/stores/workspace";

const router = useRouter();
const route = useRoute();
const workspaceStore = useWorkspaceStore();
const { t } = useI18n();

const email = ref("admin");
const password = ref("");
const formError = ref("");
const passwordVisible = ref(false);

const isFormValid = computed(
  () => email.value.trim().length > 0 && password.value.trim().length > 0,
);
const noticeMessage = computed(() =>
  route.query.notice === "password-updated" ? t("login.notice.passwordUpdated") : "",
);

function handleBack() {
  if (typeof window !== "undefined" && window.history.length > 1) {
    router.back();
    return;
  }

  void router.replace(DEFAULT_LOGIN_REDIRECT);
}

async function handleSubmit() {
  formError.value = "";

  if (!isFormValid.value) {
    formError.value = t("login.errors.missingCredentials");
    return;
  }

  const success = await workspaceStore.login(email.value.trim(), password.value.trim());

  if (!success) {
    formError.value = workspaceStore.authError;
    return;
  }

  await router.replace(resolveLoginRedirect(router, route.query.redirect));
}
</script>

<template>
  <div
    class="relative flex min-h-[100dvh] bg-stone-50 px-5 pt-[calc(env(safe-area-inset-top)+1rem)] pb-[calc(env(safe-area-inset-bottom)+1.5rem)] text-stone-900 sm:px-8 sm:py-8"
  >
    <button
      type="button"
      class="absolute top-[calc(env(safe-area-inset-top)+1rem)] left-5 inline-flex min-h-10 items-center gap-2 text-sm font-medium text-stone-500 transition-colors hover:text-stone-950 focus-visible:ring-2 focus-visible:ring-amber-700 focus-visible:ring-offset-4 focus-visible:outline-none sm:top-8 sm:left-8"
      @click="handleBack"
    >
      <span aria-hidden="true">←</span>
      <span>{{ t("common.actions.back") }}</span>
    </button>

    <main class="mx-auto flex w-full max-w-sm flex-1 items-center py-20 sm:py-16">
      <section class="w-full">
        <header class="text-center">
          <img
            src="/icon.jpg"
            alt=""
            class="mx-auto h-12 w-12 rounded-lg border border-stone-200 object-contain"
          />
          <p class="mt-4 text-sm font-semibold tracking-[0.08em] text-stone-700">Ink</p>
          <h1 class="mt-6 text-3xl font-semibold tracking-tight text-stone-950">
            {{ t("login.form.title") }}
          </h1>
        </header>

        <div class="mt-8 border-y border-stone-200 py-7 sm:mt-10 sm:py-8">
          <p
            v-if="noticeMessage"
            class="mb-5 border-l-2 border-emerald-600 py-1 pl-3 text-sm leading-6 text-emerald-700"
          >
            {{ noticeMessage }}
          </p>

          <form class="space-y-6" @submit.prevent="handleSubmit">
            <div>
              <label for="email" class="block text-sm font-medium text-stone-800">
                {{ t("login.form.accountLabel") }}
              </label>
              <input
                id="email"
                v-model="email"
                type="text"
                autocomplete="username"
                :placeholder="t('login.form.accountPlaceholder')"
                class="mt-2 w-full border-0 border-b border-stone-300 bg-transparent px-0 py-3 text-base text-stone-950 transition-colors placeholder:text-stone-400 focus:border-amber-700 focus:ring-0 focus:outline-none"
              />
            </div>
            <div>
              <label for="password" class="block text-sm font-medium text-stone-800">
                {{ t("login.form.passwordLabel") }}
              </label>
              <div
                class="mt-2 flex items-center gap-3 border-b border-stone-300 focus-within:border-amber-700"
              >
                <input
                  id="password"
                  v-model="password"
                  :type="passwordVisible ? 'text' : 'password'"
                  autocomplete="current-password"
                  :placeholder="t('login.form.passwordPlaceholder')"
                  class="min-w-0 flex-1 border-0 bg-transparent px-0 py-3 text-base text-stone-950 placeholder:text-stone-400 focus:ring-0 focus:outline-none"
                />
                <button
                  type="button"
                  class="min-h-10 shrink-0 px-1 text-sm font-medium text-stone-500 transition-colors hover:text-stone-950 focus-visible:ring-2 focus-visible:ring-amber-700 focus-visible:outline-none"
                  @click="passwordVisible = !passwordVisible"
                >
                  {{ passwordVisible ? t("common.actions.hide") : t("common.actions.show") }}
                </button>
              </div>
            </div>

            <p
              v-if="formError"
              class="border-l-2 border-rose-600 py-1 pl-3 text-sm leading-6 text-rose-700"
              role="alert"
            >
              {{ formError }}
            </p>

            <button
              type="submit"
              class="ui-btn-primary min-h-12 w-full px-6 text-base"
              :disabled="workspaceStore.authLoading"
            >
              {{
                workspaceStore.authLoading ? t("login.form.loggingIn") : t("common.actions.login")
              }}
            </button>
          </form>
        </div>
      </section>
    </main>
  </div>
</template>
