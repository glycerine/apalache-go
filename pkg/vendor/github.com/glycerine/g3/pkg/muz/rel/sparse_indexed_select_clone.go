package rel

func (plan *SparseIndexedSelectPlan) Clone() *SparseIndexedSelectPlan {
	if plan == nil {
		return nil
	}
	return &SparseIndexedSelectPlan{
		signature: Signature{domains: plan.signature.Domains()}, functional: plan.functional,
		column: plan.column, value: plan.value,
		resultSignature: Signature{domains: plan.resultSignature.Domains()}, resultFunctional: plan.resultFunctional,
	}
}

func (plan *SparseIndexedSelectPlan) Selection() (Element, int, error) {
	if plan == nil {
		return 0, 0, ErrTable
	}
	return plan.value, plan.column, nil
}
