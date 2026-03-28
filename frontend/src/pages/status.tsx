import { Link } from "react-router";
import { useHealth, useProjectStatus } from "@/api";
import type { HealthResponse, ProjectStatusResponse } from "@/api";
import { Button } from "@/ui/components/button";
import { m } from "@/i18n/messages";

function HealthSection({ data }: { data: HealthResponse }) {
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
      <dt className="text-muted-foreground">{m.label_status_field()}</dt>
      <dd>{data.status}</dd>
      <dt className="text-muted-foreground">{m.label_version_field()}</dt>
      <dd className="font-mono">{data.version}</dd>
      <dt className="text-muted-foreground">{m.label_time_field()}</dt>
      <dd className="font-mono">{data.time}</dd>
    </dl>
  );
}

function ProjectSection({ data }: { data: ProjectStatusResponse | null }) {
  if (!data) {
    return (
      <div className="rounded-2xl border bg-muted/30 p-4">
        <p className="text-sm font-medium">{m.msg_no_project_yet()}</p>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">
          {m.msg_no_project_yet_help()}
        </p>
        <div className="mt-4 flex flex-wrap gap-2">
          <Button asChild size="sm">
            <Link to="/import">{m.nav_import()}</Link>
          </Button>
        </div>
      </div>
    );
  }
  return (
    <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
      <dt className="text-muted-foreground">{m.label_name_field()}</dt>
      <dd>{data.name}</dd>
      <dt className="text-muted-foreground">{m.label_crs_field()}</dt>
      <dd className="font-mono">{data.crs}</dd>
      <dt className="text-muted-foreground">{m.label_scenarios_field()}</dt>
      <dd>{String(data.scenario_count)}</dd>
      <dt className="text-muted-foreground">{m.label_runs_field()}</dt>
      <dd>{String(data.run_count)}</dd>
    </dl>
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
      <p className="text-sm text-destructive">
        {error?.message ?? "Unknown error"}
      </p>
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
      <h2 className="text-lg font-semibold">{m.page_title_status()}</h2>

      <section className="grid gap-2">
        <h3 className="text-sm font-medium text-muted-foreground">
          {m.section_backend_health()}
        </h3>
        <QueryResult {...health} loadingText={m.status_loading_health()}>
          {(data) => <HealthSection data={data} />}
        </QueryResult>
      </section>

      <section className="grid gap-2">
        <h3 className="text-sm font-medium text-muted-foreground">
          {m.section_project()}
        </h3>
        {project.isLoading ? (
          <p className="text-sm text-muted-foreground">
            {m.status_loading_project()}
          </p>
        ) : project.isError ? (
          <p className="text-sm text-destructive">
            {project.error?.message ?? "Unknown error"}
          </p>
        ) : (
          <ProjectSection data={project.data ?? null} />
        )}
      </section>
    </div>
  );
}
