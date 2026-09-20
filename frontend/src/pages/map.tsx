import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";
import { List, ShieldAlert, X } from "lucide-react";
import type { MapGeoJSONFeature } from "maplibre-gl";
import { Link, useNavigate, useSearchParams } from "react-router";
import { TooltipProvider } from "@/ui/components/tooltip";
import { Badge } from "@/ui/components/badge";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { PageHeader } from "@/ui/page-header";
import { MapView } from "@/map/map-view";
import { MapPanel } from "@/map/map-panel";
import { LayerControl } from "@/map/layer-control";
import { CoordinateDisplay } from "@/map/coordinate-display";
import { DrawToolbar } from "@/map/draw-toolbar";
import { FeatureEditor } from "@/map/feature-editor";
import { FeatureList } from "@/map/feature-list";
import { NewFeatureDialog } from "@/map/new-feature-dialog";
import { ValidationPanel } from "@/map/validation-panel";
import { FeatureFocus } from "@/map/feature-focus";
import type { FocusRequest } from "@/map/feature-focus";
import { FindingStepper } from "@/map/finding-stepper";
import { UndoRedoBar } from "@/map/undo-redo-bar";
import { ModelLayers } from "@/map/model-layers";
import { ResultLayers } from "@/map/result-layers";
import { fitViewToWorkspace } from "@/map/extent";
import { DISPLAY_CRS, useDisplayModel } from "@/map/display-model";
import { DrawProvider } from "@/map/draw-provider";
import { DRAW_PARAM, RUN_PARAM, SELECT_PARAM } from "@/map/map-params";
import { useDrawContext } from "@/map/use-draw-context";
import { useDrawProjection } from "@/map/use-draw-projection";
import type { DrawProjectionStatus } from "@/map/use-draw-projection";
import { useGeometryEdit } from "@/map/use-geometry-edit";
import type { DisplayModel } from "@/map/display-model";
import { useGlobalShortcut } from "@/ui/hooks/use-global-shortcut";
import { backend } from "@/api/backend";
import type { RunSummary } from "@/api/client";
import { RECEIVER_PARAM } from "@/results/results-params";
import type { CalcArea, Geometry, ModelFeature, Position } from "@/model/types";
import type { DrawMode } from "@/map/use-draw";
import { useModelStore } from "@/model/model-store";
import {
  modelValidation,
  useModelValidation,
} from "@/model/use-model-validation";
import {
  getAcousticsReviewed,
  markAcousticsReviewed,
  needsAcousticsReview,
} from "@/model/source-acoustics";
import {
  cursorFor,
  cursorPosition,
  findingQueue,
  stepFinding,
} from "@/model/finding-queue";
import type { FindingCursor } from "@/model/finding-queue";
import { m } from "@/i18n/messages";

/**
 * The model workspace, served at `/model`. The file keeps its name: renaming
 * it to `model.tsx` would sit confusingly beside `src/map/`, which is the
 * MapLibre layer this page composes.
 */
export default function MapPage() {
  return <MapWorkspace />;
}

/**
 * The empty-model hint, laid over the map rather than replacing it.
 *
 * It used to be an early return in `MapPage`, so on an empty model the canvas,
 * the draw toolbar, the layer control and the validation panel were all
 * unmounted — the user was told to start a workspace by a screen that had no
 * way to draw one.
 *
 * A labelled region, not a dialog: the map must stay pannable and the toolbar
 * reachable while this is up, so there is no focus trap and no autofocus. The
 * backdrop passes pointer events through; the card catches its own. The card
 * is opaque because it sits over a map raster, where neither a contrast rule
 * nor a reviewer can predict what is behind the text.
 */
/**
 * Arm a tool *and* clear the overlay. Arming alone leaves the card covering
 * the canvas the user was just told to click; dismissing alone leaves them on
 * an empty map beside a toolbar nobody pointed at.
 *
 * Point mode because a single click completes it and opens the new-feature
 * dialog — line and polygon need a double-click to finish, which strands a
 * first-timer. Callers must sit inside `DrawProvider`.
 */
function useStartDrawing(onDismiss: () => void): () => void {
  const { setMode } = useDrawContext();
  return useCallback(() => {
    setMode("point");
    onDismiss();
  }, [setMode, onDismiss]);
}

