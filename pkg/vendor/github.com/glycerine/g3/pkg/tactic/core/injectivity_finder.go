package core

// Source: src/tactic/core/injectivity_tactic.cpp finder::is_axiom and finder application.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (t *InjectivityTactic) application(h ast.ASTHandle) (decl ast.ASTHandle, args []ast.ASTHandle, app bool, err error) {
	kind, err := t.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return decl, nil, false, err
	}
	decl, err = t.manager.GetDecl(h)
	if err != nil {
		return decl, nil, false, err
	}
	n, err := t.manager.GetNumArgs(h)
	if err != nil {
		return decl, nil, false, err
	}
	args = make([]ast.ASTHandle, n)
	for i := range args {
		args[i], err = t.manager.GetArg(h, uint32(i))
		if err != nil {
			return decl, nil, false, err
		}
	}
	return decl, args, true, nil
}

func (t *InjectivityTactic) equality(h ast.ASTHandle) (left, right ast.ASTHandle, matched bool, err error) {
	decl, args, app, err := t.application(h)
	if err != nil || !app || len(args) != 2 {
		return left, right, false, err
	}
	family, err := t.manager.GetFamilyID(decl)
	if err != nil || family != t.manager.BasicFamilyID() {
		return left, right, false, err
	}
	op, err := t.manager.GetDeclKind(decl)
	if err != nil || op != ast.ASTOpEq {
		return left, right, false, err
	}
	return args[0], args[1], true, nil
}

func (t *InjectivityTactic) inverseAxiom(h ast.ASTHandle) (f, g ast.ASTHandle, matched bool, err error) {
	kind, err := t.manager.GetKind(h)
	if err != nil || kind != ast.ASTQuantifier {
		return f, g, false, err
	}
	forall, err := t.manager.IsForall(h)
	if err != nil || !forall {
		return f, g, false, err
	}
	n, err := t.manager.GetNumDeclarations(h)
	if err != nil || n != 1 {
		return f, g, false, err
	}
	body, err := t.manager.GetQuantifierBody(h)
	if err != nil {
		return f, g, false, err
	}
	a, b, eq, err := t.equality(body)
	if err != nil || !eq {
		return f, g, false, err
	}
	aKind, err := t.manager.GetKind(a)
	if err != nil {
		return f, g, false, err
	}
	bKind, err := t.manager.GetKind(b)
	if err != nil {
		return f, g, false, err
	}
	if aKind != ast.ASTApp || bKind != ast.ASTVar {
		if bKind == ast.ASTApp && aKind == ast.ASTVar {
			a, b = b, a
		} else {
			return f, g, false, nil
		}
	}
	index, err := t.manager.GetVarIndex(b)
	if err != nil || index != 0 {
		return f, g, false, err
	}
	g, outerArgs, app, err := t.application(a)
	if err != nil || !app || len(outerArgs) != 1 {
		return f, g, false, err
	}
	f, innerArgs, app, err := t.application(outerArgs[0])
	if err != nil || !app || len(innerArgs) != 1 || innerArgs[0] != b {
		return f, g, false, err
	}
	return f, g, true, nil
}

func (t *InjectivityTactic) discover(goal *tactic.Goal) error {
	if err := tactic.FailIfUnsatCoreGeneration("injectivity", goal); err != nil {
		return err
	}
	if err := tactic.FailIfProofGeneration("injectivity", goal); err != nil {
		return err
	}
	n, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		formula, err := goal.Form(i)
		if err != nil {
			return err
		}
		f, g, matched, err := t.inverseAxiom(formula)
		if err != nil {
			return err
		}
		if matched {
			if err = t.relations.Insert(f, g); err != nil {
				return err
			}
		}
	}
	return nil
}
