import { mount } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import PrintPreview from "@/components/PrintPreview.vue";

let resize: () => void;
let width = 432;
const disconnect = vi.fn<() => void>();

beforeEach(() => {
  width = 432;
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockImplementation(() => width);
  vi.stubGlobal(
    "ResizeObserver",
    class {
      constructor(callback: () => void) {
        resize = callback;
      }
      observe() {}
      disconnect = disconnect;
    },
  );
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("PrintPreview", () => {
  it("scales the sheet down to fit a narrow viewport and back up", async () => {
    const wrapper = mount(PrintPreview, {
      props: { title: "纸条", content: "内容", image: "png" },
    });
    const sheet = () => wrapper.find(".print-preview-sheet").attributes("style");
    expect(sheet()).toContain("zoom: 1");
    width = 240;
    resize();
    await wrapper.vm.$nextTick();
    expect(sheet()).toContain("zoom: 0.5");
    expect(wrapper.find("img").attributes("src")).toBe("data:image/png;base64,png");
    width = 432;
    resize();
    await wrapper.vm.$nextTick();
    expect(sheet()).toContain("zoom: 1");
  });

  it("releases its observer on unmount", () => {
    const wrapper = mount(PrintPreview, { props: { title: "纸条", content: "内容" } });
    wrapper.unmount();
    expect(disconnect).toHaveBeenCalled();
  });

  it("preserves content when an image fails and recovers when a new image arrives", async () => {
    const wrapper = mount(PrintPreview, {
      props: { title: "纸条", content: "内容", image: "png" },
    });
    expect(wrapper.find(".print-preview-content").exists()).toBe(false);
    await wrapper.find("img").trigger("error");
    expect(wrapper.find(".print-preview-content").text()).toBe("内容");
    await wrapper.setProps({ image: "new-png" });
    expect(wrapper.find("img").exists()).toBe(true);
  });
});
