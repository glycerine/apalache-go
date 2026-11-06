package fp

// Source: dl_compiler.cpp make_add_constant_column/make_full_relation and
// dl_instruction.cpp execution_context register ownership.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// Lookup entries become visible only after initialization. The independent
// owner list also holds a product whose initialization failed, until rollback.
type externalIntervalDatalogConstantRegisters struct {
	unary   map[externalIntervalGroundConstant]*rel.ExternalIntervalProduct
	nullary map[ast.ASTHandle]*rel.ExternalIntervalProduct
	total   map[externalIntervalDatalogTotalKey]*rel.ExternalIntervalProduct
	owned   []*rel.ExternalIntervalProduct
	next    int
}

type externalIntervalDatalogTotalKey struct {
	declaration ast.ASTHandle
	sort        ast.ASTHandle
}

func (c *externalIntervalDatalogConstantRegisters) close() error {
	if c == nil {
		return nil
	}
	for c.next < len(c.owned) {
		if c.owned[c.next] != nil {
			if err := c.owned[c.next].Close(); err != nil {
				return err
			}
			c.owned[c.next] = nil
		}
		c.next++
	}
	c.unary, c.nullary, c.total = nil, nil, nil
	return nil
}

func (r *externalIntervalDatalogRegisters) constantRegisters() *externalIntervalDatalogConstantRegisters {
	if r.constants == nil {
		r.constants = &externalIntervalDatalogConstantRegisters{
			unary:   make(map[externalIntervalGroundConstant]*rel.ExternalIntervalProduct),
			nullary: make(map[ast.ASTHandle]*rel.ExternalIntervalProduct),
			total:   make(map[externalIntervalDatalogTotalKey]*rel.ExternalIntervalProduct),
		}
	}
	return r.constants
}

func (r *externalIntervalDatalogRegisters) totalRelation(head, sort ast.ASTHandle) (*rel.ExternalIntervalProduct, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	entry := r.activation.entries[head]
	if entry == nil {
		return nil, ErrExternalIntervalDatalogFragment
	}
	cache := r.constantRegisters()
	key := externalIntervalDatalogTotalKey{declaration: head, sort: sort}
	if product := cache.total[key]; product != nil {
		return product, nil
	}
	product, selected, err := rel.NewSelectedExternalIntervalTotal(r.activation.plugin, []ast.ASTHandle{sort}, entry.kinds)
	if err != nil {
		return nil, err
	}
	if !selected || product == nil {
		return nil, ErrExternalIntervalDatalogFragment
	}
	cache.owned = append(cache.owned, product)
	cache.total[key] = product
	return product, nil
}

func (r *externalIntervalDatalogRegisters) unarySingleton(head, sort, value ast.ASTHandle) (*rel.ExternalIntervalProduct, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	entry := r.activation.entries[head]
	if entry == nil {
		return nil, ErrExternalIntervalDatalogFragment
	}
	cache := r.constantRegisters()
	key := externalIntervalGroundConstant{sort: sort, value: value}
	if product := cache.unary[key]; product != nil {
		return product, nil
	}
	// The first scheduled head chooses the representation; kinds do not
	// belong to the source (sort, numeral AST) cache key.
	product, selected, err := rel.NewSelectedExternalIntervalProduct(r.activation.plugin, []ast.ASTHandle{sort}, entry.kinds)
	if err != nil {
		return nil, err
	}
	if !selected || product == nil {
		return nil, ErrExternalIntervalDatalogFragment
	}
	cache.owned = append(cache.owned, product)
	if err := product.AddFact([]ast.ASTHandle{value}); err != nil {
		return nil, err
	}
	cache.unary[key] = product
	return product, nil
}

func (r *externalIntervalDatalogRegisters) nullaryTotal(head ast.ASTHandle) (*rel.ExternalIntervalProduct, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	entry := r.activation.entries[head]
	if entry == nil || len(entry.sorts) != 0 {
		return nil, ErrExternalIntervalDatalogFragment
	}
	cache := r.constantRegisters()
	if product := cache.nullary[head]; product != nil {
		return product, nil
	}
	product, selected, err := rel.NewSelectedExternalIntervalNullaryTotal(r.activation.plugin, entry.kinds)
	if err != nil {
		return nil, err
	}
	if !selected || product == nil {
		return nil, ErrExternalIntervalDatalogFragment
	}
	cache.owned = append(cache.owned, product)
	cache.nullary[head] = product
	return product, nil
}
