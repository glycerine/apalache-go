package rel

import "github.com/glycerine/g3/pkg/ast"

// CheckedConcreteKinds reports the actual finite/index plugin or checked pair.
// Explanation AST ownership remains in the separately owned inner relations.
func (p *FiniteExplanationProduct) CheckedConcreteKinds() (primary, checker string, checked bool, err error) {
	if err = p.open(); err != nil {
		return "", "", false, err
	}
	index, ok := p.table.(*CheckedExplanationIndex)
	if !ok {
		switch p.table.(type) {
		case *SparseTable:
			return "sparse", "", false, nil
		case *LazySparseTable:
			return "lazy_sparse", "", false, nil
		default:
			return "", "", false, ErrTableRelation
		}
	}
	primary, checker, err = index.ConcreteKinds()
	return primary, checker, true, err
}

// IndexSignature includes the final functional inner-relation index. Even a
// nullary predicate therefore has a nonempty index signature.
func (p *FiniteExplanationProduct) IndexSignature() (Signature, error) {
	if err := p.open(); err != nil {
		return Signature{}, err
	}
	return p.table.Signature()
}

// Derived products preserve the existing index plugin and lazy engine after
// the manager restores its query-scoped selection. Plain products entering a
// checked query use its capability-aware favourite selection.
func (p *FiniteExplanationProduct) newProduct(sorts []ast.ASTHandle, explanations int) (*FiniteExplanationProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	_, checked := p.table.(*CheckedExplanationIndex)
	if !checked && p.manager.checkedFinite != nil {
		return newFiniteExplanationProduct(p.manager, sorts, explanations)
	}
	return newFiniteExplanationProductWithIndex(p.manager, sorts, explanations, p.table)
}
