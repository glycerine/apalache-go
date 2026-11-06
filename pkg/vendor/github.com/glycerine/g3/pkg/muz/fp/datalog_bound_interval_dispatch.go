package fp

// Source: rel_context::set_predicate_representation, add_fact and query;
// product_relation preserves the explicitly requested component order.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func isBoundIntervalRepresentation(kinds []string) bool {
	return len(kinds) == 2 && (kinds[0] == "bound_relation" && kinds[1] == "interval_relation" || kinds[0] == "interval_relation" && kinds[1] == "bound_relation")
}

func (e *PropositionalEngine) selectBoundIntervalRepresentation(declaration ast.ASTHandle, kinds []string) error {
	state := e.predicates[declaration]
	if state == nil || state.external != nil {
		return ErrBoundIntervalDatalogFragment
	}
	if selected, err := rel.CanSelectIntervalBoundProduct(e.manager, state.sorts, kinds); err != nil || !selected {
		return errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	// Source set_predicate_kind requires a predicate without an existing
	// relation. Coarse seeds from another arithmetic plugin cannot reconstruct
	// the missing component of a newly selected product.
	if e.externalIntervalDatalog != nil || e.udocDatalog != nil || e.sparseIntervalDatalog != nil || e.intervalDatalog != nil || e.boundDatalog != nil {
		return ErrBoundIntervalDatalogFragment
	}
	state.requestedRepresentations = append([]string(nil), kinds...)
	state.explicitFiniteRepresentation = false
	e.revision++
	return nil
}

func (e *PropositionalEngine) ensureBoundIntervalDatalog() (bool, error) {
	selected := false
	for _, state := range e.predicates {
		selected = selected || isBoundIntervalRepresentation(state.requestedRepresentations)
	}
	if !selected {
		return false, nil
	}
	if e.externalIntervalDatalog != nil || e.udocDatalog != nil || e.sparseIntervalDatalog != nil || e.intervalDatalog != nil || e.boundDatalog != nil {
		return true, ErrBoundIntervalDatalogFragment
	}
	predicates := make([]boundIntervalDatalogPredicate, 0, len(e.predicateOrder))
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if state == nil || state.external != nil || state.explicitFiniteRepresentation || !isBoundIntervalRepresentation(state.requestedRepresentations) || !rel.CanHandleIntervalSorts(e.manager, state.sorts) {
			return true, ErrBoundIntervalDatalogFragment
		}
		predicates = append(predicates, boundIntervalDatalogPredicate{declaration: declaration, kinds: state.requestedRepresentations})
	}
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return true, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	if _, enabled, err := e.datalogCheckRelationName(); err != nil || enabled {
		return true, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return true, errors.Join(err, ErrBoundIntervalDatalogFragment)
	}
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return true, err
		}
		magic, err := view.Bool("datalog.magic_sets_for_queries", false)
		err = errors.Join(err, view.Close())
		if err != nil || magic {
			return true, errors.Join(err, ErrBoundIntervalDatalogFragment)
		}
	}
	if e.boundIntervalDatalog == nil {
		owner, supported, err := newBoundIntervalDatalogActivation(e.manager, e.set, predicates)
		if err != nil || !supported {
			return true, errors.Join(err, ErrBoundIntervalDatalogFragment)
		}
		e.boundIntervalDatalog = owner
	} else if err := e.boundIntervalDatalog.syncPredicates(predicates); err != nil {
		return true, err
	}
	// Nullary finite facts can precede explicit selection. Their unique tuple
	// transfers exactly to both components. The relation manager keeps its
	// original table owner, but public state thereafter uses the product seed.
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if len(state.sorts) == 0 && state.table != nil {
			count, err := state.table.Len()
			if err != nil {
				return true, err
			}
			if count != 0 {
				if err = e.boundIntervalDatalog.addFact(declaration, nil); err != nil {
					return true, err
				}
			}
			state.table = nil
		}
	}
	return true, nil
}

func (e *PropositionalEngine) queryBoundIntervalDatalog(declarations []ast.ASTHandle) (lbool.LBool, error) {
	widen, allOrNothing := false, false
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return lbool.Undef, err
		}
		widen, err = view.Bool("datalog.compile_with_widening", false)
		if err == nil && widen {
			allOrNothing, err = view.Bool("datalog.all_or_nothing_deltas", false)
		}
		err = errors.Join(err, view.Close())
		if err != nil {
			return lbool.Undef, err
		}
	}
	e.compiled = false
	e.datalogStats.Reset()
	normalize, err := e.boundIntervalDatalog.needsJoinNormalization()
	if err != nil {
		e.boundIntervalDatalog.evaluated = nil
		return lbool.Undef, err
	}
	if normalize {
		return e.boundIntervalDatalog.queryNormalized(declarations, widen, allOrNothing)
	}
	return e.boundIntervalDatalog.queryWithWidening(declarations, widen, allOrNothing)
}

func (e *PropositionalEngine) closeBoundIntervalDatalog() error {
	if e.boundIntervalDatalog == nil {
		return nil
	}
	if err := e.boundIntervalDatalog.close(); err != nil {
		return err
	}
	e.boundIntervalDatalog = nil
	return nil
}
