import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { Alert, AlertDescription, AlertTitle } from "./alert";

describe("Alert", () => {
  it("exposes role=alert with a title and description", () => {
    render(
      <Alert>
        <AlertTitle>Heads up</AlertTitle>
        <AlertDescription>Something to know.</AlertDescription>
      </Alert>,
    );
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Heads up");
    expect(alert).toHaveTextContent("Something to know.");
  });

  it("renders the requested variant", () => {
    render(
      <Alert variant="warning">
        <AlertTitle>Careful</AlertTitle>
      </Alert>,
    );
    expect(screen.getByRole("alert").className).toContain("text-warning");
  });
});
