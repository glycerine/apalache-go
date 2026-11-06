package rel

// Source: src/muz/rel/rel_context.cpp multi-plugin predicate
// representations and src/muz/rel/dl_product_relation.cpp/.h.

import "github.com/glycerine/g3/pkg/ast"

// ExternalIntervalProduct is the concrete source product for an arithmetic
// predicate represented simultaneously by external_relation and
// interval_relation. Both components describe the complete predicate
// signature; their conjunction is the represented relation.
type ExternalIntervalProduct struct {
	plugin   *ExternalRelationPlugin
	sorts    []ast.ASTHandle
	kinds    []string
	external *ExternalRelation
	interval *IntervalRelation
	closed   bool
}

func externalIntervalProductKinds(kinds []string) bool {
	if len(kinds) != 2 || kinds[0] == kinds[1] {
		return false
	}
	return kinds[0] == "external_relation" && kinds[1] == "interval_relation" ||
		kinds[0] == "interval_relation" && kinds[1] == "external_relation"
}

// NewSelectedExternalIntervalProduct constructs the exact live mixed product
// selected by rel_context. A different kind list is not selected; the exact
// pair with a non-arithmetic signature is a configuration error.
func NewSelectedExternalIntervalProduct(plugin *ExternalRelationPlugin, sorts []ast.ASTHandle, kinds []string) (*ExternalIntervalProduct, bool, error) {
	if !externalIntervalProductKinds(kinds) {
		return nil, false, nil
	}
	if plugin == nil || plugin.closed || plugin.ast == nil || plugin.manager == nil || plugin.manager.open() != nil {
		return nil, true, ErrTableRelation
	}
	if !CanHandleIntervalSorts(plugin.ast, sorts) {
		return nil, true, ast.ErrASTDomain
	}
	external, err := plugin.NewEmpty(sorts)
	if err != nil {
		return nil, true, err
	}
	return &ExternalIntervalProduct{
		plugin:   plugin,
		sorts:    append([]ast.ASTHandle(nil), sorts...),
		kinds:    append([]string(nil), kinds...),
		external: external,
		interval: NewIntervalEmpty(uint32(len(sorts))),
	}, true, nil
}

func (p *ExternalIntervalProduct) open() error {
	if p == nil || p.closed || p.plugin == nil || p.external == nil || p.interval == nil || p.interval.values == nil {
		return ErrTableRelation
	}
	if err := p.external.open(); err != nil {
		return err
	}
	return nil
}

// Kinds returns the selected component order used by source product
// short-circuit operations.
func (p *ExternalIntervalProduct) Kinds() []string {
	if p == nil || p.open() != nil {
		return nil
	}
	return append([]string(nil), p.kinds...)
}

// Sorts returns the borrowed signature handles in a caller-owned slice.
func (p *ExternalIntervalProduct) Sorts() ([]ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), p.sorts...), nil
}

func (p *ExternalIntervalProduct) externalFirst() bool {
	return len(p.kinds) == 2 && p.kinds[0] == "external_relation"
}

// IsPrecise is false because the interval component is an abstraction.
func (p *ExternalIntervalProduct) IsPrecise() bool { return false }

// Empty mirrors product_relation::empty: the conjunction is empty when the
// first empty component is encountered, preserving requested component order.
func (p *ExternalIntervalProduct) Empty() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if !p.externalFirst() && p.interval.Empty() {
		return true, nil
	}
	empty, err := p.external.Empty()
	if err != nil || empty {
		return empty, err
	}
	return p.interval.Empty(), nil
}

// AddFact applies the fact to both components. The interval update is prepared
// privately before the observable external STORE callback and published only
// after that callback reports success, so local interval state is not advanced
// by a rejected callback.
func (p *ExternalIntervalProduct) AddFact(values []ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	candidate, err := p.interval.Clone()
	if err != nil {
		return err
	}
	if err = candidate.AddFactExpressions(p.plugin.ast, p.sorts, values); err != nil {
		return err
	}
	if err = p.external.AddFact(values); err != nil {
		return err
	}
	p.interval = candidate
	return nil
}

// Clone copies both aligned components and preserves their source kind order.
func (p *ExternalIntervalProduct) Clone() (*ExternalIntervalProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	interval, err := p.interval.Clone()
	if err != nil {
		return nil, err
	}
	external, err := p.external.Clone()
	if err != nil {
		return nil, err
	}
	return &ExternalIntervalProduct{
		plugin:   p.plugin,
		sorts:    append([]ast.ASTHandle(nil), p.sorts...),
		kinds:    append([]string(nil), p.kinds...),
		external: external,
		interval: interval,
	}, nil
}

func (p *ExternalIntervalProduct) transformed(external *ExternalRelation, interval *IntervalRelation) *ExternalIntervalProduct {
	return &ExternalIntervalProduct{
		plugin:   p.plugin,
		sorts:    append([]ast.ASTHandle(nil), external.sorts...),
		kinds:    append([]string(nil), p.kinds...),
		external: external,
		interval: interval,
	}
}

// Project applies the same column removal to both aligned components.
func (p *ExternalIntervalProduct) Project(removed []uint32) (*ExternalIntervalProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	interval, err := p.interval.Project(removed)
	if err != nil {
		return nil, err
	}
	external, err := p.external.Project(removed)
	if err != nil {
		return nil, err
	}
	return p.transformed(external, interval), nil
}

// Rename applies the source permutation cycle to both aligned components.
func (p *ExternalIntervalProduct) Rename(cycle []uint32) (*ExternalIntervalProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	interval, err := p.interval.Rename(cycle)
	if err != nil {
		return nil, err
	}
	external, err := p.external.Rename(cycle)
	if err != nil {
		return nil, err
	}
	return p.transformed(external, interval), nil
}

// FilterEqual intersects both components with one arithmetic value. The
// interval candidate is published only after the external callback succeeds.
func (p *ExternalIntervalProduct) FilterEqual(column uint32, value ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	candidate, err := p.interval.Clone()
	if err != nil {
		return err
	}
	if err = candidate.FilterEqualExpression(p.plugin.ast, column, value); err != nil {
		return err
	}
	if err = p.external.FilterEqual(column, value); err != nil {
		return err
	}
	p.interval = candidate
	return nil
}

// FilterIdentical intersects both components with the same equality class.
func (p *ExternalIntervalProduct) FilterIdentical(columns []uint32) error {
	if err := p.open(); err != nil {
		return err
	}
	candidate, err := p.interval.Clone()
	if err != nil {
		return err
	}
	if err = candidate.FilterIdenticalColumns(columns); err != nil {
		return err
	}
	if err = p.external.FilterIdentical(columns); err != nil {
		return err
	}
	p.interval = candidate
	return nil
}

// FilterInterpreted applies one source condition to both components.
func (p *ExternalIntervalProduct) FilterInterpreted(condition ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	candidate, err := p.interval.Clone()
	if err != nil {
		return err
	}
	if err = candidate.FilterInterpreted(p.plugin.ast, condition); err != nil {
		return err
	}
	if err = p.external.FilterInterpreted(condition); err != nil {
		return err
	}
	p.interval = candidate
	return nil
}

// Close releases the owned external relation. A refused close leaves the
// product intact and retryable.
func (p *ExternalIntervalProduct) Close() error {
	if p == nil || p.closed {
		return nil
	}
	if p.external != nil {
		if err := p.external.Close(); err != nil {
			return err
		}
	}
	p.closed = true
	p.plugin = nil
	p.external = nil
	p.interval = nil
	p.sorts = nil
	p.kinds = nil
	return nil
}
