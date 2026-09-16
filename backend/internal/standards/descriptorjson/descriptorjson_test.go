package descriptorjson_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aconiq/backend/internal/standards/descriptorjson"
	"github.com/aconiq/backend/internal/standards/framework"
)

// An empty registry answers `[]`, not `null`. A client that iterates the
// response would throw on null, and `GET /api/v1/standards` has always answered
// with an array.
func TestEmptyDescriptorListEncodesAsAnArray(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(descriptorjson.FromDescriptors(nil))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if string(raw) != "[]" {
		t.Errorf("encoded %s, want []", raw)
	}
}

// The omitempty set is the published contract: `required` and `context` are
// always present so a strict consumer may rely on them, while an absent unit,
// default, description, enum or bound is absent rather than empty.
func TestOmitemptySetIsWhatConsumersRelyOn(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(descriptorjson.FromDescriptor(framework.StandardDescriptor{
		Context:        framework.StandardContextPlanning,
		ID:             "example",
		Description:    "example",
		EvidenceTier:   framework.EvidenceTierTestFixture,
		DefaultVersion: "1",
		Versions: []framework.Version{{
			Name:           "1",
			DefaultProfile: "default",
			Profiles: []framework.Profile{{
				Name: "default",
				ParameterSchema: framework.ParameterSchema{
					Parameters: []framework.ParameterDefinition{
						{Name: "bare", Kind: framework.ParameterKindString},
					},
				},
			}},
		}},
	}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	encoded := string(raw)

	for _, present := range []string{`"required":false`, `"context":"planning"`, `"evidence_tier":"test-fixture"`} {
		if !strings.Contains(encoded, present) {
			t.Errorf("%s is missing from %s", present, encoded)
		}
	}

	// Checked against the parameter object alone: the standard's own
	// `description` has no omitempty and is always present.
	const wantParameter = `{"name":"bare","kind":"string","required":false}`

	if !strings.Contains(encoded, wantParameter) {
		t.Errorf("a parameter carrying nothing optional encoded as something other than %s: %s",
			wantParameter, encoded)
	}
}

// The evidence tier is carried across as the module declares it, rather than
// derived a second time. That is the whole reason the field exists on the
// descriptor.
func TestEvidenceTierIsCarriedVerbatim(t *testing.T) {
	t.Parallel()

	for _, tier := range []framework.EvidenceTier{
		framework.EvidenceTierNormative,
		framework.EvidenceTierPreview,
		framework.EvidenceTierScaffold,
		framework.EvidenceTierTestFixture,
	} {
		got := descriptorjson.FromDescriptor(framework.StandardDescriptor{EvidenceTier: tier})
		if got.EvidenceTier != string(tier) {
			t.Errorf("evidence tier %q encoded as %q", tier, got.EvidenceTier)
		}
	}
}
