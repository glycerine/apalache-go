package qe

// Source: src/qe/qe_datatype_plugin.cpp, lift_foreign_vars.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// Simplify lifts quantified non-datatype fields out of constructor equalities.
// For C(x...) = t, the source rewrite emits is-C(t) and x_i = accessor_i(t).
func (p *DatatypeSolverPlugin) Simplify(formula ast.ASTRef, _ *rewriter.TheoryRewriter) (changed bool, resultErr error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if formula == (ast.ASTRef{}) {
		return false, ast.ErrASTHandle
	}
	root, err := p.manager.RefGet(formula)
	if err != nil {
		return false, err
	}
	mapping, err := ast.NewASTMapProc(p.manager)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, mapping.Close()) }()

	var visit func(ast.ASTHandle) error
	visit = func(expression ast.ASTHandle) error {
		if _, found, e := mapping.GetExpr(expression); e != nil || found {
			return e
		}
		kind, e := p.manager.GetKind(expression)
		if e != nil {
			return e
		}
		switch kind {
		case ast.ASTVar:
			return mapping.VisitVar(expression)
		case ast.ASTQuantifier:
			body, cause := p.manager.GetQuantifierBody(expression)
			if cause != nil {
				return cause
			}
			if cause = visit(body); cause != nil {
				return cause
			}
			return mapping.VisitQuantifier(expression)
		case ast.ASTApp:
			count, cause := p.manager.GetNumArgs(expression)
			if cause != nil {
				return cause
			}
			for i := uint32(0); i < count; i++ {
				child, getErr := p.manager.GetArg(expression, i)
				if getErr != nil {
					return getErr
				}
				if getErr = visit(child); getErr != nil {
					return getErr
				}
			}
			replacement, lifted, cause := p.liftForeignConstructorEquality(expression)
			if cause != nil {
				return cause
			}
			if lifted {
				changed = true
				return mapping.Insert(expression, replacement)
			}
			return mapping.Reconstruct(expression)
		default:
			return ast.ErrASTDomain
		}
	}
	if err = visit(root); err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}
	rewritten, found, err := mapping.GetExpr(root)
	if err != nil {
		return false, err
	}
	if !found {
		return false, ast.ErrASTDomain
	}
	if err = p.manager.SetRef(formula, rewritten); err != nil {
		return false, err
	}
	return true, nil
}

func (p *DatatypeSolverPlugin) liftForeignConstructorEquality(expression ast.ASTHandle) (ast.ASTHandle, bool, error) {
	family, err := p.manager.GetFamilyID(expression)
	if err != nil || family != p.manager.BasicFamilyID() {
		return ast.ASTHandle{}, false, err
	}
	op, err := p.manager.GetDeclKind(expression)
	if err != nil || op != ast.ASTOpEq {
		return ast.ASTHandle{}, false, err
	}
	left, err := p.manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	right, err := p.manager.GetArg(expression, 1)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if replacement, matched, cause := p.liftConstructorSide(left, right); cause != nil || matched {
		return replacement, matched, cause
	}
	return p.liftConstructorSide(right, left)
}

func (p *DatatypeSolverPlugin) liftConstructorSide(term, value ast.ASTHandle) (ast.ASTHandle, bool, error) {
	constructor, matched, err := p.manager.DatatypeConstructorApp(term)
	if err != nil || !matched {
		return ast.ASTHandle{}, false, err
	}
	argsCount, err := p.manager.GetNumArgs(term)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	hasForeign, err := p.hasForeignConstructorField(term)
	if err != nil || !hasForeign {
		return ast.ASTHandle{}, false, err
	}
	recognizer, err := p.manager.DatatypeConstructorRecognizer(constructor)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	guard, err := p.manager.MkApp(recognizer, []ast.ASTHandle{value})
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	accessors, err := p.manager.DatatypeConstructorAccessors(constructor)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if uint32(len(accessors)) != argsCount {
		return ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	conjuncts := make([]ast.ASTHandle, 1, len(accessors)+1)
	conjuncts[0] = guard
	for i, accessor := range accessors {
		field, cause := p.manager.GetArg(term, uint32(i))
		if cause != nil {
			return ast.ASTHandle{}, false, cause
		}
		selected, cause := p.manager.MkApp(accessor, []ast.ASTHandle{value})
		if cause != nil {
			return ast.ASTHandle{}, false, cause
		}
		equality, cause := p.manager.MkEq(field, selected)
		if cause != nil {
			return ast.ASTHandle{}, false, cause
		}
		conjuncts = append(conjuncts, equality)
	}
	result, err := p.manager.MkAndArgs(conjuncts)
	return result, err == nil, err
}

func (p *DatatypeSolverPlugin) hasForeignConstructorField(term ast.ASTHandle) (bool, error) {
	count, err := p.context.NumVariables()
	if err != nil {
		return false, err
	}
	boolSort, err := p.manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		variable, cause := p.context.Variable(i)
		if cause != nil {
			return false, cause
		}
		sort, cause := p.manager.GetSort(variable)
		if cause != nil {
			return false, cause
		}
		sortFamily, cause := p.manager.GetFamilyID(sort)
		if cause != nil {
			return false, cause
		}
		if sortFamily == p.family || sort == boolSort {
			continue
		}
		contains, cause := p.context.Contains(i)
		if cause != nil {
			return false, cause
		}
		found, cause := contains.Check(term)
		if cause != nil || found {
			return found, cause
		}
	}
	return false, nil
}
