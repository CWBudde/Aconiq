import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import type { ArtifactRef, RunSummary } from "@/api/client";
import ExportPage from "./export";
import { formatDateTime } from "@/ui/format";
import { m } from "@/i18n/messages";

/**
 * A characterisation net around the export page, written before a refactor:
 * every assertion below describes what the page does today, not what it
 * ought to do. Where today's behaviour is wrong it is pinned anyway and
 * marked as a defect, so a later fix has to change the test on purpose.
 *
 * The page branches on `backend.capabilities.canExport` in three separate
 * places — the dialog's description, the CLI command it offers instead of a
 * button, and the Generate button itself — so the capability is a mutable
 * getter here and every branch is exercised with both values.
 */

const state = vi.hoisted(() => {
  const value: {
    /** Descriptors for `useStandardLabel`; empty means no tier qualifier. */
    standards: { id: string; evidence_tier?: string }[];
    canExport: boolean;
    runs: unknown[] | undefined;
    runsLoading: boolean;
    runsError: Error | null;
    createdExports: string[];
    createExportPending: boolean;
    createExportError: Error | null;
    createExportResult: { id: string };
  } = {
    standards: [],
    canExport: false,
    runs: [],
    runsLoading: false,
    runsError: null,
    createdExports: [],
    createExportPending: false,
    createExportError: null,
    createExportResult: { id: "run-1" },
  };
  return value;
});

// Explicit capabilities rather than whatever the env selects: `canExport` is
// the page's only mode switch, and both of its values have to be reachable.
vi.mock("@/api/backend", () => ({
  backend: {
    get capabilities() {
      return {
        kind: state.canExport ? "browser" : "http",
        canExport: state.canExport,
        runsAgainstSavedModel: !state.canExport,
        runsChangeExternally: !state.canExport,
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
  useCreateExport: () => ({
    mutate: (
      runId: string,
      opts?: { onSuccess?: (run: { id: string }) => void },
    ) => {
      state.createdExports.push(runId);
      opts?.onSuccess?.(state.createExportResult);
    },
    isPending: state.createExportPending,
    isError: state.createExportError !== null,
    error: state.createExportError,
  }),
  getArtifactContentURL: (id: string) => `/api/v1/artifacts/${id}/content`,
}));

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const BUNDLE_CREATED_AT = "2026-03-04T12:05:00Z";
const RUN_FINISHED_AT = "2026-03-04T11:00:30Z";

function artifact(
  id: string,
  kind: string,
  path: string,
  createdAt: string = BUNDLE_CREATED_AT,
): ArtifactRef {
  return { id, kind, path, created_at: createdAt };
}

function run(id: string, artifacts: ArtifactRef[]): RunSummary {
  return {
    id,
    scenario_id: "default",
    standard_id: "rls19-road",
    version: "2019",
    profile: "default",
    status: "completed",
    started_at: "2026-03-04T11:00:00Z",
    finished_at: RUN_FINISHED_AT,
    log_path: `runs/${id}/run.log`,
    artifacts,
  };
}

const bundle = artifact(
  "a-bundle",
  "export.bundle",
  "exports/run-1/bundle.zip",
);
const htmlReport = artifact(
  "a-html",
  "export.report_html",
  "exports/run-1/report.html",
);
const markdownReport = artifact(
  "a-md",
  "export.report_markdown",
  "exports/run-1/report.md",
);
const jsonContext = artifact(
  "a-json",
  "export.report_context_json",
  "exports/run-1/report-context.json",
);

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Reads the current path out, so a click's navigation is asserted directly. */
function PathProbe() {
  return <span data-testid="pathname">{useLocation().pathname}</span>;
}

function pathname(): string {
  return screen.getByTestId("pathname").textContent;
}

/**
 * Renders the page under the same index/`:runId` pair `routes.tsx` registers.
 *
 * `path` defaults to the first run with exports, because that is what "a
 * bundle is open" means now and most of this file is about the detail pane.
 * The tests about the selection itself pass a path explicitly.
 */
function renderPage(runs: RunSummary[], path?: string) {
  state.runs = runs;
  const first = runs.find((r) =>
    r.artifacts.some((a) => a.kind.startsWith("export.")),
  );
  const entry = path ?? (first ? `/export/${first.id}` : "/export");
  render(
    <MemoryRouter initialEntries={[entry]}>
      <PathProbe />
      <Routes>
        <Route path="/export">
          <Route index element={<ExportPage />} />
          <Route path=":runId" element={<ExportPage />} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

/** The page header's action; the dialog's Generate button shares its label. */
function openDialog() {
  fireEvent.click(screen.getByRole("button", { name: m.action_new_export() }));
  return screen.getByRole("dialog");
}

function generateButton(dialog: HTMLElement): HTMLElement | null {
  return within(dialog).queryByRole("button", { name: m.action_new_export() });
}

/**
 * Radix' Select opens on a pointer event and measures the viewport; jsdom
 * implements neither. The shims in `beforeAll` are what let this pick a run
 * the way a user does rather than reaching past the control.
 */
function selectRun(dialog: HTMLElement, runId: string) {
  fireEvent.pointerDown(
    within(dialog).getByRole("combobox", { name: m.label_select_run() }),
    { button: 0, ctrlKey: false, pointerType: "mouse" },
  );
  fireEvent.click(screen.getByRole("option", { name: new RegExp(runId) }));
}

function listItem(runId: string): HTMLElement {
  return screen.getByRole("link", { name: new RegExp(runId) });
}

/**
 * The page header block. Its description interleaves a number with a message
 * (`{count} {runs with exports}`), so it is asserted as one text content
 * rather than through a text query that would have to match half a node.
 */
function pageHeader(): HTMLElement {
  const header = document.querySelector<HTMLElement>(
    '[data-slot="page-header"]',
  );
  if (header === null) throw new Error("no page header rendered");
  return header;
}

const writeText = vi.fn<(text: string) => Promise<void>>();

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
  // jsdom ships no clipboard; `CopyField`'s button only needs `writeText`.
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText },
    configurable: true,
  });
});

