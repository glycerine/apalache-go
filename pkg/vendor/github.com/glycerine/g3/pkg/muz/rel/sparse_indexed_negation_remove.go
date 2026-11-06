package rel

func removeSparseIndexedNegationFacts(target *SparseTable, targetFacts []Fact, marked map[string]bool) (int, error) {
	removed := 0
	// C++ removes descending physical offsets so swap-removal cannot invalidate a
	// pending offset. Reverse source order has the same property here.
	for i := len(targetFacts) - 1; i >= 0; i-- {
		if !marked[factKey(targetFacts[i])] {
			continue
		}
		changed, err := target.Remove(targetFacts[i])
		if err != nil {
			return removed, err
		}
		if changed {
			removed++
		}
	}
	return removed, nil
}
