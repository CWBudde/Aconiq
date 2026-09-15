import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ProjectSummary } from "./project-summary";
import { m } from "@/i18n/messages";

type Status = {
  isLoading: boolean;
  isError: boolean;
  error: null | { message: string };
  data: null | {
    name: string;
    crs: string;
    scenario_count: number;
    run_count: number;
  };
};

let status: Status = {
  isLoading: false,
  isError: false,
  error: null,
  data: null,
};

vi.mock("@/api/hooks", () => ({
  useProjectStatus: () => status,
}));

const loaded = {
  name: "Demo Project",
  crs: "EPSG:25832",
  scenario_count: 2,
  run_count: 7,
};

function renderWith(next: Partial<Status>, emptyAction?: React.ReactNode) {
  status = {
    isLoading: false,
    isError: false,
    error: null,
    data: null,
    ...next,
  };
  render(<ProjectSummary emptyAction={emptyAction} />);
}

describe("ProjectSummary", () => {
  it("says the request is in flight", () => {
    renderWith({ isLoading: true });
    expect(screen.getByText(m.status_loading_project())).toBeInTheDocument();
  });

  it("surfaces the error rather than reading as an empty project", () => {
    renderWith({ isError: true, error: { message: "Request failed: 500" } });
    expect(screen.getByText("Request failed: 500")).toBeInTheDocument();
    expect(screen.queryByText(m.msg_no_project_yet())).toBeNull();
  });

  it("offers the caller's action when there is no project", () => {
    renderWith({ data: null }, <button type="button">Import</button>);
    expect(screen.getByText(m.msg_no_project_yet())).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Import" })).toBeVisible();
  });

  it("shows the project's identity and size", () => {
    renderWith({ data: loaded });
    expect(screen.getByText("Demo Project")).toBeInTheDocument();
    expect(screen.getByText("EPSG:25832")).toBeInTheDocument();
    expect(screen.getByText("7")).toBeInTheDocument();
  });
});
