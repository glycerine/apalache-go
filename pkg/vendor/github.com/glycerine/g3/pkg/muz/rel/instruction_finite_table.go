package rel

import "errors"

// Source: src/muz/rel/dl_relation_manager.cpp default table-operation
// fallbacks. Concrete plugins are used when both operands agree; mixed finite
// representations use the source row-wise fallback without changing either
// stored relation owner.

func finiteInstructionRegister(context *ExecutionContext, index uint32) (FiniteTable, error) {
	if index == VoidRegister {
		return nil, nil
	}
	return context.FiniteRegister(index)
}

func setNonemptyFiniteRegister(context *ExecutionContext, index uint32, table FiniteTable) error {
	if table == nil {
		return context.MakeEmpty(index)
	}
	count, err := table.Len()
	if err != nil {
		return err
	}
	// Empty sparse owners carry their concrete representation through later
	// unions and stores. Hash empties preserve the established nil fast path.
	if count == 0 {
		switch table.(type) {
		case *SparseTable, *FiniteCheckedPair, *FormulaCheckedTable, *FormulaCheckedFinitePair:
			// Preserve concrete empty owners selected by the relation manager.
		default:
			if err = table.Close(); err != nil {
				return err
			}
			return context.MakeEmpty(index)
		}
	}
	return context.SetFiniteRegister(index, table)
}

func emptyFiniteLike(table FiniteTable) (FiniteTable, error) {
	signature, err := table.Signature()
	if err != nil {
		return nil, err
	}
	if sparse, ok := table.(*SparseTable); ok && signature.Len() != 0 {
		return NewSparseTable(signature, sparse.functional)
	}
	if lazy, ok := table.(*LazySparseTable); ok {
		return lazySparseInstructionResult(lazy, signature, lazy.root.functional)
	}
	if checked, ok := table.(*FiniteCheckedPair); ok {
		return checked.EmptyLike()
	}
	if checked, ok := table.(*FormulaCheckedTable); ok {
		return emptyFormulaCheckedLike(checked)
	}
	if checked, ok := table.(*FormulaCheckedFinitePair); ok {
		return checked.EmptyLike()
	}
	return NewHashTable(signature), nil
}

func newFiniteJoinResult(signature Signature, functional int, first, second FiniteTable) (FiniteTable, error) {
	// Source default_table_join_fn asks each actual plugin whether it can
	// handle the result. Lazy Sparse inherits Sparse's nonnullary capability;
	// its same-kind direct join above this fallback can still return Lazy.
	if lazy, ok := first.(*LazySparseTable); ok && signature.Len() != 0 {
		return lazySparseInstructionResult(lazy, signature, functional)
	}
	if _, sparse := first.(*SparseTable); sparse && signature.Len() != 0 {
		return NewSparseTable(signature, functional)
	}
	if functional == 0 {
		if _, hash := first.(*HashTable); hash {
			return NewHashTable(signature), nil
		}
	}
	if _, sparse := second.(*SparseTable); sparse && signature.Len() != 0 {
		return NewSparseTable(signature, functional)
	}
	if lazy, ok := second.(*LazySparseTable); ok && signature.Len() != 0 {
		return lazySparseInstructionResult(lazy, signature, functional)
	}
	return NewHashTable(signature), nil
}

