import { config, enableAutoUnmount } from "@vue/test-utils";
import { disposePinia, getActivePinia, setActivePinia } from "pinia";
import { afterEach, beforeEach, vi } from "vitest";

import i18n, { setI18nLocale } from "@/i18n";
import { configureAuthRefresh } from "@/services/http";

afterEach(() => {
  const pinia = getActivePinia();
  if (pinia) disposePinia(pinia);
  setActivePinia(undefined);
  configureAuthRefresh(null);
});

enableAutoUnmount(afterEach);

type MatchMediaEventHandler = (
  type: string,
  listener: EventListenerOrEventListenerObject | null,
) => void;
type LegacyMediaQueryListenerHandler = (
  listener: ((event: MediaQueryListEvent) => void) | null,
) => void;
type MatchMediaDispatchEventHandler = (event: Event) => boolean;
type MatchMediaFactory = (query: string) => MediaQueryList;

config.global.stubs = {
  transition: false,
};
config.global.plugins = [i18n];

Object.defineProperty(window.navigator, "language", {
  configurable: true,
  value: "zh-CN",
});

function createMemoryStorage(): Storage {
  const values = new Map<string, string>();

  return {
    get length() {
      return values.size;
    },
    clear() {
      values.clear();
    },
    getItem(key) {
      return values.get(key) ?? null;
    },
    key(index) {
      return [...values.keys()][index] ?? null;
    },
    removeItem(key) {
      values.delete(key);
    },
    setItem(key, value) {
      values.set(key, value);
    },
  };
}

function hasStorageApi(storage: Storage | undefined) {
  return (
    typeof storage?.getItem === "function" &&
    typeof storage.setItem === "function" &&
    typeof storage.removeItem === "function" &&
    typeof storage.clear === "function"
  );
}

if (!hasStorageApi(window.localStorage)) {
  Object.defineProperty(window, "localStorage", {
    configurable: true,
    value: createMemoryStorage(),
  });
}

if (!hasStorageApi(window.sessionStorage)) {
  Object.defineProperty(window, "sessionStorage", {
    configurable: true,
    value: createMemoryStorage(),
  });
}

// jsdom exposes HTMLDialogElement but not showModal() or close(). Without them a
// component that opens a native dialog fails on first use, so the test
// environment gets just enough behavior to exercise the open state and events.
function reflectDialogOpen(dialog: HTMLDialogElement, open: boolean) {
  if (open) {
    dialog.setAttribute("open", "");
    return;
  }

  dialog.removeAttribute("open");
}

if (typeof HTMLDialogElement !== "undefined" && !HTMLDialogElement.prototype.showModal) {
  HTMLDialogElement.prototype.showModal = function showModal(this: HTMLDialogElement) {
    reflectDialogOpen(this, true);
  };
  HTMLDialogElement.prototype.show = function show(this: HTMLDialogElement) {
    reflectDialogOpen(this, true);
  };
  HTMLDialogElement.prototype.close = function close(this: HTMLDialogElement) {
    if (!this.open) {
      return;
    }
    reflectDialogOpen(this, false);
    this.dispatchEvent(new Event("close"));
  };
}

beforeEach(() => {
  setI18nLocale("zh-CN");
  window.localStorage.clear();
  window.sessionStorage.clear();
  document.title = "Ink";
  document.documentElement.lang = "zh-CN";
  delete document.documentElement.dataset.theme;
  delete document.documentElement.dataset.colorMode;
  document.documentElement.style.colorScheme = "";
});

if (typeof window.matchMedia !== "function") {
  Object.defineProperty(window, "matchMedia", {
    configurable: true,
    value: vi.fn<MatchMediaFactory>((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn<MatchMediaEventHandler>(() => undefined),
      removeEventListener: vi.fn<MatchMediaEventHandler>(() => undefined),
      addListener: vi.fn<LegacyMediaQueryListenerHandler>(() => undefined),
      removeListener: vi.fn<LegacyMediaQueryListenerHandler>(() => undefined),
      dispatchEvent: vi.fn<MatchMediaDispatchEventHandler>(() => true),
    })),
  });
}
