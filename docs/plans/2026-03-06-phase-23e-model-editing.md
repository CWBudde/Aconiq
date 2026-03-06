# Phase 23e: Model Editing Workflows — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable practical model authoring and correction from the GUI — drawing features on the map, editing attributes, validating models, importing GeoJSON files, and undoing/redoing edits.

**Architecture:** Frontend-first with API stubs (Option 3). All edits accumulate in a Zustand model store holding GeoJSON features in-memory. A command stack enables undo/redo. Drawing uses `terra-draw` (map-library-agnostic drawing toolkit). Validation runs client-side mirroring the Go backend rules. An import assistant lets users upload GeoJSON, preview normalized features, and confirm. Real backend API integration is deferred — typed interfaces define the boundary.

**Tech Stack:** React 19, TypeScript (strict), Zustand, MapLibre GL JS, terra-draw, Vitest, existing shadcn/ui components

---

## Task 1: Model Types

Define TypeScript types mirroring the Go backend's `modelgeojson` package. These are the core data types used by every subsequent task.

**Files:**

- Create: `frontend/src/model/types.ts`
- Test: `frontend/src/model/types.test.ts`

**Step 1: Write the type definitions**

```typescript
// frontend/src/model/types.ts

/** Feature kind — matches Go backend's normalized model */
export type FeatureKind = "source" | "building" | "barrier";

/** Source geometry subtype */
export type SourceType = "point" | "line" | "area";

/** GeoJSON geometry types we support */
export type GeometryType =
  | "Point"
  | "MultiPoint"
  | "LineString"
  | "MultiLineString"
  | "Polygon"
  | "MultiPolygon";

/** A position is [lng, lat] or [x, y] */
export type Position = [number, number];

/** GeoJSON geometry object */
export interface Geometry {
  type: GeometryType;
  coordinates: Position | Position[] | Position[][] | Position[][][];
}

/** A normalized model feature (mirrors Go Feature struct) */
export interface ModelFeature {
  id: string;
  kind: FeatureKind;
  sourceType?: SourceType;
  heightM?: number;
  geometry: Geometry;
}

/** Validation issue severity */
export type IssueSeverity = "error" | "warning";

/** A single validation finding */
export interface ValidationIssue {
  level: IssueSeverity;
  code: string;
  featureId: string;
  message: string;
}

/** Full validation report */
export interface ValidationReport {
  valid: boolean;
  errors: ValidationIssue[];
  warnings: ValidationIssue[];
  checkedAt: string;
}

/** GeoJSON FeatureCollection for import/export */
export interface GeoJSONFeatureCollection {
  type: "FeatureCollection";
  features: GeoJSONFeature[];
  crs?: Record<string, unknown>;
}

export interface GeoJSONFeature {
  type: "Feature";
  id?: string | number;
  properties: Record<string, unknown>;
  geometry: {
    type: string;
    coordinates: unknown;
  };
}

/** Create a new feature ID */
export function createFeatureId(): string {
  return crypto.randomUUID();
}

/** Check if a geometry type is compatible with a source type */
export function isGeometryCompatible(
  geometryType: GeometryType,
  sourceType: SourceType,
): boolean {
  switch (sourceType) {
    case "point":
      return geometryType === "Point" || geometryType === "MultiPoint";
    case "line":
      return (
        geometryType === "LineString" || geometryType === "MultiLineString"
      );
    case "area":
      return geometryType === "Polygon" || geometryType === "MultiPolygon";
  }
}
```

**Step 2: Write tests for utility functions**

```typescript
// frontend/src/model/types.test.ts
import { describe, expect, it } from "vitest";
import { createFeatureId, isGeometryCompatible } from "./types";

describe("createFeatureId", () => {
  it("returns a non-empty string", () => {
    const id = createFeatureId();
    expect(id).toBeTruthy();
    expect(typeof id).toBe("string");
  });

  it("returns unique ids", () => {
    const ids = new Set(Array.from({ length: 100 }, () => createFeatureId()));
    expect(ids.size).toBe(100);
  });
});

describe("isGeometryCompatible", () => {
  it("point source accepts Point geometry", () => {
    expect(isGeometryCompatible("Point", "point")).toBe(true);
  });

  it("point source accepts MultiPoint geometry", () => {
    expect(isGeometryCompatible("MultiPoint", "point")).toBe(true);
  });

  it("point source rejects Polygon geometry", () => {
    expect(isGeometryCompatible("Polygon", "point")).toBe(false);
  });

  it("line source accepts LineString geometry", () => {
    expect(isGeometryCompatible("LineString", "line")).toBe(true);
  });

  it("area source accepts Polygon geometry", () => {
    expect(isGeometryCompatible("Polygon", "area")).toBe(true);
  });

  it("area source rejects Point geometry", () => {
    expect(isGeometryCompatible("Point", "area")).toBe(false);
  });
});
```

**Step 3: Run tests**

Run: `cd frontend && bun run test -- src/model/types.test.ts`
Expected: PASS

**Step 4: Commit**

```
feat(model): add model feature types mirroring Go backend schema
```

---

## Task 2: Command Stack (Undo/Redo)

Build the undo/redo infrastructure before the model store, so the store can use it from the start.

**Files:**

- Create: `frontend/src/model/command-stack.ts`
- Test: `frontend/src/model/command-stack.test.ts`

**Step 1: Write the failing tests**

```typescript
// frontend/src/model/command-stack.test.ts
import { describe, expect, it, vi } from "vitest";
import { CommandStack, type Command } from "./command-stack";

function makeCommand(log: string[]): Command {
  return {
    description: "test",
    execute: vi.fn(() => {
      log.push("do");
    }),
    undo: vi.fn(() => {
      log.push("undo");
    }),
  };
}

describe("CommandStack", () => {
  it("starts with empty undo/redo", () => {
    const stack = new CommandStack();
    expect(stack.canUndo()).toBe(false);
    expect(stack.canRedo()).toBe(false);
  });

  it("execute runs the command and enables undo", () => {
    const log: string[] = [];
    const stack = new CommandStack();
    const cmd = makeCommand(log);
    stack.execute(cmd);
    expect(log).toEqual(["do"]);
    expect(stack.canUndo()).toBe(true);
    expect(stack.canRedo()).toBe(false);
  });

  it("undo reverses the last command", () => {
    const log: string[] = [];
    const stack = new CommandStack();
    stack.execute(makeCommand(log));
    stack.undo();
    expect(log).toEqual(["do", "undo"]);
    expect(stack.canUndo()).toBe(false);
    expect(stack.canRedo()).toBe(true);
  });

  it("redo re-applies the last undone command", () => {
    const log: string[] = [];
    const stack = new CommandStack();
    stack.execute(makeCommand(log));
    stack.undo();
    stack.redo();
    expect(log).toEqual(["do", "undo", "do"]);
    expect(stack.canUndo()).toBe(true);
    expect(stack.canRedo()).toBe(false);
  });

  it("new command after undo clears redo stack", () => {
    const log: string[] = [];
    const stack = new CommandStack();
    stack.execute(makeCommand(log));
    stack.undo();
    stack.execute(makeCommand(log));
    expect(stack.canRedo()).toBe(false);
  });

  it("respects max stack size", () => {
    const log: string[] = [];
    const stack = new CommandStack(3);
    for (let i = 0; i < 5; i++) {
      stack.execute(makeCommand(log));
    }
    // Only 3 undos possible
    let undoCount = 0;
    while (stack.canUndo()) {
      stack.undo();
      undoCount++;
    }
    expect(undoCount).toBe(3);
  });

  it("clear removes all history", () => {
    const log: string[] = [];
    const stack = new CommandStack();
    stack.execute(makeCommand(log));
    stack.clear();
    expect(stack.canUndo()).toBe(false);
    expect(stack.canRedo()).toBe(false);
  });

  it("notifies listener on changes", () => {
    const stack = new CommandStack();
    const listener = vi.fn();
    stack.subscribe(listener);
    stack.execute(makeCommand([]));
    expect(listener).toHaveBeenCalledTimes(1);
    stack.undo();
    expect(listener).toHaveBeenCalledTimes(2);
  });
});
```

**Step 2: Run tests to verify they fail**

Run: `cd frontend && bun run test -- src/model/command-stack.test.ts`
Expected: FAIL — module not found

**Step 3: Implement CommandStack**