beforeEach(() => {
  state.canExport = false;
  state.runs = [];
  state.runsLoading = false;
  state.runsError = null;
  state.createdExports = [];
  state.createExportPending = false;
  state.createExportError = null;
  state.createExportResult = { id: "run-1" };
  writeText.mockReset();
  writeText.mockResolvedValue(undefined);
});

// ---------------------------------------------------------------------------

describe("ExportPage new-export dialog: canExport branches", () => {
  it("describes the CLI route when the backend cannot export", () => {
    renderPage([run("run-1", [bundle])]);

    const dialog = openDialog();
    expect(
      within(dialog).getByText(m.dialog_desc_new_export()),
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByText(m.dialog_desc_new_export_browser()),
    ).toBeNull();
  });

  it("describes the in-browser route when the backend can export", () => {
    state.canExport = true;
    renderPage([run("run-1", [bundle])]);

    const dialog = openDialog();
    expect(
      within(dialog).getByText(m.dialog_desc_new_export_browser()),
    ).toBeInTheDocument();
    expect(within(dialog).queryByText(m.dialog_desc_new_export())).toBeNull();
  });

  it("offers the CLI command, and a disabled Generate button, without canExport", () => {
    // Was: "and no Generate button". Hiding it left the user hunting for a
    // control that was not there; `ModeGate` disables it and says why, and the
    // CLI command stays as the route that does work here.
    renderPage([run("run-1", [bundle])]);

    const dialog = openDialog();
    expect(within(dialog).getByText(m.label_command())).toBeInTheDocument();
    // No run picked yet: the command carries the literal placeholder.
    expect(
      within(dialog).getByText("aconiq export --run-id RUN_ID"),
    ).toBeInTheDocument();

    const button = generateButton(dialog);
    expect(button).not.toBeNull();
    expect(button).toHaveAttribute("aria-disabled", "true");
  });

  it("refuses to generate from the gated button", () => {
    renderPage([run("run-1", [bundle])]);

    const dialog = openDialog();
    selectRun(dialog, "run-1");
    const button = generateButton(dialog);
    if (button) fireEvent.click(button);

    expect(state.createdExports).toHaveLength(0);
  });

  it("substitutes the picked run into the CLI command", () => {
    renderPage([run("run-7", [bundle])]);

    const dialog = openDialog();
    selectRun(dialog, "run-7");

    expect(
      within(dialog).getByText("aconiq export --run-id run-7"),
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByText("aconiq export --run-id RUN_ID"),
    ).toBeNull();
  });

  it("offers a Generate button, and no CLI command, with canExport", () => {
    state.canExport = true;
    renderPage([run("run-1", [bundle])]);

    const dialog = openDialog();
    expect(generateButton(dialog)).toBeInTheDocument();
    expect(within(dialog).queryByText(m.label_command())).toBeNull();
    expect(
      within(dialog).queryByText("aconiq export --run-id RUN_ID"),
    ).toBeNull();
  });

  it("keeps the Generate button disabled until a run is picked", () => {
    state.canExport = true;
    renderPage([run("run-1", [bundle])]);

    const dialog = openDialog();
    expect(generateButton(dialog)).toBeDisabled();

    selectRun(dialog, "run-1");
    expect(generateButton(dialog)).toBeEnabled();
  });

  it("generates for the picked run and closes the dialog", () => {
    state.canExport = true;
    renderPage([run("run-1", [bundle])]);

    const dialog = openDialog();
    selectRun(dialog, "run-1");
    const button = generateButton(dialog);
    expect(button).not.toBeNull();
    if (button) fireEvent.click(button);

    expect(state.createdExports).toEqual(["run-1"]);
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("says it is generating while the mutation is in flight", () => {
    state.canExport = true;
    state.createExportPending = true;
    renderPage([run("run-1", [bundle])]);

    const dialog = openDialog();
    expect(
      within(dialog).getByRole("button", { name: m.status_generating() }),
    ).toBeDisabled();
  });

  it("shows a failed generation as the mutation's own message", () => {
    state.canExport = true;
    state.createExportError = new Error("export failed: disk full");
    renderPage([run("run-1", [bundle])]);

    const dialog = openDialog();
    expect(
      within(dialog).getByText("export failed: disk full"),
    ).toBeInTheDocument();
  });

  it("offers every run in the picker, not only those with exports", () => {
    // The list column filters to runs that carry exports; the dialog does not.
    renderPage([run("run-1", [bundle]), run("run-2", [])]);

    const dialog = openDialog();
    fireEvent.pointerDown(
      within(dialog).getByRole("combobox", { name: m.label_select_run() }),
      { button: 0, ctrlKey: false, pointerType: "mouse" },
    );

    expect(screen.getByRole("option", { name: /run-1/ })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /run-2/ })).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: m.option_select_run_placeholder() }),
    ).toBeInTheDocument();
  });

  it("closes without generating from the Close action", () => {
    state.canExport = true;
    renderPage([run("run-1", [bundle])]);

    const dialog = openDialog();
    // The corner control names what it closes ("Close dialog"), so the footer
    // action is reachable by its own name alone — no two controls in this
    // dialog answer to the same one.
    fireEvent.click(
      within(dialog).getByRole("button", { name: m.action_close() }),
    );

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(state.createdExports).toEqual([]);
  });

  it("gives the corner close a name of its own", () => {
    state.canExport = true;
    renderPage([run("run-1", [bundle])]);

    const dialog = openDialog();

    // Two ways out of the same dialog. Radix' corner control and the footer
    // button used to answer to "Close" alike, which leaves a screen-reader
    // user picking between two identical entries.
    expect(
      within(dialog).getByRole("button", { name: m.action_close_dialog() }),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: m.action_close() }),
    ).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------

