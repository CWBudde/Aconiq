import { Eye, EyeOff } from "lucide-react";
import { Button } from "@/ui/components/button";
import {
  MODEL_LAYER_GROUPS,
  RESULT_LAYER_GROUPS,
  type LayerGroup,
} from "./layers";
import { MapPanel } from "./map-panel";
import { useMapStore } from "./map-store";
import { useMap } from "./use-map";
import { BASEMAP_IDS, basemapLabel } from "./basemap";
import { m } from "@/i18n/messages";

function LayerToggle({ group }: { group: LayerGroup }) {
  const map = useMap();
  const visibility = useMapStore((s) => s.layerVisibility);
  const toggleLayer = useMapStore((s) => s.toggleLayer);

  const visible = visibility[group.id] ?? group.defaultVisible;

  const handleToggle = () => {
    toggleLayer(group.id);
    if (!map) return;
    const newVisibility = !visible ? "visible" : "none";
    for (const layerId of group.layerIds) {
      try {
        map.setLayoutProperty(layerId, "visibility", newVisibility);
      } catch {
        // Layer may not exist yet
      }
    }
  };

  return (
    <Button
      variant="ghost"
      size="sm"
      className="h-7 justify-start gap-2 px-2 text-xs"
      onClick={handleToggle}
      aria-label={
        visible
          ? m.action_hide_layer({ label: group.label() })
          : m.action_show_layer({ label: group.label() })
      }
    >
      {visible ? (
        <Eye className="size-3.5" aria-hidden="true" />
      ) : (
        <EyeOff className="size-3.5 text-muted-foreground" aria-hidden="true" />
      )}
      <span className={visible ? "" : "text-muted-foreground"}>
        {group.label()}
      </span>
    </Button>
  );
}

/**
 * Which basemap the map is drawn on.
 *
 * `aria-pressed` on three ordinary buttons rather than a select: there are
 * three of them, they are mutually exclusive, and the choice is visible on the
 * canvas the instant it is made. Nothing here is ever refused, so nothing needs
 * `aria-disabled` — the current basemap's button stays pressable, and pressing
 * it again is a no-op the store absorbs.
 *
 * Switching rebuilds the map (`map-view.tsx` keys its init effect on it), which
 * is also what restores the model layers; the viewport is carried across.
 */
function BasemapPicker() {
  const basemap = useMapStore((s) => s.basemap);
  const setBasemap = useMapStore((s) => s.setBasemap);

  return (
    <div
      role="group"
      aria-label={m.section_basemap()}
      className="grid grid-cols-3 gap-0.5"
    >
      {BASEMAP_IDS.map((id) => {
        const active = basemap === id;
        return (
          <Button
            key={id}
            variant={active ? "secondary" : "ghost"}
            size="sm"
            className="h-7 px-2 text-xs"
            aria-pressed={active}
            onClick={() => {
              setBasemap(id);
            }}
          >
            {basemapLabel(id)}
          </Button>
        );
      })}
    </div>
  );
}

// Sits beside MapLibre's navigation control, which owns the top-right corner.
export function LayerControl() {
  return (
    <MapPanel
      position="top-right"
      inset="right-12 top-2"
      translucent
      role="group"
      aria-label={m.label_layers()}
    >
      <div className="mb-1 text-xs font-medium text-muted-foreground">
        {m.section_model()}
      </div>
      <div className="grid gap-0.5">
        {MODEL_LAYER_GROUPS.map((g) => (
          <LayerToggle key={g.id} group={g} />
        ))}
      </div>
      <div className="mb-1 mt-2 text-xs font-medium text-muted-foreground">
        {m.section_results()}
      </div>
      <div className="grid gap-0.5">
        {RESULT_LAYER_GROUPS.map((g) => (
          <LayerToggle key={g.id} group={g} />
        ))}
      </div>
      <div className="mb-1 mt-2 text-xs font-medium text-muted-foreground">
        {m.section_basemap()}
      </div>
      <BasemapPicker />
    </MapPanel>
  );
}
