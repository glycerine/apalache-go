package rel

import "errors"

// sparseJoinProjectFallback mirrors relation_manager's default join-project
// composition when the sparse indexed implementation declines a functional
// join. If an equivalent nonfunctional key survives projection, source uses
// project-with-reduce to preserve the functional suffix; its reducer is
// unreachable because the surviving equivalent key prevents row merging.
func sparseJoinProjectFallback(left, right *SparseTable, leftColumns, rightColumns, removed []int) (result *SparseTable, resultErr error) {
	joined, err := left.Join(right, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	if len(removed) == 0 {
		return joined, nil
	}
	defer func() {
		if closeErr := joined.Close(); closeErr != nil {
			if result != nil {
				closeErr = errors.Join(closeErr, result.Close())
				result = nil
			}
			resultErr = errors.Join(resultErr, closeErr)
		}
	}()
	functional, err := sparseJoinProjectFunctionalCount(left, right, leftColumns, rightColumns, removed)
	if err != nil {
		return nil, err
	}
	if functional == 0 {
		return joined.Project(removed)
	}
	collided := false
	result, err = joined.ProjectWithReduce(removed, func(_, _ []Element) { collided = true })
	if err != nil {
		return nil, err
	}
	if collided {
		return nil, errors.Join(ErrTable, result.Close())
	}
	actual, err := result.FunctionalColumns()
	if err != nil || actual != functional {
		return nil, errors.Join(err, ErrTable, result.Close())
	}
	return result, nil
}
