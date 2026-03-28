import { useCallback, useMemo, useState } from "react";
import { X } from "lucide-react";
import type { MapGeoJSONFeature, MapMouseEvent } from "maplibre-gl";
import { Link } from "react-router";
import { TooltipProvider } from "@/ui/components/tooltip";
import { Button } from "@/ui/components/button";
import { useProjectStatus } from "@/api";
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
import { fitViewToWorkspace } from "@/map/extent";
import { useDraw } from "@/map/use-draw";
import type { CalcArea, Geometry, Position } from "@/model/types";
import type { DrawMode } from "@/map/use-draw";
import { useModelStore } from "@/model/model-store";
import { m } from "@/i18n/messages";

export default function MapPage() {
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);
  const calcArea = useModelStore((s) => s.calcArea);
  const hasWorkspaceContent =
    features.length > 0 || receivers.length > 0 || calcArea !== null;

  if (!hasWorkspaceContent) {
    return <WorkspaceStart />;
  }

  return <MapWorkspace />;
}

function WorkspaceStart() {
  const project = useProjectStatus();

  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <div className="grid w-full max-w-5xl gap-6 lg:grid-cols-[minmax(0,1.3fr)_minmax(18rem,0.7fr)]">
        <section className="rounded-3xl border bg-card p-8 shadow-sm">
          <div className="max-w-2xl space-y-4">
            <div className="inline-flex items-center gap-2 rounded-full border bg-muted/50 px-3 py-1 text-xs font-medium text-muted-foreground">
              {m.section_workspace()}
            </div>
            <h2 className="text-3xl font-semibold tracking-tight">
              {m.heading_map_workspace()}
            </h2>
            <p className="text-sm leading-6 text-muted-foreground sm:text-base">
              {m.msg_map_workspace_description()}
            </p>
          </div>

          <div className="mt-8 flex flex-wrap gap-3">
            <Button asChild>
              <Link to="/import">{m.nav_import()}</Link>
            </Button>
            <Button asChild variant="outline">
              <Link to="/status">{m.nav_status()}</Link>
            </Button>
          </div>
        </section>

        <section className="rounded-3xl border bg-card p-6 shadow-sm">
          <div className="space-y-3">
            <h3 className="text-sm font-semibold uppercase tracking-[0.2em] text-muted-foreground">
              {m.section_project()}
            </h3>
            {project.isLoading ? (
              <p className="text-sm text-muted-foreground">
                {m.status_loading_project()}
              </p>
            ) : project.isError ? (
              <p className="text-sm text-destructive">
                {project.error?.message ?? "Unknown error"}
              </p>
            ) : project.data ? (
              <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 text-sm">
                <dt className="text-muted-foreground">
                  {m.label_name_field()}
                </dt>
                <dd>{project.data.name}</dd>
                <dt className="text-muted-foreground">{m.label_crs_field()}</dt>
                <dd className="font-mono">{project.data.crs}</dd>
                <dt className="text-muted-foreground">
                  {m.label_scenarios_field()}
                </dt>
                <dd>{String(project.data.scenario_count)}</dd>
                <dt className="text-muted-foreground">
                  {m.label_runs_field()}
                </dt>
                <dd>{String(project.data.run_count)}</dd>
              </dl>
            ) : (
              <div className="rounded-2xl border bg-muted/30 p-4">
                <p className="text-sm font-medium">{m.msg_no_project_yet()}</p>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">
                  {m.msg_no_project_yet_help()}
                </p>
              </div>
            )}
          </div>
        </section>
      </div>
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
  const setCalcArea = useModelStore((s) => s.setCalcArea);
  const clearCalcArea = useModelStore((s) => s.clearCalcArea);
  const calcArea = useModelStore((s) => s.calcArea);
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);

  const workspaceView = useMemo(
    () => fitViewToWorkspace(features, receivers, calcArea, [10.45, 51.16]),
    [features, receivers, calcArea],
  );

  const handleDrawFinish = useCallback(
    (mode: DrawMode, feature: GeoJSON.Feature) => {
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
    [setCalcArea],
  );

  const { activeMode, setMode, cancel } = useDraw({
    onFinish: handleDrawFinish,
  });

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
      <MapView
        center={workspaceView.center}
        zoom={workspaceView.zoom}
        onFeatureClick={handleFeatureClick}
      >
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
          onClose={() => {
            setEditingFeatureId(null);
          }}
        />
        <UndoRedoBar />
        {calcArea ? (
          <div className="absolute bottom-14 right-3 z-10 flex items-center gap-1.5 rounded-md border border-blue-300 bg-blue-50 px-2 py-1 text-xs text-blue-800 shadow-sm dark:border-blue-700 dark:bg-blue-950 dark:text-blue-200">
            <span>{m.label_calc_area()}</span>
            <Button
              variant="ghost"
              size="icon"
              className="h-4 w-4 text-blue-600 hover:bg-blue-100 dark:hover:bg-blue-900"
              onClick={clearCalcArea}
              aria-label={m.action_clear_calc_area()}
            >
              <X className="h-3 w-3" />
            </Button>
          </div>
        ) : null}
        {showValidation ? (
          <div className="absolute bottom-14 left-3 z-10 w-80 rounded-md border bg-background shadow-md">
            <ValidationPanel onSelectFeature={handleSelectFromValidation} />
          </div>
        ) : null}
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
