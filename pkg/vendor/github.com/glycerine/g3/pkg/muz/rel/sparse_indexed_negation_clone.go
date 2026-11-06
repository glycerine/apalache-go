package rel

func (plan *SparseIndexedNegationPlan) Clone() *SparseIndexedNegationPlan {
	if plan == nil {
		return nil
	}
	return &SparseIndexedNegationPlan{
		targetSignature: Signature{domains: plan.targetSignature.Domains()}, negativeSignature: Signature{domains: plan.negativeSignature.Domains()},
		targetFunctional: plan.targetFunctional, negativeFunctional: plan.negativeFunctional,
		targetColumns: append([]int(nil), plan.targetColumns...), negativeColumns: append([]int(nil), plan.negativeColumns...),
	}
}

func (plan *SparseIndexedNegationPlan) Columns() (target, negative []int) {
	if plan == nil {
		return nil, nil
	}
	return append([]int(nil), plan.targetColumns...), append([]int(nil), plan.negativeColumns...)
}
