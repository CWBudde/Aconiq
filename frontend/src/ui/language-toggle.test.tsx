/**
 * The header's language menu, and the one thing it has to get right besides
 * switching: saying which language is in force.
 *
 * It used to read `getLocale()` during render, which is correct exactly once —
 * a reload followed every switch, so the component was always freshly mounted.
 * Without the reload a render-time read is a stale read, and the menu would
 * keep marking the language the user just left.
 */

import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { m } from "@/i18n/messages";
import { changeLocale, currentLocale } from "@/locale";
import { LanguageToggle } from "./language-toggle";

// Radix opens its menu on `pointerdown`, not on `click`, and jsdom dispatches
// neither from the other. The same shim the run and export page tests use for
// their selects.
function openMenu(): void {
  fireEvent.pointerDown(screen.getByRole("button", { name: m.language() }), {
    button: 0,
    ctrlKey: false,
    pointerType: "mouse",
  });
}

/** The menu item for a language, by the label the catalogue gives it. */
function item(label: string): HTMLElement {
  return screen.getByRole("menuitem", { name: label });
}

beforeEach(() => {
  changeLocale("en");
});

afterEach(() => {
  changeLocale("en");
});

describe("LanguageToggle", () => {
  it("marks the language in force", () => {
    render(<LanguageToggle />);
    openMenu();

    expect(item(m.language_en())).toHaveAttribute("data-active", "true");
    expect(item(m.language_de())).toHaveAttribute("data-active", "false");
  });

  it("switches, and moves the mark with the switch", () => {
    render(<LanguageToggle />);
    openMenu();

    fireEvent.click(item(m.language_de()));

    expect(currentLocale()).toBe("de");

    // Re-opened rather than asserted in place: the menu closes on selection,
    // and what matters is what it says the next time it is looked at.
    openMenu();
    expect(item(m.language_de())).toHaveAttribute("data-active", "true");
    expect(item(m.language_en())).toHaveAttribute("data-active", "false");
  });

  it("relabels its own trigger", () => {
    // The trigger's `aria-label` is a message like any other, so it is the
    // cheapest proof that this component re-rendered rather than merely
    // updating a data attribute.
    render(<LanguageToggle />);
    openMenu();
    fireEvent.click(item(m.language_de()));

    expect(
      screen.getByRole("button", { name: m.language({}, { locale: "de" }) }),
    ).toBeInTheDocument();
  });
});
