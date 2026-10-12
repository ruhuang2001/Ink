import { createPinia, setActivePinia } from "pinia";

import { useDesignStore } from "./design";

describe("design preference", () => {
  beforeEach(() => {
    localStorage.removeItem("ink.design.v1");
    window.history.replaceState({}, "", "/");
    setActivePinia(createPinia());
  });
  it("defaults to structured and persists an explicit original design", () => {
    const store = useDesignStore();
    expect(store.selectedDesign).toBe("structured");
    store.setDesign("paper");
    setActivePinia(createPinia());
    expect(useDesignStore().selectedDesign).toBe("paper");
  });
  it("allows previewing a design without overwriting the saved preference", () => {
    window.history.replaceState({}, "", "/?design=structured");
    const store = useDesignStore();
    expect(store.selectedDesign).toBe("structured");
    expect(localStorage.getItem("ink.design.v1")).toBeNull();
    store.setDesign("paper");
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
  it("previews the original design without accessing blocked storage", () => {
    window.history.replaceState({}, "", "/?design=paper");
    const read = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    expect(useDesignStore().selectedDesign).toBe("paper");
    expect(read).not.toHaveBeenCalled();
    read.mockRestore();
  });
});
