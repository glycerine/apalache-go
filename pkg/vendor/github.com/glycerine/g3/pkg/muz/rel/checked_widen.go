package rel

// Source: src/muz/rel/check_relation.cpp check_relation_plugin::mk_widen_fn
// reuses union_fn; finite table widening is the same exact set union.

// WidenFrom applies the checked formula union verifier used by the source
// widening factory, including its delta bounds.
func (r *FormulaCheckedTable) WidenFrom(source, delta *FormulaCheckedTable) error {
	return r.UnionFrom(source, delta)
}

// WidenFrom applies exact finite-table widening through both paired backends.
func (t *CheckTable) WidenFrom(source, delta *CheckTable) error {
	return t.UnionFrom(source, delta)
}
