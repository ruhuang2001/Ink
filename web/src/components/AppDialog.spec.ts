import { mount } from "@vue/test-utils";
import { afterEach, expect, it } from "vitest";
import { nextTick } from "vue";

import AppDialog from "@/components/AppDialog.vue";

afterEach(() => {
  document.querySelector(".ink-page")?.remove();
});

it("places nested dialogs outside the animated page and closes only the top dialog on Escape", async () => {
  const shell = document.createElement("div");
  shell.className = "ink-page";
  const page = document.createElement("main");
  page.style.transform = "translateY(0)";
  shell.append(page);
  document.body.append(shell);
  const editor = mount(AppDialog, { attachTo: page, props: { open: true, title: "编辑" } });
  const preview = mount(AppDialog, {
    attachTo: page,
    props: { open: true, title: "预览", size: "wide" },
    slots: { default: '<button id="last-preview-control">缩放</button>' },
  });
  await nextTick();
  const dialogs = shell.querySelectorAll('[role="dialog"]');
  expect(dialogs.length).toBe(2);
  expect(page.querySelector('[role="dialog"]')).toBeNull();
  const close = dialogs[1]!.querySelector("button")!;
  expect(document.activeElement).toBe(close);
  document.dispatchEvent(
    new KeyboardEvent("keydown", { key: "Tab", shiftKey: true, cancelable: true }),
  );
  expect(document.activeElement?.id).toBe("last-preview-control");
  document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", cancelable: true }));
  expect(preview.emitted("close")).toHaveLength(1);
  expect(editor.emitted("close")).toBeUndefined();
  preview.unmount();
  editor.unmount();
});

it("teleports to body without a shell and restores focus after closing", async () => {
  const host = document.createElement("div");
  host.style.transform = "translateY(0)";
  const trigger = document.createElement("button");
  host.append(trigger);
  document.body.append(host);
  trigger.focus();
  const wrapper = mount(AppDialog, { attachTo: host, props: { open: true, title: "Fallback" } });
  await nextTick();
  expect(host.querySelector('[role="dialog"]')).toBeNull();
  expect(document.body.querySelector('[aria-label="Fallback"]')).not.toBeNull();
  await wrapper.setProps({ open: false });
  expect(document.activeElement).toBe(trigger);
  wrapper.unmount();
  host.remove();
});
