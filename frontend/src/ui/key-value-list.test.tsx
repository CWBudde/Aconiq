import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { KeyValueList } from "./key-value-list";

describe("KeyValueList", () => {
  it("renders a definition list with one term and one value per item", () => {
    render(
      <KeyValueList
        items={[
          { label: "Name", value: "Demo" },
          { label: "CRS", value: "EPSG:25832", mono: true },
        ]}
      />,
    );
    const terms = screen.getAllByRole("term");
    const values = screen.getAllByRole("definition");
    expect(terms.map((t) => t.textContent)).toEqual(["Name", "CRS"]);
    expect(values.map((d) => d.textContent)).toEqual(["Demo", "EPSG:25832"]);
  });

  it("puts the monospace face only on values that ask for it", () => {
    render(
      <KeyValueList
        items={[
          { label: "Name", value: "Demo" },
          { label: "CRS", value: "EPSG:25832", mono: true },
        ]}
      />,
    );
    expect(screen.getByText("Demo")).not.toHaveClass("font-mono");
    expect(screen.getByText("EPSG:25832")).toHaveClass("font-mono");
  });

  it("keeps dt and dd as direct children of the dl for the grid", () => {
    const { container } = render(
      <KeyValueList items={[{ label: "Runs", value: 3 }]} />,
    );
    const dl = container.querySelector("dl");
    expect(dl).not.toBeNull();
    expect(Array.from(dl?.children ?? []).map((el) => el.tagName)).toEqual([
      "DT",
      "DD",
    ]);
  });

  it("switches to the dense scale", () => {
    const { container } = render(
      <KeyValueList dense items={[{ label: "Runs", value: 3 }]} />,
    );
    const dl = container.querySelector("dl");
    expect(dl).toHaveClass("text-xs");
    expect(dl).not.toHaveClass("text-sm");
  });

  it("accepts rich values", () => {
    render(
      <KeyValueList
        items={[{ label: "Link", value: <a href="/x">open</a> }]}
      />,
    );
    expect(screen.getByRole("link", { name: "open" })).toBeVisible();
  });
});
