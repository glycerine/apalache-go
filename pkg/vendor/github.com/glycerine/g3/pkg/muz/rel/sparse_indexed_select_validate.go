package rel

func NewSparseIndexedSelectPlan(table *SparseTable, value Element, column int) (*SparseIndexedSelectPlan, error) {
	if table == nil || table.open() != nil || table.signature.Len() == 1 {
		return nil, ErrTable
	}
	firstFunctional := table.signature.Len() - table.functional
	if column < 0 || column >= firstFunctional {
		return nil, ErrTable
	}
	resultSignature, err := projectedSignature(table.signature, []int{column})
	if err != nil {
		return nil, err
	}
	resultFunctional, err := sparseProjectFunctionalCount(table, []int{column})
	if err != nil {
		return nil, err
	}
	return &SparseIndexedSelectPlan{
		signature: Signature{domains: table.signature.Domains()}, functional: table.functional,
		column: column, value: value, resultSignature: resultSignature, resultFunctional: resultFunctional,
	}, nil
}

func (plan *SparseIndexedSelectPlan) validateTable(table *SparseTable) error {
	if plan == nil || table == nil || table.open() != nil || plan.functional != table.functional || !plan.signature.equal(table.signature) {
		return ErrTable
	}
	return nil
}
