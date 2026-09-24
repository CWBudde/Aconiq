import { useCallback, useMemo } from "react";
import type { Dispatch, SetStateAction } from "react";
import { Maximize } from "lucide-react";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { Callout } from "@/ui/callout";
import { PageHeader } from "@/ui/page-header";
import { backend } from "@/api/backend";
import { useImportFromLGLN } from "@/api/hooks";
import type { LglnImportResponse } from "@/api/client";
import { isLonLatCRS } from "@/model/footprint";
import type { LonLatBBox } from "@/model/footprint";
import { getFeatureBBox, resolveGridExtent } from "@/model/grid-estimate";
import { useModelStore } from "@/model/model-store";
import type { CalcArea, ModelFeature } from "@/model/types";
import { BBoxFields } from "./bbox-fields";
import { parseBBox } from "./bbox";
import type { BBoxText } from "./bbox";
import { LGLN_MAX_TILES, estimateLglnTiles, lglnFailureText } from "./lgln";
import { m } from "@/i18n/messages";

/**
 * The workspace's extent as a box: the calculation area when there is one,
 * and every feature otherwise. `null` for an empty workspace.
 */
function modelExtent(
  features: ModelFeature[],
  calcArea: CalcArea | null,
): LonLatBBox | null {
  const extent =
    (calcArea === null
      ? null
      : resolveGridExtent({ features: [], calcArea })) ??
    getFeatureBBox(features);
  if (extent === null) return null;
  return {
    south: extent.minY,
    west: extent.minX,
    north: extent.maxY,
    east: extent.maxX,
  };
}

/**
 * The LGLN half of the import wizard: the official LoD2 buildings of Lower
 * Saxony for a box, loaded by `aconiq serve`.
 *
 * The box is the page's, shared with the OSM tab — the whole point is to
 * replace the buildings an OSM fetch of the same box brought. Generic over the
 * page's state type so the OSM query, which carries an endpoint besides, can
 * be handed in with its own setter.
 *
 * In browser mode the tab stays visible and says what it needs, rather than
 * disappearing: a reader who was told about it should find it, and find out
 * why it does not work here.
 */
export function LglnImport<T extends BBoxText>({
  bbox,
  onBBoxChange,
  onCollection,
  onError,
}: {
  bbox: T;
  onBBoxChange: Dispatch<SetStateAction<T>>;
  onCollection: (collection: LglnImportResponse, bbox: LonLatBBox) => void;
  onError: (message: string | null) => void;
}) {
  const available = backend.capabilities.canImportLGLN;
  const mutation = useImportFromLGLN();
  const features = useModelStore((s) => s.features);
  const calcArea = useModelStore((s) => s.calcArea);
  const crs = useModelStore((s) => s.crs);

  const parsed = parseBBox(bbox);
  const tiles = parsed === null ? null : estimateLglnTiles(parsed);

  const extent = useMemo(
    () => modelExtent(features, calcArea),
    [features, calcArea],
  );
  const extentUsable = extent !== null && isLonLatCRS(crs);

  const setField = useCallback(
    (field: keyof BBoxText, value: string) => {
      onBBoxChange((current) => ({ ...current, [field]: value }));
    },
    [onBBoxChange],
  );

  const handleUseModelExtent = useCallback(() => {
    if (extent === null) return;
    // Six decimals is ~0.1 m, which is finer than any building edge.
    onBBoxChange((current) => ({
      ...current,
      south: extent.south.toFixed(6),
      west: extent.west.toFixed(6),
      north: extent.north.toFixed(6),
      east: extent.east.toFixed(6),
    }));
  }, [extent, onBBoxChange]);

  const handleLoad = useCallback(() => {
    onError(null);
    const box = parseBBox(bbox);
    if (box === null) {
      onError(m.msg_bbox_required());
      return;
    }
    mutation.mutate(box, {
      onSuccess: (collection) => {
        onCollection(collection, box);
      },
      onError: (err: unknown) => {
        onError(lglnFailureText(err));
      },
    });
  }, [bbox, mutation, onCollection, onError]);

  const pending = mutation.isPending;

  return (
    <Card className="flex flex-col gap-4 p-6">
      <PageHeader
        title={m.heading_import_from_lgln()}
        description={m.msg_import_lgln_description()}
      />
      {available ? null : (
        <Callout variant="info">{m.msg_lgln_needs_server()}</Callout>
      )}
      {extent === null ? null : (
        <div className="flex flex-col gap-1">
          <Button
            variant="outline"
            size="sm"
            onClick={handleUseModelExtent}
            disabled={!available || !extentUsable || pending}
            className="self-start"
          >
            <Maximize aria-hidden="true" />
            {m.action_use_model_extent()}
          </Button>
          {extentUsable ? null : (
            <p className="text-xs text-muted-foreground">
              {m.msg_lgln_model_extent_unavailable()}
            </p>
          )}
        </div>
      )}
      <BBoxFields
        idPrefix="lgln"
        value={bbox}
        disabled={!available}
        onFieldChange={setField}
      />
      <p className="text-xs text-muted-foreground">
        {m.msg_lgln_bbox_shared()}
      </p>
      {tiles === null ? null : tiles > LGLN_MAX_TILES ? (
        <Callout variant="warning">
          {m.msg_lgln_tile_estimate({ count: tiles })}{" "}
          {m.msg_lgln_tile_estimate_too_many()}
        </Callout>
      ) : (
        <p className="text-sm text-muted-foreground">
          {m.msg_lgln_tile_estimate({ count: tiles })}
        </p>
      )}
      <Button onClick={handleLoad} disabled={!available || pending}>
        {pending ? m.status_lgln_loading() : m.action_fetch_from_lgln()}
      </Button>
      <p className="text-xs text-muted-foreground">
        {m.msg_lgln_first_load_slow()}
      </p>
    </Card>
  );
}
