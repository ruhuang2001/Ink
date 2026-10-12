import { defineStore } from "pinia";
import { ref } from "vue";

type Design = "paper" | "structured";
const STORAGE_KEY = "ink.design.v1";

export const useDesignStore = defineStore("design", () => {
  const selectedDesign = ref<Design>("paper");
  try {
    const query = new URLSearchParams(window.location.search).get("design");
    const saved = window.localStorage.getItem(STORAGE_KEY);
    const value = query === "structured" || query === "paper" ? query : saved;
    if (value === "structured" || value === "paper") selectedDesign.value = value;
  } catch {
    // Keep the original theme when browser storage is unavailable.
  }

  function setDesign(design: Design) {
    selectedDesign.value = design;
    try {
      window.localStorage.setItem(STORAGE_KEY, design);
      const url = new URL(window.location.href);
      if (url.searchParams.has("design")) {
        url.searchParams.delete("design");
        window.history.replaceState(window.history.state, "", url);
      }
    } catch {
      // The selection still applies for the current session.
    }
  }

  return { selectedDesign, setDesign };
});
