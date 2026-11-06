package rel

func NewSparseNegatedJoinPlan(target, first, second *SparseTable, targetColumns, sourceColumns, firstJoinColumns, secondJoinColumns []int) (*SparseNegatedJoinPlan, error) {
	if target == nil || first == nil || second == nil || first == second {
		return nil, ErrTable
	}
	if err := target.open(); err != nil {
		return nil, err
	}
	if err := first.open(); err != nil {
		return nil, err
	}
	if err := second.open(); err != nil {
		return nil, err
	}
	if len(targetColumns) != len(sourceColumns) || len(firstJoinColumns) != len(secondJoinColumns) {
		return nil, ErrTable
	}
	for i, targetColumn := range targetColumns {
		if targetColumn < 0 || targetColumn >= target.signature.Len() || sourceColumns[i] < 0 || sourceColumns[i] >= first.signature.Len()+second.signature.Len() {
			return nil, ErrTable
		}
		var domain uint64
		if sourceColumns[i] < first.signature.Len() {
			domain = first.signature.domains[sourceColumns[i]]
		} else {
			domain = second.signature.domains[sourceColumns[i]-first.signature.Len()]
		}
		if target.signature.domains[targetColumn] != domain {
			return nil, ErrTable
		}
	}
	for i, firstColumn := range firstJoinColumns {
		secondColumn := secondJoinColumns[i]
		if firstColumn < 0 || firstColumn >= first.signature.Len() || secondColumn < 0 || secondColumn >= second.signature.Len() || first.signature.domains[firstColumn] != second.signature.domains[secondColumn] {
			return nil, ErrTable
		}
	}
	return &SparseNegatedJoinPlan{
		targetSignature: Signature{domains: target.signature.Domains()}, firstSignature: Signature{domains: first.signature.Domains()}, secondSignature: Signature{domains: second.signature.Domains()},
		targetFunctional: target.functional, firstFunctional: first.functional, secondFunctional: second.functional,
		targetColumns: append([]int(nil), targetColumns...), sourceColumns: append([]int(nil), sourceColumns...),
		firstJoinColumns: append([]int(nil), firstJoinColumns...), secondJoinColumns: append([]int(nil), secondJoinColumns...),
	}, nil
}

func (plan *SparseNegatedJoinPlan) validateTables(target, first, second *SparseTable) error {
	if plan == nil || target == nil || first == nil || second == nil || first == second {
		return ErrTable
	}
	if target.open() != nil || first.open() != nil || second.open() != nil {
		return ErrTable
	}
	if !plan.targetSignature.equal(target.signature) || !plan.firstSignature.equal(first.signature) || !plan.secondSignature.equal(second.signature) || plan.targetFunctional != target.functional || plan.firstFunctional != first.functional || plan.secondFunctional != second.functional {
		return ErrTable
	}
	return nil
}
