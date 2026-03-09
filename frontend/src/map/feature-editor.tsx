import { useCallback, useEffect, useState } from "react";
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
import { useModelStore } from "@/model/model-store";
import type { ModelFeature, ModelReceiver, SourceType } from "@/model/types";
import { Trash2 } from "lucide-react";
import { m } from "@/i18n/messages";

interface FeatureEditorProps {
  featureId: string | null;
  onClose: () => void;
}

export function FeatureEditor({ featureId, onClose }: FeatureEditorProps) {
  const feature = useModelStore((s) =>
    featureId ? s.getFeatureById(featureId) : undefined,
  );
  const receiver = useModelStore((s) =>
    featureId && !feature ? s.getReceiverById(featureId) : undefined,
  );

  if (receiver) {
    return <ReceiverEditor receiverId={receiver.id} onClose={onClose} />;
  }

  if (!feature) return null;

  return (
    <div className="absolute right-3 top-3 z-10 w-72 rounded-md border bg-background p-4 shadow-md">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-semibold capitalize">{feature.kind}</h3>
        <Button
          variant="ghost"
          size="sm"
          onClick={onClose}
          aria-label={m.tooltip_close_editor()}
        >
          &times;
        </Button>
      </div>
      <div className="space-y-3">
        <div>
          <Label className="text-xs text-muted-foreground">{m.label_id()}</Label>
          <p className="font-mono text-xs">{feature.id}</p>
        </div>
        <div>
          <Label className="text-xs text-muted-foreground">{m.label_geometry()}</Label>
          <p className="text-xs">{feature.geometry.type}</p>
        </div>
        <FeatureFields feature={feature} />
        <DeleteButton featureId={feature.id} onDelete={onClose} />
      </div>
    </div>
  );
}

function ReceiverEditor({
  receiverId,
  onClose,
}: {
  receiverId: string;
  onClose: () => void;
}) {
  const receiver = useModelStore((s) => s.getReceiverById(receiverId));
  const updateReceiver = useModelStore((s) => s.updateReceiver);
  const removeReceiver = useModelStore((s) => s.removeReceiver);

  const [heightValue, setHeightValue] = useState(
    String(receiver?.heightM ?? "4"),
  );

  useEffect(() => {
    if (receiver) {
      setHeightValue(String(receiver.heightM));
    }
  }, [receiver?.heightM]);

  const handleHeightBlur = useCallback(() => {
    if (!receiver) return;
    const num = parseFloat(heightValue);
    if (Number.isFinite(num) && num > 0) {
      updateReceiver({ ...receiver, heightM: num });
    }
  }, [receiver, heightValue, updateReceiver]);

  const handleDelete = useCallback(() => {
    removeReceiver(receiverId);
    onClose();
  }, [receiverId, removeReceiver, onClose]);

  if (!receiver) return null;

  return (
    <div className="absolute right-3 top-3 z-10 w-72 rounded-md border bg-background p-4 shadow-md">
      <div className="mb-3 flex items-center justify-between">
        <h3 className="text-sm font-semibold">Receiver</h3>
        <Button
          variant="ghost"
          size="sm"
          onClick={onClose}
          aria-label={m.tooltip_close_editor()}
        >
          &times;
        </Button>
      </div>
      <div className="space-y-3">
        <div>
          <Label className="text-xs text-muted-foreground">{m.label_id()}</Label>
          <p className="font-mono text-xs">{receiver.id}</p>
        </div>
        <div>
          <Label className="text-xs text-muted-foreground">{m.label_geometry()}</Label>
          <p className="text-xs">{receiver.geometry.type}</p>
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="receiver-height" className="text-xs">
            {m.label_height_m()}
          </Label>
          <Input
            id="receiver-height"
            type="number"
            step="0.1"
            min="0.1"
            className="h-8 text-xs"
            value={heightValue}
            onChange={(e) => {
              setHeightValue(e.target.value);
            }}
            onBlur={handleHeightBlur}
          />
        </div>
        <Button
          variant="destructive"
          size="sm"
          className="mt-2 w-full"
          onClick={handleDelete}
        >
          <Trash2 className="mr-1.5 h-3.5 w-3.5" />
          {m.action_delete_feature()}
        </Button>
      </div>
    </div>
  );
}

function FeatureFields({ feature }: { feature: ModelFeature }) {
  switch (feature.kind) {
    case "source":
      return <SourceFields feature={feature} />;
    case "building":
    case "barrier":
      return <HeightField feature={feature} />;
  }
}

function SourceFields({ feature }: { feature: ModelFeature }) {
  const updateFeature = useModelStore((s) => s.updateFeature);

  const handleTypeChange = useCallback(
    (value: string) => {
      updateFeature({ ...feature, sourceType: value as SourceType });
    },
    [feature, updateFeature],
  );

  return (
    <div className="grid gap-1.5">
      <Label htmlFor="source-type" className="text-xs">
        {m.label_source_type()}
      </Label>
      <Select value={feature.sourceType ?? ""} onValueChange={handleTypeChange}>
        <SelectTrigger id="source-type" className="h-8 text-xs">
          <SelectValue placeholder="Select type" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="point">{m.option_source_type_point()}</SelectItem>
          <SelectItem value="line">{m.option_source_type_line()}</SelectItem>
          <SelectItem value="area">{m.option_source_type_area()}</SelectItem>
        </SelectContent>
      </Select>
    </div>
  );
}

function HeightField({ feature }: { feature: ModelFeature }) {
  const updateFeature = useModelStore((s) => s.updateFeature);
  const [value, setValue] = useState(String(feature.heightM ?? ""));

  useEffect(() => {
    setValue(String(feature.heightM ?? ""));
  }, [feature.heightM]);

  const handleBlur = useCallback(() => {
    const num = parseFloat(value);
    if (Number.isFinite(num) && num > 0) {
      updateFeature({ ...feature, heightM: num });
    }
  }, [feature, value, updateFeature]);

  return (
    <div className="grid gap-1.5">
      <Label htmlFor="height" className="text-xs">
        {m.label_height_m()}
      </Label>
      <Input
        id="height"
        type="number"
        step="0.1"
        min="0.1"
        className="h-8 text-xs"
        value={value}
        onChange={(e) => {
          setValue(e.target.value);
        }}
        onBlur={handleBlur}
      />
    </div>
  );
}

function DeleteButton({
  featureId,
  onDelete,
}: {
  featureId: string;
  onDelete: () => void;
}) {
  const removeFeature = useModelStore((s) => s.removeFeature);

  const handleDelete = useCallback(() => {
    removeFeature(featureId);
    onDelete();
  }, [featureId, removeFeature, onDelete]);

  return (
    <Button
      variant="destructive"
      size="sm"
      className="mt-2 w-full"
      onClick={handleDelete}
    >
      <Trash2 className="mr-1.5 h-3.5 w-3.5" />
      {m.action_delete_feature()}
    </Button>
  );
}
