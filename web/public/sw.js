const CACHE_NAME = "ink-shell-v2";
const APP_SHELL_PATHS = [
  "./",
  "./site.webmanifest",
  "./favicon.svg",
  "./icon.jpg",
  "./apple-touch-icon.png",
  "./pwa-192.png",
  "./pwa-512.png",
];

function resolveAppUrl(path) {
  return new URL(path, self.registration.scope).toString();
}

const APP_SHELL_URL = resolveAppUrl("./");
const PRECACHE_URLS = APP_SHELL_PATHS.map((path) => resolveAppUrl(path));

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(CACHE_NAME)
      .then((cache) =>
        cache.addAll(PRECACHE_URLS.map((url) => new Request(url, { cache: "reload" }))),
      ),
  );
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    Promise.all([
      caches
        .keys()
        .then((keys) =>
          Promise.all(keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key))),
        ),
      self.clients.claim(),
    ]),
  );
});

function isStaticAsset(request, url) {
  if (url.origin !== self.location.origin) {
    return false;
  }

  if (url.pathname.includes("/api/")) {
    return false;
  }

  return (
    request.destination === "style" ||
    request.destination === "script" ||
    request.destination === "worker" ||
    request.destination === "font" ||
    request.destination === "image" ||
    url.pathname.includes("/assets/") ||
    url.pathname.endsWith(".webmanifest")
  );
}

async function respondToNavigation(request) {
  const cache = await caches.open(CACHE_NAME);

  try {
    const response = await fetch(request);
    await cache.put(APP_SHELL_URL, response.clone());
    return response;
  } catch {
    return (await caches.match(request)) || (await caches.match(APP_SHELL_URL)) || Response.error();
  }
}

async function respondToStaticAsset(request) {
  const cached = await caches.match(request);

  if (cached) {
    void fetch(request)
      .then(async (response) => {
        if (!response || response.status >= 400) {
          return;
        }

        const cache = await caches.open(CACHE_NAME);
        await cache.put(request, response.clone());
      })
      .catch(() => undefined);

    return cached;
  }

  const response = await fetch(request);
  if (response && response.status < 400) {
    const cache = await caches.open(CACHE_NAME);
    await cache.put(request, response.clone());
  }
  return response;
}

self.addEventListener("fetch", (event) => {
  const { request } = event;
  if (request.method !== "GET") {
    return;
  }

  const url = new URL(request.url);
  if (request.mode === "navigate") {
    event.respondWith(respondToNavigation(request));
    return;
  }

  if (isStaticAsset(request, url)) {
    event.respondWith(respondToStaticAsset(request));
  }
});
