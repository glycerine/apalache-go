package rel

func NewSparseIndexedJoinPlan(left, right *SparseTable, leftColumns, rightColumns, removed []int) (*SparseIndexedJoinPlan, error) {
	if left == nil || right == nil || left.open() != nil || right.open() != nil {
		return nil, ErrTable
	}
	if err := validateColumns(left.signature, right.signature, leftColumns, rightColumns); err != nil {
		return nil, err
	}
	leftFirst := left.signature.Len() - left.functional
	rightFirst := right.signature.Len() - right.functional
	for i := range leftColumns {
		if leftColumns[i] >= leftFirst || rightColumns[i] >= rightFirst {
			return nil, ErrTable
		}
	}
	joined, _, err := sparseJoinFunctionalSignature(left, right)
	if err != nil || len(removed) == joined.Len() {
		return nil, ErrTable
	}
	result, err := projectedSignature(joined, removed)
	if err != nil {
		return nil, err
	}
	functional, err := sparseJoinProjectFunctionalCount(left, right, leftColumns, rightColumns, removed)
	if err != nil {
		return nil, err
	}
	return &SparseIndexedJoinPlan{
		leftSignature: Signature{domains: left.signature.Domains()}, rightSignature: Signature{domains: right.signature.Domains()},
		leftFunctional: left.functional, rightFunctional: right.functional,
		leftColumns: append([]int(nil), leftColumns...), rightColumns: append([]int(nil), rightColumns...),
		removed: append([]int(nil), removed...), resultSignature: result, resultFunctional: functional,
	}, nil
}

func (plan *SparseIndexedJoinPlan) validateTables(left, right *SparseTable) error {
	if plan == nil || left == nil || right == nil || left.open() != nil || right.open() != nil {
		return ErrTable
	}
	if plan.leftFunctional != left.functional || plan.rightFunctional != right.functional ||
		!plan.leftSignature.equal(left.signature) || !plan.rightSignature.equal(right.signature) {
		return ErrTable
	}
	return nil
}
