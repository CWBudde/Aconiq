import { useCallback, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { Button } from "@/ui/components/button";
import { Input } from "@/ui/components/input";
import { Label } from "@/ui/components/label";
import { useModelStore } from "@/model/model-store";
import { normalizeGeoJSON } from "@/model/normalize";
import { validateModel } from "@/model/validate";
import type {
  GeoJSONFeatureCollection,
  ModelFeature,
  ValidationReport,
} from "@/model/types";
import { FileInput, CheckCircle2, AlertTriangle, XCircle } from "lucide-react";
import { useImportFromOSM } from "@/api/hooks";

type ImportStep = "upload" | "preview" | "done";
type ImportSource = "file" | "osm";

export default function ImportPage() {
  const [step, setStep] = useState<ImportStep>("upload");
  const [source, setSource] = useState<ImportSource>("file");
  const [features, setFeatures] = useState<ModelFeature[]>([]);
  const [skippedCount, setSkippedCount] = useState(0);
  const [report, setReport] = useState<ValidationReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const loadFeatures = useModelStore((s) => s.loadFeatures);
  const navigate = useNavigate();

  // OSM form state
  const [osmSouth, setOsmSouth] = useState("");
  const [osmWest, setOsmWest] = useState("");
  const [osmNorth, setOsmNorth] = useState("");
  const [osmEast, setOsmEast] = useState("");
  const [osmEndpoint, setOsmEndpoint] = useState("");

  const osmMutation = useImportFromOSM();

  const handleNormalizeAndPreview = useCallback(
    (collection: GeoJSONFeatureCollection) => {
      const result = normalizeGeoJSON(collection);
      setFeatures(result.features);
      setSkippedCount(result.skipped.length);
      setReport(validateModel(result.features));
      setStep("preview");
    },
    [],
  );

  const handleFile = useCallback(
    async (file: File) => {
      setError(null);
      try {
        const text = await file.text();
        const parsed = JSON.parse(text) as Record<string, unknown>;
        if (
          parsed["type"] !== "FeatureCollection" ||
          !Array.isArray(parsed["features"])
        ) {
          setError("File must be a GeoJSON FeatureCollection");
          return;
        }
        handleNormalizeAndPreview(
          parsed as unknown as GeoJSONFeatureCollection,
        );
      } catch {
        setError("Failed to parse file as JSON");
      }
    },
    [handleNormalizeAndPreview],
  );

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

  const handleOSMFetch = useCallback(() => {
    setError(null);
    const south = parseFloat(osmSouth);
    const west = parseFloat(osmWest);
    const north = parseFloat(osmNorth);
    const east = parseFloat(osmEast);

    if (isNaN(south) || isNaN(west) || isNaN(north) || isNaN(east)) {
      setError("All four bounding box values are required");
      return;
    }

    osmMutation.mutate(
      {
        south,
        west,
        north,
        east,
        overpass_endpoint: osmEndpoint || undefined,
      },
      {
        onSuccess: (collection) => {
          handleNormalizeAndPreview(collection);
        },
        onError: (err: unknown) => {
          setError(err instanceof Error ? err.message : "OSM fetch failed");
        },
      },
    );
  }, [
    osmSouth,
    osmWest,
    osmNorth,
    osmEast,
    osmEndpoint,
    osmMutation,
    handleNormalizeAndPreview,
  ]);

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
          <div className="flex flex-col gap-4">
            {/* Source toggle */}
            <div className="flex gap-2">
              <Button
                variant={source === "file" ? "default" : "ghost"}
                onClick={() => {
                  setSource("file");
                  setError(null);
                }}
              >
                From File
              </Button>
              <Button
                variant={source === "osm" ? "default" : "ghost"}
                onClick={() => {
                  setSource("osm");
                  setError(null);
                }}
              >
                From OpenStreetMap
              </Button>
            </div>

            {source === "file" ? (
              <div
                className="flex flex-col items-center gap-4 rounded-lg border-2 border-dashed p-12 text-center"
                onDrop={handleDrop}
                onDragOver={(e) => {
                  e.preventDefault();
                }}
              >
                <FileInput className="h-10 w-10 text-muted-foreground" />
                <div>
                  <h2 className="text-lg font-semibold">Import GeoJSON</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Drop a GeoJSON file here or click to browse
                  </p>
                </div>
                <Button
                  onClick={() => {
                    fileRef.current?.click();
                  }}
                >
                  Choose File
                </Button>
                <input
                  ref={fileRef}
                  type="file"
                  accept=".geojson,.json"
                  className="hidden"
                  onChange={handleInputChange}
                />
              </div>
            ) : (
              <div className="flex flex-col gap-4 rounded-lg border p-6">
                <div>
                  <h2 className="text-lg font-semibold">
                    Import from OpenStreetMap
                  </h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Enter a bounding box to fetch roads, buildings, and barriers
                    via Overpass API.
                  </p>
                </div>
                <div className="grid grid-cols-4 gap-3">
                  <div className="flex flex-col gap-1">
                    <Input
                      type="number"
                      step="any"
                      value={osmSouth}
                      onChange={(e) => {
                        setOsmSouth(e.target.value);
                      }}
                      placeholder="52.49"
                    />
                    <Label className="text-center text-xs text-muted-foreground">
                      South
                    </Label>
                  </div>
                  <div className="flex flex-col gap-1">
                    <Input
                      type="number"
                      step="any"
                      value={osmWest}
                      onChange={(e) => {
                        setOsmWest(e.target.value);
                      }}
                      placeholder="13.35"
                    />
                    <Label className="text-center text-xs text-muted-foreground">
                      West
                    </Label>
                  </div>
                  <div className="flex flex-col gap-1">
                    <Input
                      type="number"
                      step="any"
                      value={osmNorth}
                      onChange={(e) => {
                        setOsmNorth(e.target.value);
                      }}
                      placeholder="52.52"
                    />
                    <Label className="text-center text-xs text-muted-foreground">
                      North
                    </Label>
                  </div>
                  <div className="flex flex-col gap-1">
                    <Input
                      type="number"
                      step="any"
                      value={osmEast}
                      onChange={(e) => {
                        setOsmEast(e.target.value);
                      }}
                      placeholder="13.40"
                    />
                    <Label className="text-center text-xs text-muted-foreground">
                      East
                    </Label>
                  </div>
                </div>
                <div className="flex flex-col gap-1">
                  <Label className="text-xs text-muted-foreground">
                    Overpass Endpoint (optional)
                  </Label>
                  <Input
                    type="text"
                    value={osmEndpoint}
                    onChange={(e) => {
                      setOsmEndpoint(e.target.value);
                    }}
                    placeholder="https://overpass-api.de/api/interpreter"
                  />
                </div>
                <Button
                  onClick={handleOSMFetch}
                  disabled={osmMutation.isPending}
                >
                  {osmMutation.isPending ? "Fetching…" : "Fetch from OSM"}
                </Button>
              </div>
            )}

            {error ? <p className="text-sm text-destructive">{error}</p> : null}
          </div>
        ) : null}

        {step === "preview" && report ? (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">Import Preview</h2>
            <div className="rounded-md border p-4 text-sm">
              <p>{String(features.length)} features normalized</p>
              {skippedCount > 0 ? (
                <p className="text-yellow-600">
                  {String(skippedCount)} features skipped (unknown kind)
                </p>
              ) : null}
              <div className="mt-2 space-y-1">
                <p>
                  Sources:{" "}
                  {String(features.filter((f) => f.kind === "source").length)}
                </p>
                <p>
                  Buildings:{" "}
                  {String(features.filter((f) => f.kind === "building").length)}
                </p>
                <p>
                  Barriers:{" "}
                  {String(features.filter((f) => f.kind === "barrier").length)}
                </p>
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
              <Button
                variant="ghost"
                onClick={() => {
                  setStep("upload");
                }}
              >
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
