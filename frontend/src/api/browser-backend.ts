import type {
  ArtifactRef,
  HealthResponse,
  ProjectStatusResponse,
  RasterMetadata,
  ReceiverTable,
  RunLog,
  RunSummary,
  StandardDescriptor,
} from "./client";
import type { GeoJSONFeatureCollection, ModelFeature } from "@/model/types";
import { useModelStore } from "@/model/model-store";
import { getKernel } from "@/wasm/kernel";
import type {
  Barrier,
  ComputeRequest,
  Point2D,
  PointReceiver,
  ReceiverOutput,
  RoadSource,
} from "@/wasm/types";

const STORAGE_KEY = "aconiq.browser_backend.v1";
const DEFAULT_PROJECT_ID = "browser-project";
const DEFAULT_PROJECT_NAME = "Aconiq Browser Project";
const DEFAULT_PROJECT_PATH = "browser://local-storage";
const DEFAULT_CRS = "WGS84 / web map";
const DEFAULT_OSM_ENDPOINT = "https://overpass-api.de/api/interpreter";

type StoredArtifactContent = {
  mimeType: string;
  kind: string;
  encoding: "json" | "text";
  value: unknown;
};

type StoredRun = {
  run: RunSummary;
  log: RunLog;
  artifacts: Record<string, StoredArtifactContent>;
};

type BrowserBackendState = {
  projectId: string;
  projectName: string;
  projectPath: string;
  crs: string;
  runs: StoredRun[];
};

export type BrowserRunSpec = {
  standardId: string;
  version: string;
  profile: string;
  params: Record<string, string>;
  receiverMode: "auto-grid" | "custom";
};

type OverpassPoint = {
  lat: number;
  lon: number;
};

type OverpassWay = {
  type: "way";
  id: number;
  tags?: Record<string, string>;
  geometry?: OverpassPoint[];
};

type OverpassResponse = {
  elements?: Array<OverpassWay | Record<string, unknown>>;
};

const urlCache = new Map<string, string>();

const BROWSER_STANDARDS: StandardDescriptor[] = [
  {
    id: "rls19-road",
    description:
      "RLS-19 road noise computed locally in the browser via WebAssembly.",
    default_version: "2019",
    versions: [
      {
        name: "2019",
        default_profile: "default",
        profiles: [
          {
            name: "default",
            supported_source_types: ["line"],
            supported_indicators: ["LrDay", "LrNight"],
            parameters: [
              {
                name: "grid_resolution_m",
                kind: "float",
                required: true,
                default_value: "10",
                description: "Receiver grid spacing in map units",
                min: 0.001,
              },
              {
                name: "grid_padding_m",
                kind: "float",
                required: true,
                default_value: "20",
                description: "Padding around source extent in map units",
                min: 0,
              },
              {
                name: "receiver_height_m",
                kind: "float",
                required: true,
                default_value: "4",
                description: "Receiver height above ground",
                min: 0,
              },
              {
                name: "surface_type",
                kind: "string",
                required: true,
                default_value: "SMA",
                description: "Default road surface type",
                enum: [
                  "SMA",
                  "AB",
                  "OPA",
                  "Pflaster",
                  "Beton",
                  "LOA",
                  "DSH-V",
                  "Gussasphalt",
                  "beschaedigt",
                ],
              },
              {
                name: "speed_pkw_kph",
                kind: "float",
                required: true,
                default_value: "100",
                min: 0.001,
              },
              {
                name: "speed_lkw1_kph",
                kind: "float",
                required: true,
                default_value: "100",
                min: 0.001,
              },
              {
                name: "speed_lkw2_kph",
                kind: "float",
                required: true,
                default_value: "80",
                min: 0.001,
              },
              {
                name: "speed_krad_kph",
                kind: "float",
                required: true,
                default_value: "100",
                min: 0.001,
              },
              {
                name: "gradient_percent",
                kind: "float",
                required: true,
                default_value: "0",
                min: -12,
                max: 12,
              },
              {
                name: "traffic_day_pkw",
                kind: "float",
                required: true,
                default_value: "900",
                min: 0,
              },
              {
                name: "traffic_day_lkw1",
                kind: "float",
                required: true,
                default_value: "40",
                min: 0,
              },
              {
                name: "traffic_day_lkw2",
                kind: "float",
                required: true,
                default_value: "60",
                min: 0,
              },
              {
                name: "traffic_day_krad",
                kind: "float",
                required: true,
                default_value: "10",
                min: 0,
              },
              {
                name: "traffic_night_pkw",
                kind: "float",
                required: true,
                default_value: "200",
                min: 0,
              },
              {
                name: "traffic_night_lkw1",
                kind: "float",
                required: true,
                default_value: "10",
                min: 0,
              },
              {
                name: "traffic_night_lkw2",
                kind: "float",
                required: true,
                default_value: "20",
                min: 0,
              },
              {
                name: "traffic_night_krad",
                kind: "float",
                required: true,
                default_value: "2",
                min: 0,
              },
              {
                name: "segment_length_m",
                kind: "float",
                required: true,
                default_value: "1",
                min: 0.001,
              },
              {
                name: "min_distance_m",
                kind: "float",
                required: true,
                default_value: "3",
                min: 0.001,
              },
            ],
          },
        ],
      },
    ],
  },
];

