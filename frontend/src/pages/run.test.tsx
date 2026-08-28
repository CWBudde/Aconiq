import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type { StandardDescriptor } from "@/api/client";
import {
  APIRequestError,
  ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED,
} from "@/api/api-error";
import RunPage from "./run";
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
    runSpecs: Record<string, unknown>[];
    createRunError: Error | null;
  } = { standards: [], runSpecs: [], createRunError: null };
  return value;
});

vi.mock("@/api/hooks", () => ({
  useStandards: () => ({
    data: state.standards,
    isLoading: false,
    error: null,
  }),
  useRuns: () => ({ data: [], isLoading: false, error: null }),
  useRunLog: () => ({ data: { run_id: "", lines: [] }, isLoading: false }),
  useCreateRun: () => ({
    mutate: (spec: Record<string, unknown>) => {
      state.runSpecs.push(spec);
    },
    isPending: false,
    isError: state.createRunError !== null,
    error: state.createRunError,
  }),
}));

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
  state.runSpecs = [];
  state.createRunError = null;
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
