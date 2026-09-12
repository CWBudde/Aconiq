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
    </MapPanel>
  );
}
