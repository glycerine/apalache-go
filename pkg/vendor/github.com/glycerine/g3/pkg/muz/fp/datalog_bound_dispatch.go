package fp

// Source: dl_relation_manager.cpp mk_empty_relation first tries a favourite,
// then finite table conversion, then registered relation plugins. The bound
// plugin precedes interval in rel_context.cpp registration order.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) ensureBoundDatalog() (bool, error) {
	if err := e.cacheDatalogRelationFavourite(); err != nil {
		return false, err
	}
	if len(e.predicateOrder) == 0 {
		return false, nil
	}
	explicit := e.datalogRelationFavourite == "bound_relation"
	if !explicit && e.datalogRelationFavourite != "pentagon" && e.datalogRelationFavourite != "doc" {
		return false, nil
	}
	if !explicit {
		arithmetic, finite := false, false
		for _, d := range e.predicateOrder {
			if e.auxiliary[d] {
				continue
			}
			state := e.predicates[d]
			if len(state.sorts) > 0 && rel.CanHandleIntervalSorts(e.manager, state.sorts) {
				arithmetic = true
			} else {
				finite = true
			}
		}
		// Original finite predicates keep their table semantics. A synthetic
		// query predicate instead inherits its arithmetic producer's relation kind.
		if !arithmetic {
			return false, nil
		}
		if finite {
			return true, ErrBoundDatalogFragment
		}
	}
	for _, state := range e.predicates {
		if state.explicitFiniteRepresentation || state.external != nil {
			return true, ErrBoundDatalogFragment
		}
	}
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return true, errors.Join(err, ErrBoundDatalogFragment)
	}
	checkName, checkEnabled, err := e.datalogCheckRelationName()
	if err != nil || checkEnabled && checkName != "bound_relation" {
		return true, errors.Join(err, ErrBoundDatalogFragment)
	}
	var formulaCheck *smtcore.SMTSolverConfig
	if checkEnabled {
		formulaCheck = &smtcore.SMTSolverConfig{Params: e.params, Globals: e.globals}
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return true, errors.Join(err, ErrBoundDatalogFragment)
	}
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return true, err
		}
		magic, err := view.Bool("datalog.magic_sets_for_queries", false)
		err = errors.Join(err, view.Close())
		if err != nil || magic {
			return true, errors.Join(err, ErrBoundDatalogFragment)
		}
	}
	if e.boundDatalog != nil {
		e.boundDatalog.formulaCheck = formulaCheck
		return true, e.boundDatalog.syncPredicates(e.predicateOrder)
	}
	owner, selected, err := newBoundDatalogActivation(e.manager, e.set, e.predicateOrder)
	if err != nil {
		return true, err
	}
	if !selected {
		return true, ErrBoundDatalogFragment
	}
	owner.formulaCheck = formulaCheck
	if err = owner.verifySeedRelations(); err != nil {
		return true, errors.Join(err, owner.close())
	}
	e.boundDatalog = owner
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		if state.table != nil && len(state.sorts) == 0 {
			count, err := state.table.Len()
			if err != nil {
				return true, err
			}
			if count != 0 {
				if err = owner.addFact(d, nil); err != nil {
					return true, err
				}
			}
		}
	}
	return true, nil
}

func (e *PropositionalEngine) queryBoundDatalog(declarations []ast.ASTHandle) (lbool.LBool, error) {
	e.compiled = false
	e.datalogStats.Reset()
	enabled, threshold := true, uint32(11)
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return lbool.Undef, err
		}
		enabled, err = view.Bool("datalog.similarity_compressor", true)
		if err == nil && enabled {
			threshold, err = view.UInt("datalog.similarity_compressor_threshold", 11)
		}
		err = errors.Join(err, view.Close())
		if err != nil {
			return lbool.Undef, err
		}
	}
	if enabled {
		return e.boundDatalog.queryWithSimilarity(declarations, threshold)
	}
	return e.boundDatalog.query(declarations)
}
func (e *PropositionalEngine) closeBoundDatalog() error {
	if e.boundDatalog == nil {
		return nil
	}
	if err := e.boundDatalog.close(); err != nil {
		return err
	}
	e.boundDatalog = nil
	return nil
}
