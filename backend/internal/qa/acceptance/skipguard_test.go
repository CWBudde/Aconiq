package acceptance

import (
	"strings"
	"testing"
)

func TestResolveSuiteSkipLeavesExecutedSuitesAlone(t *testing.T) {
	t.Setenv(StrictSuiteEnv, "")

	status, reason, apply := ResolveSuiteSkip(10, 9, "")
	if apply {
		t.Fatalf("a suite with one executed task must not be treated as skipped: %q %q", status, reason)
	}

	if _, _, apply := ResolveSuiteSkip(10, 0, ""); apply {
		t.Fatal("a fully executed suite must not be treated as skipped")
	}
}

func TestResolveSuiteSkipAlwaysProducesAReason(t *testing.T) {
	t.Setenv(StrictSuiteEnv, "")

	cases := []struct {
		name         string
		taskCount    int
		skippedCount int
		reason       string
		wantContains string
	}{
		{
			name:         "no tasks at all",
			taskCount:    0,
			skippedCount: 0,
			wantContains: "executed no tasks",
		},
		{
			name:         "every task skipped",
			taskCount:    7,
			skippedCount: 7,
			wantContains: "all 7 suite tasks skipped",
		},
		{
			name:         "suite supplied its own reason",
			taskCount:    0,
			skippedCount: 0,
			reason:       "local suite directory not found",
			wantContains: "local suite directory not found",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, reason, apply := ResolveSuiteSkip(tc.taskCount, tc.skippedCount, tc.reason)
			if !apply {
				t.Fatal("expected the skip verdict to apply")
			}

			if status != StatusSkipped {
				t.Fatalf("status = %q, want %q", status, StatusSkipped)
			}

			if !strings.Contains(reason, tc.wantContains) {
				t.Fatalf("reason = %q, want it to contain %q", reason, tc.wantContains)
			}
		})
	}
}

func TestResolveSuiteSkipEscalatesUnderStrictMode(t *testing.T) {
	t.Setenv(StrictSuiteEnv, "1")

	status, reason, apply := ResolveSuiteSkip(7, 7, "fixtures not installed")
	if !apply {
		t.Fatal("expected the skip verdict to apply")
	}

	if status != StatusFailed {
		t.Fatalf("status = %q, want %q under %s=1", status, StatusFailed, StrictSuiteEnv)
	}

	if !strings.Contains(reason, "fixtures not installed") {
		t.Fatalf("reason = %q, want the original reason preserved", reason)
	}

	if !strings.Contains(reason, StrictSuiteEnv) {
		t.Fatalf("reason = %q, want it to name %s so the escalation is explainable", reason, StrictSuiteEnv)
	}

	// A suite that actually executed tasks stays untouched even in strict mode.
	if _, _, apply := ResolveSuiteSkip(7, 6, ""); apply {
		t.Fatal("strict mode must not touch a suite that produced evidence")
	}
}

func TestStrictSuitesReadsEnvironment(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{value: "1", want: true},
		{value: "true", want: true},
		{value: "TRUE", want: true},
		{value: "0", want: false},
		{value: "false", want: false},
		{value: "", want: false},
		{value: "yes", want: false},
	}

	for _, tc := range cases {
		t.Run("value="+tc.value, func(t *testing.T) {
			t.Setenv(StrictSuiteEnv, tc.value)

			if got := StrictSuites(); got != tc.want {
				t.Fatalf("StrictSuites() with %q = %t, want %t", tc.value, got, tc.want)
			}
		})
	}
}
