package rel

// Source: src/muz/rel/rel_context.cpp multi-name predicate representation,
// dl_product_relation.cpp product components, and dl_sieve_relation.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrSparseIntervalProduct = errors.New("muz/rel: invalid sparse/interval product")

type sparseIntervalComponentShape uint8

const (
	sparseIntervalShapeAbsent sparseIntervalComponentShape = iota + 1
	sparseIntervalShapeRaw
	sparseIntervalShapeSieve
)

// SparseIntervalSieveProduct is the concrete product selected by the ordered
// representation pair tr_sparse + interval_relation. Its components are
// independent abstractions over the same mixed signature, matching source
// product_relation rather than the more precise finite_product_relation.
type SparseIntervalSieveProduct struct {
	manager       *RelationManager
	sorts         []ast.ASTHandle
	finiteColumns []int
	finiteSorts   []ast.ASTHandle
	finite        *SparseTable
	interval      *IntervalSieve
	intervalShape sparseIntervalComponentShape
	// intervalNullary distinguishes interval_relation's zero-arity truth
	// value from a hashtable zero-arity truth value. Their signatures are
	// identical, but their exact and transparent product components are
	// opposite and must survive Clone/Reset.
	intervalNullary bool
	closed          bool
}

func sparseIntervalKinds(kinds []string) bool {
	if len(kinds) != 2 {
		return false
	}
	seenSparse, seenInterval := false, false
	for _, kind := range kinds {
		switch kind {
		case "tr_sparse":
			if seenSparse {
				return false
			}
			seenSparse = true
		case "interval_relation":
			if seenInterval {
				return false
			}
			seenInterval = true
		default:
			return false
		}
	}
	return seenSparse && seenInterval
}

func sparseIntervalProductColumns(manager *RelationManager, sorts []ast.ASTHandle) (finiteColumns []int, finiteSorts []ast.ASTHandle, domains []uint64, resultErr error) {
	if manager == nil || manager.open() != nil {
		return nil, nil, nil, ErrSparseIntervalProduct
	}
	for column, sort := range sorts {
		if CanHandleIntervalSorts(manager.manager, []ast.ASTHandle{sort}) {
			continue
		}
		domain, err := manager.sortDomain(sort)
		if err != nil {
			return nil, nil, nil, errors.Join(ErrSparseIntervalProduct, err)
		}
		finiteColumns = append(finiteColumns, column)
		finiteSorts = append(finiteSorts, sort)
		domains = append(domains, domain)
	}
	return finiteColumns, finiteSorts, domains, nil
}

// CanSelectSparseIntervalSieveProduct checks the caller-visible concrete
// representation boundary without constructing an owned relation. Generated
// pure and nullary intermediates use NewSparseIntervalSieveProductAuxiliary.
func CanSelectSparseIntervalSieveProduct(manager *RelationManager, sorts []ast.ASTHandle, kinds []string) (bool, error) {
	if !sparseIntervalKinds(kinds) {
		return false, nil
	}
	finiteColumns, _, _, err := sparseIntervalProductColumns(manager, sorts)
	if err != nil {
		return false, err
	}
	return len(finiteColumns) != 0 && len(finiteColumns) != len(sorts), nil
}

