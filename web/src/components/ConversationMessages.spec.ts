import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";

import ConversationMessages from "@/components/ConversationMessages.vue";
import type { ConversationMessage } from "@/types/workspace";

const message: ConversationMessage = {
  id: "m1",
  role: "assistant",
  text: "第一行\n第二行\n<script>alert(1)</script>",
  createdAt: "2026-10-15T00:00:00Z",
};
const props = {
  conversationId: "c1",
  messages: [message],
  generating: false,
  selecting: false,
  selectedIds: [] as string[],
  emptyHint: "开始写作",
};

describe("ConversationMessages", () => {
  it("keeps multiline text literal and separates reading from print selection", async () => {
    const wrapper = mount(ConversationMessages, { props });
    expect(wrapper.get(".conversation-message-text").element.textContent).toBe(message.text);
    expect(wrapper.find("script").exists()).toBe(false);
    await wrapper.get(".conversation-message-text").trigger("click");
    expect(wrapper.emitted("toggle")).toBeUndefined();
    await wrapper.setProps({ selecting: true });
    await wrapper.get("input").setValue(true);
    expect(wrapper.emitted("toggle")).toEqual([["m1"]]);
    await wrapper.setProps({ selectedIds: ["m1"] });
    expect((wrapper.get("input").element as HTMLInputElement).checked).toBe(true);
  });

  it("follows new messages at the bottom but preserves reading position until asked", async () => {
    const wrapper = mount(ConversationMessages, { props });
    await flushPromises();
    const viewport = wrapper.get(".conversation-message-viewport");
    const el = viewport.element as HTMLElement;
    let height = 1000;
    Object.defineProperty(el, "clientHeight", { get: () => 300 });
    Object.defineProperty(el, "scrollHeight", { get: () => height });
    el.scrollTop = 700;
    await viewport.trigger("scroll");
    height = 1200;
    await wrapper.setProps({ messages: [message, { ...message, id: "m2" }] });
    expect(el.scrollTop).toBe(1200);
    el.scrollTop = 200;
    await viewport.trigger("scroll");
    height = 1400;
    await wrapper.setProps({ messages: [message, { ...message, id: "m2", text: "more text" }] });
    expect(el.scrollTop).toBe(200);
    await wrapper.get(".conversation-jump").trigger("click");
    expect(el.scrollTop).toBe(1400);
    expect(wrapper.find(".conversation-jump").exists()).toBe(false);
    el.scrollTop = 200;
    await viewport.trigger("scroll");
    await wrapper.setProps({ conversationId: "c2" });
    expect(el.scrollTop).toBe(1400);
  });
});
