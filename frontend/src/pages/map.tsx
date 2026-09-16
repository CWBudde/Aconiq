import { useCallback, useEffect, useRef, useState } from "react";
import { ShieldAlert, X } from "lucide-react";
import type { MapGeoJSONFeature, MapMouseEvent } from "maplibre-gl";
import { Link, useSearchParams } from "react-router";
import { TooltipProvider } from "@/ui/components/tooltip";
import { Badge } from "@/ui/components/badge";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { PageHeader } from "@/ui/page-header";
import { MapView } from "@/map/map-view";
import { MapPanel } from "@/map/map-panel";
import { LayerControl } from "@/map/layer-control";
import { CoordinateDisplay } from "@/map/coordinate-display";
import { FeaturePopup } from "@/map/feature-popup";
import { DrawToolbar } from "@/map/draw-toolbar";
import { FeatureEditor } from "@/map/feature-editor";
import { NewFeatureDialog } from "@/map/new-feature-dialog";
import { ValidationPanel } from "@/map/validation-panel";
import { UndoRedoBar } from "@/map/undo-redo-bar";
import { ModelLayers } from "@/map/model-layers";
import { fitViewToWorkspace } from "@/map/extent";
import { DISPLAY_CRS } from "@/map/display-model";
import { DrawProvider } from "@/map/draw-provider";
import { DRAW_PARAM, SELECT_PARAM } from "@/map/map-params";
import { useDrawContext } from "@/map/use-draw-context";
import type { CalcArea, Geometry, Position } from "@/model/types";
import type { DrawMode } from "@/map/use-draw";
import { useModelStore } from "@/model/model-store";
import { useModelValidation } from "@/model/use-model-validation";
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

function WorkspaceStart({ onDismiss }: { onDismiss: () => void }) {
  const startDrawing = useStartDrawing(onDismiss);

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
            <Button variant="outline" onClick={startDrawing}>
              {m.action_start_drawing()}
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
}

function MapWorkspace() {
  const [clickedFeature, setClickedFeature] =
    useState<MapGeoJSONFeature | null>(null);
  const [popupLngLat, setPopupLngLat] = useState<[number, number] | null>(null);
  const [editingFeatureId, setEditingFeatureId] = useState<string | null>(null);
  const [newGeometry, setNewGeometry] = useState<Geometry | null>(null);
  const [showValidation, setShowValidation] = useState(false);
  // Local, deliberately: the overlay disappears on its own as soon as the
  // first feature lands, and it comes back when the model empties again or
  // the route is left. Persisting the dismissal would hide the only pointer
  // to "Start drawing" from the one user who needs it.
  const [startDismissed, setStartDismissed] = useState(false);
  const setCalcArea = useModelStore((s) => s.setCalcArea);
  const clearCalcArea = useModelStore((s) => s.clearCalcArea);
  const calcArea = useModelStore((s) => s.calcArea);
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);
  const crs = useModelStore((s) => s.crs);

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
  // the one thing the map must never be. The draw-finish callback is the only
  // coordinate writer on the map side (`FeatureEditor` writes attributes only,
  // and nothing ever calls `draw.addFeatures`), so refusing here closes the
  // hole outright.
  //
  // The real fix is an inverse 4326 → `store.crs` transform on this path. It is
  // unavailable in API mode until a transform endpoint exists, and is recorded
  // in PLAN.md Priority 8 Phase D.
  const drawingDisabled = crs !== DISPLAY_CRS;

  const handleDrawFinish = useCallback(
    (mode: DrawMode, feature: GeoJSON.Feature) => {
      if (drawingDisabled) return;
      if (mode === "calc-area") {
        const geom = feature.geometry;
        if (geom.type === "Polygon") {
          const area: CalcArea = {
            geometry: {
              type: "Polygon",
              coordinates: geom.coordinates as Position[][],
            },
          };
          setCalcArea(area);
        }
        return;
      }
      setNewGeometry(feature.geometry as Geometry);
    },
    [setCalcArea, drawingDisabled],
  );

  const handleFeatureClick = useCallback(
    (features: MapGeoJSONFeature[], e: MapMouseEvent) => {
      const feature = features[0];
      if (feature) {
        setClickedFeature(feature);
        setPopupLngLat([e.lngLat.lng, e.lngLat.lat]);
        const props = feature.properties as Record<string, unknown>;
        const featureId = props["id"] ?? feature.id;
        if (featureId != null) {
          setEditingFeatureId(
            typeof featureId === "string"
              ? featureId
              : String(featureId as number),
          );
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
      {/* The map canvas carries no visible heading; the page outline still
          needs one under the shell's h1, and the E2E helper waits for it. */}
      <h2 className="sr-only">{m.nav_model()}</h2>
      <MapView
        center={workspaceView.center}
        zoom={workspaceView.zoom}
        onFeatureClick={handleFeatureClick}
      >
        <DrawProvider onFinish={handleDrawFinish}>
          <ModelLayers />
          <WorkspaceDrawToolbar
            disabled={drawingDisabled}
            disabledReason={m.msg_draw_disabled_crs({ crs })}
          />
          <LayerControl />
          <CoordinateDisplay />
          <FeaturePopup feature={clickedFeature} lngLat={popupLngLat} />
          <FeatureEditor
            featureId={editingFeatureId}
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
              <ValidationPanel onSelectFeature={handleSelectFromValidation} />
            </MapPanel>
          ) : null}
          <DrawRequest
            onDismiss={() => {
              setStartDismissed(true);
            }}
          />
          <SelectRequest onSelect={handleSelectFromValidation} />
          {showStart ? (
            <WorkspaceStart
              onDismiss={() => {
                setStartDismissed(true);
              }}
            />
          ) : null}
        </DrawProvider>
      </MapView>
      <NewFeatureDialog
        open={newGeometry !== null}
        geometry={newGeometry}
        onClose={() => {
          setNewGeometry(null);
        }}
      />
    </TooltipProvider>
  );
}

