package qe

// Source: src/qe/qe.cpp update_current, Boolean, bit-vector and arithmetic branches.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
)

// stepExprQEScalarBranch first records an assignment, then substitutes its
// concrete branch on the next visit.
func stepExprQEScalarBranch(parent *exprQETree, variables *exprQEBranchVariables, plugin SolverPlugin, branch *big.Rat, config Config, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer, solver *smtcore.SMTSolver, withDefinitions bool) (child *exprQETree, status exprQETreeStatus, err error) {
	if parent == nil || parent.manager == nil || variables == nil || variables.manager != parent.manager || plugin == nil || branch == nil || rw == nil || nnf == nil || parent.selected == (ast.ASTRef{}) {
		return nil, status, ast.ErrASTHandle
	}
	switch plugin.(type) {
	case *BoolPlugin, *BitVectorPlugin:
	case *ArithmeticPlugin, *NonlinearArithmeticPlugin, *DatatypeSolverPlugin, *DatalogPlugin:
		if solver == nil {
			return nil, status, ast.ErrASTHandle
		}
	default:
		return nil, status, ast.ErrASTDomain
	}
	m := parent.manager
	var datatype *DatatypeSolverPlugin
	var oldVariables int
	if p, ok := plugin.(*DatatypeSolverPlugin); ok {
		datatype = p
		oldVariables = len(p.context.variables)
	}
	variable, err := m.RefGet(parent.selected)
	if err != nil {
		return nil, status, err
	}
	entry, ok := variables.index[variable]
	if !ok {
		return nil, status, ast.ErrASTDomain
	}
	formula, err := m.RefGet(parent.formula)
	if err != nil {
		return nil, status, err
	}
	n, err := m.NumeralService().NewValue()
	if err != nil {
		return nil, status, err
	}
	defer func() { err = errors.Join(err, m.NumeralService().Del(&n)) }()
	if err = m.NumeralService().SetString(&n, branch.RatString()); err != nil {
		return nil, status, err
	}
	index, exists := parent.branchIndex[branch.RatString()]
	if !exists {
		proxy, e := variables.branch(variable)
		if e != nil {
			return nil, status, e
		}
		assignment, e := exprQEBranchEquality(m, proxy, branch)
		if e != nil {
			return nil, status, e
		}
		defer func() { err = errors.Join(err, m.DelRef(&assignment)) }()
		h, e := m.RefGet(assignment)
		if e != nil {
			return nil, status, e
		}
		child, e = parent.addBranch(config, branch, h)
		if e != nil {
			return nil, status, e
		}
		switch p := plugin.(type) {
		case *BoolPlugin:
			e = p.Assign(variables.entries[entry].contains, formula, &n)
		case *BitVectorPlugin:
			e = p.Assign(variables.entries[entry].contains, formula, &n)
		case *ArithmeticPlugin:
			e = p.Assign(variables.entries[entry].contains, formula, &n)
		case *NonlinearArithmeticPlugin:
			e = p.Assign(variables.entries[entry].contains, formula, &n)
		case *DatatypeSolverPlugin:
			e = p.Assign(variables.entries[entry].contains, formula, &n)
		case *DatalogPlugin:
			e = p.Assign(variables.entries[entry].contains, formula, &n)
		}
		if e != nil {
			return nil, status, e
		}
		if p, ok := plugin.(*ArithmeticPlugin); ok {
			if e = drainExprQEArithmeticOutputs(p, child, variables, solver); e != nil {
				return nil, status, e
			}
		}
		if p, ok := plugin.(*NonlinearArithmeticPlugin); ok {
			constraints, cause := p.TakeConstraints()
			if cause != nil {
				return nil, status, cause
			}
			if cause = drainExprQEConstraints(constraints, child, solver); cause != nil {
				return nil, status, cause
			}
		}
		if datatype != nil {
			constraints, cause := datatype.TakeConstraints()
			if cause != nil {
				return nil, status, cause
			}
			if cause = drainExprQEConstraints(constraints, child, solver); cause != nil {
				return nil, status, cause
			}
		}
		if p, ok := plugin.(*DatalogPlugin); ok {
			constraints, cause := p.TakeConstraints()
			if cause != nil {
				return nil, status, cause
			}
			if cause = drainExprQEConstraints(constraints, child, solver); cause != nil {
				return nil, status, cause
			}
		}
		if e = variables.consume(child); e != nil {
			return nil, status, e
		}
		return child, exprQENeedPropagation, nil
	}
	child = parent.children[index]
	if child.formula != (ast.ASTRef{}) {
		return child, exprQEChooseVariable, nil
	}
	tmp, err := m.NewRef(formula)
	if err != nil {
		return nil, status, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
	var definition ast.ASTRef
	var definitionArg *ast.ASTRef
	if withDefinitions {
		definition, err = m.NewRef(ast.ASTHandle{})
		if err != nil {
			return nil, status, err
		}
		defer func() { err = errors.Join(err, m.DelRef(&definition)) }()
		definitionArg = &definition
	}
	switch p := plugin.(type) {
	case *BoolPlugin:
		err = p.Substitute(variables.entries[entry].contains, &n, tmp, definitionArg)
	case *BitVectorPlugin:
		err = p.Substitute(variables.entries[entry].contains, &n, tmp, definitionArg)
	case *ArithmeticPlugin:
		err = p.Substitute(variables.entries[entry].contains, &n, tmp, definitionArg)
	case *NonlinearArithmeticPlugin:
		err = p.Substitute(variables.entries[entry].contains, &n, tmp, definitionArg)
	case *DatatypeSolverPlugin:
		var supported bool
		supported, err = p.SubstituteNonRecursive(variables.entries[entry].contains, &n, tmp, definitionArg)
		if err == nil && !supported {
			supported, err = p.SubstituteRecursive(variables.entries[entry].contains, &n, tmp, definitionArg)
		}
		if err == nil && !supported {
			err = ast.ErrASTDomain
		}
	case *DatalogPlugin:
		err = p.Substitute(variables.entries[entry].contains, &n, tmp, definitionArg)
	}
	if err != nil {
		return nil, status, err
	}
	// A recursive equality branch assumes the selected constructor. Preserve
	// that assumption on the replacement term in the projected formula.
	if datatype != nil {
		cached := datatype.equalities[arithmeticPluginKey{variable: variable, formula: formula}]
		index, e := m.NumeralService().Unsigned(&n)
		if e != nil {
			return nil, status, e
		}
		if cached != nil && uint64(index) < uint64(len(cached.positive)) {
			constructor, found, e := datatype.findRecognizer(variable, formula)
			if e != nil {
				return nil, status, e
			}
			if found {
				recognizer, e := m.DatatypeConstructorRecognizer(constructor)
				if e != nil {
					return nil, status, e
				}
				guard, e := m.MkApp(recognizer, []ast.ASTHandle{cached.positive[index].value})
				if e != nil {
					return nil, status, e
				}
				guardRef, e := m.NewRef(guard)
				if e != nil {
					return nil, status, e
				}
				defer func() { err = errors.Join(err, m.DelRef(&guardRef)) }()
				value, e := m.RefGet(tmp)
				if e != nil {
					return nil, status, e
				}
				value, e = m.MkAnd(guard, value)
				if e != nil {
					return nil, status, e
				}
				if e = m.SetRef(tmp, value); e != nil {
					return nil, status, e
				}
			}
		}
	}
	if p, ok := plugin.(*ArithmeticPlugin); ok {
		if err = drainExprQEArithmeticOutputs(p, child, variables, solver); err != nil {
			return nil, status, err
		}
	}
	if datatype != nil {
		for _, fresh := range datatype.context.variables[oldVariables:] {
			if err = variables.add(fresh); err != nil {
				return nil, status, err
			}
		}
	}
	if err = variables.consume(child); err != nil {
		return nil, status, err
	}
	h, err := m.RefGet(tmp)
	if err != nil {
		return nil, status, err
	}
	if err = child.initialize(h); err != nil {
		return nil, status, err
	}
	if withDefinitions {
		d, e := m.RefGet(definition)
		if e != nil {
			return nil, status, e
		}
		if e = child.addDefinition(variable, d); e != nil {
			return nil, status, e
		}
	}
	if err = normalizeExprQETree(child, rw, nnf); err != nil {
		return nil, status, err
	}
	return child, exprQEChooseVariable, nil
}
