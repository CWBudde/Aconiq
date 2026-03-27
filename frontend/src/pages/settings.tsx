import { useState } from "react";
import {
  BarChart3,
  FolderKanban,
  History,
  Languages,
  Map,
  Monitor,
  Moon,
  Play,
  Server,
  Settings,
  SlidersHorizontal,
  Sun,
  Layers3,
} from "lucide-react";
import { useLocation, useNavigate } from "react-router";
import {
  IS_WASM_MODE,
  clearAPIBaseURLOverride,
  getAPIBaseURL,
  hasAPIBaseURLOverride,
  setAPIBaseURLOverride,
} from "@/api/mode";
import { m } from "@/i18n/messages";
import {
  getLocale,
  setLocale,
  localStorageKey as localeStorageKey,
} from "@/i18n/runtime";
import { DRAFT_KEY, discardDraft, hasDraft } from "@/model/use-autosave";
import { Button } from "@/ui/components/button";
import { Separator } from "@/ui/components/separator";
import { FormField } from "@/ui/form-field";
import { useTheme } from "@/ui/theme-provider";
import { cn } from "@/ui/lib/utils";

type CategoryId =
  | "app"
  | "project"
  | "model"
  | "map"
  | "runs"
  | "results"
  | "advanced";

type Category = {
  id: CategoryId;
  icon: React.ComponentType<{ className?: string }>;
  title: () => string;
  description: () => string;
};

const CATEGORY_QUERY_KEY = "category";

function SettingsCard({
  icon: Icon,
  title,
  description,
  children,
  className,
}: {
  icon: React.ComponentType<{ className?: string }>;
  title: string;
  description: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section
      className={cn(
        "flex h-full flex-col rounded-2xl border bg-card p-5 shadow-sm",
        className,
      )}
    >
      <div className="flex items-start gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
          <Icon className="h-5 w-5" aria-hidden />
        </div>
        <div className="space-y-1">
          <h3 className="text-base font-semibold">{title}</h3>
          <p className="text-sm text-muted-foreground">{description}</p>
        </div>
      </div>
      <div className="mt-5 flex-1">{children}</div>
    </section>
  );
}

function PreferencePill({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border bg-muted/40 px-3 py-2">
      <div className="text-[11px] uppercase tracking-[0.2em] text-muted-foreground">
        {label}
      </div>
      <div className="mt-1 font-medium">{value}</div>
    </div>
  );
}

function CategoryButton({
  category,
  active,
  onClick,
}: {
  category: Category;
  active: boolean;
  onClick: () => void;
}) {
  const Icon = category.icon;

  return (
    <button
      type="button"
      onClick={onClick}
      role="tab"
      aria-selected={active}
      aria-controls={`settings-panel-${category.id}`}
      id={`settings-tab-${category.id}`}
      className={cn(
        "group flex w-full items-start gap-3 rounded-2xl border p-3 text-left transition-colors",
        active
          ? "border-primary/30 bg-primary/8 shadow-sm"
          : "border-transparent hover:border-border hover:bg-muted/40",
      )}
    >
      <div
        className={cn(
          "mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border",
          active
            ? "border-primary/20 bg-primary text-primary-foreground"
            : "border-border bg-background text-foreground",
        )}
      >
        <Icon className="h-4 w-4" aria-hidden />
      </div>
      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex items-center justify-between gap-3">
          <span className="font-medium">{category.title()}</span>
        </div>
        <p className="text-sm leading-5 text-muted-foreground">
          {category.description()}
        </p>
      </div>
    </button>
  );
}

