package rel

func (plan *SparseIndexedJoinPlan) Clone() *SparseIndexedJoinPlan {
	if plan == nil {
		return nil
	}
	return &SparseIndexedJoinPlan{
		leftSignature: Signature{domains: plan.leftSignature.Domains()}, rightSignature: Signature{domains: plan.rightSignature.Domains()},
		leftFunctional: plan.leftFunctional, rightFunctional: plan.rightFunctional,
		leftColumns: append([]int(nil), plan.leftColumns...), rightColumns: append([]int(nil), plan.rightColumns...),
		removed: append([]int(nil), plan.removed...), resultSignature: Signature{domains: plan.resultSignature.Domains()}, resultFunctional: plan.resultFunctional,
	}
}

func (plan *SparseIndexedJoinPlan) Columns() (left, right, removed []int) {
	if plan == nil {
		return nil, nil, nil
	}
	return append([]int(nil), plan.leftColumns...), append([]int(nil), plan.rightColumns...), append([]int(nil), plan.removed...)
}
