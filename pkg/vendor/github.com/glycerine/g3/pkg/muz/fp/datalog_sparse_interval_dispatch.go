package fp

// Source: rel_context::set_predicate_representation creates a product kind
// from requested relation plugins; generated query heads inherit that kind.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) ensureSparseIntervalDatalog() (bool, error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	selected := false
	for _, state := range e.predicates {
		selected = selected || isSparseIntervalRepresentation(state.requestedRepresentations)
	}
	if !selected {
		return false, nil
	}
	originals := make([]sparseIntervalOriginal, 0, len(e.predicateOrder))
	declarations := make([]ast.ASTHandle, 0, len(e.predicateOrder))
	for _, declaration := range e.predicateOrder {
		state := e.predicates[declaration]
		if e.auxiliary[declaration] {
			continue
		}
		if state.external != nil {
			return true, ErrSparseIntervalDatalogFragment
		}
		class := sparseIntervalPredicateClass(0)
		switch {
		case isSparseIntervalRepresentation(state.requestedRepresentations):
			class = sparseIntervalMixedOriginal
		case len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "tr_sparse":
			class = sparseIntervalFiniteOriginal
		case len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "tr_hashtable":
			class = sparseIntervalHashOriginal
		case len(state.requestedRepresentations) == 1 && state.requestedRepresentations[0] == "interval_relation":
			class = sparseIntervalArithmeticOriginal
		case len(state.requestedRepresentations) == 0 && e.datalogRelationFavourite == "interval_relation" && rel.CanHandleIntervalSorts(e.manager, state.sorts):
			class = sparseIntervalArithmeticOriginal
		case len(state.requestedRepresentations) == 0 && state.table != nil:
			switch state.table.(type) {
			case *rel.SparseTable:
				class = sparseIntervalFiniteOriginal
			case *rel.HashTable:
				class = sparseIntervalHashOriginal
			default:
				return true, ErrSparseIntervalDatalogFragment
			}
		default:
			return true, ErrSparseIntervalDatalogFragment
		}
		originals = append(originals, sparseIntervalOriginal{declaration: declaration, class: class})
		declarations = append(declarations, declaration)
	}
	if len(originals) == 0 {
		return true, ErrSparseIntervalDatalogFragment
	}
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return true, errors.Join(err, ErrSparseIntervalDatalogFragment)
	}
	if _, enabled, err := e.datalogCheckRelationName(); err != nil || enabled {
		return true, errors.Join(err, ErrSparseIntervalDatalogFragment)
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return true, errors.Join(err, ErrSparseIntervalDatalogFragment)
	}
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return true, err
		}
		magic, err := view.Bool("datalog.magic_sets_for_queries", false)
		err = errors.Join(err, view.Close())
		if err != nil || magic {
			return true, errors.Join(err, ErrSparseIntervalDatalogFragment)
		}
	}
	if e.sparseIntervalDatalog == nil {
		owner, supported, err := newClassifiedSparseIntervalDatalogActivation(e.manager, e.set, e.guard, originals)
		if err != nil {
			return true, err
		}
		if !supported {
			return true, ErrSparseIntervalDatalogFragment
		}
		e.sparseIntervalDatalog = owner
	}
	if err := e.sparseIntervalDatalog.syncOriginals(originals); err != nil {
		return true, err
	}
	if err := e.migrateSparseIntervalFiniteSeeds(e.sparseIntervalDatalog, declarations); err != nil {
		return true, err
	}
	if e.intervalDatalog != nil {
		if err := e.migrateSparseIntervalArithmeticSeeds(e.sparseIntervalDatalog, e.intervalDatalog, declarations); err != nil {
			return true, err
		}
		if err := e.closeIntervalDatalog(); err != nil {
			return true, err
		}
	}
	// Original predicates were checked above; only the engine's generated
	// auxiliaries may use a projected pure-component or empty signature.
	return true, e.sparseIntervalDatalog.syncAuxiliaryPredicates(e.predicateOrder)
}

func (e *PropositionalEngine) querySparseIntervalDatalog(declarations []ast.ASTHandle) (lbool.LBool, error) {
	widen := false
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return lbool.Undef, err
		}
		widen, err = view.Bool("datalog.compile_with_widening", false)
		err = errors.Join(err, view.Close())
		if err != nil {
			return lbool.Undef, err
		}
	}
	e.compiled = false
	e.datalogStats.Reset()
	status, err := e.sparseIntervalDatalog.query(declarations, widen)
	if err == nil {
		err = e.publishSparseIntervalFiniteRelations(e)
	}
	return status, err
}

func (e *PropositionalEngine) closeSparseIntervalDatalog() error {
	if e.sparseIntervalDatalog == nil {
		return nil
	}
	if err := e.sparseIntervalDatalog.close(); err != nil {
		return err
	}
	e.sparseIntervalDatalog = nil
	return nil
}
