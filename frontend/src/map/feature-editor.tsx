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
import type { ModelFeature, SourceType } from "@/model/types";
import {
  getFeatureNumber,
  getFeatureString,
  getInferredFlag,
  getRLS19ReviewRequired,
  RLS19_JUNCTION_TYPES,
  RLS19_SURFACE_TYPES,
  setFeatureProperty,
} from "@/model/source-acoustics";
import { Trash2 } from "lucide-react";
import { m } from "@/i18n/messages";

const FEATURE_KIND_LABELS: Record<"source" | "building" | "barrier", string> = {
  source: m.option_source(),
  building: m.option_building(),
  barrier: m.option_barrier(),
};

const VEHICLE_CLASS_LABELS: Record<"pkw" | "lkw1" | "lkw2" | "krad", string> = {
  pkw: m.label_vehicle_class_pkw(),
  lkw1: m.label_vehicle_class_lkw1(),
  lkw2: m.label_vehicle_class_lkw2(),
  krad: m.label_vehicle_class_krad(),
};

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
        <h3 className="text-sm font-semibold capitalize">
          {FEATURE_KIND_LABELS[feature.kind]}
        </h3>
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
          <Label className="text-xs text-muted-foreground">
            {m.label_id()}
          </Label>
          <p className="font-mono text-xs">{feature.id}</p>
        </div>
        <div>
          <Label className="text-xs text-muted-foreground">
            {m.label_geometry()}
          </Label>
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
  }, [receiver, receiver?.heightM]);

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
        <h3 className="text-sm font-semibold">{m.label_receiver()}</h3>
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
          <Label className="text-xs text-muted-foreground">
            {m.label_id()}
          </Label>
          <p className="font-mono text-xs">{receiver.id}</p>
        </div>
        <div>
          <Label className="text-xs text-muted-foreground">
            {m.label_geometry()}
          </Label>
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
    <div className="space-y-3">
      <div className="grid gap-1.5">
        <Label htmlFor="source-type" className="text-xs">
          {m.label_source_type()}
        </Label>
        <Select
          value={feature.sourceType ?? ""}
          onValueChange={handleTypeChange}
        >
          <SelectTrigger id="source-type" className="h-8 text-xs">
            <SelectValue placeholder={m.placeholder_select_source_type()} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="point">
              {m.option_source_type_point()}
            </SelectItem>
            <SelectItem value="line">{m.option_source_type_line()}</SelectItem>
            <SelectItem value="area">{m.option_source_type_area()}</SelectItem>
          </SelectContent>
        </Select>
      </div>
      {feature.sourceType === "line" ? (
        <RLS19RoadFields feature={feature} />
      ) : null}
    </div>
  );
}

function RLS19RoadFields({ feature }: { feature: ModelFeature }) {
  return (
    <div className="space-y-3 rounded-md border bg-muted/30 p-3">
      <div className="space-y-1">
        <p className="text-xs font-medium">
          {m.label_section_source_acoustics()}
        </p>
        <p className="text-[11px] leading-relaxed text-muted-foreground">
          {m.msg_source_acoustics_defaults()}
        </p>
        {getRLS19ReviewRequired(feature) ? (
          <p className="text-[11px] leading-relaxed text-amber-700 dark:text-amber-300">
            {m.msg_source_acoustics_review_required()}
          </p>
        ) : null}
      </div>
      <PropertySelectField
        feature={feature}
        propertyKey="surface_type"
        aliases={["road_surface_type"]}
        label={m.label_surface_type()}
        options={[...RLS19_SURFACE_TYPES]}
      />
      <PropertyNumberField
        feature={feature}
        propertyKey="road_speed_kph"
        label={m.label_uniform_speed_kph()}
        min={0.1}
        step={1}
      />
      <div className="grid grid-cols-2 gap-2">
        <PropertyNumberField
          feature={feature}
          propertyKey="speed_pkw_kph"
          label={VEHICLE_CLASS_LABELS.pkw}
          min={0.1}
          step={1}
        />
        <PropertyNumberField
          feature={feature}
          propertyKey="speed_lkw1_kph"
          label={VEHICLE_CLASS_LABELS.lkw1}
          min={0.1}
          step={1}
        />
        <PropertyNumberField
          feature={feature}
          propertyKey="speed_lkw2_kph"
          label={VEHICLE_CLASS_LABELS.lkw2}
          min={0.1}
          step={1}
        />
        <PropertyNumberField
          feature={feature}
          propertyKey="speed_krad_kph"
          label={VEHICLE_CLASS_LABELS.krad}
          min={0.1}
          step={1}
        />
      </div>
      <PropertyNumberField
        feature={feature}
        propertyKey="gradient_percent"
        aliases={["road_gradient_percent"]}
        label={m.label_gradient_percent()}
        min={-12}
        max={12}
        step={0.1}
      />
      <PropertySelectField
        feature={feature}
        propertyKey="junction_type"
        aliases={["road_junction_type"]}
        label={m.label_junction_type()}
        options={[...RLS19_JUNCTION_TYPES]}
      />
      <div className="grid grid-cols-2 gap-2">
        <PropertyNumberField
          feature={feature}
          propertyKey="junction_distance_m"
          aliases={["road_junction_distance_m"]}
          label={m.label_junction_distance_m()}
          min={0}
          step={1}
        />
        <PropertyNumberField
          feature={feature}
          propertyKey="reflection_surcharge_db"
          label={m.label_reflection_surcharge_db()}
          step={0.1}
        />
      </div>
      <div className="space-y-2">
        <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          {m.label_traffic_day()}
        </p>
        <div className="grid grid-cols-2 gap-2">
          <PropertyNumberField
            feature={feature}
            propertyKey="traffic_day_pkw"
            label={VEHICLE_CLASS_LABELS.pkw}
            min={0}
            step={1}
          />
          <PropertyNumberField
            feature={feature}
            propertyKey="traffic_day_lkw1"
            label={VEHICLE_CLASS_LABELS.lkw1}
            min={0}
            step={1}
          />
          <PropertyNumberField
            feature={feature}
            propertyKey="traffic_day_lkw2"
            label={VEHICLE_CLASS_LABELS.lkw2}
            min={0}
            step={1}
          />
          <PropertyNumberField
            feature={feature}
            propertyKey="traffic_day_krad"
            label={VEHICLE_CLASS_LABELS.krad}
            min={0}
            step={1}
          />
        </div>
      </div>
      <div className="space-y-2">
        <p className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          {m.label_traffic_night()}
        </p>
        <div className="grid grid-cols-2 gap-2">
          <PropertyNumberField
            feature={feature}
            propertyKey="traffic_night_pkw"
            label={VEHICLE_CLASS_LABELS.pkw}
            min={0}
            step={1}
          />
          <PropertyNumberField
            feature={feature}
            propertyKey="traffic_night_lkw1"
            label={VEHICLE_CLASS_LABELS.lkw1}
            min={0}
            step={1}
          />
          <PropertyNumberField
            feature={feature}
            propertyKey="traffic_night_lkw2"
            label={VEHICLE_CLASS_LABELS.lkw2}
            min={0}
            step={1}
          />
          <PropertyNumberField
            feature={feature}
            propertyKey="traffic_night_krad"
            label={VEHICLE_CLASS_LABELS.krad}
            min={0}
            step={1}
          />
        </div>
      </div>
    </div>
  );
}

