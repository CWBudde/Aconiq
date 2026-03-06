import { useCallback, useState } from "react";
import type { MapGeoJSONFeature, MapMouseEvent } from "maplibre-gl";
import { MapView } from "@/map/map-view";
import { LayerControl } from "@/map/layer-control";
import { CoordinateDisplay } from "@/map/coordinate-display";
import { FeaturePopup } from "@/map/feature-popup";

export default function MapPage() {
  const [clickedFeature, setClickedFeature] =
    useState<MapGeoJSONFeature | null>(null);
  const [popupLngLat, setPopupLngLat] = useState<[number, number] | null>(
    null,
  );

  const handleFeatureClick = useCallback(
    (features: MapGeoJSONFeature[], e: MapMouseEvent) => {
      const feature = features[0];
      if (feature) {
        setClickedFeature(feature);
        setPopupLngLat([e.lngLat.lng, e.lngLat.lat]);
      }
    },
    [],
  );

  return (
    <MapView onFeatureClick={handleFeatureClick}>
      <LayerControl />
      <CoordinateDisplay />
      <FeaturePopup feature={clickedFeature} lngLat={popupLngLat} />
    </MapView>
  );
}
