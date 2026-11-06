package rel

// Source: rel_context.cpp multi-plugin representation selection and
// dl_product_relation.cpp product construction for bound_relation plus
// interval_relation.

import "github.com/glycerine/g3/pkg/ast"

func intervalBoundProductKinds(kinds []string) bool {
	if len(kinds) != 2 || kinds[0] == kinds[1] {
		return false
	}
	return kinds[0] == "interval_relation" && kinds[1] == "bound_relation" || kinds[0] == "bound_relation" && kinds[1] == "interval_relation"
}

// CanSelectIntervalBoundProduct validates the exact live product pair and
// the common Int/Real (or nullary) signature supported by both components.
func CanSelectIntervalBoundProduct(manager *ast.ASTManager, sorts []ast.ASTHandle, kinds []string) (bool, error) {
	if !intervalBoundProductKinds(kinds) {
		return false, nil
	}
	plugin, err := NewBoundRelationPlugin(manager)
	if err != nil {
		return false, err
	}
	bound, err := plugin.CanHandleSignature(sorts)
	if err != nil || !bound {
		return false, err
	}
	return CanHandleIntervalSorts(manager, sorts), nil
}

// NewSelectedIntervalBoundProduct constructs the selected product while
// retaining the caller's component order for subsequent transforms. A
// non-matching kind list is not selected; an exact pair with an unsupported
// signature is a configuration error.
func NewSelectedIntervalBoundProduct(manager *ast.ASTManager, sorts []ast.ASTHandle, kinds []string, empty bool) (*IntervalBoundProduct, bool, error) {
	if !intervalBoundProductKinds(kinds) {
		return nil, false, nil
	}
	selected, err := CanSelectIntervalBoundProduct(manager, sorts, kinds)
	if err != nil {
		return nil, true, err
	}
	if !selected {
		return nil, true, ast.ErrASTDomain
	}
	product := NewIntervalBoundProduct(uint32(len(sorts)), empty)
	product.kinds = append([]string(nil), kinds...)
	return product, true, nil
}

func (p *IntervalBoundProduct) Kinds() []string {
	if p == nil || !intervalBoundProductKinds(p.kinds) {
		return nil
	}
	return append([]string(nil), p.kinds...)
}
