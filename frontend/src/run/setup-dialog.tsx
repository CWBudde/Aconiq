import { useState } from "react";
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
import { backend } from "@/api/backend";
import {
  asAPIRequestError,
  ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED,
} from "@/api/api-error";
import { isScaffoldTier } from "@/api/evidence-tier";
import { useModelStore } from "@/model/model-store";
import { useProjectSync } from "@/model/use-project-sync";
import { ParameterField } from "@/run/parameter-field";
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
// Run setup dialog
// ---------------------------------------------------------------------------

type ReceiverMode = "auto-grid" | "custom";

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

export function RunSetupDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
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
    if (experimentalOptInMissing || unsavedChanges) return;

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
          handleClose();
        },
      },
    );
  }

  function handleClose() {
    // A reopened dialog is a fresh decision, not a resumed one.
    setAcknowledgedStandardId(null);
    onClose();
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) handleClose();
      }}
    >
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{m.dialog_title_new_run()}</DialogTitle>
          <DialogDescription>{m.dialog_desc_new_run()}</DialogDescription>
        </DialogHeader>

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
        ) : createRun.isError ? (
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
                            <span>{getStandardLabel(s.id)}</span>
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
                      <SelectValue
                        placeholder={m.placeholder_select_version()}
                      />
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
                      <SelectValue
                        placeholder={m.placeholder_select_profile()}
                      />
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
                      {getStandardLabel(selectedStandard.id)}
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
            {selectedProfile && selectedProfile.parameters.length > 0 ? (
              <section className="space-y-4">
                <SectionHeading variant="eyebrow">
                  {m.label_section_parameters()}
                </SectionHeading>
                <div className="grid grid-cols-2 gap-x-4 gap-y-3">
                  {selectedProfile.parameters.map((param) => (
                    <ParameterField
                      key={param.name}
                      param={param}
                      value={params[param.name] ?? ""}
                      onChange={(v) => {
                        selection.setParam(param.name, v);
                      }}
                    />
                  ))}
                </div>
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
                  {receiverCount === 1
                    ? m.msg_receivers_placed_one({ count: receiverCount })
                    : m.msg_receivers_placed_other({ count: receiverCount })}
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

        {!isLoading && !error ? (
          <DialogFooter>
            <Button variant="ghost" onClick={handleClose}>
              {m.action_cancel()}
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
      </DialogContent>
    </Dialog>
  );
}
