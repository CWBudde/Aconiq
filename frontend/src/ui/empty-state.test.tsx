import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { Play } from "lucide-react";
import { EmptyState } from "./empty-state";

describe("EmptyState", () => {
  it("shows the title, the description and the actions", () => {
    render(
      <EmptyState title="No runs yet" description="Start one to see it here.">
        <button type="button">New run</button>
      </EmptyState>,
    );
    expect(screen.getByText("No runs yet")).toBeVisible();
    expect(screen.getByText("Start one to see it here.")).toBeVisible();
    expect(screen.getByRole("button", { name: "New run" })).toBeVisible();
  });

  it("renders the icon decoratively", () => {
    const { container } = render(<EmptyState icon={Play} title="Empty" />);
    const icon = container.querySelector("svg");
    expect(icon).not.toBeNull();
    expect(icon).toHaveAttribute("aria-hidden", "true");
  });

  it("renders no description or actions wrapper when none is given", () => {
    const { container } = render(<EmptyState title="Only a title" />);
    const root = container.querySelector('[data-slot="empty-state"]');
    expect(root?.childElementCount).toBe(1);
  });

  it("scales down in compact mode", () => {
    render(<EmptyState compact title="Nothing" description="Really" />);
    expect(screen.getByText("Nothing")).toHaveClass("text-sm");
    expect(screen.getByText("Really")).toHaveClass("text-xs");
  });
});
