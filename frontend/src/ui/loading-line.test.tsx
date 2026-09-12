import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { LoadingLine } from "./loading-line";
import { m } from "@/i18n/messages";

describe("LoadingLine", () => {
  it("falls back to the generic loading message", () => {
    render(<LoadingLine />);
    expect(screen.getByText(m.status_loading())).toBeInTheDocument();
  });

  it("shows what is being loaded when the caller says", () => {
    render(<LoadingLine text={m.status_loading_health()} />);
    expect(screen.getByText(m.status_loading_health())).toBeInTheDocument();
    expect(screen.queryByText(m.status_loading())).not.toBeInTheDocument();
  });

  it("hides the spinner from the accessibility tree", () => {
    // The text carries the meaning. An exposed icon would announce itself
    // without saying what is happening.
    const { container } = render(<LoadingLine />);
    const icon = container.querySelector("svg");
    expect(icon).toHaveAttribute("aria-hidden", "true");
  });
});
