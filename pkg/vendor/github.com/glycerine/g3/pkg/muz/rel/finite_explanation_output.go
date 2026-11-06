package rel

// Source: dl_finite_product_relation.cpp can_convert_to_table_relation,
// to_table_relation (211-229), and converting_union_fn (1088-1114). The
// explanation specialization is registered by dl_mk_explanations.cpp:617.

import "errors"

// CanConvertToTableRelation reports whether all explanation columns have been
// projected away, leaving only the finite table signature.
func (p *FiniteExplanationProduct) CanConvertToTableRelation() (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	return p.explanationColumns == 0, nil
}

// collectEmptyExplanationRows implements the source conversion's
// garbage_collect(true). Removed keys must not retain obsolete inner owners.
func (p *FiniteExplanationProduct) collectEmptyExplanationRows() error {
	rows, err := p.table.Facts()
	if err != nil {
		return err
	}
	live := make([]bool, len(p.inner))
	anyLive := false
	for _, row := range rows {
		index := uint64(row[len(row)-1])
		if index >= uint64(len(p.inner)) || p.inner[index] == nil {
			return ErrTableRelation
		}
		empty, err := p.inner[index].Empty()
		if err != nil {
			return err
		}
		if empty {
			if _, err = p.table.Remove(row); err != nil {
				return err
			}
		} else {
			live[index] = true
			anyLive = true
		}
	}
	for i, inner := range p.inner {
		if inner != nil && !live[i] {
			if err = inner.Close(); err != nil {
				return err
			}
			p.inner[i] = nil
		}
	}
	if !anyLive {
		p.inner = nil
	}
	return nil
}

// ToTableRelation copies projected finite tuples into an independently owned
// table after the source empty-inner collection and functional-index removal.
func (p *FiniteExplanationProduct) ToTableRelation() (result *TableRelation, resultErr error) {
	converted, err := p.ToFiniteTable()
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, converted.Close())
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	result, err = NewTableRelation(p.manager, p.finiteSorts)
	if err != nil {
		return nil, err
	}
	rows, err := converted.Facts()
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		if _, err = result.AddTableFact(row); err != nil {
			return result, err
		}
	}
	return result, nil
}

// UnionIntoTable is the concrete converting_union_fn for a finite table
// destination and optional table delta. Existing delta tuples are preserved.
func (p *FiniteExplanationProduct) UnionIntoTable(target, delta *TableRelation) (resultErr error) {
	converted, err := p.ToTableRelation()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, converted.Close()) }()
	return target.UnionFrom(converted, delta)
}
