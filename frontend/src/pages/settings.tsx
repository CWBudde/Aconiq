import { useState } from "react";
import {
  History,
  Languages,
  Monitor,
  Moon,
  Server,
  Settings,
  SlidersHorizontal,
  Sun,
  type LucideIcon,
} from "lucide-react";
import { useLocation, useNavigate } from "react-router";
import { backend } from "@/api/backend";
import {
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
import { Card } from "@/ui/components/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/ui/components/tabs";
import { Callout } from "@/ui/callout";
import { FormField } from "@/ui/form-field";
import { PageHeader, SectionHeading } from "@/ui/page-header";
import { useTheme } from "@/ui/theme-provider";
import { cn } from "@/ui/lib/utils";

/**
 * The ids are a URL contract: they are what `?category=` carries, so they
 * outlive the labels above them. "app" is General and "advanced" is
 * Connection; renaming either would break a bookmark to buy a prettier URL.
 */
type CategoryId = "app" | "advanced";

type Category = {
  id: CategoryId;
  icon: LucideIcon;
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
  icon: LucideIcon;
  title: string;
  description: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <Card className={cn("flex h-full flex-col p-5", className)}>
      <div className="flex items-start gap-3">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
          <Icon className="size-5" aria-hidden="true" />
        </div>
        <SectionHeading description={description}>{title}</SectionHeading>
      </div>
      <div className="mt-5 flex-1">{children}</div>
    </Card>
  );
}

function PreferencePill({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border bg-muted/40 px-3 py-2">
      <div className="text-2xs font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </div>
      <div className="mt-1 text-sm font-medium">{value}</div>
    </div>
  );
}

// A vertical, card-like tab: Radix owns the `tab` role, the selected state
// and the arrow-key navigation; the trigger only restyles the strip's
// horizontal defaults into a stacked list.
function CategoryTab({ category }: { category: Category }) {
  const Icon = category.icon;

  return (
    <TabsTrigger
      value={category.id}
      className="group h-auto w-full items-start justify-start gap-3 whitespace-normal rounded-md border border-transparent p-3 text-left hover:border-border hover:bg-muted/40 data-[state=active]:border-primary/30 data-[state=active]:bg-primary/10 data-[state=active]:shadow-sm"
    >
      <span className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-md border bg-background text-foreground group-data-[state=active]:border-primary/20 group-data-[state=active]:bg-primary group-data-[state=active]:text-primary-foreground">
        <Icon aria-hidden="true" />
      </span>
      <span className="min-w-0 flex-1 space-y-1">
        <span className="block font-medium">{category.title()}</span>
        <span className="block text-sm font-normal text-muted-foreground">
          {category.description()}
        </span>
      </span>
    </TabsTrigger>
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
      <Card className="p-6">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
          <PageHeader
            title={m.settings_category_app()}
            description={m.settings_category_app_desc()}
          />
          <div className="grid gap-2 sm:grid-cols-3 lg:w-[28rem]">
            <PreferencePill label={m.label_current()} value={themeLabel} />
            <PreferencePill label={m.language()} value={localeLabel} />
            <PreferencePill label={m.section_runtime()} value={runtimeLabel} />
          </div>
        </div>
      </Card>

      <div className="grid gap-4 md:grid-cols-2">
        <SettingsCard
          icon={Sun}
          title={m.section_appearance()}
          description={m.msg_settings_appearance_help()}
        >
          <div className="grid gap-3 sm:grid-cols-3">
            <Button
              variant={theme === "light" ? "default" : "outline"}
              aria-pressed={theme === "light"}
              onClick={() => {
                setTheme("light");
              }}
            >
              <Sun aria-hidden="true" />
              {m.theme_option_light()}
            </Button>
            <Button
              variant={theme === "dark" ? "default" : "outline"}
              aria-pressed={theme === "dark"}
              onClick={() => {
                setTheme("dark");
              }}
            >
              <Moon aria-hidden="true" />
              {m.theme_option_dark()}
            </Button>
            <Button
              variant={theme === "system" ? "default" : "outline"}
              aria-pressed={theme === "system"}
              onClick={() => {
                setTheme("system");
              }}
            >
              <Monitor aria-hidden="true" />
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
            <div className="rounded-md border p-4">
              <p className="text-sm font-medium">
                {draftPresent ? m.msg_draft_present() : m.msg_draft_absent()}
              </p>
              <p className="mt-2 text-sm text-muted-foreground">
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
            <div className="rounded-md border p-4 text-sm text-muted-foreground">
              <p>{m.msg_settings_storage_summary()}</p>
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
    <Card className="p-6">
      <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
        <PageHeader
          title={m.settings_category_advanced()}
          description={m.settings_category_advanced_desc()}
        />
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
          <Callout variant="neutral" title={m.msg_api_endpoint_current()}>
            <p className="break-all font-mono text-foreground">
              {effectiveApiBaseUrl}
            </p>
            <p className="mt-2">{m.msg_api_endpoint_note()}</p>
          </Callout>
        </div>

        <div className="space-y-2 self-start rounded-md border p-4">
          <Button
            className="w-full"
            onClick={onSave}
            disabled={apiBaseUrlDraft.trim().replace(/\/$/, "") === apiBaseUrl}
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
    </Card>
  );
}

