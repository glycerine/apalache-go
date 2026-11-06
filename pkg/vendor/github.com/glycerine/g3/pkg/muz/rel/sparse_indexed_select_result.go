package rel

func (plan *SparseIndexedSelectPlan) newResult() (*SparseTable, error) {
	if plan == nil {
		return nil, ErrTable
	}
	return NewSparseTable(plan.resultSignature, plan.resultFunctional)
}

func (plan *SparseIndexedSelectPlan) ResultSignature() (Signature, int, error) {
	if plan == nil {
		return Signature{}, 0, ErrTable
	}
	return Signature{domains: plan.resultSignature.Domains()}, plan.resultFunctional, nil
}
