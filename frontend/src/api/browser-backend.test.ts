import { describe, expect, it } from "vitest";
import { buildRoadSources, overpassWayToFeature } from "./browser-backend";
import type { ModelFeature } from "@/model/types";

describe("buildRoadSources", () => {
  it("prefers feature-level RLS-19 overrides over run defaults", () => {
    const features: ModelFeature[] = [
      {
        id: "road-a",
        kind: "source",
        sourceType: "line",
        properties: {
          surface_type: "Beton",
          road_speed_kph: 60,
          speed_lkw2_kph: 50,
          gradient_percent: 4,
          traffic_day_pkw: 1200,
          traffic_night_pkw: 200,
        },
        geometry: {
          type: "LineString",
          coordinates: [
            [0, 0],
            [10, 0],
          ],
        },
      },
      {
        id: "road-b",
        kind: "source",
        sourceType: "line",
        properties: {
          traffic_day_pkw: 300,
        },
        geometry: {
          type: "LineString",
          coordinates: [
            [0, 10],
            [10, 10],
          ],
        },
      },
    ];

    const sources = buildRoadSources(features, {
      surface_type: "SMA",
      speed_pkw_kph: "100",
      speed_lkw1_kph: "100",
      speed_lkw2_kph: "80",
      speed_krad_kph: "100",
      gradient_percent: "0",
      traffic_day_pkw: "900",
      traffic_day_lkw1: "40",
      traffic_day_lkw2: "60",
      traffic_day_krad: "10",
      traffic_night_pkw: "200",
      traffic_night_lkw1: "10",
      traffic_night_lkw2: "20",
      traffic_night_krad: "2",
    });

    expect(sources).toHaveLength(2);
    expect(sources[0]?.surface_type).toBe("Beton");
    expect(sources[0]?.speeds.pkw_kph).toBe(60);
    expect(sources[0]?.speeds.lkw2_kph).toBe(50);
    expect(sources[0]?.traffic_day.pkw_per_hour).toBe(1200);
    expect(sources[1]?.surface_type).toBe("SMA");
    expect(sources[1]?.speeds.pkw_kph).toBe(100);
    expect(sources[1]?.traffic_day.pkw_per_hour).toBe(300);
  });
});

describe("overpassWayToFeature", () => {
  it("maps highway tags to source acoustics and marks review-needed imports", () => {
    const feature = overpassWayToFeature({
      type: "way",
      id: 42,
      tags: {
        highway: "primary",
        maxspeed: "50",
        surface: "asphalt",
      },
      geometry: [
        { lon: 7, lat: 50 },
        { lon: 7.1, lat: 50.1 },
      ],
    });

    expect(feature?.properties["kind"]).toBe("source");
    expect(feature?.properties["road_speed_kph"]).toBe(50);
    expect(feature?.properties["surface_type"]).toBe("SMA");
    expect(feature?.properties["road_speed_kph_inferred"]).toBe(true);
  });
});
