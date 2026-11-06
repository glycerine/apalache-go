package rel

import "errors"

func (t *FormulaCheckedFinitePair) Join(other *FormulaCheckedFinitePair, left, right []int) (*FormulaCheckedFinitePair, error) {
	if err := t.verify("join left"); err != nil {
		return nil, err
	}
	if err := other.verify("join right"); err != nil {
		return nil, err
	}
	pair, err := t.pair.Join(other.pair, left, right)
	if err != nil {
		return nil, err
	}
	formula, err := t.formula.Join(other.formula, left, right)
	if err != nil {
		return nil, errors.Join(err, pair.Close())
	}
	return adoptFormulaCheckedFinitePair(pair, formula)
}

func (t *FormulaCheckedFinitePair) JoinProject(other *FormulaCheckedFinitePair, left, right, removed []int) (*FormulaCheckedFinitePair, error) {
	if err := t.verify("join project left"); err != nil {
		return nil, err
	}
	if err := other.verify("join project right"); err != nil {
		return nil, err
	}
	pair, err := t.pair.JoinProject(other.pair, left, right, removed)
	if err != nil {
		return nil, err
	}
	formula, err := t.formula.JoinProject(other.formula, left, right, removed)
	if err != nil {
		return nil, errors.Join(err, pair.Close())
	}
	return adoptFormulaCheckedFinitePair(pair, formula)
}

func (t *FormulaCheckedFinitePair) Project(removed []int) (*FormulaCheckedFinitePair, error) {
	if err := t.verify("project input"); err != nil {
		return nil, err
	}
	pair, err := t.pair.Project(removed)
	if err != nil {
		return nil, err
	}
	formula, err := t.formula.Project(removed)
	if err != nil {
		return nil, errors.Join(err, pair.Close())
	}
	return adoptFormulaCheckedFinitePair(pair, formula)
}

func (t *FormulaCheckedFinitePair) RenameCycle(cycle []int) (*FormulaCheckedFinitePair, error) {
	if err := t.verify("rename input"); err != nil {
		return nil, err
	}
	pair, err := t.pair.RenameCycle(cycle)
	if err != nil {
		return nil, err
	}
	formula, err := t.formula.RenameCycle(cycle)
	if err != nil {
		return nil, errors.Join(err, pair.Close())
	}
	return adoptFormulaCheckedFinitePair(pair, formula)
}

func (t *FormulaCheckedFinitePair) SelectEqualAndProject(value Element, column int) (*FormulaCheckedFinitePair, error) {
	if err := t.verify("select project input"); err != nil {
		return nil, err
	}
	pair, err := t.pair.SelectEqualAndProject(value, column)
	if err != nil {
		return nil, err
	}
	clone, err := t.formula.Clone()
	if err != nil {
		return nil, errors.Join(err, pair.Close())
	}
	cleanup := &FormulaCheckedFinitePair{pair: pair, formula: clone}
	if err = formulaCheckedFilterEqual(clone, value, column); err != nil {
		return nil, finishFormulaCheckedFinitePairConstruction(err, cleanup)
	}
	formula, err := clone.Project([]int{column})
	if err != nil {
		return nil, finishFormulaCheckedFinitePairConstruction(err, cleanup)
	}
	result, err := adoptFormulaCheckedFinitePair(pair, formula)
	if err != nil {
		cleanup.pair = nil
		cleanup.formula = nil
		cleanup.retired = append(cleanup.retired, clone)
		return nil, finishFormulaCheckedFinitePairConstruction(err, cleanup)
	}
	// Keep the filtered intermediate as a retryable cleanup owner. Closing it
	// eagerly could refuse after the projected result is already constructed.
	result.retired = append(result.retired, clone)
	return result, nil
}

func (t *FormulaCheckedFinitePair) UnionFrom(source, delta *FormulaCheckedFinitePair) error {
	if err := t.verify("union target"); err != nil {
		return err
	}
	if err := source.verify("union source"); err != nil {
		return err
	}
	var pairDelta *FiniteCheckedPair
	var formulaDelta *FormulaCheckedTable
	if delta != nil {
		if err := delta.verify("union delta"); err != nil {
			return err
		}
		pairDelta, formulaDelta = delta.pair, delta.formula
	}
	if err := t.pair.UnionFrom(source.pair, pairDelta); err != nil {
		return err
	}
	if err := t.formula.UnionFrom(source.formula, formulaDelta); err != nil {
		return err
	}
	if err := t.verify("union"); err != nil {
		return err
	}
	if delta != nil {
		return delta.verify("union delta")
	}
	return nil
}

func (t *FormulaCheckedFinitePair) FilterByNegation(negative *FormulaCheckedFinitePair, columns, negativeColumns []int) error {
	if err := t.verify("negation target"); err != nil {
		return err
	}
	if err := negative.verify("negation source"); err != nil {
		return err
	}
	if err := t.pair.FilterByNegation(negative.pair, columns, negativeColumns); err != nil {
		return err
	}
	if err := t.formula.FilterByNegation(negative.formula, columns, negativeColumns); err != nil {
		return err
	}
	return t.verify("negation")
}
