package rel

func (r *CheckedRelation) FilterEqual(value Element, column int) error {
	if e := r.primary.FilterEqual(value, column); e != nil {
		return e
	}
	facts, e := r.checker.Facts()
	if e != nil {
		return e
	}
	r.checker.Reset()
	for _, f := range facts {
		if f[column] == value {
			r.checker.Add(f)
		}
	}
	return r.verify("filter equal")
}
func (r *CheckedRelation) FilterIdentical(columns []int) error {
	if e := r.primary.FilterIdentical(columns); e != nil {
		return e
	}
	facts, e := r.checker.Facts()
	if e != nil {
		return e
	}
	r.checker.Reset()
	for _, f := range facts {
		same := true
		for i := 1; i < len(columns); i++ {
			if f[columns[i]] != f[columns[0]] {
				same = false
				break
			}
		}
		if same {
			r.checker.Add(f)
		}
	}
	return r.verify("filter identical")
}
func (r *CheckedRelation) FilterNegation(negative *CheckedRelation, targetCols, negativeCols []int) error {
	if e := r.primary.FilterByNegation(negative.primary, targetCols, negativeCols); e != nil {
		return e
	}
	if e := r.checker.FilterNegation(negative.checker, targetCols, negativeCols); e != nil {
		return e
	}
	return r.verify("filter negation")
}
