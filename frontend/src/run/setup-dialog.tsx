import { useState } from "react";
import { Link } from "react-router";
import {
  AlertCircle,
  Grid2x2,
  Loader2,
  Play,
  Settings2,
  Info,
} from "lucide-react";
import { Badge } from "@/ui/components/badge";
import { Button } from "@/ui/components/button";
import { Checkbox } from "@/ui/components/checkbox";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/ui/components/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/ui/components/select";
import { Label } from "@/ui/components/label";
import { Callout } from "@/ui/callout";
import { SectionHeading } from "@/ui/page-header";
import {
  EvidenceTierBadge,
  EvidenceTierWarning,
} from "@/ui/evidence-tier-badge";
import { useCreateRun, useStandards } from "@/api/hooks";
import type { RunProgress } from "@/api/hooks";
import { backend } from "@/api/backend";
import {
  asAPIRequestError,
  ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED,
} from "@/api/api-error";
import { isScaffoldTier } from "@/api/evidence-tier";
import type { ParameterDefinition } from "@/api/client";
import { useModelStore } from "@/model/model-store";
import { useModelValidation } from "@/model/use-model-validation";
import {
  projectHydrationStore,
  retryProjectHydration,
} from "@/model/use-project-hydration";
import { useProjectSync } from "@/model/use-project-sync";
import { GridEstimateNote } from "@/run/grid-estimate-note";
import { ParameterField } from "@/run/parameter-field";
import {
  parameterDescription,
  parameterGroup,
  parameterGroupLabel,
  PARAMETER_GROUP_ORDER,
  type ParameterGroupKey,
} from "@/run/parameter-meta";
import { useGridExtent } from "@/run/use-grid-extent";
import { useRunSetupSelection } from "@/run/use-run-setup-selection";
import { getStandardDescription, getStandardLabel } from "@/run/standards-meta";
import { m } from "@/i18n/messages";

// ---------------------------------------------------------------------------
// Run creation failure
// ---------------------------------------------------------------------------

/**
 * The API refuses a run against a scaffold-tier standard that the request did
 * not acknowledge. The UI gates that ahead of the request, so reaching this
 * means the two disagree — a tier that changed server-side, or a standards
 * list this dialog cached before it did. The server's own words are shown
 * rather than a generic failure, because they name the standard and the tier.
 */
function RunCreateError({ error }: { error: Error }) {
  const apiError = asAPIRequestError(error);
  const optInRequired =
    apiError?.code === ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED;

  return (
    <Callout
      variant="destructive"
      icon={AlertCircle}
      data-testid="run-create-error"
      data-error-code={apiError?.code}
      title={
        optInRequired ? m.msg_experimental_opt_in_required_error() : undefined
      }
    >
      <p>{error.message}</p>
      {apiError?.hint !== undefined ? <p>{apiError.hint}</p> : null}
    </Callout>
  );
}

// ---------------------------------------------------------------------------
// Run progress
// ---------------------------------------------------------------------------

/**
 * What the dialog shows while a run is in flight.
 *
 * Determinate as soon as the backend has reported once, and a plain phase
 * label before that. What is left on that label is the stretch before the
 * receiver count is even known: spawning the worker, instantiating the WASM
 * module and projecting the model through `resolveComputeModel`. A mode that
 * never reports at all — the HTTP backend — stays on it for the whole run,
 * which is exactly today's behaviour there.
 *
 * It used to cover far more than that, and that was the bug: browser mode
 * reported only between chunks of 256 receivers, so a scene with buildings in
 * it — tens of milliseconds a receiver — spent twenty seconds and more on
 * "starting the run" with nothing to show. The fix is on both sides of the
 * boundary and not in this component: `kernel-client.ts` reports 0 of n the
 * moment it dispatches, so the reader at least learns the size of the job,
 * and `wasmkernel.ComputeRLS19Road` now sizes its chunks by how long they
 * take rather than by a fixed count. A bar at 0 of 13,000 is not a bar that
 * has stalled; a spinner that has said "starting" for a minute is.
 *
 * The bar carries an accessible name: a screen reader announcing "62%" with
 * nothing to attach it to is a worse answer than none, and axe fails an
 * unnamed progressbar outright. `aria-valuetext` repeats the visible sentence
 * because "62" out of "1600" is not what a listener wants to hear.
 */