describe("ExportPage list meta", () => {
  it("counts a single artifact in the singular", () => {
    renderPage([run("run-1", [bundle])]);

    expect(listItem("run-1")).toHaveTextContent(
      m.msg_artifact_count_one({ count: 1 }),
    );
  });

  it("counts several artifacts in the plural", () => {
    renderPage([run("run-1", [bundle, htmlReport, markdownReport])]);

    expect(listItem("run-1")).toHaveTextContent(
      m.msg_artifact_count_other({ count: 3 }),
    );
  });

  it("counts only the export artifacts of the run", () => {
    renderPage([
      run("run-1", [
        bundle,
        artifact("a-raster", "result.raster", "runs/run-1/results/grid.bin"),
      ]),
    ]);

    expect(listItem("run-1")).toHaveTextContent(
      m.msg_artifact_count_one({ count: 1 }),
    );
  });

  it("times the row from the bundle when one exists", () => {
    renderPage([run("run-1", [bundle, htmlReport])]);

    expect(listItem("run-1")).toHaveTextContent(
      formatDateTime(BUNDLE_CREATED_AT),
    );
  });

  it("falls back to the run's finish time when no bundle was written", () => {
    renderPage([run("run-1", [htmlReport])]);

    expect(listItem("run-1")).toHaveTextContent(
      formatDateTime(RUN_FINISHED_AT),
    );
  });

  it("counts one run with exports in the header in the singular", () => {
    renderPage([run("run-1", [bundle]), run("run-2", [])]);

    expect(pageHeader()).toHaveTextContent(`1 ${m.msg_runs_with_exports()}`);
    expect(pageHeader()).not.toHaveTextContent(
      m.msg_runs_with_exports_plural(),
    );
  });

  it("counts several runs with exports in the header in the plural", () => {
    renderPage([run("run-1", [bundle]), run("run-2", [htmlReport])]);

    expect(pageHeader()).toHaveTextContent(
      `2 ${m.msg_runs_with_exports_plural()}`,
    );
  });
});

