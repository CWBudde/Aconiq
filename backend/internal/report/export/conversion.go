package export

import "fmt"

//nolint:gosec // Bounds-checked integer narrowing centralized for exporter offsets and sizes.
func mustUint32(value int) uint32 {
	if value < 0 {
		panic(fmt.Sprintf("value %d must be non-negative", value))
	}

	return uint32(value)
}

func mustUint16(value int) uint16 {
	if value < 0 || value > int(^uint16(0)) {
		panic(fmt.Sprintf("value %d is out of range for uint16", value))
	}

	return uint16(value)
}
