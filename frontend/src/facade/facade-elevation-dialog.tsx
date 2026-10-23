import { useEffect, useId, useState } from "react";
import { useShallow } from "zustand/shallow";
import { backend } from "@/api/backend";
import { m } from "@/i18n/messages";
import type { Workspace } from "@/model/compute-crs";
import { useModelStore } from "@/model/model-store";
import { Badge } from "@/ui/components/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/ui/components/dialog";
import { Label } from "@/ui/components/label";
import { formatNumber, MISSING_VALUE } from "@/ui/format";
import type { ElevationWindow, FacadeElevationResponse } from "@/wasm/types";
import {
  facadeOptionLabel,
  reviewLabel,
  slabSourceLabel,
} from "./elevation-labels";
import { ElevationSvg } from "./elevation-svg";
import { loadFacadeElevation } from "./load-elevation";

export interface FacadeElevationDialogProps {
  buildingId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

type ElevationState =
  | { status: "loading" }
  | { status: "failed"; error: string }
  | { status: "ready"; elevation: FacadeElevationResponse };

/**
 * A building's facades unfolded, one at a time (P18 C2): pick a facade, see
 * its storeys and the windows keyed to it, and the building's windows whose
 * key no longer names one of its edges. Everything measured here is measured
 * by the Go code; this draws it.
 *
 * It follows the store: an edit anywhere in the workspace measures the
 * building again, so what it shows is what a facade run would read now.
 */
export default function FacadeElevationDialog({
  buildingId,
  open,
  onOpenChange,
}: FacadeElevationDialogProps) {
  const workspace = useModelStore(
    useShallow(
      (s): Workspace => ({
        features: s.features,
        receivers: s.receivers,
        calcArea: s.calcArea,
        crs: s.crs,
      }),
    ),
  );
  const [state, setState] = useState<ElevationState>({ status: "loading" });
  const [selected, setSelected] = useState(0);

  useEffect(() => {
    if (!open) return;
    let current = true;
    // A re-measure after an edit keeps the drawing in hand until the new one
    // arrives, rather than blanking the dialog on every change to the model.
    setState((prev) =>
      prev.status === "ready" ? prev : { status: "loading" },
    );
    loadFacadeElevation(backend, workspace, buildingId).then(
      ({ elevation }) => {
        if (current) setState({ status: "ready", elevation });
      },
      (err: unknown) => {
        if (current)
          setState({
            status: "failed",
            error: err instanceof Error ? err.message : String(err),
          });
      },
    );
    return () => {
      current = false;
    };
  }, [open, workspace, buildingId]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-4xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {m.title_facade_elevation({ building: buildingId })}
          </DialogTitle>
          <DialogDescription>
            {state.status === "ready"
              ? m.msg_facade_elevation_summary({
                  height: formatNumber(state.elevation.height_m),
                  storeys: state.elevation.slab_heights_m.length,
                  source: slabSourceLabel(state.elevation.slab_height_source),
                })
              : state.status === "loading"
                ? m.msg_facade_elevation_loading()
                : m.msg_facade_elevation_failed({ error: state.error })}
          </DialogDescription>
        </DialogHeader>
        {state.status === "ready" && (
          <ElevationBody
            elevation={state.elevation}
            selected={Math.min(selected, state.elevation.facades.length - 1)}
            onSelect={setSelected}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function ElevationBody({
  elevation,
  selected,
  onSelect,
}: {
  elevation: FacadeElevationResponse;
  selected: number;
  onSelect: (index: number) => void;
}) {
  const selectId = useId();
  const facade = elevation.facades[selected];
  if (facade === undefined) return null;

  return (
    <div className="grid gap-4">
      <div className="grid gap-1.5">
        <Label htmlFor={selectId} className="text-xs">
          {m.label_facade_select()}
        </Label>
        <select
          id={selectId}
          className="h-8 rounded-md border border-input bg-background px-2 text-xs"
          value={selected}
          onChange={(e) => {
            onSelect(Number(e.target.value));
          }}
        >
          {elevation.facades.map((f, i) => (
            <option
              key={`${String(f.part)}-${String(f.ring)}-${String(f.edge)}`}
              value={i}
            >
              {facadeOptionLabel(f, i + 1)}
            </option>
          ))}
        </select>
      </div>
      <ElevationSvg
        facade={facade}
        number={selected + 1}
        heightM={elevation.height_m}
        slabHeightsM={elevation.slab_heights_m}
      />
      <section className="grid gap-2">
        <h3 className="text-sm font-medium">{m.heading_facade_windows()}</h3>
        {facade.windows.length === 0 ? (
          <p className="text-xs text-muted-foreground">
            {m.msg_facade_no_windows()}
          </p>
        ) : (
          <WindowTable windows={facade.windows} />
        )}
      </section>
      {elevation.detached.length > 0 && (
        <section className="grid gap-2">
          <h3 className="text-sm font-medium">{m.heading_facade_detached()}</h3>
          <p className="text-xs text-muted-foreground">
            {m.msg_facade_detached_help()}
          </p>
          <WindowTable windows={elevation.detached} />
        </section>
      )}
    </div>
  );
}

function WindowTable({ windows }: { windows: ElevationWindow[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-xs">
        <thead className="text-muted-foreground">
          <tr>
            <th scope="col" className="py-1 pr-3 font-medium">
              {m.label_window_id()}
            </th>
            <th scope="col" className="py-1 pr-3 font-medium">
              {m.label_window_u()}
            </th>
            <th scope="col" className="py-1 pr-3 font-medium">
              {m.label_window_sill()}
            </th>
            <th scope="col" className="py-1 pr-3 font-medium">
              {m.label_window_width()}
            </th>
            <th scope="col" className="py-1 pr-3 font-medium">
              {m.label_window_height()}
            </th>
            <th scope="col" className="py-1 pr-3 font-medium">
              {m.label_window_sensitive()}
            </th>
            <th scope="col" className="py-1 pr-3 font-medium">
              {m.label_window_review()}
            </th>
            <th scope="col" className="py-1 font-medium">
              {m.label_window_findings()}
            </th>
          </tr>
        </thead>
        <tbody className="tabular-nums">
          {windows.map((w) => (
            <tr key={w.id} className="border-t align-top">
              <td className="py-1 pr-3 font-mono">{w.id}</td>
              <td className="py-1 pr-3">{formatNumber(w.window.u_m, 2)}</td>
              <td className="py-1 pr-3">
                {formatNumber(w.window.sill_z_m, 2)}
              </td>
              <td className="py-1 pr-3">{formatNumber(w.window.width_m, 2)}</td>
              <td className="py-1 pr-3">
                {formatNumber(w.window.height_m, 2)}
              </td>
              <td className="py-1 pr-3">
                {w.sensitive ? m.label_window_sensitive_yes() : MISSING_VALUE}
              </td>
              <td className="py-1 pr-3">
                <Badge
                  variant={
                    w.review_status === "confirmed"
                      ? "success"
                      : w.review_status === "rejected"
                        ? "outline"
                        : "warning"
                  }
                >
                  {reviewLabel(w.review_status)}
                </Badge>
              </td>
              <td className="py-1">
                {w.findings.length === 0
                  ? MISSING_VALUE
                  : w.findings.map((f, i) => (
                      <p key={`${f.code}-${String(i)}`}>
                        <span className="font-mono">{f.code}</span>: {f.message}
                      </p>
                    ))}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
