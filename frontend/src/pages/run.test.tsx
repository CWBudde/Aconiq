import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type {
  ParameterDefinition,
  ProfileInfo,
  RunSummary,
  StandardDescriptor,
} from "@/api/client";
import {
  APIRequestError,
  ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED,
} from "@/api/api-error";
import RunPage from "./run";
import { useModelStore } from "@/model/model-store";
import { resetProjectSyncStore } from "@/model/use-project-sync";
import type { CalcArea, ModelFeature } from "@/model/types";
import { m } from "@/i18n/messages";

/**
 * The run dialog is where a user commits to a standard, so it is the last
 * place the evidence tier can still be missed. These cover the tier shown for
 * the pre-selected standard, the scaffold warning that must sit next to the
 * run action, the older backend that sends no tier at all, and the deliberate
 * acknowledgement a scaffold-tier standard needs before it may be run.
 */

const state = vi.hoisted(() => {
  const value: {
    standards: unknown[];
    runs: unknown[];
    runSpecs: Record<string, unknown>[];
    createRunError: Error | null;
    runsAgainstSavedModel: boolean;
    savedModels: unknown[];
    logLines: string[];
  } = {
    standards: [],
    runs: [],
    runSpecs: [],
    createRunError: null,
    runsAgainstSavedModel: true,
    savedModels: [],
    logLines: [],
  };
  return value;
});

// Explicit capabilities rather than whatever the env selects: the dialog's
// receiver messaging, submit guard and unsaved-changes gate branch on them.
vi.mock("@/api/backend", () => ({
  backend: {
    get capabilities() {
      return {
        kind: state.runsAgainstSavedModel ? "http" : "browser",
        canExport: false,
        runsAgainstSavedModel: state.runsAgainstSavedModel,
        runsChangeExternally: state.runsAgainstSavedModel,
      };
    },
  },
}));

vi.mock("@/api/hooks", () => ({
  useStandards: () => ({
    data: state.standards,
    isLoading: false,
    error: null,
  }),
  useRuns: () => ({ data: state.runs, isLoading: false, error: null }),
  useRunLog: () => ({
    data: { run_id: "run-1", lines: state.logLines },
    isLoading: false,
  }),
  useCreateRun: () => ({
    mutate: (spec: Record<string, unknown>) => {
      state.runSpecs.push(spec);
    },
    isPending: false,
    isError: state.createRunError !== null,
    error: state.createRunError,
  }),
  // A project is always loaded here; `useProjectSync` runs for real on top
  // of these so the gate is tested through the hook, not around it.
  useProjectStatus: () => ({
    data: { name: "Demo", crs: "EPSG:25832", scenario_count: 1, run_count: 0 },
    isLoading: false,
    isError: false,
    error: null,
  }),
  useSaveModel: () => ({
    mutateAsync: (req: unknown) => {
      state.savedModels.push(req);
      return Promise.resolve({ featureCount: 0, warnings: [] });
    },
  }),
  useIsSavingModel: () => false,
}));

const sampleFeature: ModelFeature = {
  id: "s1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

function standard(
  id: string,
  evidenceTier: string | undefined,
): StandardDescriptor {
  return {
    id,
    description: `${id} description`,
    default_version: "1",
    ...(evidenceTier === undefined ? {} : { evidence_tier: evidenceTier }),
    versions: [
      {
        name: "1",
        default_profile: "default",
        profiles: [
          {
            name: "default",
            supported_source_types: ["point"],
            supported_indicators: ["Lden"],
            parameters: [],
          },
        ],
      },
    ],
  };
}

function openRunDialog(standards: StandardDescriptor[]) {
  state.standards = standards;
  render(<RunPage />);
  fireEvent.click(screen.getByRole("button", { name: m.action_new_run() }));
}

function startRunButton(): HTMLElement {
  return screen.getByRole("button", { name: m.action_start_run() });
}

function acknowledgementCheckbox(): HTMLElement {
  return screen.getByLabelText(m.label_experimental_opt_in());
}

/**
 * Radix' Select opens on a pointer event and measures the viewport; jsdom
 * implements neither. These shims are what let a test change the standard the
 * way a user does, rather than reaching past the control.
 */
function selectStandard(id: string) {
  fireEvent.pointerDown(
    screen.getByRole("combobox", { name: m.label_standard() }),
    { button: 0, ctrlKey: false, pointerType: "mouse" },
  );
  fireEvent.click(screen.getByRole("option", { name: new RegExp(id) }));
}

/**
 * The same shims for the version and profile selects, whose options carry no
 * badge and so can be matched by their exact name.
 */
function selectOption(comboboxLabel: string, option: string) {
  fireEvent.pointerDown(screen.getByRole("combobox", { name: comboboxLabel }), {
    button: 0,
    ctrlKey: false,
    pointerType: "mouse",
  });
  fireEvent.click(screen.getByRole("option", { name: option }));
}

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
});