```typescript
// frontend/src/model/command-stack.ts

export interface Command {
  description: string;
  execute: () => void;
  undo: () => void;
}

type Listener = () => void;

export class CommandStack {
  private undoStack: Command[] = [];
  private redoStack: Command[] = [];
  private readonly maxSize: number;
  private listeners: Listener[] = [];

  constructor(maxSize = 50) {
    this.maxSize = maxSize;
  }

  execute(command: Command): void {
    command.execute();
    this.undoStack.push(command);
    if (this.undoStack.length > this.maxSize) {
      this.undoStack.shift();
    }
    this.redoStack = [];
    this.notify();
  }

  undo(): void {
    const command = this.undoStack.pop();
    if (!command) return;
    command.undo();
    this.redoStack.push(command);
    this.notify();
  }

  redo(): void {
    const command = this.redoStack.pop();
    if (!command) return;
    command.execute();
    this.undoStack.push(command);
    this.notify();
  }

  canUndo(): boolean {
    return this.undoStack.length > 0;
  }

  canRedo(): boolean {
    return this.redoStack.length > 0;
  }

  clear(): void {
    this.undoStack = [];
    this.redoStack = [];
    this.notify();
  }

  subscribe(listener: Listener): () => void {
    this.listeners.push(listener);
    return () => {
      this.listeners = this.listeners.filter((l) => l !== listener);
    };
  }

  private notify(): void {
    for (const listener of this.listeners) {
      listener();
    }
  }
}
```

**Step 4: Run tests**

Run: `cd frontend && bun run test -- src/model/command-stack.test.ts`
Expected: PASS

**Step 5: Commit**

```
feat(model): add undo/redo command stack
```

---

## Task 3: Model Store

Zustand store holding all model features with CRUD operations routed through the command stack.

**Files:**

- Create: `frontend/src/model/model-store.ts`
- Test: `frontend/src/model/model-store.test.ts`

**Step 1: Write failing tests**

```typescript
// frontend/src/model/model-store.test.ts
import { describe, expect, it, beforeEach } from "vitest";
import { useModelStore } from "./model-store";
import type { ModelFeature } from "./types";

const pointSource: ModelFeature = {
  id: "src-1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

const building: ModelFeature = {
  id: "bld-1",
  kind: "building",
  heightM: 12,
  geometry: {
    type: "Polygon",
    coordinates: [
      [
        [0, 0],
        [1, 0],
        [1, 1],
        [0, 1],
        [0, 0],
      ],
    ],
  },
};

beforeEach(() => {
  useModelStore.getState().reset();
});

describe("model store", () => {
  it("starts empty", () => {
    expect(useModelStore.getState().features).toEqual([]);
  });

  it("addFeature adds a feature", () => {
    useModelStore.getState().addFeature(pointSource);
    expect(useModelStore.getState().features).toEqual([pointSource]);
  });

  it("addFeature is undoable", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().undo();
    expect(useModelStore.getState().features).toEqual([]);
  });

  it("updateFeature replaces a feature by id", () => {
    useModelStore.getState().addFeature(pointSource);
    const updated = {
      ...pointSource,
      geometry: {
        type: "Point" as const,
        coordinates: [11, 52] as [number, number],
      },
    };
    useModelStore.getState().updateFeature(updated);
    expect(useModelStore.getState().features[0]?.geometry.coordinates).toEqual([
      11, 52,
    ]);
  });

  it("updateFeature is undoable", () => {
    useModelStore.getState().addFeature(pointSource);
    const updated = {
      ...pointSource,
      geometry: {
        type: "Point" as const,
        coordinates: [11, 52] as [number, number],
      },
    };
    useModelStore.getState().updateFeature(updated);
    useModelStore.getState().undo();
    expect(useModelStore.getState().features[0]?.geometry.coordinates).toEqual([
      10, 51,
    ]);
  });

  it("removeFeature removes a feature by id", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().removeFeature("src-1");
    expect(useModelStore.getState().features).toEqual([]);
  });

  it("removeFeature is undoable", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().removeFeature("src-1");
    useModelStore.getState().undo();
    expect(useModelStore.getState().features).toEqual([pointSource]);
  });

  it("loadFeatures replaces all features (not undoable)", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().loadFeatures([building]);
    expect(useModelStore.getState().features).toEqual([building]);
  });

  it("getFeatureById returns the correct feature", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().addFeature(building);
    expect(useModelStore.getState().getFeatureById("bld-1")).toEqual(building);
  });

  it("getFeatureById returns undefined for missing id", () => {
    expect(useModelStore.getState().getFeatureById("nope")).toBeUndefined();
  });

  it("featuresByKind filters correctly", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().addFeature(building);
    expect(useModelStore.getState().featuresByKind("source")).toEqual([
      pointSource,
    ]);
    expect(useModelStore.getState().featuresByKind("building")).toEqual([
      building,
    ]);
  });

  it("dirty flag is false initially and true after edits", () => {
    expect(useModelStore.getState().dirty).toBe(false);
    useModelStore.getState().addFeature(pointSource);
    expect(useModelStore.getState().dirty).toBe(true);
  });

  it("markClean resets dirty flag", () => {
    useModelStore.getState().addFeature(pointSource);
    useModelStore.getState().markClean();
    expect(useModelStore.getState().dirty).toBe(false);
  });
});
```

**Step 2: Run tests to verify they fail**

Run: `cd frontend && bun run test -- src/model/model-store.test.ts`
Expected: FAIL

**Step 3: Implement model store**

```typescript
// frontend/src/model/model-store.ts
import { create } from "zustand";
import type { FeatureKind, ModelFeature } from "./types";
import { CommandStack } from "./command-stack";

interface ModelState {
  features: ModelFeature[];
  dirty: boolean;
  canUndo: boolean;
  canRedo: boolean;

  addFeature: (feature: ModelFeature) => void;
  updateFeature: (feature: ModelFeature) => void;
  removeFeature: (id: string) => void;
  loadFeatures: (features: ModelFeature[]) => void;
  reset: () => void;
  markClean: () => void;

  undo: () => void;
  redo: () => void;

  getFeatureById: (id: string) => ModelFeature | undefined;
  featuresByKind: (kind: FeatureKind) => ModelFeature[];
}

const commandStack = new CommandStack();

export const useModelStore = create<ModelState>((set, get) => {
  commandStack.subscribe(() => {
    set({
      canUndo: commandStack.canUndo(),
      canRedo: commandStack.canRedo(),
    });
  });

  return {
    features: [],
    dirty: false,
    canUndo: false,
    canRedo: false,

    addFeature: (feature) => {
      commandStack.execute({
        description: `Add ${feature.kind} ${feature.id}`,
        execute: () => {
          set((s) => ({ features: [...s.features, feature], dirty: true }));
        },
        undo: () => {
          set((s) => ({
            features: s.features.filter((f) => f.id !== feature.id),
            dirty: true,
          }));
        },
      });
    },

    updateFeature: (feature) => {
      const previous = get().features.find((f) => f.id === feature.id);
      if (!previous) return;
      commandStack.execute({
        description: `Update ${feature.kind} ${feature.id}`,
        execute: () => {
          set((s) => ({
            features: s.features.map((f) =>
              f.id === feature.id ? feature : f,
            ),
            dirty: true,
          }));
        },
        undo: () => {
          set((s) => ({
            features: s.features.map((f) =>
              f.id === feature.id ? previous : f,
            ),
            dirty: true,
          }));
        },
      });
    },

    removeFeature: (id) => {
      const feature = get().features.find((f) => f.id === id);
      if (!feature) return;
      const index = get().features.indexOf(feature);
      commandStack.execute({
        description: `Remove ${feature.kind} ${feature.id}`,
        execute: () => {
          set((s) => ({
            features: s.features.filter((f) => f.id !== id),
            dirty: true,
          }));
        },
        undo: () => {
          set((s) => {
            const next = [...s.features];
            next.splice(index, 0, feature);
            return { features: next, dirty: true };
          });
        },
      });
    },

    loadFeatures: (features) => {
      commandStack.clear();
      set({ features, dirty: false, canUndo: false, canRedo: false });
    },

    reset: () => {
      commandStack.clear();
      set({ features: [], dirty: false, canUndo: false, canRedo: false });
    },

    markClean: () => {
      set({ dirty: false });
    },

    undo: () => {
      commandStack.undo();
    },

    redo: () => {
      commandStack.redo();
    },

    getFeatureById: (id) => {
      return get().features.find((f) => f.id === id);
    },

    featuresByKind: (kind) => {
      return get().features.filter((f) => f.kind === kind);
    },
  };
});
```

**Step 4: Run tests**

