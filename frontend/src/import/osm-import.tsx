import { useCallback, useState } from "react";
import type { Dispatch, SetStateAction } from "react";
import { LocateFixed } from "lucide-react";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { Input } from "@/ui/components/input";
import { Label } from "@/ui/components/label";
import { PageHeader } from "@/ui/page-header";
import { BBoxFields } from "./bbox-fields";
import { parseBBox } from "./bbox";
import type { BBoxText } from "./bbox";
import { useImportFromOSM } from "@/api/hooks";
import { asAPIRequestError } from "@/api/api-error";
import type { GeoJSONFeatureCollection } from "@/model/types";
import { m } from "@/i18n/messages";

/**
 * What the reader is told when the Overpass fetch fails.
 *
 * The hint is the load-bearing half and it used to be dropped. The API
 * distinguishes a rate limit, a refusal and an outage — they arrive as one
 * `upstream_error` with the upstream status in `details` and the remedy in
 * `hint` — and rendering `err.message` alone put the reader back where the
 * envelope change was meant to move them from: "failed with status 429" says
 * nothing about whether waiting will help.
 */
export function osmFailureText(err: unknown): string {
  const apiError = asAPIRequestError(err);
  if (apiError?.hint) {
    return `${apiError.message} — ${apiError.hint}`;
  }

  return err instanceof Error ? err.message : m.error_osm_fetch_failed();
}

/**
 * The Overpass query the reader is composing: the shared box, plus the
 * endpoint only this tab has.
 *
 * `toFixed` below writes an input value, not a display string, so it stays
 * outside the locale-aware formatters.
 */
export interface OsmQuery extends BBoxText {
  endpoint: string;
}

/**
 * The Overpass half of the import wizard: a bounding box, the optional
 * endpoint, and the fetch.
 *
 * Like {@link FileImport} it hands a `FeatureCollection` up and reads nothing
 * into it.
 *
 * The query itself is the page's state, not this component's, because the tab
 * strip unmounts an inactive panel: state held here would be discarded the
 * moment the reader looked at the file tab, and "Use current location" would
 * have to be answered a second time. One object with one setter, so the panel
 * takes two props for it rather than ten.
 *
 * `onQueryChange` is a `SetStateAction` dispatcher and every write goes through
 * its updater form, never through a spread of the `query` this render closed
 * over. Only the geolocation button is disabled while the browser asks for
 * permission — the endpoint and the four box fields stay editable — so a
 * success callback that spread its captured snapshot would undo whatever was
 * typed in the meantime, including an endpoint the next fetch then would not
 * use.
 */
export function OsmImport({
  query,
  onQueryChange,
  onCollection,
  onError,
}: {
  query: OsmQuery;
  onQueryChange: Dispatch<SetStateAction<OsmQuery>>;
  onCollection: (collection: GeoJSONFeatureCollection) => void;
  onError: (message: string | null) => void;
}) {
  const [geolocating, setGeolocating] = useState(false);

  const osmMutation = useImportFromOSM();

  const setField = useCallback(
    (field: keyof OsmQuery) => (value: string) => {
      onQueryChange((current) => ({ ...current, [field]: value }));
    },
    [onQueryChange],
  );

  const handleUseCurrentLocation = useCallback(() => {
    if (typeof navigator.geolocation.getCurrentPosition !== "function") {
      onError(m.error_geolocation_not_supported());
      return;
    }
    setGeolocating(true);
    onError(null);
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        const lat = pos.coords.latitude;
        const lon = pos.coords.longitude;
        const delta = 0.005; // ~500 m radius
        onQueryChange((current) => ({
          ...current,
          south: (lat - delta).toFixed(6),
          north: (lat + delta).toFixed(6),
          west: (lon - delta).toFixed(6),
          east: (lon + delta).toFixed(6),
        }));
        setGeolocating(false);
      },
      (err) => {
        onError(m.error_location_fetch_failed() + `: ${err.message}`);
        setGeolocating(false);
      },
    );
  }, [onQueryChange, onError]);

  const handleOSMFetch = useCallback(() => {
    onError(null);
    const bbox = parseBBox(query);
    if (bbox === null) {
      onError(m.msg_bbox_required());
      return;
    }

    osmMutation.mutate(
      {
        ...bbox,
        ...(query.endpoint ? { overpass_endpoint: query.endpoint } : {}),
      },
      {
        onSuccess: (collection) => {
          onCollection(collection);
        },
        onError: (err: unknown) => {
          onError(osmFailureText(err));
        },
      },
    );
  }, [query, osmMutation, onCollection, onError]);

  return (
    <Card className="flex flex-col gap-4 p-6">
      <PageHeader
        title={m.heading_import_from_osm()}
        description={m.msg_import_osm_description()}
      />
      <Button
        variant="outline"
        size="sm"
        onClick={handleUseCurrentLocation}
        disabled={geolocating}
        className="self-start"
      >
        <LocateFixed aria-hidden="true" />
        {geolocating ? m.status_locating() : m.action_use_current_location()}
      </Button>
      <BBoxFields
        idPrefix="osm"
        value={query}
        onFieldChange={(field, value) => {
          setField(field)(value);
        }}
      />
      <div className="flex flex-col gap-1">
        <Label htmlFor="osm-endpoint" className="text-xs text-muted-foreground">
          {m.label_overpass_endpoint_optional()}
        </Label>
        <Input
          id="osm-endpoint"
          type="text"
          value={query.endpoint}
          onChange={(e) => {
            setField("endpoint")(e.target.value);
          }}
          placeholder="https://overpass-api.de/api/interpreter"
        />
      </div>
      <Button onClick={handleOSMFetch} disabled={osmMutation.isPending}>
        {osmMutation.isPending
          ? m.status_fetching()
          : m.action_fetch_from_osm()}
      </Button>
    </Card>
  );
}
