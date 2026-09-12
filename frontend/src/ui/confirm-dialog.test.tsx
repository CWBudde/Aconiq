import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConfirmDialog } from "./confirm-dialog";
import { m } from "@/i18n/messages";

function renderDialog(
  props: Partial<React.ComponentProps<typeof ConfirmDialog>> = {},
) {
  const onConfirm = vi.fn();
  const onOpenChange = vi.fn();
  render(
    <ConfirmDialog
      open
      onOpenChange={onOpenChange}
      title="Delete run-7"
      description="Its results are removed. The export bundle is kept."
      onConfirm={onConfirm}
      {...props}
    />,
  );
  return { onConfirm, onOpenChange };
}

describe("ConfirmDialog", () => {
  it("is an alert dialog, not a plain one", () => {
    // The distinction is not cosmetic: `alertdialog` tells a reader the
    // content demands a decision, and Radix drops the click-outside and
    // corner dismiss that a plain dialog offers.
    renderDialog();
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
  });

  it("carries both a title and an explanation", () => {
    renderDialog();
    const dialog = screen.getByRole("alertdialog");
    expect(dialog).toHaveAccessibleName("Delete run-7");
    expect(dialog).toHaveAccessibleDescription(
      "Its results are removed. The export bundle is kept.",
    );
  });

  it("opens with focus on the cancelling action", async () => {
    renderDialog();
    // The safe choice is the one a stray Enter should take.
    await vi.waitFor(() => {
      expect(
        screen.getByRole("button", { name: m.action_cancel() }),
      ).toHaveFocus();
    });
  });

  it("runs the action only when the reader confirms", async () => {
    const user = userEvent.setup();
    const { onConfirm } = renderDialog();

    await user.click(screen.getByRole("button", { name: m.action_cancel() }));
    expect(onConfirm).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: m.action_confirm() }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("cancels on Escape", async () => {
    const user = userEvent.setup();
    const { onConfirm, onOpenChange } = renderDialog();

    await user.keyboard("{Escape}");
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("lets the caller name both actions", () => {
    renderDialog({
      confirmLabel: m.action_discard(),
      cancelLabel: m.action_back(),
    });
    expect(
      screen.getByRole("button", { name: m.action_discard() }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: m.action_back() }),
    ).toBeInTheDocument();
  });

  it("paints the confirm action with the destructive token when nothing can be undone", () => {
    renderDialog({ tone: "destructive" });
    // A semantic token, never a Tailwind palette class: the contrast contract
    // in styles/tokens.test.ts only covers the tokens.
    expect(
      screen.getByRole("button", { name: m.action_confirm() }),
    ).toHaveClass("bg-destructive");
  });

  it("renders nothing while closed", () => {
    renderDialog({ open: false });
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });
});
