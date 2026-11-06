package tactic

// Source: src/tactic/goal.cpp:691-710.

import "github.com/glycerine/g3/pkg/ast"

func (g *Goal) isLiteral(form ast.ASTHandle) (bool, error) {
	if argument, negated, err := g.notArgument(form); err != nil {
		return false, err
	} else if negated {
		form = argument
	}
	kind, err := g.manager.GetKind(form)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := g.manager.GetFamilyID(form)
	if err != nil || family != g.manager.BasicFamilyID() {
		return err == nil, err
	}
	count, err := g.manager.GetNumArgs(form)
	if err != nil {
		return false, err
	}
	boolean, err := g.manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	for index := uint32(0); index < count; index++ {
		argument, argumentErr := g.manager.GetArg(form, index)
		if argumentErr != nil {
			return false, argumentErr
		}
		sort, sortErr := g.manager.GetSort(argument)
		if sortErr != nil {
			return false, sortErr
		}
		if sort == boolean {
			return false, nil
		}
	}
	return true, nil
}

// IsCNF recognizes the source goal-level CNF grammar: every assertion is a
// literal or a disjunction whose children are literals.
func (g *Goal) IsCNF() (bool, error) {
	if err := g.open(); err != nil {
		return false, err
	}
	for _, form := range g.forms {
		isOr, err := g.isOp(form, ast.ASTOpOr)
		if err != nil {
			return false, err
		}
		if !isOr {
			literal, literalErr := g.isLiteral(form)
			if literalErr != nil || !literal {
				return false, literalErr
			}
			continue
		}
		count, err := g.manager.GetNumArgs(form)
		if err != nil {
			return false, err
		}
		for index := uint32(0); index < count; index++ {
			child, childErr := g.manager.GetArg(form, index)
			if childErr != nil {
				return false, childErr
			}
			literal, literalErr := g.isLiteral(child)
			if literalErr != nil || !literal {
				return false, literalErr
			}
		}
	}
	return true, nil
}
