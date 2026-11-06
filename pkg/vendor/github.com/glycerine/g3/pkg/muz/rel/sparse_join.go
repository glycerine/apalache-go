package rel

func sparseJoinTraversal(leftFacts, rightFacts []Fact, leftColumns, rightColumns []int, emit func(Fact, Fact) error) error {
	match := func(left, right Fact, leftCols, rightCols []int) bool {
		for index := range leftCols {
			if left[leftCols[index]] != right[rightCols[index]] {
				return false
			}
		}
		return true
	}
	swapped := (len(leftFacts) > len(rightFacts)) == (len(leftColumns) != 0)
	if swapped {
		for _, right := range rightFacts {
			for _, left := range leftFacts {
				if match(left, right, leftColumns, rightColumns) {
					if err := emit(left, right); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	for _, left := range leftFacts {
		for _, right := range rightFacts {
			if match(left, right, leftColumns, rightColumns) {
				if err := emit(left, right); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (a *SparseTable) Join(b *SparseTable, left, right []int) (*SparseTable, error) {
	if e := a.open(); e != nil {
		return nil, e
	}
	if e := b.open(); e != nil {
		return nil, e
	}
	if e := validateColumns(a.signature, b.signature, left, right); e != nil {
		return nil, e
	}
	involvesFunctional := sparseJoinInvolvesFunctional(a, b, left, right)
	// Source sparse_table_plugin::mk_join_fn delegates every supported join to
	// its indexed join-project implementation. Preserve Go's explicit
	// zero-column sparse-table extension through the generic path below.
	if !involvesFunctional && a.signature.Len()+b.signature.Len() != 0 {
		plan, err := NewSparseIndexedJoinPlan(a, b, left, right, nil)
		if err != nil {
			return nil, err
		}
		result, _, err := plan.Execute(a, b)
		return result, err
	}
	// The sparse plugin declines indexes on functional join columns. The
	// relation manager then uses its generic nested-loop join.
	sig, functional, err := sparseJoinFunctionalSignature(a, b)
	if err != nil {
		return nil, err
	}
	out, err := NewSparseTable(sig, functional)
	if err != nil {
		return nil, err
	}
	af, err := a.Facts()
	if err != nil {
		_ = out.Close()
		return nil, err
	}
	bf, err := b.Facts()
	if err != nil {
		_ = out.Close()
		return nil, err
	}
	if err = sparseJoinTraversal(af, bf, left, right, func(x, y Fact) error {
		_, addErr := out.Add(sparseFunctionalJoinFact(x, y, a.signature.Len()-a.functional, b.signature.Len()-b.functional))
		return addErr
	}); err != nil {
		_ = out.Close()
		return nil, err
	}
	return out, nil
}
func (a *SparseTable) JoinProject(b *SparseTable, left, right, removed []int) (*SparseTable, error) {
	if err := a.open(); err != nil {
		return nil, err
	}
	if err := b.open(); err != nil {
		return nil, err
	}
	if err := validateColumns(a.signature, b.signature, left, right); err != nil {
		return nil, err
	}
	joinedSignature, _, err := sparseJoinFunctionalSignature(a, b)
	if err != nil {
		return nil, err
	}
	if _, err = projectedSignature(joinedSignature, removed); err != nil {
		return nil, err
	}
	if len(removed) < joinedSignature.Len() && !sparseJoinInvolvesFunctional(a, b, left, right) {
		plan, planErr := NewSparseIndexedJoinPlan(a, b, left, right, removed)
		if planErr != nil {
			return nil, planErr
		}
		result, _, executeErr := plan.Execute(a, b)
		return result, executeErr
	}
	return sparseJoinProjectFallback(a, b, left, right, removed)
}
