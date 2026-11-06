package rel

func (target *SparseTable) FilterNegationFunctional(negative *SparseTable, targetColumns, negativeColumns []int) (int, error) {
	if err := target.open(); err != nil {
		return 0, err
	}
	if err := negative.open(); err != nil {
		return 0, err
	}
	if err := validateColumns(target.signature, negative.signature, targetColumns, negativeColumns); err != nil {
		return 0, err
	}
	if sparseJoinInvolvesFunctional(target, negative, targetColumns, negativeColumns) {
		return 0, ErrTable
	}
	targetFacts, err := target.Facts()
	if err != nil {
		return 0, err
	}
	negativeFacts, err := negative.Facts()
	if err != nil {
		return 0, err
	}
	if len(targetColumns) == 0 {
		if len(negativeFacts) == 0 {
			return 0, nil
		}
		if err := target.Reset(); err != nil {
			return 0, err
		}
		return len(targetFacts), nil
	}
	remove := make([]Fact, 0)
	for _, fact := range targetFacts {
		matched := false
		for _, excluded := range negativeFacts {
			match := true
			for i := range targetColumns {
				if fact[targetColumns[i]] != excluded[negativeColumns[i]] {
					match = false
					break
				}
			}
			if match {
				matched = true
				break
			}
		}
		if matched {
			remove = append(remove, fact)
		}
	}
	// The C++ sparse filter collects target offsets in ascending order and
	// removes from the end so swap-with-last deletion preserves row order.
	for index := len(remove) - 1; index >= 0; index-- {
		if _, err := target.Remove(remove[index]); err != nil {
			return len(remove) - 1 - index, err
		}
	}
	return len(remove), nil
}
