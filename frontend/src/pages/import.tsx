import { useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { Button } from "@/ui/components/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/ui/components/tabs";
import { Callout } from "@/ui/callout";
import { ConfirmDialog } from "@/ui/confirm-dialog";
import { PageHeader } from "@/ui/page-header";
import { FileImport } from "@/import/file-import";
import { OsmImport } from "@/import/osm-import";
import type { OsmQuery } from "@/import/osm-import";
import { PreviewStep } from "@/import/preview-step";
import { useModelStore } from "@/model/model-store";
import { normalizeGeoJSON } from "@/model/normalize";
import { validateModel } from "@/model/validate";
import type {
  GeoJSONFeatureCollection,
  ModelFeature,
  ValidationReport,
} from "@/model/types";
import { CheckCircle2 } from "lucide-react";
import { m } from "@/i18n/messages";

type ImportStep = "upload" | "preview" | "done";
type ImportSource = "file" | "osm";

const IMPORT_SOURCES: readonly ImportSource[] = ["file", "osm"];

function isImportSource(value: string): value is ImportSource {
  return (IMPORT_SOURCES as readonly string[]).includes(value);
}

/**
 * The import wizard's step machine.
 *
 * The two sources sit in `src/import/` beside it — the same shape `pages/map.tsx`
 * and `src/map/` already have — and hand a `FeatureCollection` back. This file
 * owns what happens to one: normalize, validate, preview, and the replacement
 * the reader confirms.
 */
export default function ImportPage() {
  // The Overpass query lives here rather than in `OsmImport`, because the tab
  // strip unmounts an inactive panel: a box typed — or geolocated — into the
  // OSM tab would be gone the moment the reader glanced at the file tab.
  const [osmQuery, setOsmQuery] = useState<OsmQuery>({
    south: "",
    west: "",
    north: "",
    east: "",
    endpoint: "",
  });
  const [step, setStep] = useState<ImportStep>("upload");
  const [source, setSource] = useState<ImportSource>("file");
  const [features, setFeatures] = useState<ModelFeature[]>([]);
  const [skippedCount, setSkippedCount] = useState(0);
  const [report, setReport] = useState<ValidationReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const loadFeatures = useModelStore((s) => s.loadFeatures);
  const navigate = useNavigate();

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

  // `loadFeatures` replaces the workspace outright and clears placed
  // receivers with it, and no undo covers that — the command stack is reset,
  // not extended. So the import asks first.
  const [confirmingReplace, setConfirmingReplace] = useState(false);
  // Read while the dialog closes, which is before the next render, so a ref
  // rather than state.
  const replaced = useRef(false);
  const goToMapRef = useRef<HTMLButtonElement>(null);

  const handleConfirm = useCallback(() => {
    loadFeatures(features);
    replaced.current = true;
    setConfirmingReplace(false);
    setStep("done");
  }, [features, loadFeatures]);

  // Confirming removes the Import button the dialog was opened from, so Radix
  // has nothing to restore focus to and it would fall to `<body>`. The done
  // step's one action is where the reader is going next, and an effect is what
  // reaches it: Radix restores focus while the old content is being torn down,
  // before this button's ref is attached.
  useEffect(() => {
    if (step === "done") goToMapRef.current?.focus();
  }, [step]);

  const handleGoToMap = useCallback(() => {
    void navigate("/model");
  }, [navigate]);

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
                <FileImport
                  onCollection={handleNormalizeAndPreview}
                  onError={setError}
                />
              </TabsContent>

              <TabsContent value="osm">
                <OsmImport
                  query={osmQuery}
                  onQueryChange={setOsmQuery}
                  onCollection={handleNormalizeAndPreview}
                  onError={setError}
                />
              </TabsContent>
            </Tabs>

            {error ? <Callout variant="destructive">{error}</Callout> : null}
          </div>
        ) : null}

        {step === "preview" && report ? (
          <PreviewStep
            features={features}
            skippedCount={skippedCount}
            report={report}
            onBack={() => {
              setStep("upload");
            }}
            onImport={() => {
              setConfirmingReplace(true);
            }}
          />
        ) : null}

        {step === "done" ? (
          <div className="flex flex-col items-center gap-4 text-center">
            <CheckCircle2 className="size-10 text-success" aria-hidden="true" />
            <PageHeader
              className="justify-center text-center"
              title={m.status_import_complete()}
              description={`${String(features.length)} ${m.msg_import_complete_description()}`}
            />
            <Button ref={goToMapRef} onClick={handleGoToMap}>
              {m.action_go_to_map()}
            </Button>
          </div>
        ) : null}
      </div>

      <ConfirmDialog
        open={confirmingReplace}
        onOpenChange={setConfirmingReplace}
        tone="destructive"
        title={m.confirm_import_replace_title()}
        description={m.confirm_import_replace_desc({
          count: features.length,
        })}
        confirmLabel={m.action_import_features({ count: features.length })}
        onConfirm={handleConfirm}
        // The effect above owns where focus goes; this only stops Radix
        // sending it to a button that is no longer there first. Cancelling
        // keeps the ordinary restoration.
        onCloseAutoFocus={(event) => {
          if (!replaced.current) return;
          event.preventDefault();
        }}
      />
    </div>
  );
}