function initialState(): BrowserBackendState {
  return {
    projectId: DEFAULT_PROJECT_ID,
    projectName: DEFAULT_PROJECT_NAME,
    projectPath: DEFAULT_PROJECT_PATH,
    crs: DEFAULT_CRS,
    runs: [],
  };
}

function readState(): BrowserBackendState {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return initialState();
    const parsed = JSON.parse(raw) as BrowserBackendState;
    return {
      ...initialState(),
      ...parsed,
      runs: parsed.runs ?? [],
    };
  } catch {
    return initialState();
  }
}

function writeState(state: BrowserBackendState): void {
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  for (const url of urlCache.values()) {
    URL.revokeObjectURL(url);
  }
  urlCache.clear();
}

function nowISO(): string {
  return new Date().toISOString();
}

function formatRunIndex(index: number): string {
  return String(index + 1).padStart(4, "0");
}

function parseNumber(
  params: Record<string, string>,
  key: string,
  fallback: number,
): number {
  const parsed = Number.parseFloat(params[key] ?? "");
  return Number.isFinite(parsed) ? parsed : fallback;
}

function readArtifact<T>(artifactId: string): T {
  const state = readState();
  for (const storedRun of state.runs) {
    const artifact = storedRun.artifacts[artifactId];
    if (!artifact) continue;
    return artifact.value as T;
  }
  throw new Error(`Artifact ${artifactId} not found`);
}

function setRun(state: BrowserBackendState, storedRun: StoredRun): BrowserBackendState {
  const nextRuns = state.runs.filter((entry) => entry.run.id !== storedRun.run.id);
  nextRuns.push(storedRun);
  nextRuns.sort((a, b) => b.run.started_at.localeCompare(a.run.started_at));
  return { ...state, runs: nextRuns };
}

function getFeatureBBox(features: ModelFeature[]): {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
} | null {
  let minX = Number.POSITIVE_INFINITY;
  let minY = Number.POSITIVE_INFINITY;
  let maxX = Number.NEGATIVE_INFINITY;
  let maxY = Number.NEGATIVE_INFINITY;

  function visit(coords: unknown): void {
    if (!Array.isArray(coords)) return;
    if (
      coords.length >= 2 &&
      typeof coords[0] === "number" &&
      typeof coords[1] === "number"
    ) {
      const x = coords[0];
      const y = coords[1];
      minX = Math.min(minX, x);
      minY = Math.min(minY, y);
      maxX = Math.max(maxX, x);
      maxY = Math.max(maxY, y);
      return;
    }
    for (const item of coords) visit(item);
  }

  for (const feature of features) visit(feature.geometry.coordinates);

  if (!Number.isFinite(minX)) return null;
  return { minX, minY, maxX, maxY };
}

