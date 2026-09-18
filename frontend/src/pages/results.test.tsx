import {
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import type {
  ArtifactRef,
  RasterMetadata,
  ReceiverTable,
  RunSummary,
} from "@/api/client";
import ResultsPage from "./results";
import { useModelStore } from "@/model/model-store";
import { resetProjectSyncStore } from "@/model/use-project-sync";
import { m } from "@/i18n/messages";
import { unitFor } from "@/map/result-units";

/**
 * Mostly characterisation tests: they describe what the results page does
 * today, not what it ought to do. They exist as a safety net under a refactor,
 * so where the current behaviour is wrong the wrong behaviour is pinned
 * deliberately and marked as such.
 *
 * The selection is the exception. It used to fall back to the first completed
 * run and the tests pinned that; it is now the `:runId` segment of the URL and
 * these assertions are specification, not description.
 *
 * What this file does *not* own is the CSV spelling. The download block below
 * asserts page behaviour — which table is handed over, what blob comes back,
 * what it is named — while the bytes themselves belong to
 * `model/receiver-csv.ts` and are tested in `model/receiver-csv.test.ts` and
 * `model/receiver-csv.parity.test.ts`.
 */

const state = vi.hoisted(() => {
  const value: {
    /** Descriptors for `useStandardLabel`; empty means no tier qualifier. */
    standards: { id: string; evidence_tier?: string }[];
    runs: unknown[];
    runsLoading: boolean;
    runsError: Error | null;
    receiverTable: unknown;
    receiverTableLoading: boolean;
    receiverTableError: Error | null;
    receiverTableArtifactIds: (string | null)[];
    rasterMetadata: unknown;
    rasterMetadataLoading: boolean;
    rasterMetadataError: Error | null;
    canExport: boolean;
  } = {
    standards: [],
    runs: [],
    runsLoading: false,
    runsError: null,
    receiverTable: null,
    receiverTableLoading: false,
    receiverTableError: null,
    receiverTableArtifactIds: [],
    rasterMetadata: null,
    rasterMetadataLoading: false,
    rasterMetadataError: null,
    canExport: true,
  };
  return value;
});

// Explicit capabilities rather than whatever the env selects. The results page
// reads none of them directly, but `useProjectSync` — deliberately left
// unmocked below — does, and a getter keeps a flag flippable mid-file.
vi.mock("@/api/backend", () => ({
  backend: {
    get capabilities() {
      return {
        kind: "http",
        canExport: state.canExport,
        runsAgainstSavedModel: true,
        runsChangeExternally: true,
      };
    },
  },
}));

vi.mock("@/api/hooks", () => ({
  // Only the names this page reaches for: the factory replaces the module
  // outright, so one it omits reads as `undefined` at render time rather
  // than as a type error. `useStandardLabel` calls this one.
  useStandards: () => ({
    data: state.standards,
    isLoading: false,
    error: null,
  }),
  useRuns: () => ({
    data: state.runs,
    isLoading: state.runsLoading,
    error: state.runsError,
  }),
  useReceiverTable: (artifactId: string | null) => {
    state.receiverTableArtifactIds.push(artifactId);
    return {
      data: state.receiverTable,
      isLoading: state.receiverTableLoading,
      error: state.receiverTableError,
    };
  },
  useRasterMetadata: () => ({
    data: state.rasterMetadata,
    isLoading: state.rasterMetadataLoading,
    error: state.rasterMetadataError,
  }),
  // Not used by this page; `useProjectSync` imports them and is not mocked.
  useProjectStatus: () => ({
    data: { name: "Demo", crs: "EPSG:25832", scenario_count: 1, run_count: 1 },
    isLoading: false,
    isError: false,
    error: null,
  }),
  useSaveModel: () => ({
    mutateAsync: () => Promise.resolve({ featureCount: 0, warnings: [] }),
  }),
  useIsSavingModel: () => false,
}));

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const receiverArtifact: ArtifactRef = {
  id: "art-receivers",
  kind: "run.result.receiver_table_json",
  path: "runs/run-1/results/receivers.json",
  created_at: "2026-01-01T10:00:05Z",
};

const rasterArtifact: ArtifactRef = {
  id: "art-raster",
  kind: "run.result.raster_metadata",
  path: "runs/run-1/results/grid.meta.json",
  created_at: "2026-01-01T10:00:05Z",
};

function run(id: string, overrides: Partial<RunSummary> = {}): RunSummary {
  return {
    id,
    scenario_id: "default",
    standard_id: "rls19-road",
    version: "2019",
    profile: "default",
    status: "completed",
    started_at: "2026-01-01T10:00:00Z",
    finished_at: "2026-01-01T10:00:05Z",
    log_path: `runs/${id}/run.log`,
    artifacts: [receiverArtifact],
    ...overrides,
  };
}

/**
 * Three receivers, two indicators, and `R10` deliberately missing `Lnight`:
 * that gap is what pins the `?? 0` fallback in both the table and the sort.
 * The ids are chosen so lexicographic order (R1, R10, R2) differs from the
 * numeric order a reader might expect — the page sorts strings here.
 */
const table: ReceiverTable = {
  indicator_order: ["Lden", "Lnight"],
  units: { Lden: "dB(A)", Lnight: "dB(A)" },
  records: [
    {
      id: "R1",
      x: 100.5,
      y: 200.25,
      height_m: 4,
      values: { Lden: 62.4, Lnight: 55.1 },
    },
    {
      id: "R2",
      x: 50.5,
      y: 400.25,
      height_m: 2.5,
      values: { Lden: 58.2, Lnight: 51.9 },
    },
    { id: "R10", x: 300, y: 100, height_m: 8, values: { Lden: 71.8 } },
  ],
};

const rasterMetadata: RasterMetadata = {
  width: 120,
  height: 80,
  bands: 2,
  nodata: -9999,
  units: { Lden: "dB(A)", Lnight: "dB(A)" },
  band_names: ["Lden", "Lnight"],
};

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

/** Blobs handed to `URL.createObjectURL`, newest last. */
const createdBlobs: Blob[] = [];
/** `download` attribute of every anchor the page clicked. */
const downloadNames: string[] = [];
/** Held locally so the assertion never reaches for an unbound method. */
const revokeObjectURL = vi.fn();

beforeAll(() => {
  window.HTMLElement.prototype.scrollIntoView = vi.fn();
  window.HTMLElement.prototype.hasPointerCapture = vi.fn(() => false);
  window.HTMLElement.prototype.releasePointerCapture = vi.fn();
  globalThis.ResizeObserver = class {
    observe() {
      /* not measured in jsdom */
    }
    unobserve() {
      /* not measured in jsdom */
    }
    disconnect() {
      /* not measured in jsdom */
    }
  };

  // jsdom implements neither object URLs nor anchor-triggered downloads, so
  // the CSV is captured where it is handed over rather than where it lands.
  URL.createObjectURL = vi.fn((blob: Blob | MediaSource) => {
    createdBlobs.push(blob as Blob);
    return "blob:aconiq/receivers.csv";
  });
  URL.revokeObjectURL = revokeObjectURL;
  window.HTMLAnchorElement.prototype.click = function (
    this: HTMLAnchorElement,
  ) {
    downloadNames.push(this.download);
  };
});

/**
 * The height the stubbed viewport reports, and the row height the table lays
 * out with. `VIEWPORT_PX / ROW_PX` is how many rows fit, which is what the
 * virtualizer mounts plus its overscan.
 */
const VIEWPORT_PX = 640;
const ROW_PX = 29;

beforeEach(() => {
  /*
   * jsdom performs no layout, so every element measures 0×0. The virtualizer
   * reads its scroll element with `offsetWidth`/`offsetHeight` — not
   * `getBoundingClientRect` — and a viewport 0px tall holds no rows, so an
   * unaided test would render nothing but the overscan and quietly stop
   * addressing real rows. Giving the elements a height is what keeps the
   * assertions below about the table rather than about jsdom.
   *
   * `scrollTop` needs no stub: jsdom stores what is assigned to it, so a test
   * can set it and fire a scroll event the way a browser would.
   */
  vi.spyOn(window.HTMLElement.prototype, "offsetHeight", "get").mockReturnValue(
    VIEWPORT_PX,
  );
  vi.spyOn(window.HTMLElement.prototype, "offsetWidth", "get").mockReturnValue(
    800,
  );

  state.runs = [];
  state.runsLoading = false;
  state.runsError = null;
  state.receiverTable = table;
  state.receiverTableLoading = false;
  state.receiverTableError = null;
  state.receiverTableArtifactIds = [];
  state.rasterMetadata = rasterMetadata;
  state.rasterMetadataLoading = false;
  state.rasterMetadataError = null;
  state.canExport = true;
  createdBlobs.length = 0;
  downloadNames.length = 0;
  revokeObjectURL.mockClear();
  useModelStore.getState().reset();
  resetProjectSyncStore();
});

// The viewport stub is a prototype spy; without this it would nest once per
// test rather than being replaced.
afterEach(() => {
  vi.restoreAllMocks();
});

/** Reads the current path out, so a click's navigation is asserted directly. */
function PathProbe() {
  return <span data-testid="pathname">{useLocation().pathname}</span>;
}

function pathname(): string {
  return screen.getByTestId("pathname").textContent;
}

/**
 * Renders the page under the same index/`:runId` pair `routes.tsx` registers,
 * so `useParams` sees what it sees in the app.
 *
 * `path` defaults to the first completed run's own URL, because that is what
 * "a run is open" means now and most of this file is about the detail pane.
 * The tests that are about the selection itself pass a path explicitly — the
 * bare `/results` for "nothing selected", a bogus id for the unknown case.
 */
function renderResults(runs: RunSummary[] = [run("run-1")], path?: string) {
  state.runs = runs;
  const firstCompleted = runs.find((r) => r.status === "completed");
  const entry =
    path ?? (firstCompleted ? `/results/${firstCompleted.id}` : "/results");
  render(
    <MemoryRouter initialEntries={[entry]}>
      <PathProbe />
      <Routes>
        <Route path="/results">
          <Route index element={<ResultsPage />} />
          <Route path=":runId" element={<ResultsPage />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

function tab(name: string): HTMLElement {
  return screen.getByRole("tab", { name });
}

/**
 * Radix' tabs activate on pointer-down plus focus, not on a synthetic click,
 * so the switch goes through `user-event` the way `ui/components/tabs.test`
 * does. Everything else on this page is a plain control and uses `fireEvent`.
 */
async function openTab(name: string): Promise<void> {
  await userEvent.setup().click(tab(name));
}

function sortHeader(label: string): HTMLElement {
  return within(screen.getByRole("table")).getByRole("button", { name: label });
}

/** The id cell of every body row, in render order. */
function rowIds(): string[] {
  return within(screen.getByRole("table"))
    .getAllByRole("row")
    .slice(1)
    .map((row) => within(row).getAllByRole("cell")[0]?.textContent ?? "");
}

/** The nth row of the run list, as the link a user clicks. */
function listItem(index: number): HTMLElement {
  const item = within(screen.getByRole("list")).getAllByRole("link")[index];
  if (item === undefined)
    throw new Error(`no run list item at ${String(index)}`);
  return item;
}

function filterInput(): HTMLElement {
  return screen.getByLabelText(m.label_filter_receiver_id());
}

/**
 * The message carries the bare term and `CompareTab` punctuates it, so the
 * accessible name the `Label` gives the trigger is the term plus exactly one
 * colon. Spelled out rather than matched with a regex so that a second colon
 * creeping back in fails here too.
 */
function compareSelect(): HTMLElement {
  return screen.getByRole("combobox", { name: `${m.label_compare_with()}:` });
}

async function downloadCSV(): Promise<string> {
  fireEvent.click(
    screen.getByRole("button", { name: m.action_download_csv() }),
  );
  const blob = createdBlobs.at(-1);
  if (blob === undefined) throw new Error("no blob handed to createObjectURL");
  return await blob.text();
}

// ---------------------------------------------------------------------------
// Page shell
// ---------------------------------------------------------------------------

describe("ResultsPage shell", () => {
  it("lists only completed runs and counts only those", () => {
    renderResults([
      run("run-1"),
      run("run-2", { status: "failed" }),
      run("run-3", { status: "running" }),
      run("run-4"),
    ]);

    const list = screen.getByRole("list");
    const items = within(list).getAllByRole("link");
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent("run-1");
    expect(items[1]).toHaveTextContent("run-4");
    expect(screen.queryByText("run-2")).toBeNull();
    expect(screen.queryByText("run-3")).toBeNull();
    expect(
      screen.getByText(`2 ${m.msg_completed_runs_plural()}`),
    ).toBeInTheDocument();
  });

  it("uses the singular count for exactly one completed run", () => {
    renderResults([run("run-1")]);

    expect(screen.getByText(`1 ${m.msg_completed_runs()}`)).toBeInTheDocument();
  });

  it("selects nothing until a run id is in the URL", () => {
    // Was: "selects the first completed run without a click". The list arrives
    // in backend order, so the run this used to open was not the newest one,
    // merely the first — someone else's numbers under the user's heading.
    renderResults([run("run-1"), run("run-4")], "/results");

    const items = within(screen.getByRole("list")).getAllByRole("link");
    expect(items.some((item) => item.hasAttribute("aria-current"))).toBe(false);
    expect(
      screen.getByText(m.msg_select_completed_run_details()),
    ).toBeInTheDocument();
    // Nothing is fetched for a run nobody asked for.
    expect(state.receiverTableArtifactIds).not.toContain(receiverArtifact.id);
  });

  it("opens the run named in the URL", () => {
    renderResults([run("run-1"), run("run-4")], "/results/run-4");

    const items = within(screen.getByRole("list")).getAllByRole("link");
    expect(items[0]).not.toHaveAttribute("aria-current");
    expect(items[1]).toHaveAttribute("aria-current", "page");
    expect(state.receiverTableArtifactIds).toContain(receiverArtifact.id);
  });

  it("switches the detail pane when another run is selected", () => {
    renderResults([run("run-1"), run("run-4")], "/results/run-1");

    fireEvent.click(listItem(1));

    expect(pathname()).toBe("/results/run-4");
    const items = within(screen.getByRole("list")).getAllByRole("link");
    expect(items[0]).not.toHaveAttribute("aria-current");
    expect(items[1]).toHaveAttribute("aria-current", "page");
  });

  it("names an unknown run id rather than falling back to another run", () => {
    renderResults([run("run-1")], "/results/nope");

    expect(
      screen.getByText(m.msg_unknown_run_id({ runId: "nope" })),
    ).toBeInTheDocument();
    // The list stays populated, so the way out is right there — but no other
    // run's detail leaks in.
    const items = within(screen.getByRole("list")).getAllByRole("link");
    expect(items).toHaveLength(1);
    expect(items[0]).not.toHaveAttribute("aria-current");
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("says a run that has not completed has no results", () => {
    renderResults([run("run-2", { status: "failed" })], "/results/run-2");

    expect(
      screen.getByText(m.msg_run_not_completed({ runId: "run-2" })),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(m.msg_unknown_run_id({ runId: "run-2" })),
    ).toBeNull();
  });

  it("does not call a run unknown while the runs are loading", () => {
    // A deep link arrives before `useRuns` resolves. Calling the id unknown
    // there would flash a warning on every hard reload of a perfectly good URL.
    state.runsLoading = true;
    renderResults([], "/results/run-1");

    expect(
      screen.getByRole("heading", { name: m.page_title_results() }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(m.msg_unknown_run_id({ runId: "run-1" })),
    ).toBeNull();
  });

  it("offers the empty state when no run has completed", () => {
    renderResults([run("run-2", { status: "failed" })]);

    expect(screen.getByText(m.msg_no_completed_runs())).toBeInTheDocument();
    expect(
      screen.getByText(m.msg_select_completed_run_details()),
    ).toBeInTheDocument();
    // A link, not a button: the page that starts runs is a route, and a user
    // with nothing to look at here should be able to middle-click their way
    // to it.
    expect(screen.getByRole("link", { name: m.nav_run() })).toHaveAttribute(
      "href",
      "/run",
    );
  });
});

// ---------------------------------------------------------------------------
// The invariant the page states in a comment and nothing else enforces
// ---------------------------------------------------------------------------

describe("ResultsPage header above transient states", () => {
  function pageHeading(): HTMLElement {
    return screen.getByRole("heading", { name: m.page_title_results() });
  }

  it("keeps the page heading while the runs are loading", () => {
    state.runsLoading = true;
    renderResults([]);

    expect(pageHeading()).toBeInTheDocument();
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("keeps the page heading when the runs request fails", () => {
    state.runsError = new Error("Request failed: 500");
    renderResults([]);

    expect(pageHeading()).toBeInTheDocument();
    expect(screen.getByText(m.msg_api_error_results())).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Tabs
// ---------------------------------------------------------------------------

describe("ResultsPage tabs", () => {
  it("opens on the receivers tab", () => {
    renderResults();

    expect(tab(m.tab_receivers())).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("table")).toBeInTheDocument();
  });

  it("moves to the raster tab and back", async () => {
    renderResults([
      run("run-1", { artifacts: [receiverArtifact, rasterArtifact] }),
    ]);

    await openTab(m.tab_raster());

    expect(tab(m.tab_raster())).toHaveAttribute("aria-selected", "true");
    expect(screen.queryByRole("table")).toBeNull();
    expect(screen.getByText("grid.meta.json")).toBeInTheDocument();
    expect(screen.getByText("120 × 80")).toBeInTheDocument();

    await openTab(m.tab_receivers());

    expect(screen.getByRole("table")).toBeInTheDocument();
  });

  it("offers the command that writes these bands out", async () => {
    renderResults([
      run("run-1", { artifacts: [receiverArtifact, rasterArtifact] }),
    ]);

    await openTab(m.tab_raster());

    // The exact line, not a fragment: a command the user copies either runs in
    // their terminal or fails there with this UI's name on it. The clipboard
    // itself is `copy-field.test.tsx`'s subject, so this only asserts that the
    // page offers the control.
    expect(
      screen.getByText("aconiq export --run-id run-1 --format geotiff"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: m.action_copy() }),
    ).toBeInTheDocument();
  });

  it("says so when the run carries no raster artifact", async () => {
    renderResults();

    await openTab(m.tab_raster());

    expect(screen.getByText(m.msg_no_raster_artifacts())).toBeInTheDocument();
  });

  it("moves to the compare tab", async () => {
    renderResults();

    await openTab(m.tab_compare());

    expect(tab(m.tab_compare())).toHaveAttribute("aria-selected", "true");
    expect(screen.getByText(m.msg_select_run_compare())).toBeInTheDocument();
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("forgets the receivers filter when the tab is left and re-entered", async () => {
    renderResults();

    fireEvent.change(filterInput(), { target: { value: "R1" } });
    expect(rowIds()).toEqual(["R1", "R10"]);

    await openTab(m.tab_compare());
    await openTab(m.tab_receivers());

    // Radix unmounts the inactive panel, so the tab's state goes with it.
    expect(filterInput()).toHaveValue("");
    expect(rowIds()).toEqual(["R1", "R10", "R2"]);
  });
});

// ---------------------------------------------------------------------------
// Receivers tab: states
// ---------------------------------------------------------------------------

describe("ResultsPage receivers tab states", () => {
  it("says so when the run carries no receiver table", () => {
    renderResults([run("run-1", { artifacts: [rasterArtifact] })]);

    expect(screen.getByText(m.msg_no_receiver_artifacts())).toBeInTheDocument();
    // Not even asked for: the hook is passed null when there is no artifact.
    expect(state.receiverTableArtifactIds).toEqual([null]);
  });

  it("shows a loading line while the table is being fetched", () => {
    state.receiverTableLoading = true;
    state.receiverTable = null;
    renderResults();

    expect(
      screen.getByText(m.status_loading_receiver_table()),
    ).toBeInTheDocument();
  });

  it("shows the load error when the table cannot be fetched", () => {
    state.receiverTable = null;
    state.receiverTableError = new Error("Request failed: 404");
    renderResults();

    expect(screen.getByText(m.error_load_receiver_table())).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Receivers tab: filtering
// ---------------------------------------------------------------------------

describe("ResultsPage receiver rows link to the map", () => {
  /*
   * The table is on `/results` and the map on `/model`, so the row cannot
   * highlight anything live — there is no map on this page to highlight on.
   * What it can do is take the reader to the one that has it, and `/model`
   * already knows how to honour `?select=` and `?run=` and strip them again.
   *
   * Only for a row the map can actually open, though: an `auto-grid` run names
   * its receivers itself, none of those ids is in the model store, and a link
   * there would navigate and then open nothing.
   */

  /** Puts the given ids into the model store as explicit receivers. */
  function seedReceivers(...ids: string[]) {
    useModelStore.getState().hydrateModel({
      features: [],
      receivers: ids.map((id) => ({
        id,
        heightM: 4,
        geometry: { type: "Point" as const, coordinates: [0, 0] },
      })),
      calcArea: null,
    });
  }

  it("sends each row to the map with the receiver selected, and the run with it", () => {
    // The run travels with the receiver. Without it the map drew whichever run
    // finished last, so a row followed from an older run landed on a different
    // run's levels under the id of the one that was clicked.
    seedReceivers("R1", "R2", "R10");
    renderResults();

    const link = screen.getByRole("link", {
      name: m.action_show_receiver_on_map({ id: "R2" }),
    });

    expect(link).toHaveAttribute("href", "/model?select=R2&run=run-1");
  });

  it("is a link and not a button, so it has an href to copy", () => {
    // A button that navigates has no href, no middle-click, no context menu
    // and no entry in a screen reader's links rotor — and no axe rule catches
    // the substitution.
    seedReceivers("R1", "R2", "R10");
    renderResults();

    const row = within(screen.getByRole("table")).getAllByRole("row")[1];
    expect(row).toBeDefined();
    expect(
      within(row as HTMLElement).queryByRole("button", { name: /R1/ }),
    ).not.toBeInTheDocument();
    expect(
      within(row as HTMLElement).getByRole("link", { name: /R1/ }),
    ).toHaveAttribute("href");
  });

  it("escapes an id that would otherwise change the query", () => {
    // Receiver ids come out of an import and are not constrained to anything.
    // An id holding `&` or `#` would end the parameter and select nothing.
    state.receiverTable = {
      ...table,
      records: [{ id: "R&1 #2", x: 0, y: 0, height_m: 4, values: {} }],
    } satisfies ReceiverTable;
    seedReceivers("R&1 #2");
    renderResults();

    expect(
      screen.getByRole("link", {
        name: m.action_show_receiver_on_map({ id: "R&1 #2" }),
      }),
    ).toHaveAttribute("href", "/model?select=R%261+%232&run=run-1");
  });

  it("leaves a generated auto-grid id as plain text", () => {
    // `grid-000000` is named by the run, not by the model. `SelectRequest`
    // would hand it to `FeatureEditor`, which finds neither a feature nor a
    // receiver under it and renders nothing — a link promising a selection
    // that never happens.
    state.receiverTable = {
      ...table,
      records: [{ id: "grid-000000", x: 0, y: 0, height_m: 4, values: {} }],
    } satisfies ReceiverTable;
    renderResults();

    expect(screen.queryByRole("link", { name: /grid-000000/ })).toBeNull();
    expect(screen.getByText("grid-000000")).toBeInTheDocument();
  });
});

describe("ResultsPage receiver filtering", () => {
  /*
   * One message owns the whole count, placeholders and all. The fragments it
   * replaced read "3 / 3 / records", because the message carried a slash of
   * its own; more to the point, German pluralises the noun rather than
   * suffixing it, which no amount of JSX concatenation can express.
   */
  it("counts every record when nothing is filtered", () => {
    renderResults();

    expect(
      screen.getByText(m.msg_records_count_other({ shown: 3, total: 3 })),
    ).toBeInTheDocument();
  });

  it("uses the singular when the table holds one record", () => {
    state.receiverTable = {
      ...table,
      records: table.records.slice(0, 1),
    } satisfies ReceiverTable;
    renderResults();

    // The plural follows the total, which is what the noun counts.
    expect(
      screen.getByText(m.msg_records_count_one({ shown: 1, total: 1 })),
    ).toBeInTheDocument();
  });

  it("keeps the records whose id contains the query", () => {
    renderResults();

    fireEvent.change(filterInput(), { target: { value: "R1" } });

    expect(rowIds()).toEqual(["R1", "R10"]);
    expect(
      screen.getByText(m.msg_records_count_other({ shown: 2, total: 3 })),
    ).toBeInTheDocument();
  });

  it("matches case-insensitively", () => {
    renderResults();

    fireEvent.change(filterInput(), { target: { value: "r2" } });

    expect(rowIds()).toEqual(["R2"]);
  });

  it("matches anywhere in the id, not only at the start", () => {
    renderResults();

    fireEvent.change(filterInput(), { target: { value: "10" } });

    expect(rowIds()).toEqual(["R10"]);
  });

  it("shows the empty-filter row when nothing matches", () => {
    renderResults();

    fireEvent.change(filterInput(), { target: { value: "zzz" } });

    expect(
      screen.getByText(m.msg_no_records_match_filter()),
    ).toBeInTheDocument();
    expect(
      screen.getByText(m.msg_records_count_other({ shown: 0, total: 3 })),
    ).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Receivers tab: sorting
// ---------------------------------------------------------------------------

describe("ResultsPage receiver sorting", () => {
  it("starts sorted by id ascending", () => {
    renderResults();

    // `localeCompare`, so "R10" sits between "R1" and "R2".
    expect(rowIds()).toEqual(["R1", "R10", "R2"]);
    expect(
      within(screen.getByRole("table")).getAllByRole("columnheader")[0],
    ).toHaveAttribute("aria-sort", "ascending");
  });

  it("reverses the id column on a second click", () => {
    renderResults();

    fireEvent.click(sortHeader("id"));

    const ids = rowIds();
    expect(ids.at(0)).toBe("R2");
    expect(ids.at(-1)).toBe("R1");
    expect(
      within(screen.getByRole("table")).getAllByRole("columnheader")[0],
    ).toHaveAttribute("aria-sort", "descending");

    fireEvent.click(sortHeader("id"));

    expect(rowIds().at(0)).toBe("R1");
    expect(rowIds().at(-1)).toBe("R2");
  });

  it("sorts a numeric column numerically and starts ascending", () => {
    renderResults();

    fireEvent.click(sortHeader(m.table_header_height_m()));

    // 2.5, 4, 8 — not the string order "2.5" < "4" < "8" would also give.
    expect(rowIds().at(0)).toBe("R2");
    expect(rowIds().at(-1)).toBe("R10");

    fireEvent.click(sortHeader(m.table_header_height_m()));

    expect(rowIds().at(0)).toBe("R10");
    expect(rowIds().at(-1)).toBe("R2");
  });

  it("sorts the x column numerically", () => {
    renderResults();

    fireEvent.click(sortHeader("x"));

    expect(rowIds()).toEqual(["R2", "R1", "R10"]);
  });

  it("sorts a record missing an indicator as if it read 0 dB", () => {
    renderResults();

    // `R10` has no `Lnight`; `sortedRecords` substitutes 0, so ascending puts
    // it below every measured receiver instead of at the end or out of the way.
    fireEvent.click(sortHeader(`Lnight (${unitFor(table.units, "Lnight")})`));

    expect(rowIds()).toEqual(["R10", "R2", "R1"]);

    fireEvent.click(sortHeader(`Lnight (${unitFor(table.units, "Lnight")})`));

    // Descending it is last, again as a 0 — never flagged as absent.
    expect(rowIds()).toEqual(["R1", "R2", "R10"]);
  });

  it("resets to ascending when the sort moves to another column", () => {
    renderResults();

    fireEvent.click(sortHeader("id"));
    expect(rowIds().at(0)).toBe("R2");

    fireEvent.click(sortHeader("y"));

    expect(rowIds()).toEqual(["R10", "R1", "R2"]);
  });

  it("sorts within the filtered set only", () => {
    renderResults();

    fireEvent.change(filterInput(), { target: { value: "R1" } });
    fireEvent.click(sortHeader("id"));

    expect(rowIds()).toEqual(["R10", "R1"]);
  });
});

// ---------------------------------------------------------------------------
// Receivers tab: the window
// ---------------------------------------------------------------------------

describe("ResultsPage receiver table windowing", () => {
  const BIG = 5000;

  /**
   * A table big enough that mounting all of it would be the defect. Built
   * once and shared: nothing on the page mutates the table it is handed, and
   * five thousand records is real work to allocate.
   */
  const big: ReceiverTable = {
    indicator_order: ["Lden"],
    units: { Lden: "dB(A)" },
    records: Array.from({ length: BIG }, (_, i) => ({
      // Zero-padded, so the id order the page sorts by is the index order and
      // "the row after this one" is a claim a test can make.
      id: `R${String(i).padStart(4, "0")}`,
      x: i,
      y: i,
      height_m: 4,
      values: { Lden: 50 + (i % 20) },
    })),
  };

  /**
   * Rendering a five-thousand-row page under jsdom takes a second or so on its
   * own, and the suite runs several files at once — the default five seconds
   * is not headroom, it is a coin toss. The number is generous on purpose: it
   * is there to catch a hang, not to time the renderer.
   */
  const TIMEOUT_MS = 30_000;

  /** The scroll element the virtualizer watches: the table's own container. */
  function scrollContainer(): HTMLElement {
    const table = screen.getByRole("table");
    const container = table.parentElement;
    if (container === null) throw new Error("the table has no scroll parent");
    return container;
  }

  function scrollTo(offset: number): void {
    const container = scrollContainer();
    container.scrollTop = offset;
    fireEvent.scroll(container);
  }

  it(
    "bounds the element the virtualizer measures",
    () => {
      state.receiverTable = big;
      renderResults();

      /*
       * The window only windows anything if its scroll element has a height
       * that does not follow the row count. `overflow-auto` does not supply
       * one: the div would grow to the spacer rows, which are the size of the
       * whole table, and `virtual-core` — reading `offsetHeight` — would take
       * that for the viewport and mount every row.
       *
       * jsdom performs no layout, so this asserts the cap is declared rather
       * than that it is obeyed. It has to be a resolved length against the
       * viewport, not a percentage, which would hand the question back to an
       * ancestor chain that does not answer it.
       */
      const maxHeight = scrollContainer().style.maxHeight;
      expect(maxHeight).toMatch(/^\d+(\.\d+)?(vh|px|rem)$/);
    },
    TIMEOUT_MS,
  );

  it(
    "mounts a window of rows, not five thousand of them",
    () => {
      state.receiverTable = big;
      renderResults();

      const ids = rowIds();
      // The exact count is the viewport divided by the row height plus the
      // overscan, which is an implementation detail; that it is nowhere near
      // the table's size is not.
      expect(ids.length).toBeGreaterThan(0);
      expect(ids.length).toBeLessThan(BIG / 10);
      expect(ids[0]).toBe("R0000");
    },
    TIMEOUT_MS,
  );

  it(
    "tells a screen reader how many rows the table really has",
    () => {
      state.receiverTable = big;
      renderResults();

      // Without this the table would claim to be the size of its window. The
      // count includes the header row, and the rendered rows carry the index
      // they would have if every row were mounted.
      const table = screen.getByRole("table");
      expect(table).toHaveAttribute("aria-rowcount", String(BIG + 1));
      const rows = within(table).getAllByRole("row");
      expect(rows[0]).toHaveAttribute("aria-rowindex", "1");
      expect(rows[1]).toHaveAttribute("aria-rowindex", "2");
    },
    TIMEOUT_MS,
  );

  it(
    "renders later ids once the container is scrolled",
    () => {
      state.receiverTable = big;
      renderResults();

      expect(rowIds()).not.toContain("R3000");

      // 3000 rows down, at the row height the table lays out with.
      scrollTo(3000 * ROW_PX);

      // One scan, read three ways: role queries are not cheap on a table this
      // size and this test is the one that runs them twice.
      const rows = within(screen.getByRole("table"))
        .getAllByRole("row")
        .slice(1);
      const ids = rows.map((r) => r.querySelector("td")?.textContent ?? "");
      expect(ids).toContain("R3000");
      expect(ids).not.toContain("R0000");
      // Still a window, not the whole table pulled in behind the scroll.
      expect(ids.length).toBeLessThan(BIG / 10);
      // The index travels with the row, so row 3001 announces itself as row 3001.
      expect(rows[ids.indexOf("R3000")]).toHaveAttribute(
        "aria-rowindex",
        "3002",
      );
    },
    TIMEOUT_MS,
  );

  it(
    "scrolls to the end without running past it",
    () => {
      state.receiverTable = big;
      renderResults();

      scrollTo(BIG * ROW_PX);

      const ids = rowIds();
      expect(ids.at(-1)).toBe(`R${String(BIG - 1).padStart(4, "0")}`);
    },
    TIMEOUT_MS,
  );

  it(
    "downloads all five thousand rows, not the mounted ones",
    async () => {
      state.receiverTable = big;
      renderResults();

      const csv = await downloadCSV();
      const lines = csv.split("\n");

      // Header, BIG records, and the trailing empty string after the final LF.
      expect(lines).toHaveLength(BIG + 2);
      expect(lines[0]).toBe("id,x,y,height_m,Lden");
      expect(lines[1]).toBe("R0000,0,0,4,50");
      expect(lines[BIG]).toBe(
        `R${String(BIG - 1).padStart(4, "0")},4999,4999,4,${String(50 + ((BIG - 1) % 20))}`,
      );
    },
    TIMEOUT_MS,
  );

  it(
    "keeps the filter and the count over the whole table, not the window",
    () => {
      state.receiverTable = big;
      renderResults();

      fireEvent.change(filterInput(), { target: { value: "R490" } });

      // R4900..R4909 — ten rows, all of them found even though none of them was
      // ever mounted before the filter narrowed the table to them.
      expect(rowIds()).toEqual(
        Array.from({ length: 10 }, (_, i) => `R490${String(i)}`),
      );
      expect(
        screen.getByText(m.msg_records_count_other({ shown: 10, total: BIG })),
      ).toBeInTheDocument();
      expect(screen.getByRole("table")).toHaveAttribute("aria-rowcount", "11");
    },
    TIMEOUT_MS,
  );
});

// ---------------------------------------------------------------------------
// Receivers tab: CSV download
// ---------------------------------------------------------------------------

describe("ResultsPage CSV download", () => {
  it("hands the canonical bytes to a text/csv blob named receivers.csv", async () => {
    renderResults();

    const csv = await downloadCSV();

    // The spelling is Go's encoding/csv, produced by the shared builder in
    // model/receiver-csv.ts: fields are quoted only where they have to be, and
    // every record — the header included — ends in a single LF. The builder's
    // own rules are tested in model/receiver-csv.test.ts and pinned against the
    // CLI in model/receiver-csv.parity.test.ts; what this file owns is that the
    // page hands the right table to it and the right blob to the browser.
    expect(csv).toBe(
      [
        "id,x,y,height_m,Lden,Lnight",
        "R1,100.5,200.25,4,62.4,55.1",
        "R10,300,100,8,71.8,",
        "R2,50.5,400.25,2.5,58.2,51.9",
        "",
      ].join("\n"),
    );
    expect(createdBlobs.at(-1)?.type).toBe("text/csv");
    expect(downloadNames).toEqual(["receivers.csv"]);
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:aconiq/receivers.csv");
  });

  it("exports the filtered and sorted view, not the whole table", async () => {
    renderResults();

    fireEvent.change(filterInput(), { target: { value: "R1" } });
    fireEvent.click(sortHeader("id"));
    expect(rowIds()).toEqual(["R10", "R1"]);

    const csv = await downloadCSV();

    expect(csv).toBe(
      [
        "id,x,y,height_m,Lden,Lnight",
        "R10,300,100,8,71.8,",
        "R1,100.5,200.25,4,62.4,55.1",
        "",
      ].join("\n"),
    );
    expect(csv).not.toContain("R2");
  });

  it("writes the header and its newline when the filter matches nothing", async () => {
    renderResults();

    fireEvent.change(filterInput(), { target: { value: "zzz" } });

    // An empty table is still a complete CSV file: one header record,
    // terminated like any other.
    expect(await downloadCSV()).toBe("id,x,y,height_m,Lden,Lnight\n");
  });
});

// ---------------------------------------------------------------------------
// Compare tab
// ---------------------------------------------------------------------------

describe("ResultsPage compare tab", () => {
  async function openCompareWith(runId: string) {
    await openTab(m.tab_compare());
    fireEvent.pointerDown(compareSelect(), {
      button: 0,
      ctrlKey: false,
      pointerType: "mouse",
    });
    fireEvent.click(screen.getByRole("option", { name: new RegExp(runId) }));
  }

  it("shows both run columns once a second run is picked", async () => {
    renderResults([run("run-1"), run("run-4")]);

    await openCompareWith("run-4");

    expect(
      screen.getByRole("heading", { name: m.msg_run_column_selected() }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: m.msg_run_column_compare() }),
    ).toBeInTheDocument();
    // The prompt is replaced by the columns, not joined by a notice.
    expect(screen.queryByText(m.msg_select_run_compare())).toBeNull();
  });

  it("never offers the selected run as its own comparison", async () => {
    renderResults([run("run-1"), run("run-4")]);

    await openTab(m.tab_compare());
    fireEvent.pointerDown(compareSelect(), {
      button: 0,
      ctrlKey: false,
      pointerType: "mouse",
    });

    const options = screen.getAllByRole("option").map((o) => o.textContent);
    expect(options).toHaveLength(2);
    expect(options[0]).toBe(m.option_select_compare_run());
    expect(options[1]).toContain("run-4");
  });
});

// ---------------------------------------------------------------------------
// Label punctuation
// ---------------------------------------------------------------------------

describe("ResultsPage label punctuation", () => {
  /*
   * A `label_*` message is the bare term — "Min", "Dimensions" — and the page
   * punctuates it where it introduces a value. The colon is presentation:
   * `locale-parity.test.ts` refuses a colon typed into either catalogue, so a
   * term reused as a field name or a column head stays correct and no
   * translator has to remember not to type one. These tests pin the other
   * half — exactly one colon here, never two.
   */

  it("punctuates each indicator summary label exactly once", () => {
    renderResults();

    // One card per indicator, so each label appears twice.
    expect(screen.getAllByText(`${m.label_min()}:`)).toHaveLength(2);
    expect(screen.getAllByText(`${m.label_max()}:`)).toHaveLength(2);
    expect(screen.getAllByText(`${m.label_mean()}:`)).toHaveLength(2);
    expect(screen.queryAllByText(`${m.label_min()}::`)).toHaveLength(0);
  });

  it("punctuates each raster metadata label exactly once", async () => {
    renderResults([
      run("run-1", { artifacts: [receiverArtifact, rasterArtifact] }),
    ]);

    await openTab(m.tab_raster());

    expect(screen.getByText(`${m.label_dimensions()}:`)).toBeInTheDocument();
    expect(screen.getByText(`${m.label_nodata()}:`)).toBeInTheDocument();
    expect(screen.getByText(`${m.label_bands()}:`)).toBeInTheDocument();
    expect(screen.getByText(`${m.label_band_names()}:`)).toBeInTheDocument();
    expect(screen.queryByText(`${m.label_dimensions()}::`)).toBeNull();

    // The card has no Unit row any more: the unit is per band, so each band
    // carries its own in the list above rather than one value standing for
    // however many disagree.
    expect(screen.getByText(/Lden \(dB\(A\)\)/)).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// Accessibility
// ---------------------------------------------------------------------------

describe("ResultsPage heading order", () => {
  function assertContiguousHeadings() {
    // The shell's h1 sits above this page, so a level of 2 is the entry.
    const levels = screen
      .getAllByRole("heading")
      .map((h) => Number(h.tagName.slice(1)));
    let deepest = 1;
    for (const level of levels) {
      expect(level).toBeLessThanOrEqual(deepest + 1);
      deepest = Math.max(deepest, level);
    }
    return levels;
  }

  it("keeps heading levels contiguous on the receivers tab", () => {
    renderResults();

    expect(assertContiguousHeadings()).toEqual([2]);
  });

  it("keeps heading levels contiguous with both compare columns open", async () => {
    renderResults([run("run-1"), run("run-4")]);

    await openTab(m.tab_compare());
    fireEvent.pointerDown(compareSelect(), {
      button: 0,
      ctrlKey: false,
      pointerType: "mouse",
    });
    fireEvent.click(screen.getByRole("option", { name: /run-4/ }));

    // `RunColumn` renders the only h3 on the page; that is where a level could
    // skip from the page's h2.
    const levels = assertContiguousHeadings();
    expect(levels).toContain(3);
  });

  it("keeps heading levels contiguous in both transient states", () => {
    state.runsLoading = true;
    renderResults([]);
    expect(assertContiguousHeadings()).toEqual([2]);

    screen.getByRole("heading", { name: m.page_title_results() });
  });
});

describe("ResultsPage windowed table accessibility", () => {
  /*
   * The route-level baseline in `e2e/a11y.spec.ts` cannot reach this: `/results`
   * in a fresh browser-mode app has no completed run, so it renders an empty
   * state and the table is never on the page there. The windowed table's own
   * semantics are therefore checked here, over a table large enough that the
   * spacer rows are present — they are the part that could plausibly break a
   * table's structure, and this is what says they do not.
   *
   * Structural rules only. `color-contrast` needs a canvas jsdom does not have,
   * and contrast belongs to the e2e baseline, on a route that actually paints.
   */
  it("introduces no axe violations with the window in place", async () => {
    state.receiverTable = {
      indicator_order: ["Lden"],
      units: { Lden: "dB(A)" },
      records: Array.from({ length: 5000 }, (_, i) => ({
        id: `R${String(i).padStart(4, "0")}`,
        x: i,
        y: i,
        height_m: 4,
        values: { Lden: 50 + (i % 20) },
      })),
    } satisfies ReceiverTable;
    renderResults();

    const results = await axe.run(screen.getByRole("table"), {
      runOnly: { type: "tag", values: ["wcag2a", "wcag2aa", "best-practice"] },
    });

    // Mapped to strings so a failure names the rule rather than printing a
    // page of axe's node objects.
    expect(results.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
  }, 30_000);
});
