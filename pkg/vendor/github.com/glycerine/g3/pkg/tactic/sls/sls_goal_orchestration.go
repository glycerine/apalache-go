package sls

// Source: src/tactic/sls/sls_engine.cpp:538-583 and
// src/tactic/sls/sls_tactic.cpp:45-64.

import (
	"errors"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// ApplyGoal is the retained sls_tactic core after the QF_BV preamble. It is
// intentionally one-shot, matching tactic cleanup's replacement of the engine.
func ApplyGoal(engine *Engine, goal *tactic.Goal, out *tactic.TacticResult) (status lbool.LBool, result error) {
	if engine == nil || engine.open() != nil || goal == nil || goal.Manager() != engine.manager || out == nil {
		return lbool.Undef, ErrSLSSearch
	}
	if len(engine.assertions) != 0 {
		return lbool.Undef, ErrSLSSearch
	}
	if err := out.Prepare(engine.manager); err != nil {
		return lbool.Undef, err
	}
	wellSorted, err := goal.IsWellSorted()
	if err != nil || !wellSorted {
		if err != nil {
			return lbool.Undef, err
		}
		return lbool.Undef, ErrSLSSearch
	}
	inconsistent, err := goal.Inconsistent()
	if err != nil {
		return lbool.Undef, err
	}
	if inconsistent {
		if err = goal.IncDepth(); err != nil {
			return lbool.Undef, err
		}
		return lbool.Undef, out.AddGoal(goal)
	}
	size, err := goal.Size()
	if err != nil {
		return lbool.Undef, err
	}
	for i := uint32(0); i < size; i++ {
		formula, x := goal.Form(i)
		if x != nil {
			return lbool.Undef, x
		}
		if x = engine.AssertExpression(formula); x != nil {
			return lbool.Undef, x
		}
	}
	status, err = engine.Solve()
	if err != nil {
		return status, err
	}
	if status == lbool.True {
		if err = tactic.ReportTacticProgress("Number of flips:", engine.stats.Moves); err != nil {
			return status, err
		}
		for _, formula := range engine.assertions {
			value, x := engine.tracker.GetValue(formula)
			if x != nil {
				return status, x
			}
			one, x := engine.numbers.IsOne(value)
			if x != nil {
				return status, x
			}
			if !one {
				return status, ErrSLSSearch
			}
		}
		models, x := goal.ModelsEnabled()
		if x != nil {
			return status, x
		}
		if models {
			current, x := engine.tracker.GetModel()
			if x != nil {
				return status, x
			}
			converter, x := tactic.NewStoredModel(current.Core())
			if x != nil {
				return status, errors.Join(x, current.Close())
			}
			if x = out.SetModelConverter(converter); x != nil {
				// SetModelConverter retains before publication. A failed retain leaves
				// this zero-reference converter directly owned here.
				return status, errors.Join(x, converter.Close(), current.Close())
			}
			// converter's model reference is now the sole owner. Its final DEC
			// closes current; do not directly close the transferred model.
		}
		if err = goal.Reset(); err != nil {
			return status, err
		}
	}
	if err = goal.IncDepth(); err != nil {
		return status, err
	}
	if err = out.AddGoal(goal); err != nil {
		return status, err
	}
	return status, nil
}
