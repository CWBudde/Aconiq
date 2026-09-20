import { Label } from "@/ui/components/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/ui/components/select";
import { Switch } from "@/ui/components/switch";
import { UnitInput } from "@/ui/components/unit-input";
import type { ParameterDefinition } from "@/api/client";
import { parameterDescription, parameterLabel } from "@/run/parameter-meta";

/**
 * One editable parameter of a run profile.
 *
 * The label is the human name and nothing else. The unit sits on the control,
 * where it belongs to the value rather than to the name, and the backend's own
 * name sits under it — that name is the CLI and API identity, it is what
 * `--param` and the request body take, and a user reading the dialog should be
 * able to write the command without translating back. It used to share the
 * label's line, where it collided with the label it was meant to annotate.
 */
function ParameterLabel({
  id,
  standardId,
  param,
}: {
  id: string;
  standardId: string;
  param: ParameterDefinition;
}) {
  return (
    <Label htmlFor={id}>
      {parameterLabel(standardId, param)}
      {param.required ? <span className="ml-1 text-destructive">*</span> : null}
    </Label>
  );
}

/**
 * What the parameter is for.
 *
 * The backend's own name — `traffic_night_pkw` — used to be rendered here, on
 * the argument that it is what `--param` and the request body take. It is gone.
 * That argument was made when the label was a humanised form of the name and
 * the two were a keystroke apart; now the label is the parameter's German name,
 * the raw one is a second string saying the same thing in a spelling only the
 * CLI reads, and a dialog with nineteen of them down the side is answering a
 * question nobody asked while it is open. The CLI documents its own arguments.
 *
 * The caller resolves the sentence and renders nothing at all where there is
 * none, so that the id it names is the id of an element that exists.
 */
function ParameterNote({
  id,
  description,
}: {
  id: string;
  description: string;
}) {
  return (
    <p id={id} className="text-xs text-muted-foreground">
      {description}
    </p>
  );
}

export function ParameterField({
  standardId,
  param,
  value,
  onChange,
  describedById,
}: {
  standardId: string;
  param: ParameterDefinition;
  value: string;
  onChange: (v: string) => void;
  /**
   * A note the group already renders, because every field under it says the
   * same thing. The field then prints none of its own and points here instead —
   * the description is not lost, only said once.
   */
  describedById?: string | undefined;
}) {
  const id = `param-${param.name}`;
  const unitId = `${id}-unit`;
  // Resolved before the element is built, because whether there is a sentence
  // is what decides both. A dangling `aria-describedby` is a description a
  // screen reader announces as nothing, which is worse than none — and a
  // catalogued standard is not the only way a parameter arrives without one.
  const description =
    describedById === undefined
      ? parameterDescription(standardId, param)
      : null;
  const noteId = `${id}-note`;
  const note =
    description === null ? null : (
      <ParameterNote id={noteId} description={description} />
    );
  const describedBy = [
    param.unit ? unitId : null,
    describedById ?? (description === null ? null : noteId),
  ]
    .filter((part) => part !== null)
    .join(" ");
  const describedByProp =
    describedBy === "" ? undefined : { "aria-describedby": describedBy };

  if (param.enum && param.enum.length > 0) {
    return (
      <div className="space-y-1">
        <ParameterLabel id={id} standardId={standardId} param={param} />
        <Select value={value} onValueChange={onChange}>
          <SelectTrigger id={id} {...describedByProp}>
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
        {note}
      </div>
    );
  }

  if (param.kind === "bool") {
    return (
      <div className="space-y-1">
        <div className="flex h-10 items-center gap-3">
          <Switch
            id={id}
            {...describedByProp}
            checked={value === "true"}
            onCheckedChange={(checked) => {
              onChange(checked ? "true" : "false");
            }}
          />
          <ParameterLabel id={id} standardId={standardId} param={param} />
        </div>
        {note}
      </div>
    );
  }

  const inputType =
    param.kind === "float" || param.kind === "int" ? "number" : "text";
  const step = param.kind === "float" ? "any" : undefined;

  return (
    <div className="space-y-1">
      <ParameterLabel id={id} standardId={standardId} param={param} />
      <UnitInput
        id={id}
        type={inputType}
        step={step}
        unit={param.unit}
        unitId={unitId}
        {...describedByProp}
        value={value}
        onChange={(e) => {
          onChange(e.target.value);
        }}
        min={param.min}
        max={param.max}
      />
      {note}
    </div>
  );
}
