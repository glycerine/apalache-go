package interp

// Source checkpoint: src/interp/iz3checker.cpp and src/interp/iz3checker.h.

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func interpolationSupport(manager *Manager, treeMode bool, term ast.ASTHandle, result map[string]bool, memo map[ast.ASTHandle]bool) error {
	if memo[term] {
		return nil
	}
	memo[term] = true
	count, err := manager.NumArgs(term)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		child, e := manager.Arg(term, i)
		if e != nil {
			return e
		}
		if e = interpolationSupport(manager, treeMode, child, result, memo); e != nil {
			return e
		}
	}
	op, err := manager.Operator(term)
	if err != nil {
		return err
	}
	if op == OperatorUninterpreted && (count == 0 || !treeMode) {
		declaration, e := manager.Symbol(term)
		if e != nil {
			return e
		}
		name, e := manager.StringOfSymbol(declaration)
		if e != nil {
			return e
		}
		result[name] = true
	}
	return nil
}

func CheckInterpolants(manager *Manager, s solver.Solver, diagnostics io.Writer, constraints []ast.ASTHandle, parents []int, interpolants []ast.ASTHandle, theory []ast.ASTHandle) (bool, error) {
	if manager == nil || s == nil || diagnostics == nil {
		return false, ast.ErrASTHandle
	}
	if len(constraints) == 0 || len(interpolants)+1 < len(constraints) {
		return false, ast.ErrASTDomain
	}
	base, err := NewBase(manager, constraints, parents, theory)
	if err != nil {
		return false, err
	}
	defer base.Close()
	treeMode := len(parents) != 0
	children := make([][]int, len(constraints))
	for i := 0; i < len(constraints)-1; i++ {
		parent := i + 1
		if treeMode {
			parent = parents[i]
		}
		if parent < 0 || parent >= len(children) {
			return false, ast.ErrASTDomain
		}
		children[parent] = append(children[parent], i)
	}
	for i := range constraints {
		if err = s.Push(); err != nil {
			return false, err
		}
		for _, formula := range theory {
			if err = s.Assert(formula); err != nil {
				return false, err
			}
		}
		if err = s.Assert(constraints[i]); err != nil {
			return false, err
		}
		for _, child := range children[i] {
			if err = s.Assert(interpolants[child]); err != nil {
				return false, err
			}
		}
		if i != len(constraints)-1 {
			negated, e := manager.MkNot(interpolants[i])
			if e != nil {
				return false, e
			}
			if e = s.Assert(negated); e != nil {
				return false, e
			}
		}
		status, e := s.CheckSat(nil)
		if e != nil {
			return false, e
		}
		if status != lbool.False {
			if _, e = fmt.Fprintf(diagnostics, "interpolant %d is incorrect", i); e != nil {
				return false, e
			}
			if e = s.Pop(1); e != nil {
				return false, e
			}
			for _, formula := range theory {
				if e = s.Assert(formula); e != nil {
					return false, e
				}
			}
			for j, formula := range constraints {
				inside, e := base.scopes.InRange(j, DownwardScopeRange(i))
				if e != nil {
					return false, e
				}
				if inside {
					if e = s.Assert(formula); e != nil {
						return false, e
					}
				}
			}
			if i != len(constraints)-1 {
				negated, e := manager.MkNot(interpolants[i])
				if e != nil {
					return false, e
				}
				if e = s.Assert(negated); e != nil {
					return false, e
				}
			}
			status, e = s.CheckSat(nil)
			if e != nil {
				return false, e
			}
			if status != lbool.False {
				if _, e = fmt.Fprintf(diagnostics, "interpolant %d is not implied by its downeard closurn", i); e != nil {
					return false, e
				}
			}
			return false, nil
		}
		if err = s.Pop(1); err != nil {
			return false, err
		}
	}
	supports := make([]map[string]bool, len(constraints))
	for i, formula := range constraints {
		supports[i] = make(map[string]bool)
		if err = interpolationSupport(manager, treeMode, formula, supports[i], make(map[ast.ASTHandle]bool)); err != nil {
			return false, err
		}
	}
	for i := 0; i < len(constraints)-1; i++ {
		bSide := make([]bool, len(constraints))
		for j := len(constraints) - 1; j >= 0; j-- {
			bSide[j] = j != i
		}
		for j := len(constraints) - 1; j >= 0; j-- {
			if !bSide[j] {
				// Preserve the source's direct-child update through children[i].
				for _, child := range children[i] {
					bSide[child] = false
				}
			}
		}
		aSupport, bSupport := make(map[string]bool), make(map[string]bool)
		for j := len(constraints) - 1; j >= 0; j-- {
			target := aSupport
			if bSide[j] {
				target = bSupport
			}
			for name := range supports[j] {
				target[name] = true
			}
		}
		common := make(map[string]bool)
		for name := range aSupport {
			if bSupport[name] {
				common[name] = true
			}
		}
		theoryMemo := make(map[ast.ASTHandle]bool)
		for _, formula := range theory {
			if err = interpolationSupport(manager, treeMode, formula, common, theoryMemo); err != nil {
				return false, err
			}
		}
		interpolantSupport := make(map[string]bool)
		if err = interpolationSupport(manager, treeMode, interpolants[i], interpolantSupport, make(map[ast.ASTHandle]bool)); err != nil {
			return false, err
		}
		var bad []string
		for name := range interpolantSupport {
			if !common[name] {
				bad = append(bad, name)
			}
		}
		if len(bad) != 0 {
			sort.Strings(bad)
			if _, err = fmt.Fprintf(diagnostics, "bad symbols in interpolant %d:", i); err != nil {
				return false, err
			}
			for _, name := range bad {
				if _, err = fmt.Fprintf(diagnostics, "%s,", name); err != nil {
					return false, err
				}
			}
			return false, nil
		}
	}
	return true, nil
}

func CheckTreeInterpolants(manager *Manager, s solver.Solver, diagnostics io.Writer, constraints []ast.ASTHandle, tree ast.ASTHandle, interpolants []ast.ASTHandle) (bool, error) {
	base, err := NewBase(manager, nil, nil, nil)
	if err != nil {
		return false, err
	}
	converted, parents, theory, _, err := base.ToParentsRepresentation(constraints, tree, false)
	closeErr := base.Close()
	if err != nil || closeErr != nil {
		return false, errors.Join(err, closeErr)
	}
	return CheckInterpolants(manager, s, diagnostics, converted, parents, interpolants, theory)
}
