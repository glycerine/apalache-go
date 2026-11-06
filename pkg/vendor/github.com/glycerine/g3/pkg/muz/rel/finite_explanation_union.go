package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp union_fn, specialized to
// the fact-level explanation relation.

import "github.com/glycerine/g3/pkg/ast"

func (p *FiniteExplanationProduct) compatible(other *FiniteExplanationProduct) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if p.manager != other.manager || p.explanationColumns != other.explanationColumns || len(p.finiteSorts) != len(other.finiteSorts) {
		return ast.ErrASTDomain
	}
	for i := range p.finiteSorts {
		if p.finiteSorts[i] != other.finiteSorts[i] {
			return ast.ErrASTDomain
		}
	}
	return nil
}

// Source explanation_relation_plugin::union_fn requires every participating
// explanation row to be defined, including rows that an overlap would leave
// unchanged. Check all reachable rows before any finite keys are published.
func (p *FiniteExplanationProduct) requireDefinedExplanationRows() error {
	rows, err := p.table.Facts()
	if err != nil {
		return err
	}
	for _, row := range rows {
		index := uint64(row[len(row)-1])
		if index >= uint64(len(p.inner)) || p.inner[index] == nil {
			return ErrTableRelation
		}
		defined, err := p.inner[index].NoUndefined()
		if err != nil {
			return err
		}
		if !defined {
			return ast.ErrASTDomain
		}
	}
	return nil
}

// Union adds source tuples that are absent from the target, preserving the
// target's first explanation for overlapping tuples. Delta, when supplied,
// accumulates newly added tuples and preserves its existing explanations.
func (p *FiniteExplanationProduct) Union(source, delta *FiniteExplanationProduct) error {
	if err := p.compatible(source); err != nil {
		return err
	}
	if delta != nil {
		if err := p.compatible(delta); err != nil {
			return err
		}
		if delta == p || delta == source {
			return ast.ErrASTDomain
		}
	}
	for _, product := range []*FiniteExplanationProduct{p, source, delta} {
		if product != nil {
			if err := product.requireDefinedExplanationRows(); err != nil {
				return err
			}
		}
	}
	facts, err := source.table.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		sourceIndex := uint64(fact[len(fact)-1])
		if sourceIndex >= uint64(len(source.inner)) || source.inner[sourceIndex] == nil {
			return ErrTableRelation
		}
		data, e := source.inner[sourceIndex].Data()
		if e != nil {
			return e
		}
		if len(data) != p.explanationColumns {
			return ErrTableRelation
		}
		key := append(Fact(nil), fact...)
		found, e := p.fetchInnerKey(key)
		if e != nil {
			return e
		}
		if found {
			continue
		}
		if e = p.addEncodedRow(key, data); e != nil {
			return e
		}
		if delta != nil {
			deltaKey := append(Fact(nil), fact...)
			deltaFound, e := delta.fetchInnerKey(deltaKey)
			if e != nil {
				return e
			}
			if !deltaFound {
				if e = delta.addEncodedRow(deltaKey, data); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