function AppSettings({
  theme,
  setTheme,
  draftPresent,
  setDraftPresent,
  locale,
  runtimeLabel,
  themeLabel,
  localeLabel,
}: {
  theme: "dark" | "light" | "system";
  setTheme: (theme: "dark" | "light" | "system") => void;
  draftPresent: boolean;
  setDraftPresent: (value: boolean) => void;
  locale: "de" | "en";
  runtimeLabel: string;
  themeLabel: string;
  localeLabel: string;
}) {
  function clearDraft() {
    discardDraft();
    setDraftPresent(false);
  }

  return (
    <div className="space-y-6">
      <section className="overflow-hidden rounded-3xl border border-primary/15 bg-gradient-to-br from-primary/10 via-card to-card p-6 shadow-sm">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
          <div className="max-w-2xl space-y-3">
            <div className="inline-flex items-center gap-2 rounded-full border bg-background/80 px-3 py-1 text-xs font-medium text-muted-foreground">
              <Settings className="h-3.5 w-3.5" aria-hidden />
              {m.settings_category_app()}
            </div>
            <h3 className="text-2xl font-semibold tracking-tight sm:text-3xl">
              {m.settings_category_app()}
            </h3>
          </div>

          <div className="grid gap-2 sm:grid-cols-3 lg:w-[28rem]">
            <PreferencePill label={m.label_current()} value={themeLabel} />
            <PreferencePill label={m.language()} value={localeLabel} />
            <PreferencePill label={m.section_runtime()} value={runtimeLabel} />
          </div>
        </div>
      </section>

      <div className="grid gap-4 md:grid-cols-2">
        <SettingsCard
          icon={Sun}
          title={m.section_appearance()}
          description={m.msg_settings_appearance_help()}
          className="border-primary/10"
        >
          <div className="grid gap-3 sm:grid-cols-3">
            <Button
              variant={theme === "light" ? "default" : "outline"}
              aria-pressed={theme === "light"}
              onClick={() => {
                setTheme("light");
              }}
            >
              <Sun className="h-4 w-4" aria-hidden />
              {m.theme_option_light()}
            </Button>
            <Button
              variant={theme === "dark" ? "default" : "outline"}
              aria-pressed={theme === "dark"}
              onClick={() => {
                setTheme("dark");
              }}
            >
              <Moon className="h-4 w-4" aria-hidden />
              {m.theme_option_dark()}
            </Button>
            <Button
              variant={theme === "system" ? "default" : "outline"}
              aria-pressed={theme === "system"}
              onClick={() => {
                setTheme("system");
              }}
            >
              <Monitor className="h-4 w-4" aria-hidden />
              {m.theme_option_system()}
            </Button>
          </div>
        </SettingsCard>

        <SettingsCard
          icon={Languages}
          title={m.section_language()}
          description={m.msg_settings_language_help()}
        >
          <div className="grid gap-3 sm:grid-cols-2">
            <Button
              variant={locale === "en" ? "default" : "outline"}
              aria-pressed={locale === "en"}
              onClick={() => {
                void setLocale("en");
              }}
            >
              {m.language_en()}
            </Button>
            <Button
              variant={locale === "de" ? "default" : "outline"}
              aria-pressed={locale === "de"}
              onClick={() => {
                void setLocale("de");
              }}
            >
              {m.language_de()}
            </Button>
          </div>
        </SettingsCard>

        <SettingsCard
          icon={History}
          title={m.section_storage()}
          description={m.msg_settings_storage_help()}
        >
          <div className="space-y-4">
            <div className="rounded-xl border bg-background p-4">
              <p className="text-sm font-medium text-foreground">
                {draftPresent ? m.msg_draft_present() : m.msg_draft_absent()}
              </p>
              <p className="mt-2 text-sm leading-6 text-muted-foreground">
                {m.msg_settings_storage_note()}
              </p>
              <p className="mt-3 font-mono text-xs text-muted-foreground">
                {DRAFT_KEY}
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                onClick={clearDraft}
                disabled={!draftPresent}
                variant="outline"
              >
                {m.action_discard()}
              </Button>
            </div>
          </div>
        </SettingsCard>

        <SettingsCard
          icon={Server}
          title={m.section_runtime()}
          description={m.msg_settings_runtime_help()}
        >
          <div className="space-y-4">
            <div className="grid gap-3 sm:grid-cols-2">
              <PreferencePill label={m.label_current()} value={runtimeLabel} />
              <PreferencePill label={m.language()} value={localeLabel} />
            </div>
            <div className="rounded-xl border bg-background p-4 text-sm text-muted-foreground">
              <p className="leading-6">{m.msg_settings_storage_summary()}</p>
              <p className="mt-3 font-mono text-xs text-foreground">
                {localeStorageKey} · {DRAFT_KEY} · aconiq-theme
              </p>
            </div>
          </div>
        </SettingsCard>
      </div>
    </div>
  );
}

