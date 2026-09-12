import { Link } from "react-router";
import { useHealth, useProjectStatus } from "@/api/hooks";
import type { HealthResponse, ProjectStatusResponse } from "@/api/client";
import { Button } from "@/ui/components/button";
import { Callout } from "@/ui/callout";
import { formatDateTime } from "@/ui/format";
import { KeyValueList } from "@/ui/key-value-list";
import { PageHeader, SectionHeading } from "@/ui/page-header";
import { m } from "@/i18n/messages";

function HealthSection({ data }: { data: HealthResponse }) {
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

function ProjectSection({ data }: { data: ProjectStatusResponse | null }) {
  if (!data) {
    return (
      <Callout variant="neutral" title={m.msg_no_project_yet()}>
        <p>{m.msg_no_project_yet_help()}</p>
        <div className="mt-3 flex flex-wrap gap-2">
          <Button asChild size="sm">
            <Link to="/import">{m.nav_import()}</Link>
          </Button>
        </div>
      </Callout>
    );
  }
  return (
    <KeyValueList
      items={[
        { label: m.label_name_field(), value: data.name },
        { label: m.label_crs_field(), value: data.crs, mono: true },
        {
          label: m.label_scenarios_field(),
          value: String(data.scenario_count),
        },
        { label: m.label_runs_field(), value: String(data.run_count) },
      ]}
    />
  );
}

function QueryResult<T>({
  isLoading,
  isError,
  error,
  data,
  loadingText,
  children,
}: {
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
  data: T | null | undefined;
  loadingText: string;
  children: (data: T) => React.ReactNode;
}) {
  if (isLoading) {
    return <p className="text-sm text-muted-foreground">{loadingText}</p>;
  }
  if (isError) {
    return (
      <Callout variant="destructive">
        {error?.message ?? m.msg_unknown_error()}
      </Callout>
    );
  }
  if (data == null) {
    return null;
  }
  return <>{children(data)}</>;
}

export default function StatusPage() {
  const health = useHealth();
  const project = useProjectStatus();

  return (
    <div className="flex flex-1 flex-col gap-6 p-6">
      <PageHeader title={m.page_title_status()} />

      <section className="grid gap-3">
        <SectionHeading>{m.section_backend_health()}</SectionHeading>
        <QueryResult {...health} loadingText={m.status_loading_health()}>
          {(data) => <HealthSection data={data} />}
        </QueryResult>
      </section>

      <section className="grid gap-3">
        <SectionHeading>{m.section_project()}</SectionHeading>
        {project.isLoading ? (
          <p className="text-sm text-muted-foreground">
            {m.status_loading_project()}
          </p>
        ) : project.isError ? (
          <Callout variant="destructive">{project.error.message}</Callout>
        ) : (
          <ProjectSection data={project.data ?? null} />
        )}
      </section>
    </div>
  );
}