function WorkspaceStart({
  onDismiss,
  drawingDisabled,
  disabledReason,
}: {
  onDismiss: () => void;
  drawingDisabled: boolean;
  disabledReason: string;
}) {
  const startDrawing = useStartDrawing(onDismiss);
  const reasonId = useId();

  return (
    <div className="pointer-events-none absolute inset-0 z-20 flex items-center justify-center p-8">
      <Card
        role="region"
        aria-label={m.heading_map_workspace()}
        className="pointer-events-auto relative max-w-md bg-background p-6 shadow-lg"
      >
        <Button
          variant="ghost"
          size="icon"
          className="absolute right-2 top-2 size-7"
          onClick={onDismiss}
          aria-label={m.action_close()}
        >
          <X aria-hidden="true" />
        </Button>
        <div className="space-y-4 pr-8">
          <Badge variant="outline">{m.section_workspace()}</Badge>
          <PageHeader
            as="h3"
            title={m.heading_map_workspace()}
            description={m.msg_map_workspace_description()}
          />
          <div className="flex flex-wrap gap-3">
            <Button asChild>
              <Link to="/import">{m.action_import_data()}</Link>
            </Button>
            <Button
              variant="outline"
              onClick={startDrawing}
              disabled={drawingDisabled}
              aria-describedby={drawingDisabled ? reasonId : undefined}
            >
              {m.action_start_drawing()}
            </Button>
          </div>
          {/* Visible rather than a tooltip: this one is the *only* pointer to
              drawing an empty workspace has, and a dead button beside an
              "Import data" link reads as a bug unless it says why. */}
          {drawingDisabled ? (
            <p id={reasonId} className="text-xs text-muted-foreground">
              {disabledReason}
            </p>
          ) : null}
        </div>
      </Card>
    </div>
  );
}

/**
 * The id a clicked MapLibre feature stands for, or `null` if it carries none.
 *
 * `properties.id` before `feature.id` because the property is the app's own —
 * `ModelLayers` and `ResultLayers` both write the two together — while
 * `feature.id` is MapLibre's, optional, and typed `string | number`. The
 * fallback is kept for a source that set only the latter, and the
 * stringification with it: a numeric id would reach a URL and a store lookup
 * as a different key from the one the model holds.
 *
 * Shared by both click paths so that "which object was clicked" is answered
 * once. It used to be inline in the model handler, which is where a second
 * copy for the result handler would have started to drift from it.
 */
function clickedFeatureId(feature: MapGeoJSONFeature): string | null {
  const props = feature.properties as Record<string, unknown>;
  const featureId = props["id"] ?? feature.id;
  if (featureId == null) return null;
  return typeof featureId === "string"
    ? featureId
    : String(featureId as number);
}

