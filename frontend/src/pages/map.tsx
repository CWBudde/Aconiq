import { useCallback, useState } from "react";
import { X } from "lucide-react";
import type { MapGeoJSONFeature, MapMouseEvent } from "maplibre-gl";
import { TooltipProvider } from "@/ui/components/tooltip";
import { Button } from "@/ui/components/button";
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
import { useDraw } from "@/map/use-draw";
import type { CalcArea, Geometry, Position } from "@/model/types";
import type { DrawMode } from "@/map/use-draw";
import { useModelStore } from "@/model/model-store";
import { m } from "@/i18n/messages";

export default function MapPage() {
  const [clickedFeature, setClickedFeature] =
    useState<MapGeoJSONFeature | null>(null);
  const [popupLngLat, setPopupLngLat] = useState<[number, number] | null>(null);
  const [editingFeatureId, setEditingFeatureId] = useState<string | null>(null);
  const [newGeometry, setNewGeometry] = useState<Geometry | null>(null);
  const [showValidation, setShowValidation] = useState(false);
  const setCalcArea = useModelStore((s) => s.setCalcArea);
  const clearCalcArea = useModelStore((s) => s.clearCalcArea);
  const calcArea = useModelStore((s) => s.calcArea);

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
      <MapView onFeatureClick={handleFeatureClick}>
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