function toPoint2D(position: unknown): Point2D | null {
  if (
    Array.isArray(position) &&
    position.length >= 2 &&
    typeof position[0] === "number" &&
    typeof position[1] === "number"
  ) {
    return { x: position[0], y: position[1] };
  }
  return null;
}

function toLineStrings(coords: unknown): Point2D[][] {
  if (!Array.isArray(coords)) return [];
  const maybeLine = coords.map((point) => toPoint2D(point)).filter(Boolean);
  if (maybeLine.length > 0) {
    return [maybeLine as Point2D[]];
  }
  const lines: Point2D[][] = [];
  for (const item of coords) {
    lines.push(...toLineStrings(item));
  }
  return lines;
}

function buildRoadSources(
  features: ModelFeature[],
  params: Record<string, string>,
): RoadSource[] {
  const surfaceType = params["surface_type"] || "SMA";
  const speeds = {
    pkw_kph: parseNumber(params, "speed_pkw_kph", 100),
    lkw1_kph: parseNumber(params, "speed_lkw1_kph", 100),
    lkw2_kph: parseNumber(params, "speed_lkw2_kph", 80),
    krad_kph: parseNumber(params, "speed_krad_kph", 100),
  };
  const trafficDay = {
    pkw_per_hour: parseNumber(params, "traffic_day_pkw", 900),
    lkw1_per_hour: parseNumber(params, "traffic_day_lkw1", 40),
    lkw2_per_hour: parseNumber(params, "traffic_day_lkw2", 60),
    krad_per_hour: parseNumber(params, "traffic_day_krad", 10),
  };
  const trafficNight = {
    pkw_per_hour: parseNumber(params, "traffic_night_pkw", 200),
    lkw1_per_hour: parseNumber(params, "traffic_night_lkw1", 10),
    lkw2_per_hour: parseNumber(params, "traffic_night_lkw2", 20),
    krad_per_hour: parseNumber(params, "traffic_night_krad", 2),
  };
  const gradientPercent = parseNumber(params, "gradient_percent", 0);

  const sources: RoadSource[] = [];
  for (const feature of features) {
    if (feature.kind !== "source" || feature.sourceType !== "line") continue;
    const centerlines = toLineStrings(feature.geometry.coordinates);
    centerlines.forEach((centerline, index) => {
      if (centerline.length < 2) return;
      sources.push({
        id:
          centerlines.length === 1
            ? feature.id
            : `${feature.id}-${String(index + 1)}`,
        centerline,
        surface_type: surfaceType,
        speeds,
        gradient_percent: gradientPercent,
        traffic_day: trafficDay,
        traffic_night: trafficNight,
      });
    });
  }
  return sources;
}

function buildBarriers(features: ModelFeature[]): Barrier[] {
  const barriers: Barrier[] = [];
  for (const feature of features) {
    if (feature.kind !== "barrier") continue;
    const lines = toLineStrings(feature.geometry.coordinates);
    lines.forEach((geometry, index) => {
      if (geometry.length < 2) return;
      barriers.push({
        id: lines.length === 1 ? feature.id : `${feature.id}-${String(index + 1)}`,
        geometry,
        height_m: feature.heightM ?? 2,
      });
    });
  }
  return barriers;
}

