import { Input } from "@/ui/components/input";
import { Label } from "@/ui/components/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/ui/components/select";
import { Switch } from "@/ui/components/switch";
import type { ParameterDefinition } from "@/api/client";

// ---------------------------------------------------------------------------
// Parameter editor (shared with setup dialog)
// ---------------------------------------------------------------------------

function ParameterLabel({
  id,
  param,
}: {
  id: string;
  param: ParameterDefinition;
}) {
  return (
    <Label htmlFor={id}>
      {param.name}
      {param.required ? <span className="ml-1 text-destructive">*</span> : null}
    </Label>
  );
}

export function ParameterField({
  param,
  value,
  onChange,
}: {
  param: ParameterDefinition;
  value: string;
  onChange: (v: string) => void;
}) {
  const id = `param-${param.name}`;
  const description = param.description ? (
    <p className="text-xs text-muted-foreground">{param.description}</p>
  ) : null;

  if (param.enum && param.enum.length > 0) {
    return (
      <div className="space-y-1">
        <ParameterLabel id={id} param={param} />
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger id={id}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {param.enum.map((opt) => (
              <SelectItem key={opt} value={opt}>
                {opt}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {description}
      </div>
    );
  }

  if (param.kind === "bool") {
    return (
      <div className="space-y-1">
        <div className="flex h-10 items-center gap-3">
          <Switch
            id={id}
            checked={value === "true"}
            onCheckedChange={(checked) => {
              onChange(checked ? "true" : "false");
            }}
          />
          <ParameterLabel id={id} param={param} />
        </div>
        {description}
      </div>
    );
  }

  const inputType =
    param.kind === "float" || param.kind === "int" ? "number" : "text";
  const step = param.kind === "float" ? "any" : undefined;

  return (
    <div className="space-y-1">
      <ParameterLabel id={id} param={param} />
      <Input
        id={id}
        type={inputType}
        step={step}
        value={value}
        onChange={(e) => {
          onChange(e.target.value);
        }}
        min={param.min}
        max={param.max}
      />
      {description}
    </div>
  );
}
