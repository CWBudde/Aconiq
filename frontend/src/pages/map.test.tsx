import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { MemoryRouter, useLocation } from "react-router";
import type { Map } from "maplibre-gl";
import MapPage from "./map";
import { MapContext } from "@/map/use-map";
import { useModelStore } from "@/model/model-store";
import type { ModelFeature } from "@/model/types";
import type { TransformRequest, TransformResponse } from "@/wasm/types";
import { m } from "@/i18n/messages";

/**
 * The real `MapView` needs WebGL, but it is also what provides `MapContext` —
 * and a stub that dropped its children was half of why the dead draw wiring
 * went unnoticed. This one keeps both: it renders the children and provides
 * the context, so everything laid over the map is exercised.
 */
/** A stand-in for the MapLibre map; everything that touches it is stubbed. */
const stubMap = vi.hoisted(() => ({}) as Map);

vi.mock("@/map/map-view", () => ({
  MapView: ({
    children,
    onFeatureClick,
  }: {
    children?: React.ReactNode;
    onFeatureClick?: (features: { properties: unknown }[]) => void;
  }) => (
    // One stable object, because the real `MapView` holds the map in state:
    // a fresh one per render changes `useMap()`'s identity, which tears
    // terra-draw down and builds a new one on every keystroke.
    <MapContext value={stubMap}>
      <div data-testid="map-view">
        {/* A click on a model feature, which is the real `MapView`'s own
            `queryRenderedFeatures` call reduced to what it hands the page.
            The page treats a repeat click on the feature it already holds as a
            fresh selection, and nothing else here can produce one. */}
        <button
          type="button"
          onClick={() => {
            onFeatureClick?.([{ properties: { id: mapClick.featureId } }]);
          }}
        >
          click-feature
        </button>
        {children}
      </div>
    </MapContext>
  ),
}));
// Renders the id it was handed: the selection highlight is `ModelLayers`'s
// job, and the page's half of it is passing the edited feature down.
vi.mock("@/map/model-layers", () => ({
  ModelLayers: ({
    selectedFeatureId,
  }: {
    selectedFeatureId: string | null;
  }) => (
    <div data-testid="model-layers" data-selected={selectedFeatureId ?? ""} />
  ),
}));
vi.mock("@/map/layer-control", () => ({
  LayerControl: () => null,
}));
// Reads the run list, so it needs a QueryClient this page's tests do not set
// up. What it draws is pinned in `map/result-layers.test.tsx`; the page's own
// half is that it is mounted inside the map at all.
vi.mock("@/map/result-layers", () => ({
  ResultLayers: () => <div data-testid="result-layers" />,
}));
vi.mock("@/map/coordinate-display", () => ({
  CoordinateDisplay: () => null,
}));
vi.mock("@/map/draw-toolbar", () => ({
  DrawToolbar: ({
    activeMode,
    onModeChange,
    disabled,
    disabledReason,
    onCoordinateEntry,
  }: {
    activeMode: string;
    onModeChange: (mode: string) => void;
    disabled?: boolean;
    disabledReason?: string;
    onCoordinateEntry: () => void;
  }) => (
    <div
      data-testid="draw-toolbar"
      data-mode={activeMode}
      data-disabled={String(disabled === true)}
      data-disabled-reason={disabledReason ?? ""}
    >
      {/* The one prop that is a way *in* rather than a state to echo: the page
          has to open the dialog with no geometry when it fires. */}
      <button
        type="button"
        data-testid="coordinate-entry"
        onClick={onCoordinateEntry}
      >
        {m.action_enter_coordinates()}
      </button>
      {/* The real toolbar's select button, reduced to what a test needs of
          it: `map/draw-toolbar.test.tsx` owns how it looks and how it refuses.
          Without a way to arm select mode there is no way to reach the editing
          path from the page at all. */}
      <button
        type="button"
        onClick={() => {
          onModeChange("select");
        }}
      >
        arm-select
      </button>
    </div>
  ),
}));
// Echoes the selected id, and offers the one move the page cares about:
// reporting an id back, which is what a click on the canvas also does.
vi.mock("@/map/feature-list", () => ({
  FeatureList: ({
    selectedId,
    onSelect,
  }: {
    selectedId: string | null;
    onSelect: (id: string) => void;
  }) => (
    <div data-testid="feature-list" data-selected={selectedId ?? ""}>
      <button
        type="button"
        data-testid="feature-list-select"
        onClick={() => {
          onSelect("src-1");
        }}
      >
        src-1
      </button>
    </div>
  ),
}));
// Renders the id it was handed, which is what `?select=` has to reach.
vi.mock("@/map/feature-editor", () => ({
  FeatureEditor: ({ featureId }: { featureId: string | null }) =>
    featureId === null ? null : (
      <div data-testid="feature-editor">{featureId}</div>
    ),
}));
// Renders only when it is open, which is the signal that a drawn shape
// reached the page at all, and carries the geometry it was handed — the
// coordinates the store is about to be written with, which is the whole
// question on a metric model.
vi.mock("@/map/new-feature-dialog", () => ({
  NewFeatureDialog: ({
    open,
    geometry,
  }: {
    open: boolean;
    geometry: unknown;
  }) =>
    open ? (
      <div
        data-testid="new-feature-dialog"
        data-geometry={JSON.stringify(geometry)}
      />
    ) : null,
}));
vi.mock("@/map/validation-panel", () => ({
  ValidationPanel: () => null,
}));
vi.mock("@/map/undo-redo-bar", () => ({
  UndoRedoBar: () => null,
}));
// terra-draw is stubbed rather than `useDraw`, so the path from a control to
// the draw instance actually runs here. Mocking the hook is what hid the
// `MapContext` defect; `map/draw-provider.test.tsx` covers the wiring in
// detail.
vi.mock("terra-draw", () => {
  const mode = vi.fn();
  type Handler = (...args: never[]) => void;
  return {
    TerraDraw: class {
      // A real store, not a fixed snapshot. Editing puts the selected feature
      // in here, drags it and reads it back out, so the stub has to remember
      // what it was given — a snapshot that answered the same thing forever
      // could not tell a committed reshape from a dropped one.
      features: GeoJSON.Feature[] = [
        {
          id: "drawn-1",
          type: "Feature",
          properties: {},
          geometry: { type: "Polygon", coordinates: [DRAWN_RING] },
        },
      ];
      private handlers: Record<string, Handler[]> = {};
      setMode = vi.fn();
      start = vi.fn();
      stop = vi.fn();
      // Captured so a test can play terra-draw finishing a shape, or dragging
      // one. The adapter hands the page WGS84 coordinates whatever the model
      // is stored in, which is the whole reason both paths have an inverse
      // transform on them.
      on = vi.fn((event: string, handler: Handler) => {
        (this.handlers[event] ??= []).push(handler);
        if (event === "finish") draw.finish = handler as (id: string) => void;
      });
      getSnapshot = vi.fn(() => this.features);
      addFeatures = vi.fn((features: GeoJSON.Feature[]) => {
        this.features.push(...features);
        return features.map((feature) => ({ id: feature.id, valid: true }));
      });
      selectFeature = vi.fn();
      removeFeatures = vi.fn((ids: (string | number)[]) => {
        this.features = this.features.filter(
          (feature) => !ids.includes(feature.id as string),
        );
      });
      emit(event: string, ...args: unknown[]) {
        for (const handler of this.handlers[event] ?? []) {
          (handler as (...a: unknown[]) => void)(...args);
        }
      }
      constructor() {
        draw.instance = this as unknown as DrawStub;
      }
    },
    TerraDrawPointMode: mode,
    TerraDrawLineStringMode: mode,
    TerraDrawPolygonMode: mode,
    TerraDrawSelectMode: mode,
    TerraDrawRenderMode: mode,
  };
});
vi.mock("terra-draw-maplibre-gl-adapter", () => ({
  TerraDrawMapLibreGLAdapter: vi.fn(),
}));

