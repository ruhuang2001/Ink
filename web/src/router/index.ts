import {
  createMemoryHistory,
  createRouter,
  createWebHistory,
  type RouteRecordRaw,
} from "vue-router";
import type { RouterHistory } from "vue-router";

import { translate } from "@/i18n";
import AppShell from "@/layouts/AppShell.vue";
import { DEFAULT_LOGIN_REDIRECT, resolveLoginRedirect } from "@/router/authRedirect";
import { pinia } from "@/stores/pinia";
import { useWorkspaceStore } from "@/stores/workspace";

const ConversationsView = () => import("@/views/ConversationsView.vue");
const LoginView = () => import("@/views/LoginView.vue");
const PrintsView = () => import("@/views/PrintsView.vue");
const SettingsView = () => import("@/views/SettingsView.vue");
const StatusView = () => import("@/views/StatusView.vue");
const TutorialView = () => import("@/views/TutorialView.vue");

declare module "vue-router" {
  interface RouteMeta {
    labelKey?: string;
    titleKey?: string;
    descriptionKey?: string;
    navHintKey?: string;
    requiresAuth?: boolean;
    showInNav?: boolean;
  }
}

const shellChildren: RouteRecordRaw[] = [
  {
    path: "conversations",
    name: "conversations",
    component: ConversationsView,
    meta: {
      labelKey: "navigation.conversations.label",
      titleKey: "navigation.conversations.title",
      descriptionKey: "navigation.conversations.description",
      navHintKey: "navigation.conversations.navHint",
    },
  },
  {
    path: "status",
    name: "status",
    component: StatusView,
    meta: {
      labelKey: "navigation.status.label",
      titleKey: "navigation.status.title",
      descriptionKey: "navigation.status.description",
      navHintKey: "navigation.status.navHint",
    },
  },
  {
    path: "prints",
    name: "prints",
    component: PrintsView,
    meta: {
      labelKey: "navigation.prints.label",
      titleKey: "navigation.prints.title",
      descriptionKey: "navigation.prints.description",
      navHintKey: "navigation.prints.navHint",
    },
  },
  {
    path: "tutorial",
    name: "tutorial",
    component: TutorialView,
    meta: {
      labelKey: "navigation.tutorial.label",
      titleKey: "navigation.tutorial.title",
      descriptionKey: "navigation.tutorial.description",
      navHintKey: "navigation.tutorial.navHint",
    },
  },
  {
    path: "settings",
    name: "settings",
    component: SettingsView,
    meta: {
      labelKey: "navigation.settings.label",
      titleKey: "navigation.settings.title",
      descriptionKey: "navigation.settings.description",
      navHintKey: "navigation.settings.navHint",
      requiresAuth: true,
    },
  },
];

export const navigationItems = shellChildren
  .filter((route) => route.meta?.showInNav !== false)
  .map((route) => ({
    name: route.name as string,
    path: `/${route.path}`,
    labelKey: route.meta?.labelKey as string,
    navHintKey: route.meta?.navHintKey as string,
  }));

export const routes: RouteRecordRaw[] = [
  {
    path: "/",
    component: AppShell,
    redirect: "/conversations",
    children: shellChildren,
  },
  {
    path: "/login",
    name: "login",
    component: LoginView,
    meta: {
      titleKey: "navigation.login.title",
      descriptionKey: "navigation.login.description",
    },
  },
  {
    path: "/connections",
    redirect: "/prints",
  },
];

export function createAppRouter(
  history: RouterHistory = createWebHistory(import.meta.env.BASE_URL),
  piniaInstance = pinia,
) {
  const router = createRouter({
    history,
    routes,
  });

  router.onError((error) => {
    if (!isDynamicImportError(error) || typeof window === "undefined") {
      return;
    }

    const reloadKey = "ink.route-chunk-reload";
    try {
      if (window.sessionStorage.getItem(reloadKey) === window.location.href) {
        return;
      }
      window.sessionStorage.setItem(reloadKey, window.location.href);
      window.location.reload();
    } catch {
      // Storage and reload can be unavailable in embedded or restricted browsers.
    }
  });

  router.beforeEach(async (to) => {
    const workspaceStore = useWorkspaceStore(piniaInstance);

    if (
      workspaceStore.authSession &&
      !workspaceStore.authUser &&
      !workspaceStore.authBootstrapping
    ) {
      await workspaceStore.initializeAuth();
    }

    const isAuthenticated = workspaceStore.isAuthenticated;

    if (to.meta.requiresAuth && !isAuthenticated) {
      return {
        path: "/login",
        query: {
          redirect: to.fullPath,
        },
      };
    }

    if (to.path === "/login" && isAuthenticated) {
      return resolveLoginRedirect(router, to.query.redirect ?? DEFAULT_LOGIN_REDIRECT);
    }

    return true;
  });

  router.afterEach((to) => {
    if (typeof window !== "undefined") {
      window.sessionStorage.removeItem("ink.route-chunk-reload");
    }
    const title = to.meta.titleKey
      ? `${translate("app.name")} · ${translate(to.meta.titleKey)}`
      : translate("app.name");
    document.title = title;
  });

  return router;
}

function isDynamicImportError(error: unknown) {
  const message = error instanceof Error ? error.message : String(error);
  return /dynamically imported module|importing a module script failed|loading chunk/i.test(
    message,
  );
}

export function createTestRouter(piniaInstance = pinia) {
  return createAppRouter(createMemoryHistory(), piniaInstance);
}

const router = createAppRouter();

export default router;
