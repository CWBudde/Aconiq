import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router";
import { API_BASE_URL_OVERRIDE_KEY } from "@/api/mode";
import { DEFAULT_TILE_URL, TILE_URL_OVERRIDE_KEY } from "@/map/tile-source";
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

// The category strip is a Radix Tabs list, which activates a tab on pointer
// down rather than on click; `userEvent` fires the full pointer sequence. The
// tab's accessible name is its title *and* its description, so the match is on
// the title rather than the whole string.
async function openConnection() {
  const user = userEvent.setup();
  renderPage();
  await user.click(
    screen.getByRole("tab", {
      name: new RegExp(m.settings_category_advanced(), "i"),
    }),
  );
}

/**
 * One of the Connection card's two settings, by the group it is labelled with.
 * Both carry a "Save changes" and a "Reset to default" button, so every query
 * for one has to say which setting it means — as does a screen reader, which
 * is why the groups are there in the first place.
 */
function connectionField(label: string): HTMLElement {
  return screen.getByRole("group", { name: label });
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

  it("switches to the Connection category", async () => {
    await openConnection();

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
    await openConnection();

    const field = connectionField(m.label_api_base_url());
    const endpointInput = within(field).getByLabelText(m.label_api_base_url());
    fireEvent.change(endpointInput, {
      target: { value: "https://example.com/" },
    });
    fireEvent.click(
      within(field).getByRole("button", { name: m.action_save_changes() }),
    );

    expect(localStorage.getItem(API_BASE_URL_OVERRIDE_KEY)).toBe(
      "https://example.com",
    );
    expect(endpointInput).toHaveValue("https://example.com");

    fireEvent.click(
      within(field).getByRole("button", { name: m.action_reset_to_default() }),
    );

    expect(localStorage.getItem(API_BASE_URL_OVERRIDE_KEY)).toBeNull();
  });

  it("saves and clears the basemap tile URL override", async () => {
    // The map is built from this key, not from a prop: nothing on this page
    // holds a map, so committing the setting *is* writing the key.
    await openConnection();

    const field = connectionField(m.label_basemap_tile_url());
    const tileInput = within(field).getByLabelText(m.label_basemap_tile_url());
    expect(tileInput).toHaveValue(DEFAULT_TILE_URL);

    fireEvent.change(tileInput, {
      target: { value: "  https://tiles.internal/{z}/{x}/{y}.png  " },
    });
    fireEvent.click(
      within(field).getByRole("button", { name: m.action_save_changes() }),
    );

    expect(localStorage.getItem(TILE_URL_OVERRIDE_KEY)).toBe(
      "https://tiles.internal/{z}/{x}/{y}.png",
    );
    // Saving commits the draft: the input shows what was stored, trimmed.
    expect(tileInput).toHaveValue("https://tiles.internal/{z}/{x}/{y}.png");

    fireEvent.click(
      within(field).getByRole("button", { name: m.action_reset_to_default() }),
    );

    expect(localStorage.getItem(TILE_URL_OVERRIDE_KEY)).toBeNull();
    expect(tileInput).toHaveValue(DEFAULT_TILE_URL);
  });

  it("holds the tile URL as a draft until it is saved", async () => {
    // The draft/committed split: typing must not reach storage, or a half-typed
    // host would be what the next map build asks for tiles.
    await openConnection();

    const field = connectionField(m.label_basemap_tile_url());
    const save = within(field).getByRole("button", {
      name: m.action_save_changes(),
    });
    const reset = within(field).getByRole("button", {
      name: m.action_reset_to_default(),
    });
    expect(save).toBeDisabled();
    expect(reset).toBeDisabled();

    fireEvent.change(within(field).getByLabelText(m.label_basemap_tile_url()), {
      target: { value: "https://tiles.internal/{z}" },
    });

    expect(localStorage.getItem(TILE_URL_OVERRIDE_KEY)).toBeNull();
    expect(save).toBeEnabled();
    expect(reset).toBeDisabled();
  });

  it("says when a tile URL change takes effect", () => {
    // A change here does nothing to a map that is already built, and the map
    // is on another route — so the copy has to say so.
    renderPage(["/settings?category=advanced"]);

    expect(screen.getByText(m.msg_basemap_tile_url_note())).toBeInTheDocument();
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
