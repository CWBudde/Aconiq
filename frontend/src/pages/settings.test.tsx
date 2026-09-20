import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router";
import { API_BASE_URL_OVERRIDE_KEY } from "@/api/mode";
import { DEFAULT_TILE_URL, TILE_URL_OVERRIDE_KEY } from "@/map/tile-source";
import { BASEMAP_STORAGE_KEY, basemapLabel } from "@/map/basemap";
import { LAYER_VISIBILITY_STORAGE_KEY } from "@/map/map-preferences";
import { useMapStore } from "@/map/map-store";
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
async function openCategory(title: string) {
  const user = userEvent.setup();
  renderPage();
  await user.click(screen.getByRole("tab", { name: new RegExp(title, "i") }));
}

const openConnection = () => openCategory(m.settings_category_advanced());
const openMap = () => openCategory(m.settings_category_map());

/**
 * A settings field, by the group it is labelled with. The API base URL on the
 * Connection tab and the tile URL on the Karte tab each carry a "Save changes"
 * and a "Reset to default" button, so every query for one has to say which
 * setting it means — as does a screen reader, which is why the groups are
 * there in the first place.
 */
function settingsField(label: string): HTMLElement {
  return screen.getByRole("group", { name: label });
}

beforeEach(() => {
  localStorage.clear();
  document.documentElement.classList.remove("light", "dark");
  // The basemap and the layer toggles live in a module-scoped store, so a
  // choice made in one test is still made in the next one unless it is undone.
  useMapStore.setState({ basemap: "light", layerVisibility: {} });
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

    const field = settingsField(m.label_api_base_url());
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
    await openMap();

    const field = settingsField(m.label_basemap_tile_url());
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
    await openMap();

    const field = settingsField(m.label_basemap_tile_url());
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

  it("opens the Karte category and puts it in the URL", async () => {
    await openMap();

    expect(
      screen.getByRole("heading", { name: m.settings_category_map() }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("location-search")).toHaveTextContent(
      "?category=map",
    );
  });

  it("restores the Karte category from the URL", () => {
    renderPage(["/settings?category=map"]);

    expect(
      screen.getByRole("heading", { name: m.settings_category_map() }),
    ).toBeInTheDocument();
  });

  it("switches the basemap, which is what the map reads when it is built", () => {
    // The page holds no map: the store is the whole commit, and it is the same
    // store the picker on the map writes to.
    renderPage(["/settings?category=map"]);

    const group = settingsField(m.section_basemap());
    const dark = within(group).getByRole("button", {
      name: basemapLabel("dark"),
    });
    expect(dark).toHaveAttribute("aria-pressed", "false");

    fireEvent.click(dark);

    expect(useMapStore.getState().basemap).toBe("dark");
    expect(localStorage.getItem(BASEMAP_STORAGE_KEY)).toBe("dark");
    expect(dark).toHaveAttribute("aria-pressed", "true");
    expect(
      within(group).getByRole("button", { name: basemapLabel("light") }),
    ).toHaveAttribute("aria-pressed", "false");
  });

  it("offers no way back once every layer is hidden, except this one", () => {
    // Hiding a layer group survives a reload now, so a user who hid one and
    // forgot has no refresh to fall back on. This button is the fallback.
    useMapStore.setState({ layerVisibility: { buildings: false } });
    renderPage(["/settings?category=map"]);

    // Singular, because exactly one group is hidden: German does not let a
    // count message get away with one form the way a bare number would.
    expect(
      screen.getByText(m.msg_settings_layers_hidden({ count: 1 })),
    ).toBeInTheDocument();
    expect(m.msg_settings_layers_hidden({ count: 1 })).not.toBe(
      m.msg_settings_layers_hidden({ count: 2 }),
    );

    const reset = screen.getByRole("button", { name: m.action_reset_layers() });
    expect(reset).toBeEnabled();

    fireEvent.click(reset);

    expect(useMapStore.getState().layerVisibility).toEqual({});
    expect(localStorage.getItem(LAYER_VISIBILITY_STORAGE_KEY)).toBeNull();
    expect(reset).toBeDisabled();
  });

  it("does not offer the reset when nothing is hidden", () => {
    renderPage(["/settings?category=map"]);

    expect(
      screen.getByRole("button", { name: m.action_reset_layers() }),
    ).toBeDisabled();
  });

  it("says when a tile URL change takes effect", () => {
    // A change here does nothing to a map that is already built, and the map
    // is on another route — so the copy has to say so.
    renderPage(["/settings?category=map"]);

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
  it.each(["app", "advanced", "map"])(
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
