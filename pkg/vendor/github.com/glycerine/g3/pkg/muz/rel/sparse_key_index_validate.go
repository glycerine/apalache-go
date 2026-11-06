package rel

func NewSparseKeyLookupPlan(table *SparseTable, columns []int) (*SparseKeyLookupPlan, error) {
	if table == nil || table.open() != nil {
		return nil, ErrTable
	}
	firstFunctional := table.signature.Len() - table.functional
	for _, column := range columns {
		if column < 0 || column >= firstFunctional {
			return nil, ErrTable
		}
	}
	return &SparseKeyLookupPlan{
		signature: Signature{domains: table.signature.Domains()}, functional: table.functional,
		columns: append([]int(nil), columns...),
	}, nil
}

func (plan *SparseKeyLookupPlan) isFullSignature() bool {
	firstFunctional := plan.signature.Len() - plan.functional
	if firstFunctional == 0 || len(plan.columns) != firstFunctional {
		return false
	}
	seen := make([]bool, firstFunctional)
	for _, column := range plan.columns {
		if seen[column] {
			return false
		}
		seen[column] = true
	}
	return true
}

func (plan *SparseKeyLookupPlan) validateTable(table *SparseTable) error {
	if plan == nil || table == nil || table.open() != nil || plan.functional != table.functional || !plan.signature.equal(table.signature) {
		return ErrTable
	}
	return nil
}

func (plan *SparseKeyLookupPlan) validateKey(key Fact) error {
	if plan == nil || len(key) != len(plan.columns) {
		return ErrTable
	}
	return nil
}
