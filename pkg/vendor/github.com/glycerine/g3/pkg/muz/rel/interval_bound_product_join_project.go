package rel

// Source: relation_manager::default_relation_join_project_fn composes the
// selected relation join with projection of the joined intermediate.

func JoinProjectIntervalBoundProducts(left, right *IntervalBoundProduct, leftColumns, rightColumns, removed []uint32) (*IntervalBoundProduct, error) {
	joined, err := JoinIntervalBoundProducts(left, right, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	return joined.Project(removed)
}
