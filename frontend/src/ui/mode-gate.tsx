import { cloneElement } from "react";
import type * as React from "react";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/ui/components/tooltip";
import { backend } from "@/api/backend";
import { cn } from "@/ui/lib/utils";

/**
 * Capabilities whose `true` means "this action is possible here".
 *
 * One member, on purpose. `runsAgainstSavedModel === true` does not mean
 * "allowed" — it says which model a run reads — so it is not gateable, and
 * widening this union has to be a deliberate act rather than a sweep.
 */
type GateableCapability = "canExport";

/** The props `ModeGate` overrides on the control it wraps. */
interface GatedProps {
  className?: string | undefined;
  disabled?: boolean | undefined;
  onClick?: React.MouseEventHandler | undefined;
  "aria-disabled"?: boolean | undefined;
  "data-mode-gated"?: string | undefined;
}

export interface ModeGateProps {
  capability: GateableCapability;
  /** Why the action is unavailable here. Tooltip text and accessible description. */
  reason: string;
  /** The control. Rendered untouched when the capability is present. */
  children: React.ReactElement<GatedProps>;
}

/**
 * Disables a control the current backend cannot perform, and says why.
 *
 * The alternative this replaces was hiding the control, which leaves a user
 * looking for a button that is not there. A disabled control with a reason is
 * the honest version.
 *
 * **`aria-disabled`, not `disabled`.** `ui/components/button.tsx` puts
 * `disabled:pointer-events-none` on every button, and a disabled DOM button
 * takes no focus and fires no pointer events — so a tooltip whose trigger is a
 * disabled button never opens, by mouse or by keyboard. That is not
 * hypothetical: it is why the tooltips in `map/undo-redo-bar.tsx` are silent
 * in exactly the state they describe. `aria-disabled` keeps the element
 * hoverable and focusable, announces unavailability, and lets Radix point the
 * trigger at the reason through `aria-describedby` — which is why that
 * attribute is not set here; setting it would win over Radix's and name an id
 * that only exists while the tooltip is open. A click handler swallows
 * activation,
 * which covers Enter and Space too since both produce a click.
 *
 * The child's *own* `disabled` is converted the same way while gated. Left as
 * a real attribute it would put pointer-events back to `none` whenever both
 * applied, and the tooltip would vanish in half the cases it exists for.
 *
 * The provider is local rather than mounted in `AppShell`: the app has one
 * only as an implementation detail of `SidebarProvider`, and Radix's tooltip
 * context has no default, so a page rendered standalone in a unit test would
 * throw. Nesting providers is legal and only overrides the delay below.
 */
export function ModeGate({ capability, reason, children }: ModeGateProps) {
  if (backend.capabilities[capability]) return children;

  const gated = cloneElement<GatedProps>(children, {
    disabled: false,
    "aria-disabled": true,
    "data-mode-gated": "",
    onClick: (event) => {
      event.preventDefault();
      event.stopPropagation();
    },
    className: cn(children.props.className, "opacity-50 cursor-not-allowed"),
  });

  return (
    <TooltipProvider delayDuration={200}>
      <Tooltip>
        <TooltipTrigger asChild>{gated}</TooltipTrigger>
        <TooltipContent>{reason}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}
