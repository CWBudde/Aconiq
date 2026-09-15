import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ModeChip } from "./mode-chip";
import { m } from "@/i18n/messages";

const state = vi.hoisted(() => ({ browser: false }));

vi.mock("@/api/backend", () => ({
  backend: {
    get capabilities() {
      return {
        kind: state.browser ? "browser" : "http",
        canExport: state.browser,
        runsAgainstSavedModel: !state.browser,
        runsChangeExternally: !state.browser,
      };
    },
  },
}));

function renderChip(browser: boolean) {
  state.browser = browser;
  render(<ModeChip />);
  return screen.getByTestId("mode-chip");
}

beforeEach(() => {
  state.browser = false;
});

describe("ModeChip", () => {
  it("names the API backend", () => {
    const chip = renderChip(false);
    expect(chip).toHaveAttribute("data-mode", "http");
    expect(chip).toHaveTextContent(m.label_mode_api());
    expect(chip).toHaveTextContent(m.msg_runtime_api());
  });

  it("names the browser kernel", () => {
    const chip = renderChip(true);
    expect(chip).toHaveAttribute("data-mode", "browser");
    expect(chip).toHaveTextContent(m.label_mode_browser());
    expect(chip).toHaveTextContent(m.msg_runtime_wasm());
  });

  it("is decorative: no role, no tab stop, no prohibited label", () => {
    // `aria-label` on a role-less element is what axe's `aria-prohibited-attr`
    // flags, and a link here would add a header tab stop on every route for a
    // value that only changes when the app is restarted.
    const chip = renderChip(false);
    expect(chip).not.toHaveAttribute("aria-label");
    expect(chip).not.toHaveAttribute("role");
    expect(chip).not.toHaveAttribute("tabindex");
    expect(chip.querySelector("a, button")).toBeNull();
  });
});
