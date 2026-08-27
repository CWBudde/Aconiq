package acceptance

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Suite report statuses shared by the acceptance suite runners.
const (
	StatusPassed  = "passed"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
)

// StrictSuiteEnv names the environment variable that turns a fully skipped
// acceptance suite into a failed one.
//
// A suite in which every task skipped carries no evidence at all, so reporting
// it as green is worse than reporting it as red: nobody notices that the
// conformance claim stopped being checked. It cannot simply be a failure
// everywhere either - a clean checkout legitimately lacks the licensed local
// fixture bundles, and a developer machine should not go red for that.
//
// The compromise: an empty suite always carries an explicit skip reason in its
// report (never a silent "skipped" with no explanation), and an environment
// that is supposed to have the fixtures sets ACONIQ_STRICT_ACCEPTANCE=1 to
// escalate the same situation to a failure.
const StrictSuiteEnv = "ACONIQ_STRICT_ACCEPTANCE"

// StrictSuites reports whether fully skipped acceptance suites must be
// reported as failures. Any value that strconv.ParseBool reads as true enables
// it; an unset or unparsable value leaves it off.
func StrictSuites() bool {
	raw, ok := os.LookupEnv(StrictSuiteEnv)
	if !ok {
		return false
	}

	enabled, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false
	}

	return enabled
}

// ResolveSuiteSkip decides how a suite report must describe a run that
// produced no evidence, that is a run with no tasks at all or one in which
// every task skipped.
//
// reason is the suite's own explanation, if it has one; it is preserved and
// only defaulted when empty. apply is false when the suite actually executed
// something, in which case the caller keeps its own status untouched.
func ResolveSuiteSkip(taskCount int, skippedCount int, reason string) (status string, resolvedReason string, apply bool) {
	if taskCount > 0 && skippedCount < taskCount {
		return "", "", false
	}

	resolvedReason = strings.TrimSpace(reason)
	if resolvedReason == "" {
		if taskCount == 0 {
			resolvedReason = "the suite executed no tasks"
		} else {
			resolvedReason = fmt.Sprintf("all %d suite tasks skipped", taskCount)
		}
	}

	if StrictSuites() {
		return StatusFailed, fmt.Sprintf("%s (%s is set, so a suite without evidence fails)", resolvedReason, StrictSuiteEnv), true
	}

	return StatusSkipped, resolvedReason, true
}
