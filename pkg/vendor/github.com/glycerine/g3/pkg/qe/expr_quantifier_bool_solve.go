package qe

// Source: src/qe/qe_bool_plugin.cpp, solve_units/solve_polarized.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// chooseExprQEBoolElimination borrows the node and returns an owned result.
// Unit conjuncts take priority over variable order, as in the source solver.
func chooseExprQEBoolElimination(node *exprQETree, variables *exprQEBranchVariables, plugin *BoolPlugin, conjs *ConjunctionEnum) (index uint32, result *BoolElimination, found bool, err error) {
	if node == nil || node.manager == nil || node.atoms == nil || variables == nil || variables.manager != node.manager || plugin == nil || conjs == nil {
		return 0, nil, false, ast.ErrASTHandle
	}
	m := node.manager
	formula, err := m.RefGet(node.formula)
	if err != nil {
		return 0, nil, false, err
	}
	count, err := conjs.Len()
	if err != nil {
		return 0, nil, false, err
	}
	for i := uint32(0); i < count; i++ {
		expr, e := conjs.At(i)
		if e != nil {
			return 0, nil, false, e
		}
		kind, e := m.GetKind(expr)
		if e != nil {
			return 0, nil, false, e
		}
		if kind != ast.ASTApp {
			continue
		}
		target := expr
		truth := true
		position, e := node.variableIndex(target)
		if e != nil {
			return 0, nil, false, e
		}
		if position < 0 {
			not, arg, e := m.IsNot(expr)
			if e != nil {
				return 0, nil, false, e
			}
			if !not {
				continue
			}
			target = arg
			truth = false
			position, e = node.variableIndex(target)
			if e != nil {
				return 0, nil, false, e
			}
			if position < 0 {
				continue
			}
		}
		value, e := m.MkBoolVal(truth)
		if e != nil {
			return 0, nil, false, e
		}
		result, e = plugin.replaceAndOwn(target, value, formula)
		if e != nil {
			return 0, nil, false, e
		}
		return uint32(position), result, true, nil
	}
	boolSort, err := m.MkBoolSort()
	if err != nil {
		return 0, nil, false, err
	}
	for i, ref := range node.variables {
		v, e := m.RefGet(ref)
		if e != nil {
			return 0, nil, false, e
		}
		sort, e := m.GetSort(v)
		if e != nil {
			return 0, nil, false, e
		}
		if sort != boolSort {
			continue
		}
		entry, ok := variables.index[v]
		if !ok {
			return 0, nil, false, ast.ErrASTDomain
		}
		result, found, e = plugin.solvePolarized(variables.entries[entry].contains, formula, node.atoms)
		if e != nil {
			return 0, nil, false, e
		}
		if found {
			return uint32(i), result, true, nil
		}
	}
	return 0, nil, false, nil
}

func solveExprQEBool(node *exprQETree, variables *exprQEBranchVariables, plugin *BoolPlugin, conjs *ConjunctionEnum, config Config, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer) (child *exprQETree, solved bool, err error) {
	index, result, found, err := chooseExprQEBoolElimination(node, variables, plugin, conjs)
	if err != nil || !found {
		return nil, false, err
	}
	defer func() { err = errors.Join(err, result.Close()) }()
	formula, err := node.manager.RefGet(result.Formula)
	if err != nil {
		return nil, false, err
	}
	definition, err := node.manager.RefGet(result.Definition)
	if err != nil {
		return nil, false, err
	}
	child, err = eliminateExprQETreeVariable(node, index, formula, definition, variables, config, rw, nnf)
	return child, err == nil, err
}

// solveExprQEBoolUnits repeats the Boolean plugin's source solve pass until
// no more variables can be eliminated. Each successful pass owns a new child.
func solveExprQEBoolUnits(node *exprQETree, variables *exprQEBranchVariables, plugin *BoolPlugin, config Config, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer) (current *exprQETree, err error) {
	current = node
	for {
		if current == nil || current.manager == nil || current.formula == (ast.ASTRef{}) {
			return nil, ast.ErrASTHandle
		}
		formula, e := current.manager.RefGet(current.formula)
		if e != nil {
			return nil, e
		}
		conjs, e := NewConjunctionEnum(current.manager, formula)
		if e != nil {
			return nil, e
		}
		child, solved, solveErr := solveExprQEBool(current, variables, plugin, conjs, config, rw, nnf)
		if e = errors.Join(solveErr, conjs.Close()); e != nil {
			return nil, e
		}
		if !solved {
			return current, nil
		}
		current = child
	}
}
