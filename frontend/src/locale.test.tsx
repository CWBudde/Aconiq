/**
 * Switching language must not reload the page.
 *
 * Paraglide's `setLocale` reloads by default — `runtime.js` defaults
 * `reload: true` and ends in `window.location.reload()` under a non-URL
 * strategy — and both switchers called it bare. A reload re-downloads the
 * bundle, re-runs project hydration and throws away everything on screen, for
 * a preference change.
 *
 * What a locale change actually needs is for the messages to be read again:
 * the compiled accessors resolve the locale per call, so nothing is frozen.
 * `locale.ts` is the subscription that makes React ask.
 */

import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { m } from "@/i18n/messages";
import { overwriteSetLocale, setLocale } from "@/i18n/runtime";
import {
  changeLocale,
  currentLocale,
  syncDocumentLocale,
  useLocale,
} from "./locale";

/** Paraglide's own `setLocale`, to restore after a test overwrites it. */
const paraglideSetLocale = setLocale;

function Consumer() {
  // Deliberately reads a message *without* consuming the locale itself: this
  // is what every one of the seventy-eight `m`-importing modules looks like,
  // and it must follow the switch even though it subscribes to nothing.
  return <p data-testid="message">{m.language()}</p>;
}

function Subscriber() {
  const locale = useLocale();
  return (
    <div>
      <span data-testid="locale">{locale}</span>
      <Consumer />
    </div>
  );
}

beforeEach(() => {
  // Through `changeLocale`, not `setLocale`: the module caches the locale, and
  // resetting the runtime behind its back would leave the two disagreeing —
  // which is the very drift the last assertion below exists to catch.
  changeLocale("en");
  syncDocumentLocale();
});

afterEach(() => {
  overwriteSetLocale(paraglideSetLocale);
  changeLocale("en");
});

describe("changeLocale", () => {
  it("re-renders a subscriber and the messages under it", () => {
    render(<Subscriber />);
    expect(screen.getByTestId("message")).toHaveTextContent(
      m.language({}, { locale: "en" }),
    );

    act(() => {
      changeLocale("de");
    });

    expect(screen.getByTestId("locale")).toHaveTextContent("de");
    expect(screen.getByTestId("message")).toHaveTextContent(
      m.language({}, { locale: "de" }),
    );
  });

  it("asks paraglide not to reload", () => {
    // The absence of a reload cannot be observed from its effect: jsdom answers
    // `location.reload()` with a console "Not implemented" and no exception,
    // and its `location` is non-configurable, so it cannot be spied on either.
    // What can be observed is the argument, which is the contract.
    const calls: { locale: string; options: unknown }[] = [];
    overwriteSetLocale((locale, options) => {
      calls.push({ locale, options });
    });

    changeLocale("de");

    expect(calls).toEqual([{ locale: "de", options: { reload: false } }]);
  });

  it("moves <html lang> with the switch", () => {
    // The reload used to do this for free: `main.tsx` set `lang` once at
    // startup and every switch came back through startup. Without the reload
    // the document would keep announcing the old language to a screen reader,
    // and no axe rule fires on a `lang` that is merely wrong.
    render(<Subscriber />);

    act(() => {
      changeLocale("de");
    });

    expect(document.documentElement.lang).toBe("de");
  });

  it("is a no-op for the locale already in force", () => {
    // Not merely an optimisation: `App` keys the router on this value, so a
    // notification for a switch that did not happen remounts every route.
    const calls: string[] = [];
    overwriteSetLocale((locale) => {
      calls.push(locale);
    });

    changeLocale("en");

    expect(calls).toEqual([]);
    expect(currentLocale()).toBe("en");
  });

  it("agrees with paraglide about what the locale now is", () => {
    // `currentLocale()` is a cached snapshot, because `useSyncExternalStore`
    // compares snapshots by identity on every render and a getter delegating to
    // `getLocale()` would read `localStorage` in render. A snapshot that
    // drifted from the runtime would key the tree on one locale while the
    // messages rendered the other.
    changeLocale("de");

    expect(currentLocale()).toBe("de");
    expect(m.language()).toBe(m.language({}, { locale: "de" }));
  });
});