function PropertyNumberField({
  feature,
  propertyKey,
  aliases = [],
  label,
  min,
  max,
  step,
}: {
  feature: ModelFeature;
  propertyKey: string;
  aliases?: string[];
  label: string;
  min?: number;
  max?: number;
  step?: number;
}) {
  const updateFeature = useModelStore((s) => s.updateFeature);
  const current = getFeatureNumber(feature, propertyKey, ...aliases);
  const [value, setValue] = useState(current == null ? "" : String(current));

  useEffect(() => {
    setValue(current == null ? "" : String(current));
  }, [current]);

  const handleBlur = useCallback(() => {
    const trimmed = value.trim();
    if (trimmed === "") {
      updateFeature(
        setFeatureProperty(feature, propertyKey, undefined, ...aliases),
      );
      return;
    }

    const numeric = Number.parseFloat(trimmed);
    if (!Number.isFinite(numeric)) {
      return;
    }

    updateFeature(
      setFeatureProperty(feature, propertyKey, numeric, ...aliases),
    );
  }, [aliases, feature, propertyKey, updateFeature, value]);

  const helper = getInferredFlag(feature, propertyKey)
    ? m.msg_source_acoustics_inferred()
    : m.msg_source_acoustics_default_fallback();

  return (
    <div className="grid gap-1">
      <Label htmlFor={`${feature.id}-${propertyKey}`} className="text-[11px]">
        {label}
      </Label>
      <Input
        id={`${feature.id}-${propertyKey}`}
        type="number"
        min={min != null ? String(min) : undefined}
        max={max != null ? String(max) : undefined}
        step={step != null ? String(step) : undefined}
        className="h-8 text-xs"
        placeholder={m.placeholder_use_run_default()}
        value={value}
        onChange={(e) => {
          setValue(e.target.value);
        }}
        onBlur={handleBlur}
      />
      <p className="text-[10px] text-muted-foreground">{helper}</p>
    </div>
  );
}

function PropertySelectField({
  feature,
  propertyKey,
  aliases = [],
  label,
  options,
}: {
  feature: ModelFeature;
  propertyKey: string;
  aliases?: string[];
  label: string;
  options: string[];
}) {
  const updateFeature = useModelStore((s) => s.updateFeature);
  const current = getFeatureString(feature, propertyKey, ...aliases);

  const handleChange = useCallback(
    (value: string) => {
      updateFeature(
        setFeatureProperty(
          feature,
          propertyKey,
          value === "__default__" ? undefined : value,
          ...aliases,
        ),
      );
    },
    [aliases, feature, propertyKey, updateFeature],
  );

  const helper = getInferredFlag(feature, propertyKey)
    ? m.msg_source_acoustics_inferred()
    : m.msg_source_acoustics_default_fallback();

  return (
    <div className="grid gap-1">
      <Label className="text-[11px]">{label}</Label>
      <Select value={current ?? "__default__"} onValueChange={handleChange}>
        <SelectTrigger className="h-8 text-xs">
          <SelectValue placeholder={m.placeholder_use_run_default()} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__default__">
            {m.option_use_run_default()}
          </SelectItem>
          {options.map((option) => (
            <SelectItem key={option} value={option}>
              {option}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <p className="text-[10px] text-muted-foreground">{helper}</p>
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