Run: `cd frontend && bun run test -- src/model/model-store.test.ts`
Expected: PASS

**Step 5: Commit**

```
feat(model): add model store with undo/redo support
```

---

## Task 4: GeoJSON Normalizer

Convert raw GeoJSON FeatureCollection input into normalized `ModelFeature[]`. This mirrors the Go backend's normalize step and is used by the import assistant.

**Files:**

- Create: `frontend/src/model/normalize.ts`
- Test: `frontend/src/model/normalize.test.ts`

**Step 1: Write failing tests**

```typescript
// frontend/src/model/normalize.test.ts
import { describe, expect, it } from "vitest";
import { normalizeGeoJSON } from "./normalize";
import type { GeoJSONFeatureCollection } from "./types";

const validCollection: GeoJSONFeatureCollection = {
  type: "FeatureCollection",
  features: [
    {
      type: "Feature",
      properties: { kind: "source", source_type: "point" },
      geometry: { type: "Point", coordinates: [10, 51] },
    },
    {
      type: "Feature",
      properties: { kind: "building", height_m: 12 },
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [0, 0],
            [1, 0],
            [1, 1],
            [0, 1],
            [0, 0],
          ],
        ],
      },
    },
    {
      type: "Feature",
      properties: { kind: "barrier", height_m: 3.5 },
      geometry: {
        type: "LineString",
        coordinates: [
          [0, 0],
          [1, 1],
        ],
      },
    },
  ],
};

describe("normalizeGeoJSON", () => {
  it("normalizes a valid FeatureCollection", () => {
    const result = normalizeGeoJSON(validCollection);
    expect(result.features).toHaveLength(3);
    expect(result.features[0]?.kind).toBe("source");
    expect(result.features[0]?.sourceType).toBe("point");
    expect(result.features[1]?.kind).toBe("building");
    expect(result.features[1]?.heightM).toBe(12);
    expect(result.features[2]?.kind).toBe("barrier");
  });

  it("assigns unique IDs to features without IDs", () => {
    const result = normalizeGeoJSON(validCollection);
    const ids = result.features.map((f) => f.id);
    expect(new Set(ids).size).toBe(3);
  });

  it("preserves existing feature IDs", () => {
    const collection: GeoJSONFeatureCollection = {
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          id: "my-id",
          properties: { kind: "source", source_type: "point" },
          geometry: { type: "Point", coordinates: [10, 51] },
        },
      ],
    };
    const result = normalizeGeoJSON(collection);
    expect(result.features[0]?.id).toBe("my-id");
  });

  it("returns empty array for empty collection", () => {
    const result = normalizeGeoJSON({
      type: "FeatureCollection",
      features: [],
    });
    expect(result.features).toEqual([]);
  });

  it("skips features with unknown kind", () => {
    const collection: GeoJSONFeatureCollection = {
      type: "FeatureCollection",
      features: [
        {
          type: "Feature",
          properties: { kind: "unknown" },
          geometry: { type: "Point", coordinates: [10, 51] },
        },
      ],
    };
    const result = normalizeGeoJSON(collection);
    expect(result.features).toEqual([]);
    expect(result.skipped).toHaveLength(1);
  });
});
```

**Step 2: Run tests to verify they fail**

Run: `cd frontend && bun run test -- src/model/normalize.test.ts`
Expected: FAIL

**Step 3: Implement normalizer**

```typescript
// frontend/src/model/normalize.ts
import type {
  FeatureKind,
  GeoJSONFeature,
  GeoJSONFeatureCollection,
  Geometry,
  GeometryType,
  ModelFeature,
  SourceType,
} from "./types";
import { createFeatureId } from "./types";

const VALID_KINDS = new Set<string>(["source", "building", "barrier"]);
const VALID_SOURCE_TYPES = new Set<string>(["point", "line", "area"]);
const VALID_GEOM_TYPES = new Set<string>([
  "Point",
  "MultiPoint",
  "LineString",
  "MultiLineString",
  "Polygon",
  "MultiPolygon",
]);

interface SkippedFeature {
  index: number;
  reason: string;
}

interface NormalizeResult {
  features: ModelFeature[];
  skipped: SkippedFeature[];
}

export function normalizeGeoJSON(
  collection: GeoJSONFeatureCollection,
): NormalizeResult {
  const features: ModelFeature[] = [];
  const skipped: SkippedFeature[] = [];

  for (let i = 0; i < collection.features.length; i++) {
    const raw = collection.features[i]!;
    const result = normalizeFeature(raw, i);
    if (result.ok) {
      features.push(result.feature);
    } else {
      skipped.push({ index: i, reason: result.reason });
    }
  }

  return { features, skipped };
}

type NormalizeFeatureResult =
  | { ok: true; feature: ModelFeature }
  | { ok: false; reason: string };

function normalizeFeature(
  raw: GeoJSONFeature,
  index: number,
): NormalizeFeatureResult {
  const props = raw.properties;
  const kindRaw = String(props.kind ?? "")
    .toLowerCase()
    .trim();

  if (!VALID_KINDS.has(kindRaw)) {
    return {
      ok: false,
      reason: `feature[${String(index)}]: unknown kind "${kindRaw}"`,
    };
  }

  const kind = kindRaw as FeatureKind;
  const geomType = raw.geometry.type;

  if (!VALID_GEOM_TYPES.has(geomType)) {
    return {
      ok: false,
      reason: `feature[${String(index)}]: unsupported geometry type "${geomType}"`,
    };
  }

  const id = raw.id != null ? String(raw.id) : createFeatureId();

  const feature: ModelFeature = {
    id,
    kind,
    geometry: {
      type: geomType as GeometryType,
      coordinates: raw.geometry.coordinates as Geometry["coordinates"],
    },
  };

  if (kind === "source") {
    const st = String(props.source_type ?? "")
      .toLowerCase()
      .trim();
    if (VALID_SOURCE_TYPES.has(st)) {
      feature.sourceType = st as SourceType;
    }
  }

  if (kind === "building" || kind === "barrier") {
    const h = Number(props.height_m);
    if (Number.isFinite(h) && h > 0) {
      feature.heightM = h;
    }
  }

  return { ok: true, feature };
}
```

**Step 4: Run tests**

Run: `cd frontend && bun run test -- src/model/normalize.test.ts`
Expected: PASS

**Step 5: Commit**

```
feat(model): add GeoJSON normalizer for import pipeline
```

---

## Task 5: Client-Side Validation

Mirror the Go backend's validation rules in TypeScript for instant feedback. This runs purely in the browser.

**Files:**

- Create: `frontend/src/model/validate.ts`
- Test: `frontend/src/model/validate.test.ts`

**Step 1: Write failing tests**

