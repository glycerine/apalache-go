package base

// StructurallyEqual implements rule_eq_proc: AST identity, tail order and sign.
func (r *Rule) StructurallyEqual(other *Rule) (bool, error) {
	left, err := r.open()
	if err != nil {
		return false, err
	}
	right, err := other.open()
	if err != nil {
		return false, err
	}
	if left.manager != right.manager || left.head != right.head || len(left.tail) != len(right.tail) {
		return false, nil
	}
	for i := range left.tail {
		if left.tail[i].handle != right.tail[i].handle || left.tail[i].negated != right.tail[i].negated {
			return false, nil
		}
	}
	return true, nil
}
