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
	if s == nil {
		return lbool.Undef, nil, ast.ErrASTHandle
	}
	manager, err := s.Manager()
	if err != nil {
		return lbool.Undef, nil, err
	}
	termRefs := make([]ast.ASTRef, 0, len(terms))
	for index, term := range terms {
		ref, err := manager.NewRef(term)
		if err != nil {
			for held := range termRefs {
				_ = manager.DelRef(&termRefs[held])
			}
			return lbool.Undef, nil, fmt.Errorf("implied equality term %d reference: %w", index, err)
		}
		termRefs = append(termRefs, ref)
	}
	defer func() {
		for index := range termRefs {
			result = errors.Join(result, manager.DelRef(&termRefs[index]))
		}
	}()
	classes := newSMTImpliedEqualityClasses(len(terms))
	bySort := make(map[ast.ASTHandle][]uint32)
	for index, term := range terms {
		sort, err := manager.GetSort(term)
		if err != nil {
			return lbool.Undef, nil, fmt.Errorf("implied equality term %d sort: %w", index, err)
		}
		bySort[sort] = append(bySort[sort], uint32(index))
	}
	if err := s.Push(); err != nil {
		return lbool.Undef, nil, fmt.Errorf("implied equality outer push: %w", err)
	}
	defer func() {
		result = errors.Join(result, s.Pop(1))
	}()
	status, result = s.CheckSat(nil)
	if result != nil || status != lbool.True {
		if result != nil {
			result = fmt.Errorf("implied equality initial check: %w", result)
		}
		return status, nil, result
	}
	for _, indices := range bySort {
		for position := 1; position < len(indices); position++ {
			current := indices[position]
			if classes.find(current) != current {
				continue
			}
			for priorPosition := 0; priorPosition < position; priorPosition++ {
				prior := indices[priorPosition]
				equality, err := manager.MkEq(terms[current], terms[prior])
				if err != nil {
					return lbool.Undef, nil, fmt.Errorf("implied equality pair %d,%d expression: %w", current, prior, err)
				}
				negated, err := manager.MkNot(equality)
				if err != nil {
					return lbool.Undef, nil, fmt.Errorf("implied equality pair negation: %w", err)
				}
				if err = s.Push(); err != nil {
					return lbool.Undef, nil, fmt.Errorf("implied equality pair push: %w", err)
				}
				if err = s.Assert(negated); err != nil {
					return lbool.Undef, nil, fmt.Errorf("implied equality pair assertion: %w", errors.Join(err, s.Pop(1)))
				}
				pairStatus, err := s.CheckSat(nil)
				popErr := s.Pop(1)
				if err != nil || popErr != nil {
					return lbool.Undef, nil, fmt.Errorf("implied equality pair check: %w", errors.Join(err, popErr))
				}
				if pairStatus == lbool.Undef {
					return lbool.Undef, nil, nil
				}
				if pairStatus == lbool.False {
					classes.merge(current, prior)
					break
				}
			}
		}
	}
	classIDs = make([]uint32, len(terms))
	for index := range classIDs {
		classIDs[index] = classes.find(uint32(index))
	}
	return lbool.True, classIDs, nil
}

// ImpliedEqualities applies the package-level exact fallback to this concrete
// SMT solver.
func (s *SMTSolver) ImpliedEqualities(terms []ast.ASTHandle) (lbool.LBool, []uint32, error) {
	return ImpliedEqualities(s, terms)
}
