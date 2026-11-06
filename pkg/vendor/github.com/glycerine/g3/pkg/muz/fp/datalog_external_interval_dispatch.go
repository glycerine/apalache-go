package fp

// Source: rel_context::set_predicate_representation and specialized query
// dispatch for an external_relation plus interval_relation product.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) hasExternalIntervalRepresentation() bool {
	for _, state := range e.predicates {
		if state != nil && isExternalIntervalRepresentation(state.requestedRepresentations) {
			return true
		}
	}
	return false
}

func (e *PropositionalEngine) externalIntervalPredicates() ([]externalIntervalDatalogPredicate, bool, error) {
	selected := e.hasExternalIntervalRepresentation()
	if !selected {
		return nil, false, nil
	}
	predicates := make([]externalIntervalDatalogPredicate, 0, len(e.predicateOrder))
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state == nil {
			return nil, true, ErrExternalIntervalDatalogFragment
		}
		if e.auxiliary[declaration] || !isExternalIntervalRepresentation(state.requestedRepresentations) {
			continue
		}
		if state.external != nil || state.explicitFiniteRepresentation || !rel.CanHandleIntervalSorts(e.manager, state.sorts) {
			return nil, true, ErrExternalIntervalDatalogFragment
		}
		if state.table != nil {
			count, err := state.table.Len()
			if err != nil {
				return nil, true, err
			}
			if count != 0 {
				return nil, true, ErrExternalIntervalDatalogFragment
			}
		}
		predicates = append(predicates, externalIntervalDatalogPredicate{
			declaration: declaration,
			sorts:       append([]ast.ASTHandle(nil), state.sorts...),
			kinds:       append([]string(nil), state.requestedRepresentations...),
		})
	}
	return predicates, true, nil
}

func (e *PropositionalEngine) ensureExternalIntervalDatalog() (_ bool, resultErr error) {
	predicates, selected, err := e.externalIntervalPredicates()
	if err != nil || !selected {
		if err != nil {
			return selected, fmt.Errorf("external interval predicate selection: %w", err)
		}
		return selected, nil
	}
	if e.external == nil || len(predicates) == 0 {
		return true, ErrExternalIntervalDatalogFragment
	}
	needsNormalization, err := externalIntervalNeedsJoinNormalization(e.set)
	if err != nil {
		return true, fmt.Errorf("external interval normalization check: %w", err)
	}
	if needsNormalization {
		if e.externalIntervalDatalog != nil {
			if err = e.externalIntervalDatalog.prepareExternalIntervalNormalization(); err != nil {
				return true, err
			}
			if err = e.externalIntervalDatalog.syncPredicates(predicates); err != nil {
				return true, err
			}
			return true, nil
		}
		owner, cause := newExternalIntervalDatalogActivation(e.manager, e.external, predicates)
		if cause != nil {
			return true, cause
		}
		e.externalIntervalDatalog = owner
		return true, nil
	}
	program, err := e.externalIntervalProgramPlan()
	if err != nil {
		return true, fmt.Errorf("external interval program plan: %w", err)
	}
	var identity *externalIntervalDatalogIdentityRule
	var ground *externalIntervalDatalogGroundRule
	var cartesian *externalIntervalDatalogCartesianRule
	if program == nil {
		identity, ground, cartesian, err = e.externalIntervalRulePlans()
		if err != nil {
			return true, err
		}
	}
	keepGround := false
	defer func() {
		if ground != nil && !keepGround {
			resultErr = errors.Join(resultErr, ground.close(e.manager))
		}
	}()
	if e.udocDatalog != nil || e.sparseIntervalDatalog != nil || e.intervalDatalog != nil || e.boundDatalog != nil || e.boundIntervalDatalog != nil {
		return true, fmt.Errorf("%w: another Datalog representation is active", ErrExternalIntervalDatalogFragment)
	}
	if enabled, cause := e.datalogExplanationsEnabled(); cause != nil || enabled {
		return true, errors.Join(cause, fmt.Errorf("%w: explanation mode is active", ErrExternalIntervalDatalogFragment))
	}
	if _, enabled, cause := e.datalogCheckRelationName(); cause != nil || enabled {
		return true, errors.Join(cause, fmt.Errorf("%w: relation checking is active", ErrExternalIntervalDatalogFragment))
	}
	if _, enabled, cause := e.datalogDefaultTableCheck(); cause != nil || enabled {
		return true, errors.Join(cause, fmt.Errorf("%w: default table checking is active", ErrExternalIntervalDatalogFragment))
	}
	if e.params != nil && e.globals != nil {
		view, cause := e.fixedpointParamView()
		if cause != nil {
			return true, cause
		}
		magic, cause := view.Bool("datalog.magic_sets_for_queries", false)
		cause = errors.Join(cause, view.Close())
		if cause != nil || magic {
			return true, errors.Join(cause, fmt.Errorf("%w: magic set transformation is active", ErrExternalIntervalDatalogFragment))
		}
	}
	if e.externalIntervalDatalog != nil {
		if err = e.externalIntervalDatalog.syncPredicates(predicates); err != nil {
			return true, err
		}
		if program != nil {
			err = e.externalIntervalDatalog.installExternalIntervalProgram(program)
		} else {
			err = e.externalIntervalDatalog.installExternalIntervalRulePlans(identity, ground, cartesian)
		}
		if err != nil {
			return true, err
		}
		keepGround = true
		return true, nil
	}
	owner, err := newExternalIntervalDatalogActivation(e.manager, e.external, predicates)
	if err != nil {
		return true, err
	}
	if program != nil {
		err = owner.installExternalIntervalProgram(program)
	} else {
		err = owner.installExternalIntervalRulePlans(identity, ground, cartesian)
	}
	if err != nil {
		return true, finishExternalIntervalDatalogConstruction(err, owner)
	}
	keepGround = true
	e.externalIntervalDatalog = owner
	return true, nil
}

