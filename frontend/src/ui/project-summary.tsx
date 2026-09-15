import type * as React from "react";
import { Callout } from "@/ui/callout";
import { KeyValueList } from "@/ui/key-value-list";
import { useProjectStatus } from "@/api/hooks";
import type { ProjectStatusResponse } from "@/api/client";
import { m } from "@/i18n/messages";

export interface ProjectSummaryProps {
  /**
   * Rendered inside the "no project yet" callout — usually a link to the
   * importer, which is the only thing that can create one from here.
   */
  emptyAction?: React.ReactNode;
  className?: string;
}

/**
 * The project's identity and size. Deliberately four rows: `project_path`,
 * `manifest_version`, `last_run` and the model hash are all carried by
 * `ProjectStatusResponse` and none of them is rendered anywhere, because the
 * hash is a receipt the client never recomputes and the rest would cost a
 * label in both catalogues for a value nobody reads.
 *
 * This used to exist three times over — on the welcome page, the status page
 * and the map page — with the three copies already disagreeing about what to
 * show while the request was in flight.
 */
export function ProjectFacts({
  project,
}: {
  project: ProjectStatusResponse;
}): React.ReactNode {
  return (
    <KeyValueList
      items={[
        { label: m.label_name_field(), value: project.name },
        { label: m.label_crs_field(), value: project.crs, mono: true },
        {
          label: m.label_scenarios_field(),
          value: String(project.scenario_count),
        },
        { label: m.label_runs_field(), value: String(project.run_count) },
      ]}
    />
  );
}

/** `ProjectFacts` plus the three states the request can be in. */
export function ProjectSummary({
  emptyAction,
  className,
}: ProjectSummaryProps): React.ReactNode {
  const project = useProjectStatus();

  if (project.isLoading) {
    return (
      <p className={className ?? "text-sm text-muted-foreground"}>
        {m.status_loading_project()}
      </p>
    );
  }
  if (project.isError) {
    return (
      <Callout variant="destructive" className={className}>
        {project.error.message}
      </Callout>
    );
  }
  if (!project.data) {
    return (
      <Callout
        variant="neutral"
        title={m.msg_no_project_yet()}
        className={className}
      >
        {m.msg_no_project_yet_help()}
        {emptyAction != null ? <div className="mt-3">{emptyAction}</div> : null}
      </Callout>
    );
  }
  return <ProjectFacts project={project.data} />;
}
