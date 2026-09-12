import {
  Map,
  FileInput,
  Play,
  BarChart3,
  FileOutput,
  Settings,
  Activity,
} from "lucide-react";
import { Link, useLocation } from "react-router";
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

const navMain = [
  { title: m.nav_map, icon: Map, path: "/map" },
  { title: m.nav_import, icon: FileInput, path: "/import" },
  { title: m.nav_run, icon: Play, path: "/run" },
  { title: m.nav_results, icon: BarChart3, path: "/results" },
  { title: m.nav_export, icon: FileOutput, path: "/export" },
];

const navFooter = [
  { title: m.nav_status, icon: Activity, path: "/status" },
  { title: m.nav_settings, icon: Settings, path: "/settings" },
];

function AppSidebar() {
  const location = useLocation();
  const project = useProjectStatus();
  const showWorkspaceNav = project.data != null;
  // navMain[1] is the "Import" entry; slice keeps the element type non-optional
  const workspaceNav = showWorkspaceNav ? navMain : navMain.slice(1, 2);

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
                  <SidebarMenuItem key={item.path}>
                    <SidebarMenuButton
                      asChild
                      isActive={location.pathname === item.path}
                    >
                      <Link
                        to={item.path}
                        aria-current={
                          location.pathname === item.path ? "page" : undefined
                        }
                      >
                        <item.icon className="h-4 w-4" />
                        <span>{item.title()}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
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
                  <SidebarMenuItem key={item.path}>
                    <SidebarMenuButton
                      asChild
                      isActive={location.pathname === item.path}
                    >
                      <Link
                        to={item.path}
                        aria-current={
                          location.pathname === item.path ? "page" : undefined
                        }
                      >
                        <item.icon className="h-4 w-4" />
                        <span>{item.title()}</span>
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
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
  const allNav = [...navMain, ...navFooter];
  const current = allNav.find((item) => item.path === location.pathname);
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