func (e *PropositionalEngine) queryExternalIntervalDatalog(declarations []ast.ASTHandle) (lbool.LBool, error) {
	e.compiled = false
	e.datalogStats.Reset()
	needsNormalization, err := externalIntervalNeedsJoinNormalization(e.set)
	if err != nil {
		return lbool.Undef, err
	}
	if needsNormalization {
		if e.externalIntervalDatalog == nil {
			return lbool.Undef, ErrExternalIntervalDatalogFragment
		}
		scope, cause := prepareExternalIntervalDatalogNormalization(e.externalIntervalDatalog, e.set, declarations)
		if cause != nil {
			return lbool.Undef, cause
		}
		return scope.query(declarations)
	}
	if e.externalIntervalDatalog.program != nil {
		program := e.externalIntervalDatalog.program
		if len(program.strata) != 0 && e.params != nil && e.globals != nil {
			view, cause := e.fixedpointParamView()
			if cause != nil {
				return lbool.Undef, cause
			}
			program.widen, cause = view.Bool("datalog.compile_with_widening", false)
			if cause == nil {
				program.allOrNothing, cause = view.Bool("datalog.all_or_nothing_deltas", false)
			}
			if cause = errors.Join(cause, view.Close()); cause != nil {
				return lbool.Undef, cause
			}
		}
		err = e.externalIntervalDatalog.executeExternalIntervalProgram()
	} else {
		err = e.externalIntervalDatalog.executeExternalIntervalIdentity()
	}
	if err != nil {
		return lbool.Undef, err
	}
	return e.externalIntervalDatalog.query(declarations)
}

func (e *PropositionalEngine) closeExternalIntervalDatalog() error {
	if e.externalIntervalDatalog == nil {
		return nil
	}
	if err := e.externalIntervalDatalog.close(); err != nil {
		return err
	}
	e.externalIntervalDatalog = nil
	return nil
}