func finiteTableJoin(first, second FiniteTable, left, right []int) (FiniteTable, error) {
	if a, ok := first.(*FormulaCheckedFinitePair); ok {
		b, err := liftFormulaCheckedFinitePairLike(a, second)
		if err != nil {
			return nil, err
		}
		result, joinErr := a.Join(b, left, right)
		if b != second {
			joinErr = errors.Join(joinErr, b.Close())
		}
		return result, joinErr
	}
	if b, ok := second.(*FormulaCheckedFinitePair); ok {
		a, err := liftFormulaCheckedFinitePairLike(b, first)
		if err != nil {
			return nil, err
		}
		result, joinErr := a.Join(b, left, right)
		if a != first {
			joinErr = errors.Join(joinErr, a.Close())
		}
		return result, joinErr
	}
	switch a := first.(type) {
	case *HashTable:
		if b, ok := second.(*HashTable); ok {
			return a.Join(b, left, right)
		}
	case *SparseTable:
		if b, ok := second.(*SparseTable); ok {
			return a.Join(b, left, right)
		}
	case *LazySparseTable:
		if b, ok := second.(*LazySparseTable); ok {
			return a.Join(b, left, right)
		}
	case *FiniteCheckedPair:
		if b, ok := second.(*FiniteCheckedPair); ok {
			return a.Join(b, left, right)
		}
	case *FormulaCheckedTable:
		b, err := liftFormulaCheckedLike(a, second)
		if err != nil {
			return nil, err
		}
		result, joinErr := a.Join(b, left, right)
		if b != second {
			joinErr = errors.Join(joinErr, b.Close())
		}
		return result, joinErr
	}
	if b, ok := second.(*FormulaCheckedTable); ok {
		a, err := liftFormulaCheckedLike(b, first)
		if err != nil {
			return nil, err
		}
		result, joinErr := a.Join(b, left, right)
		if a != first {
			joinErr = errors.Join(joinErr, a.Close())
		}
		return result, joinErr
	}
	if _, firstChecked := first.(*FiniteCheckedPair); firstChecked {
		return checkedPairMixedJoin(first, second, left, right)
	}
	if _, secondChecked := second.(*FiniteCheckedPair); secondChecked {
		return checkedPairMixedJoin(first, second, left, right)
	}
	firstSignature, err := first.Signature()
	if err != nil {
		return nil, err
	}
	secondSignature, err := second.Signature()
	if err != nil {
		return nil, err
	}
	if err = validateColumns(firstSignature, secondSignature, left, right); err != nil {
		return nil, err
	}
	firstFunctional, secondFunctional := 0, 0
	if sparse, ok := first.(*SparseTable); ok {
		firstFunctional = sparse.functional
	} else if lazy, ok := first.(*LazySparseTable); ok {
		firstFunctional = lazy.root.functional
	}
	if sparse, ok := second.(*SparseTable); ok {
		secondFunctional = sparse.functional
	} else if lazy, ok := second.(*LazySparseTable); ok {
		secondFunctional = lazy.root.functional
	}
	firstDomains, secondDomains := firstSignature.Domains(), secondSignature.Domains()
	firstBoundary := len(firstDomains) - firstFunctional
	secondBoundary := len(secondDomains) - secondFunctional
	domains := make([]uint64, 0, len(firstDomains)+len(secondDomains))
	domains = append(domains, firstDomains[:firstBoundary]...)
	domains = append(domains, secondDomains[:secondBoundary]...)
	domains = append(domains, firstDomains[firstBoundary:]...)
	domains = append(domains, secondDomains[secondBoundary:]...)
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	result, err := newFiniteJoinResult(signature, firstFunctional+secondFunctional, first, second)
	if err != nil {
		return nil, err
	}
	firstFacts, err := first.Facts()
	if err != nil {
		_ = result.Close()
		return nil, err
	}
	secondFacts, err := second.Facts()
	if err != nil {
		_ = result.Close()
		return nil, err
	}
	for _, a := range firstFacts {
		for _, b := range secondFacts {
			match := true
			for column := range left {
				if a[left[column]] != b[right[column]] {
					match = false
					break
				}
			}
			if match {
				fact := sparseFunctionalJoinFact(a, b, len(a)-firstFunctional, len(b)-secondFunctional)
				if _, err = result.Add(fact); err != nil {
					_ = result.Close()
					return nil, err
				}
			}
		}
	}
	return result, nil
}

func finiteTableProject(source FiniteTable, removed []int) (FiniteTable, error) {
	if len(removed) == 0 {
		return source.CloneFinite()
	}
	if lazy, ok := source.(*LazySparseTable); ok {
		return lazy.Project(removed)
	}
	if checked, ok := source.(*FiniteCheckedPair); ok {
		return checked.Project(removed)
	}
	if checked, ok := source.(*FormulaCheckedTable); ok {
		return checked.Project(removed)
	}
	if checked, ok := source.(*FormulaCheckedFinitePair); ok {
		return checked.Project(removed)
	}
	signature, err := source.Signature()
	if err != nil {
		return nil, err
	}
	projected, err := projectedSignature(signature, removed)
	if err != nil {
		return nil, err
	}
	if projected.Len() != 0 {
		switch table := source.(type) {
		case *HashTable:
			return table.Project(removed)
		case *SparseTable:
			return table.Project(removed)
		}
	}
	result := NewHashTable(projected)
	remove := make([]bool, signature.Len())
	for _, column := range removed {
		remove[column] = true
	}
	facts, err := source.Facts()
	if err != nil {
		_ = result.Close()
		return nil, err
	}
	for _, fact := range facts {
		row := make(Fact, 0, projected.Len())
		for column, value := range fact {
			if !remove[column] {
				row = append(row, value)
			}
		}
		if _, err = result.Add(row); err != nil {
			_ = result.Close()
			return nil, err
		}
	}
	return result, nil
}