function MapWorkspace() {
  const [editingFeatureId, setEditingFeatureId] = useState<string | null>(null);
  // Bumped on every selection, a repeat of the id already open included.
  // `GeometryEdit` re-arms on it: a feature deselected on the map has had its
  // terra-draw copy removed, so clicking it again reaches only MapLibre, which
  // sets the id this page already holds and would otherwise change nothing.
  const [selectionEpoch, setSelectionEpoch] = useState(0);
  const [newGeometry, setNewGeometry] = useState<Geometry | null>(null);
  // The keyboard path into the same dialog. A separate flag rather than a
  // sentinel geometry: the dialog tells the two apart by `geometry === null`,
  // and a placeholder shape would be a coordinate this page invented.
  const [coordinateEntry, setCoordinateEntry] = useState(false);
  // A focus request is not a selection: every focus selects, but a click on
  // the canvas selects without moving the camera under the reader's pointer.
  // `FeatureFocus` serves it; the epoch is what makes asking twice for the same
  // feature move the camera twice.
  const [focusRequest, setFocusRequest] = useState<FocusRequest | null>(null);
  // Where the reader stands in the queue of findings, or null when they are not
  // walking it. It carries the index as well as the id because fixing the
  // finding you are standing on takes it out of the queue — see `stepFinding`.
  const [queueCursor, setQueueCursor] = useState<FindingCursor | null>(null);
  const [showValidation, setShowValidation] = useState(false);
  const [showFeatureList, setShowFeatureList] = useState(false);
  // Local, deliberately: the overlay disappears on its own as soon as the
  // first feature lands, and it comes back when the model empties again or
  // the route is left. Persisting the dismissal would hide the only pointer
  // to "Start drawing" from the one user who needs it.
  const [startDismissed, setStartDismissed] = useState(false);
  // Which run's results the map draws, when the reader arrived from one. State
  // and not the URL parameter, because the parameter is stripped on arrival
  // while the choice has to outlive it; `null` means "the newest completed
  // run", which is what `ResultLayers` falls back to.
  const [requestedRunId, setRequestedRunId] = useState<string | null>(null);
  // Which run the map ended up drawing, reported by `ResultLayers` — not the
  // same thing as `requestedRunId`, which is `null` for every reader who did
  // not arrive from a results row, while the map still draws the newest
  // completed run for them. Both ways back to the table are keyed on this: a
  // click on a circle, and the editor's link for a model receiver.
  const [resultRun, setResultRun] = useState<RunSummary | null>(null);
  const navigate = useNavigate();
  const setCalcArea = useModelStore((s) => s.setCalcArea);
  const clearCalcArea = useModelStore((s) => s.clearCalcArea);
  const calcArea = useModelStore((s) => s.calcArea);
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);
  const crs = useModelStore((s) => s.crs);

  // The features a reader has to visit, in report order. `useModelValidation`
  // memoises process-wide, so `report` is the same object every consumer on
  // this page holds and this `useMemo` is stable between renders rather than
  // rebuilding a 608-entry array on each one.
  const { report: validationReport } = useModelValidation();
  const findings = useMemo(
    () => findingQueue(validationReport),
    [validationReport],
  );

  // The initial viewport is computed once, on mount. `useMemo` recomputed it on
  // every model edit, and MapView treats a new `center`/`zoom` as a reason to
  // rebuild — so drawing a single feature discarded the map. Later fitting is
  // ModelLayers' job (it calls `fitBounds` when data first arrives).
  const [workspaceView] = useState(() =>
    fitViewToWorkspace(features, receivers, calcArea, [10.45, 51.16], crs),
  );

  const hasWorkspaceContent =
    features.length > 0 || receivers.length > 0 || calcArea !== null;
  const showStart = !hasWorkspaceContent && !startDismissed;

  // The dismissal covers one empty-model episode, not the whole visit. Without
  // this, a user who draws a feature and later deletes the last one is left on
  // an empty map with the hint suppressed for the rest of the route — the one
  // state it exists for. Clearing it while content exists is invisible:
  // `showStart` is already false there.
  useEffect(() => {
    if (hasWorkspaceContent) setStartDismissed(false);
  }, [hasWorkspaceContent]);

  // terra-draw emits WGS84, and `setCalcArea`/`addFeature` write straight into
  // the store. In a store that holds metres that would mix degrees into the
  // model — a round trip through the map quietly becoming the model, which is
  // the one thing the map must never be. `useDrawProjection` closes that by
  // moving the shape into the store's CRS first; the draw-finish callback is
  // the only coordinate writer on the map side (`FeatureEditor` writes
  // attributes only, and nothing ever calls `draw.addFeatures`), so one inverse
  // transform there covers the whole surface.
  //
  // What is left to refuse is every case where that transform could not
  // succeed; `drawingDisabled` below enumerates them. Each way *into* an active
  // drawing mode takes that one flag — the toolbar, `?draw=1`, the start
  // panel's button — and `DrawGuard` disarms one that a late change has
  // invalidated.

  // Runs with coordinates already in the store's CRS, or not at all.
  const landGeometry = useCallback(
    (mode: DrawMode, geometry: Geometry) => {
      if (mode === "calc-area") {
        if (geometry.type === "Polygon") {
          const area: CalcArea = {
            geometry: {
              type: "Polygon",
              coordinates: geometry.coordinates as Position[][],
            },
          };
          setCalcArea(area);
        }
        return;
      }
      setNewGeometry(geometry);
    },
    [setCalcArea],
  );

  // The map's one `useDisplayModel` instance: `ModelLayers` draws this answer,
  // and the draw gate above refuses on it. A second call would reproject the
  // whole workspace a second time on every store change.
  const display = useDisplayModel();

  const drawProjection = useDrawProjection(landGeometry);
  const acceptDrawn = drawProjection.accept;

  // Three different reasons a finished shape could not be landed, and each one
  // has to close every way into a drawing mode rather than be discovered after
  // the shape is gone.
  //
  // 1. No projector at all. Keyed on the capability rather than on the CRS:
  //    both shipped modes can project — browser mode has the kernel in memory,
  //    API mode reaches `POST /api/v1/transform` — and a store already in WGS84
  //    needs no projector to draw into either way.
  // 2. A projector that cannot handle *this* CRS. `readCollectionCRS` accepts
  //    any `EPSG:<n>` an import declares, while the kernel supports a fixed set
  //    (`geo.epsgToCRS`), so a model in, say, EPSG:3035 reaches the store and
  //    fails every transform. The capability flag is global and stays true, so
  //    it cannot see this; the display projection having failed is the evidence
  //    that it happened, and it is the same transform the draw path would make.
  //    A map that cannot draw the model cannot place a new shape in it either.
  // 3. A shape already in flight. Terra-draw has removed the finished shape
  //    from the map, so a second one accepted now would make the first stale
  //    and drop it with nothing shown — the silent loss this whole path exists
  //    to avoid.
  const drawProjectionPending = drawProjection.status.status === "projecting";
  const displayUnprojectable = display.status === "failed";
  const drawingDisabled =
    (crs !== DISPLAY_CRS && !backend.capabilities.canReprojectForDisplay) ||
    displayUnprojectable ||
    drawProjectionPending;
  const drawingDisabledReason = drawProjectionPending
    ? m.msg_draw_disabled_projecting({ crs })
    : displayUnprojectable
      ? m.msg_draw_disabled_crs_unsupported({ crs })
      : m.msg_draw_disabled_no_projection({ crs });

  // The last line of defence, kept although no armed tool can reach it: the
  // gates above disarm every way in, and this one is what makes a new way in
  // fail closed rather than write degrees into a metric model.
  const handleDrawFinish = useCallback(
    (mode: DrawMode, feature: GeoJSON.Feature) => {
      if (drawingDisabled) return;
      acceptDrawn(mode, feature);
    },
    [drawingDisabled, acceptDrawn],
  );

  /**
   * A click on the map selects: it opens the editor on the feature and, through
   * `ModelLayers`, marks it on the canvas.
   *
   * There is no second surface any more. A popup used to open here as well,
   * rendering every property of the clicked feature as an HTML string built by
   * concatenation — a model field was therefore markup, and a feature imported
   * from OSM or a CSV could carry any. The editor shows the same properties as
   * form fields, which is both the safe way and the editable one.
   */
  const handleFeatureClick = useCallback((features: MapGeoJSONFeature[]) => {
    const feature = features[0];
    if (!feature) return;

    const featureId = clickedFeatureId(feature);
    if (featureId === null) return;

    setEditingFeatureId(featureId);
    setSelectionEpoch((epoch) => epoch + 1);
  }, []);

  /**
   * A click that found a computed receiver and no model object under it: the
   * reader is taken to that receiver's row in the results table.
   *
   * The other half of one rule, and the half that carries most of the traffic.
   * In the default `auto-grid` receiver mode the CLI names the receivers
   * itself — `grid-000000`… — so nothing under the pointer is in the model
   * store and the editor has nothing to open. `MapView` decides which of the
   * two happened, by which layer the hit came from, so there is no id-sniffing
   * here and no way for the two paths to disagree.
   *
   * The run is the one the map is drawing, which is why it is read from state
   * rather than from the URL: `?run=` was stripped on arrival, and a reader
   * who never carried one is still looking at the newest completed run's
   * levels. Both segments are escaped — a receiver id comes out of an import
   * and is constrained to nothing, so a `/` in one would otherwise become a
   * path of its own.
   */
  const handleResultReceiverClick = useCallback(
    (features: MapGeoJSONFeature[]) => {
      const feature = features[0];
      if (!feature) return;

      const receiverId = clickedFeatureId(feature);
      // No run means no circles were drawn, so this cannot be reached through
      // the canvas; refusing is still cheaper than proving it unreachable, and
      // `/results/` with no id is a page that explains nothing.
      if (receiverId === null || resultRun === null) return;

      const params = new URLSearchParams({ [RECEIVER_PARAM]: receiverId });
      void navigate(
        `/results/${encodeURIComponent(resultRun.id)}?${params.toString()}`,
      );
    },
    [navigate, resultRun],
  );

  /**
   * Take the reader to a feature: select it, and move the camera to it.
   *
   * The one funnel for every way into a feature the reader cannot see — the
   * validation panel, the feature list, the `?select=` deep link and the
   * finding stepper. A click on the canvas deliberately does not come through
   * here: the reader is already looking at what they clicked, and `handleFeatureClick`
   * arms terra-draw on the same feature, so a camera flight would slide the
   * vertex handles out from under the drag that follows.
   *
   * Stable, like the handler it replaces: `ArrivalParams` has it in an effect's
   * dependency list.
   */
  const focusFeature = useCallback((featureId: string) => {
    setEditingFeatureId(featureId);
    setSelectionEpoch((epoch) => epoch + 1);
    setFocusRequest((previous) => ({
      featureId,
      epoch: (previous?.epoch ?? 0) + 1,
    }));
    setShowValidation(false);
  }, []);

  /** "Go to" from the validation panel: focus it, and start walking from there. */
  const handleSelectFromValidation = useCallback(
    (featureId: string) => {
      focusFeature(featureId);
      setQueueCursor(cursorFor(findings, featureId));
    },
    [focusFeature, findings],
  );

  const handleStep = useCallback(
    (direction: 1 | -1) => {
      const next = stepFinding(findings, queueCursor, direction);
      setQueueCursor(next);
      if (next) focusFeature(next.featureId);
    },
    [findings, queueCursor, focusFeature],
  );

  /**
   * Accepts a set of imported sources as reviewed, in one undoable step.
   *
   * The store is read here rather than subscribed to: this runs on a click, and
   * a page that re-rendered on every feature change to keep 608 of them in hand
   * would pay for the bulk action on every edit that is not one.
   */
  const signOffFeatures = useCallback((featureIds: string[]) => {
    const { features: held, updateFeatures } = useModelStore.getState();
    const byId = new Map(held.map((f) => [f.id, f]));
    const next: ModelFeature[] = [];
    for (const id of featureIds) {
      const feature = byId.get(id);
      // Skip what is already signed off: `markAcousticsReviewed` would return a
      // new object for it anyway, and the undo step should hold only the
      // features this click actually changed.
      if (feature && !getAcousticsReviewed(feature)) {
        next.push(markAcousticsReviewed(feature, true));
      }
    }
    updateFeatures(next);
  }, []);

  /**
   * The stepper's "reviewed, next", or `null` when the finding under the cursor
   * is not one a sign-off retires.
   *
   * It does not go through `handleStep`, which closes over the `findings` of
   * the render it was built in — the queue as it stood *before* the sign-off.
   * Stepping against that is how accepting the last of three used to wrap back
   * to the first, while correcting the same finding by hand and pressing "next"
   * landed on the one that took its place. The two have to mean the same thing,
   * so the queue is re-derived from the store the write just changed.
   *
   * Re-deriving is not a second validation: `modelValidation` is the memo the
   * hook reads, keyed on the store's array identities, so the render that
   * follows this write finds the result already computed.
   */
  const cursorFeature = useModelStore((s) =>
    queueCursor === null ? undefined : s.getFeatureById(queueCursor.featureId),
  );
  const handleSignOffAndStep = useMemo(() => {
    if (cursorFeature === undefined || !needsAcousticsReview(cursorFeature)) {
      return null;
    }
    return () => {
      signOffFeatures([cursorFeature.id]);

      const { features: held, receivers: heldReceivers } =
        useModelStore.getState();
      const nextFindings = findingQueue(
        modelValidation(held, heldReceivers).report,
      );
      const next = stepFinding(nextFindings, queueCursor, 1);
      setQueueCursor(next);
      if (next) focusFeature(next.featureId);
    };
  }, [cursorFeature, signOffFeatures, queueCursor, focusFeature]);

  // Alt+arrows rather than bare keys: the docked editor is full of number
  // fields, and the hook only bows out of text entry, not of the whole panel.
  // `enabled` detaches the listener entirely while nobody is stepping.
  useGlobalShortcut(
    { key: "ArrowDown", alt: true, enabled: queueCursor !== null },
    () => {
      handleStep(1);
    },
  );
  useGlobalShortcut(
    { key: "ArrowUp", alt: true, enabled: queueCursor !== null },
    () => {
      handleStep(-1);
    },
  );

  // Where the stepper says the reader is. Derived rather than read off the
  // cursor, because editing moves the queue under it: the stored index is what
  // "next" steps from, while this is what the reader is shown, and rendering
  // the two as one number is how fixing the last of 608 findings read
  // "608 / 607". `null` also unmounts the stepper once the queue empties.
  const queuePosition = cursorPosition(findings, queueCursor);

  // Stable, because `ArrivalParams` has it in an effect's dependency list and
  // an inline arrow there would re-run the effect on every render.
  const handleStartDismissed = useCallback(() => {
    setStartDismissed(true);
  }, []);

  return (
    <TooltipProvider>
      {/* The map canvas carries no visible heading; the page outline still
          needs one under the shell's h1, and the E2E helper waits for it. */}
      <h2 className="sr-only">{m.nav_model()}</h2>
      <MapView
        center={workspaceView.center}
        zoom={workspaceView.zoom}
        onFeatureClick={handleFeatureClick}
        onResultReceiverClick={handleResultReceiverClick}
      >
        <DrawProvider onFinish={handleDrawFinish}>
          <ModelLayers display={display} selectedFeatureId={editingFeatureId} />
          {/* After the model layers, so a `?select=` deep link arriving on the
              first paint of a project wins over their one-shot fit to the whole
              workspace — React runs child effects in tree order. */}
          <FeatureFocus display={display} request={focusRequest} />
          {/* After the model layers, so the computed levels read on top of
              the sources that produced them rather than under a building fill.
              The result *raster* goes the other way and is inserted below them
              with a `beforeId`, because it arrives long after this commit. */}
          <ResultLayers
            requestedRunId={requestedRunId}
            onRunDrawn={setResultRun}
          />
          <GeometryEdit
            display={display}
            featureId={editingFeatureId}
            selectionEpoch={selectionEpoch}
            disabled={drawingDisabled}
          />
          <DrawGuard disabled={drawingDisabled} />
          <DrawShortcuts />
          <WorkspaceDrawToolbar
            disabled={drawingDisabled}
            disabledReason={drawingDisabledReason}
            onCoordinateEntry={() => {
              setCoordinateEntry(true);
            }}
          />
          <DrawProjectionNotice
            status={drawProjection.status}
            onDismiss={drawProjection.dismiss}
          />
          <LayerControl />
          <CoordinateDisplay />
          <FeatureEditor
            featureId={editingFeatureId}
            resultRun={resultRun}
            onClose={() => {
              setEditingFeatureId(null);
            }}
          />
          <UndoRedoBar />
          {calcArea ? (
            <MapPanel
              position="bottom-right"
              inset="bottom-14"
              translucent
              className="flex items-center gap-1 py-1 pl-1 pr-1"
            >
              <Badge variant="info">{m.label_calc_area()}</Badge>
              <Button
                variant="ghost"
                size="icon"
                className="size-6"
                onClick={clearCalcArea}
                aria-label={m.action_clear_calc_area()}
              >
                <X aria-hidden="true" />
              </Button>
            </MapPanel>
          ) : null}
          <FeatureListToggle
            open={showFeatureList}
            onToggle={() => {
              setShowFeatureList((open) => !open);
            }}
          />
          {showFeatureList ? (
            // Through the funnel, not `setEditingFeatureId`: picking row 400 of
            // 608 from a list is the "I cannot see it" case, and the raw setter
            // also skipped the epoch bump, so re-picking the open feature never
            // re-armed the geometry editor.
            <FeatureList
              selectedId={editingFeatureId}
              onSelect={focusFeature}
            />
          ) : null}
          <ValidationToggle
            open={showValidation}
            onToggle={() => {
              setShowValidation((open) => !open);
            }}
          />
          {showValidation ? (
            <MapPanel
              position="bottom-left"
              inset="bottom-14 left-3"
              width="w-80"
              className="p-0"
              role="region"
              aria-label={m.label_validation()}
            >
              <ValidationPanel
                onSelectFeature={handleSelectFromValidation}
                onSignOffGroup={signOffFeatures}
              />
            </MapPanel>
          ) : null}
          {queuePosition !== null ? (
            <FindingStepper
              position={queuePosition}
              total={findings.length}
              onStep={handleStep}
              onSignOff={handleSignOffAndStep}
              onClose={() => {
                setQueueCursor(null);
              }}
            />
          ) : null}
          <ArrivalParams
            drawDisabled={drawingDisabled}
            onDismiss={handleStartDismissed}
            onSelect={handleSelectFromValidation}
            onRun={setRequestedRunId}
          />
          {showStart ? (
            <WorkspaceStart
              drawingDisabled={drawingDisabled}
              disabledReason={drawingDisabledReason}
              onDismiss={() => {
                setStartDismissed(true);
              }}
            />
          ) : null}
        </DrawProvider>
      </MapView>
      {/* One dialog for both ways in. A drawn shape arrives as `geometry`; the
          keyboard path opens it with none, and the dialog asks for the
          coordinates instead — in the store's own CRS, with no transform on
          that path at all. See `new-feature-dialog.tsx` for why that is not a
          second exception to "a projection *of* the model, never a source
          *for* it". */}
      <NewFeatureDialog
        open={newGeometry !== null || coordinateEntry}
        geometry={newGeometry}
        onClose={() => {
          setNewGeometry(null);
          setCoordinateEntry(false);
        }}
      />
    </TooltipProvider>
  );
}

