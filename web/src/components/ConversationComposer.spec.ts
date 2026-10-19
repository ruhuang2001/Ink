import { mount } from "@vue/test-utils";
import { afterEach, describe, expect, it, vi } from "vitest";

import ConversationComposer from "@/components/ConversationComposer.vue";

const props = {
  modelValue: "第一行\n第二行",
  generating: false,
  saving: false,
  saveStatus: "保存草稿",
  saveError: false,
};
afterEach(() => vi.restoreAllMocks());

describe("ConversationComposer", () => {
  it("keeps Enter for newlines and sends with either platform modifier", async () => {
    const wrapper = mount(ConversationComposer, { props });
    const input = wrapper.get("textarea");
    await input.trigger("keydown", { key: "Enter" });
    expect(wrapper.emitted("send")).toBeUndefined();
    await input.trigger("keydown", { key: "Enter", ctrlKey: true });
    await input.trigger("keydown", { key: "Enter", metaKey: true });
    expect(wrapper.emitted("send")).toHaveLength(2);
  });

  it("does not send while composing CJK text, on legacy IME events or key repeat", async () => {
    const wrapper = mount(ConversationComposer, { props });
    const input = wrapper.get("textarea");
    await input.trigger("compositionstart");
    await input.trigger("keydown", { key: "Enter", ctrlKey: true });
    await input.trigger("compositionend");
    await input.trigger("keydown", { key: "Enter", ctrlKey: true, isComposing: true });
    await input.trigger("keydown", { key: "Enter", ctrlKey: true, keyCode: 229 });
    await input.trigger("keydown", { key: "Enter", ctrlKey: true, repeat: true });
    expect(wrapper.emitted("send")).toBeUndefined();
    await input.trigger("keydown", { key: "Enter", ctrlKey: true });
    expect(wrapper.emitted("send")).toHaveLength(1);
  });

  it("blocks empty or in-flight submissions from buttons and keyboard", async () => {
    const wrapper = mount(ConversationComposer, { props: { ...props, modelValue: "  " } });
    expect(wrapper.get(".conversation-send").attributes("disabled")).toBeDefined();
    await wrapper.get("textarea").trigger("keydown", { key: "Enter", ctrlKey: true });
    await wrapper.setProps({ modelValue: "message", generating: true });
    await wrapper.get("textarea").trigger("keydown", { key: "Enter", metaKey: true });
    expect(wrapper.emitted("send")).toBeUndefined();
  });

  it("resizes restored drafts, caps long input and shrinks after sending", async () => {
    let height = 90;
    vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockImplementation(() => height);
    const wrapper = mount(ConversationComposer, { props });
    expect((wrapper.get("textarea").element as HTMLTextAreaElement).style.height).toBe("90px");
    height = 400;
    await wrapper.setProps({ modelValue: "long draft" });
    await wrapper.vm.$nextTick();
    expect((wrapper.get("textarea").element as HTMLTextAreaElement).style.height).toBe("160px");
    height = 64;
    await wrapper.setProps({ modelValue: "" });
    await wrapper.vm.$nextTick();
    expect((wrapper.get("textarea").element as HTMLTextAreaElement).style.height).toBe("64px");
  });
});
