package rel

import "errors"

// Complement is the concrete nested check_relation::complement path used by
// mk_full. The current finite checked backend supports the source-reached
// nullary and unary complements; wider signatures remain explicit errors in
// FiniteCheckedPair.Complement.
func (t *FormulaCheckedFinitePair) Complement() (*FormulaCheckedFinitePair, error) {
	if err := t.verify("complement input"); err != nil {
		return nil, err
	}
	pair, err := t.pair.Complement()
	if err != nil {
		return nil, err
	}
	formula, err := t.formula.Complement()
	if err != nil {
		return nil, errors.Join(err, pair.Close())
	}
	return adoptFormulaCheckedFinitePair(pair, formula)
}

// FullLike mirrors check_relation_plugin::mk_full by complementing a fresh
// empty nested checker of the same concrete kinds.
func (t *FormulaCheckedFinitePair) FullLike() (*FormulaCheckedFinitePair, error) {
	empty, err := t.EmptyLike()
	if err != nil {
		return nil, err
	}
	full, fullErr := empty.Complement()
	closeErr := empty.Close()
	if fullErr != nil || closeErr != nil {
		if full != nil {
			closeErr = errors.Join(closeErr, full.Close())
		}
		return nil, errors.Join(fullErr, closeErr)
	}
	return full, nil
}
