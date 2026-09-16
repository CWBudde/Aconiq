// Package wasmkernel holds the logic behind the WebAssembly kernel's JavaScript
// surface, separated from the syscall/js plumbing that registers it.
//
// It deliberately carries no build tag. `cmd/wasm/main.go` is `//go:build js &&
// wasm`, so nothing in it is reachable from `go test` on the host — which is
// how the browser came to advertise a standards list nothing checked and to
// hand the kernel coordinates in degrees. What a host test can reach, a host
// test can pin.
//
// It also deliberately does not import internal/standards: NewRegistry links
// all thirteen modules into the binary, and the kernel would then advertise
// twelve standards it has no entry point for — the precise dishonesty the
// evidence-tier field exists to prevent.
package wasmkernel

import (
	"encoding/json"
	"fmt"

	"github.com/aconiq/backend/internal/standards/descriptorjson"
	"github.com/aconiq/backend/internal/standards/framework"
	rls19road "github.com/aconiq/backend/internal/standards/rls19/road"
)

// Standard is one standard the kernel can actually run.
//
// Descriptor is what `aconiq.standards()` publishes; Entry is the name of the
// `window.aconiq` function that computes it. They are one struct because the
// failure this package exists to remove is a kernel that advertises a standard
// it cannot compute — and a list of descriptors alone can drift into exactly
// that without anything noticing.
type Standard struct {
	Descriptor framework.StandardDescriptor
	Entry      string
}

// Standards is the single list of what the kernel can run, in the order
// `aconiq.standards()` reports it.
//
// Adding a standards module to the kernel means adding it here *and* giving it
// an entry point in cmd/wasm/main.go; TestEveryStandardHasItsEntryPoint reads
// that file and fails when the two disagree.
func Standards() []Standard {
	return []Standard{
		{Descriptor: rls19road.Descriptor(), Entry: "rls19Road"},
	}
}

// Descriptors returns the descriptors of Standards, in the same order.
func Descriptors() []framework.StandardDescriptor {
	standards := Standards()

	descriptors := make([]framework.StandardDescriptor, 0, len(standards))
	for _, standard := range standards {
		descriptors = append(descriptors, standard.Descriptor)
	}

	return descriptors
}

// Supports reports whether the kernel has an entry point for a standard ID.
func Supports(id string) bool {
	for _, standard := range Standards() {
		if standard.Descriptor.ID == id {
			return true
		}
	}

	return false
}

// StandardsJSON renders the kernel's standards exactly as
// `GET /api/v1/standards` renders the server's, through the same
// descriptorjson encoding. A browser-mode client and an HTTP-mode client
// therefore parse one shape, and a standard's parameters, units, enum and
// evidence tier read the same in both.
func StandardsJSON() ([]byte, error) {
	out, err := json.Marshal(descriptorjson.FromDescriptors(Descriptors()))
	if err != nil {
		return nil, fmt.Errorf("marshal kernel standards: %w", err)
	}

	return out, nil
}
