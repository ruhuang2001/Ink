import { defineStore } from "pinia";
import { ref } from "vue";

type Design = "paper" | "structured";
const STORAGE_KEY = "ink.design.v1";
function isDesign(value: unknown): value is Design {
  return value === "paper" || value === "structured";
}

export const useDesignStore = defineStore("design", () => {
  const selectedDesign = ref<Design>("structured");
  const query = new URLSearchParams(window.location.search).get("design");
  if (isDesign(query)) {
    selectedDesign.value = query;
  } else {
    try {
      const saved = window.localStorage.getItem(STORAGE_KEY);
      if (isDesign(saved)) selectedDesign.value = saved;
    } catch {
      // Keep the default when browser storage is unavailable.
    }
  }

  function previewDesign(value: unknown) {
    if (isDesign(value)) selectedDesign.value = value;
  }

  function setDesign(design: Design) {
    selectedDesign.value = design;
    try {
      window.localStorage.setItem(STORAGE_KEY, design);
    } catch {
      // The selection still applies for the current session.
    }
  }

  return { selectedDesign, previewDesign, setDesign };
});
