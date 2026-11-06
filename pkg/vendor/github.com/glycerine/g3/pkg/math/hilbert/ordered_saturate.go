package hilbert

import "github.com/glycerine/g3/pkg/util/lbool"

// saturateOrdered ports hilbert_basis::saturate, the optimized path selected
// by the public source Saturate operation.
func (b *Basis) saturateOrdered(row []int64, equal bool) (lbool.LBool, error) {
	index := &orderedIndex{owner: b}
	zero := make([]*vector, 0)
	support := make([]*vector, 0)
	positive := make([]*vector, 0)
	for _, value := range b.basis {
		var err error
		value.weight, err = weight(value, row)
		if err != nil {
			return lbool.Undef, err
		}
		value.weights = make([]int64, b.current)
		for previous := 0; previous < b.current; previous++ {
			value.weights[previous], err = weight(value, b.inequalities[previous].values)
			if err != nil {
				return lbool.Undef, err
			}
		}
		index.insert(value)
		switch sign(value.weight) {
		case 0:
			zero = append(zero, value)
		case 1:
			positive = append(positive, value)
			support = append(support, value)
		case -1:
			support = append(support, value)
		}
	}
	passive := newOrderedPassive(b, support)
	if len(positive) != 0 {
		for _, value := range support {
			if value.weight < 0 {
				if err := passive.insert(value, 0); err != nil {
					return lbool.Undef, err
				}
			}
		}
	}
	for !passive.empty() {
		if !b.proceed() {
			return lbool.Undef, nil
		}
		sos, pas, offset, err := passive.pop()
		if err != nil {
			return lbool.Undef, err
		}
		resolved, err := b.resolveOrdered(sos, pas)
		if err != nil {
			return lbool.Undef, err
		}
		if index.subsumes(resolved) {
			continue
		}
		index.insert(resolved)
		if resolved.weight == 0 {
			zero = append(zero, resolved)
			continue
		}
		if !b.orderedSupport {
			offset = 0
		}
		if err := passive.insert(resolved, offset); err != nil {
			return lbool.Undef, err
		}
		if resolved.weight > 0 {
			positive = append(positive, resolved)
		}
	}
	if equal {
		positive = positive[:0]
	}
	b.basis = append(positive, zero...)
	if err := orderBasis(b.basis); err != nil {
		return lbool.Undef, err
	}
	if len(b.basis) == 0 {
		return lbool.False, nil
	}
	return lbool.True, nil
}
