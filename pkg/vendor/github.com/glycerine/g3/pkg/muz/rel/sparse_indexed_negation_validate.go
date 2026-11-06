package rel

func NewSparseIndexedNegationPlan(target, negative *SparseTable, targetColumns, negativeColumns []int) (*SparseIndexedNegationPlan, error) {
	if target == nil || negative == nil || target.open() != nil || negative.open() != nil {
		return nil, ErrTable
	}
	if err := validateColumns(target.signature, negative.signature, targetColumns, negativeColumns); err != nil {
		return nil, err
	}
	if sparseJoinInvolvesFunctional(target, negative, targetColumns, negativeColumns) {
		return nil, ErrTable
	}
	return &SparseIndexedNegationPlan{
		targetSignature: Signature{domains: target.signature.Domains()}, negativeSignature: Signature{domains: negative.signature.Domains()},
		targetFunctional: target.functional, negativeFunctional: negative.functional,
		targetColumns: append([]int(nil), targetColumns...), negativeColumns: append([]int(nil), negativeColumns...),
	}, nil
}

func (plan *SparseIndexedNegationPlan) validateTables(target, negative *SparseTable) error {
	if plan == nil || target == nil || negative == nil || target.open() != nil || negative.open() != nil {
		return ErrTable
	}
	if plan.targetFunctional != target.functional || plan.negativeFunctional != negative.functional ||
		!plan.targetSignature.equal(target.signature) || !plan.negativeSignature.equal(negative.signature) {
		return ErrTable
	}
	return nil
}
