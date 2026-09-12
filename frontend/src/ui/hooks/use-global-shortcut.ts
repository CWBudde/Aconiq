import * as React from "react";

/**
 * A keyboard shortcut the whole window answers to, regardless of focus.
 *
 * Modifiers are matched exactly: `{ key: "z", ctrl: true }` fires for Ctrl+Z
 * and not for Ctrl+Shift+Z, so a redo binding cannot be shadowed by the undo
 * one. `ctrl` stands for Ctrl on Windows/Linux and Cmd on macOS.
 */
export interface GlobalShortcut {
  /** Compared case-insensitively against `KeyboardEvent.key`. */
  key: string;
  /** Ctrl or Cmd must be held. */
  ctrl?: boolean;
  shift?: boolean;
  alt?: boolean;
  /**
   * Also fire while a text-entry control has focus. Off by default: a
   * global Ctrl+Z must not steal the undo history of the input the user is
   * typing in. Turn it on for shortcuts no text control claims for itself,
   * such as Ctrl+S.
   */
  allowInTextEntry?: boolean;
  /** Detaches the listener entirely when false. Defaults to true. */
  enabled?: boolean;
}

/**
 * True when the event target is a control with its own undo history that the
 * browser already handles (text inputs, textareas, selects, contenteditable).
 * Global keyboard shortcuts must bow out for these.
 */
export function isTextEntryTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable) return true;
  const tag = target.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT";
}

function matches(event: KeyboardEvent, shortcut: GlobalShortcut): boolean {
  return (
    event.key.toLowerCase() === shortcut.key.toLowerCase() &&
    (event.ctrlKey || event.metaKey) === (shortcut.ctrl ?? false) &&
    event.shiftKey === (shortcut.shift ?? false) &&
    event.altKey === (shortcut.alt ?? false)
  );
}

/**
 * Attaches one `keydown` listener on `window` that calls `handler` when the
 * shortcut matches, after `preventDefault` so the browser's own binding for
 * the same keys (save page, bold, history navigation) never runs alongside.
 * The listener is removed on unmount and re-attached only when the shortcut
 * itself changes; the latest `handler` is always the one called.
 */
export function useGlobalShortcut(
  shortcut: GlobalShortcut,
  handler: (event: KeyboardEvent) => void,
): void {
  const {
    key,
    ctrl = false,
    shift = false,
    alt = false,
    allowInTextEntry = false,
    enabled = true,
  } = shortcut;

  // The handler closes over component state that changes on every render;
  // holding it in a ref keeps the listener stable without going stale.
  const handlerRef = React.useRef(handler);
  React.useEffect(() => {
    handlerRef.current = handler;
  });

  React.useEffect(() => {
    if (!enabled) return;
    const listener = (event: KeyboardEvent) => {
      if (!allowInTextEntry && isTextEntryTarget(event.target)) return;
      if (!matches(event, { key, ctrl, shift, alt })) return;
      event.preventDefault();
      handlerRef.current(event);
    };
    window.addEventListener("keydown", listener);
    return () => {
      window.removeEventListener("keydown", listener);
    };
  }, [key, ctrl, shift, alt, allowInTextEntry, enabled]);
}