/**
 * Honours every parameter `/model` takes on arrival, then strips them all in
 * one navigation.
 *
 * One component and one effect, which is what this replaced. `DrawRequest` and
 * `SelectRequest` each cleared the **whole** query string with
 * `setParams({}, { replace: true })`, and that was safe only while exactly one
 * parameter was ever honoured. The results page now links with two at once, and
 * two effects in one commit would each delete the other's key before it had
 * been read. A per-key fix in each would still race: React Router's updater
 * sees the search params of the render it was called from, not of the sibling
 * effect that just ran.
 *
 * Each handler keeps the meaning it had:
 *
 * `?draw=1` is the project page's "Start drawing" — a boolean rather than a
 * mode name, so which mode drawing starts in stays a decision this file makes
 * once. `drawDisabled` is the same CRS gate the toolbar takes; without it the
 * link armed point mode over a metric model, the user drew a shape, and
 * `handleDrawFinish` dropped it without a word. A refused request is still
 * stripped, or it would be re-asked on every reload, and it leaves the start
 * panel standing, because that panel is where the reason is written.
 *
 * `?select=<featureId>` is the import page's finding link. `onSelect` is
 * `handleSelectFromValidation`, the move the validation panel already makes;
 * a second one written here would be two ways to select a feature that must
 * not diverge.
 *
 * `?run=<runId>` is the results page's row link. It is handed to state rather
 * than acted on, because unlike the other two its effect lasts: the map keeps
 * drawing that run until the page is left.
 *
 * Anything else in the query string is preserved — this strips its own three
 * keys and not the URL.
 */