/**
 * The one projection the page has, faked so the direction it is asked for is
 * visible. `map/draw-projection.parity.test.ts` runs the real kernel against
 * PROJ's own vectors — a fake answers in whichever direction it was told to, so
 * it can never catch a swapped source/target.
 */
const projection = vi.hoisted(() => {
  /** Multiplies by 100 000, so a projected coordinate is recognisable on sight. */
  const scaled = (req: TransformRequest): Promise<TransformResponse> =>
    Promise.resolve({
      source_crs: req.source_crs,
      target_crs: req.target_crs,
      applied: true,
      coordinates: req.coordinates.map((value) => value * 100000),
    });

  const value: {
    canReprojectForDisplay: boolean;
    requests: TransformRequest[];
    respond: (req: TransformRequest) => Promise<TransformResponse>;
    readonly scaled: (req: TransformRequest) => Promise<TransformResponse>;
  } = {
    canReprojectForDisplay: true,
    requests: [],
    respond: scaled,
    scaled,
  };
  return value;
});

vi.mock("@/api/backend", () => ({
  backend: {
    get capabilities() {
      return {
        kind: "browser",
        canExport: false,
        runsAgainstSavedModel: false,
        runsChangeExternally: false,
        exportsOutliveRunDelete: false,
        canReprojectForDisplay: projection.canReprojectForDisplay,
      };
    },
    transformCoordinates: (req: TransformRequest) => {
      projection.requests.push(req);
      return projection.respond(req);
    },
  },
}));

