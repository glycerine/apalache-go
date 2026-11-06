package qe

// Source: src/qe/nlqsat.cpp:650-713.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/nlsat"
	nlsattactic "github.com/glycerine/g3/pkg/nlsat/tactic"
	"github.com/glycerine/g3/pkg/sat"
)

func newNLQSearchFromCompiler(solver *nlsat.Solver, compiler *nlsattactic.GoalCompiler, blocks [][]ast.ASTRef, isTrue ast.ASTHandle) (*nlqSearch, error) {
	return newNLQSearchFromCompilerMode(solver, compiler, blocks, isTrue, qsatSAT)
}

func newNLQSearchFromCompilerMode(solver *nlsat.Solver, compiler *nlsattactic.GoalCompiler, blocks [][]ast.ASTRef, isTrue ast.ASTHandle, mode qsatMode) (*nlqSearch, error) {
	if solver == nil || compiler == nil {
		return nil, arena.ErrConfig
	}
	truth, err := compiler.Literal(isTrue)
	if err != nil {
		return nil, err
	}
	boundReal := make([][]uint32, len(blocks))
	boundBoolean := make([][]sat.BoolVar, len(blocks))
	realLevels := make([]MaxLevel, 0)
	manager, err := compiler.AtomMapping().Manager()
	if err != nil {
		return nil, err
	}
	for level, block := range blocks {
		value := EmptyMaxLevel()
		if level&1 == 0 {
			value.Existential = uint32(level)
		} else {
			value.Universal = uint32(level)
		}
		for _, ref := range block {
			expression, cause := manager.RefGet(ref)
			if cause != nil {
				return nil, cause
			}
			boolean, cause := compiler.AtomMapping().ToVar(expression)
			if cause != nil {
				return nil, cause
			}
			if boolean != ^uint32(0) {
				boundBoolean[level] = append(boundBoolean[level], sat.BoolVar(boolean))
				continue
			}
			real, cause := compiler.TermMapping().ToVar(expression)
			if cause != nil {
				return nil, cause
			}
			if real == ^uint32(0) {
				continue
			}
			for uint64(len(realLevels)) <= uint64(real) {
				realLevels = append(realLevels, EmptyMaxLevel())
			}
			realLevels[real] = value
			boundReal[level] = append(boundReal[level], real)
		}
	}
	frontend := make(map[sat.BoolVar]bool)
	entries, err := compiler.AtomMapping().Entries()
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		frontend[sat.BoolVar(entry.Variable)] = true
	}
	var converter *nlsattactic.GoalConverter
	if mode == qsatQE {
		converter, err = nlsattactic.NewGoalConverter(manager, solver, compiler.AtomMapping(), compiler.TermMapping())
		if err != nil {
			return nil, err
		}
	}
	search, err := newNLQSearchMode(solver, truth, realLevels, boundReal, boundBoolean, frontend, mode, manager, converter)
	if err != nil {
		return nil, err
	}
	var literals []sat.Literal
	if err = solver.ArithmeticLiterals(&literals); err != nil {
		return nil, errors.Join(err, search.close())
	}
	for _, literal := range literals {
		if _, err = search.state.getLevel(literal); err != nil {
			return nil, errors.Join(err, search.close())
		}
	}
	return search, nil
}
