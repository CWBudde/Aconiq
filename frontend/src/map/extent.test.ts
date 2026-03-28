import { describe, expect, it } from "vitest";
import { fitViewToWorkspace } from "./extent";
import type { CalcArea, ModelFeature, ModelReceiver } from "@/model/types";

describe("fitViewToWorkspace", () => {
  it("centers on workspace geometry when present", () => {
    const features: ModelFeature[] = [
      {
        id: "source-1",
        kind: "source",
        sourceType: "point",
        geometry: {
          type: "Point",
          coordinates: [10, 50],
        },
      },
    ];
    const receivers: ModelReceiver[] = [
      {
        id: "receiver-1",
        geometry: {
          type: "Point",
          coordinates: [11, 51],
        },
        heightM: 4,
      },
    ];
    const calcArea: CalcArea = {
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [9, 49],
            [12, 49],
            [12, 52],
            [9, 52],
            [9, 49],
          ],
        ],
      },
    };

    const view = fitViewToWorkspace(
      features,
      receivers,
      calcArea,
      [10.45, 51.16],
    );

    expect(view.center[0]).toBeCloseTo(10.5, 1);
    expect(view.center[1]).toBeCloseTo(50.5, 1);
    expect(view.zoom).toBeLessThanOrEqual(12);
  });

  it("falls back to the provided center when no geometry is available", () => {
    const view = fitViewToWorkspace([], [], null, [10.45, 51.16]);

    expect(view.center).toEqual([10.45, 51.16]);
    expect(view.zoom).toBe(6);
  });
});
