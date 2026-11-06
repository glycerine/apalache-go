package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// FilterInterpreted keeps rows for which condition rewrites to true after
// substituting the finite values in relation-column order.
func (p *FiniteSparseProduct) FilterInterpreted(condition ast.ASTHandle, theory *rewriter.TheoryRewriter) error {
	if err := p.open(); err != nil {
		return err
	}
	if theory == nil {
		return ErrTableRelation
	}
	m := p.manager.manager
	kind, err := m.GetKind(condition)
	if err != nil {
		return err
	}
	if kind != ast.ASTApp {
		return ast.ErrASTDomain
	}
	conditionSort, err := m.GetSort(condition)
	if err != nil {
		return err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	if conditionSort != boolean {
		return ast.ErrASTDomain
	}
	used, err := ast.NewASTUsedVars(m)
	if err != nil {
		return err
	}
	if err = used.Collect(condition); err != nil {
		return err
	}
	if used.MaxFoundVarIndexPlusOne() > uint32(len(p.sorts)) {
		return ast.ErrASTDomain
	}
	flat, err := p.flatten()
	if err != nil {
		return err
	}
	defer flat.Close()
	facts, err := flat.Facts()
	if err != nil {
		return err
	}
	kept := make([]Fact, 0, len(facts))
	for _, fact := range facts {
		accepted, evalErr := p.evaluateInterpreted(condition, theory, fact)
		if evalErr != nil {
			return evalErr
		}
		if accepted {
			kept = append(kept, fact)
		}
	}
	return p.replaceFacts(kept)
}

func (p *FiniteSparseProduct) evaluateInterpreted(condition ast.ASTHandle, theory *rewriter.TheoryRewriter, fact Fact) (accepted bool, resultErr error) {
	m := p.manager.manager
	bindings := make([]ast.ASTHandle, len(p.sorts))
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	for column, sort := range p.sorts {
		value, err := p.manager.DecodeValue(sort, fact[column])
		if err != nil {
			return false, err
		}
		refs = append(refs, value)
		handle, err := m.RefGet(value)
		if err != nil {
			return false, err
		}
		bindings[len(bindings)-1-column] = handle
	}
	output, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	refs = append(refs, output)
	if err = theory.RewriteWithBindings(condition, bindings, output); err != nil {
		return false, err
	}
	value, err := m.RefGet(output)
	if err != nil {
		return false, err
	}
	if m.IsTrue(value) {
		return true, nil
	}
	if m.IsFalse(value) {
		return false, nil
	}
	return false, ErrTableRelation
}
