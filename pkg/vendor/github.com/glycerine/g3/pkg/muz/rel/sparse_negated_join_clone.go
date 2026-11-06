package rel

func (plan *SparseNegatedJoinPlan) Clone() *SparseNegatedJoinPlan {
	if plan == nil {
		return nil
	}
	return &SparseNegatedJoinPlan{
		targetSignature: Signature{domains: plan.targetSignature.Domains()}, firstSignature: Signature{domains: plan.firstSignature.Domains()}, secondSignature: Signature{domains: plan.secondSignature.Domains()},
		targetFunctional: plan.targetFunctional, firstFunctional: plan.firstFunctional, secondFunctional: plan.secondFunctional,
		targetColumns: append([]int(nil), plan.targetColumns...), sourceColumns: append([]int(nil), plan.sourceColumns...),
		firstJoinColumns: append([]int(nil), plan.firstJoinColumns...), secondJoinColumns: append([]int(nil), plan.secondJoinColumns...),
	}
}

func (plan *SparseNegatedJoinPlan) Columns() (target, source, firstJoin, secondJoin []int) {
	if plan == nil {
		return nil, nil, nil, nil
	}
	return append([]int(nil), plan.targetColumns...), append([]int(nil), plan.sourceColumns...), append([]int(nil), plan.firstJoinColumns...), append([]int(nil), plan.secondJoinColumns...)
}
