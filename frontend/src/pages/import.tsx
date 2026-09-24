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
import { LglnImport } from "@/import/lgln-import";
import type { LglnImportResponse } from "@/api/client";
import type { LonLatBBox } from "@/model/footprint";
import { PreviewStep } from "@/import/preview-step";
import {
  countModelObjects,
  planMerge,
  planReplaceBuildings,
  useModelStore,
} from "@/model/model-store";
import type { LoadedModel } from "@/model/model-store";
import { normalizeModelGeoJSON } from "@/model/normalize";
import { validateProjectModel } from "@/model/validate";
import { validationIssueText } from "@/model/validation-message";
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
type ImportSource = "file" | "osm" | "lgln";

const IMPORT_SOURCES: readonly ImportSource[] = ["file", "osm", "lgln"];

/**
 * What an LGLN load brought besides its features: the box it was asked for,
 * which the replacement needs again at confirm time, and what the preview has
 * to show with the data.
 */
interface LglnLoad {
  bbox: LonLatBBox;
  tileCount: number;
  attribution: string;
}

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
  //
  // The LGLN tab reads and writes the same box. Its buildings are meant to
  // replace the ones an OSM fetch brought, so the box they are loaded for is
  // the one the OSM fetch used unless the reader changes it.
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
  // What the file declared, or null when it declared nothing. Held rather than
  // defaulted here: `loadModel` and `mergeModel` answer "nothing declared"
  // differently, and only they know what the workspace already holds.
  const [importCRS, setImportCRS] = useState<string | null>(null);
  const [skippedCount, setSkippedCount] = useState(0);
  const [report, setReport] = useState<ValidationReport | null>(null);
  // Set only while the import in hand came from the LGLN tab: it switches the
  // preview from Add/Replace to replacing the OSM buildings in its box.
  const [lglnLoad, setLglnLoad] = useState<LglnLoad | null>(null);
  const [error, setError] = useState<string | null>(null);
  const loadModel = useModelStore((s) => s.loadModel);
  const mergeModel = useModelStore((s) => s.mergeModel);
  const replaceBuildingsInBBox = useModelStore((s) => s.replaceBuildingsInBBox);
  const workspaceCRS = useModelStore((s) => s.crs);
  const workspaceFeatures = useModelStore((s) => s.features);
  const workspaceReceivers = useModelStore((s) => s.receivers);
  const workspaceCalcArea = useModelStore((s) => s.calcArea);
  const navigate = useNavigate();

  /** Everything the import brings, receivers and calculation area included. */
  const importedCount = countModelObjects({ features, receivers, calcArea });
  /** What the done step reports — an Add that skipped some lands fewer. */
  const [doneCount, setDoneCount] = useState(0);

  const workspaceEmpty =
    workspaceFeatures.length === 0 &&
    workspaceReceivers.length === 0 &&
    workspaceCalcArea === null;

  // Computed here, at preview time, and not inside the merge: the reader has
  // to see what Add will leave behind *before* choosing it. A dialog that
  // reports it afterwards is where the surprise lives.
  //
  // An LGLN load is planned the same way, with the OSM buildings it removes
  // counted first, so the preview can say "removes n, adds m" before the
  // reader commits to it.
  const replacePlan = useMemo(
    () =>
      lglnLoad === null
        ? null
        : planReplaceBuildings(
            {
              features: workspaceFeatures,
              receivers: workspaceReceivers,
              calcArea: workspaceCalcArea,
              crs: workspaceCRS,
            },
            { features, receivers, calcArea },
            lglnLoad.bbox,
          ),
    [
      lglnLoad,
      workspaceFeatures,
      workspaceReceivers,
      workspaceCalcArea,
      workspaceCRS,
      features,
      receivers,
      calcArea,
    ],
  );
  const mergeSkips = useMemo(
    () =>
      replacePlan?.merge.skipped ??
      planMerge(
        {
          features: workspaceFeatures,
          receivers: workspaceReceivers,
          calcArea: workspaceCalcArea,
        },
        { features, receivers, calcArea },
      ).skipped,
    [
      replacePlan,
      workspaceFeatures,
      workspaceReceivers,
      workspaceCalcArea,
      features,
      receivers,
      calcArea,
    ],
  );

  // The whole v1 schema, not the three kinds the map draws as features.
  // The feature-only reader this page used to call reported `kind: "receiver"`
  // as an unknown kind, so a file `aconiq import` had written came in without
  // its receivers and the first save afterwards wrote that loss back into the
  // project.
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
      setImportCRS(result.crs);
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

  // Every source but LGLN clears the LGLN context, so a file picked after an
  // LGLN load is previewed as the file it is.
  const handleCollection = useCallback(
    (collection: GeoJSONFeatureCollection) => {
      setLglnLoad(null);
      handleNormalizeAndPreview(collection);
    },
    [handleNormalizeAndPreview],
  );

  const handleLglnCollection = useCallback(
    (collection: LglnImportResponse, bbox: LonLatBBox) => {
      setLglnLoad({
        bbox,
        tileCount: collection.tiles.length,
        attribution: collection.attribution,
      });
      handleNormalizeAndPreview(collection);
    },
    [handleNormalizeAndPreview],
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

  // Revalidated against the model the import actually produced, not filtered
  // out of the preview's report. An Add skips an incoming id the workspace
  // already holds, and skipping can *resolve* a finding: a file whose feature
  // and receiver share an id is reported `receiver.id.duplicate` before the
  // merge, and after it only the feature landed, so nothing is duplicated any
  // more. The pre-merge report still names that id, and that id did land, so
  // filtering by the landed ids kept a finding the workspace no longer has.
  // Narrowing to the landed ids afterwards is still needed for the other half:
  // a finding the reader cannot act on here is one against a feature that was
  // already in the workspace before the import.
  const errorsFor = useCallback(
    (merged: LoadedModel, landed: Iterable<string>) => {
      const ids = new Set(landed);
      return validateProjectModel(
        merged.features,
        merged.receivers,
      ).errors.filter(
        (issue) => issue.featureId !== "" && ids.has(issue.featureId),
      );
    },
    [],
  );

  const handleConfirm = useCallback(() => {
    loadModel({
      features,
      receivers,
      calcArea,
      ...(importCRS !== null && { crs: importCRS }),
    });
    setDoneCount(importedCount);
    setDoneErrors(
      errorsFor({ features, receivers, calcArea }, [
        ...features.map((f) => f.id),
        ...receivers.map((r) => r.id),
      ]),
    );
    replaced.current = true;
    setConfirmingReplace(false);
    setStep("done");
  }, [
    features,
    receivers,
    calcArea,
    importCRS,
    importedCount,
    loadModel,
    errorsFor,
  ]);

  // The LGLN confirm: one undoable step that removes the OSM buildings in the
  // box and merges the rest, planned by the same function the preview read.
  const handleReplaceBuildings = useCallback(() => {
    if (lglnLoad === null) return;
    const current = {
      features: workspaceFeatures,
      receivers: workspaceReceivers,
      calcArea: workspaceCalcArea,
      crs: workspaceCRS,
    };
    const incoming = {
      features,
      receivers,
      calcArea,
      ...(importCRS !== null && { crs: importCRS }),
    };
    const plan = planReplaceBuildings(current, incoming, lglnLoad.bbox);
    replaceBuildingsInBBox(incoming, lglnLoad.bbox);
    const landed = plan.merge;
    const gone = new Set(plan.removed);
    setDoneCount(
      landed.features.length +
        landed.receivers.length +
        (landed.calcArea !== null && workspaceCalcArea === null ? 1 : 0),
    );
    setDoneErrors(
      errorsFor(
        {
          features: [
            ...workspaceFeatures.filter((f) => !gone.has(f)),
            ...landed.features,
          ],
          receivers: [...workspaceReceivers, ...landed.receivers],
          calcArea: landed.calcArea,
        },
        [
          ...landed.features.map((f) => f.id),
          ...landed.receivers.map((r) => r.id),
        ],
      ),
    );
    setStep("done");
  }, [
    lglnLoad,
    features,
    receivers,
    calcArea,
    importCRS,
    replaceBuildingsInBBox,
    errorsFor,
    workspaceFeatures,
    workspaceReceivers,
    workspaceCalcArea,
    workspaceCRS,
  ]);

  const handleAdd = useCallback(() => {
    const landed = planMerge(
      {
        features: workspaceFeatures,
        receivers: workspaceReceivers,
        calcArea: workspaceCalcArea,
      },
      { features, receivers, calcArea },
    );
    const skipped = mergeModel({
      features,
      receivers,
      calcArea,
      ...(importCRS !== null && { crs: importCRS }),
    });
    setDoneCount(
      importedCount -
        skipped.features -
        skipped.receivers -
        (skipped.calcArea ? 1 : 0),
    );
    setDoneErrors(
      errorsFor(
        {
          features: [...workspaceFeatures, ...landed.features],
          receivers: [...workspaceReceivers, ...landed.receivers],
          calcArea: landed.calcArea,
        },
        [
          ...landed.features.map((f) => f.id),
          ...landed.receivers.map((r) => r.id),
        ],
      ),
    );
    setStep("done");
  }, [
    features,
    receivers,
    calcArea,
    importCRS,
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
                <TabsTrigger value="lgln">
                  {m.action_import_from_lgln()}
                </TabsTrigger>
              </TabsList>

              <TabsContent value="file">
                <FileImport
                  onCollection={handleCollection}
                  onError={setError}
                />
              </TabsContent>

              <TabsContent value="osm">
                <OsmImport
                  query={osmQuery}
                  onQueryChange={setOsmQuery}
                  onCollection={handleCollection}
                  onError={setError}
                />
              </TabsContent>

              <TabsContent value="lgln">
                <LglnImport
                  bbox={osmQuery}
                  onBBoxChange={setOsmQuery}
                  onCollection={handleLglnCollection}
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
            lgln={
              lglnLoad === null || replacePlan === null
                ? null
                : {
                    removed: replacePlan.removed.length,
                    added: replacePlan.merge.features.length,
                    comparable: replacePlan.comparable,
                    tileCount: lglnLoad.tileCount,
                    attribution: lglnLoad.attribution,
                  }
            }
            onReplaceBuildings={handleReplaceBuildings}
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
                title={m.status_validation_errors({
                  count: doneErrors.length,
                })}
              >
                <p>{m.msg_import_errors_remain()}</p>
                <ul className="mt-2 space-y-1">
                  {doneErrors.map((issue, i) => (
                    <li key={i}>
                      {validationIssueText(issue)}{" "}
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
