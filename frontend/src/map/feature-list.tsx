import { Button } from "@/ui/components/button";
import { cn } from "@/ui/lib/utils";
import { useModelStore } from "@/model/model-store";
import type { FeatureKind } from "@/model/types";
import { MapPanel } from "./map-panel";
import { m } from "@/i18n/messages";

interface FeatureListProps {
  /** The feature the page is editing, so the list can mark its own entry. */
  selectedId: string | null;
  /**
   * Sets the page's `editingFeatureId` — the same move a click on the canvas
   * makes, which is the point: selecting from here marks the feature on the
   * map through `ModelLayers` and opens the docked editor, with no second
   * selection path to keep in step.
   */
  onSelect: (id: string) => void;
}

interface Entry {
  id: string;
  kind: FeatureKind | "receiver";
}

// Message functions resolve against the *current* locale, so they are called
// during render rather than held in a module-scope table — the mistake
// `feature-editor.tsx` documents, which froze its labels to whatever locale
// was active at import time.
function kindLabel(kind: FeatureKind | "receiver"): string {
  switch (kind) {
    case "source":
      return m.option_source();
    case "building":
      return m.option_building();
    case "barrier":
      return m.option_barrier();
    case "receiver":
      return m.option_receiver();
  }
}

/**
 * Everything in the model, as a list of Tab-reachable controls.
 *
 * The map's own selection is a click on a canvas: `MapView`'s `onFeatureClick`
 * hit-tests a pixel, which a keyboard cannot produce and a screen reader cannot
 * see. This list is the same selection by another route — it sets the page's
 * `editingFeatureId`, so the feature is marked on the canvas and the docked
 * editor opens exactly as a click does.
 *
 * It is a projection of the model and writes nothing: the entries are read from
 * the store and the only thing leaving is an id.
 *
 * On the inset: the four corners are taken on this route — the draw toolbar and
 * the docked editor top-left, the layer control top-right (offset to clear
 * MapLibre's navigation control), the undo bar and the calculation-area badge
 * bottom-right, the validation toggle and panel bottom-left, the coordinate
 * readout bottom-centre. This panel takes the space left of the layer control,
 * which is the only free anchor that does not stack on top of a panel a user
 * may have open at the same time.
 */
export function FeatureList({ selectedId, onSelect }: FeatureListProps) {
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);

  const entries: Entry[] = [
    ...features.map((feature) => ({ id: feature.id, kind: feature.kind })),
    ...receivers.map((receiver) => ({
      id: receiver.id,
      kind: "receiver" as const,
    })),
  ];

  return (
    <MapPanel
      position="top-right"
      inset="right-56 top-14"
      width="w-72"
      className="p-0"
      role="region"
      aria-label={m.label_feature_list()}
    >
      {entries.length === 0 ? (
        <p className="p-3 text-center text-xs text-muted-foreground">
          {m.msg_feature_list_empty()}
        </p>
      ) : (
        <ul className="max-h-64 divide-y overflow-y-auto">
          {entries.map((entry) => (
            <li key={entry.id}>
              <Button
                variant="ghost"
                size="sm"
                className={cn(
                  "h-auto w-full justify-start gap-2 rounded-none px-3 py-2 text-left text-xs",
                  selectedId === entry.id && "bg-accent",
                )}
                // `aria-current`, not `aria-pressed`: the entry is not a toggle
                // — activating the selected one again leaves it selected — and
                // the list has exactly one current item.
                aria-current={selectedId === entry.id ? true : undefined}
                onClick={() => {
                  onSelect(entry.id);
                }}
              >
                <span className="shrink-0">{kindLabel(entry.kind)}</span>
                {/* The id is what the validation panel, the editor and the
                    `?select=` link all name a feature by, so it is what the
                    entry has to show. Truncated rather than wrapped: they are
                    UUIDs, and a wrapped one costs three lines per entry. */}
                <span className="truncate font-mono text-2xs text-muted-foreground">
                  {entry.id}
                </span>
              </Button>
            </li>
          ))}
        </ul>
      )}
    </MapPanel>
  );
}
