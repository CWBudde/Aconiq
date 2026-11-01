import { describe, expect, it } from "vitest";
import {
  ORTHOPHOTO_LAYER_ID,
  ORTHOPHOTO_SOURCE_ID,
  orthophotoEvidence,
  orthophotoEvidenceURL,
  orthophotoStyle,
} from "./orthophoto";

const NOW = new Date("2026-11-01T12:00:00Z");

function params(url: string | null): URLSearchParams {
  expect(url).not.toBeNull();
  return new URL(url ?? "").searchParams;
}

describe("orthophotoStyle", () => {
  it("asks LGLN's WMS for each web-mercator tile, attributed as the AGNB require", () => {
    const { source, layer } = orthophotoStyle(true, NOW);

    const tile = source.tiles?.[0] ?? "";
    expect(tile).toContain(
      "opendata.lgln.niedersachsen.de/doorman/noauth/dop_wms",
    );
    expect(tile).toContain("LAYERS=ni_dop20");
    expect(tile).toContain("CRS=EPSG:3857");
    expect(tile).toContain("BBOX={bbox-epsg-3857}");
    expect(source.attribution).toContain("GeoBasis-DE/LGLN</a> 2026");
    expect(source.attribution).toContain("CC BY 4.0");
    expect(layer).toMatchObject({
      id: ORTHOPHOTO_LAYER_ID,
      source: ORTHOPHOTO_SOURCE_ID,
      layout: { visibility: "visible" },
    });
  });

  it("starts hidden unless asked for", () => {
    expect(orthophotoStyle(false, NOW).layer.layout).toEqual({
      visibility: "none",
    });
  });
});

describe("orthophotoEvidenceURL", () => {
  it("requests the padded box in a projected project CRS, at the DOP's resolution", () => {
    const query = params(
      orthophotoEvidenceURL("EPSG:25832", [550000, 5800000, 550030, 5800020]),
    );

    expect(query.get("CRS")).toBe("EPSG:25832");
    expect(query.get("BBOX")).toBe(
      "549990.000000,5799990.000000,550040.000000,5800030.000000",
    );
    // 50 m by 40 m at 0.2 m a pixel.
    expect(query.get("WIDTH")).toBe("250");
    expect(query.get("HEIGHT")).toBe("200");
  });

  it("puts latitude first for a geographic CRS, as WMS 1.3.0 reads it", () => {
    const query = params(
      orthophotoEvidenceURL("EPSG:4326", [9.7, 52.36, 9.71, 52.37]),
    );

    const [south, west, north, east] = (query.get("BBOX") ?? "")
      .split(",")
      .map(Number);
    expect(south).toBeCloseTo(52.3599);
    expect(west).toBeCloseTo(9.6999);
    expect(north).toBeCloseTo(52.3701);
    expect(east).toBeCloseTo(9.7101);
  });

  it("is the same request for the same box", () => {
    const box: [number, number, number, number] = [1, 2, 3, 4];
    expect(orthophotoEvidenceURL("EPSG:25832", box)).toBe(
      orthophotoEvidenceURL("EPSG:25832", box),
    );
  });

  it("has no request for a CRS the WMS does not offer", () => {
    expect(orthophotoEvidenceURL("EPSG:2056", [0, 0, 1, 1])).toBeNull();
  });
});

describe("orthophotoEvidence", () => {
  const BOX: [number, number, number, number] = [
    550000, 5800000, 550030, 5800020,
  ];

  it("records the request, the SHA-256 of what it answered, and the licence", async () => {
    const requested: string[] = [];
    const entry = await orthophotoEvidence("EPSG:25832", BOX, {
      now: NOW,
      fetch: (input) => {
        requested.push(String(input));
        return Promise.resolve(new Response(new TextEncoder().encode("abc")));
      },
    });

    expect(requested).toEqual([entry.source_id]);
    expect(entry).toEqual({
      kind: "orthophoto",
      source_id: orthophotoEvidenceURL("EPSG:25832", BOX),
      // SHA-256("abc").
      content_sha256:
        "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
      retrieved_at: "2026-11-01T12:00:00.000Z",
      licence: "CC-BY-4.0",
      terms_url: "https://creativecommons.org/licenses/by/4.0/",
      attribution: "© GeoBasis-DE/LGLN 2026, Daten geändert, CC BY 4.0",
    });
  });

  it("still records the source, without a hash, when the image cannot be had", async () => {
    const failed = await orthophotoEvidence("EPSG:25832", BOX, {
      now: NOW,
      fetch: () => Promise.reject(new Error("offline")),
    });
    expect(failed.content_sha256).toBeUndefined();
    expect(failed.note).toContain("offline");

    const refused = await orthophotoEvidence("EPSG:25832", BOX, {
      now: NOW,
      fetch: () => Promise.resolve(new Response("", { status: 503 })),
    });
    expect(refused.content_sha256).toBeUndefined();
    expect(refused.note).toContain("503");

    const elsewhere = await orthophotoEvidence("EPSG:2056", BOX, {
      now: NOW,
      fetch: () => Promise.reject(new Error("must not be called")),
    });
    expect(elsewhere.content_sha256).toBeUndefined();
    expect(elsewhere.note).toContain("EPSG:2056");
  });
});
