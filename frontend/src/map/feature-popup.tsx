import { useEffect, useRef } from "react";
import maplibregl from "maplibre-gl";
import type { MapGeoJSONFeature, MapMouseEvent } from "maplibre-gl";
import { useMap } from "./use-map";

interface FeaturePopupProps {
  feature: MapGeoJSONFeature | null;
  lngLat: [number, number] | null;
}

function formatProperties(
  properties: Record<string, unknown>,
): [string, string][] {
  return Object.entries(properties)
    .filter(([, v]) => v != null && v !== "")
    .map(([k, v]) => [k, String(v)]);
}

export function FeaturePopup({ feature, lngLat }: FeaturePopupProps) {
  const map = useMap();
  const popupRef = useRef<maplibregl.Popup | null>(null);

  useEffect(() => {
    if (!map) return;

    // Clean up previous popup
    popupRef.current?.remove();
    popupRef.current = null;

    if (!feature || !lngLat) return;

    const props = formatProperties(
      (feature.properties ?? {}) as Record<string, unknown>,
    );
    if (props.length === 0) return;

    const html = `
      <div style="font-family: var(--font-sans); font-size: 12px; max-width: 240px;">
        <div style="font-weight: 600; margin-bottom: 4px; color: var(--foreground);">
          ${feature.layer.id}
        </div>
        <table style="border-collapse: collapse; width: 100%;">
          ${props
            .map(
              ([k, v]) => `
            <tr>
              <td style="padding: 1px 8px 1px 0; color: var(--muted-foreground); white-space: nowrap;">${k}</td>
              <td style="padding: 1px 0; font-family: var(--font-mono);">${v}</td>
            </tr>
          `,
            )
            .join("")}
        </table>
      </div>
    `;

    const popup = new maplibregl.Popup({ closeButton: true, maxWidth: "280px" })
      .setLngLat(lngLat)
      .setHTML(html)
      .addTo(map);

    popupRef.current = popup;

    return () => {
      popup.remove();
    };
  }, [map, feature, lngLat]);

  return null;
}

/** Helper to extract lngLat from a MapMouseEvent for the popup */
export function eventToLngLat(e: MapMouseEvent): [number, number] {
  return [e.lngLat.lng, e.lngLat.lat];
}
