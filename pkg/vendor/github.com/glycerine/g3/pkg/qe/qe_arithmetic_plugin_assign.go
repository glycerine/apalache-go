package qe

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *ArithmeticPlugin) Assign(variable *ast.ContainsApp, formula ast.ASTHandle, value *numeral.OwnedRational) (resultErr error) {
	if value == nil {
		return ast.ErrASTHandle
	}
	if err := p.open(); err != nil {
		return err
	}
	e, target, err := p.entry(variable, formula)
	if err != nil {
		return err
	}
	branchNumber, err := p.manager.NumeralService().Unsigned(value)
	if err != nil {
		return err
	}
	key := ArithmeticBranchKey{Formula: formula, Variable: target, Branch: branchNumber}
	if cached := p.branches[key]; cached != nil {
		return p.replayBranch(cached)
	}
	freshStart := len(p.pendingFresh)
	v := new(big.Rat).SetInt64(int64(branchNumber))
	selection, err := p.SelectBranch(variable, formula, v)
	if err != nil {
		return err
	}
	result, err := p.manager.NewRef(formula)
	if err != nil {
		return err
	}
	var definition ast.ASTRef
	defer func() {
		if resultErr != nil {
			var definitionErr error
			if definition != (ast.ASTRef{}) {
				definitionErr = p.manager.DelRef(&definition)
			}
			resultErr = errors.Join(resultErr, definitionErr, p.manager.DelRef(&result))
		}
	}()
	substitution, _, err := p.applyCooper(target, e.processor, result)
	if err != nil {
		return err
	}
	if selection.Kind == ArithmeticInfinityBranch {
		err = p.rewriteInfinity(e, selection.Lower, result)
	} else {
		err = p.rewriteFinite(e, target, selection, result)
	}
	if err == nil {
		definition, err = p.branchDefinition(selection, substitution)
	}
	if err != nil {
		return err
	}
	return p.storeBranch(key, result, definition, selection, p.pendingFresh[freshStart:])
}