// ---------------------------------------------------------------------------

describe("ExportPage detail panel", () => {
  it("lists every export artifact with its label and filename", () => {
    renderPage([
      run("run-1", [bundle, htmlReport, markdownReport, jsonContext]),
    ]);

    expect(
      screen.getByText(m.export_artifact_label_bundle()),
    ).toBeInTheDocument();
    expect(
      screen.getByText(m.export_artifact_label_html_report()),
    ).toBeInTheDocument();
    expect(
      screen.getByText(m.export_artifact_label_markdown_report()),
    ).toBeInTheDocument();
    expect(
      screen.getByText(m.export_artifact_label_json_context()),
    ).toBeInTheDocument();

    // The row shows the basename, with the full path on the title attribute.
    expect(screen.getByText("bundle.zip")).toHaveAttribute(
      "title",
      "exports/run-1/bundle.zip",
    );
    expect(screen.getByText("report-context.json")).toBeInTheDocument();
  });

  it("leaves out artifacts that are not exports", () => {
    renderPage([
      run("run-1", [
        bundle,
        artifact("a-raster", "result.raster", "runs/run-1/results/grid.bin"),
      ]),
    ]);

    expect(screen.queryByText("grid.bin")).toBeNull();
    expect(screen.getByText("bundle.zip")).toBeInTheDocument();
  });

  it("offers Open in browser only for the HTML report", () => {
    const open = vi.fn();
    vi.stubGlobal("open", open);
    renderPage([run("run-1", [bundle, htmlReport, markdownReport])]);

    const buttons = screen.getAllByRole("button", {
      name: m.action_open_in_browser(),
    });
    expect(buttons).toHaveLength(1);

    fireEvent.click(
      screen.getByRole("button", { name: m.action_open_in_browser() }),
    );
    expect(open).toHaveBeenCalledWith(
      "/api/v1/artifacts/a-html/content",
      "_blank",
    );
    vi.unstubAllGlobals();
  });

  it("previews the HTML report in a sandboxed frame", () => {
    renderPage([run("run-1", [bundle, htmlReport])]);

    const frame = screen.getByTitle(m.label_html_report_preview());
    expect(frame).toHaveAttribute("src", "/api/v1/artifacts/a-html/content");
    expect(frame).toHaveAttribute("sandbox", "allow-scripts");
    expect(screen.queryByText(m.msg_no_html_report_yet())).toBeNull();
  });

  it("explains the missing preview when no HTML report was written", () => {
    renderPage([run("run-1", [bundle])]);

    expect(screen.getByText(m.msg_no_html_report_yet())).toBeInTheDocument();
    expect(screen.queryByTitle(m.label_html_report_preview())).toBeNull();
  });

  /** The `CopyField` that shows `text`, so a page with several can be told apart. */
  function copyFieldFor(text: string): HTMLElement {
    const code = screen.getByText(text);
    expect(code.tagName).toBe("CODE");
    const field = code.closest('[data-slot="copy-field"]');
    expect(field).not.toBeNull();
    return field as HTMLElement;
  }

  it("shows the run's export command and copies it verbatim", () => {
    renderPage([run("run-42", [bundle])]);

    expect(screen.getByText(m.section_cli_command())).toBeInTheDocument();
    const field = copyFieldFor("aconiq export --run-id run-42");

    const copy = within(field).getByRole("button", { name: m.action_copy() });
    expect(copy).not.toHaveAttribute("data-copied");
    fireEvent.click(copy);
    expect(writeText).toHaveBeenCalledWith("aconiq export --run-id run-42");
  });

  /*
   * `--pdf` shipped with the CLI and the artifact it writes has been labelled
   * here since the kind table learned `export.report_pdf`. The invitation was
   * the part still missing: the page showed one command, and it was not the
   * one that produces the report most people ask for.
   */
  it("offers the PDF report as its own copyable command", () => {
    renderPage([run("run-42", [bundle])]);

    const field = copyFieldFor("aconiq export --run-id run-42 --pdf");
    expect(
      within(field).getByText(m.label_command_with_pdf()),
    ).toBeInTheDocument();

    fireEvent.click(
      within(field).getByRole("button", { name: m.action_copy() }),
    );
    expect(writeText).toHaveBeenCalledWith(
      "aconiq export --run-id run-42 --pdf",
    );
  });

  it("says a selected run carries no export artifacts", () => {
    // Reachable only through the dialog's picker, which offers every run:
    // generating for a run whose artifacts have not landed yet selects it.
    state.canExport = true;
    state.createExportResult = { id: "run-2" };
    renderPage([run("run-1", [bundle]), run("run-2", [])]);

    const dialog = openDialog();
    selectRun(dialog, "run-2");
    const button = generateButton(dialog);
    expect(button).not.toBeNull();
    if (button) fireEvent.click(button);

    // The title of this test always described this; the body used to assert
    // the opposite, because the selection fell back to the first run with
    // exports. The run is resolved against every run, so its own empty detail
    // is what shows, and no other run's bundle leaks in.
    expect(pathname()).toBe("/export/run-2");
    expect(screen.getByText(m.msg_no_artifacts_for_run())).toBeInTheDocument();
    expect(screen.queryByText("bundle.zip")).toBeNull();
  });

  it("selects another run from the list", () => {
    renderPage([run("run-1", [bundle]), run("run-2", [htmlReport])]);

    expect(listItem("run-1")).toHaveAttribute("aria-current", "page");
    fireEvent.click(listItem("run-2"));

    expect(pathname()).toBe("/export/run-2");
    expect(listItem("run-2")).toHaveAttribute("aria-current", "page");
    expect(listItem("run-1")).not.toHaveAttribute("aria-current");
    expect(
      screen.getByText("aconiq export --run-id run-2"),
    ).toBeInTheDocument();
  });

  it("selects nothing at the bare export route", () => {
    renderPage([run("run-1", [bundle])], "/export");

    expect(listItem("run-1")).not.toHaveAttribute("aria-current");
    expect(
      screen.getByText(m.msg_select_run_for_details()),
    ).toBeInTheDocument();
  });

  it("names an unknown run id rather than falling back to another run", () => {
    renderPage([run("run-1", [bundle])], "/export/nope");

    expect(
      screen.getByText(m.msg_unknown_run_id({ runId: "nope" })),
    ).toBeInTheDocument();
    expect(screen.queryByText("bundle.zip")).toBeNull();
    expect(listItem("run-1")).not.toHaveAttribute("aria-current");
  });

  it("does not call a run unknown while the runs are loading", () => {
    state.runsLoading = true;
    renderPage([], "/export/run-1");

    expect(
      screen.getByRole("heading", { name: m.page_title_exports() }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(m.msg_unknown_run_id({ runId: "run-1" })),
    ).toBeNull();
  });
});

// ---------------------------------------------------------------------------

describe("ExportPage unknown artifact kinds", () => {
  /**
   * `export.report_pdf` is a kind `aconiq export --pdf` really writes, so it
   * belongs in `EXPORT_KIND_LABELS` and carries a translated name like every
   * other known kind. Only a kind the table does not know falls through to
   * `kindMeta`'s fallback and prints its own identifier.
   */
  it("labels a PDF report rather than printing its kind", () => {
    const pdf = artifact(
      "a-pdf",
      "export.report_pdf",
      "exports/run-1/report.pdf",
    );
    renderPage([run("run-1", [bundle, pdf])]);

    expect(
      screen.getByText(m.export_artifact_label_pdf_report()),
    ).toBeInTheDocument();
    expect(screen.queryByText("export.report_pdf")).toBeNull();
    expect(screen.getByText("report.pdf")).toBeInTheDocument();
    // It is still counted and still an export artifact.
    expect(listItem("run-1")).toHaveTextContent(
      m.msg_artifact_count_other({ count: 2 }),
    );
    // And it gets no Open in browser affordance, unlike the HTML report.
    expect(
      screen.queryByRole("button", { name: m.action_open_in_browser() }),
    ).toBeNull();
  });

  /*
   * `export.report_typst` is written on every export that does not pass
   * --skip-report, and `export.assessment_16bimschv_json` on every RLS-19 or
   * Schall 03 run with a model and a receiver table. Both are ordinary output,
   * so neither may print its own identifier at the reader.
   */
  it("labels a Typst report rather than printing its kind", () => {
    const typst = artifact(
      "a-typ",
      "export.report_typst",
      "exports/run-1/report.typ",
    );
    renderPage([run("run-1", [bundle, typst])]);

    expect(
      screen.getByText(m.export_artifact_label_typst_report()),
    ).toBeInTheDocument();
    expect(screen.queryByText("export.report_typst")).toBeNull();
    expect(screen.getByText("report.typ")).toBeInTheDocument();
  });

  it("labels a 16. BImSchV assessment rather than printing its kind", () => {
    const assessment = artifact(
      "a-bim",
      "export.assessment_16bimschv_json",
      "exports/run-1/assessment-16bimschv.json",
    );
    renderPage([run("run-1", [bundle, assessment])]);

    expect(
      screen.getByText(m.export_artifact_label_bimschv16_assessment()),
    ).toBeInTheDocument();
    expect(screen.queryByText("export.assessment_16bimschv_json")).toBeNull();
    expect(screen.getByText("assessment-16bimschv.json")).toBeInTheDocument();
  });

  it("prints the raw kind of any other unknown export kind", () => {
    renderPage([
      run("run-1", [
        artifact("a-x", "export.geotiff", "exports/run-1/grid.tif"),
      ]),
    ]);

    expect(screen.getByText("export.geotiff")).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------

describe("ExportPage empty state", () => {
  it("shows the empty state when no run carries an export artifact", () => {
    renderPage([
      run("run-1", [
        artifact("a-raster", "result.raster", "runs/run-1/results/grid.bin"),
      ]),
    ]);

    expect(
      screen.getByText(m.msg_no_exports_empty_state()),
    ).toBeInTheDocument();
    expect(
      screen.getByText(m.msg_select_run_for_details()),
    ).toBeInTheDocument();
    expect(pageHeader()).toHaveTextContent(
      `0 ${m.msg_runs_with_exports_plural()}`,
    );
  });

  it("shows the empty state when there are no runs at all", () => {
    renderPage([]);

    expect(
      screen.getByText(m.msg_no_exports_empty_state()),
    ).toBeInTheDocument();
    expect(
      screen.getByText(m.msg_select_run_for_details()),
    ).toBeInTheDocument();
    // The dialog is still reachable, so the CLI route stays available.
    const dialog = openDialog();
    expect(within(dialog).getByText(m.label_command())).toBeInTheDocument();
  });
});

// ---------------------------------------------------------------------------

describe("ExportPage transient states keep the page header", () => {
  /**
   * `export.tsx` carries a comment that the `PageHeader` must stay above both
   * the loading and the error state, because `waitForPage` in `e2e/app.ts`
   * waits for the page's heading. Nothing else guards it.
   */
  it("keeps the heading above the loading spinner", () => {
    state.runsLoading = true;
    state.runs = undefined;
    renderPage([]);

    const heading = screen.getByRole("heading", {
      name: m.page_title_exports(),
    });
    expect(heading).toBeInTheDocument();
    expect(screen.queryByText(m.msg_api_error_export())).toBeNull();

    const spinner = document.querySelector(".animate-spin");
    expect(spinner).not.toBeNull();
    if (spinner) {
      expect(
        heading.compareDocumentPosition(spinner) &
          Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
    }
  });

  it("keeps the heading above the error callout", () => {
    state.runsError = new Error("Request failed: 500");
    state.runs = undefined;
    renderPage([]);

    const heading = screen.getByRole("heading", {
      name: m.page_title_exports(),
    });
    const callout = screen.getByText(m.msg_api_error_export());
    expect(
      heading.compareDocumentPosition(callout) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("reports the API failure rather than the raw error", () => {
    state.runsError = new Error("Request failed: 500");
    state.runs = undefined;
    renderPage([]);

    expect(screen.getByText(m.msg_api_error_export())).toBeInTheDocument();
    expect(screen.queryByText("Request failed: 500")).toBeNull();
    // Neither the list nor the New Export action is offered while it fails.
    expect(
      screen.queryByRole("button", { name: m.action_new_export() }),
    ).toBeNull();
  });

  it("prefers the error state over the spinner when both are set", () => {
    state.runsLoading = true;
    state.runsError = new Error("Request failed: 500");
    state.runs = undefined;
    renderPage([]);

    expect(screen.getByText(m.msg_api_error_export())).toBeInTheDocument();
    expect(document.querySelector(".animate-spin")).toBeNull();
  });
});

// ---------------------------------------------------------------------------

describe("ExportPage heading order", () => {
  function headingLevels(): number[] {
    return screen
      .getAllByRole("heading")
      .map((h) => Number(h.tagName.slice(1)));
  }

  function expectContiguous(levels: number[]) {
    // The shell's h1 sits above this page, so a level of 2 is the entry.
    let deepest = 1;
    for (const level of levels) {
      expect(level).toBeLessThanOrEqual(deepest + 1);
      deepest = Math.max(deepest, level);
    }
  }

  it("keeps heading levels contiguous with a run selected", () => {
    renderPage([run("run-1", [bundle, htmlReport])]);

    const levels = headingLevels();
    expect(levels).toContain(3);
    expectContiguous(levels);
  });

  it("keeps heading levels contiguous in the empty state", () => {
    renderPage([]);
    expectContiguous(headingLevels());
  });

  it("keeps heading levels contiguous while loading", () => {
    state.runsLoading = true;
    state.runs = undefined;
    renderPage([]);
    expectContiguous(headingLevels());
  });

  it("keeps heading levels contiguous in the error state", () => {
    state.runsError = new Error("Request failed: 500");
    state.runs = undefined;
    renderPage([]);
    expectContiguous(headingLevels());
  });
});
