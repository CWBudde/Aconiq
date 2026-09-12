import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { Info } from "lucide-react";
import { Callout } from "./callout";

describe("Callout", () => {
  it("is plain content by default: a neutral hint has no live-region role", () => {
    render(<Callout data-testid="hint">A quiet hint</Callout>);
    const hint = screen.getByTestId("hint");
    expect(hint).not.toHaveAttribute("role");
    expect(hint).toHaveAttribute("data-variant", "neutral");
    expect(hint).toHaveTextContent("A quiet hint");
  });

  it("announces a warning and a failure as an alert", () => {
    render(
      <>
        <Callout variant="warning">Watch out</Callout>
        <Callout variant="destructive">It broke</Callout>
      </>,
    );
    const alerts = screen.getAllByRole("alert");
    expect(alerts.map((el) => el.textContent)).toEqual([
      "Watch out",
      "It broke",
    ]);
  });

  it("announces info and success politely as a status", () => {
    render(
      <>
        <Callout variant="info">Did you know</Callout>
        <Callout variant="success">Saved</Callout>
      </>,
    );
    const statuses = screen.getAllByRole("status");
    expect(statuses.map((el) => el.textContent)).toEqual([
      "Did you know",
      "Saved",
    ]);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("lets the caller override the role", () => {
    render(
      <Callout variant="warning" role="note">
        Static caveat
      </Callout>,
    );
    expect(screen.getByRole("note")).toHaveTextContent("Static caveat");
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("renders the icon in the Alert icon slot, hidden from readers", () => {
    render(
      <Callout icon={Info} data-testid="with-icon">
        Text
      </Callout>,
    );
    const icon = screen.getByTestId("with-icon").querySelector(":scope > svg");
    expect(icon).not.toBeNull();
    expect(icon).toHaveAttribute("aria-hidden", "true");
  });

  it("renders an optional bold title above the body", () => {
    render(
      <Callout variant="destructive" title="Run refused">
        The standard needs an acknowledgement.
      </Callout>,
    );
    const alert = screen.getByRole("alert");
    expect(alert.querySelector('[data-slot="alert-title"]')).toHaveTextContent(
      "Run refused",
    );
    expect(
      alert.querySelector('[data-slot="alert-description"]'),
    ).toHaveTextContent("The standard needs an acknowledgement.");
  });

  it("passes through div props and merges classes", () => {
    render(
      <Callout id="c1" className="mt-4" data-testid="callout">
        x
      </Callout>,
    );
    const el = screen.getByTestId("callout");
    expect(el).toHaveAttribute("id", "c1");
    expect(el).toHaveClass("mt-4");
    expect(el).toHaveClass("text-xs");
  });
});
