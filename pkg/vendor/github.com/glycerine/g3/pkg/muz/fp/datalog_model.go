package fp

// The finite-table model is an existing Go extension. Source rel_context
// inherits the empty model from src/muz/base/dl_engine_base.h get_model.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// datalogModel materializes the least fixed-point relation tables as Boolean
// predicate interpretations. The omitted declaration is the synthetic Horn
// tactic query predicate filtered by the native model converter.
func (e *PropositionalEngine) datalogModel(omitted ast.ASTHandle) (result *model.Model, resultErr error) {
	if e.backend != "datalog" || !e.queried || e.predicates[omitted] == nil {
		return nil, ErrPropositionalEngine
	}
	// Product answers are formulas, not the eager finite tables that may
	// still contain pre-selection nullary seeds. Formula-backed product model
	// construction is a separate consumer from the finite table model below.
	if e.boundIntervalDatalog != nil || e.queryClone != nil && e.queryClone.boundIntervalDatalog != nil {
		return nil, ErrBoundIntervalDatalogFragment
	}
	attempt := &datalogModelAttempt{manager: e.manager}
	var err error
	attempt.seed, err = model.NewModel(e.manager)
	defer func() {
		if resultErr != nil {
			result = nil
			resultErr = attempt.fail(resultErr)
		}
	}()
	if err != nil {
		return nil, err
	}
	truth, err := e.manager.MkTrue()
	if err != nil {
		return nil, err
	}
	falsity, err := e.manager.MkFalse()
	if err != nil {
		return nil, err
	}
	for _, declaration := range e.predicateOrder {
		if declaration == omitted {
			continue
		}
		state := e.predicates[declaration]
		if state == nil || state.table == nil {
			return nil, ErrPropositionalEngine
		}
		facts, x := state.table.Facts()
		if x != nil {
			return nil, x
		}
		if len(state.sorts) == 0 {
			value := falsity
			if len(facts) != 0 {
				value = truth
			}
			if x = attempt.seed.Core().RegisterConstant(declaration, value); x != nil {
				return nil, x
			}
			continue
		}
		attempt.interpretation, x = model.NewFunctionInterpretation(e.manager, uint32(len(state.sorts)))
		if x != nil {
			return nil, x
		}
		if x = attempt.interpretation.SetElse(falsity); x != nil {
			return nil, x
		}
		for _, fact := range facts {
			values, y := e.relations.DecodeFact(state.sorts, fact)
			attempt.values = values
			if y != nil {
				return nil, y
			}
			handles := make([]ast.ASTHandle, len(values))
			for index := range values {
				handles[index], y = e.manager.RefGet(values[index])
				if y != nil {
					break
				}
			}
			if y == nil {
				y = attempt.interpretation.InsertNewEntry(handles, truth)
			}
			if y != nil {
				return nil, y
			}
			if y = attempt.clearValues(); y != nil {
				return nil, y
			}
		}
		if x = attempt.seed.Core().RegisterFunction(declaration, attempt.interpretation); x != nil {
			return nil, x
		}
		attempt.interpretation = nil
	}
	attempt.expanded, err = e.expandDatalogModel(attempt.seed)
	if err != nil {
		return nil, err
	}
	return attempt.expanded, nil
}
