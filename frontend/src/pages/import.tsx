import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router";
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
  ValidationIssue,
  ValidationReport,
} from "@/model/types";
import { CheckCircle2, XCircle } from "lucide-react";
import { SELECT_PARAM } from "@/map/map-params";
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
  //
  // "Nothing to import" is answered here, *above* the validator, the way
  // `useModelValidation` answers "empty". `validateProjectModel` pushes a
  // synthetic `model.empty` error whose message is hardcoded English and whose
  // own comment says it must never reach the UI — and this page calls the
  // validator directly, so an empty FeatureCollection printed that English
  // string verbatim to a German reader. A file carrying only a calculation
  // area is the same case seen from the other side: there is something to
  // import and nothing for the validator to check, so it is not run at all.
  const handleNormalizeAndPreview = useCallback(
    (collection: GeoJSONFeatureCollection) => {
      const result = normalizeModelGeoJSON(collection);
      if (
        result.features.length === 0 &&
        result.receivers.length === 0 &&
        result.calcArea === null
      ) {
        setError(m.msg_import_nothing());
        return;
      }
      setFeatures(result.features);
      setReceivers(result.receivers);
      setCalcArea(result.calcArea);
      setSkippedCount(result.skipped.length);
      setReport(
        result.features.length === 0 && result.receivers.length === 0
          ? null
          : validateProjectModel(result.features, result.receivers),
      );
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

  // The findings the done step offers to open on the map. Only those whose
  // feature actually landed: an Add skips what the workspace already holds,
  // and a link to a feature the import did not bring selects nothing.
  const [doneErrors, setDoneErrors] = useState<ValidationIssue[]>([]);

  const errorsFor = useCallback(
    (landed: Iterable<string>) => {
      const ids = new Set(landed);
      return (report?.errors ?? []).filter(
        (issue) => issue.featureId !== "" && ids.has(issue.featureId),
      );
    },
    [report],
  );

  const handleConfirm = useCallback(() => {
    loadModel({ features, receivers, calcArea });
    setDoneCount(importedCount);
    setDoneErrors(
      errorsFor([...features.map((f) => f.id), ...receivers.map((r) => r.id)]),
    );
    replaced.current = true;
    setConfirmingReplace(false);
    setStep("done");
  }, [features, receivers, calcArea, importedCount, loadModel, errorsFor]);

  const handleAdd = useCallback(() => {
    const landed = planMerge(
      {
        features: workspaceFeatures,
        receivers: workspaceReceivers,
        calcArea: workspaceCalcArea,
      },
      { features, receivers, calcArea },
    );
    const skipped = mergeModel({ features, receivers, calcArea });
    setDoneCount(importedCount - skipped.features - skipped.receivers);
    setDoneErrors(
      errorsFor([
        ...landed.features.map((f) => f.id),
        ...landed.receivers.map((r) => r.id),
      ]),
    );
    setStep("done");
  }, [
    features,
    receivers,
    calcArea,
    importedCount,
    mergeModel,
    errorsFor,
    workspaceFeatures,
    workspaceReceivers,
    workspaceCalcArea,
  ]);

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

        {step === "preview" ? (
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

            {/* Here the findings can be acted on: the features are in the
                store, so each link opens the editor on the one it names. */}
            {doneErrors.length > 0 ? (
              <Callout
                variant="destructive"
                icon={XCircle}
                className="text-left"
                title={`${String(doneErrors.length)} ${m.status_validation_errors()}`}
              >
                <p>{m.msg_import_errors_remain()}</p>
                <ul className="mt-2 space-y-1">
                  {doneErrors.map((issue, i) => (
                    <li key={i}>
                      {issue.message}{" "}
                      <Button
                        asChild
                        variant="link"
                        size="sm"
                        className="h-auto p-0"
                      >
                        <Link
                          to={`/model?${SELECT_PARAM}=${encodeURIComponent(issue.featureId)}`}
                        >
                          {m.action_show_feature_on_map({
                            id: issue.featureId,
                          })}
                        </Link>
                      </Button>
                    </li>
                  ))}
                </ul>
              </Callout>
            ) : null}
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
