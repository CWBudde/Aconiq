import { useCallback, useState } from "react";
import { ShieldAlert, X } from "lucide-react";
import type { MapGeoJSONFeature, MapMouseEvent } from "maplibre-gl";
import { Link } from "react-router";
import { TooltipProvider } from "@/ui/components/tooltip";
import { Badge } from "@/ui/components/badge";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { Callout } from "@/ui/callout";
import { KeyValueList } from "@/ui/key-value-list";
import { PageHeader, SectionHeading } from "@/ui/page-header";
import { useProjectStatus } from "@/api/hooks";
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
import { useDraw } from "@/map/use-draw";
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

function ProjectSummary() {
  const project = useProjectStatus();

  if (project.isLoading) {
    return (
      <p className="text-sm text-muted-foreground">
        {m.status_loading_project()}
      </p>
    );
  }
  if (project.isError) {
    return <Callout variant="destructive">{project.error.message}</Callout>;
  }
  if (!project.data) {
    return (
      <Callout variant="neutral" title={m.msg_no_project_yet()}>
        {m.msg_no_project_yet_help()}
      </Callout>
    );
  }
  return (
    <KeyValueList
      items={[
        { label: m.label_name_field(), value: project.data.name },
        { label: m.label_crs_field(), value: project.data.crs, mono: true },
        {
          label: m.label_scenarios_field(),
          value: String(project.data.scenario_count),
        },
        { label: m.label_runs_field(), value: String(project.data.run_count) },
      ]}
    />
  );
}

function WorkspaceStart() {
  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <div className="grid w-full max-w-5xl gap-6 lg:grid-cols-[minmax(0,1.3fr)_minmax(18rem,0.7fr)]">
        <Card className="p-8">
          <div className="max-w-2xl space-y-4">
            <Badge variant="outline">{m.section_workspace()}</Badge>
            <PageHeader
              title={m.heading_map_workspace()}
              description={m.msg_map_workspace_description()}
            />
          </div>

          <div className="mt-8 flex flex-wrap gap-3">
            <Button asChild>
              <Link to="/import">{m.nav_import()}</Link>
            </Button>
            <Button asChild variant="outline">
              <Link to="/status">{m.nav_status()}</Link>
            </Button>
          </div>
        </Card>

        <Card className="space-y-3 p-6">
          <SectionHeading variant="eyebrow">
            {m.section_project()}
          </SectionHeading>
          <ProjectSummary />
        </Card>
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

  // The initial viewport is computed once, on mount. `useMemo` recomputed it on
  // every model edit, and MapView treats a new `center`/`zoom` as a reason to
  // rebuild — so drawing a single feature discarded the map. Later fitting is
  // ModelLayers' job (it calls `fitBounds` when data first arrives).
  const [workspaceView] = useState(() =>
    fitViewToWorkspace(features, receivers, calcArea, [10.45, 51.16]),
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
      {/* The map canvas carries no visible heading; the page outline still
          needs one under the shell's h1, and the E2E helper waits for it. */}
      <h2 className="sr-only">{m.nav_model()}</h2>
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
