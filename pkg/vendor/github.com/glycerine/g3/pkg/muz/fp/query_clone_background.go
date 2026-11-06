package fp

// Source: spacer_dl_interface.cpp snapshots get_background_assertion before
// installing the query.

import "errors"

func (e *PropositionalEngine) copyQueryBackground(clone *PropositionalEngine) (resultErr error) {
	background, err := e.manager.RefGet(e.background)
	if err != nil {
		return err
	}
	if err = clone.SetBackground(background); err != nil {
		return err
	}
	assertions, err := e.BackgroundAssertions()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(e.manager, assertions)) }()
	for i := range assertions {
		formula, x := e.manager.RefGet(assertions[i])
		if x != nil {
			return x
		}
		if x = clone.AssertBackground(formula); x != nil {
			return x
		}
	}
	return nil
}
