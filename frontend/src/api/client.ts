import type { GeoJSONFeatureCollection } from "@/model/types";
import type { components } from "./schema";

/**
 * The wire contract with the local API.
 *
 * The DTOs below are **not written here**. They are aliases onto
 * `./schema.ts`, which `scripts/generate-api-client.mjs` generates from the
 * document `aconiq openapi` exports, and `just fe-api-check` fails the build
 * when the two have drifted. This file exists so that the app keeps importing
 * the names it always did — `RunSummary`, not
 * `components["schemas"]["RunSummary"]` — and so that the parts of the contract
 * the document *cannot* state have one place to be stated in.
 *
 * There are three of those, each marked below:
 *
 *   1. the two model endpoints, whose GeoJSON body the spec declares as a bare
 *      `object`;
 *   2. the artifact-content endpoint, whose payloads are files rather than
 *      schemas — `ReceiverTable` and `RasterMetadata` are the shapes the run
 *      writes into a project, not shapes the API declares;
 *   3. the request headers, which are transport and not a schema at all.
 *
 * Anything else added here is drift waiting to happen: put it in `openapi.go`
 * and regenerate.
 */

type Schemas = components["schemas"];

/**
 * The custom request header the local API requires on every state-changing
 * method. A CORS simple request cannot carry a custom header, so sending one
 * forces a preflight, which is what makes a cross-site write visible to the
 * server's origin allowlist. The server never reads the value — only its
 * presence counts.
 */
export const CLIENT_HEADER_NAME = "X-Aconiq-Client";

const CLIENT_HEADER_VALUE = "aconiq-web";

/**
 * Optional bearer token, matching `aconiq serve --api-token`. Empty unless the
 * server was started with one, in which case every request must present it.
 */
const API_TOKEN = import.meta.env.VITE_API_TOKEN ?? "";

/**
 * Headers every request to the local API carries. `extra` wins over the
 * defaults, so a caller can set its own Content-Type without restating the
 * rest.
 */
export function apiHeaders(
  extra?: Readonly<Record<string, string>>,
): Record<string, string> {
  const headers: Record<string, string> = {
    Accept: "application/json",
    [CLIENT_HEADER_NAME]: CLIENT_HEADER_VALUE,
    ...extra,
  };

  if (API_TOKEN !== "") {
    headers.Authorization = `Bearer ${API_TOKEN}`;
  }

  return headers;
}

/* -------------------------------------------------------------------------
 * Generated schemas, under the names the app uses.
 * ---------------------------------------------------------------------- */

export type APIError = Schemas["APIError"];
export type ErrorEnvelope = Schemas["ErrorEnvelope"];
export type HealthResponse = Schemas["HealthResponse"];
export type LastRunStatus = Schemas["LastRunStatus"];
export type ProjectStatusResponse = Schemas["ProjectStatusResponse"];

/**
 * The saved model's receipt. A client that kept the `hash` from its last
 * `POST /api/v1/model` compares the two as strings to learn whether its local
 * draft is still the project's model, without fetching anything. Never
 * recompute the hash locally: a re-serialised model is not the same bytes.
 */
export type ProjectModelStatus = Schemas["ProjectModelStatus"];

export type ArtifactRef = Schemas["ArtifactRef"];
export type RunSummary = Schemas["RunSummary"];
export type CreateRunRequest = Schemas["CreateRunRequest"];
export type RunLog = Schemas["RunLog"];

/**
 * Response of `DELETE /api/v1/runs/{id}`. Both arrays are always present,
 * empty rather than null.
 *
 * A run that is still `pending` or `running` is refused with 409 and error code
 * `run_not_finished` — its directory is still being written.
 */
export type DeleteRunResponse = Schemas["DeleteRunResponse"];

/**
 * One validation finding as the API reports it. Named apart from the model
 * store's own `ValidationIssue`, which carries a level and camel-cased fields;
 * this one is the wire shape.
 */
export type APIValidationIssue = Schemas["ValidationIssue"];

/**
 * Response of a successful model save. A model with validation errors is
 * refused instead, with error code `model_invalid` and the findings under
 * `details.errors` as `APIValidationIssue[]`.
 */
export type ModelSaveResponse = Schemas["ModelSaveResponse"];

/**
 * The terrain import's receipt (`POST /api/v1/import/terrain`).
 *
 * Nothing in the app calls that endpoint yet — the type is here because the
 * generated schema carries it, and because the binding is the next thing an
 * upload control would need. See `PLAN.md`, Priority 8 Phase F.
 */
