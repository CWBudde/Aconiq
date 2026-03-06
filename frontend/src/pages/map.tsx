import { useCallback, useState } from "react";
import type { MapGeoJSONFeature, MapMouseEvent } from "maplibre-gl";
import { TooltipProvider } from "@/ui/components/tooltip";
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
import type { Geometry } from "@/model/types";
import type { DrawMode } from "@/map/use-draw";

export default function MapPage() {
  const [clickedFeature, setClickedFeature] =
    useState<MapGeoJSONFeature | null>(null);
  const [popupLngLat, setPopupLngLat] = useState<[number, number] | null>(null);
  const [editingFeatureId, setEditingFeatureId] = useState<string | null>(null);
  const [newGeometry, setNewGeometry] = useState<Geometry | null>(null);
  const [showValidation, setShowValidation] = useState(false);

  const handleDrawFinish = useCallback(
    (_mode: DrawMode, feature: GeoJSON.Feature) => {
      setNewGeometry(feature.geometry as Geometry);
    },
    [],
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
