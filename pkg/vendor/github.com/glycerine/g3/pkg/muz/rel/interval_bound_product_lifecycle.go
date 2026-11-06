package rel

import "github.com/glycerine/g3/pkg/ast"

func (p *IntervalBoundProduct) Clone() (*IntervalBoundProduct, error) {
	if p == nil || p.interval == nil || p.bound == nil || !intervalBoundProductKinds(p.kinds) {
		return nil, ast.ErrASTDomain
	}
	interval, err := p.interval.Clone()
	if err != nil {
		return nil, err
	}
	bound := p.bound.Clone()
	if bound == nil {
		return nil, ast.ErrASTDomain
	}
	return &IntervalBoundProduct{interval: interval, bound: bound, kinds: append([]string(nil), p.kinds...)}, nil
}

// IsPrecise mirrors product_relation::is_precise. Both retained components
// override is_precise with false, so their product is always approximate.
func (p *IntervalBoundProduct) IsPrecise() (bool, error) {
	if p == nil || p.interval == nil || p.bound == nil || !intervalBoundProductKinds(p.kinds) {
		return false, ast.ErrASTDomain
	}
	return false, nil
}