export type TerrainInfo = Schemas["TerrainInfo"];

/**
 * The standards descriptor contract. It is declared in `@/standards/descriptor`
 * because both backends publish it and neither owns it — `aconiq serve` through
 * `GET /api/v1/standards`, the WASM kernel through `aconiq.standards()` — and
 * re-exported here so that an existing `from "./client"` import keeps working.
 *
 * The generated `Schemas["StandardDescriptor"]` describes the same shape, but
 * only the HTTP half of it: aliasing it here would quietly make the API the
 * owner of a contract the kernel serves too.
 */
export type {
  ParameterDefinition,
  ProfileInfo,
  StandardDescriptor,
  VersionInfo,
} from "@/standards/descriptor";

/* -------------------------------------------------------------------------
 * (1) The model endpoints.
 *
 * `openapi.go` declares both bodies as `{"type": "object"}` with the schema in
 * prose — a GeoJSON FeatureCollection in the v1 input schema
 * (docs/geojson-schema-v1.md). The generator can only read that as
 * `Record<string, never>`, which is an object with no properties: assigning a
 * FeatureCollection to it is an error. So the `model` field is replaced with
 * the type the app actually holds, and the rest of each body stays generated.
 * ---------------------------------------------------------------------- */

/** Body of `POST /api/v1/model`. The model is handed over unparsed. */
export type ModelSaveRequest = Omit<Schemas["ModelSaveRequest"], "model"> & {
  model: GeoJSONFeatureCollection;
};

/**
 * Response of `GET /api/v1/model`. A project that loaded but has no model yet
 * is refused with error code `model_not_found`, which is deliberately not the
 * `not_found` a missing project produces.
 */
export type ModelResponse = Omit<Schemas["ModelResponse"], "model"> & {
  model: GeoJSONFeatureCollection;
};

/* -------------------------------------------------------------------------
 * (2) Artifact payloads.
 *
 * `GET /api/v1/artifacts/{id}/content` streams whatever file the ref names —
 * JSON for a receiver table or a raster sidecar, `application/octet-stream`
 * for the raster itself. The endpoint declares no schema because it has no one
 * schema, so these mirror the Go writers in
 * `backend/internal/report/results/` rather than the API document.
 * ---------------------------------------------------------------------- */

export interface ReceiverRecord {
  id: string;
  x: number;
  y: number;
  height_m: number;
  values: Record<string, number>;
}

export interface ReceiverTable {
  indicator_order: string[];
  /**
   * The unit each indicator's values carry, keyed by indicator name.
   *
   * Per indicator because the values are: `beb-exposure` lists Lden and
   * Lnight beside six dwelling and person counts. It used to be one string
   * for the whole table, which forced that module to write `"mixed"` — a word
   * true of no column, and one this app read as "not decibels" and refused
   * the whole run over.
   *
   * Keyed by name and not a slice parallel to `indicator_order`, matching
   * `results.ReceiverTable` in Go: a reorder of the names would silently
   * relabel every value.
   */
  units: Record<string, string>;
  records: ReceiverRecord[];
}

/**
 * Where a raster's cells sit on the ground. Mirrors `results.Georeference` in
 * `backend/internal/report/results/raster.go`.
 *
 * `origin_x`/`origin_y` is the **centre of cell (0,0)**, not a corner: a grid
 * receiver is a point in the middle of the cell it stands for, so the origin is
 * exactly the first row of `receivers.csv`. Anything drawing this raster has to
 * offset by half a pixel to get a corner-based extent.
 *
 * `row_order` is `"south-up"`: row 0 holds the southernmost cells. Nothing
 * writes anything else today, and the Go side refuses a value it does not
 * recognise rather than reading it as south-up.
 */
export interface RasterGeoreference {
  origin_x: number;
  origin_y: number;
  pixel_size_m: number;
  row_order: string;
}

export interface RasterMetadata {
  width: number;
  height: number;
  bands: number;
  nodata: number;
  /** The unit each band's cells carry, keyed by band name. See {@link ReceiverTable.units}. */
  units: Record<string, string>;
  band_names?: string[];
  /** The CRS the values are in — the run's *compute* CRS, not the project's. */
  crs?: string;
  /**
   * Absent when the receivers are not a grid: explicit receiver mode places
   * points, and no cell size describes them. Absence means "not a grid", never
   * "a grid at the origin".
   */
  georeference?: RasterGeoreference;
}
