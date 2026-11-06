package productiter

// Package productiter implements the retained uint32 product-coordinate step
// from src/util/util.cpp:112-119.

// Next advances the first coordinate fastest and resets completed coordinates.
// The caller supplies equal-length slices, as at the source call site.
func Next(sizes, coordinates []uint32) bool {
	for i := range sizes {
		coordinates[i]++
		if coordinates[i] < sizes[i] {
			return true
		}
		coordinates[i] = 0
	}
	return false
}
