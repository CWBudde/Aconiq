import { useState } from "react";
import {
  Check,
  History,
  Languages,
  Layers,
  Map,
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
import {
  DEFAULT_TILE_URL,
  clearTileURLOverride,
  getTileURL,
  hasTileURLOverride,
  setTileURLOverride,
} from "@/map/tile-source";
import { BASEMAP_IDS, type BasemapId, basemapLabel } from "@/map/basemap";
import { MODEL_LAYER_GROUPS, RESULT_LAYER_GROUPS } from "@/map/layers";
import { useMapStore } from "@/map/map-store";
import { m } from "@/i18n/messages";
import { changeLocale, useLocale } from "@/locale";
import { localStorageKey as localeStorageKey } from "@/i18n/runtime";
import { DRAFT_KEY, discardDraft, hasDraft } from "@/model/use-autosave";
import { Button } from "@/ui/components/button";
import { Card } from "@/ui/components/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/ui/components/tabs";
import { Callout } from "@/ui/callout";
import { FormField } from "@/ui/form-field";
import { PageHeader, SectionHeading } from "@/ui/page-header";
import { THEME_STORAGE_KEY, useTheme } from "@/ui/theme-provider";
import { cn } from "@/ui/lib/utils";

/**
 * The ids are a URL contract: they are what `?category=` carries, so they
 * outlive the labels above them. "app" is General, "advanced" is Connection
 * and "map" is Karte; renaming any of them would break a bookmark to buy a
 * prettier URL. Adding one, as "map" was, costs nothing — an id that meant
 * nothing before fell back to General, and no existing link changes meaning.
 */
type CategoryId = "app" | "advanced" | "map";

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
                changeLocale("en");
              }}
            >
              {m.language_en()}
            </Button>
            <Button
              variant={locale === "de" ? "default" : "outline"}
              aria-pressed={locale === "de"}
              onClick={() => {
                changeLocale("de");
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
              {/* The keys come from the modules that write them, and the
                  sentence is the only place they are listed: spelling one of
                  them into the catalogue as well puts a copy in two files
                  that no rename would keep in step. */}
              <p>
                {m.msg_settings_storage_summary({
                  themeKey: THEME_STORAGE_KEY,
                  localeKey: localeStorageKey,
                  draftKey: DRAFT_KEY,
                })}
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

      {/* The group is kept although this card is down to one setting: it is
          what names the Save and Reset buttons for a screen reader, and the
          Karte tab's tile URL carries the same pair of labels. Two tabs is not
          two documents to someone reading the accessibility tree of whichever
          one is open, but the label costs nothing and the rule it follows —
          every Save button says what it saves — is worth keeping local. */}
      <div
        role="group"
        aria-label={m.label_api_base_url()}
        className="mt-6 grid gap-5 lg:grid-cols-[minmax(0,1fr)_18rem]"
      >
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

/**
 * Everything about how the map is drawn, in one place.
 *
 * The basemap used to be reachable only from the panel on the map, and the
 * tile URL sat on the Connection tab beside the API base URL as though it were
 * a backend endpoint. It is not: it says how the map looks, not which
 * computation answers. Connection is left meaning one thing.
 *
 * The picker on the map stays. Both write the same store, so they cannot
 * drift, and a basemap is judged by looking at a map rather than at a list of
 * three words.
 */
function MapSettings({
  basemap,
  setBasemap,
  hiddenLayerCount,
  onResetLayers,
  tileUrl,
  tileUrlDraft,
  setTileUrlDraft,
  onSaveTileUrl,
  onResetTileUrl,
  hasTileOverride,
}: {
  basemap: BasemapId;
  setBasemap: (id: BasemapId) => void;
  hiddenLayerCount: number;
  onResetLayers: () => void;
  tileUrl: string;
  tileUrlDraft: string;
  setTileUrlDraft: (value: string) => void;
  onSaveTileUrl: () => void;
  onResetTileUrl: () => void;
  hasTileOverride: boolean;
}) {
  return (
    <div className="space-y-6">
      <Card className="p-6">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
          <PageHeader
            title={m.settings_category_map()}
            description={m.settings_category_map_desc()}
          />
          <div className="grid gap-2 sm:grid-cols-2 lg:w-[30rem]">
            <PreferencePill
              label={m.section_basemap()}
              value={basemapLabel(basemap)}
            />
            <PreferencePill
              label={m.msg_basemap_tile_url_current()}
              value={
                hasTileOverride
                  ? m.msg_api_endpoint_override_active()
                  : m.msg_api_endpoint_override_default()
              }
            />
          </div>
        </div>
      </Card>

      <div className="grid gap-4 md:grid-cols-2">
        {/* The same shape as the theme card on the General tab: a few mutually
            exclusive choices, all visible, marked with `aria-pressed` and the
            filled variant rather than hidden behind a select.

            The picker on the map cannot carry a tick — `map/layer-control.tsx`
            records why at length, and the reason is that panel's geometry, not
            a stylistic rule: equal grid columns sized to the widest cell plus a
            content-sized panel mean an icon widens the whole panel by a
            different amount per label and pushes it into the feature-count
            pill beside it. None of that applies to a settings card, which has
            room, so here the mark is explicit. */}
        <SettingsCard
          icon={Map}
          title={m.section_basemap()}
          description={m.msg_settings_basemap_help()}
        >
          {/* Grouped and labelled the way the picker on the map is: three
              buttons that are one choice, not three independent toggles. */}
          <div
            role="group"
            aria-label={m.section_basemap()}
            className="grid gap-3 sm:grid-cols-3"
          >
            {BASEMAP_IDS.map((id) => {
              const active = basemap === id;
              return (
                <Button
                  key={id}
                  variant={active ? "default" : "outline"}
                  aria-pressed={active}
                  onClick={() => {
                    setBasemap(id);
                  }}
                >
                  {active ? <Check aria-hidden="true" /> : null}
                  {basemapLabel(id)}
                </Button>
              );
            })}
          </div>
        </SettingsCard>

        {/* The escape hatch that persisting the layer toggles made necessary.
            The ten switches themselves stay on the map, where the thing they
            hide is visible; duplicating them here would be a second write path
            to keep in step for no gain. */}
        <SettingsCard
          icon={Layers}
          title={m.section_model()}
          description={m.msg_settings_map_layers_help()}
        >
          <div className="space-y-4">
            <div className="rounded-md border p-4">
              <p className="text-sm font-medium">
                {hiddenLayerCount > 0
                  ? m.msg_settings_layers_hidden({ count: hiddenLayerCount })
                  : m.msg_settings_layers_all_visible()}
              </p>
              <p className="mt-2 text-sm text-muted-foreground">
                {m.msg_settings_layers_note()}
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                onClick={onResetLayers}
                disabled={hiddenLayerCount === 0}
                variant="outline"
              >
                {m.action_reset_layers()}
              </Button>
            </div>
          </div>
        </SettingsCard>
      </div>

      <Card className="p-6">
        {/* The same draft/committed split the API base URL uses: the input
            edits a draft, Save normalises and commits it. The group is what
            tells a screen reader which "Save changes" this is — the Connection
            tab carries a button by the same name.

            Unlike the API base URL, this one is not read again until a map is
            built. Writing the key is the whole commit; there is no live map on
            this route to push the new tiles into. */}
        <div
          role="group"
          aria-label={m.label_basemap_tile_url()}
          className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_18rem]"
        >
          <div className="space-y-4">
            <FormField
              id="basemap-tile-url"
              label={m.label_basemap_tile_url()}
              hint={m.msg_basemap_tile_url_help()}
              value={tileUrlDraft}
              onChange={(event) => {
                setTileUrlDraft(event.target.value);
              }}
              placeholder={DEFAULT_TILE_URL}
            />
            <Callout variant="neutral" title={m.msg_basemap_tile_url_current()}>
              <p className="break-all font-mono text-foreground">{tileUrl}</p>
              <p className="mt-2">{m.msg_basemap_tile_url_note()}</p>
            </Callout>
          </div>

          <div className="space-y-2 self-start rounded-md border p-4">
            <Button
              className="w-full"
              onClick={onSaveTileUrl}
              disabled={tileUrlDraft.trim() === tileUrl}
            >
              {m.action_save_changes()}
            </Button>
            <Button
              className="w-full"
              variant="outline"
              onClick={onResetTileUrl}
              disabled={!hasTileOverride}
            >
              {m.action_reset_to_default()}
            </Button>
          </div>
        </div>
      </Card>
    </div>
  );
}

export default function SettingsPage() {
  const { theme, setTheme } = useTheme();
  const locale = useLocale();
  const location = useLocation();
  const navigate = useNavigate();
  const [draftPresent, setDraftPresent] = useState(() => hasDraft());
  const [apiBaseUrl, setApiBaseUrl] = useState(() => getAPIBaseURL());
  const [apiBaseUrlDraft, setApiBaseUrlDraft] = useState(() => getAPIBaseURL());
  const [apiBaseUrlOverridePresent, setApiBaseUrlOverridePresent] = useState(
    () => hasAPIBaseURLOverride(),
  );
  const [tileUrl, setTileUrl] = useState(() => getTileURL());
  const [tileUrlDraft, setTileUrlDraft] = useState(() => getTileURL());
  const [tileUrlOverridePresent, setTileUrlOverridePresent] = useState(() =>
    hasTileURLOverride(),
  );

  // Read straight from the map's store rather than lifted into this page: it
  // is module-scoped, so the picker in the map's layer control and this one
  // are two views of one value and cannot disagree. A change here reaches the
  // map the next time it is built, which is what a basemap switch does anyway
  // — `map-view.tsx` keys its init effect on this.
  const basemap = useMapStore((s) => s.basemap);
  const setBasemap = useMapStore((s) => s.setBasemap);
  const layerVisibility = useMapStore((s) => s.layerVisibility);
  const resetLayerVisibility = useMapStore((s) => s.resetLayerVisibility);

  // Only groups switched *off* count. An entry is written for every group the
  // user touches, including the ones they turned back on, so counting keys
  // would report a layer that is plainly visible as hidden.
  const hiddenLayerCount = [...MODEL_LAYER_GROUPS, ...RESULT_LAYER_GROUPS]
    .map((group) => layerVisibility[group.id] ?? group.defaultVisible)
    .filter((visible) => !visible).length;

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
      id: "map",
      icon: Map,
      title: m.settings_category_map,
      description: m.settings_category_map_desc,
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

  // Writing the key is the whole commit: nothing re-reads it until a map is
  // built, so there is no live map to push the new tiles into from here. The
  // same is true of the layer reset below, for the same reason — Settings and
  // the map are different routes, and `ModelLayers` re-applies the store's
  // visibility as it adds the layers.
  function saveTileUrl() {
    setTileURLOverride(tileUrlDraft.trim());
    const next = getTileURL();
    setTileUrl(next);
    setTileUrlDraft(next);
    setTileUrlOverridePresent(hasTileURLOverride());
  }

  function resetTileUrl() {
    clearTileURLOverride();
    const next = getTileURL();
    setTileUrl(next);
    setTileUrlDraft(next);
    setTileUrlOverridePresent(hasTileURLOverride());
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
          <TabsContent value="map" className="mt-0">
            <MapSettings
              basemap={basemap}
              setBasemap={setBasemap}
              hiddenLayerCount={hiddenLayerCount}
              onResetLayers={resetLayerVisibility}
              tileUrl={tileUrl}
              tileUrlDraft={tileUrlDraft}
              setTileUrlDraft={setTileUrlDraft}
              onSaveTileUrl={saveTileUrl}
              onResetTileUrl={resetTileUrl}
              hasTileOverride={tileUrlOverridePresent}
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