function ArrivalParams({
  drawDisabled,
  onDismiss,
  onSelect,
  onRun,
}: {
  drawDisabled: boolean;
  onDismiss: () => void;
  onSelect: (featureId: string) => void;
  onRun: (runId: string) => void;
}) {
  // Must therefore sit inside `DrawProvider`, as its predecessor did.
  const onDraw = useStartDrawing(onDismiss);
  const [params, setParams] = useSearchParams();
  const draw = params.get(DRAW_PARAM) === "1";
  const select = params.get(SELECT_PARAM) ?? "";
  const run = params.get(RUN_PARAM) ?? "";
  const handled = useRef(false);

  useEffect(() => {
    if (handled.current) return;
    if (!draw && select === "" && run === "") return;
    handled.current = true;

    // The run first: it decides which run's levels the map draws, and the
    // selection below may be a receiver read off that run's table.
    if (run !== "") onRun(run);
    if (select !== "") onSelect(select);
    if (draw && !drawDisabled) onDraw();

    setParams(
      (current) => {
        const next = new URLSearchParams(current);
        next.delete(DRAW_PARAM);
        next.delete(SELECT_PARAM);
        next.delete(RUN_PARAM);
        return next;
      },
      { replace: true },
    );
  }, [draw, select, run, drawDisabled, onDraw, onSelect, onRun, setParams]);

  return null;
}