interface DrawStub {
  features: GeoJSON.Feature[];
  addFeatures: ReturnType<typeof vi.fn>;
  selectFeature: ReturnType<typeof vi.fn>;
  removeFeatures: ReturnType<typeof vi.fn>;
  /** Plays a terra-draw event, which is how a test stands in for the user. */
  emit: (event: string, ...args: unknown[]) => void;
}

/**
 * The handler `useDraw` registers on terra-draw's "finish" event, and the stub
 * instance itself — the editing path needs both directions.
 */
const draw: {
  finish: ((id: string) => void) | null;
  instance: DrawStub | null;
} = { finish: null, instance: null };

/** Which feature the `MapView` stub's click button reports. */
const mapClick = { featureId: "" };

/** A ring in WGS84, which is all terra-draw ever emits. */
const DRAWN_RING = [
  [10, 51],
  [10.1, 51],
  [10.1, 51.1],
  [10, 51],
];

describe("MapPage", () => {
  beforeEach(() => {
    draw.finish = null;
    draw.instance = null;
    useModelStore.getState().reset();
    projection.canReprojectForDisplay = true;
    projection.requests = [];
    projection.respond = projection.scaled;
  });

  function renderPage() {
    render(
      <MemoryRouter>
        <MapPage />
      </MemoryRouter>,
    );
  }

  function LocationProbe() {
    return <div data-testid="location-search">{useLocation().search}</div>;
  }

  function renderPageAt(entry: string) {
    render(
      <MemoryRouter initialEntries={[entry]}>
        <MapPage />
        <LocationProbe />
      </MemoryRouter>,
    );
  }

  const source: ModelFeature = {
    id: "src-1",
    kind: "source",
    sourceType: "point",
    geometry: { type: "Point", coordinates: [10, 51] },
  };

  it("mounts the map even with nothing in the model", () => {
    // Was: "shows the workspace start screen before geometry is loaded", which
    // asserted `map-view` was *absent*. The start panel replaced the canvas,
    // the toolbar and the validation panel, so the screen telling the user to
    // start a workspace was the one screen with no way to draw one.
    renderPage();

    expect(screen.getByTestId("map-view")).toBeInTheDocument();
    expect(screen.getByTestId("draw-toolbar")).toBeInTheDocument();
  });

  it("lays the start panel over the map as a labelled region", () => {
    renderPage();

    const panel = screen.getByRole("region", {
      name: m.heading_map_workspace(),
    });
    expect(panel).toBeVisible();
    expect(
      screen.getByRole("link", { name: m.action_import_data() }),
    ).toHaveAttribute("href", "/import");
  });

  it("arms point mode and clears the panel on Start drawing", () => {
    renderPage();

    fireEvent.click(
      screen.getByRole("button", { name: m.action_start_drawing() }),
    );

    expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
      "data-mode",
      "point",
    );
    expect(
      screen.queryByRole("region", { name: m.heading_map_workspace() }),
    ).toBeNull();
  });

  it("dismisses the panel without arming a tool on Close", () => {
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: m.action_close() }));

    expect(
      screen.queryByRole("region", { name: m.heading_map_workspace() }),
    ).toBeNull();
    expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
      "data-mode",
      "static",
    );
  });

  it("brings the panel back when the model empties again", () => {
    // The dismissal covers one empty-model episode, not the whole visit: a
    // user who draws a feature and then deletes the last one is back on an
    // empty map, which is the one state the hint exists for.
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: m.action_close() }));
    act(() => {
      useModelStore.getState().addFeature(source);
    });
    act(() => {
      useModelStore.getState().removeFeature(source.id);
    });

    expect(
      screen.getByRole("region", { name: m.heading_map_workspace() }),
    ).toBeVisible();
  });

  it("opens the editor on the feature the select parameter names", () => {
    // The link the import page's done step builds for a finding.
    useModelStore
      .getState()
      .loadModel({ features: [source], receivers: [], calcArea: null });
    renderPageAt("/model?select=src-1");

    expect(screen.getByTestId("feature-editor")).toHaveTextContent("src-1");
  });

  it("marks the selected feature on the map as well as in the editor", () => {
    // Before this, nothing on the canvas said which feature the panel was
    // editing: the editor opened over the layer control and the map looked
    // exactly as it had.
    useModelStore
      .getState()
      .loadModel({ features: [source], receivers: [], calcArea: null });
    renderPageAt("/model?select=src-1");

    expect(screen.getByTestId("model-layers")).toHaveAttribute(
      "data-selected",
      "src-1",
    );
  });

  it("strips the select parameter once it has been honoured", () => {
    // Otherwise a Back re-opens the editor on a feature the reader has moved
    // on from.
    useModelStore
      .getState()
      .loadModel({ features: [source], receivers: [], calcArea: null });
    renderPageAt("/model?select=src-1");

    expect(screen.getByTestId("location-search").textContent).toBe("");
  });

  it("does not show the panel when the model already has content", () => {
    useModelStore
      .getState()
      .loadModel({ features: [source], receivers: [], calcArea: null });
    renderPage();

    expect(
      screen.queryByRole("region", { name: m.heading_map_workspace() }),
    ).toBeNull();
    expect(screen.getByTestId("map-view")).toBeInTheDocument();
  });

  it("takes a drawn shape into the page for a model the map draws in", () => {
    // A store already in WGS84 needs no transform, and must not pay for one:
    // the dialog opens on the same tick the shape is finished, with the
    // coordinates terra-draw emitted.
    renderPageAt("/model?draw=1");

    expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
      "data-disabled",
      "false",
    );
    act(() => {
      draw.finish?.("drawn-1");
    });

    const dialog = screen.getByTestId("new-feature-dialog");
    expect(dialog).toBeInTheDocument();
    expect(JSON.parse(dialog.dataset["geometry"] ?? "null")).toEqual({
      type: "Polygon",
      coordinates: [DRAWN_RING],
    });
    expect(projection.requests).toEqual([]);
  });

  it("moves a drawn shape into the store's CRS before the page sees it", async () => {
    // terra-draw emits WGS84 whatever the store is in. Landing those degrees
    // in a model held in metres is the map quietly becoming the model, so the
    // finish path asks for the inverse of the display projection — WGS84 in,
    // the store's CRS out, and the target named explicitly rather than left to
    // `auto`, which would resolve a zone from the shape instead of honouring
    // the one the model is already in.
    useModelStore.getState().loadModel({
      features: [source],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    renderPageAt("/model?draw=1");

    expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
      "data-disabled",
      "false",
    );
    act(() => {
      draw.finish?.("drawn-1");
    });

    await waitFor(() => {
      expect(screen.getByTestId("new-feature-dialog")).toBeInTheDocument();
    });
    // Two requests, in this order. The page holds the map's one
    // `useDisplayModel` instance — `ModelLayers` is mocked out here, so this is
    // the page's own — which projects the workspace *out* of the store's CRS
    // for the map. The second is this test's subject: the same transform
    // inverted, carrying the finished shape back in.
    expect(projection.requests).toEqual([
      {
        source_crs: "EPSG:25832",
        target_crs: "EPSG:4326",
        coordinates: [10, 51],
      },
      {
        source_crs: "EPSG:4326",
        target_crs: "EPSG:25832",
        coordinates: DRAWN_RING.flat(),
      },
    ]);
    const dialog = screen.getByTestId("new-feature-dialog");
    expect(JSON.parse(dialog.dataset["geometry"] ?? "null")).toEqual({
      type: "Polygon",
      coordinates: [
        DRAWN_RING.map((position) => position.map((value) => value * 100000)),
      ],
    });
  });

  it("says the shape is being projected while the answer is out", async () => {
    // The finish path is synchronous and the transform is not. A shape that
    // simply disappeared for the length of a round trip is what the old CRS
    // gate was put there to avoid, so the wait is on screen.
    useModelStore.getState().loadModel({
      features: [source],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    let answer: ((response: TransformResponse) => void) | null = null;
    projection.respond = () =>
      new Promise<TransformResponse>((resolve) => {
        answer = resolve;
      });
    renderPageAt("/model?draw=1");

    act(() => {
      draw.finish?.("drawn-1");
    });

    expect(
      screen.getByText(m.msg_draw_projecting({ crs: "EPSG:25832" })),
    ).toBeVisible();
    expect(screen.queryByTestId("new-feature-dialog")).toBeNull();

    act(() => {
      answer?.({
        source_crs: "EPSG:4326",
        target_crs: "EPSG:25832",
        applied: true,
        coordinates: DRAWN_RING.flat().map((value) => value * 100000),
      });
    });

    await waitFor(() => {
      expect(screen.getByTestId("new-feature-dialog")).toBeInTheDocument();
    });
    expect(
      screen.queryByText(m.msg_draw_projecting({ crs: "EPSG:25832" })),
    ).toBeNull();
  });

  it("keeps a failed projection out of the model and says so", async () => {
    useModelStore.getState().loadModel({
      features: [source],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    const fixtureFeatures = useModelStore.getState().features;
    projection.respond = () => Promise.reject(new Error("zone out of range"));
    renderPageAt("/model?draw=1");

    act(() => {
      draw.finish?.("drawn-1");
    });

    await waitFor(() => {
      expect(
        screen.getByText(
          m.msg_draw_projection_failed({
            crs: "EPSG:25832",
            reason: "zone out of range",
          }),
        ),
      ).toBeVisible();
    });
    expect(screen.queryByTestId("new-feature-dialog")).toBeNull();
    expect(useModelStore.getState().calcArea).toBeNull();
    // Reference identity: a failure may not touch what the store already holds.
    expect(useModelStore.getState().features).toBe(fixtureFeatures);
    expect(useModelStore.getState().features[0]).toBe(fixtureFeatures[0]);
    expect(useModelStore.getState().crs).toBe("EPSG:25832");

    // The notice stays until it is read, and then goes.
    fireEvent.click(screen.getByRole("button", { name: m.action_close() }));
    expect(
      screen.queryByText(
        m.msg_draw_projection_failed({
          crs: "EPSG:25832",
          reason: "zone out of range",
        }),
      ),
    ).toBeNull();
  });

  it("refuses a finished shape where no projector is reachable", () => {
    // The gate is the capability, not the CRS: what cannot be done honestly is
    // landing a WGS84 shape in a metric model with nothing to move it.
    projection.canReprojectForDisplay = false;
    useModelStore.getState().loadModel({
      features: [source],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    const fixtureFeatures = useModelStore.getState().features;
    renderPageAt("/model?draw=1");

    const toolbar = screen.getByTestId("draw-toolbar");
    expect(toolbar).toHaveAttribute("data-disabled", "true");
    expect(toolbar.getAttribute("data-disabled-reason")).toContain(
      "EPSG:25832",
    );
    // `?draw=1` was a way past the disabled toolbar: it armed point mode, the
    // user drew a shape, and the finish handler dropped it without a word.
    expect(toolbar).toHaveAttribute("data-mode", "static");
    // Stripped even when refused, or a reload would re-ask.
    expect(screen.getByTestId("location-search").textContent).toBe("");

    act(() => {
      draw.finish?.("drawn-1");
    });

    expect(screen.queryByTestId("new-feature-dialog")).toBeNull();
    expect(projection.requests).toEqual([]);
    expect(useModelStore.getState().calcArea).toBeNull();
    // Reference identity: nothing on the map side may write a coordinate back.
    expect(useModelStore.getState().features).toBe(fixtureFeatures);
    expect(useModelStore.getState().features[0]).toBe(fixtureFeatures[0]);
    expect(useModelStore.getState().crs).toBe("EPSG:25832");
  });

  it("refuses drawing where the model's own CRS could not be projected", async () => {
    // `readCollectionCRS` takes any `EPSG:<n>` an import declares, while the
    // kernel supports a fixed set — so a model in EPSG:3035 reaches the store
    // and every transform of it fails. `canReprojectForDisplay` is global and
    // stays true, so it cannot see this; what can is the display projection
    // having already failed on the same transform the draw path would make.
    projection.respond = () =>
      Promise.reject(new Error("unsupported EPSG code 3035"));
    useModelStore.getState().loadModel({
      features: [source],
      receivers: [],
      calcArea: null,
      crs: "EPSG:3035",
    });
    renderPageAt("/model?draw=1");

    const toolbar = screen.getByTestId("draw-toolbar");
    await waitFor(() => {
      expect(toolbar).toHaveAttribute("data-disabled", "true");
    });
    expect(toolbar.getAttribute("data-disabled-reason")).toBe(
      m.msg_draw_disabled_crs_unsupported({ crs: "EPSG:3035" }),
    );
    // `?draw=1` armed point mode on mount, while the display projection was
    // still out and the gate still open. `DrawGuard` is what disarms a mode a
    // late refusal has invalidated, and this is the case it exists for — the
    // refusal here can only ever arrive after the tool is already armed.
    await waitFor(() => {
      expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
        "data-mode",
        "static",
      );
    });
  });

  it("holds the toolbar shut while a finished shape is still projecting", async () => {
    // Terra-draw has already taken the finished shape off the map, so a second
    // one accepted now would make the first stale and drop it with nothing
    // shown — the silent loss the whole projection path exists to avoid.
    let answer: ((response: TransformResponse) => void) | null = null;
    useModelStore.getState().loadModel({
      features: [],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    renderPageAt("/model?draw=1");

    await waitFor(() => {
      expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
        "data-disabled",
        "false",
      );
    });

    projection.respond = () =>
      new Promise<TransformResponse>((resolve) => {
        answer = resolve;
      });
    act(() => {
      draw.finish?.("drawn-1");
    });

    const toolbar = screen.getByTestId("draw-toolbar");
    await waitFor(() => {
      expect(toolbar).toHaveAttribute("data-disabled", "true");
    });
    expect(toolbar.getAttribute("data-disabled-reason")).toBe(
      m.msg_draw_disabled_projecting({ crs: "EPSG:25832" }),
    );

    await waitFor(() => {
      expect(answer).not.toBeNull();
    });
    act(() => {
      answer?.({
        source_crs: "EPSG:4326",
        target_crs: "EPSG:25832",
        applied: true,
        coordinates: DRAWN_RING.flat().map((v) => v * 100000),
      });
    });

    // And opens again once the shape has landed, rather than staying shut.
    await waitFor(() => {
      expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
        "data-disabled",
        "false",
      );
    });
  });

  it("disables Start drawing without a projector and says why", () => {
    // The other entry point the finish-time refusal does not cover. The panel
    // stays up on a refused `?draw=1` precisely because it is what carries the
    // reason — the toolbar can only say it in a tooltip.
    projection.canReprojectForDisplay = false;
    useModelStore.getState().loadModel({
      features: [],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    renderPageAt("/model?draw=1");

    expect(
      screen.getByRole("region", { name: m.heading_map_workspace() }),
    ).toBeVisible();
    expect(
      screen.getByRole("button", { name: m.action_start_drawing() }),
    ).toBeDisabled();
    // The toolbar's wording, not a second one invented for this surface.
    expect(
      screen.getByText(
        m.msg_draw_disabled_no_projection({ crs: "EPSG:25832" }),
      ),
    ).toBeVisible();
    expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
      "data-mode",
      "static",
    );
  });

  it("offers Start drawing over a metric model it can project", () => {
    useModelStore.getState().loadModel({
      features: [],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    renderPage();

    expect(
      screen.getByRole("button", { name: m.action_start_drawing() }),
    ).toBeEnabled();
  });

  it("opens the dialog with no geometry for the typed path", () => {
    // The keyboard way in. `geometry` stays null, which is how the dialog
    // tells the two paths apart — a placeholder shape here would be a
    // coordinate this page invented.
    renderPage();

    fireEvent.click(screen.getByTestId("coordinate-entry"));

    const dialog = screen.getByTestId("new-feature-dialog");
    expect(dialog).toBeInTheDocument();
    expect(JSON.parse(dialog.dataset["geometry"] ?? '"missing"')).toBeNull();
    // Nothing was projected, because nothing needed to be.
    expect(projection.requests).toEqual([]);
  });

  it("offers the typed path over a model no projector can reach", () => {
    // The state where every drawing tool is refused. Typed numbers are already
    // in the store's CRS, so the dialog still opens — and it is then the only
    // way to add a feature at all.
    projection.canReprojectForDisplay = false;
    useModelStore.getState().loadModel({
      features: [source],
      receivers: [],
      calcArea: null,
      crs: "EPSG:25832",
    });
    renderPage();

    expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
      "data-disabled",
      "true",
    );
    fireEvent.click(screen.getByTestId("coordinate-entry"));

    expect(screen.getByTestId("new-feature-dialog")).toBeInTheDocument();
    expect(projection.requests).toEqual([]);
  });

  it("opens the feature list from its toggle and closes it again", () => {
    useModelStore
      .getState()
      .loadModel({ features: [source], receivers: [], calcArea: null });
    renderPage();

    const toggle = screen.getByRole("button", {
      name: new RegExp(m.label_feature_list()),
    });
    expect(screen.queryByTestId("feature-list")).toBeNull();
    expect(toggle).toHaveAttribute("aria-pressed", "false");

    fireEvent.click(toggle);
    expect(screen.getByTestId("feature-list")).toBeInTheDocument();
    expect(toggle).toHaveAttribute("aria-pressed", "true");

    fireEvent.click(toggle);
    expect(screen.queryByTestId("feature-list")).toBeNull();
  });

  it("selecting from the list marks the feature and opens the editor", () => {
    // Exactly what a click on the canvas does — the list sets the same
    // `editingFeatureId`, so there is no second selection path to keep in step.
    useModelStore
      .getState()
      .loadModel({ features: [source], receivers: [], calcArea: null });
    renderPage();

    fireEvent.click(
      screen.getByRole("button", { name: new RegExp(m.label_feature_list()) }),
    );
    fireEvent.click(screen.getByTestId("feature-list-select"));

    expect(screen.getByTestId("feature-editor")).toHaveTextContent("src-1");
    expect(screen.getByTestId("model-layers")).toHaveAttribute(
      "data-selected",
      "src-1",
    );
    expect(screen.getByTestId("feature-list")).toHaveAttribute(
      "data-selected",
      "src-1",
    );
  });

  it("cancels the armed tool on Escape", () => {
    // Esc is the shape's way out. It goes through the same `cancel` the
    // toolbar's X calls, and it is armed only while a tool is: an always-on
    // binding calls `preventDefault` and would take Esc from every dialog on
    // the route.
    renderPage();

    fireEvent.click(
      screen.getByRole("button", { name: m.action_start_drawing() }),
    );
    expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
      "data-mode",
      "point",
    );

    fireEvent.keyDown(window, { key: "Escape" });

    expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
      "data-mode",
      "static",
    );
  });

  describe("geometry editing", () => {
    /** Arms select mode on the selected feature, the way the toolbar does. */
    function armSelect() {
      fireEvent.click(screen.getByRole("button", { name: "arm-select" }));
    }

    /** One pointer move: terra-draw's copy changes and it says so. */
    function dragTo(id: string, coordinates: [number, number]) {
      const instance = draw.instance;
      const held = instance?.features.find((f) => f.id === id);
      if (held) {
        held.geometry = { type: "Point", coordinates };
      }
      act(() => {
        instance?.emit("change", [id], "update");
      });
    }

    it("hands the selected feature to terra-draw and selects it", async () => {
      // Select mode was always built with draggable features and draggable,
      // deletable midpoints — nothing had ever called `addFeatures`, so it was
      // armed over an empty store and clicking a feature did nothing.
      useModelStore
        .getState()
        .loadModel({ features: [source], receivers: [], calcArea: null });
      renderPageAt("/model?select=src-1");

      armSelect();

      await waitFor(() => {
        expect(draw.instance?.addFeatures).toHaveBeenCalled();
      });
      expect(draw.instance?.addFeatures).toHaveBeenCalledWith([
        {
          type: "Feature",
          id: "src-1",
          // Terra-draw keys a feature to the mode that owns it and rejects one
          // naming a mode the instance does not hold.
          properties: { mode: "point" },
          geometry: { type: "Point", coordinates: [10, 51] },
        },
      ]);
      expect(draw.instance?.selectFeature).toHaveBeenCalledWith("src-1");
    });

    it("commits every change, and a whole drag is one undo step", async () => {
      // The store is in WGS84, so the inverse is the identity and a commit is
      // free: the model follows the pointer. What must not follow it is the
      // undo stack — each command carries `geometry:<id>` and the stack merges
      // the run.
      useModelStore
        .getState()
        .loadModel({ features: [source], receivers: [], calcArea: null });
      renderPageAt("/model?select=src-1");
      armSelect();
      await waitFor(() => {
        expect(draw.instance?.addFeatures).toHaveBeenCalled();
      });

      dragTo("src-1", [10.5, 51.5]);
      dragTo("src-1", [11, 52]);
      dragTo("src-1", [12, 53]);

      expect(useModelStore.getState().features[0]?.geometry).toEqual({
        type: "Point",
        coordinates: [12, 53],
      });
      // No transform was asked for on the way in, and none on the way out.
      expect(projection.requests).toEqual([]);

      act(() => {
        useModelStore.getState().undo();
      });
      // The geometry the drag started from, not the pointer move before its
      // end — the merge keeps the *oldest* undo.
      expect(useModelStore.getState().features[0]?.geometry).toEqual({
        type: "Point",
        coordinates: [10, 51],
      });
      expect(useModelStore.getState().canUndo).toBe(false);

      act(() => {
        useModelStore.getState().redo();
      });
      expect(useModelStore.getState().features[0]?.geometry).toEqual({
        type: "Point",
        coordinates: [12, 53],
      });
    });

    it("starts a new undo step for a second drag of the same feature", async () => {
      // The stack cannot see that a gesture ended: a second drag carries the
      // same key and looks like a continuation. `sealHistory` on deselect is
      // what separates them.
      useModelStore
        .getState()
        .loadModel({ features: [source], receivers: [], calcArea: null });
      renderPageAt("/model?select=src-1");
      armSelect();
      await waitFor(() => {
        expect(draw.instance?.addFeatures).toHaveBeenCalled();
      });

      dragTo("src-1", [10.5, 51.5]);
      act(() => {
        draw.instance?.emit("deselect", "src-1");
      });
      expect(draw.instance?.removeFeatures).toHaveBeenCalledWith(["src-1"]);

      // The feature is picked up again — which is what `selectionEpoch` is for:
      // the page already holds this id, so without it the click changes nothing
      // and the feature the user just clicked stays uneditable.
      mapClick.featureId = "src-1";
      fireEvent.click(screen.getByRole("button", { name: "click-feature" }));
      await waitFor(() => {
        expect(draw.instance?.addFeatures).toHaveBeenCalledTimes(2);
      });
      dragTo("src-1", [13, 54]);

      act(() => {
        useModelStore.getState().undo();
      });
      expect(useModelStore.getState().features[0]?.geometry).toEqual({
        type: "Point",
        coordinates: [10.5, 51.5],
      });
      act(() => {
        useModelStore.getState().undo();
      });
      expect(useModelStore.getState().features[0]?.geometry).toEqual({
        type: "Point",
        coordinates: [10, 51],
      });
    });

    it("projects a metric reshape once, when the gesture ends", async () => {
      // One `POST /api/v1/transform` per drag rather than one per pointer move.
      // The asymmetry with the synchronous path above is the point: the cheap
      // path gets live feedback, the expensive one gets a single round trip,
      // and both are one undo step.
      useModelStore.getState().loadModel({
        features: [source],
        receivers: [],
        calcArea: null,
        crs: "EPSG:25832",
      });
      renderPageAt("/model?select=src-1");
      await waitFor(() => {
        expect(projection.requests).toHaveLength(1);
      });
      armSelect();
      await waitFor(() => {
        expect(draw.instance?.addFeatures).toHaveBeenCalled();
      });
      // The shape terra-draw is given is the *display* geometry, so the handle
      // lands on the feature the user can see rather than a few hundred
      // kilometres off the coast of Africa.
      expect(draw.instance?.addFeatures).toHaveBeenCalledWith([
        {
          type: "Feature",
          id: "src-1",
          properties: { mode: "point" },
          geometry: { type: "Point", coordinates: [1000000, 5100000] },
        },
      ]);

      const beforeDrag = projection.requests.length;
      dragTo("src-1", [1, 2]);
      dragTo("src-1", [3, 4]);
      dragTo("src-1", [5, 6]);

      // Nothing has been asked of the kernel, and nothing has been written.
      expect(projection.requests).toHaveLength(beforeDrag);
      expect(useModelStore.getState().features[0]?.geometry).toEqual({
        type: "Point",
        coordinates: [10, 51],
      });

      act(() => {
        draw.instance?.emit("deselect", "src-1");
      });

      await waitFor(() => {
        expect(useModelStore.getState().features[0]?.geometry).toEqual({
          type: "Point",
          coordinates: [500000, 600000],
        });
      });
      // Exactly one inverse request across the whole gesture, carrying only the
      // last position, and with the target named rather than left to `auto`.
      // (The forward requests around it are `useDisplayModel` drawing the model
      // the commit has just changed.)
      expect(
        projection.requests.filter((req) => req.source_crs === "EPSG:4326"),
      ).toEqual([
        {
          source_crs: "EPSG:4326",
          target_crs: "EPSG:25832",
          coordinates: [5, 6],
        },
      ]);
    });

    it("refuses to arm editing on a model the map cannot project", async () => {
      // The same gate every way into an active tool takes. A reshape goes back
      // through the same inverse transform a drawn shape does, so refusing
      // afterwards would mean refusing a shape the user had already moved.
      projection.canReprojectForDisplay = false;
      useModelStore.getState().loadModel({
        features: [source],
        receivers: [],
        calcArea: null,
        crs: "EPSG:25832",
      });
      const fixtureFeatures = useModelStore.getState().features;
      renderPageAt("/model?select=src-1");

      armSelect();

      await waitFor(() => {
        expect(screen.getByTestId("feature-editor")).toHaveTextContent("src-1");
      });
      expect(draw.instance?.addFeatures).not.toHaveBeenCalled();
      // Reference identity: nothing on the map side may write a coordinate back.
      expect(useModelStore.getState().features).toBe(fixtureFeatures);
    });

    it("takes the feature back out when select mode is put away", async () => {
      useModelStore
        .getState()
        .loadModel({ features: [source], receivers: [], calcArea: null });
      renderPageAt("/model?select=src-1");
      armSelect();
      await waitFor(() => {
        expect(draw.instance?.addFeatures).toHaveBeenCalled();
      });

      fireEvent.keyDown(window, { key: "Escape" });

      await waitFor(() => {
        expect(draw.instance?.removeFeatures).toHaveBeenCalledWith(["src-1"]);
      });
      expect(screen.getByTestId("draw-toolbar")).toHaveAttribute(
        "data-mode",
        "static",
      );
    });

    it("moves a receiver through its own command", async () => {
      // Receivers are a separate store array, so a reshape of one is a separate
      // command — `updateFeatureGeometry` would not find it and would silently
      // do nothing.
      useModelStore.getState().loadModel({
        features: [],
        receivers: [
          {
            id: "rcv-1",
            heightM: 4,
            geometry: { type: "Point", coordinates: [10, 51] },
          },
        ],
        calcArea: null,
      });
      renderPageAt("/model?select=rcv-1");
      armSelect();
      await waitFor(() => {
        expect(draw.instance?.addFeatures).toHaveBeenCalled();
      });

      dragTo("rcv-1", [10.25, 51.25]);

      expect(useModelStore.getState().receivers[0]).toEqual({
        id: "rcv-1",
        heightM: 4,
        geometry: { type: "Point", coordinates: [10.25, 51.25] },
      });
    });
  });
});
