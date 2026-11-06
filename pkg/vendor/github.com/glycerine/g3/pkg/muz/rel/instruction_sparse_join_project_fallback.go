package rel

import "errors"

func finiteTableFunctionalColumns(table FiniteTable) int {
	switch concrete := table.(type) {
	case *SparseTable:
		return concrete.functional
	case *LazySparseTable:
		return concrete.root.functional
	default:
		return 0
	}
}

// finiteTableSparseJoinProjectReduce handles the source default-table
// join-project path when mixed plugins select Sparse for a functional result.
// A surviving equivalent key makes reductions unreachable and preserves the
// functional suffix across projection.
func finiteTableSparseJoinProjectReduce(first, second, joined FiniteTable, leftColumns, rightColumns, removed []int) (_ FiniteTable, handled bool, resultErr error) {
	sparse, ok := joined.(*SparseTable)
	if !ok || len(removed) == 0 {
		return nil, false, nil
	}
	firstSignature, err := first.Signature()
	if err != nil {
		return nil, true, err
	}
	secondSignature, err := second.Signature()
	if err != nil {
		return nil, true, err
	}
	functional, err := sparseJoinProjectFunctionalCountSignatures(
		firstSignature, finiteTableFunctionalColumns(first),
		secondSignature, finiteTableFunctionalColumns(second),
		leftColumns, rightColumns, removed)
	if err != nil {
		return nil, true, err
	}
	if functional == 0 {
		return nil, false, nil
	}
	collided := false
	result, err := sparse.ProjectWithReduce(removed, func(_, _ []Element) { collided = true })
	if err != nil {
		return nil, true, err
	}
	if collided {
		return nil, true, errors.Join(ErrTable, result.Close())
	}
	actual, err := result.FunctionalColumns()
	if err != nil || actual != functional {
		return nil, true, errors.Join(err, ErrTable, result.Close())
	}
	return result, true, nil
}
