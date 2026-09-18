import { useSyncExternalStore } from "react";
import { getLocale, locales, setLocale } from "@/i18n/runtime";

export type Locale = (typeof locales)[number];

/**
 * The app's locale, as something React can subscribe to.
 *
 * Paraglide already resolves the locale on every `m.*()` call — the compiled
 * accessors read `getLocale()` inside the function body, and `getLocale()`
 * re-reads `localStorage` each time — so a locale change needs no new plumbing
 * to reach the messages. What it needs is a reason for React to call them
 * again, which is the whole of this module. `setLocale`'s default is to reload
 * the page instead, which is the plumbing being replaced.
 *
 * It lives beside `locale-parity.test.ts` rather than in `src/i18n/`, because
 * that directory is paraglide's output: gitignored, wiped and rewritten by
 * `compile:i18n`, and excluded from eslint. Hand-written code placed there
 * disappears on the next compile.
 *
 * The snapshot is cached rather than delegating to `getLocale()`, because
 * `useSyncExternalStore` compares snapshots by identity on every render and
 * calls the getter outside of a change as well; a getter that reads
 * `localStorage` would be doing storage IO in render.
 */
let current: Locale = getLocale();

const listeners = new Set<() => void>();

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** The locale in force, outside of a React render. */
export function currentLocale(): Locale {
  return current;
}

/** The locale in force, re-rendering the caller when it changes. */
export function useLocale(): Locale {
  return useSyncExternalStore(subscribe, currentLocale, currentLocale);
}

/**
 * Switch language without leaving the page.
 *
 * `reload: false` is the point. Beyond that the two things the reload used to
 * do for free have to be done here: `<html lang>` has to follow, because
 * `main.tsx` only ever set it at startup and every switch used to come back
 * through startup — and no axe rule fires on a `lang` that is merely wrong;
 * and the subscribers have to be told, because paraglide notifies nobody.
 *
 * Switching to the locale already in force returns without notifying. It is
 * not merely an optimisation: `App` keys the router on this value, so a
 * spurious notification would remount every route for nothing.
 */
export function changeLocale(next: Locale): void {
  if (next === current) {
    return;
  }

  void setLocale(next, { reload: false });
  current = next;
  syncDocumentLocale();

  for (const listener of listeners) {
    listener();
  }
}

/** Put the locale on `<html lang>`; `index.html` hardcodes `en`. */
export function syncDocumentLocale(): void {
  document.documentElement.lang = current;
}