func finiteTableJoinProject(first, second FiniteTable, left, right, removed []int) (FiniteTable, error) {
	if a, ok := first.(*FormulaCheckedFinitePair); ok {
		b, err := liftFormulaCheckedFinitePairLike(a, second)
		if err != nil {
			return nil, err
		}
		result, joinErr := a.JoinProject(b, left, right, removed)
		if b != second {
			joinErr = errors.Join(joinErr, b.Close())
		}
		return result, joinErr
	}
	if b, ok := second.(*FormulaCheckedFinitePair); ok {
		a, err := liftFormulaCheckedFinitePairLike(b, first)
		if err != nil {
			return nil, err
		}
		result, joinErr := a.JoinProject(b, left, right, removed)
		if a != first {
			joinErr = errors.Join(joinErr, a.Close())
		}
		return result, joinErr
	}
	switch a := first.(type) {
	case *HashTable:
		if b, ok := second.(*HashTable); ok {
			return a.JoinProject(b, left, right, removed)
		}
	case *SparseTable:
		if b, ok := second.(*SparseTable); ok {
			joinedSignature, _, err := sparseJoinFunctionalSignature(a, b)
			if err != nil {
				return nil, err
			}
			projected, err := projectedSignature(joinedSignature, removed)
			if err != nil {
				return nil, err
			}
			if projected.Len() != 0 {
				return a.JoinProject(b, left, right, removed)
			}
		}
	case *LazySparseTable:
		if b, ok := second.(*LazySparseTable); ok {
			return lazySparseInstructionJoinProject(a, b, left, right, removed)
		}
	case *FiniteCheckedPair:
		if b, ok := second.(*FiniteCheckedPair); ok {
			return a.JoinProject(b, left, right, removed)
		}
	case *FormulaCheckedTable:
		b, err := liftFormulaCheckedLike(a, second)
		if err != nil {
			return nil, err
		}
		result, joinErr := a.JoinProject(b, left, right, removed)
		if b != second {
			joinErr = errors.Join(joinErr, b.Close())
		}
		return result, joinErr
	}
	if b, ok := second.(*FormulaCheckedTable); ok {
		a, err := liftFormulaCheckedLike(b, first)
		if err != nil {
			return nil, err
		}
		result, joinErr := a.JoinProject(b, left, right, removed)
		if a != first {
			joinErr = errors.Join(joinErr, a.Close())
		}
		return result, joinErr
	}
	if _, firstChecked := first.(*FiniteCheckedPair); firstChecked {
		return checkedPairMixedJoinProject(first, second, left, right, removed)
	}
	if _, secondChecked := second.(*FiniteCheckedPair); secondChecked {
		return checkedPairMixedJoinProject(first, second, left, right, removed)
	}
	joined, err := finiteTableJoin(first, second, left, right)
	if err != nil {
		return nil, err
	}
	result, handled, projectErr := finiteTableSparseJoinProjectReduce(first, second, joined, left, right, removed)
	if !handled {
		result, projectErr = finiteTableProject(joined, removed)
	}
	closeErr := joined.Close()
	if projectErr != nil || closeErr != nil {
		if result != nil {
			closeErr = errors.Join(closeErr, result.Close())
		}
		return nil, errors.Join(projectErr, closeErr)
	}
	return result, nil
}

func finiteTableRename(source FiniteTable, cycle []int) (FiniteTable, error) {
	switch table := source.(type) {
	case *HashTable:
		return table.RenameCycle(cycle)
	case *SparseTable:
		return table.Rename(cycle)
	case *LazySparseTable:
		return table.RenameCycle(cycle)
	case *FiniteCheckedPair:
		return table.RenameCycle(cycle)
	case *FormulaCheckedTable:
		return table.RenameCycle(cycle)
	case *FormulaCheckedFinitePair:
		return table.RenameCycle(cycle)
	default:
		return nil, ErrCompilerUnsupported
	}
}

