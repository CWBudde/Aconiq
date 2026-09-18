import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import ProjectPage from "./project";
import { useModelStore } from "@/model/model-store";
import type { ModelFeature, ModelReceiver } from "@/model/types";
import { m } from "@/i18n/messages";

type Query = {
  isLoading: boolean;
  isError: boolean;
  error: null | { message: string };
  data: unknown;
};

const idle: Query = {
  isLoading: false,
  isError: false,
  error: null,
  data: null,
};

const state = vi.hoisted(() => ({
  health: {
    isLoading: false,
    isError: false,
    error: null as null | { message: string },
    data: null as unknown,
  },
  project: {
    isLoading: false,
    isError: false,
    error: null as null | { message: string },
    data: null as unknown,
  },
}));

vi.mock("@/api/hooks", () => ({
  useHealth: () => state.health,
  useProjectStatus: () => state.project,
}));

const source: ModelFeature = {
  id: "src-1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

const badReceiver: ModelReceiver = {
  id: "rcv-1",
  heightM: -1,
  geometry: { type: "Point", coordinates: [10, 51] },
};

function renderPage() {
  render(
    <MemoryRouter>
      <ProjectPage />
    </MemoryRouter>,
  );
}

/** Heading levels in document order. */
function headingLevels(): number[] {
  return screen.getAllByRole("heading").map((h) => Number(h.tagName.slice(1)));
}

beforeEach(() => {
  useModelStore.getState().reset();
  state.health = { ...idle };
  state.project = { ...idle };
});

describe("ProjectPage", () => {
  it("offers both ways in", () => {
    renderPage();

    expect(
      screen.getByRole("link", { name: m.action_start_import() }),
    ).toHaveAttribute("href", "/import");
    expect(
      screen.getByRole("link", { name: m.action_start_drawing() }),
    ).toHaveAttribute("href", "/model?draw=1");
  });

  it("shows the project's facts once they arrive", () => {
    state.project = {
      ...idle,
      data: {
        name: "Demo Project",
        crs: "EPSG:25832",
        scenario_count: 2,
        run_count: 7,
      },
    };
    renderPage();

    expect(screen.getByText("Demo Project")).toBeInTheDocument();
    expect(screen.getByText("EPSG:25832")).toBeInTheDocument();
  });

  it("offers the importer when there is no project yet", () => {
    renderPage();

    expect(screen.getByText(m.msg_no_project_yet())).toBeInTheDocument();
    expect(
      screen.getAllByRole("link", { name: m.nav_import() }).length,
    ).toBeGreaterThan(0);
  });

  it("surfaces a health error rather than an empty panel", () => {
    state.health = {
      ...idle,
      isError: true,
      error: { message: "Request failed: 500" },
    };
    renderPage();

    expect(screen.getByText("Request failed: 500")).toBeInTheDocument();
  });
});

describe("ProjectPage validation summary", () => {
  it("says there is nothing to validate yet, not that there is one error", () => {
    // `validateProjectModel` pushes a synthetic `model.empty` error. Reported
    // as a finding, a fresh install would read "1 error" for having nothing in
    // it yet.
    renderPage();

    expect(
      screen.getByText(m.msg_validation_nothing_yet()),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(m.msg_validation_error_count({ count: 1 })),
    ).toBeNull();
  });

  it("reports a well-formed model as valid", () => {
    useModelStore
      .getState()
      .loadModel({ features: [source], receivers: [], calcArea: null });
    renderPage();

    expect(screen.getByText(m.msg_model_valid())).toBeInTheDocument();
  });

  it("counts a defect that only a receiver carries", () => {
    // The summary reads the receivers. A receiver-blind report would call this
    // model valid and nothing would fail.
    useModelStore
      .getState()
      .loadModel({ features: [source], receivers: [], calcArea: null });
    useModelStore.getState().addReceiver(badReceiver);
    renderPage();

    expect(screen.queryByText(m.msg_model_valid())).toBeNull();
    expect(screen.getByRole("link", { name: m.nav_model() })).toHaveAttribute(
      "href",
      "/model",
    );
  });
});

describe("ProjectPage heading contract", () => {
  /**
   * `waitForPage` in `e2e/app.ts` waits for a header `h1` and an `h2` or `h3`
   * inside `main`. The shell owns the `h1`; if this page's `h2` ever moved
   * inside a query branch, a backend that is down would hang every e2e spec on
   * this route for the full Playwright timeout instead of failing legibly.
   */
  const healthStates: [string, Query][] = [
    ["loading", { ...idle, isLoading: true }],
    ["error", { ...idle, isError: true, error: { message: "boom" } }],
    [
      "ok",
      {
        ...idle,
        data: { status: "ok", version: "1", time: "2026-01-01T00:00:00Z" },
      },
    ],
  ];
  const projectStates: [string, Query][] = [
    ["loading", { ...idle, isLoading: true }],
    ["error", { ...idle, isError: true, error: { message: "boom" } }],
    ["absent", { ...idle }],
    [
      "loaded",
      {
        ...idle,
        data: { name: "P", crs: "EPSG:4326", scenario_count: 0, run_count: 0 },
      },
    ],
  ];

  for (const [healthName, health] of healthStates) {
    for (const [projectName, project] of projectStates) {
      it(`keeps a contiguous outline with health ${healthName} and project ${projectName}`, () => {
        state.health = health;
        state.project = project;
        renderPage();

        const levels = headingLevels();
        expect(levels[0]).toBe(2);
        let deepest = 1;
        for (const level of levels) {
          expect(level).toBeLessThanOrEqual(deepest + 1);
          deepest = Math.max(deepest, level);
        }
      });
    }
  }
});
