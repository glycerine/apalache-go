package smtcore

// Source: src/smt/smt_implied_equalities.h and the exact basic fallback in
// src/smt/smt_implied_equalities.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtImpliedEqualityClasses struct{ parent []uint32 }

func newSMTImpliedEqualityClasses(size int) smtImpliedEqualityClasses {
	parent := make([]uint32, size)
	for index := range parent {
		parent[index] = uint32(index)
	}
	return smtImpliedEqualityClasses{parent: parent}
}

func (classes *smtImpliedEqualityClasses) find(value uint32) uint32 {
	root := value
	for classes.parent[root] != root {
		root = classes.parent[root]
	}
	for classes.parent[value] != value {
		next := classes.parent[value]
		classes.parent[value] = root
		value = next
	}
	return root
}

func (classes *smtImpliedEqualityClasses) merge(first, second uint32) {
	first, second = classes.find(first), classes.find(second)
	if first == second {
		return
	}
	if first > second {
		first, second = second, first
	}
	classes.parent[second] = first
}

// ImpliedEqualities returns a class identifier for each input term. Equal
// identifiers mean the current solver assertions imply equality. Terms of
// different sorts always remain in separate classes. The solver assertion
// stack is restored before return.
func ImpliedEqualities(s solver.Solver, terms []ast.ASTHandle) (status lbool.LBool, classIDs []uint32, result error) {
	status, classIDs, _, result = ImpliedEqualitiesWithStats(s, terms)
	return status, classIDs, result
}

func ImpliedEqualitiesWithStats(s solver.Solver, terms []ast.ASTHandle) (status lbool.LBool, classIDs []uint32, stats SMTImpliedEqualityStats, result error) {
	if s == nil {
		return lbool.Undef, nil, stats, ast.ErrASTHandle
	}
	manager, err := s.Manager()
	if err != nil {
		return lbool.Undef, nil, stats, err
	}
	termRefs := make([]ast.ASTRef, 0, len(terms))
	for index, term := range terms {
		ref, err := manager.NewRef(term)
		if err != nil {
			for held := range termRefs {
				_ = manager.DelRef(&termRefs[held])
			}
			return lbool.Undef, nil, stats, fmt.Errorf("implied equality term %d reference: %w", index, err)
		}
		termRefs = append(termRefs, ref)
	}
	defer func() {
		for index := range termRefs {
			result = errors.Join(result, manager.DelRef(&termRefs[index]))
		}
	}()
	engine := newSMTImpliedEqualityEngine(s, manager, terms)
	if err := engine.partition(); err != nil {
		return lbool.Undef, nil, stats, err
	}
	if err := s.Push(); err != nil {
		return lbool.Undef, nil, stats, fmt.Errorf("implied equality outer push: %w", err)
	}
	defer func() {
		result = errors.Join(result, s.Pop(1))
	}()
	status, result = s.CheckSat(nil)
	if result != nil || status != lbool.True {
		if result != nil {
			result = fmt.Errorf("implied equality initial check: %w", result)
		}
		return status, nil, stats, result
	}
	if err := engine.run(); err != nil {
		return lbool.Undef, nil, engine.stats, err
	}
	return lbool.True, engine.results(), engine.stats, nil
}

// ImpliedEqualities applies the package-level exact fallback to this concrete
// SMT solver.
func (s *SMTSolver) ImpliedEqualities(terms []ast.ASTHandle) (lbool.LBool, []uint32, error) {
	status, classes, _, err := s.ImpliedEqualitiesWithStats(terms)
	return status, classes, err
}

func (s *SMTSolver) ImpliedEqualitiesWithStats(terms []ast.ASTHandle) (status lbool.LBool, classes []uint32, stats SMTImpliedEqualityStats, err error) {
	previous := s.produceModels
	if err := s.SetProduceModels(true); err != nil {
		return lbool.Undef, nil, stats, err
	}
	status, classes, stats, err = ImpliedEqualitiesWithStats(s, terms)
	return status, classes, stats, errors.Join(err, s.SetProduceModels(previous))
}
