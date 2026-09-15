import {
  ArrowRight,
  FileInput,
  Play,
  ShieldCheck,
  Sparkles,
  type LucideIcon,
} from "lucide-react";
import { Link } from "react-router";
import { useProjectStatus } from "@/api/hooks";
import { Badge } from "@/ui/components/badge";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { Callout } from "@/ui/callout";
import { KeyValueList } from "@/ui/key-value-list";
import { SectionHeading } from "@/ui/page-header";
import { m } from "@/i18n/messages";

function FeatureCard({
  icon: Icon,
  title,
  description,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
}) {
  return (
    <Card className="p-5">
      <div className="flex items-start gap-3">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Icon className="size-5" aria-hidden="true" />
        </div>
        <SectionHeading description={description}>{title}</SectionHeading>
      </div>
    </Card>
  );
}

function ProjectSummary() {
  const project = useProjectStatus();

  if (project.isLoading) {
    return (
      <p className="text-sm text-muted-foreground">
        {m.status_loading_project()}
      </p>
    );
  }
  if (project.isError) {
    return <Callout variant="destructive">{project.error.message}</Callout>;
  }
  if (!project.data) {
    return (
      <Callout variant="neutral" title={m.msg_no_project_yet()}>
        {m.msg_no_project_yet_help()}
      </Callout>
    );
  }
  return (
    <KeyValueList
      items={[
        { label: m.label_name_field(), value: project.data.name },
        { label: m.label_crs_field(), value: project.data.crs, mono: true },
        {
          label: m.label_scenarios_field(),
          value: String(project.data.scenario_count),
        },
        { label: m.label_runs_field(), value: String(project.data.run_count) },
      ]}
    />
  );
}

export default function WelcomePage() {
  return (
    <div className="mx-auto flex w-full max-w-6xl flex-col gap-6 px-4 py-8 sm:px-6 lg:px-8">
      <Card className="grid gap-6 p-8 lg:grid-cols-[minmax(0,1.3fr)_minmax(18rem,0.7fr)]">
        <div className="space-y-6">
          <Badge variant="outline">
            <Sparkles aria-hidden="true" />
            {m.page_title_welcome()}
          </Badge>

          <div className="space-y-3">
            <h2 className="max-w-xl text-xl font-semibold tracking-tight">
              {m.heading_welcome()}
            </h2>
            <p className="max-w-2xl text-base text-muted-foreground">
              {m.msg_welcome_intro()}
            </p>
          </div>

          <div className="flex flex-wrap gap-3">
            <Button asChild>
              <Link to="/import">
                <FileInput aria-hidden="true" />
                {m.action_start_import()}
              </Link>
            </Button>
            <Button asChild variant="outline">
              <Link to="/model">
                <ArrowRight aria-hidden="true" />
                {m.action_open_workspace()}
              </Link>
            </Button>
          </div>
        </div>

        <div className="rounded-lg border bg-muted/40 p-5">
          <p className="text-sm font-semibold">
            {m.msg_welcome_product_summary()}
          </p>
          <p className="mt-2 text-sm text-muted-foreground">
            {m.msg_welcome_product_detail()}
          </p>
          <div className="mt-5 flex flex-wrap gap-2">
            <Badge variant="outline" className="text-muted-foreground">
              {m.msg_welcome_local_first()}
            </Badge>
            <Badge variant="outline" className="text-muted-foreground">
              {m.msg_welcome_offline_first()}
            </Badge>
            <Badge variant="outline" className="text-muted-foreground">
              {m.msg_welcome_cli_first()}
            </Badge>
          </div>
        </div>
      </Card>

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
        <Card className="space-y-3 p-6">
          <SectionHeading variant="eyebrow">
            {m.section_project()}
          </SectionHeading>
          <ProjectSummary />
        </Card>

        <Card className="space-y-3 p-6">
          <SectionHeading variant="eyebrow">
            {m.section_workspace()}
          </SectionHeading>
          <p className="text-sm text-muted-foreground">
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
        </Card>
      </section>
    </div>
  );
}
