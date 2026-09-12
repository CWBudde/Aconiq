import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

/**
 * Characterisation tests: they describe what the results page does today, not
 * what it ought to do. They exist as a safety net under a refactor, so where
 * the current behaviour is wrong the wrong behaviour is pinned deliberately
 * and marked as such — see the CSV quoting cases below.
 */

const state = vi.hoisted(() => {
  const value: {
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
  unit: "dB(A)",
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
  unit: "dB(A)",
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

beforeEach(() => {
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

/** Renders the page with `runs` and the first completed run auto-selected. */
function renderResults(runs: RunSummary[] = [run("run-1")]) {
  state.runs = runs;
  render(<ResultsPage />);
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

/** The nth row of the run list, as the button a user clicks. */
function listItem(index: number): HTMLElement {
  const item = within(screen.getByRole("list")).getAllByRole("button")[index];
  if (item === undefined)
    throw new Error(`no run list item at ${String(index)}`);
  return item;
}

function filterInput(): HTMLElement {
  return screen.getByLabelText(m.label_filter_receiver_id());
}

/**
 * The label message already ends in a colon and `CompareTab` appends a second
 * one, so the accessible name really is "Compare with::". Pinned here rather
 * than papered over with a regex — see the "punctuates its own labels twice"
 * test below.
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
    const items = within(list).getAllByRole("button");
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

  it("selects the first completed run without a click", () => {
    renderResults([run("run-1"), run("run-4")]);

    const items = within(screen.getByRole("list")).getAllByRole("button");
    expect(items[0]).toHaveAttribute("aria-current", "true");
    expect(items[1]).not.toHaveAttribute("aria-current");
    expect(state.receiverTableArtifactIds).toContain(receiverArtifact.id);
  });

  it("switches the detail pane when another run is selected", () => {
    renderResults([run("run-1"), run("run-4")]);

    fireEvent.click(listItem(1));

    const items = within(screen.getByRole("list")).getAllByRole("button");
    expect(items[0]).not.toHaveAttribute("aria-current");
    expect(items[1]).toHaveAttribute("aria-current", "true");
  });

  it("offers the empty state when no run has completed", () => {
    renderResults([run("run-2", { status: "failed" })]);

    expect(screen.getByText(m.msg_no_completed_runs())).toBeInTheDocument();
    expect(
      screen.getByText(m.msg_select_completed_run_details()),
    ).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------
// The invariant results.tsx:688-689 states in a comment and nothing enforces
// ---------------------------------------------------------------------------

describe("ResultsPage header above transient states", () => {
  function pageHeading(): HTMLElement {
    return screen.getByRole("heading", { name: m.page_title_results() });
  }

  it("keeps the page heading while the runs are loading", () => {
    state.runsLoading = true;
    render(<ResultsPage />);

    expect(pageHeading()).toBeInTheDocument();
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("keeps the page heading when the runs request fails", () => {
    state.runsError = new Error("Request failed: 500");
    render(<ResultsPage />);

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

describe("ResultsPage receiver filtering", () => {
  it("counts every record when nothing is filtered", () => {
    renderResults();

    expect(
      screen.getByText(`3 / 3 ${m.msg_records_count()}`),
    ).toBeInTheDocument();
  });

  it("keeps the records whose id contains the query", () => {
    renderResults();

    fireEvent.change(filterInput(), { target: { value: "R1" } });

    expect(rowIds()).toEqual(["R1", "R10"]);
    expect(
      screen.getByText(`2 / 3 ${m.msg_records_count()}`),
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
      screen.getByText(`0 / 3 ${m.msg_records_count()}`),
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
    fireEvent.click(sortHeader(`Lnight (${table.unit})`));

    expect(rowIds()).toEqual(["R10", "R2", "R1"]);

    fireEvent.click(sortHeader(`Lnight (${table.unit})`));

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
// Receivers tab: CSV download
// ---------------------------------------------------------------------------

describe("ResultsPage CSV download", () => {
  it("writes the header row and every record, quoted throughout", async () => {
    renderResults();

    const csv = await downloadCSV();

    expect(csv).toBe(
      [
        '"id","x","y","height_m","Lden","Lnight"',
        '"R1","100.5","200.25","4","62.4","55.1"',
        '"R10","300","100","8","71.8",""',
        '"R2","50.5","400.25","2.5","58.2","51.9"',
      ].join("\n"),
    );
    expect(createdBlobs.at(-1)?.type).toBe("text/csv");
    expect(downloadNames).toEqual(["receivers.csv"]);
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:aconiq/receivers.csv");
  });

  it("writes the header from indicator_order, not from the records", async () => {
    state.receiverTable = {
      indicator_order: ["Lnight", "Lden"],
      unit: "dB(A)",
      records: [
        { id: "R1", x: 1, y: 2, height_m: 3, values: { Lden: 60, Lnight: 50 } },
      ],
    } satisfies ReceiverTable;
    renderResults();

    const csv = await downloadCSV();

    expect(csv).toBe(
      [
        '"id","x","y","height_m","Lnight","Lden"',
        '"R1","1","2","3","50","60"',
      ].join("\n"),
    );
  });

  it("leaves a missing indicator empty rather than writing the 0 the table shows", async () => {
    renderResults();

    const csv = await downloadCSV();

    // The table cell for R10/Lnight renders `?? 0`; the CSV writes `?? ""`.
    expect(csv).toContain('"R10","300","100","8","71.8",""');
    expect(rowIds()).toContain("R10");
  });

  it("exports the filtered and sorted view, not the whole table", async () => {
    renderResults();

    fireEvent.change(filterInput(), { target: { value: "R1" } });
    fireEvent.click(sortHeader("id"));
    expect(rowIds()).toEqual(["R10", "R1"]);

    const csv = await downloadCSV();

    expect(csv).toBe(
      [
        '"id","x","y","height_m","Lden","Lnight"',
        '"R10","300","100","8","71.8",""',
        '"R1","100.5","200.25","4","62.4","55.1"',
      ].join("\n"),
    );
    expect(csv).not.toContain('"R2"');
  });

  it("writes an empty body when the filter matches nothing", async () => {
    renderResults();

    fireEvent.change(filterInput(), { target: { value: "zzz" } });

    expect(await downloadCSV()).toBe('"id","x","y","height_m","Lden","Lnight"');
  });

  it("emits broken CSV for an id containing a double quote", async () => {
    state.receiverTable = {
      indicator_order: ["Lden"],
      unit: "dB(A)",
      records: [{ id: 'R"1', x: 1, y: 2, height_m: 3, values: { Lden: 60 } }],
    } satisfies ReceiverTable;
    renderResults();

    const csv = await downloadCSV();

    // KNOWN DEFECT, pinned on purpose: `downloadCSV` wraps every field in
    // quotes but never doubles an embedded one, so `R"1` leaves the field at
    // the first inner quote and the row no longer parses. RFC 4180 wants
    // `"R""1"`. The fix is one line in results.tsx (escape `"` as `""`);
    // update this expectation in the same commit.
    expect(csv).toBe(
      ['"id","x","y","height_m","Lden"', '"R"1","1","2","3","60"'].join("\n"),
    );
  });

  it("quotes an id containing a comma correctly today", async () => {
    state.receiverTable = {
      indicator_order: ["Lden"],
      unit: "dB(A)",
      records: [{ id: "R,2", x: 1, y: 2, height_m: 3, values: { Lden: 60 } }],
    } satisfies ReceiverTable;
    renderResults();

    const csv = await downloadCSV();

    // The always-quote style happens to be correct for a comma: the field is
    // delimited, so a reader does not split on it. Pinned so the quote fix
    // above cannot regress this case.
    expect(csv).toBe(
      ['"id","x","y","height_m","Lden"', '"R,2","1","2","3","60"'].join("\n"),
    );
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
    expect(
      screen.getByText(m.msg_run_to_run_diff_deferred()),
    ).toBeInTheDocument();
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
   * KNOWN DEFECT, pinned on purpose: every `label_*` message already ends in a
   * colon, and results.tsx appends another one at each use, so the UI reads
   * "Min::", "Dimensions::" and "Compare with::". The fix is to drop the
   * literal colons from results.tsx; update these expectations with it.
   */

  it("doubles the colon on the indicator summary labels", () => {
    renderResults();

    // One card per indicator, so each label appears twice.
    expect(screen.getAllByText(`${m.label_min()}:`)).toHaveLength(2);
    expect(screen.getAllByText(`${m.label_max()}:`)).toHaveLength(2);
    expect(screen.getAllByText(`${m.label_mean()}:`)).toHaveLength(2);
  });

  it("doubles the colon on the raster metadata labels", async () => {
    renderResults([
      run("run-1", { artifacts: [receiverArtifact, rasterArtifact] }),
    ]);

    await openTab(m.tab_raster());

    expect(screen.getByText(`${m.label_dimensions()}:`)).toBeInTheDocument();
    expect(screen.getByText(`${m.label_nodata()}:`)).toBeInTheDocument();
    expect(screen.getByText(`${m.label_unit()}:`)).toBeInTheDocument();
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
    render(<ResultsPage />);
    expect(assertContiguousHeadings()).toEqual([2]);

    screen.getByRole("heading", { name: m.page_title_results() });
  });
});