func finiteTableFilterEqual(table FiniteTable, value Element, column int) error {
	switch concrete := table.(type) {
	case *HashTable:
		return concrete.FilterEqual(value, column)
	case *SparseTable:
		return concrete.FilterEqual(value, column)
	case *LazySparseTable:
		return concrete.FilterEqual(value, column)
	case *FiniteCheckedPair:
		return concrete.FilterEqual(value, column)
	case *FormulaCheckedTable:
		return formulaCheckedFilterEqual(concrete, value, column)
	case *FormulaCheckedFinitePair:
		return concrete.FilterEqual(value, column)
	default:
		return ErrCompilerUnsupported
	}
}

func finiteTableFilterIdentical(table FiniteTable, columns []int) error {
	switch concrete := table.(type) {
	case *HashTable:
		return concrete.FilterIdentical(columns)
	case *SparseTable:
		return concrete.FilterIdentical(columns)
	case *LazySparseTable:
		return concrete.FilterIdentical(columns)
	case *FiniteCheckedPair:
		return concrete.FilterIdentical(columns)
	case *FormulaCheckedTable:
		return concrete.FilterIdentical(columns)
	case *FormulaCheckedFinitePair:
		return concrete.FilterIdentical(columns)
	default:
		return ErrCompilerUnsupported
	}
}

func finiteTableSelectProject(table FiniteTable, value Element, column int) (FiniteTable, error) {
	signature, err := table.Signature()
	if err != nil || column < 0 || column >= signature.Len() {
		return nil, ErrTable
	}
	if concrete, ok := table.(*LazySparseTable); ok {
		return lazySparseInstructionSelectProject(concrete, value, column)
	}
	if concrete, ok := table.(*FiniteCheckedPair); ok {
		return concrete.SelectEqualAndProject(value, column)
	}
	if _, ok := table.(*FormulaCheckedTable); ok {
		clone, err := table.CloneFinite()
		if err != nil {
			return nil, err
		}
		if err = finiteTableFilterEqual(clone, value, column); err != nil {
			_ = clone.Close()
			return nil, err
		}
		result, projectErr := finiteTableProject(clone, []int{column})
		return result, errors.Join(projectErr, clone.Close())
	}
	if concrete, ok := table.(*FormulaCheckedFinitePair); ok {
		return concrete.SelectEqualAndProject(value, column)
	}
	if signature.Len() == 1 {
		clone, err := table.CloneFinite()
		if err != nil {
			return nil, err
		}
		if err = finiteTableFilterEqual(clone, value, column); err != nil {
			_ = clone.Close()
			return nil, err
		}
		result, err := finiteTableProject(clone, []int{column})
		closeErr := clone.Close()
		if err != nil || closeErr != nil {
			if result != nil {
				closeErr = errors.Join(closeErr, result.Close())
			}
			return nil, errors.Join(err, closeErr)
		}
		return result, nil
	}
	switch concrete := table.(type) {
	case *HashTable:
		return concrete.SelectEqualAndProject(value, column)
	case *SparseTable:
		return concrete.SelectEqualAndProject(value, column)
	default:
		return nil, ErrCompilerUnsupported
	}
}

