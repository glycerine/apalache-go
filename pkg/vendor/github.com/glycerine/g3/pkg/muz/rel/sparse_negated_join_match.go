package rel

func (plan *SparseNegatedJoinPlan) targetMatchesFirst(target, first Fact) bool {
	for i, sourceColumn := range plan.sourceColumns {
		if sourceColumn < plan.firstSignature.Len() && target[plan.targetColumns[i]] != first[sourceColumn] {
			return false
		}
	}
	return true
}

func (plan *SparseNegatedJoinPlan) targetMatchesSecond(target, second Fact) bool {
	firstWidth := plan.firstSignature.Len()
	for i, sourceColumn := range plan.sourceColumns {
		if sourceColumn >= firstWidth && target[plan.targetColumns[i]] != second[sourceColumn-firstWidth] {
			return false
		}
	}
	return true
}

func (plan *SparseNegatedJoinPlan) sourcesJoin(first, second Fact) bool {
	for i, firstColumn := range plan.firstJoinColumns {
		if first[firstColumn] != second[plan.secondJoinColumns[i]] {
			return false
		}
	}
	return true
}
