import { ref } from "vue";

export type ConfirmRequest = {
  message: string;
  confirmLabel?: string;
  /** Destructive choices get a warning tone. */
  tone?: "default" | "danger";
};

type PendingConfirm = ConfirmRequest & { settle: (confirmed: boolean) => void };

// The oldest unanswered request is the one shown; the rest wait their turn.
const queue = ref<PendingConfirm[]>([]);

/** The request the host should currently render, if any. */
export const activeConfirm = ref<PendingConfirm | null>(null);

/**
 * Asks the user to confirm an action and resolves with their answer.
 *
 * Requests queue instead of replacing each other: a caller awaiting a decision
 * must not have its dialog swapped out from under it, and a double click must not
 * leave the first answer unread.
 */
export function requestConfirm(request: ConfirmRequest): Promise<boolean> {
  return new Promise((resolve) => {
    queue.value = [...queue.value, { ...request, settle: resolve }] as PendingConfirm[];
    if (!activeConfirm.value) {
      activeConfirm.value = queue.value[0] ?? null;
    }
  });
}

/** Answers the pending request. Answering nothing is treated as a cancellation. */
export function settleConfirm(confirmed: boolean) {
  const current = activeConfirm.value;
  if (!current) {
    return;
  }

  queue.value = queue.value.filter((item) => item !== current);
  activeConfirm.value = queue.value[0] ?? null;
  current.settle(confirmed);
}

export function resetConfirmsForTests() {
  const waiting = queue.value;
  queue.value = [];
  activeConfirm.value = null;
  for (const item of waiting) {
    item.settle(false);
  }
}
