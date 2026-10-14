// Package runestimate says what a run will cost in time before anyone starts
// it, by timing a small, deterministic sample of the run's own work and
// extrapolating.
//
// # Why a probe and not a formula
//
// What a receiver costs is not a function of anything a run dialog shows. On
// an RLS-19 scene it is dominated by the reflection search, which RLS-19
// Nr. 3.6 and Nr. 3.5 make a function of how many buildings stand near the
// road — a factor of thousands between an open field and a city block at the
// same receiver count and the same Teilstück length. A formula over the
// inputs would have to know the reflection geometry, and the only thing that
// knows it is the kernel. So the kernel is asked: a handful of receivers,
// spread over the grid, each walks a spread sample of its own work units, and
// the clock says what one unit costs there.
//
// # What is deterministic and what is not
//
// The sample is. Which receivers are probed and in which order each one's
// units are visited are pure functions of the receiver positions and the unit
// counts, so two estimates of one run time the same work in the same order —
// see SamplePlan. How *many* units fit in the budget is not, and neither are
// the timings: they are measurements of a machine, and the band says so.
//
// Nothing here computes a level. A standard opts in by implementing Kernel,
// whose Walker runs the same per-unit step the run's walk runs and throws the
// result away; a standard that does not is reported as having no estimate,
// which is the honest answer rather than a guess dressed up as one.
package runestimate

// Band is a duration range in seconds: a point estimate with the range the
// probe's own spread and sampling error put around it.
type Band struct {
	Low      float64 `json:"low"`
	Estimate float64 `json:"estimate"`
	High     float64 `json:"high"`
}

// scaled returns the band multiplied by factor.
func (b Band) scaled(factor float64) Band {
	return Band{Low: b.Low * factor, Estimate: b.Estimate * factor, High: b.High * factor}
}

// Estimate is what a run will cost, or why that cannot be said.
//
// It is one JSON shape for `aconiq run --estimate --json`, for
// `POST /api/v1/runs/estimate` and for the WASM kernel's `estimate`, because
// all three are this struct marshalled.
type Estimate struct {
	// Available is false for a standard that has no probe. Reason and
	// StandardID are then the only fields present.
	Available bool `json:"available"`

	// Reason says why there is no estimate, when there is none.
	Reason string `json:"reason,omitempty"`

	StandardID string `json:"standard_id,omitempty"`

	// WallSeconds is the elapsed time the run is expected to take on the
	// stated number of workers, scene preparation included.
	WallSeconds *Band `json:"wall_s,omitempty"`

	// CPUSeconds is the compute summed over every worker.
	CPUSeconds *Band `json:"cpu_s,omitempty"`

	// CPUPerReceiverSeconds is CPUSeconds over the receiver count: what a
	// receiver costs on average, which is what a caller scales by when it
	// asks what a coarser grid would cost.
	CPUPerReceiverSeconds *Band `json:"cpu_per_receiver_s,omitempty"`

	// Parallelism is the speed-up over one worker the point estimate assumes,
	// after the chunk schedule and the parallel efficiency. It is at most
	// Inputs.Workers.
	Parallelism float64 `json:"parallelism,omitempty"`

	Inputs *Inputs       `json:"inputs,omitempty"`
	Probe  *ProbeReport  `json:"probe,omitempty"`
	Alts   []Alternative `json:"alternatives,omitempty"`
}

// Inputs are the quantities the estimate scaled by, so a reader can see what
// made a run expensive.
type Inputs struct {
	Receivers int `json:"receivers"`
	Workers   int `json:"workers"`

	// Units is the number of work units the run walks, summed over every
	// receiver. For RLS-19 a unit is one (source, Teilstück) pair or one
	// Parkplatz, in the run's own segment_length_mode.
	Units int64 `json:"units"`

	// Counts carries the standard's own inputs by name — for RLS-19
	// road_sources, teilstuecke, buildings, reflector_walls, barriers and
	// parking_sources.
	Counts map[string]int `json:"counts,omitempty"`
}

// ProbeReport is what the probe measured.
type ProbeReport struct {
	// ProbeReceivers is how many receivers were chosen, MeasuredReceivers how
	// many of them the budget reached.
	ProbeReceivers    int `json:"probe_receivers"`
	MeasuredReceivers int `json:"measured_receivers"`

	// UnitsTimed is the number of work units the probe ran.
	UnitsTimed int `json:"units_timed"`

	BudgetSeconds  float64 `json:"budget_s"`
	ElapsedSeconds float64 `json:"elapsed_s"`

	// PrepareSeconds is the scene preparation, measured rather than sampled:
	// the estimate prepared the same scene the run will.
	PrepareSeconds float64 `json:"prepare_s"`

	// BudgetExhausted reports that the budget ran out before every probe
	// receiver had sampled all it wanted to.
	BudgetExhausted bool `json:"budget_exhausted"`

	// Clock is what the units were timed on: ClockThreadCPU where the
	// platform has a per-thread CPU clock, ClockWall otherwise.
	Clock string `json:"clock"`
}

// The clocks a probe can time its units on.
const (
	ClockThreadCPU = "thread_cpu"
	ClockWall      = "wall"
)

// Alternative is what the same run would cost with some parameters changed,
// costed from the same measurement.
//
// Measured is false when the budget ran out before the variant could be
// probed; Units and both bands are then absent rather than guessed.
type Alternative struct {
	Params      map[string]string `json:"params"`
	Measured    bool              `json:"measured"`
	Units       int64             `json:"units,omitempty"`
	CPUSeconds  *Band             `json:"cpu_s,omitempty"`
	WallSeconds *Band             `json:"wall_s,omitempty"`
}

// Unavailable is the estimate of a standard that has no probe.
func Unavailable(standardID, reason string) Estimate {
	return Estimate{
		Available:  false,
		Reason:     reason,
		StandardID: standardID,
	}
}
