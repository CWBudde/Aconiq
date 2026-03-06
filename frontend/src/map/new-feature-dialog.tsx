import { useCallback, useEffect, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/ui/components/dialog";
import { Button } from "@/ui/components/button";
import { Input } from "@/ui/components/input";
import { Label } from "@/ui/components/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/ui/components/select";
import type { FeatureKind, Geometry, SourceType } from "@/model/types";
import { createFeatureId } from "@/model/types";
import { useModelStore } from "@/model/model-store";

interface NewFeatureDialogProps {
  open: boolean;
  geometry: Geometry | null;
  onClose: () => void;
}

function inferKind(geomType: string): FeatureKind {
  if (geomType === "Point" || geomType === "MultiPoint") return "source";
  if (geomType === "LineString" || geomType === "MultiLineString")
    return "barrier";
  return "building";
}

function inferSourceType(geomType: string): SourceType {
  if (geomType === "Point" || geomType === "MultiPoint") return "point";
  if (geomType === "LineString" || geomType === "MultiLineString")
    return "line";
  return "area";
}

export function NewFeatureDialog({
  open,
  geometry,
  onClose,
}: NewFeatureDialogProps) {
  const addFeature = useModelStore((s) => s.addFeature);
  const defaultKind = geometry ? inferKind(geometry.type) : "source";
  const defaultSourceType = geometry ? inferSourceType(geometry.type) : "point";

  const [kind, setKind] = useState<FeatureKind>(defaultKind);
  const [sourceType, setSourceType] = useState<SourceType>(defaultSourceType);
  const [height, setHeight] = useState("5");

  // Reset form when geometry changes
  useEffect(() => {
    if (geometry) {
      setKind(inferKind(geometry.type));
      setSourceType(inferSourceType(geometry.type));
      setHeight("5");
    }
  }, [geometry]);

  const handleSave = useCallback(() => {
    if (!geometry) return;
    const feature = {
      id: createFeatureId(),
      kind,
      geometry,
      ...(kind === "source" ? { sourceType } : {}),
      ...(kind === "building" || kind === "barrier"
        ? { heightM: Math.max(0.1, parseFloat(height) || 5) }
        : {}),
    };
    addFeature(feature);
    onClose();
  }, [geometry, kind, sourceType, height, addFeature, onClose]);

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>New Feature</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 py-2">
          <div className="grid gap-1.5">
            <Label className="text-xs">Kind</Label>
            <Select
              value={kind}
              onValueChange={(v) => {
                setKind(v as FeatureKind);
              }}
            >
              <SelectTrigger className="h-8 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="source">Source</SelectItem>
                <SelectItem value="building">Building</SelectItem>
                <SelectItem value="barrier">Barrier</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {kind === "source" ? (
            <div className="grid gap-1.5">
              <Label className="text-xs">Source Type</Label>
              <Select
                value={sourceType}
                onValueChange={(v) => {
                  setSourceType(v as SourceType);
                }}
              >
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="point">Point</SelectItem>
                  <SelectItem value="line">Line</SelectItem>
                  <SelectItem value="area">Area</SelectItem>
                </SelectContent>
              </Select>
            </div>
          ) : null}

          {kind === "building" || kind === "barrier" ? (
            <div className="grid gap-1.5">
              <Label className="text-xs">Height (m)</Label>
              <Input
                type="number"
                step="0.1"
                min="0.1"
                className="h-8 text-xs"
                value={height}
                onChange={(e) => {
                  setHeight(e.target.value);
                }}
              />
            </div>
          ) : null}
        </div>
        <DialogFooter>
          <Button variant="ghost" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <Button size="sm" onClick={handleSave}>
            Add Feature
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
