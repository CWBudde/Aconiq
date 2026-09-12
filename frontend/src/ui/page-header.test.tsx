import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { PageHeader, SectionHeading } from "./page-header";

describe("PageHeader", () => {
  it("renders the title as an h2 by default, under the shell's h1", () => {
    render(<PageHeader title="Runs" />);
    expect(
      screen.getByRole("heading", { level: 2, name: "Runs" }),
    ).toBeVisible();
    expect(screen.queryByRole("heading", { level: 1 })).toBeNull();
  });

  it("takes another heading level", () => {
    render(<PageHeader as="h3" title="Nested" />);
    expect(
      screen.getByRole("heading", { level: 3, name: "Nested" }),
    ).toBeVisible();
  });

  it("shows the description and the actions when given", () => {
    render(
      <PageHeader
        title="Exports"
        description="3 runs with exports"
        actions={<button type="button">New export</button>}
      />,
    );
    expect(screen.getByText("3 runs with exports")).toBeVisible();
    expect(screen.getByRole("button", { name: "New export" })).toBeVisible();
  });

  it("renders no empty slots", () => {
    const { container } = render(<PageHeader title="Only" />);
    const header = container.querySelector('[data-slot="page-header"]');
    expect(header).not.toBeNull();
    // Title block only: no actions wrapper.
    expect(header?.childElementCount).toBe(1);
  });
});

describe("SectionHeading", () => {
  it("renders an h3 by default", () => {
    render(<SectionHeading>Progress</SectionHeading>);
    expect(
      screen.getByRole("heading", { level: 3, name: "Progress" }),
    ).toBeVisible();
  });

  it("can be the column header of a list at h2", () => {
    render(
      <SectionHeading as="h2" description="4 runs">
        Runs
      </SectionHeading>,
    );
    expect(
      screen.getByRole("heading", { level: 2, name: "Runs" }),
    ).toBeVisible();
    expect(screen.getByText("4 runs")).toBeVisible();
  });

  it("marks the eyebrow variant as data and upper-cases it", () => {
    const { container } = render(
      <SectionHeading variant="eyebrow">Project</SectionHeading>,
    );
    const wrapper = container.querySelector('[data-slot="section-heading"]');
    expect(wrapper).toHaveAttribute("data-variant", "eyebrow");
    expect(screen.getByRole("heading", { name: "Project" })).toHaveClass(
      "uppercase",
    );
  });

  it("renders the actions slot on the right", () => {
    render(
      <SectionHeading actions={<button type="button">Clear</button>}>
        Filters
      </SectionHeading>,
    );
    expect(screen.getByRole("button", { name: "Clear" })).toBeVisible();
  });
});
