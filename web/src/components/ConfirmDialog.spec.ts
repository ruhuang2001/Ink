import { flushPromises, mount } from "@vue/test-utils";
import { afterEach, expect, it } from "vitest";

import ConfirmDialog from "@/components/ConfirmDialog.vue";
import { requestConfirm, resetConfirmsForTests } from "@/services/confirm";

afterEach(() => {
  resetConfirmsForTests();
  document.body.innerHTML = "";
});

async function openConfirm(request: Parameters<typeof requestConfirm>[0]) {
  const wrapper = mount(ConfirmDialog, { attachTo: document.body });
  const answer = requestConfirm(request);
  await flushPromises();
  return { wrapper, answer };
}

function buttonByText(text: string) {
  return [...document.querySelectorAll("dialog button")].find(
    (button) => button.textContent?.trim() === text,
  ) as HTMLButtonElement | undefined;
}

it("resolves true when the confirm action is chosen", async () => {
  const { wrapper, answer } = await openConfirm({ message: "继续吗？" });

  expect(document.querySelector("dialog")?.textContent).toContain("继续吗？");
  buttonByText("确认")?.click();
  await flushPromises();

  await expect(answer).resolves.toBe(true);
  expect(document.querySelector("dialog")?.hasAttribute("open")).toBe(false);
  wrapper.unmount();
});

it("treats cancelling, dismissing, and closing as no", async () => {
  for (const dismiss of [
    () => buttonByText("取消")?.click(),
    () =>
      document.querySelector("dialog")?.dispatchEvent(new Event("cancel", { cancelable: true })),
    () => document.querySelector("dialog")?.dispatchEvent(new PointerEvent("pointerdown")),
  ]) {
    const { wrapper, answer } = await openConfirm({ message: "继续吗？" });
    dismiss();
    await flushPromises();
    await expect(answer).resolves.toBe(false);
    wrapper.unmount();
  }
});

it("uses the caller's label and keeps a second request waiting its turn", async () => {
  const wrapper = mount(ConfirmDialog, { attachTo: document.body });
  const first = requestConfirm({ message: "第一件事", confirmLabel: "重新加载" });
  const second = requestConfirm({ message: "第二件事", confirmLabel: "丢弃", tone: "danger" });
  await flushPromises();

  expect(document.querySelector("dialog")?.textContent).toContain("第一件事");
  // The second request is invisible until the first is answered, so a caller
  // awaiting a decision never has its dialog replaced underneath it.
  expect(document.querySelector("dialog")?.textContent).not.toContain("第二件事");
  expect(buttonByText("重新加载")).toBeDefined();

  buttonByText("重新加载")?.click();
  await flushPromises();
  await expect(first).resolves.toBe(true);
  expect(document.querySelector("dialog")?.textContent).toContain("第二件事");
  expect(buttonByText("丢弃")?.className).toContain("ui-btn-danger");

  buttonByText("取消")?.click();
  await flushPromises();
  await expect(second).resolves.toBe(false);
  expect(document.querySelector("dialog")?.hasAttribute("open")).toBe(false);

  wrapper.unmount();
});
