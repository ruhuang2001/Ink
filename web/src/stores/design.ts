import { defineStore } from "pinia";
import { ref } from "vue";

type Design = "paper" | "structured";
const STORAGE_KEY = "ink.design.v1";
function isDesign(value: unknown): value is Design {
  return value === "paper" || value === "structured";
}

export const useDesignStore = defineStore("design", () => {
  const selectedDesign = ref<Design>("structured");
  let explicitDesign: Design | undefined;
  function readExplicitDesign(): Design {
    if (explicitDesign !== undefined) return explicitDesign;
    explicitDesign = "structured";
    try {
      const saved = window.localStorage.getItem(STORAGE_KEY);
      if (isDesign(saved)) explicitDesign = saved;
    } catch {
      // Keep the default when browser storage is unavailable.
    }
    return explicitDesign;
  }

  function previewDesign(value: unknown) {
    selectedDesign.value = isDesign(value) ? value : readExplicitDesign();
  }
  previewDesign(new URLSearchParams(window.location.search).get("design"));

  function setDesign(design: Design) {
    explicitDesign = design;
    selectedDesign.value = design;
    try {
      window.localStorage.setItem(STORAGE_KEY, design);
    } catch {
      // The selection still applies for the current session.
    }
  }

  return { selectedDesign, previewDesign, setDesign };
});
