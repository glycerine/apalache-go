package rel

// Source: src/muz/rel/dl_product_relation.cpp/.h, concrete product of the
// interval and bound relations registered by rel_context.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// IntervalBoundProduct is the concrete interval/bound product relation.
type IntervalBoundProduct struct {
	interval *IntervalRelation
	bound    *BoundRelation
	kinds    []string
}

func NewIntervalBoundProduct(arity uint32, empty bool) *IntervalBoundProduct {
	interval := NewIntervalFull(arity)
	if empty {
		interval = NewIntervalEmpty(arity)
	}
	return &IntervalBoundProduct{interval: interval, bound: NewBoundRelation(arity, empty), kinds: []string{"interval_relation", "bound_relation"}}
}
func (p *IntervalBoundProduct) Empty() bool { return p == nil || p.interval.Empty() || p.bound.Empty() }

func (p *IntervalBoundProduct) AddFact(values []*big.Rat) error {
	if p == nil || p.interval == nil || p.bound == nil || len(values) != len(p.bound.columns) {
		return ast.ErrASTDomain
	}
	for _, value := range values {
		if value == nil {
			return ast.ErrASTDomain
		}
	}
	if err := p.interval.AddFact(values); err != nil {
		return err
	}
	return p.bound.AddFact(make([]ast.ASTHandle, len(values)))
}

func (p *IntervalBoundProduct) ContainsFact(values []*big.Rat) bool {
	return p != nil && p.interval != nil && p.bound != nil && p.interval.ContainsFact(values) && p.bound.ContainsFact(make([]ast.ASTHandle, len(values)))
}

// Union combines aligned components. The C++ product skips cross-component
// unions unless both directions are supported; interval and bound do not
// satisfy that condition.
func (p *IntervalBoundProduct) Union(source, delta *IntervalBoundProduct, widen bool) (bool, error) {
	if p == nil || source == nil || p.interval == nil || source.interval == nil || p.interval.values == nil || source.interval.values == nil || p.bound == nil || source.bound == nil {
		return false, ast.ErrASTDomain
	}
	arity := len(p.bound.columns)
	if len(p.interval.values.columns) != arity || len(source.bound.columns) != arity || len(source.interval.values.columns) != arity {
		return false, ast.ErrASTDomain
	}
	if delta != nil && (delta == p || delta == source || delta.interval == nil || delta.interval.values == nil || delta.bound == nil || len(delta.bound.columns) != arity || len(delta.interval.values.columns) != arity) {
		return false, ast.ErrASTDomain
	}
	if delta == nil {
		changedInterval, err := p.interval.Union(source.interval, widen)
		if err != nil {
			return false, err
		}
		changedBound, err := p.bound.Union(source.bound)
		return changedInterval || changedBound, err
	}
	// The source unions into fresh component deltas, then unions those into
	// the supplied delta, which may already contain results from earlier work.
	fresh := NewIntervalBoundProduct(uint32(arity), true)
	changedInterval, err := p.interval.UnionWithDelta(source.interval, fresh.interval, widen)
	if err != nil {
		return false, err
	}
	changedBound, err := p.bound.UnionWithDelta(source.bound, fresh.bound, widen)
	if err != nil {
		return false, err
	}
	if _, err = delta.interval.Union(fresh.interval, false); err != nil {
		return false, err
	}
	if _, err = delta.bound.Union(fresh.bound); err != nil {
		return false, err
	}
	return changedInterval || changedBound, nil
}

func (p *IntervalBoundProduct) FilterInterpreted(m *ast.ASTManager, condition ast.ASTHandle) error {
	if p == nil || m == nil || !intervalBoundProductKinds(p.kinds) {
		return ast.ErrASTDomain
	}
	for _, kind := range p.kinds {
		switch kind {
		case "interval_relation":
			if p.interval == nil {
				return ast.ErrASTDomain
			}
			if err := p.interval.FilterInterpreted(m, condition); err != nil {
				return err
			}
		case "bound_relation":
			if p.bound == nil {
				return ast.ErrASTDomain
			}
			if _, err := p.bound.FilterInterpreted(m, condition); err != nil {
				return err
			}
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (p *IntervalBoundProduct) FilterEqual(column uint32, value *big.Rat) error {
	if p == nil || p.interval == nil || p.bound == nil {
		return ast.ErrASTDomain
	}
	return p.interval.FilterEqual(column, value)
}

func (p *IntervalBoundProduct) FilterIdentical(first, second uint32) error {
	if p == nil || p.interval == nil || p.bound == nil {
		return ast.ErrASTDomain
	}
	if err := p.interval.FilterIdentical(first, second); err != nil {
		return err
	}
	return p.bound.Equate(first, second)
}

// ToFormula returns the source conjunction of both component formulas.
func (p *IntervalBoundProduct) ToFormula(m *ast.ASTManager, sorts []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if p == nil || p.interval == nil || p.bound == nil || m == nil {
		return result, ast.ErrASTDomain
	}
	first, err := p.interval.ToFormula(m, sorts)
	if err != nil {
		return result, err
	}
	var second ast.ASTRef
	owned := false
	defer func() {
		if second != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&second))
		}
		resultErr = errors.Join(resultErr, m.DelRef(&first))
		if resultErr != nil && owned {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	second, err = p.bound.ToFormula(m, sorts)
	if err != nil {
		return result, err
	}
	a, err := m.RefGet(first)
	if err != nil {
		return result, err
	}
	b, err := m.RefGet(second)
	if err != nil {
		return result, err
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	owned = true
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, []ast.ASTHandle{a, b}, result); err != nil {
		return result, err
	}
	return result, nil
}

func JoinIntervalBoundProducts(left, right *IntervalBoundProduct, leftColumns, rightColumns []uint32) (*IntervalBoundProduct, error) {
	if left == nil || right == nil {
		return nil, ast.ErrASTDomain
	}
	interval, err := JoinIntervalRelations(left.interval, right.interval, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	bound, err := JoinBoundRelations(left.bound, right.bound, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	return &IntervalBoundProduct{interval: interval, bound: bound, kinds: append([]string(nil), left.kinds...)}, nil
}

func (p *IntervalBoundProduct) Project(removed []uint32) (*IntervalBoundProduct, error) {
	if p == nil || p.interval == nil || p.bound == nil {
		return nil, ast.ErrASTDomain
	}
	interval, err := p.interval.Project(removed)
	if err != nil {
		return nil, err
	}
	bound, err := p.bound.Project(removed)
	if err != nil {
		return nil, err
	}
	return &IntervalBoundProduct{interval: interval, bound: bound, kinds: append([]string(nil), p.kinds...)}, nil
}

func (p *IntervalBoundProduct) Rename(cycle []uint32) (*IntervalBoundProduct, error) {
	if p == nil || p.interval == nil || p.bound == nil {
		return nil, ast.ErrASTDomain
	}
	interval, err := p.interval.Rename(cycle)
	if err != nil {
		return nil, err
	}
	bound, err := p.bound.Rename(cycle)
	if err != nil {
		return nil, err
	}
	return &IntervalBoundProduct{interval: interval, bound: bound, kinds: append([]string(nil), p.kinds...)}, nil
}
