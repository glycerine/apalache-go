package rel

// Source: relation_manager::default_relation_filter_interpreted_and_project_fn
// clones its input, applies the single compiler-produced interpreted
// condition, then projects the requested columns.

import "github.com/glycerine/g3/pkg/ast"

func (p *IntervalBoundProduct) FilterInterpretedAndProject(manager *ast.ASTManager, condition ast.ASTHandle, removed []uint32) (*IntervalBoundProduct, error) {
	if p == nil || manager == nil || len(removed) == 0 {
		return nil, ast.ErrASTDomain
	}
	working, err := p.Clone()
	if err != nil {
		return nil, err
	}
	if err = working.FilterInterpreted(manager, condition); err != nil {
		return nil, err
	}
	return working.Project(removed)
}
