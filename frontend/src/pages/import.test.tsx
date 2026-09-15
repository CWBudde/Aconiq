import { m } from "@/i18n/messages";
import { beforeEach, describe, expect, it } from "vitest";
import {
  act,
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
import type { ModelFeature } from "@/model/types";

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

/** Already in the workspace when a test wants the import to have a choice. */
const existing: ModelFeature = {
  id: "placed-1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [9, 50] },
};

/** One source with no `source_type`: an error the report can name an id for. */
const invalidGeoJSON = JSON.stringify({
  type: "FeatureCollection",
  features: [
    {
      type: "Feature",
      properties: { id: "broken-1", kind: "source" },
      geometry: { type: "Point", coordinates: [10, 51] },
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

  /**
   * Imports into an empty workspace, where there is one button and no dialog.
   */
  function confirmImport() {
    fireEvent.click(screen.getByRole("button", { name: /import/i }));
  }

  /** Imports into a workspace that already holds something, by replacing it. */
  function replaceWorkspace() {
    fireEvent.click(
      screen.getByRole("button", { name: m.action_import_replace() }),
    );
    fireEvent.click(
      within(screen.getByRole("alertdialog")).getByRole("button", {
        name: /import/i,
      }),
    );
  }

  /** Puts one feature in the workspace, so the import has something to lose. */
  function seedWorkspace(feature: ModelFeature = existing) {
    act(() => {
      useModelStore.getState().addFeature(feature);
    });
  }

  it("refuses an empty file without showing the validator's English", async () => {
    // `validateProjectModel` answers an empty model with `model.empty`, whose
    // message is hardcoded English and whose own comment says it must never
    // reach the UI. This page calls the validator directly, so the refusal has
    // to happen above it.
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, {
      target: {
        files: [makeFile('{"type":"FeatureCollection","features":[]}')],
      },
    });

    await waitFor(() => {
      expect(screen.getByText(m.msg_import_nothing())).toBeInTheDocument();
    });
    expect(screen.queryByText(/Model contains no features/i)).toBeNull();
    // Still on the upload step, where another file can be chosen.
    expect(screen.getByText("Import GeoJSON")).toBeInTheDocument();
  });

  it("refuses a file whose every feature was skipped", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, {
      target: {
        files: [
          makeFile(
            JSON.stringify({
              type: "FeatureCollection",
              features: [
                {
                  type: "Feature",
                  properties: { kind: "terrain" },
                  geometry: { type: "Point", coordinates: [10, 51] },
                },
              ],
            }),
          ),
        ],
      },
    });

    await waitFor(() => {
      expect(screen.getByText(m.msg_import_nothing())).toBeInTheDocument();
    });
  });

  it("previews a file that holds only a calculation area", async () => {
    // Something to import, nothing for the validator to check: running it
    // would produce the same `model.empty` in English.
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, {
      target: {
        files: [
          makeFile(
            JSON.stringify({
              type: "FeatureCollection",
              features: [
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
            }),
          ),
        ],
      },
    });

    await waitFor(() => screen.getByText("Import Preview"));
    expect(screen.getByText(m.label_calc_area())).toBeInTheDocument();
    expect(screen.queryByText(/Model contains no features/i)).toBeNull();
  });

  it("asks before replacing the workspace, and replaces nothing until then", async () => {
    seedWorkspace();
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));

    fireEvent.click(
      screen.getByRole("button", { name: m.action_import_replace() }),
    );

    // `loadModel` replaces the model outright and resets the command stack
    // rather than extending it, so no undo covers it.
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    expect(useModelStore.getState().features).toEqual([existing]);
  });

  it("replaces everything the workspace held once confirmed", async () => {
    seedWorkspace();
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));

    replaceWorkspace();

    expect(useModelStore.getState().features).toHaveLength(2);
    expect(
      useModelStore.getState().features.some((f) => f.id === existing.id),
    ).toBe(false);
  });

  it("offers one button and no dialog when the workspace is empty", async () => {
    // Nothing to lose is nothing to choose between: Add and Replace would do
    // the same thing, so the reader is asked neither question.
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));

    expect(
      screen.queryByRole("button", { name: m.action_import_add() }),
    ).toBeNull();
    expect(
      screen.queryByRole("button", { name: m.action_import_replace() }),
    ).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: /import/i }));

    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(useModelStore.getState().features).toHaveLength(2);
  });

  it("adds to the workspace without asking, keeping what is there", async () => {
    seedWorkspace();
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));

    fireEvent.click(
      screen.getByRole("button", { name: m.action_import_add() }),
    );

    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(useModelStore.getState().features).toHaveLength(3);
    expect(useModelStore.getState().features[0]).toEqual(existing);
  });

  it("is one undo, however many features the add brought", async () => {
    seedWorkspace();
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));
    fireEvent.click(
      screen.getByRole("button", { name: m.action_import_add() }),
    );

    act(() => {
      useModelStore.getState().undo();
    });

    expect(useModelStore.getState().features).toEqual([existing]);
  });

  it("says what the add will skip before the reader commits to it", async () => {
    // The dialog must not be where the user first learns what will happen —
    // and Add has no dialog at all.
    seedWorkspace({ ...existing, id: "src-1" });
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(projectGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));

    expect(
      screen.getByText(m.msg_import_skips_existing({ count: 1 })),
    ).toBeInTheDocument();
  });

  it("reports what actually landed, not what the file held", async () => {
    seedWorkspace({ ...existing, id: "src-1" });
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(projectGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));
    fireEvent.click(
      screen.getByRole("button", { name: m.action_import_add() }),
    );

    // Two of the three arrived: `src-1` was already there.
    expect(
      screen.getByText(`1 ${m.msg_import_complete_description()}`),
    ).toBeInTheDocument();
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

  it("names the feature in a preview error without linking it", async () => {
    // Nothing to link to: the features are not in the store until the reader
    // chooses Add or Replace.
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(invalidGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));

    expect(screen.getByText("broken-1")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", {
        name: m.action_show_feature_on_map({ id: "broken-1" }),
      }),
    ).toBeNull();
  });

  it("links a surviving error to the feature it names", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(invalidGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));
    confirmImport();

    expect(
      screen.getByRole("link", {
        name: m.action_show_feature_on_map({ id: "broken-1" }),
      }),
    ).toHaveAttribute("href", "/model?select=broken-1");
  });

  it("does not link a finding whose feature the import skipped", async () => {
    // Add keeps what the workspace already holds, so the imported copy never
    // landed and the link would select the feature the reader already has.
    seedWorkspace({ ...existing, id: "broken-1" });
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(invalidGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));
    fireEvent.click(
      screen.getByRole("button", { name: m.action_import_add() }),
    );

    expect(
      screen.queryByRole("link", {
        name: m.action_show_feature_on_map({ id: "broken-1" }),
      }),
    ).toBeNull();
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
