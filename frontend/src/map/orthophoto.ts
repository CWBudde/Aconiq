import type {
  RasterLayerSpecification,
  RasterSourceSpecification,
} from "maplibre-gl";

/**
 * The LGLN orthophoto (DOP20) as a map overlay, and the evidence record a
 * source traced over it carries (P18 E2).
 *
 * The image comes straight from LGLN's open WMS: online only, like the OSM
 * basemap, and never stored. What a project keeps is a feature and, in its
 * `evidence`, the GetMap request that shows the area it was traced in and the
 * SHA-256 of that response — enough for a reviewer to fetch the same pixels
 * again and see whether they still are the same. The licence is CC BY 4.0
 * (LGLN AGNB §7.1; `docs/research/facade-evidence-sources.md`).
 */

/** LGLN's open DOP WMS. Verified 2026-11-01: `Access-Control-Allow-Origin: *`. */
export const ORTHOPHOTO_WMS_URL =
  "https://opendata.lgln.niedersachsen.de/doorman/noauth/dop_wms";

/** The WMS layer: the current DOP20 flight, RGB. */
export const ORTHOPHOTO_WMS_LAYER = "ni_dop20";

export const ORTHOPHOTO_SOURCE_ID = "lgln-dop";
export const ORTHOPHOTO_LAYER_ID = "lgln-dop-raster";

/** The layer-visibility key the overlay is toggled under. Off by default. */
export const ORTHOPHOTO_GROUP_ID = "orthophoto";

export const ORTHOPHOTO_LICENCE = "CC-BY-4.0";
export const ORTHOPHOTO_TERMS_URL =
  "https://creativecommons.org/licenses/by/4.0/";

/**
 * The CRSs the WMS offers (its GetCapabilities, 2026-11-01). An evidence
 * request is made in the project's own CRS so the feature's coordinates go
 * out as they are stored, with no projection here; a CRS outside this list
 * gets an evidence entry without a hash.
 */
const WMS_PROJECTED_CRS = new Set([
  "EPSG:25832",
  "EPSG:25833",
  "EPSG:32632",
  "EPSG:32633",
  "EPSG:3044",
  "EPSG:3045",
  "EPSG:3034",
  "EPSG:3035",
  "EPSG:3857",
  "EPSG:31466",
  "EPSG:31467",
  "EPSG:31468",
  "EPSG:31469",
  "EPSG:4647",
]);

/** Geographic CRSs, whose WMS 1.3.0 axis order is latitude first. */
const WMS_GEOGRAPHIC_CRS = new Set(["EPSG:4326", "EPSG:4258"]);

/** The AGNB §7.2 form for the image as shown, with the licence named. */
export function orthophotoAttribution(now: Date = new Date()): string {
  return `&copy; <a href="https://www.lgln.niedersachsen.de">GeoBasis-DE/LGLN</a> ${String(now.getUTCFullYear())}, <a href="${ORTHOPHOTO_TERMS_URL}">CC BY 4.0</a>`;
}

/**
 * The attribution a feature traced from the image carries in its evidence:
 * the AGNB form for changed data, with the year of download, and the licence
 * (`data-handling.md`, LGLN DOP row).
 */
export function orthophotoDerivedAttribution(now: Date): string {
  return `© GeoBasis-DE/LGLN ${String(now.getUTCFullYear())}, Daten geändert, CC BY 4.0`;
}

function getMapURL(
  crs: string,
  bbox: string,
  width: number,
  height: number,
): string {
  const params = [
    "SERVICE=WMS",
    "VERSION=1.3.0",
    "REQUEST=GetMap",
    `LAYERS=${ORTHOPHOTO_WMS_LAYER}`,
    "STYLES=",
    `CRS=${crs}`,
    `BBOX=${bbox}`,
    `WIDTH=${String(width)}`,
    `HEIGHT=${String(height)}`,
    "FORMAT=image/png",
  ];
  return `${ORTHOPHOTO_WMS_URL}?${params.join("&")}`;
}

/** The overlay's source and layer, spliced into a basemap style above its tiles. */
export function orthophotoStyle(
  visible: boolean,
  now: Date = new Date(),
): { source: RasterSourceSpecification; layer: RasterLayerSpecification } {
  return {
    source: {
      type: "raster",
      // MapLibre fills `{bbox-epsg-3857}` per tile as minx,miny,maxx,maxy,
      // which is the axis order WMS 1.3.0 reads for EPSG:3857.
      tiles: [getMapURL("EPSG:3857", "{bbox-epsg-3857}", 256, 256)],
      tileSize: 256,
      attribution: orthophotoAttribution(now),
    },
    layer: {
      id: ORTHOPHOTO_LAYER_ID,
      type: "raster",
      source: ORTHOPHOTO_SOURCE_ID,
      layout: { visibility: visible ? "visible" : "none" },
    },
  };
}