/**
 * Feeds the selected feature into terra-draw's select mode, so it can be
 * reshaped, and reads the result back into the model.
 *
 * A component rather than a call in `MapWorkspace` for the reason
 * `draw-provider.tsx` gives: everything that talks to terra-draw has to run
 * **inside** `DrawProvider`, and being a child is what makes that structural
 * instead of a convention.
 *
 * It takes the same `drawingDisabled` every other way into an active tool takes.
 * A model the map cannot project cannot be reshaped either — the reshape goes
 * back through the same inverse transform a drawn shape does, and refusing
 * afterwards would mean refusing a shape the user had already moved.
 */
function GeometryEdit({
  display,
  featureId,
  selectionEpoch,
  disabled,
}: {
  display: DisplayModel;
  featureId: string | null;
  selectionEpoch: number;
  disabled: boolean;
}) {
  useGeometryEdit({ display, featureId, selectionEpoch, disabled });
  return null;
}

/**
 * Disarms the tools when drawing stops being allowed under an already active
 * mode.
 *
 * The gates on the entry points decide with the CRS the store held at the time.
 * `use-project-hydration` loads the model *after* mount, so `/model?draw=1` can
 * arm point mode over an empty 4326 store one tick before a metric model lands
 * in it — and the user is then drawing into a model whose shape will be
 * refused. Cancelling is the only honest move: the toolbar has already greyed
 * out around them, and the notice says why.
 */
