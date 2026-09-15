/**
 * The shell's content region, and how to put keyboard focus back into it.
 *
 * `AppShell` renders one element with this id and `tabIndex={-1}`: the skip
 * link's target, and the only always-mounted element in the app that can hold
 * focus without being a control.
 *
 * That makes it the answer to a problem `ConfirmDialog` cannot solve on its
 * own. Radix returns focus to whatever had it when the dialog opened, but a
 * confirmed destructive action often removes exactly that element — the banner
 * that held the Discard button, the editor panel that held Delete — and focus
 * falls to `<body>`, at the top of the document, with no announcement. Where
 * the action leaves a control that still makes sense to be on, focus that
 * instead; this is for the cases where the whole surface is gone and the
 * reader belongs back at the start of the content.
 *
 * No axe rule covers a lost focus target, and every DOM-shaped assertion
 * passes while it is wrong, so the call sites test it explicitly.
 */

export const MAIN_CONTENT_ID = "main-content";

/**
 * Focus the content region, if the shell is there to provide it.
 *
 * A no-op when it is absent, which is the case in a test that renders a page
 * without `AppShell` and in nothing else.
 */
export function focusMainContent(): void {
  document.getElementById(MAIN_CONTENT_ID)?.focus();
}
