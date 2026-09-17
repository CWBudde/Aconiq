import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { MapPanel, type MapPanelPosition } from "./map-panel";

const anchors: Record<MapPanelPosition, string[]> = {
  "top-left": ["left-3", "top-3"],
  "top-right": ["right-3", "top-3"],
  "bottom-left": ["bottom-3", "left-3"],
  "bottom-right": ["bottom-3", "right-3"],
  "bottom-center": ["bottom-3", "left-1/2", "-translate-x-1/2"],
};

describe("MapPanel", () => {
  it.each(Object.keys(anchors) as MapPanelPosition[])(
    "anchors to the %s of the map, above the canvas",
    (position) => {
      render(
        <MapPanel position={position} data-testid="panel">
          x
        </MapPanel>,
      );
      const panel = screen.getByTestId("panel");
      expect(panel).toHaveClass("absolute", "z-10", ...anchors[position]);
      expect(panel).toHaveAttribute("data-position", position);
    },
  );

  it("is opaque with a shadow by default and see-through when translucent", () => {
    const { rerender } = render(
      <MapPanel position="top-left" data-testid="panel">
        x
      </MapPanel>,
    );
    const panel = screen.getByTestId("panel");
    expect(panel).toHaveClass("bg-background", "shadow-md");
    expect(panel).not.toHaveClass("backdrop-blur-sm");

    rerender(
      <MapPanel position="top-left" translucent data-testid="panel">
        x
      </MapPanel>,
    );
    expect(panel).toHaveClass(
      "bg-background/90",
      "backdrop-blur-sm",
      "shadow-sm",
    );
    expect(panel).not.toHaveClass("shadow-md");
  });

  it("lets an inset override the corner offset, last class winning", () => {
    render(
      <MapPanel position="top-right" inset="right-12 top-2" data-testid="panel">
        x
      </MapPanel>,
    );
    const panel = screen.getByTestId("panel");
    expect(panel).toHaveClass("right-12", "top-2");
    expect(panel).not.toHaveClass("right-3");
    expect(panel).not.toHaveClass("top-3");
  });

  it("applies a width and lets className replace the padding", () => {
    render(
      <MapPanel
        position="bottom-left"
        width="w-80"
        className="p-4"
        data-testid="panel"
      >
        x
      </MapPanel>,
    );
    const panel = screen.getByTestId("panel");
    expect(panel).toHaveClass("w-80", "p-4");
    expect(panel).not.toHaveClass("p-2");
  });

  it("passes a role and an accessible name through", () => {
    render(
      <MapPanel position="top-left" role="toolbar" aria-label="Draw">
        <button type="button">Point</button>
      </MapPanel>,
    );
    expect(screen.getByRole("toolbar", { name: "Draw" })).toContainElement(
      screen.getByRole("button", { name: "Point" }),
    );
  });
});
