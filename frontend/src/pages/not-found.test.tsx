import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import NotFoundPage from "./not-found";
import { m } from "@/i18n/messages";

function renderAt(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <NotFoundPage />
    </MemoryRouter>,
  );
}

describe("NotFoundPage", () => {
  it("names the path that matched nothing", () => {
    renderAt("/nonsense");
    expect(
      screen.getByText(m.msg_not_found({ path: "/nonsense" })),
    ).toBeInTheDocument();
  });

  it("keeps the heading structure every route owes the shell", () => {
    // `waitForPage` in e2e/app.ts waits for a header h1 and an h2 or h3 inside
    // main. The shell supplies the h1; this is the page's half.
    renderAt("/nonsense");
    const heading = screen.getAllByRole("heading")[0];
    expect(heading?.tagName).toBe("H2");
  });

  it("offers a way back out", () => {
    renderAt("/nonsense");
    expect(screen.getByRole("link", { name: m.nav_import() })).toHaveAttribute(
      "href",
      "/import",
    );
  });
});
