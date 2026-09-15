import { useCallback, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { Input } from "@/ui/components/input";
import { Label } from "@/ui/components/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/ui/components/tabs";
import { Callout } from "@/ui/callout";
import { KeyValueList } from "@/ui/key-value-list";
import { PageHeader } from "@/ui/page-header";
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

const IMPORT_SOURCES: readonly ImportSource[] = ["file", "osm"];

function isImportSource(value: string): value is ImportSource {
  return (IMPORT_SOURCES as readonly string[]).includes(value);
}

/** How many validation errors the preview lists before it summarises the rest. */
const PREVIEW_ERROR_LIMIT = 5;

function BBoxField({
  id,
  label,
  value,
  placeholder,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  placeholder: string;
  onChange: (value: string) => void;
}) {
  return (
    <div className="flex flex-col gap-1">
      <Input
        id={id}
        type="number"
        step="any"
        value={value}
        onChange={(e) => {
          onChange(e.target.value);
        }}
        placeholder={placeholder}
      />
      <Label htmlFor={id} className="text-center text-xs text-muted-foreground">
        {label}
      </Label>
    </div>
  );
}

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

  // OSM form state. The bounding box is held as text so a half-typed value
  // survives a re-render; `toFixed` here writes an input value, not a display
  // string, so it stays outside the locale-aware formatters.
  const [osmSouth, setOsmSouth] = useState("");
  const [osmWest, setOsmWest] = useState("");
  const [osmNorth, setOsmNorth] = useState("");
  const [osmEast, setOsmEast] = useState("");
  const [osmEndpoint, setOsmEndpoint] = useState("");

  const [geolocating, setGeolocating] = useState(false);

  const osmMutation = useImportFromOSM();

  const handleUseCurrentLocation = useCallback(() => {
    if (typeof navigator.geolocation.getCurrentPosition !== "function") {
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
        ...(osmEndpoint ? { overpass_endpoint: osmEndpoint } : {}),
      },
      {
        onSuccess: (collection) => {
          handleNormalizeAndPreview(collection);
        },
        onError: (err: unknown) => {
          setError(
            err instanceof Error ? err.message : m.error_osm_fetch_failed(),
          );
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
    void navigate("/model");
  }, [navigate]);

  const countByKind = (kind: ModelFeature["kind"]) =>
    String(features.filter((f) => f.kind === kind).length);

  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <div className="w-full max-w-lg">
        {step === "upload" ? (
          <div className="flex flex-col gap-4">
            <Tabs
              value={source}
              onValueChange={(value) => {
                if (!isImportSource(value)) return;
                setSource(value);
                setError(null);
              }}
            >
              <TabsList>
                <TabsTrigger value="file">
                  {m.action_import_from_file()}
                </TabsTrigger>
                <TabsTrigger value="osm">
                  {m.action_import_from_osm()}
                </TabsTrigger>
              </TabsList>

              <TabsContent value="file">
                <div
                  className="flex flex-col items-center gap-4 rounded-lg border-2 border-dashed p-12 text-center"
                  onDrop={handleDrop}
                  onDragOver={(e) => {
                    e.preventDefault();
                  }}
                >
                  <FileInput
                    className="size-10 text-muted-foreground"
                    aria-hidden="true"
                  />
                  <PageHeader
                    className="justify-center text-center"
                    title={m.heading_import_geojson()}
                    description={m.msg_drag_or_click()}
                  />
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
              </TabsContent>

              <TabsContent value="osm">
                <Card className="flex flex-col gap-4 p-6">
                  <PageHeader
                    title={m.heading_import_from_osm()}
                    description={m.msg_import_osm_description()}
                  />
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={handleUseCurrentLocation}
                    disabled={geolocating}
                    className="self-start"
                  >
                    <LocateFixed aria-hidden="true" />
                    {geolocating
                      ? m.status_locating()
                      : m.action_use_current_location()}
                  </Button>
                  <div className="grid grid-cols-4 gap-3">
                    <BBoxField
                      id="osm-south"
                      label={m.label_south()}
                      value={osmSouth}
                      placeholder="52.49"
                      onChange={setOsmSouth}
                    />
                    <BBoxField
                      id="osm-west"
                      label={m.label_west()}
                      value={osmWest}
                      placeholder="13.35"
                      onChange={setOsmWest}
                    />
                    <BBoxField
                      id="osm-north"
                      label={m.label_north()}
                      value={osmNorth}
                      placeholder="52.52"
                      onChange={setOsmNorth}
                    />
                    <BBoxField
                      id="osm-east"
                      label={m.label_east()}
                      value={osmEast}
                      placeholder="13.40"
                      onChange={setOsmEast}
                    />
                  </div>
                  <div className="flex flex-col gap-1">
                    <Label
                      htmlFor="osm-endpoint"
                      className="text-xs text-muted-foreground"
                    >
                      {m.label_overpass_endpoint_optional()}
                    </Label>
                    <Input
                      id="osm-endpoint"
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
                    {osmMutation.isPending
                      ? m.status_fetching()
                      : m.action_fetch_from_osm()}
                  </Button>
                </Card>
              </TabsContent>
            </Tabs>

            {error ? <Callout variant="destructive">{error}</Callout> : null}
          </div>
        ) : null}

        {step === "preview" && report ? (
          <div className="space-y-4">
            <PageHeader title={m.heading_import_preview()} />
            <Card className="space-y-3 p-4 text-sm">
              <p>
                {String(features.length)} {m.msg_features_normalized()}
              </p>
              {skippedCount > 0 ? (
                <p className="text-warning">
                  {String(skippedCount)} {m.msg_features_skipped()}
                </p>
              ) : null}
              <KeyValueList
                items={[
                  { label: m.label_sources(), value: countByKind("source") },
                  {
                    label: m.label_buildings(),
                    value: countByKind("building"),
                  },
                  { label: m.label_barriers(), value: countByKind("barrier") },
                ]}
              />
            </Card>

            {report.errors.length > 0 ? (
              <Callout
                variant="destructive"
                icon={XCircle}
                title={`${String(report.errors.length)} ${m.status_validation_errors()}`}
              >
                <ul className="space-y-1">
                  {report.errors.slice(0, PREVIEW_ERROR_LIMIT).map((e, i) => (
                    <li key={i}>{e.message}</li>
                  ))}
                  {report.errors.length > PREVIEW_ERROR_LIMIT ? (
                    <li className="text-muted-foreground">
                      {m.msg_and_more({
                        count: report.errors.length - PREVIEW_ERROR_LIMIT,
                      })}
                    </li>
                  ) : null}
                </ul>
              </Callout>
            ) : null}

            {report.warnings.length > 0 ? (
              <Callout variant="warning" icon={AlertTriangle}>
                {String(report.warnings.length)}{" "}
                {m.status_validation_warnings()}
              </Callout>
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
            <CheckCircle2 className="size-10 text-success" aria-hidden="true" />
            <PageHeader
              className="justify-center text-center"
              title={m.status_import_complete()}
              description={`${String(features.length)} ${m.msg_import_complete_description()}`}
            />
            <Button onClick={handleGoToMap}>{m.action_go_to_map()}</Button>
          </div>
        ) : null}
      </div>
    </div>
  );
}
