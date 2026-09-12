import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { Badge, badgeVariants } from "./badge";

describe("Badge", () => {
  it("renders its content with the default variant", () => {
    render(<Badge>Draft</Badge>);
    const badge = screen.getByText("Draft");
    expect(badge).toHaveAttribute("data-slot", "badge");
    expect(badge.className).toContain("bg-primary");
  });

  it("renders the soft semantic variants in their colour", () => {
    render(
      <>
        <Badge variant="success">Ok</Badge>
        <Badge variant="warning">Careful</Badge>
        <Badge variant="info">Note</Badge>
        <Badge variant="destructive">Failed</Badge>
      </>,
    );
    expect(screen.getByText("Ok").className).toContain("text-success");
    expect(screen.getByText("Careful").className).toContain("text-warning");
    expect(screen.getByText("Note").className).toContain("text-info");
    expect(screen.getByText("Failed").className).toContain("text-destructive");
  });

  it("exposes the variant classes for use on other elements", () => {
    expect(badgeVariants({ variant: "outline" })).toContain("text-foreground");
  });
});