function DrawGuard({ disabled }: { disabled: boolean }) {
  const { activeMode, cancel } = useDrawContext();

  useEffect(() => {
    if (disabled && activeMode !== "static") cancel();
  }, [disabled, activeMode, cancel]);

  return null;
}

/**
 * Esc abandons the shape in progress and puts the tools back to select-nothing
 * — the same thing the toolbar's X does, which is why it goes through
 * `useDrawContext().cancel` rather than reaching for terra-draw.
 *
 * Armed only while a tool is: Esc belongs to whatever is on top of the map when
 * nothing is being drawn, and `useGlobalShortcut` calls `preventDefault`, so an
 * always-on binding would quietly take Esc away from every dialog on the route.
 * The hook's own text-entry guard covers the editor's inputs.
 */
function DrawShortcuts() {
  const { activeMode, cancel } = useDrawContext();
  useGlobalShortcut(
    { key: "Escape", enabled: activeMode !== "static" },
    cancel,
  );
  return null;
}

/**
 * What became of the shape between terra-draw finishing it and the store
 * holding it.
 *
 * Nothing at all while the store is in WGS84, which needs no transform and
 * stays synchronous. Over a metric model the round trip through the projection
 * is a visible step: `role="status"` announces it, and a failure stays up with
 * the reason until it is dismissed, because the alternative — the old
 * behaviour — was a completed shape vanishing without a word.
 *
 * It sits beside the draw toolbar rather than in a free corner: it is about the
 * shape just drawn, and the toolbar is where the user's attention already is.
 */
