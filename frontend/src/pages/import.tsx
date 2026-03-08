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
import {
  FileInput,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  LocateFixed,
} from "lucide-react";
import { useImportFromOSM } from "@/api/hooks";
import { m } from "@/i18n/messages";

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

  const [geolocating, setGeolocating] = useState(false);

  const osmMutation = useImportFromOSM();

  const handleUseCurrentLocation = useCallback(() => {
    if (!navigator.geolocation) {
      setError(m.error_geolocation_not_supported());
      return;
    }
    setGeolocating(true);
    setError(null);
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        const lat = pos.coords.latitude;
        const lon = pos.coords.longitude;
        const delta = 0.005; // ~500 m radius
        setOsmSouth((lat - delta).toFixed(6));
        setOsmNorth((lat + delta).toFixed(6));
        setOsmWest((lon - delta).toFixed(6));
        setOsmEast((lon + delta).toFixed(6));
        setGeolocating(false);
      },
      (err) => {
        setError(m.error_location_fetch_failed() + `: ${err.message}`);
        setGeolocating(false);
      },
    );
  }, []);

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
          setError(m.msg_geojson_error_invalid());
          return;
        }
        handleNormalizeAndPreview(
          parsed as unknown as GeoJSONFeatureCollection,
        );
      } catch {
        setError(m.msg_geojson_error_parse());
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
      setError(m.msg_bbox_required());
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
                  <h2 className="text-lg font-semibold">{m.heading_import_geojson()}</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {m.msg_drag_or_click()}
                  </p>
                </div>
                <Button
                  onClick={() => {
                    fileRef.current?.click();
                  }}
                >
                  {m.action_choose_file()}
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
                    {m.heading_import_from_osm()}
                  </h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {m.msg_import_osm_description()}
                  </p>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleUseCurrentLocation}
                  disabled={geolocating}
                  className="self-start"
                >
                  <LocateFixed className="mr-2 h-4 w-4" />
                  {geolocating ? m.status_locating() : m.action_use_current_location()}
                </Button>
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
                      {m.label_south()}
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
                      {m.label_west()}
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
                      {m.label_north()}
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
                      {m.label_east()}
                    </Label>
                  </div>
                </div>
                <div className="flex flex-col gap-1">
                  <Label className="text-xs text-muted-foreground">
                    {m.label_overpass_endpoint_optional()}
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
                  {osmMutation.isPending ? m.status_fetching() : m.action_fetch_from_osm()}
                </Button>
              </div>
            )}

            {error ? <p className="text-sm text-destructive">{error}</p> : null}
          </div>
        ) : null}

        {step === "preview" && report ? (
          <div className="space-y-4">
            <h2 className="text-lg font-semibold">{m.heading_import_preview()}</h2>
            <div className="rounded-md border p-4 text-sm">
              <p>{String(features.length)} {m.msg_features_normalized()}</p>
              {skippedCount > 0 ? (
                <p className="text-yellow-600">
                  {String(skippedCount)} {m.msg_features_skipped()}
                </p>
              ) : null}
              <div className="mt-2 space-y-1">
                <p>
                  {m.label_sources()}:{" "}
                  {String(features.filter((f) => f.kind === "source").length)}
                </p>
                <p>
                  {m.label_buildings()}:{" "}
                  {String(features.filter((f) => f.kind === "building").length)}
                </p>
                <p>
                  {m.label_barriers()}:{" "}
                  {String(features.filter((f) => f.kind === "barrier").length)}
                </p>
              </div>
            </div>

            {report.errors.length > 0 ? (
              <div className="rounded-md border border-destructive/50 p-3">
                <div className="flex items-center gap-2 text-sm font-medium text-destructive">
                  <XCircle className="h-4 w-4" />
                  {String(report.errors.length)} {m.status_validation_errors()}
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
                  {String(report.warnings.length)} {m.status_validation_warnings()}
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
                {m.action_back()}
              </Button>
              <Button onClick={handleConfirm}>
                {m.action_import_features({ count: features.length })}
              </Button>
            </div>
          </div>
        ) : null}

        {step === "done" ? (
          <div className="flex flex-col items-center gap-4 text-center">
            <CheckCircle2 className="h-10 w-10 text-green-500" />
            <h2 className="text-lg font-semibold">{m.status_import_complete()}</h2>
            <p className="text-sm text-muted-foreground">
              {String(features.length)} {m.msg_import_complete_description()}
            </p>
            <Button onClick={handleGoToMap}>{m.action_go_to_map()}</Button>
          </div>
        ) : null}
      </div>
    </div>
  );
}
