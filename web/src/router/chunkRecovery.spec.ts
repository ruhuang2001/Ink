import { createMemoryHistory, createRouter } from "vue-router";

import { installChunkRecovery, routeLoadFailure } from "@/router/chunkRecovery";

it.each([
  "Failed to fetch dynamically imported module",
  "Unable to preload CSS for /assets/settings.css",
])(
  "recovers %s under the deployment base and exposes a retry after another failure",
  async (message) => {
    const router = createRouter({
      history: createMemoryHistory("/ink/"),
      routes: [{ path: "/settings", component: () => Promise.reject(new Error(message)) }],
    });
    const navigate = vi.fn<(target: string) => void>();
    installChunkRecovery(router, navigate);
    await expect(router.push("/settings?tab=devices")).rejects.toThrow(message);
    expect(navigate).toHaveBeenCalledWith("/ink/settings?tab=devices");
    await expect(router.push("/settings?tab=devices")).rejects.toThrow(message);
    expect(navigate).toHaveBeenCalledTimes(1);
    expect(routeLoadFailure.value).toBe("/ink/settings?tab=devices");
    routeLoadFailure.value = null;
  },
);