```typescript
// frontend/src/model/validate.test.ts
import { describe, expect, it } from "vitest";
import { validateModel } from "./validate";
import type { ModelFeature } from "./types";

const validSource: ModelFeature = {
  id: "src-1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};

const validBuilding: ModelFeature = {
  id: "bld-1",
  kind: "building",
  heightM: 12,
  geometry: {
    type: "Polygon",
    coordinates: [
      [
        [0, 0],
        [1, 0],
        [1, 1],
        [0, 1],
        [0, 0],
      ],
    ],
  },
};

const validBarrier: ModelFeature = {
  id: "bar-1",
  kind: "barrier",
  heightM: 3,
  geometry: {
    type: "LineString",
    coordinates: [
      [0, 0],
      [1, 1],
    ],
  },
};

describe("validateModel", () => {
  it("valid model returns valid=true with no errors", () => {
    const report = validateModel([validSource, validBuilding, validBarrier]);
    expect(report.valid).toBe(true);
    expect(report.errors).toHaveLength(0);
  });

  it("empty model produces an error", () => {
    const report = validateModel([]);
    expect(report.valid).toBe(false);
    expect(report.errors[0]?.code).toBe("model.empty");
  });

  it("source without source_type produces error", () => {
    const bad: ModelFeature = {
      id: "s1",
      kind: "source",
      geometry: { type: "Point", coordinates: [0, 0] },
    };
    const report = validateModel([bad]);
    expect(report.errors.some((e) => e.code === "source.type.required")).toBe(
      true,
    );
  });

  it("source with wrong geometry produces error", () => {
    const bad: ModelFeature = {
      id: "s1",
      kind: "source",
      sourceType: "point",
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [0, 0],
            [1, 0],
            [1, 1],
            [0, 1],
            [0, 0],
          ],
        ],
      },
    };
    const report = validateModel([bad]);
    expect(
      report.errors.some((e) => e.code === "source.geometry.mismatch"),
    ).toBe(true);
  });

  it("building without height produces error", () => {
    const bad: ModelFeature = {
      id: "b1",
      kind: "building",
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [0, 0],
            [1, 0],
            [1, 1],
            [0, 1],
            [0, 0],
          ],
        ],
      },
    };
    const report = validateModel([bad]);
    expect(
      report.errors.some((e) => e.code === "building.height.required"),
    ).toBe(true);
  });

  it("building with negative height produces error", () => {
    const bad: ModelFeature = {
      id: "b1",
      kind: "building",
      heightM: -5,
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [0, 0],
            [1, 0],
            [1, 1],
            [0, 1],
            [0, 0],
          ],
        ],
      },
    };
    const report = validateModel([bad]);
    expect(
      report.errors.some((e) => e.code === "building.height.invalid"),
    ).toBe(true);
  });

  it("barrier without height produces error", () => {
    const bad: ModelFeature = {
      id: "br1",
      kind: "barrier",
      geometry: {
        type: "LineString",
        coordinates: [
          [0, 0],
          [1, 1],
        ],
      },
    };
    const report = validateModel([bad]);
    expect(
      report.errors.some((e) => e.code === "barrier.height.required"),
    ).toBe(true);
  });

  it("building with non-polygon geometry produces error", () => {
    const bad: ModelFeature = {
      id: "b1",
      kind: "building",
      heightM: 10,
      geometry: { type: "Point", coordinates: [0, 0] },
    };
    const report = validateModel([bad]);
    expect(
      report.errors.some((e) => e.code === "building.geometry.invalid"),
    ).toBe(true);
  });

  it("duplicate IDs produce error", () => {
    const a = { ...validSource };
    const b = { ...validBuilding, id: "src-1" };
    const report = validateModel([a, b]);
    expect(report.errors.some((e) => e.code === "feature.id.duplicate")).toBe(
      true,
    );
  });
});
```

**Step 2: Run tests to verify they fail**

Run: `cd frontend && bun run test -- src/model/validate.test.ts`
Expected: FAIL

**Step 3: Implement validator**

```typescript
// frontend/src/model/validate.ts
import type { ModelFeature, ValidationIssue, ValidationReport } from "./types";
import { isGeometryCompatible } from "./types";

export function validateModel(features: ModelFeature[]): ValidationReport {
  const errors: ValidationIssue[] = [];
  const warnings: ValidationIssue[] = [];

  if (features.length === 0) {
    errors.push({
      level: "error",
      code: "model.empty",
      featureId: "",
      message: "Model contains no features",
    });
    return {
      valid: false,
      errors,
      warnings,
      checkedAt: new Date().toISOString(),
    };
  }

  const ids = new Set<string>();

  for (const feature of features) {
    if (ids.has(feature.id)) {
      errors.push({
        level: "error",
        code: "feature.id.duplicate",
        featureId: feature.id,
        message: "Duplicate feature ID",
      });
    }
    ids.add(feature.id);

    validateFeature(feature, errors, warnings);
  }

  return {
    valid: errors.length === 0,
    errors,
    warnings,
    checkedAt: new Date().toISOString(),
  };
}

function validateFeature(
  feature: ModelFeature,
  errors: ValidationIssue[],
  _warnings: ValidationIssue[],
): void {
  const { id, kind } = feature;

  switch (kind) {
    case "source": {
      if (!feature.sourceType) {
        errors.push({
          level: "error",
          code: "source.type.required",
          featureId: id,
          message: "Source requires source_type (point|line|area)",
        });
      } else if (
        !isGeometryCompatible(feature.geometry.type, feature.sourceType)
      ) {
        errors.push({
          level: "error",
          code: "source.geometry.mismatch",
          featureId: id,
          message: `Geometry ${feature.geometry.type} incompatible with source_type ${feature.sourceType}`,
        });
      }
      break;
    }
    case "building": {
      if (feature.heightM == null) {
        errors.push({
          level: "error",
          code: "building.height.required",
          featureId: id,
          message: "Building requires height_m",
        });
      } else if (feature.heightM <= 0) {
        errors.push({
          level: "error",
          code: "building.height.invalid",
          featureId: id,
          message: "Building height_m must be > 0",
        });
      }
      if (
        feature.geometry.type !== "Polygon" &&
        feature.geometry.type !== "MultiPolygon"
      ) {
        errors.push({
          level: "error",
          code: "building.geometry.invalid",
          featureId: id,
          message: "Building geometry must be Polygon or MultiPolygon",
        });
      }
      break;
    }
    case "barrier": {
      if (feature.heightM == null) {
        errors.push({
          level: "error",
          code: "barrier.height.required",
          featureId: id,
          message: "Barrier requires height_m",
        });
      } else if (feature.heightM <= 0) {
        errors.push({
          level: "error",
          code: "barrier.height.invalid",
          featureId: id,
          message: "Barrier height_m must be > 0",
        });
      }
      if (
        feature.geometry.type !== "LineString" &&
        feature.geometry.type !== "MultiLineString"
      ) {
        errors.push({
          level: "error",
          code: "barrier.geometry.invalid",
          featureId: id,
          message: "Barrier geometry must be LineString or MultiLineString",
        });
      }
      break;
    }
  }
}
```

**Step 4: Run tests**

Run: `cd frontend && bun run test -- src/model/validate.test.ts`
Expected: PASS

**Step 5: Commit**

```
feat(model): add client-side model validation mirroring Go backend
```

---

## Task 6: Model-to-GeoJSON Converter

Convert `ModelFeature[]` to GeoJSON FeatureCollection for map display and export. Needed by the map integration and the export/download flow.

**Files:**

- Create: `frontend/src/model/to-geojson.ts`
- Test: `frontend/src/model/to-geojson.test.ts`

**Step 1: Write failing tests**

```typescript
// frontend/src/model/to-geojson.test.ts
import { describe, expect, it } from "vitest";
import { featuresToGeoJSON, featuresToSourceGroups } from "./to-geojson";
import type { ModelFeature } from "./types";

const src: ModelFeature = {
  id: "s1",
  kind: "source",
  sourceType: "point",
  geometry: { type: "Point", coordinates: [10, 51] },
};
const bld: ModelFeature = {
  id: "b1",
  kind: "building",
  heightM: 10,
  geometry: {
    type: "Polygon",
    coordinates: [
      [
        [0, 0],
        [1, 0],
        [1, 1],
        [0, 1],
        [0, 0],
      ],
    ],
  },
};
const bar: ModelFeature = {
  id: "br1",
  kind: "barrier",
  heightM: 3,
  geometry: {
    type: "LineString",
    coordinates: [
      [0, 0],
      [1, 1],
    ],
  },
};

describe("featuresToGeoJSON", () => {
  it("produces a valid FeatureCollection", () => {
    const fc = featuresToGeoJSON([src, bld]);
    expect(fc.type).toBe("FeatureCollection");
    expect(fc.features).toHaveLength(2);
    expect(fc.features[0]?.properties.kind).toBe("source");
    expect(fc.features[0]?.properties.source_type).toBe("point");
    expect(fc.features[1]?.properties.kind).toBe("building");
    expect(fc.features[1]?.properties.height_m).toBe(10);
  });
});

describe("featuresToSourceGroups", () => {
  it("groups features by kind for map sources", () => {
    const groups = featuresToSourceGroups([src, bld, bar]);
    expect(groups.sources.features).toHaveLength(1);
    expect(groups.buildings.features).toHaveLength(1);
    expect(groups.barriers.features).toHaveLength(1);
  });
});
```

**Step 2: Run tests to verify they fail**

Run: `cd frontend && bun run test -- src/model/to-geojson.test.ts`
Expected: FAIL

**Step 3: Implement converter**

```typescript
// frontend/src/model/to-geojson.ts
import type { GeoJSONFeatureCollection, ModelFeature } from "./types";

export function featuresToGeoJSON(
  features: ModelFeature[],
): GeoJSONFeatureCollection {
  return {
    type: "FeatureCollection",
    features: features.map((f) => ({
      type: "Feature" as const,
      id: f.id,
      properties: {
        kind: f.kind,
        ...(f.sourceType != null ? { source_type: f.sourceType } : {}),
        ...(f.heightM != null ? { height_m: f.heightM } : {}),
      },
      geometry: {
        type: f.geometry.type,
        coordinates: f.geometry.coordinates as unknown,
      },
    })),
  };
}

interface SourceGroups {
  sources: GeoJSONFeatureCollection;
  buildings: GeoJSONFeatureCollection;
  barriers: GeoJSONFeatureCollection;
}

export function featuresToSourceGroups(features: ModelFeature[]): SourceGroups {
  return {
    sources: featuresToGeoJSON(features.filter((f) => f.kind === "source")),
    buildings: featuresToGeoJSON(features.filter((f) => f.kind === "building")),
    barriers: featuresToGeoJSON(features.filter((f) => f.kind === "barrier")),
  };
}
```

