package facadeset

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/aconiq/backend/internal/geo"
	"github.com/aconiq/backend/internal/geo/modelgeojson"
)

// MergeMode says what MergeWindows does with a window whose id the model
// already holds.
type MergeMode int

const (
	// ReplaceWindows replaces the model's window of the same id, so importing
	// a corrected window list again updates it (`aconiq import --windows`).
	ReplaceWindows MergeMode = iota
	// AddWindowsOnly refuses an id the model already holds, so a proposal
	// never touches a window a person has confirmed or rejected (P18 D2).
	AddWindowsOnly
)

// MergeWindows writes placed window features into the model, leaving the
// model passed in as it was. An id another kind of feature holds is refused
// in either mode, as is an id given twice. It returns the new model and how
// many windows it replaced.
func MergeWindows(model modelgeojson.Model, windows []modelgeojson.Feature, mode MergeMode) (modelgeojson.Model, int, error) {
	kinds := make(map[string]string, len(model.Features))
	for _, feature := range model.Features {
		kinds[strings.TrimSpace(feature.ID)] = feature.Kind
	}

	byID := make(map[string]modelgeojson.Feature, len(windows))

	for _, w := range windows {
		if _, twice := byID[w.ID]; twice {
			return modelgeojson.Model{}, 0, fmt.Errorf("window %q is given twice", w.ID)
		}

		if kind, taken := kinds[w.ID]; taken {
			switch {
			case kind != modelgeojson.FeatureKindWindow:
				return modelgeojson.Model{}, 0, fmt.Errorf("id %q is already a %s feature of the model", w.ID, kind)
			case mode == AddWindowsOnly:
				return modelgeojson.Model{}, 0, fmt.Errorf("window %q is already in the model; a proposal only adds windows", w.ID)
			}
		}

		byID[w.ID] = w
	}

	out := model
	out.Features = slices.Clone(model.Features)
	replaced := 0

	for i, feature := range out.Features {
		w, ok := byID[strings.TrimSpace(feature.ID)]
		if !ok || feature.Kind != modelgeojson.FeatureKindWindow {
			continue
		}

		out.Features[i] = w
		delete(byID, w.ID)

		replaced++
	}

	for _, w := range windows {
		if _, pending := byID[w.ID]; pending {
			out.Features = append(out.Features, w)
		}
	}

	return out, replaced, nil
}

// EvidenceKinds are the kinds of source a window's evidence entry may name
// (facade-receivers.md, Receiver properties). There is no Google kind:
// nothing seen in Google imagery is recorded (PLAN.md Priority 18).
var EvidenceKinds = []string{"orthophoto", "oblique", "street-level", "site-photo", "plan", "citygml-lod3"}

// PropertyTransmissionBasis is the evidence member a proposal is stamped
// with: the reference to the per-engagement decision under which the image
// it was read from could be sent to the model that read it.
const PropertyTransmissionBasis = "transmission_basis"

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// proposalEvidenceKeys are the members a proposal's evidence entry may carry:
// what names the source and what its record needs, and nothing else, so a
// path, a URL or text read off the image cannot ride into the model on it.
// transmission_basis is not among them: the server stamps it.
var proposalEvidenceKeys = []string{"kind", "source_id", "content_sha256", "captured_at", "licence", "attribution"}

// maxTransmissionBasisLength bounds the transmission decision reference: an
// engagement's decision record is named in a few words, not pasted in.
const maxTransmissionBasisLength = 200