func finiteTableUnion(target, source, delta FiniteTable) error {
	targetSignature, err := target.Signature()
	if err != nil {
		return err
	}
	sourceSignature, err := source.Signature()
	if err != nil || !targetSignature.equal(sourceSignature) {
		return ErrTable
	}
	if delta != nil {
		deltaSignature, x := delta.Signature()
		if x != nil || !targetSignature.equal(deltaSignature) {
			return ErrTable
		}
	}
	if a, ok := target.(*HashTable); ok {
		if b, same := source.(*HashTable); same {
			if delta == nil {
				return a.Union(b, nil)
			}
			if d, sameDelta := delta.(*HashTable); sameDelta {
				return a.Union(b, d)
			}
		}
	}
	if a, ok := target.(*SparseTable); ok {
		if b, same := source.(*SparseTable); same {
			if delta == nil {
				return a.Union(b, nil)
			}
			if d, sameDelta := delta.(*SparseTable); sameDelta {
				return a.Union(b, d)
			}
		}
	}
	if a, ok := target.(*LazySparseTable); ok {
		if b, same := source.(*LazySparseTable); same {
			if delta == nil {
				return a.UnionFrom(b, nil)
			}
			if d, sameDelta := delta.(*LazySparseTable); sameDelta {
				return a.UnionFrom(b, d)
			}
		}
	}
	if a, ok := target.(*FiniteCheckedPair); ok {
		if b, same := source.(*FiniteCheckedPair); same {
			if delta == nil {
				return a.UnionFrom(b, nil)
			}
			if d, sameDelta := delta.(*FiniteCheckedPair); sameDelta {
				return a.UnionFrom(b, d)
			}
		}
		if delta == nil {
			return checkedPairMixedUnion(a, source, nil)
		}
		if d, sameDelta := delta.(*FiniteCheckedPair); sameDelta {
			return checkedPairMixedUnion(a, source, d)
		}
	}
	if a, ok := target.(*FormulaCheckedTable); ok {
		b, same := source.(*FormulaCheckedTable)
		if !same {
			return ErrCompilerUnsupported
		}
		if delta == nil {
			return a.UnionFrom(b, nil)
		}
		d, same := delta.(*FormulaCheckedTable)
		if !same {
			return ErrCompilerUnsupported
		}
		return a.UnionFrom(b, d)
	}
	if a, ok := target.(*FormulaCheckedFinitePair); ok {
		b, same := source.(*FormulaCheckedFinitePair)
		if !same {
			return ErrCompilerUnsupported
		}
		if delta == nil {
			return a.UnionFrom(b, nil)
		}
		d, same := delta.(*FormulaCheckedFinitePair)
		if !same {
			return ErrCompilerUnsupported
		}
		return a.UnionFrom(b, d)
	}
	facts, err := source.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		inserted, x := target.Add(fact)
		if x != nil {
			return x
		}
		if inserted && delta != nil {
			if _, x = delta.Add(fact); x != nil {
				return x
			}
		}
	}
	return nil
}

func finiteTableFilterNegation(target, negative FiniteTable, targetColumns, negativeColumns []int) error {
	if a, ok := target.(*HashTable); ok {
		if b, same := negative.(*HashTable); same {
			return a.FilterByNegation(b, targetColumns, negativeColumns)
		}
	}
	if a, ok := target.(*SparseTable); ok {
		if b, same := negative.(*SparseTable); same {
			return a.FilterNegation(b, targetColumns, negativeColumns)
		}
	}
	if a, ok := target.(*LazySparseTable); ok {
		if b, same := negative.(*LazySparseTable); same {
			return a.FilterByNegation(b, targetColumns, negativeColumns)
		}
	}
	if a, ok := target.(*FiniteCheckedPair); ok {
		if b, same := negative.(*FiniteCheckedPair); same {
			return a.FilterByNegation(b, targetColumns, negativeColumns)
		}
		return checkedPairMixedNegation(a, negative, targetColumns, negativeColumns)
	}
	if a, ok := target.(*FormulaCheckedTable); ok {
		b, same := negative.(*FormulaCheckedTable)
		if !same {
			return ErrCompilerUnsupported
		}
		return a.FilterByNegation(b, targetColumns, negativeColumns)
	}
	if a, ok := target.(*FormulaCheckedFinitePair); ok {
		b, same := negative.(*FormulaCheckedFinitePair)
		if !same {
			return ErrCompilerUnsupported
		}
		return a.FilterByNegation(b, targetColumns, negativeColumns)
	}
	targetSignature, err := target.Signature()
	if err != nil {
		return err
	}
	negativeSignature, err := negative.Signature()
	if err != nil {
		return err
	}
	if err = validateColumns(targetSignature, negativeSignature, targetColumns, negativeColumns); err != nil {
		return err
	}
	targetFacts, err := target.Facts()
	if err != nil {
		return err
	}
	negativeFacts, err := negative.Facts()
	if err != nil {
		return err
	}
	kept := make([]Fact, 0, len(targetFacts))
	for _, fact := range targetFacts {
		remove := false
		for _, other := range negativeFacts {
			match := true
			for index := range targetColumns {
				if fact[targetColumns[index]] != other[negativeColumns[index]] {
					match = false
					break
				}
			}
			if match {
				remove = true
				break
			}
		}
		if !remove {
			kept = append(kept, fact)
		}
	}
	if err = target.Reset(); err != nil {
		return err
	}
	for _, fact := range kept {
		if _, err = target.Add(fact); err != nil {
			return err
		}
	}
	return nil
}
