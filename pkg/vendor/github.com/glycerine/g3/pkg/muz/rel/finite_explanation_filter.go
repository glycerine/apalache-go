package rel

// Source: src/muz/rel/dl_finite_product_relation.cpp finite-table equality
// and identical-column filter paths.

import "github.com/glycerine/g3/pkg/ast"

// FilterFiniteEqual keeps keys whose selected finite column equals value.
func (p *FiniteExplanationProduct) FilterFiniteEqual(column int, value ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	if column < 0 || column >= len(p.finiteSorts) {
		return ast.ErrASTDomain
	}
	encoded, err := p.manager.EncodeValue(p.finiteSorts[column], value)
	if err != nil {
		return err
	}
	facts, err := p.table.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		if fact[column] != encoded {
			if _, err = p.table.Remove(fact); err != nil {
				return err
			}
		}
	}
	return nil
}

// FilterFiniteIdentical keeps keys with equal values in all selected finite
// columns. Selected columns must have the same finite sort.
func (p *FiniteExplanationProduct) FilterFiniteIdentical(columns []int) error {
	if err := p.open(); err != nil {
		return err
	}
	if len(columns) < 2 {
		return nil
	}
	first := columns[0]
	if first < 0 || first >= len(p.finiteSorts) {
		return ast.ErrASTDomain
	}
	for _, col := range columns[1:] {
		if col < 0 || col >= len(p.finiteSorts) || p.finiteSorts[col] != p.finiteSorts[first] {
			return ast.ErrASTDomain
		}
	}
	facts, err := p.table.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		for _, col := range columns[1:] {
			if fact[col] != fact[first] {
				if _, err = p.table.Remove(fact); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// FilterFiniteNegation removes keys matching any tuple in negative on the
// specified finite columns. Explanation rows do not participate in the key.
func (p *FiniteExplanationProduct) FilterFiniteNegation(negative *FiniteExplanationProduct, targetColumns, negativeColumns []int) error {
	if err := p.open(); err != nil {
		return err
	}
	if err := negative.open(); err != nil {
		return err
	}
	if p.manager != negative.manager || len(targetColumns) != len(negativeColumns) {
		return ast.ErrASTDomain
	}
	for i, col := range targetColumns {
		other := negativeColumns[i]
		if col < 0 || col >= len(p.finiteSorts) || other < 0 || other >= len(negative.finiteSorts) || p.finiteSorts[col] != negative.finiteSorts[other] {
			return ast.ErrASTDomain
		}
	}
	rows, err := p.table.Facts()
	if err != nil {
		return err
	}
	excluded, err := negative.table.Facts()
	if err != nil {
		return err
	}
	for _, row := range rows {
		for _, other := range excluded {
			match := true
			for index, column := range targetColumns {
				if row[column] != other[negativeColumns[index]] {
					match = false
					break
				}
			}
			if match {
				if _, err := p.table.Remove(row); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// filterFiniteEqualEncoded is the finite-index filter used by bytecode after
// literal encoding. Proof ownership and functional indices remain unchanged.
func (p *FiniteExplanationProduct) filterFiniteEqualEncoded(column int, value Element) error {
	if err := p.open(); err != nil {
		return err
	}
	if column < 0 || column >= len(p.finiteSorts) {
		return ast.ErrASTDomain
	}
	signature, err := p.table.Signature()
	if err != nil {
		return err
	}
	domain, err := signature.Domain(column)
	if err != nil {
		return err
	}
	if uint64(value) >= domain {
		return ast.ErrASTDomain
	}
	rows, err := p.table.Facts()
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row[column] != value {
			if _, err = p.table.Remove(row); err != nil {
				return err
			}
		}
	}
	return nil
}