**Step 4: Run tests**

Run: `cd frontend && bun run test -- src/model/to-geojson.test.ts`
Expected: PASS

**Step 5: Commit**

```
feat(model): add model-to-GeoJSON converter for map display and export
```

---

## Task 7: Model Module Barrel Export

Create the public API for the model module.

**Files:**

- Create: `frontend/src/model/index.ts`

**Step 1: Create barrel export**

```typescript
// frontend/src/model/index.ts
export type {
  FeatureKind,
  SourceType,
  GeometryType,
  Position,
  Geometry,
  ModelFeature,
  ValidationIssue,
  ValidationReport,
  GeoJSONFeatureCollection,
  GeoJSONFeature,
} from "./types";
export { createFeatureId, isGeometryCompatible } from "./types";
export { useModelStore } from "./model-store";
export { CommandStack } from "./command-stack";
export type { Command } from "./command-stack";
export { normalizeGeoJSON } from "./normalize";
export { validateModel } from "./validate";
export { featuresToGeoJSON, featuresToSourceGroups } from "./to-geojson";
```

**Step 2: Run all model tests**

Run: `cd frontend && bun run test -- src/model/`
Expected: All PASS

**Step 3: Commit**

```
feat(model): add barrel export for model module
```

---

## Task 8: Drawing Tools Integration

Install `terra-draw` and create a React hook that manages drawing modes on the map.

**Files:**

- Create: `frontend/src/map/use-draw.ts`
- Modify: `frontend/src/map/index.ts` (add export)

**Step 1: Install terra-draw**

Run: `cd frontend && bun add terra-draw`

**Step 2: Implement drawing hook**

```typescript
// frontend/src/map/use-draw.ts
import { useCallback, useEffect, useRef, useState } from "react";
import {
  TerraDraw,
  TerraDrawMapLibreGLAdapter,
  TerraDrawPointMode,
  TerraDrawLineStringMode,
  TerraDrawPolygonMode,
  TerraDrawSelectMode,
  TerraDrawRenderMode,
} from "terra-draw";
import type { Map } from "maplibre-gl";
import { useMap } from "./use-map";

export type DrawMode = "point" | "linestring" | "polygon" | "select" | "static";

interface UseDrawOptions {
  onFinish?: (mode: DrawMode, feature: GeoJSON.Feature) => void;
}

interface UseDrawReturn {
  activeMode: DrawMode;
  setMode: (mode: DrawMode) => void;
  cancel: () => void;
}

export function useDraw(options: UseDrawOptions = {}): UseDrawReturn {
  const map = useMap();
  const drawRef = useRef<TerraDraw | null>(null);
  const [activeMode, setActiveMode] = useState<DrawMode>("static");
  const onFinishRef = useRef(options.onFinish);
  onFinishRef.current = options.onFinish;

  useEffect(() => {
    if (!map) return;

    const draw = new TerraDraw({
      adapter: new TerraDrawMapLibreGLAdapter({
        map: map as unknown as Parameters<
          typeof TerraDrawMapLibreGLAdapter
        >[0]["map"],
      }),
      modes: [
        new TerraDrawPointMode(),
        new TerraDrawLineStringMode(),
        new TerraDrawPolygonMode(),
        new TerraDrawSelectMode({
          flags: {
            point: { feature: { draggable: true } },
            linestring: {
              feature: {
                draggable: true,
                coordinates: {
                  midpoints: true,
                  draggable: true,
                  deletable: true,
                },
              },
            },
            polygon: {
              feature: {
                draggable: true,
                coordinates: {
                  midpoints: true,
                  draggable: true,
                  deletable: true,
                },
              },
            },
          },
        }),
        new TerraDrawRenderMode({ modeName: "static" }),
      ],
    });

    draw.start();
    draw.setMode("static");

    draw.on("finish", (id: string) => {
      const snapshot = draw.getSnapshot();
      const feature = snapshot.find((f) => f.id === id);
      if (feature && onFinishRef.current) {
        const mode = activeMode;
        // Remove from terra-draw after capturing
        setTimeout(() => {
          try {
            draw.removeFeatures([id]);
          } catch {
            /* may already be removed */
          }
        }, 0);
        onFinishRef.current(mode, feature as GeoJSON.Feature);
      }
    });

    drawRef.current = draw;

    return () => {
      draw.stop();
      drawRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [map]);

  const setMode = useCallback((mode: DrawMode) => {
    drawRef.current?.setMode(mode);
    setActiveMode(mode);
  }, []);

  const cancel = useCallback(() => {
    drawRef.current?.setMode("static");
    setActiveMode("static");
  }, []);

  return { activeMode, setMode, cancel };
}
```

Note: The terra-draw adapter type may need adjustment during implementation — the exact import path and constructor signature should be verified against the installed version. The `onFinish` callback captures the drawn geometry and the implementer should verify terra-draw's event API.

**Step 3: Add export to map barrel**

Add to `frontend/src/map/index.ts`:

```typescript
export { useDraw } from "./use-draw";
export type { DrawMode } from "./use-draw";
```

**Step 4: Run typecheck**

Run: `cd frontend && bun run typecheck`
Expected: PASS (or minor type adjustments needed for terra-draw generics)

**Step 5: Commit**

```
feat(map): add terra-draw integration for geometry drawing
```

---

## Task 9: Drawing Toolbar

A toolbar component that switches drawing modes. Rendered on the map page.

**Files:**

- Create: `frontend/src/map/draw-toolbar.tsx`

**Step 1: Implement toolbar**

```typescript
// frontend/src/map/draw-toolbar.tsx
import { MousePointer, Circle, Minus, Pentagon, X } from "lucide-react";
import { Button } from "@/ui/components/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/ui/components/tooltip";
import type { DrawMode } from "./use-draw";
import { cn } from "@/ui/lib/utils";

interface DrawToolbarProps {
  activeMode: DrawMode;
  onModeChange: (mode: DrawMode) => void;
  onCancel: () => void;
}

const tools: { mode: DrawMode; icon: typeof Circle; label: string }[] = [
  { mode: "select", icon: MousePointer, label: "Select / Edit" },
  { mode: "point", icon: Circle, label: "Draw Point" },
  { mode: "linestring", icon: Minus, label: "Draw Line" },
  { mode: "polygon", icon: Pentagon, label: "Draw Polygon" },
];

export function DrawToolbar({ activeMode, onModeChange, onCancel }: DrawToolbarProps) {
  const isDrawing = activeMode !== "static";

  return (
    <div className="absolute left-3 top-3 z-10 flex flex-col gap-1 rounded-md border bg-background p-1 shadow-md">
      {tools.map(({ mode, icon: Icon, label }) => (
        <Tooltip key={mode}>
          <TooltipTrigger asChild>
            <Button
              variant={activeMode === mode ? "default" : "ghost"}
              size="icon"
              className={cn("h-8 w-8")}
              onClick={() => { onModeChange(mode); }}
              aria-label={label}
            >
              <Icon className="h-4 w-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent side="right">{label}</TooltipContent>
        </Tooltip>
      ))}
      {isDrawing ? (
        <>
          <div className="my-1 border-t" />
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 text-destructive"
                onClick={onCancel}
                aria-label="Cancel drawing"
              >
                <X className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="right">Cancel</TooltipContent>
          </Tooltip>
        </>
      ) : null}
    </div>
  );
}
```

**Step 2: Add export to map barrel**

Add to `frontend/src/map/index.ts`:

```typescript
export { DrawToolbar } from "./draw-toolbar";
```

**Step 3: Commit**

```
feat(map): add drawing toolbar component
```

---

## Task 10: Feature Editor Panel

A side panel that shows/edits attributes for the selected feature. Different forms per kind.

**Files:**

- Create: `frontend/src/map/feature-editor.tsx`

**Step 1: Install shadcn select component (needed for dropdowns)**

Run: `cd frontend && bunx --bun shadcn@latest add select`

**Step 2: Implement feature editor**