function AdvancedSettings({
  apiBaseUrl,
  apiBaseUrlDraft,
  setApiBaseUrlDraft,
  onSave,
  onReset,
  hasOverride,
}: {
  apiBaseUrl: string;
  apiBaseUrlDraft: string;
  setApiBaseUrlDraft: (value: string) => void;
  onSave: () => void;
  onReset: () => void;
  hasOverride: boolean;
}) {
  const effectiveApiBaseUrl = apiBaseUrl || "same-origin";

  return (
    <section className="overflow-hidden rounded-3xl border border-amber-200/70 bg-gradient-to-br from-amber-50 via-card to-card p-6 shadow-sm dark:border-amber-900/40 dark:from-amber-950/20">
      <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
        <div className="max-w-2xl space-y-3">
          <div className="inline-flex items-center gap-2 rounded-full border bg-background/80 px-3 py-1 text-xs font-medium text-muted-foreground">
            <SlidersHorizontal className="h-3.5 w-3.5" aria-hidden />
            {m.settings_category_advanced()}
          </div>
          <h3 className="text-2xl font-semibold tracking-tight sm:text-3xl">
            {m.settings_category_advanced()}
          </h3>
          <p className="text-sm leading-6 text-muted-foreground sm:text-base">
            {m.settings_category_advanced_desc()}
          </p>
        </div>

        <div className="grid gap-2 sm:grid-cols-2 lg:w-[30rem]">
          <PreferencePill
            label={m.label_api_base_url()}
            value={effectiveApiBaseUrl}
          />
          <PreferencePill
            label={m.msg_api_endpoint_current()}
            value={
              hasOverride
                ? m.msg_api_endpoint_override_active()
                : m.msg_api_endpoint_override_default()
            }
          />
        </div>
      </div>

      <div className="mt-6 grid gap-5 lg:grid-cols-[minmax(0,1fr)_18rem]">
        <div className="space-y-4">
          <FormField
            id="api-base-url"
            label={m.label_api_base_url()}
            hint={m.msg_api_endpoint_help()}
            value={apiBaseUrlDraft}
            onChange={(event) => {
              setApiBaseUrlDraft(event.target.value);
            }}
            placeholder={m.placeholder_api_endpoint()}
          />
          <div className="rounded-2xl border bg-background p-4">
            <p className="text-sm font-medium text-foreground">
              {m.msg_api_endpoint_current()}
            </p>
            <p className="mt-2 break-all font-mono text-sm text-muted-foreground">
              {effectiveApiBaseUrl}
            </p>
            <p className="mt-3 text-sm leading-6 text-muted-foreground">
              {m.msg_api_endpoint_note()}
            </p>
          </div>
        </div>

        <div className="space-y-3 rounded-2xl border bg-muted/30 p-4">
          <p className="text-sm font-medium text-foreground">
            {m.msg_api_endpoint_current()}
          </p>
          <div className="rounded-xl border bg-background px-3 py-2 font-mono text-xs text-foreground">
            {effectiveApiBaseUrl}
          </div>
          <div className="space-y-2 pt-2">
            <Button
              className="w-full"
              onClick={onSave}
              disabled={
                apiBaseUrlDraft.trim().replace(/\/$/, "") === apiBaseUrl
              }
            >
              {m.action_save_changes()}
            </Button>
            <Button
              className="w-full"
              variant="outline"
              onClick={onReset}
              disabled={!hasOverride}
            >
              {m.action_reset_to_default()}
            </Button>
          </div>
        </div>
      </div>
    </section>
  );
}

function PlannedCategory({ category }: { category: Category }) {
  const Icon = category.icon;

  return (
    <section className="rounded-3xl border bg-card/90 p-6 shadow-sm">
      <div className="flex items-start gap-4">
        <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-muted text-foreground">
          <Icon className="h-5 w-5" aria-hidden />
        </div>
        <div className="space-y-2">
          <h3 className="text-2xl font-semibold tracking-tight">
            {category.title()}
          </h3>
          <p className="max-w-2xl text-sm leading-6 text-muted-foreground">
            {category.description()}
          </p>
        </div>
      </div>

      <div className="mt-6 rounded-2xl border border-dashed bg-muted/30 p-6">
        <p className="text-sm leading-6 text-muted-foreground">
          This category is reserved for future settings in the product and will
          live here once those controls are defined.
        </p>
      </div>
    </section>
  );
}

