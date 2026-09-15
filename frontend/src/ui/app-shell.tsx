import {
  Map,
  FileInput,
  Play,
  BarChart3,
  FileOutput,
  Settings,
  Activity,
} from "lucide-react";
import { Link, matchPath, useLocation, useMatch } from "react-router";
import type { LucideIcon } from "lucide-react";
import { useProjectStatus } from "@/api/hooks";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
} from "@/ui/components/sidebar";
import { Separator } from "@/ui/components/separator";
import { ThemeToggle } from "@/ui/theme-toggle";
import { LanguageToggle } from "@/ui/language-toggle";
import { SaveStatus } from "@/ui/save-status";
import { m } from "@/i18n/messages";

interface NavEntry {
  title: () => string;
  icon: LucideIcon;
  path: string;
  /**
   * The route pattern this entry answers "is this the current page?" with,
   * when the link target alone does not describe it. It mirrors
   * `routes.tsx`, so the rail lights for exactly the URLs that render the
   * section and for no others: a bare prefix match would also claim
   * `/settings/typo` and `/results/a/b`, which the catch-all renders as the
   * not-found page. Defaults to `path`, matched whole.
   */
  match?: string;
}

const importNav: NavEntry = {
  title: m.nav_import,
  icon: FileInput,
  path: "/import",
};

const navMain: NavEntry[] = [
  { title: m.nav_model, icon: Map, path: "/model" },
  importNav,
  { title: m.nav_run, icon: Play, path: "/run" },
  // The two parameterised sections: `/results` and `/results/<id>` are the
  // section, `/results/a/b` is not.
  {
    title: m.nav_results,
    icon: BarChart3,
    path: "/results",
    match: "/results/:runId?",
  },
  {
    title: m.nav_export,
    icon: FileOutput,
    path: "/export",
    match: "/export/:runId?",
  },
];

const navFooter: NavEntry[] = [
  { title: m.nav_status, icon: Activity, path: "/status" },
  { title: m.nav_settings, icon: Settings, path: "/settings" },
];

/**
 * A rail link, and the one place the "is this the current page?" question is
 * answered. `useMatch` matches the pattern whole and on segment boundaries, so
 * `/results/:runId?` is current for `/results` and `/results/<id>` but for
 * neither `/results-archive` nor `/results/a/b` — an equality test marked the
 * run URL as nothing, and a prefix match marks the not-found page as Results.
 * It is a hook, which is why this is a component rather than a helper called
 * inside `.map()`.
 */
function NavItem({ item }: { item: NavEntry }) {
  const match = useMatch(item.match ?? item.path);
  const current = match !== null;
  return (
    <SidebarMenuItem>
      <SidebarMenuButton asChild isActive={current}>
        <Link to={item.path} aria-current={current ? "page" : undefined}>
          <item.icon className="h-4 w-4" />
          <span>{item.title()}</span>
        </Link>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

function AppSidebar() {
  const project = useProjectStatus();
  const showWorkspaceNav = project.data != null;
  // Named entries, not an index range: with `navMain.slice(1, 2)` here,
  // inserting a rail item silently re-aimed the no-project rail at whatever
  // landed at index 1.
  const workspaceNav = showWorkspaceNav ? navMain : [importNav];

  // One <nav> landmark holds the logo, the workspace links and the footer
  // links, so nothing in the rail sits outside a landmark (axe `region`).
  return (
    <Sidebar collapsible="icon">
      <nav
        aria-label={m.nav_primary_label()}
        className="flex h-full min-h-0 w-full flex-col"
      >
        <SidebarHeader className="border-b border-sidebar-border px-4 py-3">
          <div className="flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary text-primary-foreground text-xs font-bold">
              AQ
            </div>
            <span className="text-sm font-semibold tracking-tight group-data-[collapsible=icon]:hidden">
              AconiQ
            </span>
          </div>
        </SidebarHeader>

        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupLabel>{m.section_workspace()}</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {workspaceNav.map((item) => (
                  <NavItem key={item.path} item={item} />
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>

        <SidebarFooter>
          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu>
                {navFooter.map((item) => (
                  <NavItem key={item.path} item={item} />
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarFooter>
      </nav>
    </Sidebar>
  );
}

function PageTitle() {
  const location = useLocation();
  // The same matcher the rail uses, so the heading and the current link can
  // never disagree. An equality lookup titled every parameterised path
  // "Workspace" the moment a run id appeared in the URL.
  const allNav = [...navMain, ...navFooter];
  const current = allNav.find((item) =>
    matchPath(item.match ?? item.path, location.pathname),
  );
  if (location.pathname === "/welcome") {
    return (
      <h1 className="text-sm font-medium text-muted-foreground">
        {m.page_title_welcome()}
      </h1>
    );
  }
  return (
    <h1 className="text-sm font-medium text-muted-foreground">
      {current ? current.title() : m.nav_workspace()}
    </h1>
  );
}

/** The id the skip link targets; the content wrapper below carries it. */
const MAIN_CONTENT_ID = "main-content";

export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <SidebarProvider>
      {/* First focusable element in the document: keyboard users jump past
          the rail to the page. Visually hidden until it has focus. */}
      <a
        href={`#${MAIN_CONTENT_ID}`}
        className="sr-only focus:not-sr-only focus:fixed focus:top-2 focus:left-2 focus:z-50 focus:rounded-md focus:bg-background focus:px-3 focus:py-2 focus:ring-2 focus:ring-ring"
      >
        {m.action_skip_to_content()}
      </a>
      <AppSidebar />
      {/* SidebarInset is the document's one <main>; the wrapper below is a
          plain div so no second main landmark nests inside it. */}
      <SidebarInset>
        <header className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
          <SidebarTrigger className="-ml-1" />
          <Separator orientation="vertical" className="mr-2 h-4" />
          <div className="flex flex-1 items-center justify-between">
            <PageTitle />
            <div className="flex items-center gap-1">
              <SaveStatus />
              <LanguageToggle />
              <ThemeToggle />
            </div>
          </div>
        </header>
        <div
          id={MAIN_CONTENT_ID}
          tabIndex={-1}
          className="flex flex-1 flex-col outline-none"
        >
          {children}
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
