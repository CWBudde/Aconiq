import { beforeEach, describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router";
import { API_BASE_URL_OVERRIDE_KEY } from "@/api/mode";
import { DRAFT_KEY } from "@/model/use-autosave";
import { ThemeProvider } from "@/ui/theme-provider";
import SettingsPage from "./settings";
import { m } from "@/i18n/messages";

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
      screen.getByRole("heading", { name: m.page_title_settings() }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(m.section_settings_categories()),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: m.theme_option_system() }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: m.language_en() }),
    ).toBeInTheDocument();
    expect(screen.getByText(m.msg_settings_local_only())).toBeInTheDocument();
  });

  it("switches to a planned settings category", () => {
    renderPage();

    fireEvent.click(screen.getByRole("tab", { name: /Project/i }));

    expect(
      screen.getByRole("heading", { name: m.settings_category_project() }),
    ).toBeInTheDocument();
    expect(screen.getAllByText(m.msg_settings_planned())).not.toHaveLength(0);
    expect(screen.getByTestId("location-search")).toHaveTextContent(
      "?category=project",
    );
  });

  it("restores the active category from the URL", () => {
    renderPage(["/settings?category=results"]);

    expect(
      screen.getByRole("heading", { name: m.settings_category_results() }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("location-search")).toHaveTextContent(
      "?category=results",
    );
  });

  it("saves and clears the advanced API endpoint override", () => {
    renderPage();

    fireEvent.click(screen.getByRole("tab", { name: /Advanced/i }));

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
