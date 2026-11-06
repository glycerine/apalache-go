package rel

func (t *SparseTable) SelectEqualAndProject(value Element, column int) (*SparseTable, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if column < 0 || column >= t.signature.Len() {
		return nil, ErrTable
	}
	firstFunctional := t.signature.Len() - t.functional
	if t.signature.Len() > 1 && column < firstFunctional {
		plan, err := NewSparseIndexedSelectPlan(t, value, column)
		if err != nil {
			return nil, err
		}
		result, _, err := plan.Execute(t)
		return result, err
	}
	filtered, err := t.Clone()
	if err != nil {
		return nil, err
	}
	defer filtered.Close()
	if err := filtered.FilterEqual(value, column); err != nil {
		return nil, err
	}
	return filtered.Project([]int{column})
}
func (t *SparseTable) FilterNegation(negative *SparseTable, tCols, nCols []int) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := negative.open(); err != nil {
		return err
	}
	if err := validateColumns(t.signature, negative.signature, tCols, nCols); err != nil {
		return err
	}
	if sparseJoinInvolvesFunctional(t, negative, tCols, nCols) {
		// The sparse plugin refuses indexes on functional columns, so the
		// relation manager falls back to its generic row filter, which removes
		// the collected rows in table iteration order.
		facts, err := t.Facts()
		if err != nil {
			return err
		}
		negated, err := negative.Facts()
		if err != nil {
			return err
		}
		remove := make([]Fact, 0)
		for _, fact := range facts {
			for _, excluded := range negated {
				match := true
				for i := range tCols {
					if fact[tCols[i]] != excluded[nCols[i]] {
						match = false
						break
					}
				}
				if match {
					remove = append(remove, fact)
					break
				}
			}
		}
		_, err = t.RemoveFacts(remove)
		return err
	}
	plan, err := NewSparseIndexedNegationPlan(t, negative, tCols, nCols)
	if err != nil {
		return err
	}
	_, err = plan.Execute(t, negative)
	return err
}