function DrawProjectionNotice({
  status,
  onDismiss,
}: {
  status: DrawProjectionStatus;
  onDismiss: () => void;
}) {
  if (status.status === "idle") return null;

  return (
    <MapPanel
      position="top-left"
      inset="left-16 top-3"
      width="w-72"
      translucent
      role="status"
      aria-label={m.label_draw_projection()}
      className="space-y-1 text-xs leading-relaxed"
    >
      {status.status === "projecting" ? (
        <p>{m.msg_draw_projecting({ crs: status.targetCRS })}</p>
      ) : (
        <>
          <p>
            {m.msg_draw_projection_failed({
              crs: status.targetCRS,
              reason: status.error.message,
            })}
          </p>
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs"
            onClick={onDismiss}
          >
            {m.action_close()}
          </Button>
        </>
      )}
    </MapPanel>
  );
}

/** Binds the presentational toolbar to the provider above it. */
function WorkspaceDrawToolbar({
  disabled,
  disabledReason,
  onCoordinateEntry,
}: {
  disabled: boolean;
  disabledReason: string;
  onCoordinateEntry: () => void;
}) {
  const { activeMode, setMode, cancel } = useDrawContext();
  return (
    <DrawToolbar
      activeMode={activeMode}
      onModeChange={setMode}
      onCancel={cancel}
      disabled={disabled}
      disabledReason={disabledReason}
      onCoordinateEntry={onCoordinateEntry}
    />
  );
}

/**
 * Opens the feature list. Modelled on `ValidationToggle` below, down to the
 * count badge: the two are the same kind of control — a switch for a panel
 * that reads the model — and the map already has enough different-looking
 * buttons on it.
 */
function FeatureListToggle({
  open,
  onToggle,
}: {
  open: boolean;
  onToggle: () => void;
}) {
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);
  const count = features.length + receivers.length;

  return (
    <Button
      variant="outline"
      size="sm"
      className="absolute right-56 top-3 z-10 h-8 gap-1.5 text-xs shadow-md"
      aria-pressed={open}
      onClick={onToggle}
    >
      <List aria-hidden="true" className="size-3.5" />
      {m.label_feature_list()}
      {count > 0 ? (
        <span className="rounded-full bg-secondary px-1.5 py-0.5 text-2xs tabular-nums">
          {String(count)}
        </span>
      ) : null}
    </Button>
  );
}

/**
 * Opens the validation panel. Without it the panel was unreachable: nothing in
 * the workspace ever set `showValidation` to `true`, so `ValidationPanel` and
 * `handleSelectFromValidation` were wired to a switch that did not exist.
 */
function ValidationToggle({
  open,
  onToggle,
}: {
  open: boolean;
  onToggle: () => void;
}) {
  const { errorCount, warningCount } = useModelValidation();
  const issueCount = errorCount + warningCount;

  return (
    <Button
      variant="outline"
      size="sm"
      className="absolute bottom-3 left-3 z-10 h-8 gap-1.5 text-xs shadow-md"
      aria-pressed={open}
      onClick={onToggle}
    >
      <ShieldAlert
        aria-hidden="true"
        className={errorCount > 0 ? "size-3.5 text-destructive" : "size-3.5"}
      />
      {m.label_validation()}
      {issueCount > 0 ? (
        <span className="rounded-full bg-secondary px-1.5 py-0.5 text-2xs tabular-nums">
          {String(issueCount)}
        </span>
      ) : null}
    </Button>
  );
}
