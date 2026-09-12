import { describe, expect, it, vi } from "vitest";
import { fireEvent } from "@testing-library/react";
import { renderHook } from "@testing-library/react";
import { isTextEntryTarget, useGlobalShortcut } from "./use-global-shortcut";
import type { GlobalShortcut } from "./use-global-shortcut";

function press(
  target: EventTarget,
  key: string,
  modifiers: Partial<
    Pick<KeyboardEventInit, "ctrlKey" | "metaKey" | "shiftKey" | "altKey">
  > = {},
): KeyboardEvent {
  const event = new KeyboardEvent("keydown", {
    key,
    bubbles: true,
    cancelable: true,
    ...modifiers,
  });
  target.dispatchEvent(event);
  return event;
}

function mount(shortcut: GlobalShortcut) {
  const handler = vi.fn();
  const view = renderHook(() => {
    useGlobalShortcut(shortcut, handler);
  });
  return { handler, ...view };
}

describe("isTextEntryTarget", () => {
  it("recognises the controls that own their undo history", () => {
    expect(isTextEntryTarget(document.createElement("input"))).toBe(true);
    expect(isTextEntryTarget(document.createElement("textarea"))).toBe(true);
    expect(isTextEntryTarget(document.createElement("select"))).toBe(true);
    expect(isTextEntryTarget(document.createElement("div"))).toBe(false);
    expect(isTextEntryTarget(null)).toBe(false);
  });

  it("recognises contenteditable elements", () => {
    const div = document.createElement("div");
    // jsdom does not compute `isContentEditable` from the attribute.
    Object.defineProperty(div, "isContentEditable", { value: true });
    expect(isTextEntryTarget(div)).toBe(true);
  });
});

describe("useGlobalShortcut", () => {
  it("fires on the matching key and prevents the browser default", () => {
    const { handler } = mount({ key: "s", ctrl: true });
    const event = press(window, "s", { ctrlKey: true });
    expect(handler).toHaveBeenCalledTimes(1);
    expect(handler).toHaveBeenCalledWith(event);
    expect(event.defaultPrevented).toBe(true);
  });

  it("accepts Cmd in place of Ctrl", () => {
    const { handler } = mount({ key: "s", ctrl: true });
    press(window, "s", { metaKey: true });
    expect(handler).toHaveBeenCalledTimes(1);
  });

  it("ignores the key's case", () => {
    const { handler } = mount({ key: "z", ctrl: true, shift: true });
    // Browsers report Shift+Z as "Z".
    press(window, "Z", { ctrlKey: true, shiftKey: true });
    expect(handler).toHaveBeenCalledTimes(1);
  });

  it("requires every declared modifier", () => {
    const { handler } = mount({ key: "s", ctrl: true });
    const event = press(window, "s");
    expect(handler).not.toHaveBeenCalled();
    expect(event.defaultPrevented).toBe(false);
  });

  it("rejects modifiers the shortcut does not declare", () => {
    const { handler } = mount({ key: "z", ctrl: true });
    press(window, "z", { ctrlKey: true, shiftKey: true });
    press(window, "z", { ctrlKey: true, altKey: true });
    expect(handler).not.toHaveBeenCalled();
  });

  it("ignores other keys", () => {
    const { handler } = mount({ key: "s", ctrl: true });
    press(window, "b", { ctrlKey: true });
    expect(handler).not.toHaveBeenCalled();
  });

  it.each(["input", "textarea"] as const)(
    "bows out while a %s has focus",
    (tag) => {
      const control = document.createElement(tag);
      document.body.appendChild(control);
      const { handler } = mount({ key: "z", ctrl: true });
      const event = press(control, "z", { ctrlKey: true });
      expect(handler).not.toHaveBeenCalled();
      expect(event.defaultPrevented).toBe(false);
      control.remove();
    },
  );

  it("bows out for contenteditable elements", () => {
    const editor = document.createElement("div");
    Object.defineProperty(editor, "isContentEditable", { value: true });
    document.body.appendChild(editor);
    const { handler } = mount({ key: "z", ctrl: true });
    press(editor, "z", { ctrlKey: true });
    expect(handler).not.toHaveBeenCalled();
    editor.remove();
  });

  it("fires inside a text control when allowInTextEntry is set", () => {
    const input = document.createElement("input");
    document.body.appendChild(input);
    const { handler } = mount({ key: "s", ctrl: true, allowInTextEntry: true });
    const event = press(input, "s", { ctrlKey: true });
    expect(handler).toHaveBeenCalledTimes(1);
    expect(event.defaultPrevented).toBe(true);
    input.remove();
  });

  it("does nothing while disabled", () => {
    const { handler } = mount({ key: "s", ctrl: true, enabled: false });
    const event = press(window, "s", { ctrlKey: true });
    expect(handler).not.toHaveBeenCalled();
    expect(event.defaultPrevented).toBe(false);
  });

  it("calls the latest handler without re-attaching the listener", () => {
    const addSpy = vi.spyOn(window, "addEventListener");
    const first = vi.fn();
    const second = vi.fn();
    const { rerender } = renderHook(
      ({ handler }: { handler: () => void }) => {
        useGlobalShortcut({ key: "s", ctrl: true }, handler);
      },
      { initialProps: { handler: first } },
    );
    const attached = addSpy.mock.calls.filter(([type]) => type === "keydown");
    rerender({ handler: second });
    expect(
      addSpy.mock.calls.filter(([type]) => type === "keydown"),
    ).toHaveLength(attached.length);
    fireEvent.keyDown(window, { key: "s", ctrlKey: true });
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);
    addSpy.mockRestore();
  });

  it("removes the listener on unmount", () => {
    const { handler, unmount } = mount({ key: "s", ctrl: true });
    unmount();
    const event = press(window, "s", { ctrlKey: true });
    expect(handler).not.toHaveBeenCalled();
    expect(event.defaultPrevented).toBe(false);
  });
});