function buildReceiverGrid(
  bbox: { minX: number; minY: number; maxX: number; maxY: number },
  params: Record<string, string>,
): { receivers: PointReceiver[]; width: number; height: number } {
  const resolution = parseNumber(params, "grid_resolution_m", 10);
  const padding = parseNumber(params, "grid_padding_m", 20);
  const receiverHeight = parseNumber(params, "receiver_height_m", 4);

  const minX = bbox.minX - padding;
  const minY = bbox.minY - padding;
  const maxX = bbox.maxX + padding;
  const maxY = bbox.maxY + padding;

  const width = Math.max(1, Math.floor((maxX - minX) / resolution) + 1);
  const height = Math.max(1, Math.floor((maxY - minY) / resolution) + 1);

  const receivers: PointReceiver[] = [];
  let seq = 1;
  for (let row = 0; row < height; row++) {
    const y = minY + row * resolution;
    for (let col = 0; col < width; col++) {
      const x = minX + col * resolution;
      receivers.push({
        id: `R${String(seq).padStart(4, "0")}`,
        point: { x, y },
        height_m: receiverHeight,
      });
      seq++;
    }
  }

  return { receivers, width, height };
}

async function sha256Hex(value: string): Promise<string> {
  const bytes = new TextEncoder().encode(value);
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest))
    .map((part) => part.toString(16).padStart(2, "0"))
    .join("");
}

function buildReceiverTable(outputs: ReceiverOutput[]): ReceiverTable {
  return {
    indicator_order: ["LrDay", "LrNight"],
    unit: "dB(A)",
    records: outputs.map((output) => ({
      id: output.Receiver.id,
      x: output.Receiver.point.x,
      y: output.Receiver.point.y,
      height_m: output.Receiver.height_m,
      values: {
        LrDay: output.Indicators.lr_day,
        LrNight: output.Indicators.lr_night,
      },
    })),
  };
}

function buildReceiverCSV(table: ReceiverTable): string {
  const headers = ["id", "x", "y", "height_m", ...table.indicator_order];
  const rows = table.records.map((record) => [
    record.id,
    String(record.x),
    String(record.y),
    String(record.height_m),
    ...table.indicator_order.map((indicator) => String(record.values[indicator] ?? "")),
  ]);
  return [headers, ...rows]
    .map((row) => row.map((value) => `"${value.replaceAll('"', '""')}"`).join(","))
    .join("\n");
}

function makeArtifact(
  runId: string,
  suffix: string,
  kind: string,
  path: string,
  createdAt: string,
): ArtifactRef {
  return {
    id: `artifact-${runId}-${suffix}`,
    kind,
    path,
    created_at: createdAt,
  };
}