function RunProgressPanel({ progress }: { progress: RunProgress | null }) {
  if (progress === null) {
    return (
      <p
        className="flex items-center gap-2 text-xs text-muted-foreground"
        data-testid="run-progress-starting"
      >
        <Loader2 aria-hidden="true" className="h-3.5 w-3.5 animate-spin" />
        {m.status_starting_run()}
      </p>
    );
  }

  const { done, total } = progress;
  // A run with no receivers is refused before it starts, so the guard is not
  // for a case that happens — it is for not dividing by a number this
  // component was handed rather than computed.
  const percent = total > 0 ? Math.round(Math.min(done / total, 1) * 100) : 0;
  const label = m.status_run_progress({ done, total });

  return (
    <div className="space-y-1.5" data-testid="run-progress">
      <div
        role="progressbar"
        aria-label={m.label_run_progress()}
        aria-valuemin={0}
        aria-valuemax={total}
        aria-valuenow={done}
        aria-valuetext={label}
        className="h-1.5 w-full overflow-hidden rounded-full bg-muted"
      >
        {/* `ease-linear` over the 100 ms the worker throttles to: the bar
            should move at the speed the receivers are computed at, and an
            ease-out would make every tick look like it was slowing down. */}
        <div
          className="h-full rounded-full bg-primary transition-[width] duration-150 ease-linear"
          style={{ width: `${String(percent)}%` }}
        />
      </div>
      <p className="text-xs text-muted-foreground">{label}</p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Run setup dialog
// ---------------------------------------------------------------------------

type ReceiverMode = "auto-grid" | "custom";

/** Non-empty groups in render order, each keeping the backend's own ordering. */
function groupParameters(
  parameters: ParameterDefinition[],
): Array<[ParameterGroupKey, ParameterDefinition[]]> {
  const byGroup = new Map<ParameterGroupKey, ParameterDefinition[]>();
  for (const param of parameters) {
    const key = parameterGroup(param.name);
    const members = byGroup.get(key);
    if (members) members.push(param);
    else byGroup.set(key, [param]);
  }
  return PARAMETER_GROUP_ORDER.filter((key) => byGroup.has(key)).map((key) => [
    key,
    byGroup.get(key) ?? [],
  ]);
}

/**
 * The sentence every field in a group shares, if they share one.
 *
 * The four fields under "Nachtverkehr" all read "Vorgabe für Quellen ohne
 * eigenen Wert", and printing it four times says it no better than once while
 * making the group four lines taller. Hoisted to the group, the sentence is
 * still each field's accessible description — several controls may name the
 * same `aria-describedby` id — so nothing is lost to a screen reader either.
 *
 * `null` unless *every* member agrees and there is more than one of them: a
 * group of one has no repetition to remove, and one dissenting field would make
 * a hoisted sentence a claim about a parameter it is not true of.
 */
function sharedDescription(
  standardId: string,
  members: ParameterDefinition[],
): string | null {
  if (members.length < 2) return null;
  const first = parameterDescription(
    standardId,
    members[0] as ParameterDefinition,
  );
  if (first === null) return null;
  return members.every((p) => parameterDescription(standardId, p) === first)
    ? first
    : null;
}

const groupNoteId = (group: ParameterGroupKey): string =>
  `param-group-${group}-note`;

/** The hoisted sentence, rendered under the legend — or nothing. */
function groupNote(
  standardId: string,
  group: ParameterGroupKey,
  members: ParameterDefinition[],
) {
  const shared = sharedDescription(standardId, members);
  if (shared === null) return null;
  return (
    <p id={groupNoteId(group)} className="-mt-1 text-xs text-muted-foreground">
      {shared}
    </p>
  );
}

function ReceiverModeButton({
  mode,
  current,
  onSelect,
  icon: Icon,
  title,
  description,
}: {
  mode: ReceiverMode;
  current: ReceiverMode;
  onSelect: (mode: ReceiverMode) => void;
  icon: typeof Grid2x2;
  title: string;
  description: string;
}) {
  const selected = mode === current;
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={() => {
        onSelect(mode);
      }}
      className={`flex items-start gap-3 rounded-lg border p-4 text-left transition-colors ${
        selected
          ? "border-primary bg-primary/5"
          : "border-border hover:bg-muted/50"
      }`}
    >
      <Icon
        aria-hidden="true"
        className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground"
      />
      <div>
        <p className="text-sm font-medium">{title}</p>
        <p className="text-xs text-muted-foreground">{description}</p>
      </div>
    </button>
  );
}

/**
 * The dialog shell. It owns nothing but its own openness.
 *
 * The form is a child rather than the body of this component so that the
 * queries and the model validation it needs run while the dialog is open and
 * not on every render of `/run` — and so that closing it discards the
 * acknowledgement, the cascade and any creation error by unmounting, rather
 * than by a handler that has to remember each one.
 */
export function RunSetupDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (runId: string) => void;
}) {
  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onClose();
      }}
    >
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{m.dialog_title_new_run()}</DialogTitle>
          <DialogDescription>{m.dialog_desc_new_run()}</DialogDescription>
        </DialogHeader>
        <RunSetupForm onClose={onClose} onCreated={onCreated} />
      </DialogContent>
    </Dialog>
  );
}