// ValidateTransmissionBasis checks the reference to the per-engagement
// decision a proposal is made under (P18 D2): a short, opaque reference,
// never a path, which could name the client (docs/policies/data-handling.md).
// `aconiq mcp --photo-transmission` and POST /api/v1/facades/windows hold it
// to the same rule, since ProposeWindows stamps it into the model.
func ValidateTransmissionBasis(reference string) error {
	trimmed := strings.TrimSpace(reference)

	switch {
	case trimmed == "":
		return fmt.Errorf("the %s reference is empty", PropertyTransmissionBasis)
	case len(trimmed) > maxTransmissionBasisLength:
		return fmt.Errorf("the %s reference is longer than %d characters; name the decision, do not paste it",
			PropertyTransmissionBasis, maxTransmissionBasisLength)
	case strings.ContainsAny(trimmed, `/\`):
		return fmt.Errorf("the %s reference must not be a path: name the decision in the engagement's records, not a file",
			PropertyTransmissionBasis)
	}

	return nil
}

// ErrNoProposal refuses a proposal call that proposes nothing.
var ErrNoProposal = errors.New("no window was proposed")

// ProposeWindows places windows an agent proposes from an image (P18 D2) and
// adds them to the model. A proposal is a window nobody has checked yet: it
// must be proposed and not sensitive, since only a person marks either, and
// it must say what it was read from — at least one evidence entry of a known
// kind, with the source's reference and the content hash of the image. Each
// entry is stamped with basis, the transmission decision the server was
// started under. Nothing is corrected: the first proposal that breaks a rule
// refuses them all, and an id the model already holds is refused rather than
// replaced. It returns the new model and the ids it added, in order.
func ProposeWindows(model modelgeojson.Model, proposals []WindowPlacement, basis string) (modelgeojson.Model, []string, error) {
	err := ValidateTransmissionBasis(basis)
	if err != nil {
		return modelgeojson.Model{}, nil, err
	}

	if len(proposals) == 0 {
		return modelgeojson.Model{}, nil, ErrNoProposal
	}

	placer, err := modelgeojson.NewWindowPlacer(model)
	if err != nil {
		return modelgeojson.Model{}, nil, fmt.Errorf("no metric frame to place windows in: %w", err)
	}

	windows := make([]modelgeojson.Feature, 0, len(proposals))
	ids := make([]string, 0, len(proposals))

	for i, p := range proposals {
		evidence, err := proposalEvidence(p, basis)
		if err != nil {
			return modelgeojson.Model{}, nil, fmt.Errorf("windows[%d] %q: %w", i, p.ID, err)
		}

		feature, err := placer.Feature(modelgeojson.WindowSpec{
			ID:           p.ID,
			BuildingRef:  strings.TrimSpace(p.BuildingRef),
			Part:         p.Part,
			Ring:         p.Ring,
			Edge:         p.Edge,
			Frame:        p.Frame,
			ReviewStatus: geo.ReviewStatusProposed,
			Evidence:     evidence,
		})
		if err != nil {
			return modelgeojson.Model{}, nil, fmt.Errorf("windows[%d]: %w", i, err)
		}

		windows = append(windows, feature)
		ids = append(ids, feature.ID)
	}

	merged, _, err := MergeWindows(model, windows, AddWindowsOnly)
	if err != nil {
		return modelgeojson.Model{}, nil, err
	}

	return merged, ids, nil
}

// proposalEvidence checks one proposal's review fields and evidence, and
// returns the evidence stamped with basis.
func proposalEvidence(p WindowPlacement, basis string) ([]any, error) {
	if p.ReviewStatus != "" && p.ReviewStatus != geo.ReviewStatusProposed {
		return nil, fmt.Errorf("review_status %q: a proposal is proposed; confirming or rejecting a window is a person's act", p.ReviewStatus)
	}

	if p.Sensitive {
		return nil, errors.New("sensitive: a proposal is not sensitive; whether the room behind a window is schutzbedürftig is a person's call")
	}

	if len(p.Evidence) == 0 {
		return nil, errors.New("evidence is required: name the image the window was read from")
	}

	stamped := make([]any, len(p.Evidence))

	for i, raw := range p.Evidence {
		entry, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("evidence[%d] must be an object", i)
		}

		for _, key := range slices.Sorted(maps.Keys(entry)) {
			if !slices.Contains(proposalEvidenceKeys, key) {
				return nil, fmt.Errorf("evidence[%d].%s is not a member a proposal's evidence carries (%s)",
					i, key, strings.Join(proposalEvidenceKeys, ", "))
			}
		}

		if kind, _ := entry["kind"].(string); !slices.Contains(EvidenceKinds, kind) {
			return nil, fmt.Errorf("evidence[%d].kind %#v must be one of %s", i, entry["kind"], strings.Join(EvidenceKinds, "|"))
		}

		if ref, _ := entry["source_id"].(string); strings.TrimSpace(ref) == "" || strings.ContainsAny(ref, `/\`) {
			return nil, fmt.Errorf("evidence[%d].source_id is required: the image's opaque reference in the engagement's store, never a path", i)
		}

		if hash, _ := entry["content_sha256"].(string); !sha256Hex.MatchString(hash) {
			return nil, fmt.Errorf("evidence[%d].content_sha256 must be the image's SHA-256 as 64 lowercase hex digits", i)
		}

		copied := maps.Clone(entry)
		copied[PropertyTransmissionBasis] = basis
		stamped[i] = copied
	}

	return stamped, nil
}
