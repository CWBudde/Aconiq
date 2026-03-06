import { Eye, EyeOff } from "lucide-react";
import { Button } from "@/ui/components/button";
import {
  MODEL_LAYER_GROUPS,
  RESULT_LAYER_GROUPS,
  type LayerGroup,
} from "./layers";
import { useMapStore } from "./map-store";
import { useMap } from "./use-map";

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
      aria-label={`${visible ? "Hide" : "Show"} ${group.label}`}
    >
      {visible ? (
        <Eye className="h-3.5 w-3.5" />
      ) : (
        <EyeOff className="h-3.5 w-3.5 text-muted-foreground" />
      )}
      <span className={visible ? "" : "text-muted-foreground"}>
        {group.label}
      </span>
    </Button>
  );
}

export function LayerControl() {
  return (
    <div className="absolute top-2 left-2 z-10 rounded-md border bg-background/90 p-2 shadow-sm backdrop-blur-sm">
      <div className="mb-1 text-xs font-medium text-muted-foreground">
        Model
      </div>
      <div className="grid gap-0.5">
        {MODEL_LAYER_GROUPS.map((g) => (
          <LayerToggle key={g.id} group={g} />
        ))}
      </div>
      <div className="mb-1 mt-2 text-xs font-medium text-muted-foreground">
        Results
      </div>
      <div className="grid gap-0.5">
        {RESULT_LAYER_GROUPS.map((g) => (
          <LayerToggle key={g.id} group={g} />
        ))}
      </div>
    </div>
  );
}