function RunSetupForm({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: (runId: string) => void;
}) {
  const { data: standards, isLoading, error } = useStandards();
  const createRun = useCreateRun();
  const receiverCount = useModelStore((s) => s.receivers.length);
  const calcArea = useModelStore((s) => s.calcArea);
  // Where a run reads the saved model, starting one with unsaved edits would
  // silently compute the project's older copy. The same save the header
  // offers is offered here, so the user need not leave the dialog.
  const projectSync = useProjectSync();
  const unsavedChanges = projectSync.enabled && projectSync.dirty;

  // The gate on the model itself. `useModelValidation` is the only sanctioned
  // entry point: it passes the receivers, so a model whose only defect is a
  // receiver does not read valid, and it answers "empty" *above* the validator
  // so the hardcoded-English `model.empty` error never reaches the UI.
  //
  // Errors only, never warnings. An RLS-19 source that wants a review raises a
  // warning on a perfectly runnable model; refusing on those would make the
  // common case unrunnable.
  const validation = useModelValidation();
  const hydrationFailed = projectHydrationStore(
    (s) => s.status === "error" && backend.capabilities.runsAgainstSavedModel,
  );
  // An empty store is not an empty project when the run reads the saved model
  // and hydration never delivered it. Saying "nothing to calculate" there would
  // be a claim about the project this dialog cannot make — so the validation
  // verdict is suppressed, and the hydration callout speaks instead.
  const modelBlocksRun =
    !hydrationFailed &&
    (validation.state === "empty" || validation.errorCount > 0);

  // Not knowing is also a reason to refuse. A failed hydration leaves the store
  // as a copy that never arrived, so neither this dialog nor the user can say
  // whether the saved model the run would read is valid; the callout above
  // says so and offers the retry, and the action has to agree with it. Kept
  // separate from `modelBlocksRun` so the two callouts stay mutually
  // exclusive: this one must not also claim the model is empty.
  const startBlocked = hydrationFailed || modelBlocksRun;

  const selection = useRunSetupSelection(standards);
  const {
    standardId: effectiveStandardId,
    version: effectiveVersion,
    profile: effectiveProfile,
    standard: selectedStandard,
    versionInfo: selectedVersion,
    profileInfo: selectedProfile,
    params,
  } = selection;

  const [receiverMode, setReceiverMode] = useState<ReceiverMode>("auto-grid");

  // The extent the automatic grid would cover, in metres. Only in auto-grid
  // mode: a custom receiver set is the points the user placed, and no
  // resolution describes it. The projection it needs runs once per model, not
  // once per keystroke — see `useGridExtent`.
  const gridExtent = useGridExtent(receiverMode === "auto-grid");

  // Asked of the capability, not of the mode: cancelling means terminating
  // the kernel this tab owns, and no such lever exists over a run the API is
  // executing.
  const cancelRunOffered =
    createRun.isPending && backend.capabilities.runsAreCancellable;

  // A scaffold module carries no normative coefficients, so the API refuses to
  // run one until the request says so. The tier of the standard actually
  // selected decides that — never the checkbox, which only records that the
  // user was told.
  const requiresExperimentalOptIn = isScaffoldTier(
    selectedStandard?.evidence_tier,
  );

  // The acknowledgement is stored as the standard it was given for, not as a
  // flag: it is an acknowledgement of one choice, not a preference. Changing
  // the standard clears it outright, so switching away and back asks again,
  // and the identity check keeps a stale tick from surviving a standards list
  // that reloads under it.
  const [acknowledgedStandardId, setAcknowledgedStandardId] = useState<
    string | null
  >(null);
  const experimentalAcknowledged =
    acknowledgedStandardId !== null &&
    acknowledgedStandardId === effectiveStandardId;
  const experimentalOptInMissing =
    requiresExperimentalOptIn && !experimentalAcknowledged;

  // The acknowledgement rides along with the standard rather than inside the
  // cascade: it is an evidence-tier decision, not part of choosing a profile.
  function handleStandardChange(id: string) {
    selection.selectStandard(id);
    setAcknowledgedStandardId(null);
  }

  function handleSubmit() {
    if (experimentalOptInMissing || unsavedChanges || startBlocked) return;

    createRun.mutate(
      {
        standardId: effectiveStandardId,
        version: effectiveVersion,
        profile: effectiveProfile,
        params,
        receiverMode,
        // Derived from the tier of the standard being run, so a normative one
        // can never inherit an opt-in from an earlier selection.
        ...(requiresExperimentalOptIn ? { experimental: true } : {}),
      },
      {
        onSuccess: (run) => {
          onCreated(run.id);
          onClose();
        },
      },
    );
  }

  return (
    <>
      {isLoading ? (
        <div className="flex items-center justify-center py-12">
          <Loader2
            aria-hidden="true"
            className="h-6 w-6 animate-spin text-muted-foreground"
          />
        </div>
      ) : error ? (
        <Callout variant="destructive" icon={AlertCircle}>
          {m.msg_api_error_standards()}
        </Callout>
      ) : createRun.isError && !createRun.cancelled ? (
        <RunCreateError error={createRun.error} />
      ) : (
        <div className="space-y-6">
          {/* Standard / Version / Profile */}
          <section className="space-y-4">
            <SectionHeading variant="eyebrow">
              {m.label_standard()}
            </SectionHeading>
            <div className="grid grid-cols-3 gap-3">
              <div className="space-y-1">
                <Label htmlFor="standard">{m.label_standard()}</Label>
                <Select
                  value={effectiveStandardId}
                  onValueChange={handleStandardChange}
                >
                  <SelectTrigger id="standard">
                    <SelectValue
                      placeholder={m.placeholder_select_standard()}
                    />
                  </SelectTrigger>
                  <SelectContent>
                    {standards?.map((s) => (
                      <SelectItem key={s.id} value={s.id}>
                        <span className="flex items-center gap-2">
                          <span>{getStandardLabel(s.id, s.evidence_tier)}</span>
                          <EvidenceTierBadge tier={s.evidence_tier} />
                        </span>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label htmlFor="version">{m.label_version()}</Label>
                <Select
                  value={effectiveVersion}
                  onValueChange={selection.selectVersion}
                  disabled={!selectedStandard}
                >
                  <SelectTrigger id="version">
                    <SelectValue placeholder={m.placeholder_select_version()} />
                  </SelectTrigger>
                  <SelectContent>
                    {selectedStandard?.versions.map((v) => (
                      <SelectItem key={v.name} value={v.name}>
                        {v.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label htmlFor="profile">{m.label_profile()}</Label>
                <Select
                  value={effectiveProfile}
                  onValueChange={selection.selectProfile}
                  disabled={!selectedVersion}
                >
                  <SelectTrigger id="profile">
                    <SelectValue placeholder={m.placeholder_select_profile()} />
                  </SelectTrigger>
                  <SelectContent>
                    {selectedVersion?.profiles.map((p) => (
                      <SelectItem key={p.name} value={p.name}>
                        {p.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            {selectedStandard ? (
              <div className="space-y-2">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-medium">
                    {getStandardLabel(
                      selectedStandard.id,
                      selectedStandard.evidence_tier,
                    )}
                  </span>
                  <EvidenceTierBadge tier={selectedStandard.evidence_tier} />
                </div>
                <p className="text-xs text-muted-foreground">
                  {getStandardDescription(
                    selectedStandard.id,
                    selectedStandard.description,
                  )}
                </p>
              </div>
            ) : null}

            {selectedProfile ? (
              <div className="flex flex-wrap gap-2">
                {selectedProfile.supported_indicators.map((ind) => (
                  <Badge key={ind} variant="secondary">
                    {ind}
                  </Badge>
                ))}
              </div>
            ) : null}
          </section>

          {/* Parameters */}
          {/* `selectedStandard` is in the condition because a parameter's
              description is resolved per standard — only the normative
              modules have a German one — and a profile never exists
              without the standard it belongs to anyway. */}
          {selectedStandard &&
          selectedProfile &&
          selectedProfile.parameters.length > 0 ? (
            <section className="space-y-4">
              <SectionHeading variant="eyebrow">
                {m.label_section_parameters()}
              </SectionHeading>
              {/* Grouped, because nineteen fields in one flat grid is a
                    wall. A `fieldset`/`legend` rather than a heading: these are
                    groups of controls, and adding an h4 under the section's h3
                    would put document structure where form structure belongs.
                    Within a group the order is the backend's own — grouping
                    already moves fields, and re-sorting inside a group on top
                    of that would move them again for nothing. */}
              {groupParameters(selectedProfile.parameters).map(
                ([group, members]) => (
                  <fieldset key={group} className="space-y-3 border-0 p-0">
                    <legend className="mb-1 text-xs font-medium text-muted-foreground">
                      {parameterGroupLabel(group)}
                    </legend>
                    {groupNote(selectedStandard.id, group, members)}
                    {/* Above the fields rather than below them: the cost of
                        the grid is what the reader should have in mind while
                        choosing a resolution, and a number that appears
                        underneath is a number found after the decision. */}
                    {group === "grid" ? (
                      <GridEstimateNote
                        state={gridExtent}
                        params={params}
                        onUseResolution={(resolutionM) => {
                          selection.setParam(
                            "grid_resolution_m",
                            String(resolutionM),
                          );
                        }}
                      />
                    ) : null}
                    <div className="grid grid-cols-2 gap-x-4 gap-y-3">
                      {members.map((param) => (
                        <ParameterField
                          key={param.name}
                          standardId={selectedStandard.id}
                          param={param}
                          describedById={
                            sharedDescription(selectedStandard.id, members) ===
                            null
                              ? undefined
                              : groupNoteId(group)
                          }
                          value={params[param.name] ?? ""}
                          onChange={(v) => {
                            selection.setParam(param.name, v);
                          }}
                        />
                      ))}
                    </div>
                  </fieldset>
                ),
              )}
            </section>
          ) : null}

          {/* Receiver set */}
          <section className="space-y-3">
            <SectionHeading variant="eyebrow">
              {m.label_receivers()}
            </SectionHeading>
            <div className="grid grid-cols-2 gap-3">
              <ReceiverModeButton
                mode="auto-grid"
                current={receiverMode}
                onSelect={setReceiverMode}
                icon={Grid2x2}
                title={m.label_receiver_auto_grid()}
                description={m.msg_receiver_auto_grid_desc()}
              />
              <ReceiverModeButton
                mode="custom"
                current={receiverMode}
                onSelect={setReceiverMode}
                icon={Settings2}
                title={m.label_receiver_custom_set()}
                description={m.msg_receiver_custom_set_desc()}
              />
            </div>
            {/* The same sentence in both modes: `modelToGeoJSON` now emits
                  the area as a `calc-area` feature, and the backend resolves
                  the auto-grid extent from it. No separate gate is needed for
                  an area that has not been saved yet — setting one marks the
                  model dirty, and the dialog already refuses to start a run
                  while it is. */}
            {receiverMode === "auto-grid" && calcArea ? (
              <p className="text-xs text-info">{m.msg_calc_area_active()}</p>
            ) : null}
            {receiverMode === "custom" &&
            receiverCount === 0 &&
            !backend.capabilities.runsAgainstSavedModel ? (
              <Callout variant="warning" icon={AlertCircle}>
                {m.msg_no_explicit_receivers()}
              </Callout>
            ) : null}
            {receiverMode === "custom" && receiverCount > 0 ? (
              <p className="text-xs text-muted-foreground">
                {m.msg_receivers_placed({ count: receiverCount })}
              </p>
            ) : null}
            {receiverMode === "custom" &&
            backend.capabilities.runsAgainstSavedModel ? (
              <p className="text-xs text-muted-foreground">
                {m.msg_api_mode_reads_explicit_receivers()}
              </p>
            ) : null}
          </section>

          {/* Determinism hint */}
          {selectedProfile ? (
            <Callout variant="neutral" icon={Info}>
              {m.msg_determinism_hint_dialog()}
            </Callout>
          ) : null}

          {/* Sits immediately above the run action: a scaffold module has no
                normative coefficients, so nothing it produces may be read as
                an assessment result. */}
          <EvidenceTierWarning tier={selectedStandard?.evidence_tier} />

          {/* The deliberate acknowledgement the API demands before a
                scaffold-tier standard may emit levels. It sits with the
                warning it acknowledges, and gates the run action. The warning
                above is the live region; this box is a plain group, so a
                reader is not interrupted twice for one fact. */}
          {requiresExperimentalOptIn ? (
            <Callout variant="warning" role="group">
              <div className="flex items-start gap-2.5">
                <Checkbox
                  id="experimental-opt-in"
                  className="mt-0.5"
                  checked={experimentalAcknowledged}
                  onCheckedChange={(checked) => {
                    setAcknowledgedStandardId(
                      checked === true ? effectiveStandardId : null,
                    );
                  }}
                />
                <div className="space-y-1">
                  <Label
                    htmlFor="experimental-opt-in"
                    className="text-xs font-medium"
                  >
                    {m.label_experimental_opt_in()}
                  </Label>
                  <p className="text-xs text-muted-foreground">
                    {m.msg_experimental_opt_in_help()}
                  </p>
                </div>
              </div>
            </Callout>
          ) : null}

          {/* The run reads the project's model, and hydration never delivered
              it, so this dialog does not know what is in it. Gates the run
              action. */}
          {hydrationFailed ? (
            <Callout
              variant="destructive"
              icon={AlertCircle}
              data-testid="hydration-failed-callout"
            >
              <div className="flex flex-wrap items-center gap-2">
                <span className="flex-1">
                  {m.msg_project_model_load_failed()}
                </span>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={retryProjectHydration}
                >
                  {m.action_retry()}
                </Button>
              </div>
            </Callout>
          ) : null}

          {/* The model itself. Gates the run action. */}
          {modelBlocksRun ? (
            <Callout
              variant="warning"
              icon={AlertCircle}
              data-testid="model-invalid-callout"
            >
              <div className="flex flex-wrap items-center gap-2">
                <span className="flex-1">
                  {validation.state === "empty"
                    ? m.msg_model_empty_before_run()
                    : `${m.msg_model_invalid_before_run()} ${m.msg_validation_error_count(
                        { count: validation.errorCount },
                      )}.`}
                </span>
                <Button asChild size="sm" variant="outline">
                  <Link to="/model">{m.action_open_model()}</Link>
                </Button>
              </div>
            </Callout>
          ) : null}

          {/* The run reads the project's copy of the model, so unsaved
                edits would not be in it. Gates the run action. */}
          {unsavedChanges ? (
            <Callout
              variant="warning"
              icon={AlertCircle}
              data-testid="unsaved-changes-callout"
            >
              <div className="flex flex-wrap items-center gap-2">
                <span className="flex-1">
                  {m.msg_unsaved_changes_before_run()}
                  {projectSync.status === "error"
                    ? ` ${m.msg_save_failed()}.`
                    : null}
                </span>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={projectSync.status === "saving"}
                  onClick={() => {
                    void projectSync.save();
                  }}
                >
                  {projectSync.status === "saving"
                    ? m.status_saving()
                    : m.action_save_to_project()}
                </Button>
              </div>
            </Callout>
          ) : null}
        </div>
      )}

      {/* A cancelled run is not a failed one: nothing went wrong and nothing
          was written, so the form stays on screen with a neutral note rather
          than being replaced by a red error the user has to dismiss to try
          again with a smaller grid. */}
      {createRun.cancelled ? (
        <Callout variant="warning" icon={Info} data-testid="run-cancelled">
          {m.msg_run_cancelled()}
        </Callout>
      ) : null}

      {createRun.isPending ? (
        <RunProgressPanel progress={createRun.progress} />
      ) : null}

      {!isLoading && !error ? (
        <DialogFooter>
          {/* One button, two jobs, because there is only one thing the user
              can call off at a time: while a cancellable run is in flight it
              stops the run, and otherwise it closes the dialog. Where the
              backend cannot stop a run it keeps closing the dialog — a button
              that dismissed the dialog while the server went on computing
              would be a lie about what it did. */}
          <Button
            variant="ghost"
            onClick={cancelRunOffered ? createRun.cancel : onClose}
          >
            {cancelRunOffered ? m.action_cancel_run() : m.action_cancel()}
          </Button>
          <Button
            onClick={handleSubmit}
            title={
              experimentalOptInMissing
                ? m.tooltip_experimental_opt_in_required()
                : undefined
            }
            disabled={
              !selectedProfile ||
              createRun.isPending ||
              experimentalOptInMissing ||
              unsavedChanges ||
              startBlocked ||
              (!backend.capabilities.runsAgainstSavedModel &&
                receiverMode === "custom" &&
                receiverCount === 0)
            }
          >
            <Play aria-hidden="true" className="mr-2 h-4 w-4" />
            {createRun.isPending
              ? m.status_starting_run()
              : m.action_start_run()}
          </Button>
        </DialogFooter>
      ) : null}
    </>
  );
}