```typescript
// frontend/src/map/feature-editor.tsx
import { useCallback, useEffect, useState } from "react";
import { Button } from "@/ui/components/button";
import { Input } from "@/ui/components/input";
import { Label } from "@/ui/components/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/ui/components/select";
import { useModelStore } from "@/model/model-store";
import type { FeatureKind, ModelFeature, SourceType } from "@/model/types";
import { Trash2 } from "lucide-react";

interface FeatureEditorProps {
  featureId: string | null;
  onClose: () => void;
}

export function FeatureEditor({ featureId, onClose }: FeatureEditorProps) {
  const feature = useModelStore((s) =>
    featureId ? s.getFeatureById(featureId) : undefined,
  );

  if (!feature) return null;

  return (
    <div className="absolute right-3 top-3 z-10 w-72 rounded-md border bg-background p-4 shadow-md">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-semibold capitalize">{feature.kind}</h3>
        <Button variant="ghost" size="sm" onClick={onClose} aria-label="Close editor">
          &times;
        </Button>
      </div>
      <div className="space-y-3">
        <div>
          <Label className="text-xs text-muted-foreground">ID</Label>
          <p className="font-mono text-xs">{feature.id}</p>
        </div>
        <div>
          <Label className="text-xs text-muted-foreground">Geometry</Label>
          <p className="text-xs">{feature.geometry.type}</p>
        </div>
        <FeatureFields feature={feature} />
        <DeleteButton featureId={feature.id} onDelete={onClose} />
      </div>
    </div>
  );
}

function FeatureFields({ feature }: { feature: ModelFeature }) {
  switch (feature.kind) {
    case "source":
      return <SourceFields feature={feature} />;
    case "building":
      return <HeightField feature={feature} />;
    case "barrier":
      return <HeightField feature={feature} />;
  }
}

function SourceFields({ feature }: { feature: ModelFeature }) {
  const updateFeature = useModelStore((s) => s.updateFeature);

  const handleTypeChange = useCallback(
    (value: string) => {
      updateFeature({ ...feature, sourceType: value as SourceType });
    },
    [feature, updateFeature],
  );

  return (
    <div className="grid gap-1.5">
      <Label htmlFor="source-type" className="text-xs">Source Type</Label>
      <Select value={feature.sourceType ?? ""} onValueChange={handleTypeChange}>
        <SelectTrigger id="source-type" className="h-8 text-xs">
          <SelectValue placeholder="Select type" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="point">Point</SelectItem>
          <SelectItem value="line">Line</SelectItem>
          <SelectItem value="area">Area</SelectItem>
        </SelectContent>
      </Select>
    </div>
  );
}

function HeightField({ feature }: { feature: ModelFeature }) {
  const updateFeature = useModelStore((s) => s.updateFeature);
  const [value, setValue] = useState(String(feature.heightM ?? ""));

  useEffect(() => {
    setValue(String(feature.heightM ?? ""));
  }, [feature.heightM]);

  const handleBlur = useCallback(() => {
    const num = parseFloat(value);
    if (Number.isFinite(num) && num > 0) {
      updateFeature({ ...feature, heightM: num });
    }
  }, [feature, value, updateFeature]);

  return (
    <div className="grid gap-1.5">
      <Label htmlFor="height" className="text-xs">Height (m)</Label>
      <Input
        id="height"
        type="number"
        step="0.1"
        min="0.1"
        className="h-8 text-xs"
        value={value}
        onChange={(e) => { setValue(e.target.value); }}
        onBlur={handleBlur}
      />
    </div>
  );
}

function DeleteButton({ featureId, onDelete }: { featureId: string; onDelete: () => void }) {
  const removeFeature = useModelStore((s) => s.removeFeature);

  const handleDelete = useCallback(() => {
    removeFeature(featureId);
    onDelete();
  }, [featureId, removeFeature, onDelete]);

  return (
    <Button
      variant="destructive"
      size="sm"
      className="mt-2 w-full"
      onClick={handleDelete}
    >
      <Trash2 className="mr-1.5 h-3.5 w-3.5" />
      Delete Feature
    </Button>
  );
}
```

**Step 3: Commit**

```
feat(map): add feature attribute editor panel
```

---

## Task 11: New Feature Dialog

When drawing finishes, prompt the user for kind + attributes before adding to the model store.

**Files:**

- Create: `frontend/src/map/new-feature-dialog.tsx`

**Step 1: Implement the dialog**

```typescript
// frontend/src/map/new-feature-dialog.tsx
import { useCallback, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/ui/components/dialog";
import { Button } from "@/ui/components/button";
import { Input } from "@/ui/components/input";
import { Label } from "@/ui/components/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/ui/components/select";
import type { FeatureKind, Geometry, SourceType } from "@/model/types";
import { createFeatureId } from "@/model/types";
import { useModelStore } from "@/model/model-store";

interface NewFeatureDialogProps {
  open: boolean;
  geometry: Geometry | null;
  onClose: () => void;
}

function inferKind(geomType: string): FeatureKind {
  if (geomType === "Point" || geomType === "MultiPoint") return "source";
  if (geomType === "LineString" || geomType === "MultiLineString") return "barrier";
  return "building";
}

function inferSourceType(geomType: string): SourceType {
  if (geomType === "Point" || geomType === "MultiPoint") return "point";
  if (geomType === "LineString" || geomType === "MultiLineString") return "line";
  return "area";
}

export function NewFeatureDialog({ open, geometry, onClose }: NewFeatureDialogProps) {
  const addFeature = useModelStore((s) => s.addFeature);
  const defaultKind = geometry ? inferKind(geometry.type) : "source";

  const [kind, setKind] = useState<FeatureKind>(defaultKind);
  const [sourceType, setSourceType] = useState<SourceType>(
    geometry ? inferSourceType(geometry.type) : "point",
  );
  const [height, setHeight] = useState("5");

  const handleSave = useCallback(() => {
    if (!geometry) return;
    const feature = {
      id: createFeatureId(),
      kind,
      geometry,
      ...(kind === "source" ? { sourceType } : {}),
      ...(kind === "building" || kind === "barrier"
        ? { heightM: Math.max(0.1, parseFloat(height) || 5) }
        : {}),
    };
    addFeature(feature);
    onClose();
  }, [geometry, kind, sourceType, height, addFeature, onClose]);

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>New Feature</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 py-2">
          <div className="grid gap-1.5">
            <Label className="text-xs">Kind</Label>
            <Select value={kind} onValueChange={(v) => { setKind(v as FeatureKind); }}>
              <SelectTrigger className="h-8 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="source">Source</SelectItem>
                <SelectItem value="building">Building</SelectItem>
                <SelectItem value="barrier">Barrier</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {kind === "source" ? (
            <div className="grid gap-1.5">
              <Label className="text-xs">Source Type</Label>
              <Select value={sourceType} onValueChange={(v) => { setSourceType(v as SourceType); }}>
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="point">Point</SelectItem>
                  <SelectItem value="line">Line</SelectItem>
                  <SelectItem value="area">Area</SelectItem>
                </SelectContent>
              </Select>
            </div>
          ) : null}

          {kind === "building" || kind === "barrier" ? (
            <div className="grid gap-1.5">
              <Label className="text-xs">Height (m)</Label>
              <Input
                type="number"
                step="0.1"
                min="0.1"
                className="h-8 text-xs"
                value={height}
                onChange={(e) => { setHeight(e.target.value); }}
              />
            </div>
          ) : null}
        </div>
        <DialogFooter>
          <Button variant="ghost" size="sm" onClick={onClose}>Cancel</Button>
          <Button size="sm" onClick={handleSave}>Add Feature</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
```

**Step 2: Commit**

```
feat(map): add new feature dialog for drawn geometries
```

---

## Task 12: Validation Panel

A collapsible panel showing validation issues, with click-to-select for each issue.

**Files:**

- Create: `frontend/src/map/validation-panel.tsx`

**Step 1: Implement panel**

