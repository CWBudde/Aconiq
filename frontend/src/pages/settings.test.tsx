import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router";
import { API_BASE_URL_OVERRIDE_KEY } from "@/api/mode";
import { DRAFT_KEY } from "@/model/use-autosave";
import { ThemeProvider } from "@/ui/theme-provider";
import SettingsPage from "./settings";
import { m } from "@/i18n/messages";

// The runtime label comes from the selected backend; pin it rather than
// depending on the env the test runner happens to have.
vi.mock("@/api/backend", () => ({
  backend: {
    capabilities: {
      kind: "http",
      canExport: false,
      runsAgainstSavedModel: true,
      runsChangeExternally: true,
    },
  },
}));

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location-search">{location.search}</div>;
}

function renderPage(initialEntries: string[] = ["/settings"]) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <ThemeProvider>
        <LocationProbe />
        <SettingsPage />
      </ThemeProvider>
    </MemoryRouter>,
  );
}

beforeEach(() => {
  localStorage.clear();
  document.documentElement.classList.remove("light", "dark");
});

describe("SettingsPage", () => {
  it("renders the main preference sections", () => {
    renderPage();

    expect(
      screen.getByText(m.section_settings_categories()),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: m.theme_option_system() }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: m.language_en() }),
    ).toBeInTheDocument();
  });

  // The category strip is a Radix Tabs list, which activates a tab on pointer
  // down rather than on click; `userEvent` fires the full pointer sequence.
  it("switches to the Connection category", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(
      // The tab's accessible name is its title *and* its description, so this
      // matches on the title rather than the whole string.
      screen.getByRole("tab", {
        name: new RegExp(m.settings_category_advanced(), "i"),
      }),
    );

    expect(
      screen.getByRole("heading", { name: m.settings_category_advanced() }),
    ).toBeInTheDocument();
    // The id stays "advanced" while the label became "Connection": the id is
    // a URL, and renaming it would break a bookmark to buy a prettier one.
    expect(screen.getByTestId("location-search")).toHaveTextContent(
      "?category=advanced",
    );
  });

  it("restores the active category from the URL", () => {
    renderPage(["/settings?category=advanced"]);

    expect(
      screen.getByRole("heading", { name: m.settings_category_advanced() }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("location-search")).toHaveTextContent(
      "?category=advanced",
    );
  });

  it("falls back to General for a category that no longer exists", () => {
    // Five reserved categories were removed; `?category=results` is a live
    // bookmark for anyone who opened one.
    renderPage(["/settings?category=results"]);

    expect(
      screen.getByRole("heading", { name: m.settings_category_app() }),
    ).toBeInTheDocument();
  });

  it("saves and clears the advanced API endpoint override", async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(
      // The tab's accessible name is its title *and* its description, so this
      // matches on the title rather than the whole string.
      screen.getByRole("tab", {
        name: new RegExp(m.settings_category_advanced(), "i"),
      }),
    );

    const endpointInput = screen.getByLabelText(m.label_api_base_url());
    fireEvent.change(endpointInput, {
      target: { value: "https://example.com/" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: m.action_save_changes() }),
    );

    expect(localStorage.getItem(API_BASE_URL_OVERRIDE_KEY)).toBe(
      "https://example.com",
    );
    expect(endpointInput).toHaveValue("https://example.com");

    fireEvent.click(
      screen.getByRole("button", { name: m.action_reset_to_default() }),
    );

    expect(localStorage.getItem(API_BASE_URL_OVERRIDE_KEY)).toBeNull();
  });

  it("switches the stored theme preference", () => {
    renderPage();

    fireEvent.click(
      screen.getByRole("button", { name: m.theme_option_dark() }),
    );

    expect(localStorage.getItem("aconiq-theme")).toBe("dark");
  });

  it("clears a saved draft", () => {
    localStorage.setItem(
      DRAFT_KEY,
      JSON.stringify({ features: [], receivers: [] }),
    );

    renderPage();

    const clearButton = screen.getByRole("button", {
      name: m.action_discard(),
    });
    expect(clearButton).toBeEnabled();

    fireEvent.click(clearButton);

    expect(localStorage.getItem(DRAFT_KEY)).toBeNull();
  });
});

describe("SettingsPage heading order", () => {
  it.each(["app", "advanced"])(
    "keeps heading levels contiguous in the %s category",
    (category) => {
      // The shell's h1 sits above this page, so a level of 2 is the entry;
      // the sections once opened with h3 (axe `heading-order`).
      renderPage([`/settings?category=${category}`]);
      const levels = screen
        .getAllByRole("heading")
        .map((h) => Number(h.tagName.slice(1)));
      expect(levels[0]).toBe(2);
      let deepest = 1;
      for (const level of levels) {
        expect(level).toBeLessThanOrEqual(deepest + 1);
        deepest = Math.max(deepest, level);
      }
    },
  );
});
