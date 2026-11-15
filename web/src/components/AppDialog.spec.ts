import { mount } from "@vue/test-utils";
import { afterEach, expect, it } from "vitest";
import { nextTick } from "vue";

import AppDialog from "@/components/AppDialog.vue";

afterEach(() => {
  document.body.innerHTML = "";
});

it("opens each dialog as a modal element and nests content inside it", async () => {
  const editor = mount(AppDialog, {
    attachTo: document.body,
    props: { open: true, title: "编辑" },
  });
  const preview = mount(AppDialog, {
    attachTo: document.body,
    props: { open: true, title: "预览", size: "wide" },
    slots: { default: '<button id="last-preview-control">缩放</button>' },
  });
  await nextTick();

  const dialogs = document.querySelectorAll("dialog");
  expect(dialogs).toHaveLength(2);
  expect([...dialogs].map((dialog) => dialog.open)).toEqual([true, true]);

  // The top-layer modal needs no portal: the rendered tree stays inside the
  // component that opened it, and content is reachable inside the dialog.
  const previewDialog = dialogs[1]!;
  expect(previewDialog.contains(document.querySelector("#last-preview-control"))).toBe(true);
  expect(previewDialog.getAttribute("aria-label")).toBe("预览");

  preview.unmount();
  editor.unmount();
});

it("reports a close request from the browser and leaves the element to the owner", async () => {
  const wrapper = mount(AppDialog, {
    attachTo: document.body,
    props: { open: true, title: "编辑" },
  });
  await nextTick();
  const dialog = document.querySelector("dialog") as HTMLDialogElement;

  // The platform fires cancel for Escape. The component declines the default
  // close so the owner decides, which keeps its state and the DOM in step.
  dialog.dispatchEvent(new Event("cancel", { cancelable: true }));

  expect(wrapper.emitted("close")).toHaveLength(1);
  expect(dialog.open).toBe(true);

  wrapper.unmount();
});

it("applies the open prop to the element instead of mounting and unmounting", async () => {
  const wrapper = mount(AppDialog, {
    attachTo: document.body,
    props: { open: true, title: "Fallback" },
  });
  await nextTick();
  const dialog = document.querySelector("dialog") as HTMLDialogElement;
  expect(dialog.open).toBe(true);

  await wrapper.setProps({ open: false });
  await nextTick();
  expect(dialog.open).toBe(false);
  expect(document.querySelector("dialog")).not.toBeNull();

  wrapper.unmount();
  expect(document.querySelector("dialog")).toBeNull();
});

it("closes on a pointer down outside the panel but not inside it", async () => {
  const wrapper = mount(AppDialog, {
    attachTo: document.body,
    props: { open: true, title: "Backdrop" },
    slots: { default: '<button id="inside">inside</button>' },
  });
  await nextTick();
  const dialog = document.querySelector("dialog") as HTMLDialogElement;

  // ::backdrop is not a node, so an outside click targets the dialog element.
  dialog
    .querySelector("#inside")
    ?.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true }));
  expect(wrapper.emitted("close")).toBeUndefined();

  dialog.dispatchEvent(new PointerEvent("pointerdown"));
  expect(wrapper.emitted("close")).toHaveLength(1);

  wrapper.unmount();
});
