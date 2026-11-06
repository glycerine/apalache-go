package rel

// Source: relation_base::reset obtains the plugin's interpreted-filter
// mutator and applies false. Product relations dispatch that mutator to every
// concrete component.

import "github.com/glycerine/g3/pkg/ast"

func (p *IntervalBoundProduct) Reset(manager *ast.ASTManager) error {
	if p == nil || manager == nil || !intervalBoundProductKinds(p.kinds) {
		return ast.ErrASTDomain
	}
	bottom, err := manager.MkFalse()
	if err != nil {
		return err
	}
	return p.FilterInterpreted(manager, bottom)
}
