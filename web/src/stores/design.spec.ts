import { createPinia, setActivePinia } from "pinia";

import { useDesignStore } from "./design";

describe("design preference", () => {
  beforeEach(() => {
    localStorage.removeItem("ink.design.v1");
    window.history.replaceState({}, "", "/");
    setActivePinia(createPinia());
  });
  it("defaults to the original design and persists the second design", () => {
    const store = useDesignStore();
    expect(store.selectedDesign).toBe("paper");
    store.setDesign("structured");
    setActivePinia(createPinia());
    expect(useDesignStore().selectedDesign).toBe("structured");
  });
  it("allows previewing a design without overwriting the saved preference", () => {
    window.history.replaceState({}, "", "/?design=structured");
    const store = useDesignStore();
    expect(store.selectedDesign).toBe("structured");
    expect(localStorage.getItem("ink.design.v1")).toBeNull();
    store.setDesign("paper");
    expect(window.location.search).toBe("");
    expect(localStorage.getItem("ink.design.v1")).toBe("paper");
  });
  it("ignores an invalid preview value and retains the saved preference", () => {
    localStorage.setItem("ink.design.v1", "structured");
    window.history.replaceState({}, "", "/?design=unknown");
    expect(useDesignStore().selectedDesign).toBe("structured");
  });
  it("applies the selection even when storage is blocked", () => {
    const write = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("Storage blocked");
    });
    const store = useDesignStore();
    store.setDesign("structured");
    expect(store.selectedDesign).toBe("structured");
    write.mockRestore();
  });
});