```typescript
// frontend/src/map/validation-panel.tsx
import { useMemo } from "react";
import { AlertTriangle, XCircle } from "lucide-react";
import { Button } from "@/ui/components/button";
import { useModelStore } from "@/model/model-store";
import { validateModel } from "@/model/validate";
import type { ValidationIssue } from "@/model/types";

interface ValidationPanelProps {
  onSelectFeature: (featureId: string) => void;
}

export function ValidationPanel({ onSelectFeature }: ValidationPanelProps) {
  const features = useModelStore((s) => s.features);
  const report = useMemo(() => validateModel(features), [features]);

  if (report.valid && report.warnings.length === 0) {
    return (
      <div className="p-3 text-center text-xs text-muted-foreground">
        Model is valid — no issues found.
      </div>
    );
  }

  const allIssues: ValidationIssue[] = [...report.errors, ...report.warnings];

  return (
    <div className="max-h-64 overflow-y-auto">
      <div className="border-b px-3 py-2 text-xs font-medium">
        {report.errors.length > 0
          ? `${String(report.errors.length)} error(s)`
          : ""}
        {report.errors.length > 0 && report.warnings.length > 0 ? ", " : ""}
        {report.warnings.length > 0
          ? `${String(report.warnings.length)} warning(s)`
          : ""}
      </div>
      <ul className="divide-y">
        {allIssues.map((issue, i) => (
          <li key={i} className="flex items-start gap-2 px-3 py-2">
            {issue.level === "error" ? (
              <XCircle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-destructive" />
            ) : (
              <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0 text-yellow-500" />
            )}
            <div className="min-w-0 flex-1">
              <p className="text-xs">{issue.message}</p>
              <p className="font-mono text-[10px] text-muted-foreground">{issue.code}</p>
            </div>
            {issue.featureId ? (
              <Button
                variant="ghost"
                size="sm"
                className="h-6 px-2 text-[10px]"
                onClick={() => { onSelectFeature(issue.featureId); }}
              >
                Go to
              </Button>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  );
}
```

**Step 2: Commit**

```
feat(map): add validation panel with issue list
```

---

## Task 13: Import Assistant

The import page gets a full workflow: upload file, preview features, confirm and load into model store.

**Files:**

- Modify: `frontend/src/pages/import.tsx` (replace placeholder)

**Step 1: Implement import page**

```typescript
// frontend/src/pages/import.tsx
import { useCallback, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { Button } from "@/ui/components/button";
import { useModelStore } from "@/model/model-store";
import { normalizeGeoJSON } from "@/model/normalize";
import { validateModel } from "@/model/validate";
import type { GeoJSONFeatureCollection, ModelFeature, ValidationReport } from "@/model/types";
import { FileInput, CheckCircle2, AlertTriangle, XCircle } from "lucide-react";

type ImportStep = "upload" | "preview" | "done";

export default function ImportPage() {
  const [step, setStep] = useState<ImportStep>("upload");
  const [features, setFeatures] = useState<ModelFeature[]>([]);
  const [skippedCount, setSkippedCount] = useState(0);
  const [report, setReport] = useState<ValidationReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const loadFeatures = useModelStore((s) => s.loadFeatures);
  const navigate = useNavigate();

  const handleFile = useCallback(async (file: File) => {
    setError(null);
    try {
      const text = await file.text();
      const parsed = JSON.parse(text) as GeoJSONFeatureCollection;
      if (parsed.type !== "FeatureCollection" || !Array.isArray(parsed.features)) {
        setError("File must be a GeoJSON FeatureCollection");
        return;
      }
      const result = normalizeGeoJSON(parsed);
      setFeatures(result.features);
      setSkippedCount(result.skipped.length);
      setReport(validateModel(result.features));
      setStep("preview");
    } catch {
      setError("Failed to parse file as JSON");
    }
  }, []);

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      const file = e.dataTransfer.files[0];
      if (file) void handleFile(file);
    },
    [handleFile],
  );

  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) void handleFile(file);
    },
    [handleFile],
  );

  const handleConfirm = useCallback(() => {
    loadFeatures(features);
    setStep("done");
  }, [features, loadFeatures]);

  const handleGoToMap = useCallback(() => {
    void navigate("/map");
  }, [navigate]);

  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <div className="w-full max-w-lg">
        {step === "upload" ? (
          <div
            className="flex flex-col items-center gap-4 rounded-lg border-2 border-dashed p-12 text-center"
            onDrop={handleDrop}
            onDragOver={(e) => { e.preventDefault(); }}
          >
            <FileInput className="h-10 w-10 text-muted-foreground" />
            <div>
              <h2 className="text-lg font-semibold">Import GeoJSON</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                Drop a GeoJSON file here or click to browse
              </p>
            </div>
            <Button onClick={() => { fileRef.current?.click(); }}>
              Choose File
            </Button>
            <input
              ref={fileRef}
              type="file"
              accept=".geojson,.json"
              className="hidden"
              onChange={handleInputChange}
            />
            {error ? (
              <p className="text-sm text-destructive">{error}</p>
            ) : null}
          </div>
        ) : null}

        {step === "preview" && report ? (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">Import Preview</h2>
            <div className="rounded-md border p-4 text-sm">
              <p>{String(features.length)} features normalized</p>
              {skippedCount > 0 ? (
                <p className="text-yellow-600">{String(skippedCount)} features skipped (unknown kind)</p>
              ) : null}
              <div className="mt-2 space-y-1">
                <p>Sources: {String(features.filter((f) => f.kind === "source").length)}</p>
                <p>Buildings: {String(features.filter((f) => f.kind === "building").length)}</p>
                <p>Barriers: {String(features.filter((f) => f.kind === "barrier").length)}</p>
              </div>
            </div>

            {report.errors.length > 0 ? (
              <div className="rounded-md border border-destructive/50 p-3">
                <div className="flex items-center gap-2 text-sm font-medium text-destructive">
                  <XCircle className="h-4 w-4" />
                  {String(report.errors.length)} validation error(s)
                </div>
                <ul className="mt-2 space-y-1 text-xs">
                  {report.errors.slice(0, 5).map((e, i) => (
                    <li key={i}>{e.message}</li>
                  ))}
                  {report.errors.length > 5 ? (
                    <li className="text-muted-foreground">
                      ...and {String(report.errors.length - 5)} more
                    </li>
                  ) : null}
                </ul>
              </div>
            ) : null}

            {report.warnings.length > 0 ? (
              <div className="rounded-md border border-yellow-500/50 p-3">
                <div className="flex items-center gap-2 text-sm font-medium text-yellow-600">
                  <AlertTriangle className="h-4 w-4" />
                  {String(report.warnings.length)} warning(s)
                </div>
              </div>
            ) : null}

            <div className="flex gap-2">
              <Button variant="ghost" onClick={() => { setStep("upload"); }}>
                Back
              </Button>
              <Button onClick={handleConfirm}>
                Import {String(features.length)} Features
              </Button>
            </div>
          </div>
        ) : null}

        {step === "done" ? (
          <div className="flex flex-col items-center gap-4 text-center">
            <CheckCircle2 className="h-10 w-10 text-green-500" />
            <h2 className="text-lg font-semibold">Import Complete</h2>
            <p className="text-sm text-muted-foreground">
              {String(features.length)} features loaded into the model.
            </p>
            <Button onClick={handleGoToMap}>Go to Map</Button>
          </div>
        ) : null}
      </div>
    </div>
  );
}
```

**Step 2: Commit**

```
feat(import): implement import assistant with preview and validation
```

---

## Task 14: Undo/Redo Toolbar

Keyboard shortcuts (Ctrl+Z / Ctrl+Shift+Z) and toolbar buttons for undo/redo.

**Files:**

- Create: `frontend/src/map/undo-redo-bar.tsx`

**Step 1: Implement component**

```typescript
// frontend/src/map/undo-redo-bar.tsx
import { useEffect } from "react";
import { Undo2, Redo2 } from "lucide-react";
import { Button } from "@/ui/components/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/ui/components/tooltip";
import { useModelStore } from "@/model/model-store";

export function UndoRedoBar() {
  const canUndo = useModelStore((s) => s.canUndo);
  const canRedo = useModelStore((s) => s.canRedo);
  const undo = useModelStore((s) => s.undo);
  const redo = useModelStore((s) => s.redo);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "z") {
        e.preventDefault();
        if (e.shiftKey) {
          redo();
        } else {
          undo();
        }
      }
      if ((e.ctrlKey || e.metaKey) && e.key === "y") {
        e.preventDefault();
        redo();
      }
    };
    window.addEventListener("keydown", handler);
    return () => { window.removeEventListener("keydown", handler); };
  }, [undo, redo]);

  return (
    <div className="absolute bottom-3 left-3 z-10 flex gap-1 rounded-md border bg-background p-1 shadow-md">
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            disabled={!canUndo}
            onClick={undo}
            aria-label="Undo"
          >
            <Undo2 className="h-4 w-4" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>Undo (Ctrl+Z)</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8"
            disabled={!canRedo}
            onClick={redo}
            aria-label="Redo"
          >
            <Redo2 className="h-4 w-4" />
          </Button>
        </TooltipTrigger>
        <TooltipContent>Redo (Ctrl+Shift+Z)</TooltipContent>
      </Tooltip>
    </div>
  );
}
```

