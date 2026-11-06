package rel

import "github.com/glycerine/g3/pkg/ast"

// Source: src/muz/rel/dl_product_relation.cpp product_relation_plugin::mk_full,
// specialized to the live external/interval product.

// NewSelectedExternalIntervalNullaryTotal constructs the source full nullary
// product. The external component uses Empty().Complement(), while the
// interval component is the full zero-column relation.
func NewSelectedExternalIntervalNullaryTotal(plugin *ExternalRelationPlugin, kinds []string) (*ExternalIntervalProduct, bool, error) {
	if !externalIntervalProductKinds(kinds) {
		return nil, false, nil
	}
	if plugin == nil || plugin.closed || plugin.ast == nil || plugin.manager == nil || plugin.manager.open() != nil {
		return nil, true, ErrTableRelation
	}
	external, err := plugin.NewTotal(nil)
	if err != nil {
		return nil, true, err
	}
	return &ExternalIntervalProduct{
		plugin:   plugin,
		kinds:    append([]string(nil), kinds...),
		external: external,
		interval: NewIntervalFull(0),
	}, true, nil
}

// NewSelectedExternalIntervalTotal constructs the source full product for an
// arithmetic signature. The external component is Empty(signature).Complement
// and the interval component is the corresponding full interval relation.
func NewSelectedExternalIntervalTotal(plugin *ExternalRelationPlugin, sorts []ast.ASTHandle, kinds []string) (*ExternalIntervalProduct, bool, error) {
	if !externalIntervalProductKinds(kinds) {
		return nil, false, nil
	}
	if plugin == nil || plugin.closed || plugin.ast == nil || plugin.manager == nil || plugin.manager.open() != nil {
		return nil, true, ErrTableRelation
	}
	if len(sorts) == 0 || !CanHandleIntervalSorts(plugin.ast, sorts) {
		return nil, true, ast.ErrASTDomain
	}
	external, err := plugin.NewTotal(sorts)
	if err != nil {
		return nil, true, err
	}
	return &ExternalIntervalProduct{
		plugin:   plugin,
		sorts:    append([]ast.ASTHandle(nil), sorts...),
		kinds:    append([]string(nil), kinds...),
		external: external,
		interval: NewIntervalFull(uint32(len(sorts))),
	}, true, nil
}
