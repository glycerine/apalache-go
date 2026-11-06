package smtcore

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *smtImpliedEqualityEngine) implied(first, second uint32) (bool, error) {
	equality, err := e.manager.MkEq(e.terms[first], e.terms[second])
	if err != nil {
		return false, fmt.Errorf("implied equality pair %d,%d expression: %w", first, second, err)
	}
	negated, err := e.manager.MkNot(equality)
	if err != nil {
		return false, fmt.Errorf("implied equality pair negation: %w", err)
	}
	if err = e.solver.Push(); err != nil {
		return false, fmt.Errorf("implied equality pair push: %w", err)
	}
	if err = e.solver.Assert(negated); err != nil {
		return false, fmt.Errorf("implied equality pair assertion: %w", errors.Join(err, e.solver.Pop(1)))
	}
	status, err := e.solver.CheckSat(nil)
	popErr := e.solver.Pop(1)
	if err != nil || popErr != nil {
		return false, fmt.Errorf("implied equality pair check: %w", errors.Join(err, popErr))
	}
	e.stats.PairChecks++
	// Native treats UNKNOWN as an unrelated pair, not as a failure of the
	// complete query.
	return status == lbool.False, nil
}
