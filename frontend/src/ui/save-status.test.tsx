import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { APIRequestError, ERROR_CODE_MODEL_INVALID } from "@/api/api-error";
import type { ProjectSync } from "@/model/use-project-sync";
import { SaveStatus } from "./save-status";
import { m } from "@/i18n/messages";

const sync = vi.hoisted(() => {
  const value: {
    current: {
      enabled: boolean;
      status: "clean" | "dirty" | "saving" | "error";
      dirty: boolean;
      error: Error | null;
    };
    save: ReturnType<typeof vi.fn>;
  } = {
    current: { enabled: true, status: "clean", dirty: false, error: null },
    save: vi.fn(),
  };
  return value;
});

// The component is about presenting the sync state and routing the two ways
// of saving (button, shortcut) to one action; the hook that owns the state
// has its own tests.
vi.mock("@/model/use-project-sync", () => ({
  useProjectSync: (): ProjectSync => ({
    ...sync.current,
    save: sync.save as unknown as ProjectSync["save"],
  }),
}));

function setSync(patch: Partial<typeof sync.current>) {
  sync.current = { ...sync.current, ...patch };
}

function saveButton(): HTMLElement {
  return screen.getByRole("button", { name: m.action_save_to_project() });
}

function pressCtrlS(): KeyboardEvent {
  const event = new KeyboardEvent("keydown", {
    key: "s",
    ctrlKey: true,
    cancelable: true,
    bubbles: true,
  });
  window.dispatchEvent(event);
  return event;
}

beforeEach(() => {
  sync.current = { enabled: true, status: "clean", dirty: false, error: null };
  sync.save = vi.fn(() => Promise.resolve());
});

describe("SaveStatus", () => {
  it("renders nothing when a project save is not available", () => {
    setSync({ enabled: false, status: "dirty", dirty: true });
    const { container } = render(<SaveStatus />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the synced state when clean", () => {
    render(<SaveStatus />);
    expect(screen.getByText(m.status_saved_to_project())).toBeVisible();
    expect(
      screen.queryByRole("button", { name: m.action_save_to_project() }),
    ).toBeNull();
  });

  it("offers the save action when dirty and calls save on click", () => {
    setSync({ status: "dirty", dirty: true });
    render(<SaveStatus />);
    expect(screen.queryByText(m.status_saved_to_project())).toBeNull();
    fireEvent.click(saveButton());
    expect(sync.save).toHaveBeenCalledTimes(1);
  });

  it("disables the action and says so while saving", () => {
    setSync({ status: "saving", dirty: true });
    render(<SaveStatus />);
    const button = screen.getByRole("button", { name: m.status_saving() });
    expect(button).toBeDisabled();
  });

  it("reports a failure and keeps the action for a retry", () => {
    setSync({
      status: "error",
      dirty: true,
      error: new Error("Request failed: 500"),
    });
    render(<SaveStatus />);
    expect(screen.getByText(m.msg_save_failed())).toBeVisible();
    fireEvent.click(saveButton());
    expect(sync.save).toHaveBeenCalledTimes(1);
    // A plain failure carries no findings to show.
    expect(
      screen.queryByRole("button", { name: m.action_show_details() }),
    ).toBeNull();
  });

  it("lists the validation findings of a refused model", () => {
    setSync({
      status: "error",
      dirty: true,
      error: new APIRequestError({
        code: ERROR_CODE_MODEL_INVALID,
        message: "model validation failed",
        details: {
          errors: [
            {
              code: "missing_height",
              message: "building needs height_m > 0",
              feature_id: "b1",
            },
            { code: "no_receivers", message: "no receivers in model" },
          ],
        },
      }),
    });
    render(<SaveStatus />);
    fireEvent.click(
      screen.getByRole("button", { name: m.action_show_details() }),
    );

    const dialog = screen.getByRole("dialog", {
      name: m.dialog_title_model_invalid(),
    });
    expect(dialog).toHaveTextContent("missing_height");
    expect(dialog).toHaveTextContent("building needs height_m > 0");
    expect(dialog).toHaveTextContent("no receivers in model");
    const ids = screen.getAllByTestId("issue-feature-id");
    expect(ids.map((el) => el.textContent)).toEqual(["b1"]);
  });

  it("offers no details for a refusal that names no findings", () => {
    setSync({
      status: "error",
      dirty: true,
      error: new APIRequestError({
        code: ERROR_CODE_MODEL_INVALID,
        message: "model validation failed",
        details: { errors: [] },
      }),
    });
    render(<SaveStatus />);
    expect(screen.getByText(m.msg_save_failed())).toBeVisible();
    expect(
      screen.queryByRole("button", { name: m.action_show_details() }),
    ).toBeNull();
  });

  it("saves on Ctrl+S when dirty", () => {
    setSync({ status: "dirty", dirty: true });
    render(<SaveStatus />);
    const event = pressCtrlS();
    expect(event.defaultPrevented).toBe(true);
    expect(sync.save).toHaveBeenCalledTimes(1);
  });

  it("saves on Ctrl+S from inside a text field", () => {
    setSync({ status: "dirty", dirty: true });
    render(
      <>
        <SaveStatus />
        <input aria-label="height" />
      </>,
    );
    // Unlike undo, Ctrl+S is nobody's but ours: a user who has just typed a
    // value expects it to save, not to open the browser's save-page dialog.
    const event = new KeyboardEvent("keydown", {
      key: "s",
      ctrlKey: true,
      cancelable: true,
      bubbles: true,
    });
    screen.getByLabelText("height").dispatchEvent(event);
    expect(event.defaultPrevented).toBe(true);
    expect(sync.save).toHaveBeenCalledTimes(1);
  });

  it("retries on Ctrl+S after a failed save", () => {
    setSync({
      status: "error",
      dirty: true,
      error: new Error("Request failed: 500"),
    });
    render(<SaveStatus />);
    pressCtrlS();
    // The model is still unsaved in the error state; the shortcut must not
    // go dead just because the status word changed.
    expect(sync.save).toHaveBeenCalledTimes(1);
  });

  it("swallows Ctrl+S when clean without saving", () => {
    render(<SaveStatus />);
    const event = pressCtrlS();
    // The browser's save-page dialog must not appear in a workspace...
    expect(event.defaultPrevented).toBe(true);
    // ...but a clean model has nothing to send.
    expect(sync.save).not.toHaveBeenCalled();
  });

  it("does not save on Ctrl+S while a save is in flight", () => {
    setSync({ status: "saving", dirty: true });
    render(<SaveStatus />);
    pressCtrlS();
    expect(sync.save).not.toHaveBeenCalled();
  });

  it("leaves Ctrl+S to the browser when disabled", () => {
    setSync({ enabled: false });
    render(<SaveStatus />);
    const event = pressCtrlS();
    expect(event.defaultPrevented).toBe(false);
  });

  it("removes the shortcut listener on unmount", () => {
    setSync({ status: "dirty", dirty: true });
    const { unmount } = render(<SaveStatus />);
    unmount();
    pressCtrlS();
    expect(sync.save).not.toHaveBeenCalled();
  });
});
