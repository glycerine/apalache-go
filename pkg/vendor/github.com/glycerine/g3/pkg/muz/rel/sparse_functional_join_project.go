package rel

func (left *SparseTable) JoinProjectFunctional(right *SparseTable, leftColumns, rightColumns, removed []int) (*SparseTable, error) {
	joined, err := left.JoinFunctional(right, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	defer joined.Close()
	return joined.ProjectFunctional(removed)
}
