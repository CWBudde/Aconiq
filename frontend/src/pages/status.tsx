import { useHealth, useProjectStatus } from "@/api";
import type { HealthResponse, ProjectStatusResponse } from "@/api";
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

function ProjectSection({ data }: { data: ProjectStatusResponse }) {
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
  data: T | undefined;
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
  if (!data) {
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
        <QueryResult {...project} loadingText={m.status_loading_project()}>
          {(data) => <ProjectSection data={data} />}
        </QueryResult>
      </section>
    </div>
  );
}