**Step 2: Commit**

```
feat(map): add undo/redo toolbar with keyboard shortcuts
```

---

## Task 15: Wire Everything into the Map Page

Connect all the new components into the map page: model store drives map sources, drawing tools create features, editor panel edits them, validation panel shows issues.

**Files:**

- Modify: `frontend/src/pages/map.tsx` (replace current implementation)
- Create: `frontend/src/map/model-layers.tsx` (sync model store to map sources)

**Step 1: Create model-layers sync component**

```typescript
// frontend/src/map/model-layers.tsx
import { useEffect } from "react";
import { useMap } from "./use-map";
import { useModelStore } from "@/model/model-store";
import { featuresToSourceGroups } from "@/model/to-geojson";
import {
  SOURCE_IDS,
  BUILDING_LAYERS,
  BARRIER_LAYERS,
  SOURCE_LAYERS,
} from "./layers";

/**
 * Syncs the model store features to MapLibre GeoJSON sources.
 * Must be rendered as a child of MapView (inside MapContext).
 */
export function ModelLayers() {
  const map = useMap();
  const features = useModelStore((s) => s.features);

  useEffect(() => {
    if (!map) return;

    const groups = featuresToSourceGroups(features);

    // Ensure sources exist
    for (const [sourceId, data] of [
      [SOURCE_IDS.buildings, groups.buildings],
      [SOURCE_IDS.barriers, groups.barriers],
      [SOURCE_IDS.sources, groups.sources],
    ] as const) {
      const existing = map.getSource(sourceId);
      if (existing && "setData" in existing) {
        (existing as maplibregl.GeoJSONSource).setData(data as GeoJSON.GeoJSON);
      } else if (!existing) {
        map.addSource(sourceId, {
          type: "geojson",
          data: data as GeoJSON.GeoJSON,
        });
      }
    }

    // Ensure layers exist (idempotent — skip if already added)
    const allLayers = [...BUILDING_LAYERS, ...BARRIER_LAYERS, ...SOURCE_LAYERS];
    for (const layer of allLayers) {
      if (!map.getLayer(layer.id)) {
        map.addLayer(layer);
      }
    }
  }, [map, features]);

  return null;
}
```

**Step 2: Update map page**

Replace `frontend/src/pages/map.tsx`:

```typescript
// frontend/src/pages/map.tsx
import { useCallback, useState } from "react";
import type { MapGeoJSONFeature, MapMouseEvent } from "maplibre-gl";
import { TooltipProvider } from "@/ui/components/tooltip";
import { MapView } from "@/map/map-view";
import { LayerControl } from "@/map/layer-control";
import { CoordinateDisplay } from "@/map/coordinate-display";
import { FeaturePopup } from "@/map/feature-popup";
import { DrawToolbar } from "@/map/draw-toolbar";
import { FeatureEditor } from "@/map/feature-editor";
import { NewFeatureDialog } from "@/map/new-feature-dialog";
import { ValidationPanel } from "@/map/validation-panel";
import { UndoRedoBar } from "@/map/undo-redo-bar";
import { ModelLayers } from "@/map/model-layers";
import { useDraw } from "@/map/use-draw";
import type { Geometry } from "@/model/types";
import type { DrawMode } from "@/map/use-draw";

export default function MapPage() {
  const [clickedFeature, setClickedFeature] =
    useState<MapGeoJSONFeature | null>(null);
  const [popupLngLat, setPopupLngLat] = useState<[number, number] | null>(null);
  const [editingFeatureId, setEditingFeatureId] = useState<string | null>(null);
  const [newGeometry, setNewGeometry] = useState<Geometry | null>(null);
  const [showValidation, setShowValidation] = useState(false);

  const handleDrawFinish = useCallback((_mode: DrawMode, feature: GeoJSON.Feature) => {
    if (feature.geometry) {
      setNewGeometry(feature.geometry as Geometry);
    }
  }, []);

  const { activeMode, setMode, cancel } = useDraw({ onFinish: handleDrawFinish });

  const handleFeatureClick = useCallback(
    (features: MapGeoJSONFeature[], e: MapMouseEvent) => {
      const feature = features[0];
      if (feature) {
        setClickedFeature(feature);
        setPopupLngLat([e.lngLat.lng, e.lngLat.lat]);
        const id = feature.properties?.id ?? feature.id;
        if (id != null) {
          setEditingFeatureId(String(id));
        }
      }
    },
    [],
  );

  const handleSelectFromValidation = useCallback((featureId: string) => {
    setEditingFeatureId(featureId);
    setShowValidation(false);
  }, []);

  return (
    <TooltipProvider>
      <MapView onFeatureClick={handleFeatureClick}>
        <ModelLayers />
        <DrawToolbar
          activeMode={activeMode}
          onModeChange={setMode}
          onCancel={cancel}
        />
        <LayerControl />
        <CoordinateDisplay />
        <FeaturePopup feature={clickedFeature} lngLat={popupLngLat} />
        <FeatureEditor
          featureId={editingFeatureId}
          onClose={() => { setEditingFeatureId(null); }}
        />
        <UndoRedoBar />
        {showValidation ? (
          <div className="absolute bottom-14 left-3 z-10 w-80 rounded-md border bg-background shadow-md">
            <ValidationPanel onSelectFeature={handleSelectFromValidation} />
          </div>
        ) : null}
      </MapView>
      <NewFeatureDialog
        open={newGeometry !== null}
        geometry={newGeometry}
        onClose={() => { setNewGeometry(null); }}
      />
    </TooltipProvider>
  );
}
```

**Step 3: Update map barrel export**

Add to `frontend/src/map/index.ts`:

```typescript
export { ModelLayers } from "./model-layers";
export { DrawToolbar } from "./draw-toolbar";
export { FeatureEditor } from "./feature-editor";
export { NewFeatureDialog } from "./new-feature-dialog";
export { ValidationPanel } from "./validation-panel";
export { UndoRedoBar } from "./undo-redo-bar";
export { useDraw } from "./use-draw";
export type { DrawMode } from "./use-draw";
```

**Step 4: Run typecheck + lint + build**

Run: `cd /mnt/projekte/Code/Soundplan && just fe-ci`
Expected: PASS (may need minor type fixes)

**Step 5: Commit**

```
feat(map): wire model editing, drawing, validation into map page
```

---

## Task 16: Update PLAN.md

Check off Phase 23e items.

**Files:**

- Modify: `PLAN.md`

**Step 1: Update checkboxes**

Change all Phase 23e items from `[ ]` to `[x]`.

**Step 2: Commit**

```
docs: mark Phase 23e complete in PLAN.md
```

---

## Summary of new files

```
frontend/src/model/
  types.ts              — Core TypeScript types (ModelFeature, ValidationIssue, etc.)
  types.test.ts         — Tests for type utilities
  command-stack.ts      — Undo/redo command stack
  command-stack.test.ts — Tests for command stack
  model-store.ts        — Zustand store with CRUD + undo/redo
  model-store.test.ts   — Tests for model store
  normalize.ts          — GeoJSON → ModelFeature[] normalizer
  normalize.test.ts     — Tests for normalizer
  validate.ts           — Client-side validation (mirrors Go backend)
  validate.test.ts      — Tests for validation
  to-geojson.ts         — ModelFeature[] → GeoJSON converter
  to-geojson.test.ts    — Tests for converter
  index.ts              — Barrel export

frontend/src/map/
  use-draw.ts           — terra-draw React hook
  draw-toolbar.tsx      — Drawing mode toolbar
  feature-editor.tsx    — Attribute editor panel
  new-feature-dialog.tsx — Dialog for new features after drawing
  validation-panel.tsx  — Validation issues list
  undo-redo-bar.tsx     — Undo/redo buttons + keyboard shortcuts
  model-layers.tsx      — Syncs model store → MapLibre sources

Modified:
  frontend/src/pages/map.tsx    — Full editing map page
  frontend/src/pages/import.tsx — Import assistant with file upload + preview
  frontend/src/map/index.ts     — Updated barrel exports
```

## Dependencies to add

```
terra-draw          — Map-agnostic drawing toolkit
@radix-ui/react-select — For shadcn Select component (via shadcn CLI)
```