beforeEach(() => {
  state.standards = [];
  state.runs = [];
  state.runSpecs = [];
  state.createRunError = null;
  state.runsAgainstSavedModel = true;
  state.savedModels = [];
  state.logLines = [];
  useModelStore.getState().reset();
  resetProjectSyncStore();
});

describe("RunPage evidence tiers", () => {
  it("shows the tier of the pre-selected standard", () => {
    openRunDialog([standard("cnossos-road", "scaffold")]);

    const badges = screen.getAllByTestId("evidence-tier-badge");
    expect(badges.length).toBeGreaterThan(0);
    for (const badge of badges) {
      expect(badge).toHaveAttribute("data-tier", "scaffold");
      expect(badge).toHaveTextContent(m.evidence_tier_scaffold());
    }
  });

  it("warns next to the run action when a scaffold standard is selected", () => {
    openRunDialog([standard("cnossos-road", "scaffold")]);

    expect(screen.getByRole("alert")).toHaveTextContent(
      m.msg_evidence_tier_scaffold_warning(),
    );
  });

  it("does not warn for a normative standard", () => {
    openRunDialog([standard("rls19-road", "normative")]);

    expect(screen.queryByRole("alert")).toBeNull();
    expect(
      screen.queryByText(m.msg_evidence_tier_scaffold_warning()),
    ).toBeNull();
    expect(screen.getAllByTestId("evidence-tier-badge")[0]).toHaveAttribute(
      "data-tier",
      "normative",
    );
  });

  it("renders the dialog unchanged when the backend sends no tier", () => {
    openRunDialog([standard("rls19-road", undefined)]);

    expect(screen.queryByTestId("evidence-tier-badge")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
    // The standard itself is still offered, just without a tier claim.
    expect(screen.getByText("rls19-road description")).toBeInTheDocument();
  });

  it("shows a neutral badge and no warning for an unrecognised tier", () => {
    openRunDialog([standard("future-standard", "provisional")]);

    expect(screen.getAllByTestId("evidence-tier-badge")[0]).toHaveAttribute(
      "data-tier",
      "unknown",
    );
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

describe("RunPage experimental opt-in", () => {
  it("asks for an acknowledgement only for a scaffold standard", () => {
    openRunDialog([standard("cnossos-road", "scaffold")]);

    const checkbox = acknowledgementCheckbox();
    expect(checkbox).toBeInTheDocument();
    expect(checkbox).not.toBeChecked();
    expect(
      screen.getByText(m.msg_experimental_opt_in_help()),
    ).toBeInTheDocument();
  });

  it("asks for no acknowledgement for a normative standard", () => {
    openRunDialog([standard("rls19-road", "normative")]);

    expect(screen.queryByLabelText(m.label_experimental_opt_in())).toBeNull();
    expect(startRunButton()).toBeEnabled();
  });

  it("keeps the run action disabled until the acknowledgement is ticked", () => {
    openRunDialog([standard("cnossos-road", "scaffold")]);

    expect(startRunButton()).toBeDisabled();
    expect(startRunButton()).toHaveAttribute(
      "title",
      m.tooltip_experimental_opt_in_required(),
    );

    fireEvent.click(acknowledgementCheckbox());

    expect(acknowledgementCheckbox()).toBeChecked();
    expect(startRunButton()).toBeEnabled();
  });

  it("starts no run while the acknowledgement is missing", () => {
    openRunDialog([standard("cnossos-road", "scaffold")]);

    fireEvent.click(startRunButton());

    expect(state.runSpecs).toHaveLength(0);
  });

  it("sends experimental: true once the scaffold tier is acknowledged", () => {
    openRunDialog([standard("cnossos-road", "scaffold")]);

    fireEvent.click(acknowledgementCheckbox());
    fireEvent.click(startRunButton());

    expect(state.runSpecs).toHaveLength(1);
    expect(state.runSpecs[0]).toMatchObject({
      standardId: "cnossos-road",
      experimental: true,
    });
  });

  it("never sends the flag for a normative standard", () => {
    openRunDialog([standard("rls19-road", "normative")]);

    fireEvent.click(startRunButton());

    expect(state.runSpecs).toHaveLength(1);
    expect(state.runSpecs[0]).not.toHaveProperty("experimental");
  });

  it("asks again after the standard is switched away and back", () => {
    openRunDialog([
      standard("cnossos-road", "scaffold"),
      standard("rls19-road", "normative"),
    ]);

    fireEvent.click(acknowledgementCheckbox());
    expect(startRunButton()).toBeEnabled();

    selectStandard("rls19-road");
    expect(screen.queryByLabelText(m.label_experimental_opt_in())).toBeNull();

    selectStandard("cnossos-road");
    expect(acknowledgementCheckbox()).not.toBeChecked();
    expect(startRunButton()).toBeDisabled();
  });

  it("carries no acknowledgement over to a different standard", () => {
    openRunDialog([
      standard("cnossos-road", "scaffold"),
      standard("cnossos-rail", "scaffold"),
    ]);

    fireEvent.click(acknowledgementCheckbox());
    selectStandard("cnossos-rail");

    expect(acknowledgementCheckbox()).not.toBeChecked();
    expect(startRunButton()).toBeDisabled();
  });
});

describe("RunPage run creation errors", () => {
  it("shows the server's message and hint when the opt-in is refused", () => {
    state.createRunError = new APIRequestError({
      code: ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED,
      message: 'standard "cnossos-road" is evidence tier "scaffold"',
      hint: 'Set "experimental": true in the request body.',
      details: { standard_id: "cnossos-road", evidence_tier: "scaffold" },
    });
    openRunDialog([standard("cnossos-road", "scaffold")]);

    const alert = screen.getByTestId("run-create-error");
    expect(alert).toHaveAttribute(
      "data-error-code",
      ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED,
    );
    expect(alert).toHaveTextContent(m.msg_experimental_opt_in_required_error());
    expect(alert).toHaveTextContent(
      'standard "cnossos-road" is evidence tier "scaffold"',
    );
    expect(alert).toHaveTextContent(
      'Set "experimental": true in the request body.',
    );
  });

  it("shows a plain failure without inventing an opt-in explanation", () => {
    state.createRunError = new Error("Request failed: 500");
    openRunDialog([standard("rls19-road", "normative")]);

    const alert = screen.getByTestId("run-create-error");
    expect(alert).not.toHaveAttribute("data-error-code");
    expect(alert).toHaveTextContent("Request failed: 500");
    expect(alert).not.toHaveTextContent(
      m.msg_experimental_opt_in_required_error(),
    );
  });
});

describe("RunPage unsaved changes", () => {
  function unsavedCallout(): HTMLElement | null {
    return screen.queryByTestId("unsaved-changes-callout");
  }

  it("gates the run action while the workspace differs from the project", () => {
    useModelStore.getState().addFeature(sampleFeature);
    openRunDialog([standard("rls19-road", "normative")]);

    expect(unsavedCallout()).toHaveTextContent(
      m.msg_unsaved_changes_before_run(),
    );
    expect(startRunButton()).toBeDisabled();
    fireEvent.click(startRunButton());
    expect(state.runSpecs).toEqual([]);
  });

  it("does not gate a clean workspace", () => {
    openRunDialog([standard("rls19-road", "normative")]);

    expect(unsavedCallout()).toBeNull();
    expect(startRunButton()).toBeEnabled();
  });

  it("does not gate in browser mode, where runs read the store directly", () => {
    state.runsAgainstSavedModel = false;
    useModelStore.getState().addFeature(sampleFeature);
    openRunDialog([standard("rls19-road", "normative")]);

    expect(unsavedCallout()).toBeNull();
    expect(startRunButton()).toBeEnabled();
  });

  it("saves from the callout and then lets the run start", async () => {
    useModelStore.getState().addFeature(sampleFeature);
    openRunDialog([standard("rls19-road", "normative")]);

    fireEvent.click(
      screen.getByRole("button", { name: m.action_save_to_project() }),
    );

    expect(await screen.findByText(m.action_start_run())).toBeEnabled();
    expect(state.savedModels).toHaveLength(1);
    expect(state.savedModels[0]).toMatchObject({ crs: "EPSG:4326" });
    expect(useModelStore.getState().dirty).toBe(false);
    expect(unsavedCallout()).toBeNull();
  });
});

describe("RunPage calculation area", () => {
  const calcArea: CalcArea = {
    geometry: {
      type: "Polygon",
      coordinates: [
        [
          [10, 51],
          [10.1, 51],
          [10.1, 51.1],
          [10, 51.1],
          [10, 51],
        ],
      ],
    },
  };

  it("says the area is not in the project where runs read the saved model", () => {
    useModelStore.getState().setCalcArea(calcArea);
    useModelStore.getState().markClean();
    openRunDialog([standard("rls19-road", "normative")]);

    expect(screen.getByTestId("calc-area-not-in-project")).toHaveTextContent(
      m.msg_calc_area_not_in_project(),
    );
    expect(screen.queryByText(m.msg_calc_area_active())).toBeNull();
    expect(startRunButton()).toBeEnabled();
  });

  it("reports the area as active in browser mode, where the grid honours it", () => {
    state.runsAgainstSavedModel = false;
    useModelStore.getState().setCalcArea(calcArea);
    openRunDialog([standard("rls19-road", "normative")]);

    expect(screen.getByText(m.msg_calc_area_active())).toBeInTheDocument();
    expect(screen.queryByTestId("calc-area-not-in-project")).toBeNull();
  });
});

describe("RunPage heading order", () => {
  const completedRun: RunSummary = {
    id: "run-1",
    scenario_id: "default",
    standard_id: "rls19-road",
    version: "1",
    profile: "default",
    status: "completed",
    started_at: "2026-01-01T10:00:00Z",
    finished_at: "2026-01-01T10:00:05Z",
    log_path: "runs/run-1/run.log",
    artifacts: [],
  };

  it("keeps heading levels contiguous with a run selected", () => {
    // Only a selected run renders the detail panel, and that is where the
    // levels once skipped from the page's h2 to h4 (axe `heading-order`).
    // The shell's h1 sits above this page, so a level of 2 is the entry.
    state.runs = [completedRun];
    state.standards = [standard("rls19-road", "normative")];
    render(<RunPage />);

    const levels = screen
      .getAllByRole("heading")
      .map((h) => Number(h.tagName.slice(1)));
    expect(levels).toContain(3);
    let deepest = 1;
    for (const level of levels) {
      expect(level).toBeLessThanOrEqual(deepest + 1);
      deepest = Math.max(deepest, level);
    }
  });
});

/**
 * The progress timeline is parsed out of the raw run log by a pure function
 * the page does not export, so these drive it through `RunDetail` — the only
 * caller — with log lines shaped like the ones the CLI actually writes
 * (`backend/internal/app/cli/run_pipeline.go`, `run_modules.go`). Asserting on
 * the rendered steps rather than on the parser keeps them true when the parser
 * moves into a module of its own.
 */
describe("RunPage progress timeline", () => {
  // RFC3339 stamp plus the payload, exactly as `runLog.addf` emits it.
  const LOG_RUN_STARTED = "2026-01-01T10:00:00Z run started";
  const LOG_MODEL = "2026-01-01T10:00:00Z model=model/normalized.geojson";
  const LOG_SOURCES = "2026-01-01T10:00:01Z sources=12";
  const LOG_RECEIVERS = "2026-01-01T10:00:01Z receivers=400 grid=20x20";
  const LOG_COMPUTE = "2026-01-01T10:00:02Z stage=compute chunk=0 1/4";
  const LOG_OUTPUT_HASH = "2026-01-01T10:00:04Z output_hash=9f2c1b";
  const LOG_PERSISTED =
    "2026-01-01T10:00:04Z persisted=runs/run-1/results/run-summary.json";
  const LOG_COMPLETED = "2026-01-01T10:00:05Z run completed";
  const LOG_FAILED = "2026-01-01T10:00:05Z run failed";

  type StepState = "done" | "active" | "pending";

  function stepLabels(): string[] {
    return [
      m.timeline_run_started(),
      m.timeline_loading_model(),
      m.timeline_extracting_sources(),
      m.timeline_building_receivers(),
      m.timeline_computing(),
      m.timeline_persisting_outputs(),
      m.timeline_finalised(),
    ];
  }

  function progressSection(): HTMLElement {
    const section = screen.getByText(m.section_progress()).closest("section");
    if (!section) throw new Error("no progress section rendered");
    return section;
  }

  /** The step rows in the order they are painted. */
  function renderedLabels(): string[] {
    const known = new Set(stepLabels());
    return Array.from(progressSection().querySelectorAll("p"))
      .map((p) => p.textContent)
      .filter((text) => known.has(text));
  }

  function stepRow(label: string): HTMLElement {
    const row = screen.getByText(label).parentElement?.parentElement;
    if (!row) throw new Error(`no timeline row for "${label}"`);
    return row;
  }

  /**
   * A step's state is only ever shown as a shape and a colour — a tick, a
   * spinner, or a plain dot — and every one of those is `aria-hidden`, so the
   * marker has to be read the way the eye reads it.
   */
  function stepState(label: string): StepState {
    const icon = stepRow(label).querySelector("svg");
    if (!icon) return "pending";
    return icon.classList.contains("animate-spin") ? "active" : "done";
  }

  function stepStates(): StepState[] {
    return stepLabels().map(stepState);
  }

  function stepTimestamp(label: string): string | null {
    const paragraphs = stepRow(label).querySelectorAll("p");
    return paragraphs.length > 1 ? (paragraphs[1]?.textContent ?? null) : null;
  }

  function showRun(status: RunSummary["status"], lines: string[]) {
    state.logLines = lines;
    state.standards = [standard("rls19-road", "normative")];
    state.runs = [
      {
        id: "run-1",
        scenario_id: "default",
        standard_id: "rls19-road",
        version: "1",
        profile: "default",
        status,
        started_at: "2026-01-01T10:00:00Z",
        finished_at: "2026-01-01T10:00:05Z",
        log_path: "runs/run-1/run.log",
        artifacts: [],
      } satisfies RunSummary,
    ];
    render(<RunPage />);
  }

  it("lists the seven pipeline steps in order whatever the log holds", () => {
    showRun("running", []);

    expect(renderedLabels()).toEqual(stepLabels());
  });

  it("marks only the first step active for an empty log", () => {
    showRun("running", []);

    expect(stepStates()).toEqual([
      "active",
      "pending",
      "pending",
      "pending",
      "pending",
      "pending",
      "pending",
    ]);
    expect(stepTimestamp(m.timeline_run_started())).toBeNull();
  });

  it("marks the matched stages done and the next one active mid-run", () => {
    showRun("running", [LOG_RUN_STARTED, LOG_MODEL, LOG_SOURCES]);

    expect(stepStates()).toEqual([
      "done",
      "done",
      "done",
      "active",
      "pending",
      "pending",
      "pending",
    ]);
  });

  it("stamps a matched step with the first log line that matched it", () => {
    showRun("running", [LOG_RUN_STARTED, LOG_MODEL, LOG_SOURCES]);

    // The raw RFC3339 stamp, sliced from the line rather than formatted.
    expect(stepTimestamp(m.timeline_run_started())).toBe(
      "2026-01-01T10:00:00Z",
    );
    expect(stepTimestamp(m.timeline_extracting_sources())).toBe(
      "2026-01-01T10:00:01Z",
    );
    expect(stepTimestamp(m.timeline_building_receivers())).toBeNull();
  });

  it("marks every step done and none active for a completed run", () => {
    showRun("completed", [
      LOG_RUN_STARTED,
      LOG_MODEL,
      LOG_SOURCES,
      LOG_RECEIVERS,
      LOG_COMPUTE,
      LOG_OUTPUT_HASH,
      LOG_PERSISTED,
      LOG_COMPLETED,
    ]);

    expect(stepStates()).toEqual([
      "done",
      "done",
      "done",
      "done",
      "done",
      "done",
      "done",
    ]);
  });

  it("leaves the stage a failed run never reached pending, and none active", () => {
    // The log stops at the compute stage, so nothing was persisted — but the
    // closing "run failed" line still ticks the last step. A step later than
    // an unreached one can therefore read as done.
    showRun("failed", [
      LOG_RUN_STARTED,
      LOG_MODEL,
      LOG_SOURCES,
      LOG_RECEIVERS,
      LOG_COMPUTE,
      LOG_FAILED,
    ]);

    expect(stepStates()).toEqual([
      "done",
      "done",
      "done",
      "done",
      "done",
      "pending",
      "done",
    ]);
    expect(stepStates()).not.toContain("active");
  });
});

/**
 * Standard → version → profile → parameters is a cascade: each choice resets
 * everything below it, and the parameter values are seeded from the selected
 * profile's own defaults. The reset handlers and the render-phase seeding are
 * the subtlest part of the dialog, so they are pinned here before they move
 * into a hook.
 */
describe("RunPage standard cascade", () => {
  function profileWith(
    name: string,
    parameters: ParameterDefinition[],
  ): ProfileInfo {
    return {
      name,
      supported_source_types: ["point"],
      supported_indicators: ["Lden"],
      parameters,
    };
  }

  const standardA: StandardDescriptor = {
    id: "rls19-road",
    description: "rls19-road description",
    default_version: "2020",
    evidence_tier: "normative",
    versions: [
      {
        name: "2020",
        default_profile: "urban",
        profiles: [
          profileWith("urban", [
            {
              name: "grid_spacing",
              kind: "string",
              required: false,
              default_value: "10",
            },
          ]),
          profileWith("rural", [
            {
              name: "grid_spacing",
              kind: "string",
              required: false,
              default_value: "25",
            },
            // No default: the seeding has to fall back to the empty string.
            { name: "wind_correction", kind: "string", required: false },
          ]),
        ],
      },
      {
        name: "2019",
        default_profile: "legacy",
        profiles: [
          profileWith("legacy", [
            {
              name: "grid_spacing",
              kind: "string",
              required: false,
              default_value: "50",
            },
          ]),
        ],
      },
    ],
  };

  const standardB: StandardDescriptor = {
    id: "schall03",
    description: "schall03 description",
    default_version: "2014",
    evidence_tier: "normative",
    versions: [
      {
        name: "2014",
        default_profile: "standard",
        profiles: [
          profileWith("standard", [
            {
              name: "grid_spacing",
              kind: "string",
              required: false,
              default_value: "200",
            },
          ]),
        ],
      },
    ],
  };

  function versionTrigger(): HTMLElement {
    return screen.getByRole("combobox", { name: m.label_version() });
  }

  function profileTrigger(): HTMLElement {
    return screen.getByRole("combobox", { name: m.label_profile() });
  }

  function parameter(name: string): HTMLElement {
    return screen.getByLabelText(name);
  }

  it("seeds the parameters from the default profile when the dialog opens", () => {
    openRunDialog([standardA]);

    expect(versionTrigger()).toHaveTextContent("2020");
    expect(profileTrigger()).toHaveTextContent("urban");
    expect(parameter("grid_spacing")).toHaveValue("10");
  });

  it("resets version, profile and parameters when the standard changes", () => {
    openRunDialog([standardA, standardB]);

    selectOption(m.label_version(), "2019");
    expect(profileTrigger()).toHaveTextContent("legacy");
    expect(parameter("grid_spacing")).toHaveValue("50");

    selectStandard("schall03");

    expect(versionTrigger()).toHaveTextContent("2014");
    expect(profileTrigger()).toHaveTextContent("standard");
    expect(parameter("grid_spacing")).toHaveValue("200");
  });

  it("returns a re-selected standard to its own defaults, not the last choice", () => {
    openRunDialog([standardA, standardB]);

    selectOption(m.label_version(), "2019");
    selectStandard("schall03");
    selectStandard("rls19-road");

    expect(versionTrigger()).toHaveTextContent("2020");
    expect(profileTrigger()).toHaveTextContent("urban");
    expect(parameter("grid_spacing")).toHaveValue("10");
  });

  it("resets profile and parameters when the version changes", () => {
    openRunDialog([standardA]);

    selectOption(m.label_profile(), "rural");
    expect(parameter("grid_spacing")).toHaveValue("25");

    selectOption(m.label_version(), "2019");

    expect(profileTrigger()).toHaveTextContent("legacy");
    expect(parameter("grid_spacing")).toHaveValue("50");
  });

  it("reseeds the parameters from the profile's own default values", () => {
    openRunDialog([standardA]);

    selectOption(m.label_profile(), "rural");

    expect(parameter("grid_spacing")).toHaveValue("25");
    // A parameter the profile declares without a default seeds as empty
    // rather than being left out of the form.
    expect(parameter("wind_correction")).toHaveValue("");
  });

  it("drops the previous profile's fields rather than carrying them over", () => {
    openRunDialog([standardA]);

    selectOption(m.label_profile(), "rural");
    expect(screen.queryByLabelText("wind_correction")).not.toBeNull();

    selectOption(m.label_profile(), "urban");

    expect(screen.queryByLabelText("wind_correction")).toBeNull();
    expect(parameter("grid_spacing")).toHaveValue("10");
  });

  /**
   * Every value a field held while `run` ran, in order — including the ones
   * that lived for a single commit. React writes a controlled input's value to
   * the `value` attribute as well as to the property, so each mutation record
   * carries what the field showed before that write.
   */
  function valuesShownDuring(field: HTMLElement, run: () => void): string[] {
    const observer = new MutationObserver(() => {
      /* records are drained by hand, not on the microtask */
    });
    observer.observe(field, {
      attributes: true,
      attributeFilter: ["value"],
      attributeOldValue: true,
    });
    run();
    const shown = observer.takeRecords().map((r) => r.oldValue ?? "");
    observer.disconnect();
    shown.push((field as HTMLInputElement).value);
    return shown;
  }

  it("shows the new profile's defaults in the same commit as the switch", () => {
    // The seeding is a render-phase `setParams` guarded by the profile key, so
    // the switch and the new values land in one commit and the field is never
    // painted empty. Deferring it to an effect would split that in two: the
    // `setParams({})` in the reset handler would commit first, showing "", and
    // only the following commit would carry the defaults. Both end in the same
    // final value, so it is the sequence that has to be asserted.
    openRunDialog([standardA]);

    fireEvent.change(parameter("grid_spacing"), { target: { value: "999" } });
    expect(parameter("grid_spacing")).toHaveValue("999");

    const shown = valuesShownDuring(parameter("grid_spacing"), () => {
      selectOption(m.label_profile(), "rural");
    });

    expect(shown).toEqual(["999", "25"]);
  });

  it("sends the reseeded parameters, not the edited ones, with the run", () => {
    openRunDialog([standardA]);

    fireEvent.change(parameter("grid_spacing"), { target: { value: "999" } });
    selectOption(m.label_profile(), "rural");
    fireEvent.click(startRunButton());

    expect(state.runSpecs).toHaveLength(1);
    expect(state.runSpecs[0]).toMatchObject({
      standardId: "rls19-road",
      version: "2020",
      profile: "rural",
      params: { grid_spacing: "25", wind_correction: "" },
    });
  });
});
