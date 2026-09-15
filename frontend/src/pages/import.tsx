import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
import { planMerge, useModelStore } from "@/model/model-store";
import { normalizeModelGeoJSON } from "@/model/normalize";
import { validateProjectModel } from "@/model/validate";
import type {
  CalcArea,
  GeoJSONFeatureCollection,
  ModelFeature,
  ModelReceiver,
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
  const [receivers, setReceivers] = useState<ModelReceiver[]>([]);
  const [calcArea, setCalcArea] = useState<CalcArea | null>(null);
  const [skippedCount, setSkippedCount] = useState(0);
  const [report, setReport] = useState<ValidationReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const loadModel = useModelStore((s) => s.loadModel);
  const mergeModel = useModelStore((s) => s.mergeModel);
  const workspaceFeatures = useModelStore((s) => s.features);
  const workspaceReceivers = useModelStore((s) => s.receivers);
  const workspaceCalcArea = useModelStore((s) => s.calcArea);
  const navigate = useNavigate();

  /** Everything the import brings, receivers and calculation area included. */
  const importedCount = features.length + receivers.length;
  /** What the done step reports — an Add that skipped some lands fewer. */
  const [doneCount, setDoneCount] = useState(0);

  const workspaceEmpty =
    workspaceFeatures.length === 0 &&
    workspaceReceivers.length === 0 &&
    workspaceCalcArea === null;

  // Computed here, at preview time, and not inside the merge: the reader has
  // to see what Add will leave behind *before* choosing it. A dialog that
  // reports it afterwards is where the surprise lives.
  const mergeSkips = useMemo(
    () =>
      planMerge(
        {
          features: workspaceFeatures,
          receivers: workspaceReceivers,
          calcArea: workspaceCalcArea,
        },
        { features, receivers, calcArea },
      ).skipped,
    [
      workspaceFeatures,
      workspaceReceivers,
      workspaceCalcArea,
      features,
      receivers,
      calcArea,
    ],
  );

  // The whole v1 schema, not the three kinds the map draws as features.
  // `normalizeGeoJSON` reported `kind: "receiver"` as an unknown kind, so a
  // file `aconiq import` had written came in without its receivers and the
  // first save afterwards wrote that loss back into the project.
  //
  // The validator follows: `validateProjectModel` checks receiver ids against
  // feature ids and the receivers themselves, which a features-only report
  // cannot do — it would call a model with a duplicate receiver valid.
  const handleNormalizeAndPreview = useCallback(
    (collection: GeoJSONFeatureCollection) => {
      const result = normalizeModelGeoJSON(collection);
      setFeatures(result.features);
      setReceivers(result.receivers);
      setCalcArea(result.calcArea);
      setSkippedCount(result.skipped.length);
      setReport(validateProjectModel(result.features, result.receivers));
      setStep("preview");
    },
    [],
  );

  // Replace goes through `loadModel`, which drops the workspace outright and
  // resets the command stack rather than extending it, so no undo covers it —
  // hence the confirmation. Add is the opposite on both counts: it loses
  // nothing and it is a single undo, so asking would be an obstacle rather
  // than a safeguard.
  const [confirmingReplace, setConfirmingReplace] = useState(false);
  // Read while the dialog closes, which is before the next render, so a ref
  // rather than state.
  const replaced = useRef(false);
  const goToMapRef = useRef<HTMLButtonElement>(null);

  const handleConfirm = useCallback(() => {
    loadModel({ features, receivers, calcArea });
    setDoneCount(importedCount);
    replaced.current = true;
    setConfirmingReplace(false);
    setStep("done");
  }, [features, receivers, calcArea, importedCount, loadModel]);

  const handleAdd = useCallback(() => {
    const skipped = mergeModel({ features, receivers, calcArea });
    setDoneCount(importedCount - skipped.features - skipped.receivers);
    setStep("done");
  }, [features, receivers, calcArea, importedCount, mergeModel]);

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
            receivers={receivers}
            calcArea={calcArea}
            skippedCount={skippedCount}
            report={report}
            workspaceEmpty={workspaceEmpty}
            mergeSkips={mergeSkips}
            onBack={() => {
              setStep("upload");
            }}
            onAdd={handleAdd}
            onReplace={() => {
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
              description={`${String(doneCount)} ${m.msg_import_complete_description()}`}
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
          count: importedCount,
        })}
        confirmLabel={m.action_import_features({ count: importedCount })}
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