export default function SettingsPage() {
  const { theme, setTheme } = useTheme();
  const locale = getLocale();
  const location = useLocation();
  const navigate = useNavigate();
  const [draftPresent, setDraftPresent] = useState(() => hasDraft());
  const [apiBaseUrl, setApiBaseUrl] = useState(() => getAPIBaseURL());
  const [apiBaseUrlDraft, setApiBaseUrlDraft] = useState(() => getAPIBaseURL());
  const [apiBaseUrlOverridePresent, setApiBaseUrlOverridePresent] = useState(
    () => hasAPIBaseURLOverride(),
  );

  const visibleApiBaseUrl = apiBaseUrl || "same-origin";
  const runtimeLabel =
    backend.capabilities.kind === "browser"
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
      id: "advanced",
      icon: SlidersHorizontal,
      title: m.settings_category_advanced,
      description: m.settings_category_advanced_desc,
    },
  ];

  const findCategory = (id: string | null) =>
    categories.find((category) => category.id === id);

  const searchParams = new URLSearchParams(location.search);
  const activeCategory: CategoryId =
    findCategory(searchParams.get(CATEGORY_QUERY_KEY))?.id ?? "app";

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
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">
      {/* The active category lives in the URL, so the tabs are controlled:
          a trigger navigates, and the route decides what is selected. */}
      <Tabs
        value={activeCategory}
        onValueChange={(value) => {
          const category = findCategory(value);
          if (category) setActiveCategory(category.id);
        }}
        orientation="vertical"
        className="grid gap-6 lg:grid-cols-[18rem_minmax(0,1fr)]"
      >
        <Card className="self-start p-3">
          <div className="px-3 pb-3 pt-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            {m.section_settings_categories()}
          </div>
          <TabsList className="flex h-auto flex-col items-stretch gap-1 bg-transparent p-0 text-foreground">
            {categories.map((category) => (
              <CategoryTab key={category.id} category={category} />
            ))}
          </TabsList>
        </Card>

        {/* A div, not <main>: the shell already provides the document's
            one main landmark and a nested one fails axe. */}
        <div className="min-w-0">
          <TabsContent value="app" className="mt-0">
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
          </TabsContent>
          <TabsContent value="advanced" className="mt-0">
            <AdvancedSettings
              apiBaseUrl={visibleApiBaseUrl}
              apiBaseUrlDraft={apiBaseUrlDraft}
              setApiBaseUrlDraft={setApiBaseUrlDraft}
              onSave={saveApiBaseUrl}
              onReset={resetApiBaseUrl}
              hasOverride={apiBaseUrlOverridePresent}
            />
          </TabsContent>
        </div>
      </Tabs>
    </div>
  );
}
