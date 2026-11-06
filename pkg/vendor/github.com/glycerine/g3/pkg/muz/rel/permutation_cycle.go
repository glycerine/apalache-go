package rel

// Source: src/muz/base/dl_util.cpp try_remove_cycle_from_permutation.
// The caller supplies a valid permutation of its index range.
func removePermutationCycle(permutation []uint32) ([]uint32, bool) {
	for i := range permutation {
		if permutation[i] == uint32(i) {
			continue
		}
		var cycle []uint32
		for current := uint32(i); ; {
			cycle = append(cycle, current)
			next := permutation[current]
			permutation[current] = current
			if next == uint32(i) {
				return cycle, true
			}
			current = next
		}
	}
	return nil, false
}
