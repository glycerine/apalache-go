package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp create_basis.

import "math"

var pb2BVSortingPrimes = [...]uint64{2, 3, 5, 7, 11, 13, 17}

type pb2BVSortingBasis struct {
	current []uint64
	best    []uint64
	cost    uint64
}

func (b *pb2BVSortingBasis) search(sequence []uint64, carry, cost uint64) {
	if cost >= b.cost {
		return
	}
	delta := uint64(0)
	for _, coefficient := range sequence {
		if math.MaxUint64-delta < coefficient {
			return
		}
		delta += coefficient
	}
	if cost <= math.MaxUint64-delta && cost+delta < b.cost {
		b.cost = cost + delta
		b.best = append(b.best[:0], b.current...)
		b.best = append(b.best, delta+1)
	}
	for _, prime := range pb2BVSortingPrimes {
		next := make([]uint64, 0, len(sequence))
		rest := carry
		for _, coefficient := range sequence {
			if math.MaxUint64-rest < coefficient%prime {
				return
			}
			rest += coefficient % prime
			if coefficient >= prime {
				next = append(next, coefficient/prime)
			}
		}
		if math.MaxUint64-cost < rest {
			continue
		}
		b.current = append(b.current, prime)
		b.search(next, rest/prime, cost+rest)
		b.current = b.current[:len(b.current)-1]
	}
}

func pb2BVCreateSortingBasis(coefficients []uint32) ([]uint32, bool) {
	sequence := make([]uint64, len(coefficients))
	for i, coefficient := range coefficients {
		sequence[i] = uint64(coefficient)
	}
	b := pb2BVSortingBasis{cost: math.MaxInt32}
	b.search(sequence, 0, 0)
	if len(b.best) == 0 || b.best[len(b.best)-1] > uint64(20*len(b.best)) {
		return nil, false
	}
	result := make([]uint32, len(b.best))
	for i, base := range b.best {
		if base > math.MaxUint32 {
			return nil, false
		}
		result[i] = uint32(base)
	}
	return result, true
}
