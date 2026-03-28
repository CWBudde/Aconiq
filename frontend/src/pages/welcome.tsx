import {
  ArrowRight,
  FileInput,
  Play,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import { Link } from "react-router";
import { useProjectStatus } from "@/api";
import { Button } from "@/ui/components/button";
import { m } from "@/i18n/messages";

function FeatureCard({
  icon: Icon,
  title,
  description,
}: {
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  description: string;
}) {
  return (
    <div className="rounded-2xl border bg-card p-5 shadow-sm">
      <div className="flex items-start gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
          <Icon className="h-5 w-5" aria-hidden />
        </div>
        <div className="space-y-1">
          <h3 className="text-sm font-semibold">{title}</h3>
          <p className="text-sm leading-6 text-muted-foreground">
            {description}
          </p>
        </div>
      </div>
    </div>
  );
}

export default function WelcomePage() {
  const project = useProjectStatus();

  return (
    <div className="relative flex flex-1 overflow-hidden">
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 top-0 h-72 bg-[radial-gradient(circle_at_top_left,rgba(20,184,166,0.18),transparent_34%),radial-gradient(circle_at_top_right,rgba(15,23,42,0.08),transparent_30%)]"
      />

      <div className="relative mx-auto flex w-full max-w-6xl flex-col gap-6 px-4 py-8 sm:px-6 lg:px-8">
        <section className="grid gap-6 rounded-3xl border bg-card/95 p-8 shadow-sm backdrop-blur lg:grid-cols-[minmax(0,1.3fr)_minmax(18rem,0.7fr)]">
          <div className="space-y-6">
            <div className="inline-flex items-center gap-2 rounded-full border bg-background/80 px-3 py-1 text-xs font-medium text-muted-foreground">
              <Sparkles className="h-3.5 w-3.5" aria-hidden />
              {m.page_title_welcome()}
            </div>

            <div className="space-y-4">
              <h2 className="max-w-xl text-4xl font-semibold tracking-tight sm:text-5xl">
                {m.heading_welcome()}
              </h2>
              <p className="max-w-2xl text-base leading-7 text-muted-foreground">
                {m.msg_welcome_intro()}
              </p>
            </div>

            <div className="flex flex-wrap gap-3">
              <Button asChild>
                <Link to="/import">
                  <FileInput className="h-4 w-4" aria-hidden />
                  {m.action_start_import()}
                </Link>
              </Button>
              <Button asChild variant="outline">
                <Link to="/map">
                  <ArrowRight className="h-4 w-4" aria-hidden />
                  {m.action_open_workspace()}
                </Link>
              </Button>
            </div>
          </div>

          <div className="rounded-2xl border bg-muted/40 p-5">
            <p className="text-sm font-semibold text-foreground">
              {m.msg_welcome_product_summary()}
            </p>
            <p className="mt-2 text-sm leading-6 text-muted-foreground">
              {m.msg_welcome_product_detail()}
            </p>
            <div className="mt-5 flex flex-wrap gap-2">
              <span className="rounded-full border bg-background px-3 py-1 text-xs text-muted-foreground">
                {m.msg_welcome_local_first()}
              </span>
              <span className="rounded-full border bg-background px-3 py-1 text-xs text-muted-foreground">
                {m.msg_welcome_offline_first()}
              </span>
              <span className="rounded-full border bg-background px-3 py-1 text-xs text-muted-foreground">
                {m.msg_welcome_cli_first()}
              </span>
            </div>
          </div>
        </section>

        <section className="grid gap-4 md:grid-cols-3">
          <FeatureCard
            icon={FileInput}
            title={m.msg_welcome_step_import_title()}
            description={m.msg_welcome_step_import_desc()}
          />
          <FeatureCard
            icon={ShieldCheck}
            title={m.msg_welcome_step_validate_title()}
            description={m.msg_welcome_step_validate_desc()}
          />
          <FeatureCard
            icon={Play}
            title={m.msg_welcome_step_run_title()}
            description={m.msg_welcome_step_run_desc()}
          />
        </section>

        <section className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_18rem]">
          <div className="rounded-3xl border bg-card p-6 shadow-sm">
            <div className="space-y-2">
              <h3 className="text-sm font-semibold uppercase tracking-[0.2em] text-muted-foreground">
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
              ) : project.data ? (
                <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 text-sm">
                  <dt className="text-muted-foreground">
                    {m.label_name_field()}
                  </dt>
                  <dd>{project.data.name}</dd>
                  <dt className="text-muted-foreground">
                    {m.label_crs_field()}
                  </dt>
                  <dd className="font-mono">{project.data.crs}</dd>
                  <dt className="text-muted-foreground">
                    {m.label_scenarios_field()}
                  </dt>
                  <dd>{String(project.data.scenario_count)}</dd>
                  <dt className="text-muted-foreground">
                    {m.label_runs_field()}
                  </dt>
                  <dd>{String(project.data.run_count)}</dd>
                </dl>
              ) : (
                <div className="rounded-2xl border bg-muted/30 p-4">
                  <p className="text-sm font-medium">
                    {m.msg_no_project_yet()}
                  </p>
                  <p className="mt-2 text-sm leading-6 text-muted-foreground">
                    {m.msg_no_project_yet_help()}
                  </p>
                </div>
              )}
            </div>
          </div>

          <div className="rounded-3xl border bg-card p-6 shadow-sm">
            <div className="space-y-3">
              <h3 className="text-sm font-semibold uppercase tracking-[0.2em] text-muted-foreground">
                {m.section_workspace()}
              </h3>
              <p className="text-sm leading-6 text-muted-foreground">
                {m.msg_welcome_workspace_detail()}
              </p>
              <div className="flex flex-col gap-2 pt-2">
                <Button asChild variant="outline" className="justify-start">
                  <Link to="/status">{m.nav_status()}</Link>
                </Button>
                <Button asChild variant="outline" className="justify-start">
                  <Link to="/settings">{m.nav_settings()}</Link>
                </Button>
              </div>
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}