export default function SettingsPage() {
  const { theme, setTheme } = useTheme();
  const locale = getLocale();
  const location = useLocation();
  const navigate = useNavigate();
  const [draftPresent, setDraftPresent] = useState(hasDraft());
  const [apiBaseUrl, setApiBaseUrl] = useState(() => getAPIBaseURL());
  const [apiBaseUrlDraft, setApiBaseUrlDraft] = useState(() => getAPIBaseURL());
  const [apiBaseUrlOverridePresent, setApiBaseUrlOverridePresent] = useState(
    () => hasAPIBaseURLOverride(),
  );

  const visibleApiBaseUrl = apiBaseUrl || "same-origin";
  const runtimeLabel = IS_WASM_MODE
    ? m.msg_runtime_wasm()
    : m.msg_runtime_api();
  const localeLabel = locale === "de" ? m.language_de() : m.language_en();
  const themeLabel =
    theme === "light"
      ? m.theme_option_light()
      : theme === "dark"
        ? m.theme_option_dark()
        : m.theme_option_system();

  const categories: Category[] = [
    {
      id: "app",
      icon: Settings,
      title: m.settings_category_app,
      description: m.settings_category_app_desc,
    },
    {
      id: "project",
      icon: FolderKanban,
      title: m.settings_category_project,
      description: m.settings_category_project_desc,
    },
    {
      id: "model",
      icon: Layers3,
      title: m.settings_category_model,
      description: m.settings_category_model_desc,
    },
    {
      id: "map",
      icon: Map,
      title: m.settings_category_map,
      description: m.settings_category_map_desc,
    },
    {
      id: "runs",
      icon: Play,
      title: m.settings_category_runs,
      description: m.settings_category_runs_desc,
    },
    {
      id: "results",
      icon: BarChart3,
      title: m.settings_category_results,
      description: m.settings_category_results_desc,
    },
    {
      id: "advanced",
      icon: SlidersHorizontal,
      title: m.settings_category_advanced,
      description: m.settings_category_advanced_desc,
    },
  ];

  const searchParams = new URLSearchParams(location.search);
  const requestedCategory = searchParams.get(CATEGORY_QUERY_KEY);
  const activeCategory = categories.some(
    (category) => category.id === requestedCategory,
  )
    ? (requestedCategory as CategoryId)
    : "app";
  const active = categories.find((category) => category.id === activeCategory);

  function setActiveCategory(categoryId: CategoryId) {
    const nextParams = new URLSearchParams(location.search);
    if (categoryId === "app") {
      nextParams.delete(CATEGORY_QUERY_KEY);
    } else {
      nextParams.set(CATEGORY_QUERY_KEY, categoryId);
    }
    const nextSearch = nextParams.toString();
    void navigate(
      {
        pathname: location.pathname,
        search: nextSearch ? `?${nextSearch}` : "",
      },
      { replace: true },
    );
  }

  function saveApiBaseUrl() {
    const normalized = apiBaseUrlDraft.trim().replace(/\/$/, "");
    setAPIBaseURLOverride(normalized);
    const next = getAPIBaseURL();
    setApiBaseUrl(next);
    setApiBaseUrlDraft(next);
    setApiBaseUrlOverridePresent(hasAPIBaseURLOverride());
  }

  function resetApiBaseUrl() {
    clearAPIBaseURLOverride();
    const next = getAPIBaseURL();
    setApiBaseUrl(next);
    setApiBaseUrlDraft(next);
    setApiBaseUrlOverridePresent(hasAPIBaseURLOverride());
  }

  return (
    <div className="relative flex flex-1 overflow-hidden">
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 top-0 h-72 bg-[radial-gradient(circle_at_top_left,rgba(20,184,166,0.16),transparent_35%),radial-gradient(circle_at_top_right,rgba(15,23,42,0.08),transparent_30%)]"
      />

      <div className="relative mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">
        <div className="grid gap-6 lg:grid-cols-[18rem_minmax(0,1fr)]">
          <aside className="rounded-3xl border bg-card/80 p-3 shadow-sm">
            <div className="px-3 pb-3 pt-2">
              <div className="text-xs font-medium uppercase tracking-[0.25em] text-muted-foreground">
                {m.section_settings_categories()}
              </div>
            </div>
            <div
              className="space-y-2"
              role="tablist"
              aria-orientation="vertical"
            >
              {categories.map((category) => (
                <CategoryButton
                  key={category.id}
                  category={category}
                  active={activeCategory === category.id}
                  onClick={() => {
                    setActiveCategory(category.id);
                  }}
                />
              ))}
            </div>
          </aside>

          <main className="min-w-0">
            {active?.id === "app" ? (
              <div
                role="tabpanel"
                id={`settings-panel-${active.id}`}
                aria-labelledby={`settings-tab-${active.id}`}
              >
                <AppSettings
                  theme={theme}
                  setTheme={setTheme}
                  draftPresent={draftPresent}
                  setDraftPresent={setDraftPresent}
                  locale={locale}
                  runtimeLabel={runtimeLabel}
                  themeLabel={themeLabel}
                  localeLabel={localeLabel}
                />
              </div>
            ) : active?.id === "advanced" ? (
              <div
                role="tabpanel"
                id={`settings-panel-${active.id}`}
                aria-labelledby={`settings-tab-${active.id}`}
              >
                <AdvancedSettings
                  apiBaseUrl={visibleApiBaseUrl}
                  apiBaseUrlDraft={apiBaseUrlDraft}
                  setApiBaseUrlDraft={setApiBaseUrlDraft}
                  onSave={saveApiBaseUrl}
                  onReset={resetApiBaseUrl}
                  hasOverride={apiBaseUrlOverridePresent}
                />
              </div>
            ) : active ? (
              <div
                role="tabpanel"
                id={`settings-panel-${active.id}`}
                aria-labelledby={`settings-tab-${active.id}`}
              >
                <PlannedCategory category={active} />
              </div>
            ) : null}
          </main>
        </div>
      </div>
    </div>
  );
}
