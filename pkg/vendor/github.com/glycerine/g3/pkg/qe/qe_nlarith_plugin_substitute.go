package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *NonlinearArithmeticPlugin) Substitute(variable *ast.ContainsApp, value *numeral.OwnedRational, formula ast.ASTRef, definition *ast.ASTRef) error {
	if value == nil {
		return ast.ErrASTHandle
	}
	root, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	e, _, err := p.entry(variable, root)
	if err != nil {
		return err
	}
	branch, err := p.manager.NumeralService().Unsigned(value)
	if err != nil || int(branch) >= len(e.branches.Branches) {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	row := e.branches.Branches[branch]
	if len(row.Substitutions) != len(e.branches.Predicates) {
		return ast.ErrASTDomain
	}
	for i := range row.Substitutions {
		predicate, cause := p.manager.RefGet(e.branches.Predicates[i])
		if cause != nil {
			return cause
		}
		replacement, cause := p.manager.RefGet(row.Substitutions[i])
		if cause != nil {
			return cause
		}
		if cause = p.replacer.ApplySubstitution(predicate, replacement, formula); cause != nil {
			return cause
		}
	}
	rewritten, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	constraint, err := p.manager.RefGet(row.Constraint)
	if err != nil {
		return err
	}
	combined, err := p.manager.MkAnd(constraint, rewritten)
	if err != nil {
		return err
	}
	if err = p.manager.SetRef(formula, combined); err != nil {
		return err
	}
	if definition != nil {
		d, err := p.manager.RefGet(row.Definition)
		if err != nil {
			return err
		}
		return p.manager.SetRef(*definition, d)
	}
	return nil
}
