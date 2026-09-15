import { Globe, Server } from "lucide-react";
import { Badge } from "@/ui/components/badge";
import { backend } from "@/api/backend";
import { m } from "@/i18n/messages";

/**
 * Which backend this UI is talking to, in the header.
 *
 * This is the one place outside Settings that reads `capabilities.kind`, and
 * it reads it exactly the way the field's own doc comment asks — "for labels,
 * not for branching". Anything that changes *behaviour* by mode branches on a
 * named capability instead.
 *
 * Decorative: a plain badge with no role, no link and no `title`. A link here
 * would add a fifth header tab stop on every route for a value that only
 * changes when the app is restarted against a different backend, and the place
 * to change it — Settings, Connection — is already one rail click away. The
 * long form rides in an `sr-only` span rather than an `aria-label`, because an
 * `aria-label` on a role-less element is what axe's `aria-prohibited-attr`
 * flags, and the baseline is checked in both directions.
 */
export function ModeChip() {
  const browser = backend.capabilities.kind === "browser";
  const Icon = browser ? Globe : Server;

  return (
    <Badge
      variant="outline"
      data-testid="mode-chip"
      data-mode={backend.capabilities.kind}
      className="hidden gap-1 font-normal text-muted-foreground sm:inline-flex"
    >
      <Icon aria-hidden="true" className="size-3" />
      <span aria-hidden="true">
        {browser ? m.label_mode_browser() : m.label_mode_api()}
      </span>
      <span className="sr-only">
        {browser ? m.msg_runtime_wasm() : m.msg_runtime_api()}
      </span>
    </Badge>
  );
}