/**
 * Honours `?draw=1`, the flag the project page's "Start drawing" sets, then
 * strips it so a reload or a Back does not arm the tool again. A boolean
 * rather than a mode name, so which mode drawing starts in stays a decision
 * this file makes once.
 */
function DrawRequest({ onDismiss }: { onDismiss: () => void }) {
  const [params, setParams] = useSearchParams();
  const startDrawing = useStartDrawing(onDismiss);
  const requested = params.get(DRAW_PARAM) === "1";
  const handled = useRef(false);

  useEffect(() => {
    if (!requested || handled.current) return;
    handled.current = true;
    startDrawing();
    setParams({}, { replace: true });
  }, [requested, startDrawing, setParams]);

  return null;
}

/**
 * Honours `?select=<featureId>`, the link the import page's done step builds
 * for a finding, then strips it so a Back does not re-open the editor on a
 * feature the reader has moved on from.
 *
 * `onSelect` is `handleSelectFromValidation`, the move the validation panel
 * already makes: open the editor on that feature and close the panel. A second
 * one written here would be two ways to select a feature that must not
 * diverge.
 */
function SelectRequest({
  onSelect,
}: {
  onSelect: (featureId: string) => void;
}) {
  const [params, setParams] = useSearchParams();
  const requested = params.get(SELECT_PARAM) ?? "";
  const handled = useRef(false);

  useEffect(() => {
    if (requested === "" || handled.current) return;
    handled.current = true;
    onSelect(requested);
    setParams({}, { replace: true });
  }, [requested, onSelect, setParams]);

  return null;
}

/** Binds the presentational toolbar to the provider above it. */
function WorkspaceDrawToolbar({
  disabled,
  disabledReason,
}: {
  disabled: boolean;
  disabledReason: string;
}) {
  const { activeMode, setMode, cancel } = useDrawContext();
  return (
    <DrawToolbar
      activeMode={activeMode}
      onModeChange={setMode}
      onCancel={cancel}
      disabled={disabled}
      disabledReason={disabledReason}
    />
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
