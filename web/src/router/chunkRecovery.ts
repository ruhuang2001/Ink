import { ref } from "vue";
import type { Router } from "vue-router";

const reloadKey = "ink.route-chunk-reload";
export const routeLoadFailure = ref<string | null>(null);

export function installChunkRecovery(
  router: Router,
  navigate = (target: string) => window.location.assign(target),
) {
  router.onError((error, to) => {
    const message = error instanceof Error ? error.message : String(error);
    if (
      typeof window === "undefined" ||
      !/dynamically imported module|importing a module script failed|loading chunk|unable to preload css/i.test(
        message,
      )
    ) {
      return;
    }

    const target = to ? router.resolve(to.fullPath).href : window.location.href;
    routeLoadFailure.value = target;
    try {
      if (window.sessionStorage.getItem(reloadKey) === target) return;
      window.sessionStorage.setItem(reloadKey, target);
    } catch {
      // Keep the retry surface available when browser storage is restricted.
      return;
    }
    navigate(target);
  });

  router.afterEach((_to, _from, failure) => {
    if (failure) return;
    routeLoadFailure.value = null;
    try {
      window.sessionStorage.removeItem(reloadKey);
    } catch {
      // Storage can be unavailable in embedded browsers.
    }
  });
}

export function retryRouteLoad() {
  if (!routeLoadFailure.value) return;
  try {
    window.sessionStorage.removeItem(reloadKey);
  } catch {
    // An explicit retry does not need persistent reload-loop protection.
  }
  window.location.assign(routeLoadFailure.value);
}
