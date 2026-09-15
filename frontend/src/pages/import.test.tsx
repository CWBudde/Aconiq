import { m } from "@/i18n/messages";
import { beforeEach, describe, expect, it } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import ImportPage from "./import";
import { useModelStore } from "@/model/model-store";

const validGeoJSON = JSON.stringify({
  type: "FeatureCollection",
  features: [
    {
      type: "Feature",
      properties: { kind: "source", source_type: "point" },
      geometry: { type: "Point", coordinates: [10, 51] },
    },
    {
      type: "Feature",
      properties: { kind: "building", height_m: 10 },
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [0, 0],
            [1, 0],
            [1, 1],
            [0, 1],
            [0, 0],
          ],
        ],
      },
    },
  ],
});

/** What `aconiq import` writes: features, receivers and the calculation area. */
const projectGeoJSON = JSON.stringify({
  type: "FeatureCollection",
  features: [
    {
      type: "Feature",
      properties: { id: "src-1", kind: "source", source_type: "point" },
      geometry: { type: "Point", coordinates: [10, 51] },
    },
    {
      type: "Feature",
      properties: { id: "rcv-1", kind: "receiver", height_m: 4 },
      geometry: { type: "Point", coordinates: [10.001, 51.001] },
    },
    {
      type: "Feature",
      properties: { id: "area-1", kind: "calc-area" },
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [0, 0],
            [2, 0],
            [2, 2],
            [0, 0],
          ],
        ],
      },
    },
  ],
});

function makeFile(content: string, name = "model.geojson"): File {
  return new File([content], name, { type: "application/json" });
}

function getFileInput(): HTMLInputElement {
  const input = document.querySelector<HTMLInputElement>('input[type="file"]');
  if (!input) {
    throw new Error("file input not found");
  }
  return input;
}

function renderImportPage() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
      mutations: {
        retry: false,
      },
    },
  });
  const router = createMemoryRouter(
    [
      { path: "/import", element: <ImportPage /> },
      { path: "/model", element: <div>Map page</div> },
    ],
    { initialEntries: ["/import"] },
  );
  return render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useModelStore.getState().reset();
});

describe("ImportPage", () => {
  it("renders the upload step initially", () => {
    renderImportPage();
    expect(screen.getByText("Import GeoJSON")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /choose file/i }),
    ).toBeInTheDocument();
  });

  it("shows preview after a valid file is selected", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => {
      expect(screen.getByText("Import Preview")).toBeInTheDocument();
    });
    expect(screen.getByText(/2 features normalized/i)).toBeInTheDocument();
  });

  it("shows an error for invalid JSON", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile("not json")] } });
    await waitFor(() => {
      expect(screen.getByText(/failed to parse/i)).toBeInTheDocument();
    });
  });

  it("shows an error for a non-FeatureCollection JSON", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, {
      target: { files: [makeFile('{"type":"Feature"}')] },
    });
    await waitFor(() => {
      expect(
        screen.getByText(/must be a GeoJSON FeatureCollection/i),
      ).toBeInTheDocument();
    });
  });

  /** Clicks Import and answers the replace confirmation. */
  function confirmImport() {
    fireEvent.click(screen.getByRole("button", { name: /import/i }));
    fireEvent.click(
      within(screen.getByRole("alertdialog")).getByRole("button", {
        name: /import/i,
      }),
    );
  }

  it("asks before replacing the workspace, and replaces nothing until then", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));

    fireEvent.click(screen.getByRole("button", { name: /import/i }));

    // `loadFeatures` replaces the model outright and clears placed receivers,
    // and the command stack is reset rather than extended, so no undo covers
    // it.
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(useModelStore.getState().features).toHaveLength(0);
  });

  it("loads features into the model store on confirm", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));
    confirmImport();
    expect(useModelStore.getState().features).toHaveLength(2);
  });

  it("moves focus to the done step's action, not to the body", async () => {
    // Confirming removes the Import button the dialog was opened from, so
    // Radix restores focus to an element that no longer exists and it falls to
    // `<body>`. Nothing in the axe baseline covers a lost focus target.
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));
    confirmImport();

    await waitFor(() => {
      expect(document.activeElement).toBe(
        screen.getByRole("button", { name: m.action_go_to_map() }),
      );
    });
  });

  it("shows done step after confirm", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));
    confirmImport();
    expect(screen.getByText("Import Complete")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /go to map/i }),
    ).toBeInTheDocument();
  });

  it("keeps the receivers and the calculation area an import brings", async () => {
    // The wizard used to normalize with `normalizeGeoJSON`, which reported
    // both kinds as an unknown kind, and to load with `loadFeatures`, which
    // cleared the receivers the store held. "Import, then Save to project"
    // therefore wrote a model without the receivers `aconiq import` had put
    // there.
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(projectGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));
    confirmImport();

    const state = useModelStore.getState();
    expect(state.features.map((f) => f.id)).toEqual(["src-1"]);
    expect(state.receivers.map((r) => r.id)).toEqual(["rcv-1"]);
    expect(state.calcArea).not.toBeNull();
  });

  it("counts the receivers and the calculation area in the preview", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(projectGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));

    const receivers = screen.getByText(m.label_receivers());
    expect(receivers.nextElementSibling).toHaveTextContent("1");
    expect(screen.getByText(m.label_calc_area())).toBeInTheDocument();
    // Nothing was skipped: both kinds are part of the schema the wizard reads.
    expect(screen.queryByText(/features skipped/i)).toBeNull();
  });

  it("goes back to upload step from preview", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));
    fireEvent.click(screen.getByRole("button", { name: /back/i }));
    expect(screen.getByText("Import GeoJSON")).toBeInTheDocument();
  });

  /**
   * Radix' tabs activate on pointer-down plus focus, not on a synthetic click,
   * so the switch goes through `user-event` the way `ui/components/tabs.test`
   * does.
   */
  async function openTab(name: string): Promise<void> {
    await userEvent.setup().click(screen.getByRole("tab", { name }));
  }

  it("keeps the bounding box across a tab switch", async () => {
    // Radix unmounts the inactive panel, so a box held inside `OsmImport`
    // would be discarded the moment the reader glanced at the file tab — and
    // "Use current location" would have to be answered again. The page holds
    // it, as it did before the split.
    renderImportPage();
    await openTab(m.action_import_from_osm());
    fireEvent.change(screen.getByLabelText(m.label_south()), {
      target: { value: "52.49" },
    });

    await openTab(m.action_import_from_file());
    await openTab(m.action_import_from_osm());

    expect(screen.getByLabelText<HTMLInputElement>(m.label_south()).value).toBe(
      "52.49",
    );
  });
});
