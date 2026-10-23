import { useId } from "react";
import { m } from "@/i18n/messages";
import {
  facadeOptInMissing,
  facadeRunSpec,
  type FacadeDraft,
} from "@/run/facade-draft";
import { Checkbox } from "@/ui/components/checkbox";
import { Label } from "@/ui/components/label";
import { UnitInput } from "@/ui/components/unit-input";
import { cn } from "@/ui/lib/utils";

/**
 * The inputs of the facade receiver mode. The opt-in is offered only for a
 * normative standard, the only kind whose run refuses unconfirmed points.
 */
export function FacadeFields({
  draft,
  onChange,
  normative,
}: {
  draft: FacadeDraft;
  onChange: (draft: FacadeDraft) => void;
  normative: boolean;
}) {
  const id = useId();
  const buildingsId = `${id}-buildings`;
  const spacingId = `${id}-spacing`;
  const courtyardId = `${id}-courtyard`;
  const incomplete = facadeRunSpec(draft) === null;

  return (
    <div className="space-y-3">
      <div className="space-y-1">
        <Label htmlFor={buildingsId} className="text-xs font-medium">
          {m.label_facade_buildings()}
        </Label>
        <textarea
          id={buildingsId}
          rows={3}
          value={draft.buildings}
          aria-describedby={`${buildingsId}-help`}
          onChange={(event) => {
            onChange({ ...draft, buildings: event.target.value });
          }}
          className={cn(
            "flex w-full rounded-md border border-input bg-background px-3 py-2 text-base ring-offset-background",
            "placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 md:text-sm",
          )}
        />
        <p id={`${buildingsId}-help`} className="text-xs text-muted-foreground">
          {m.msg_facade_buildings_help()}
        </p>
      </div>

      <div className="space-y-1">
        <Label htmlFor={spacingId} className="text-xs font-medium">
          {m.label_facade_spacing()}
        </Label>
        <UnitInput
          id={spacingId}
          type="number"
          min={0}
          step="any"
          unit="m"
          unitId={`${spacingId}-unit`}
          value={draft.spacing}
          aria-describedby={`${spacingId}-unit ${spacingId}-help`}
          onChange={(event) => {
            onChange({ ...draft, spacing: event.target.value });
          }}
        />
        <p id={`${spacingId}-help`} className="text-xs text-muted-foreground">
          {m.msg_facade_spacing_help()}
        </p>
      </div>

      <div className="flex items-center gap-2">
        <Checkbox
          id={courtyardId}
          checked={draft.includeCourtyard}
          onCheckedChange={(checked) => {
            onChange({ ...draft, includeCourtyard: checked === true });
          }}
        />
        <Label htmlFor={courtyardId} className="text-xs">
          {m.label_facade_courtyard()}
        </Label>
      </div>

      {normative ? (
        <>
          <AllowUnconfirmedField
            checked={draft.allowUnconfirmed}
            onChange={(allowUnconfirmed) => {
              onChange({ ...draft, allowUnconfirmed });
            }}
          />
          {facadeOptInMissing(draft, normative) ? (
            <p className="text-xs text-warning">
              {m.msg_facade_opt_in_required()}
            </p>
          ) : null}
        </>
      ) : null}

      {incomplete ? (
        <p className="text-xs text-muted-foreground">
          {m.msg_facade_incomplete()}
        </p>
      ) : null}
    </div>
  );
}

/**
 * The opt-in to run a normative standard over unconfirmed facade points, as
 * `--allow-unconfirmed` gives it. Facade and custom runs both offer it: both
 * are refused over such points without it.
 */
export function AllowUnconfirmedField({
  checked,
  onChange,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  const id = useId();

  return (
    <div className="flex items-start gap-2.5">
      <Checkbox
        id={id}
        className="mt-0.5"
        checked={checked}
        onCheckedChange={(value) => {
          onChange(value === true);
        }}
      />
      <div className="space-y-1">
        <Label htmlFor={id} className="text-xs font-medium">
          {m.label_allow_unconfirmed()}
        </Label>
        <p className="text-xs text-muted-foreground">
          {m.msg_allow_unconfirmed_help()}
        </p>
      </div>
    </div>
  );
}
