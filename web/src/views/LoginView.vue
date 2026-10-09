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
    class="relative flex min-h-[100dvh] bg-[color:var(--app-bg)] px-5 pt-[calc(env(safe-area-inset-top)+1rem)] pb-[calc(env(safe-area-inset-bottom)+1.5rem)] text-stone-900 sm:px-8 sm:py-8"
  >
    <button
      type="button"
      class="absolute top-[calc(env(safe-area-inset-top)+1rem)] left-5 inline-flex min-h-10 items-center gap-2 text-sm font-medium text-stone-500 transition-colors hover:text-stone-950 focus-visible:ring-2 focus-visible:ring-stone-700 focus-visible:ring-offset-4 focus-visible:outline-none sm:top-8 sm:left-8"
      @click="handleBack"
    >
      <span aria-hidden="true">←</span>
      <span>{{ t("common.actions.back") }}</span>
    </button>

    <main class="mx-auto flex w-full max-w-2xl flex-1 items-center py-20 sm:py-16">
      <div
        class="grid w-full lg:grid-cols-[15rem_minmax(0,1fr)] lg:overflow-hidden lg:border lg:border-stone-200 lg:bg-[color:var(--app-surface-raised)] lg:shadow-[0_18px_50px_rgba(28,25,23,0.06)]"
      >
        <section
          class="hidden min-h-[28rem] items-center justify-center border-r border-stone-200 bg-[color:var(--app-surface-soft)] lg:flex"
        >
          <div class="text-center">
            <img src="/icon.png" alt="" class="mx-auto h-16 w-16 rounded-lg object-contain" />
            <p class="mt-5 text-xl font-semibold tracking-tight text-stone-950">Ink</p>
          </div>
        </section>

        <section class="w-full max-w-sm justify-self-center lg:max-w-none lg:p-12">
          <header>
            <div class="flex items-center gap-3 lg:hidden">
              <img src="/icon.png" alt="" class="h-10 w-10 rounded-md object-contain" />
              <p class="text-sm font-semibold text-stone-700">Ink</p>
            </div>
            <h1 class="mt-8 text-3xl font-semibold tracking-tight text-stone-950 lg:mt-0">
              {{ t("login.form.title") }}
            </h1>
          </header>

          <div class="mt-8">
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
                  class="mt-2 w-full border-0 border-b border-stone-300 bg-transparent px-0 py-3 text-base text-stone-950 transition-colors placeholder:text-stone-400 focus:border-stone-700 focus:ring-0 focus:outline-none"
                />
              </div>
              <div>
                <label for="password" class="block text-sm font-medium text-stone-800">
                  {{ t("login.form.passwordLabel") }}
                </label>
                <div
                  class="mt-2 flex items-center gap-3 border-b border-stone-300 focus-within:border-stone-700"
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
                    class="min-h-10 shrink-0 px-1 text-sm font-medium text-stone-500 transition-colors hover:text-stone-950 focus-visible:ring-2 focus-visible:ring-stone-700 focus-visible:outline-none"
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
      </div>
    </main>
  </div>
</template>
