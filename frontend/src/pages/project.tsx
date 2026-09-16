import { Link } from "react-router";
import { AlertTriangle, CheckCircle2, FileInput, PenLine } from "lucide-react";
import { useHealth } from "@/api/hooks";
import type { HealthResponse } from "@/api/client";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { Callout } from "@/ui/callout";
import { formatDateTime } from "@/ui/format";
import { KeyValueList } from "@/ui/key-value-list";
import { PageHeader, SectionHeading } from "@/ui/page-header";
import { ProjectSummary } from "@/ui/project-summary";
import { useModelValidation } from "@/model/use-model-validation";
import { DRAW_PARAM } from "@/map/map-params";
import { m } from "@/i18n/messages";

/**
 * The project page, at `/`.
 *
 * It replaces a pair: a welcome page whose live content was a project summary
 * under three cards of static marketing copy, and a status page carrying the
 * same summary again. What is left answers one question — what state is this
 * project in, and where do I start.
 *
 * The `PageHeader` renders unconditionally, outside every query branch. That
 * is not tidiness: `waitForPage` in `e2e/app.ts` waits for a header `h1` *and*
 * an `h2` or `h3` inside `main`, so a page whose only heading sat inside a
 * success branch would, with the backend down, hang every e2e spec on this
 * route for the full Playwright timeout instead of failing legibly.
 */
export default function ProjectPage() {
  return (
    <div className="flex flex-1 flex-col gap-6 p-6">
      <PageHeader
        title={m.page_title_project()}
        description={m.msg_project_intro()}
      />

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.6fr)_minmax(18rem,1fr)]">
        <div className="grid gap-6">
          <Card className="grid gap-3 p-6">
            <SectionHeading>{m.section_get_started()}</SectionHeading>
            <div className="flex flex-wrap gap-3">
              <Button asChild>
                <Link to="/import">
                  <FileInput aria-hidden="true" />
                  {m.action_start_import()}
                </Link>
              </Button>
              <Button asChild variant="outline">
                {/* A boolean, not a mode name: which mode drawing starts in is
                    the map's decision and lives in one place. */}
                <Link to={`/model?${DRAW_PARAM}=1`}>
                  <PenLine aria-hidden="true" />
                  {m.action_start_drawing()}
                </Link>
              </Button>
            </div>
          </Card>

          <Card className="grid gap-3 p-6">
            <SectionHeading>{m.label_validation()}</SectionHeading>
            <ValidationSummary />
          </Card>
        </div>

        <div className="grid gap-6">
          <Card className="grid gap-3 p-6">
            <SectionHeading>{m.section_backend_health()}</SectionHeading>
            <HealthSummary />
          </Card>

          <Card className="grid gap-3 p-6">
            <SectionHeading>{m.section_project()}</SectionHeading>
            <ProjectSummary
              emptyAction={
                <Button asChild size="sm">
                  <Link to="/import">{m.nav_import()}</Link>
                </Button>
              }
            />
          </Card>
        </div>
      </div>
    </div>
  );
}

function HealthFacts({ data }: { data: HealthResponse }) {
  return (
    <KeyValueList
      items={[
        { label: m.label_status_field(), value: data.status },
        { label: m.label_version_field(), value: data.version, mono: true },
        { label: m.label_time_field(), value: formatDateTime(data.time) },
      ]}
    />
  );
}

function HealthSummary() {
  const health = useHealth();

  if (health.isLoading) {
    return (
      <p className="text-sm text-muted-foreground">
        {m.status_loading_health()}
      </p>
    );
  }
  if (health.isError) {
    return <Callout variant="destructive">{health.error.message}</Callout>;
  }
  if (health.data == null) return null;
  return <HealthFacts data={health.data} />;
}

/**
 * The model's validation state, through `useModelValidation` — which passes
 * the receivers and answers "empty" before validating, so a fresh install
 * reads as "nothing yet" rather than as one error in English.
 */
function ValidationSummary() {
  const { state, errorCount, warningCount } = useModelValidation();

  if (state === "empty") {
    return (
      <Callout variant="neutral">{m.msg_validation_nothing_yet()}</Callout>
    );
  }
  if (state === "valid") {
    return (
      <Callout variant="success" icon={CheckCircle2}>
        {m.msg_model_valid()}
      </Callout>
    );
  }

  return (
    <Callout variant="warning" icon={AlertTriangle}>
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <span>
          {errorCount > 0
            ? errorCount === 1
              ? m.msg_validation_error_count_one({ count: errorCount })
              : m.msg_validation_error_count_other({ count: errorCount })
            : null}
          {errorCount > 0 && warningCount > 0 ? ", " : null}
          {warningCount > 0
            ? warningCount === 1
              ? m.msg_validation_warning_count_one({ count: warningCount })
              : m.msg_validation_warning_count_other({ count: warningCount })
            : null}
        </span>
        <Button asChild size="sm" variant="outline">
          <Link to="/model">{m.nav_model()}</Link>
        </Button>
      </div>
    </Callout>
  );
}
