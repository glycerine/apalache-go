package rel

// Source: dl_finite_product_relation.cpp to_table_relation projects the final
// functional relation-index column through relation_manager::mk_project_fn;
// dl_check_table.cpp project_fn applies that projection independently to the
// selected primary and checker tables.

import "errors"

// ProjectFinite removes the one functional explanation-index suffix and
// returns an independently owned checked finite table over the original
// predicate columns. The source index remains unchanged.
func (t *CheckedExplanationIndex) ProjectFinite() (_ *FiniteCheckedPair, resultErr error) {
	if err := t.verify("project input"); err != nil {
		return nil, err
	}
	signature, err := t.primary.Signature()
	if err != nil || t.functional != 1 || signature.Len() < t.functional {
		return nil, errors.Join(err, ErrCheckTable)
	}
	removed := []int{signature.Len() - 1}
	primary, err := finiteTableProject(t.primary, removed)
	if err != nil {
		return nil, err
	}
	checker, err := finiteTableProject(t.checker, removed)
	if err != nil {
		return nil, errors.Join(err, primary.Close())
	}
	return adoptFiniteCheckedPair(primary, checker)
}
