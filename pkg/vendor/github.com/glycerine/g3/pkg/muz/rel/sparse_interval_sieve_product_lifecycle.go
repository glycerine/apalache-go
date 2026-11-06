package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Clone returns an independent copy of both source product components.
func (p *SparseIntervalSieveProduct) Clone() (*SparseIntervalSieveProduct, error) {
	if p == nil {
		return nil, ErrSparseIntervalProduct
	}
	return p.CloneTo(p.manager)
}

// CloneTo returns an independent copy bound to another live relation manager
// over the same AST manager. The clone does not borrow the source manager's
// lifecycle.
func (p *SparseIntervalSieveProduct) CloneTo(manager *RelationManager) (*SparseIntervalSieveProduct, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if manager == nil || manager.open() != nil || manager.manager != p.manager.manager {
		return nil, ErrSparseIntervalProduct
	}
	finite, err := p.finite.Clone()
	if err != nil {
		return nil, err
	}
	interval, err := p.interval.Clone()
	if err != nil {
		return nil, errors.Join(err, finite.Close())
	}
	return &SparseIntervalSieveProduct{
		manager:         manager,
		sorts:           append([]ast.ASTHandle(nil), p.sorts...),
		finiteColumns:   append([]int(nil), p.finiteColumns...),
		finiteSorts:     append([]ast.ASTHandle(nil), p.finiteSorts...),
		finite:          finite,
		interval:        interval,
		intervalShape:   p.intervalShape,
		intervalNullary: p.intervalNullary,
	}, nil
}

// Reset makes both independent product components empty.
func (p *SparseIntervalSieveProduct) Reset() error {
	if err := p.open(); err != nil {
		return err
	}
	pureFinite := len(p.finiteColumns) == len(p.sorts) && !p.intervalNullary
	pureArithmetic := len(p.sorts) != 0 && len(p.finiteColumns) == 0
	var interval *IntervalSieve
	var err error
	if pureFinite {
		interval, err = NewIntervalSieveFull(p.manager.manager, p.sorts)
	} else {
		interval, err = NewIntervalSieveEmpty(p.manager.manager, p.sorts)
	}
	if err != nil {
		return err
	}
	if err = p.finite.Reset(); err != nil {
		return err
	}
	if pureArithmetic || p.intervalNullary {
		signature, signatureErr := p.finite.Signature()
		if signatureErr != nil {
			return signatureErr
		}
		if err = fillTotalTable(p.finite, signature); err != nil {
			return err
		}
	}
	p.interval = interval
	return nil
}