func newSparseIntervalSieveProduct(manager *RelationManager, sorts []ast.ASTHandle, full, requireMixed bool) (_ *SparseIntervalSieveProduct, resultErr error) {
	finiteColumns, finiteSorts, domains, err := sparseIntervalProductColumns(manager, sorts)
	if err != nil {
		return nil, err
	}
	if requireMixed && (len(finiteColumns) == 0 || len(finiteColumns) == len(sorts)) {
		return nil, ErrSparseIntervalProduct
	}
	p := &SparseIntervalSieveProduct{
		manager:       manager,
		sorts:         append([]ast.ASTHandle(nil), sorts...),
		finiteColumns: finiteColumns,
		finiteSorts:   finiteSorts,
	}
	switch {
	case len(sorts) != 0 && len(finiteColumns) == 0:
		p.intervalShape = sparseIntervalShapeRaw
	case len(finiteColumns) == len(sorts):
		p.intervalShape = sparseIntervalShapeAbsent
	default:
		p.intervalShape = sparseIntervalShapeSieve
	}
	signature, err := NewSignature(domains)
	if err != nil {
		return nil, err
	}
	p.finite, err = NewSparseTable(signature, 0)
	if err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if resultErr != nil && !published {
			resultErr = errors.Join(resultErr, p.finite.Close())
		}
	}()
	pureFinite := len(finiteColumns) == len(sorts)
	pureArithmetic := len(sorts) != 0 && len(finiteColumns) == 0
	if full || pureArithmetic {
		if err = fillTotalTable(p.finite, signature); err != nil {
			return nil, err
		}
	}
	if full || pureFinite {
		p.interval, err = NewIntervalSieveFull(manager.manager, sorts)
	} else {
		p.interval, err = NewIntervalSieveEmpty(manager.manager, sorts)
	}
	if err != nil {
		return nil, err
	}
	published = true
	return p, nil
}

// NewSparseIntervalSieveProduct is the caller-ready representation-selection
// boundary. full chooses the source mk_full rather than mk_empty components.
func NewSparseIntervalSieveProduct(manager *RelationManager, sorts []ast.ASTHandle, kinds []string, full bool) (_ *SparseIntervalSieveProduct, resultErr error) {
	if !sparseIntervalKinds(kinds) {
		return nil, ErrSparseIntervalProduct
	}
	return newSparseIntervalSieveProduct(manager, sorts, full, true)
}

func (p *SparseIntervalSieveProduct) open() error {
	if p == nil || p.closed || p.manager == nil || p.manager.open() != nil || p.finite == nil || p.finite.open() != nil || p.interval == nil || p.interval.open() != nil {
		return ErrSparseIntervalProduct
	}
	return nil
}

func (p *SparseIntervalSieveProduct) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed = true
	err := p.finite.Close()
	p.manager, p.sorts, p.finiteColumns, p.finiteSorts, p.finite, p.interval = nil, nil, nil, nil, nil, nil
	return err
}

func (p *SparseIntervalSieveProduct) Empty() (bool, error) {
	if err := p.open(); err != nil {
		return true, err
	}
	length, err := p.finite.Len()
	return length == 0 || p.interval.Empty(), err
}

func (p *SparseIntervalSieveProduct) finiteFact(values []ast.ASTHandle) (Fact, error) {
	if err := p.open(); err != nil || len(values) != len(p.sorts) {
		if err != nil {
			return nil, err
		}
		return nil, ErrSparseIntervalProduct
	}
	fact := make(Fact, len(p.finiteColumns))
	for local, outer := range p.finiteColumns {
		valueSort, err := p.manager.manager.GetSort(values[outer])
		if err != nil || valueSort != p.sorts[outer] {
			return nil, errors.Join(ErrSparseIntervalProduct, err)
		}
		fact[local], err = p.manager.EncodeValue(p.sorts[outer], values[outer])
		if err != nil {
			return nil, err
		}
	}
	return fact, nil
}

// AddFact adds the projection to each independent source product component.
func (p *SparseIntervalSieveProduct) AddFact(values []ast.ASTHandle) error {
	fact, err := p.finiteFact(values)
	if err != nil {
		return err
	}
	// Validate and update the interval component before publishing a finite row.
	if err = p.interval.AddFact(values); err != nil {
		return err
	}
	_, err = p.finite.Add(fact)
	return err
}

func (p *SparseIntervalSieveProduct) ContainsFact(values []ast.ASTHandle) (bool, error) {
	fact, err := p.finiteFact(values)
	if err != nil {
		return false, err
	}
	finite, err := p.finite.Contains(fact)
	if err != nil || !finite {
		return false, err
	}
	return p.interval.ContainsFact(values)
}

func (p *SparseIntervalSieveProduct) FiniteColumns() []int {
	if p.open() != nil {
		return nil
	}
	return append([]int(nil), p.finiteColumns...)
}

func (p *SparseIntervalSieveProduct) IntervalComponent() *IntervalSieve {
	if p.open() != nil {
		return nil
	}
	return p.interval
}
