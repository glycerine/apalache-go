package rel

import "errors"

// Source: src/muz/rel/dl_relation_manager.cpp default_table_project_with_reduce_fn.

// SparseFunctionalRowReducer receives the existing and incoming functional
// suffixes when projection maps multiple rows to the same non-functional key.
type SparseFunctionalRowReducer func(existing, incoming []Element)

// ProjectWithReduce projects a functional sparse table and reports every
// functional collision to reducer. Mutations to existing replace the kept
// functional suffix, matching the source suggest/reduce/ensure sequence.
func (table *SparseTable) ProjectWithReduce(removed []int, reducer SparseFunctionalRowReducer) (_ *SparseTable, resultErr error) {
	if err := table.open(); err != nil {
		return nil, err
	}
	if table.functional == 0 || len(removed) == 0 || reducer == nil {
		return nil, ErrTable
	}
	signature, err := projectedSignature(table.signature, removed)
	if err != nil {
		return nil, err
	}
	functional := table.functional
	firstFunctional := table.signature.Len() - table.functional
	for _, column := range removed {
		if column >= firstFunctional {
			functional--
		}
	}
	result, err := NewSparseTable(signature, functional)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	facts, err := table.Facts()
	if err != nil {
		return nil, err
	}
	resultFirstFunctional := signature.Len() - functional
	for _, fact := range facts {
		candidate := sparseProjectFact(fact, removed)
		incoming := append([]Element(nil), candidate[resultFirstFunctional:]...)
		added, addErr := result.SuggestFact(candidate)
		if addErr != nil {
			return nil, addErr
		}
		if added {
			continue
		}
		existing := append([]Element(nil), candidate[resultFirstFunctional:]...)
		reducer(existing, incoming)
		copy(candidate[resultFirstFunctional:], existing)
		if err = result.EnsureFact(candidate); err != nil {
			return nil, err
		}
	}
	keep = true
	return result, nil
}

func (table *SparseTable) ProjectFunctional(removed []int) (*SparseTable, error) {
	if err := table.open(); err != nil {
		return nil, err
	}
	signature, err := projectedSignature(table.signature, removed)
	if err != nil {
		return nil, err
	}
	functional, err := sparseProjectFunctionalCount(table, removed)
	if err != nil {
		return nil, err
	}
	result, err := NewSparseTable(signature, functional)
	if err != nil {
		return nil, err
	}
	facts, err := table.Facts()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		if _, err = result.Add(sparseProjectFact(fact, removed)); err != nil {
			return nil, err
		}
	}
	return result, nil
}