/** A feature's bounding box in its own CRS: [minX, minY, maxX, maxY]. */
export type BBox = [number, number, number, number];

/** How far the evidence image reaches past the feature, so it shows context. */
const PAD_M = 10;
const PAD_DEG = 0.0001;
/** The DOP20's ground resolution; the evidence image is not finer than it. */
const DOP_RESOLUTION_M = 0.2;
const MIN_PX = 64;
const MAX_PX = 2048;

function clampPx(px: number): number {
  return Math.min(MAX_PX, Math.max(MIN_PX, Math.ceil(px)));
}

/** Fixed decimals, so the same box always gives the same request URL. */
function coord(value: number): string {
  return value.toFixed(6);
}

/**
 * The GetMap request for the area a feature was traced in, in the project's
 * CRS, or `null` when the WMS does not offer that CRS.
 */
export function orthophotoEvidenceURL(crs: string, bbox: BBox): string | null {
  const [minX, minY, maxX, maxY] = bbox;
  if (WMS_PROJECTED_CRS.has(crs)) {
    const box: BBox = [minX - PAD_M, minY - PAD_M, maxX + PAD_M, maxY + PAD_M];
    const width = clampPx((box[2] - box[0]) / DOP_RESOLUTION_M);
    const height = clampPx((box[3] - box[1]) / DOP_RESOLUTION_M);
    return getMapURL(crs, box.map(coord).join(","), width, height);
  }
  if (WMS_GEOGRAPHIC_CRS.has(crs)) {
    // x is longitude here, and WMS 1.3.0 wants latitude first for these.
    const box: BBox = [
      minX - PAD_DEG,
      minY - PAD_DEG,
      maxX + PAD_DEG,
      maxY + PAD_DEG,
    ];
    const midLatRad = ((box[1] + box[3]) / 2) * (Math.PI / 180);
    const lonSpan = (box[2] - box[0]) * Math.cos(midLatRad);
    const latSpan = box[3] - box[1];
    const longer = Math.max(lonSpan, latSpan);
    const width = clampPx((MAX_PX / 2) * (lonSpan / longer));
    const height = clampPx((MAX_PX / 2) * (latSpan / longer));
    const bboxText = [box[1], box[0], box[3], box[2]].map(coord).join(",");
    return getMapURL(crs, bboxText, width, height);
  }
  return null;
}

/** One `evidence` entry, in the field names the data model documents. */
export interface OrthophotoEvidence {
  kind: "orthophoto";
  source_id: string;
  content_sha256?: string;
  retrieved_at: string;
  licence: string;
  terms_url: string;
  attribution: string;
  /** Why the entry has no hash, when it has none. */
  note?: string;
}

function hex(bytes: ArrayBuffer): string {
  return Array.from(new Uint8Array(bytes), (b) =>
    b.toString(16).padStart(2, "0"),
  ).join("");
}

/**
 * The evidence entry for a feature traced over the overlay: the GetMap
 * request for its padded box and the SHA-256 of the response. A request that
 * fails, or a CRS the WMS does not offer, still yields an entry — the feature
 * was traced from the orthophoto either way — but without a hash, and it
 * says why.
 */
export async function orthophotoEvidence(
  crs: string,
  bbox: BBox,
  options: { fetch?: typeof fetch; now?: Date } = {},
): Promise<OrthophotoEvidence> {
  const now = options.now ?? new Date();
  const doFetch = options.fetch ?? fetch;
  const url = orthophotoEvidenceURL(crs, bbox);
  const entry: OrthophotoEvidence = {
    kind: "orthophoto",
    source_id: url ?? `${ORTHOPHOTO_WMS_URL}#${ORTHOPHOTO_WMS_LAYER}`,
    retrieved_at: now.toISOString(),
    licence: ORTHOPHOTO_LICENCE,
    terms_url: ORTHOPHOTO_TERMS_URL,
    attribution: orthophotoDerivedAttribution(now),
  };
  if (url === null) {
    return {
      ...entry,
      note: `the WMS does not offer ${crs}, so no image was requested to hash`,
    };
  }
  try {
    const response = await doFetch(url);
    if (!response.ok) {
      return {
        ...entry,
        note: `the WMS answered ${String(response.status)}, so the image was not hashed`,
      };
    }
    const digest = await crypto.subtle.digest(
      "SHA-256",
      await response.arrayBuffer(),
    );
    return { ...entry, content_sha256: hex(digest) };
  } catch (error) {
    const reason = error instanceof Error ? error.message : String(error);
    return {
      ...entry,
      note: `the image could not be fetched (${reason}), so it was not hashed`,
    };
  }
}
