import { Link, useLocation } from "react-router";
import { Compass } from "lucide-react";
import { Button } from "@/ui/components/button";
import { PageHeader } from "@/ui/page-header";
import { EmptyState } from "@/ui/empty-state";
import { m } from "@/i18n/messages";

/**
 * The catch-all, rendered *inside* the shell rather than as the router's
 * `errorElement`. An `errorElement` on the layout route replaces the layout,
 * so the rail and the header `<h1>` would both vanish: the user would have no
 * way out, and `waitForPage` in `e2e/app.ts` would hang for the full timeout
 * reporting a missing locator instead of the real cause.
 */
export default function NotFoundPage() {
  const location = useLocation();
  return (
    <div className="flex flex-1 flex-col gap-6 p-6">
      <PageHeader title={m.page_title_not_found()} />
      <EmptyState
        icon={Compass}
        title={m.page_title_not_found()}
        description={m.msg_not_found({ path: location.pathname })}
      >
        <Button asChild variant="outline">
          <Link to="/import">{m.nav_import()}</Link>
        </Button>
      </EmptyState>
    </div>
  );
}