function browserExportHTML(run: RunSummary, table: ReceiverTable): string {
  const previewRows = table.records.slice(0, 20);
  const rowHtml = previewRows
    .map(
      (record) =>
        `<tr><td>${record.id}</td><td>${record.x.toFixed(2)}</td><td>${record.y.toFixed(2)}</td><td>${record.values.LrDay.toFixed(1)}</td><td>${record.values.LrNight.toFixed(1)}</td></tr>`,
    )
    .join("");
  return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>Aconiq Export ${run.id}</title>
    <style>
      body { font-family: "IBM Plex Sans", sans-serif; margin: 2rem; color: #1f2937; }
      table { border-collapse: collapse; width: 100%; margin-top: 1rem; }
      th, td { border: 1px solid #d1d5db; padding: 0.5rem; text-align: left; font-size: 0.875rem; }
      th { background: #f3f4f6; }
      .meta { color: #6b7280; font-size: 0.875rem; }
    </style>
  </head>
  <body>
    <h1>Aconiq Export</h1>
    <p class="meta">Run ${run.id} · ${run.standard_id} / ${run.version}${run.profile ? ` / ${run.profile}` : ""}</p>
    <p>Receiver preview (${previewRows.length} of ${table.records.length})</p>
    <table>
      <thead>
        <tr><th>ID</th><th>X</th><th>Y</th><th>LrDay</th><th>LrNight</th></tr>
      </thead>
      <tbody>${rowHtml}</tbody>
    </table>
  </body>
</html>`;
}

function browserExportMarkdown(run: RunSummary, table: ReceiverTable): string {
  const previewRows = table.records
    .slice(0, 10)
    .map(
      (record) =>
        `| ${record.id} | ${record.x.toFixed(2)} | ${record.y.toFixed(2)} | ${record.values.LrDay.toFixed(1)} | ${record.values.LrNight.toFixed(1)} |`,
    )
    .join("\n");
  return `# Aconiq Export

Run: \`${run.id}\`

Standard: \`${run.standard_id}\` / \`${run.version}\`${run.profile ? ` / \`${run.profile}\`` : ""}

## Receiver Preview

| ID | X | Y | LrDay | LrNight |
| --- | ---: | ---: | ---: | ---: |
${previewRows}
`;
}

function findRunByID(state: BrowserBackendState, runId: string): StoredRun {
  const storedRun = state.runs.find((entry) => entry.run.id === runId);
  if (!storedRun) {
    throw new Error(`Run ${runId} not found`);
  }
  return storedRun;
}

export const browserBackend = {
  async getHealth(): Promise<HealthResponse> {
    await getKernel();
    return {
      status: "ok",
      version: "wasm-browser",
      time: nowISO(),
    };
  },

  async getProjectStatus(): Promise<ProjectStatusResponse> {
    const state = readState();
    const features = useModelStore.getState().features;
    const lastRun = state.runs
      .map((entry) => entry.run)
      .sort((a, b) => b.started_at.localeCompare(a.started_at))[0];

    return {
      project_id: state.projectId,
      name:
        features.length > 0 ? `${state.projectName} (${features.length} features)` : state.projectName,
      project_path: state.projectPath,
      manifest_version: 1,
      crs: state.crs,
      scenario_count: 1,
      run_count: state.runs.length,
      ...(lastRun
        ? {
            last_run: {
              id: lastRun.id,
              status: lastRun.status,
              standard_id: lastRun.standard_id,
              version: lastRun.version,
              ...(lastRun.profile ? { profile: lastRun.profile } : {}),
              started_at: lastRun.started_at,
              finished_at: lastRun.finished_at,
            },
          }
        : {}),
    };
  },

  async getStandards(): Promise<StandardDescriptor[]> {
    return BROWSER_STANDARDS;
  },

  async getRuns(): Promise<RunSummary[]> {
    return readState().runs
      .map((entry) => entry.run)
      .sort((a, b) => b.started_at.localeCompare(a.started_at));
  },

  async getRunLog(runId: string): Promise<RunLog> {
    return findRunByID(readState(), runId).log;
  },

  async getArtifactContent<T>(artifactId: string): Promise<T> {
    return readArtifact<T>(artifactId);
  },

  getArtifactURL(artifactId: string): string {
    const cached = urlCache.get(artifactId);
    if (cached) return cached;
    const state = readState();
    for (const storedRun of state.runs) {
      const content = storedRun.artifacts[artifactId];
      if (!content) continue;
      const body =
        content.encoding === "json"
          ? JSON.stringify(content.value, null, 2)
          : String(content.value);
      const url = URL.createObjectURL(new Blob([body], { type: content.mimeType }));
      urlCache.set(artifactId, url);
      return url;
    }
    throw new Error(`Artifact ${artifactId} not found`);
  },

  async importFromOSM(req: {
    south: number;
    west: number;
    north: number;
    east: number;
    overpass_endpoint?: string;
  }): Promise<GeoJSONFeatureCollection> {
    const endpoint = req.overpass_endpoint || DEFAULT_OSM_ENDPOINT;
    const query = `[out:json][timeout:25];
(
  way["highway"](${req.south},${req.west},${req.north},${req.east});
  way["railway"~"^(rail|tram)$"](${req.south},${req.west},${req.north},${req.east});
  way["building"](${req.south},${req.west},${req.north},${req.east});
  way["barrier"~"^(wall|fence)$"](${req.south},${req.west},${req.north},${req.east});
);
out geom;`;

    const response = await fetch(endpoint, {
      method: "POST",
      headers: {
        Accept: "application/json",
        "Content-Type": "text/plain;charset=UTF-8",
      },
      body: query,
    });
    if (!response.ok) {
      throw new Error(`Request failed: ${String(response.status)}`);
    }
    const data = (await response.json()) as OverpassResponse;
    const features = (data.elements ?? [])
      .filter((element): element is OverpassWay => {
        return element["type"] === "way";
      })
      .map((way) => overpassWayToFeature(way))
      .filter((feature): feature is GeoJSONFeatureCollection["features"][number] => {
        return feature !== null;
      });
    return { type: "FeatureCollection", features };
  },

  async startRun(spec: BrowserRunSpec): Promise<RunSummary> {
    if (spec.standardId !== "rls19-road") {
      throw new Error(`Standard ${spec.standardId} is not available in browser mode`);
    }
    if (spec.receiverMode !== "auto-grid") {
      throw new Error("Only auto-grid receivers are supported in browser mode");
    }

    const features = useModelStore.getState().features;
    const sources = buildRoadSources(features, spec.params);
    if (sources.length === 0) {
      throw new Error("Browser mode currently requires at least one line source");
    }

    const barriers = buildBarriers(features);
    const bbox = getFeatureBBox(features.filter((feature) => feature.kind === "source"));
    if (!bbox) {
      throw new Error("Could not derive source extent from the current model");
    }

    const receiverGrid = buildReceiverGrid(bbox, spec.params);
    const startedAt = nowISO();
    const stateBefore = readState();
    const runId = `run-${formatRunIndex(stateBefore.runs.length)}`;
    const basePath = `${DEFAULT_PROJECT_PATH}/runs/${runId}`;

    const kernel = await getKernel();
    const request: ComputeRequest = {
      receivers: receiverGrid.receivers,
      sources,
      barriers,
      config: {
        SegmentLengthM: parseNumber(spec.params, "segment_length_m", 1),
        MinDistanceM: parseNumber(spec.params, "min_distance_m", 3),
        ReceiverHeightM: parseNumber(spec.params, "receiver_height_m", 4),
      },
    };

    const outputs = await kernel.rls19Road(request);
    const receiverTable = buildReceiverTable(outputs);
    const receiverCSV = buildReceiverCSV(receiverTable);
    const rasterMetadata: RasterMetadata = {
      width: receiverGrid.width,
      height: receiverGrid.height,
      bands: 2,
      nodata: -9999,
      unit: "dB(A)",
      band_names: ["LrDay", "LrNight"],
    };
    const summary = {
      run_id: runId,
      status: "completed",
      grid_width: receiverGrid.width,
      grid_height: receiverGrid.height,
      source_count: sources.length,
      receiver_count: outputs.length,
      reporting_precision_db: 0.1,
    };

    const hashPayload = outputs.map((output) => ({
      receiver_id: output.Receiver.id,
      indicators: output.Indicators,
    }));
    const outputHash = await sha256Hex(JSON.stringify(hashPayload));
    const finishedAt = nowISO();
    const createdAt = finishedAt;

    const artifacts: ArtifactRef[] = [
      makeArtifact(
        runId,
        "receivers-json",
        "run.result.receiver_table_json",
        `${basePath}/results/receivers.json`,
        createdAt,
      ),
      makeArtifact(
        runId,
        "receivers-csv",
        "run.result.receiver_table_csv",
        `${basePath}/results/receivers.csv`,
        createdAt,
      ),
      makeArtifact(
        runId,
        "raster-meta",
        "run.result.raster_metadata",
        `${basePath}/results/rls19-road.json`,
        createdAt,
      ),
      makeArtifact(
        runId,
        "raster-bin",
        "run.result.raster_binary",
        `${basePath}/results/rls19-road.bin`,
        createdAt,
      ),
      makeArtifact(
        runId,
        "summary",
        "run.result.summary",
        `${basePath}/results/run-summary.json`,
        createdAt,
      ),
    ];

    const run: RunSummary = {
      id: runId,
      scenario_id: "default",
      standard_id: spec.standardId,
      version: spec.version,
      ...(spec.profile ? { profile: spec.profile } : {}),
      status: "completed",
      started_at: startedAt,
      finished_at: finishedAt,
      log_path: `${basePath}/run.log`,
      artifacts,
    };

    const log: RunLog = {
      run_id: runId,
      lines: [
        `${startedAt} run started`,
        `${startedAt} model=browser`,
        `${startedAt} rls19_road_sources=${sources.length}`,
        `${startedAt} receivers=${receiverGrid.receivers.length}`,
        `${startedAt} stage=compute`,
        `${finishedAt} output_hash=${outputHash}`,
        `${finishedAt} persisted=browser`,
        `${finishedAt} run completed`,
      ],
    };

    const artifactMap: Record<string, StoredArtifactContent> = {
      [artifacts[0].id]: {
        kind: artifacts[0].kind,
        mimeType: "application/json",
        encoding: "json",
        value: receiverTable,
      },
      [artifacts[1].id]: {
        kind: artifacts[1].kind,
        mimeType: "text/csv",
        encoding: "text",
        value: receiverCSV,
      },
      [artifacts[2].id]: {
        kind: artifacts[2].kind,
        mimeType: "application/json",
        encoding: "json",
        value: rasterMetadata,
      },
      [artifacts[3].id]: {
        kind: artifacts[3].kind,
        mimeType: "application/octet-stream",
        encoding: "text",
        value: outputHash,
      },
      [artifacts[4].id]: {
        kind: artifacts[4].kind,
        mimeType: "application/json",
        encoding: "json",
        value: { ...summary, output_hash: outputHash },
      },
    };

    const nextState = setRun(readState(), { run, log, artifacts: artifactMap });
    writeState(nextState);
    return run;
  },

  async createExport(runId: string): Promise<RunSummary> {
    const state = readState();
    const storedRun = findRunByID(state, runId);
    const tableArtifact = storedRun.run.artifacts.find(
      (artifact) => artifact.kind === "run.result.receiver_table_json",
    );
    if (!tableArtifact) {
      throw new Error("Run has no receiver table artifact");
    }
    const receiverTable = storedRun.artifacts[tableArtifact.id]?.value as ReceiverTable;
    const exportedAt = nowISO();
    const exportBase = `${DEFAULT_PROJECT_PATH}/exports/${runId}-${exportedAt.replaceAll(":", "").replaceAll(".", "")}`;

    const context = {
      exported_at: exportedAt,
      project_id: DEFAULT_PROJECT_ID,
      run: storedRun.run,
      receiver_table: receiverTable,
    };
    const html = browserExportHTML(storedRun.run, receiverTable);
    const markdown = browserExportMarkdown(storedRun.run, receiverTable);
    const bundleSummary = {
      export_id: `${runId}-${exportedAt}`,
      run_id: runId,
      exported_at: exportedAt,
      copied_files: [
        "results/receivers.json",
        "results/receivers.csv",
        "results/run-summary.json",
      ],
      generated_reports: [
        "report/report-context.json",
        "report/report.md",
        "report/report.html",
      ],
    };

    const exportArtifacts: ArtifactRef[] = [
      makeArtifact(
        runId,
        `export-bundle-${storedRun.run.artifacts.filter((artifact) => artifact.kind.startsWith("export.")).length + 1}`,
        "export.bundle",
        `${exportBase}/export-summary.json`,
        exportedAt,
      ),
      makeArtifact(
        runId,
        `export-context-${Date.now()}`,
        "export.report_context_json",
        `${exportBase}/report/report-context.json`,
        exportedAt,
      ),
      makeArtifact(
        runId,
        `export-markdown-${Date.now()}`,
        "export.report_markdown",
        `${exportBase}/report/report.md`,
        exportedAt,
      ),
      makeArtifact(
        runId,
        `export-html-${Date.now()}`,
        "export.report_html",
        `${exportBase}/report/report.html`,
        exportedAt,
      ),
    ];

    const nextStoredRun: StoredRun = {
      run: {
        ...storedRun.run,
        artifacts: [...storedRun.run.artifacts, ...exportArtifacts],
      },
      log: storedRun.log,
      artifacts: {
        ...storedRun.artifacts,
        [exportArtifacts[0].id]: {
          kind: exportArtifacts[0].kind,
          mimeType: "application/json",
          encoding: "json",
          value: bundleSummary,
        },
        [exportArtifacts[1].id]: {
          kind: exportArtifacts[1].kind,
          mimeType: "application/json",
          encoding: "json",
          value: context,
        },
        [exportArtifacts[2].id]: {
          kind: exportArtifacts[2].kind,
          mimeType: "text/markdown",
          encoding: "text",
          value: markdown,
        },
        [exportArtifacts[3].id]: {
          kind: exportArtifacts[3].kind,
          mimeType: "text/html",
          encoding: "text",
          value: html,
        },
      },
    };

    writeState(setRun(state, nextStoredRun));
    return nextStoredRun.run;
  },
};

function overpassWayToFeature(
  way: OverpassWay,
): GeoJSONFeatureCollection["features"][number] | null {
  const geometry = sanitizeOverpassGeometry(way.geometry);
  if (geometry.length < 2) return null;
  const tags = way.tags ?? {};
  const properties: Record<string, unknown> = {
    osm_id: String(way.id),
  };

  if (tags["highway"]) {
    properties["kind"] = "source";
    properties["source_type"] = "line";
    properties["highway"] = tags["highway"];
    return {
      type: "Feature",
      id: `osm-way-${way.id}`,
      properties,
      geometry: {
        type: "LineString",
        coordinates: geometry.map((point) => [point.lon, point.lat]),
      },
    };
  }

  if (tags["railway"]) {
    properties["kind"] = "source";
    properties["source_type"] = "line";
    properties["railway"] = tags["railway"];
    return {
      type: "Feature",
      id: `osm-way-${way.id}`,
      properties,
      geometry: {
        type: "LineString",
        coordinates: geometry.map((point) => [point.lon, point.lat]),
      },
    };
  }

  if (tags["building"]) {
    const ring = geometry.map((point) => [point.lon, point.lat]);
    if (ring.length < 3) return null;
    const first = ring[0];
    const last = ring[ring.length - 1];
    if (first[0] !== last[0] || first[1] !== last[1]) {
      ring.push([first[0], first[1]]);
    }
    properties["kind"] = "building";
    properties["building"] = tags["building"];
    if (tags["building:levels"]) {
      properties["building:levels"] = tags["building:levels"];
    }
    properties["height_m"] = parseTagHeight(tags["height"]) ?? 9;
    return {
      type: "Feature",
      id: `osm-way-${way.id}`,
      properties,
      geometry: {
        type: "Polygon",
        coordinates: [ring],
      },
    };
  }

  if (tags["barrier"]) {
    properties["kind"] = "barrier";
    properties["barrier"] = tags["barrier"];
    properties["height_m"] = parseTagHeight(tags["height"]) ?? 2;
    return {
      type: "Feature",
      id: `osm-way-${way.id}`,
      properties,
      geometry: {
        type: "LineString",
        coordinates: geometry.map((point) => [point.lon, point.lat]),
      },
    };
  }

  return null;
}

function parseTagHeight(value: string | undefined): number | null {
  if (!value) return null;
  const parsed = Number.parseFloat(value.replace(" m", "").replace("m", "").trim());
  return Number.isFinite(parsed) ? parsed : null;
}

function sanitizeOverpassGeometry(
  geometry: OverpassPoint[] | undefined,
): OverpassPoint[] {
  if (!geometry) return [];
  return geometry.filter((point) => {
    return (
      point != null &&
      Number.isFinite(point.lon) &&
      Number.isFinite(point.lat)
    );
  });
}
