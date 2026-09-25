import { Input } from "@/ui/components/input";
import { Label } from "@/ui/components/label";
import type { BBoxText } from "./bbox";
import { m } from "@/i18n/messages";

function BBoxField({
  id,
  label,
  value,
  placeholder,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  placeholder: string;
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <div className="flex flex-col gap-1">
      <Input
        id={id}
        type="number"
        step="any"
        value={value}
        disabled={disabled}
        onChange={(e) => {
          onChange(e.target.value);
        }}
        placeholder={placeholder}
      />
      <Label htmlFor={id} className="text-center text-xs text-muted-foreground">
        {label}
      </Label>
    </div>
  );
}

/**
 * The four box inputs, south-west-north-east, as the Overpass query orders
 * them. `idPrefix` keeps the ids unique per tab; `onFieldChange` receives one
 * member at a time, so a caller writes it through its own updater rather than
 * spreading a box this render closed over.
 */
export function BBoxFields({
  idPrefix,
  value,
  disabled = false,
  onFieldChange,
}: {
  idPrefix: string;
  value: BBoxText;
  disabled?: boolean;
  onFieldChange: (field: keyof BBoxText, value: string) => void;
}) {
  return (
    <div className="grid grid-cols-4 gap-3">
      <BBoxField
        id={`${idPrefix}-south`}
        label={m.label_south()}
        value={value.south}
        placeholder="52.49"
        disabled={disabled}
        onChange={(v) => {
          onFieldChange("south", v);
        }}
      />
      <BBoxField
        id={`${idPrefix}-west`}
        label={m.label_west()}
        value={value.west}
        placeholder="13.35"
        disabled={disabled}
        onChange={(v) => {
          onFieldChange("west", v);
        }}
      />
      <BBoxField
        id={`${idPrefix}-north`}
        label={m.label_north()}
        value={value.north}
        placeholder="52.52"
        disabled={disabled}
        onChange={(v) => {
          onFieldChange("north", v);
        }}
      />
      <BBoxField
        id={`${idPrefix}-east`}
        label={m.label_east()}
        value={value.east}
        placeholder="13.40"
        disabled={disabled}
        onChange={(v) => {
          onFieldChange("east", v);
        }}
      />
    </div>
  );
}
