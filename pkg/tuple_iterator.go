package apalache

import (
	"fmt"
	"math/big"
)

// IntTupleIterator enumerates [0..limit[0]] x ... x [0..limit[n-1]],
// incrementing the first coordinate fastest, exactly like Scala's iterator.
type IntTupleIterator struct {
	limits    []int
	vector    []int
	remaining *big.Int
}

func NewIntTupleIterator(limits []int) *IntTupleIterator {
	copied := append([]int(nil), limits...)
	vector := make([]int, len(copied))
	if len(vector) > 0 {
		vector[0] = -1
	}
	remaining := big.NewInt(1)
	for _, limit := range copied {
		factor := new(big.Int).SetInt64(int64(limit))
		factor.Add(factor, big.NewInt(1))
		remaining.Mul(remaining, factor)
	}
	if remaining.Sign() < 0 {
		remaining.SetInt64(0)
	}
	return &IntTupleIterator{limits: copied, vector: vector, remaining: remaining}
}

func (iterator *IntTupleIterator) HasNext() bool {
	return iterator != nil && iterator.remaining.Sign() > 0
}

func (iterator *IntTupleIterator) Remaining() *big.Int {
	if iterator == nil || iterator.remaining == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(iterator.remaining)
}

func (iterator *IntTupleIterator) Next() ([]int, error) {
	if !iterator.HasNext() {
		return nil, fmt.Errorf("all elements have been enumerated")
	}
	if iterator.remaining.Cmp(big.NewInt(MaxProductSize)) > 0 {
		return nil, fmt.Errorf("too many elements to enumerate: %s", iterator.remaining)
	}
	iterator.remaining.Sub(iterator.remaining, big.NewInt(1))
	index := -1
	for position := range iterator.vector {
		if iterator.vector[position] < iterator.limits[position] {
			index = position
			break
		}
	}
	if index < 0 {
		return nil, fmt.Errorf("tuple iterator has inconsistent limits")
	}
	for position := 0; position < index; position++ {
		iterator.vector[position] = 0
	}
	iterator.vector[index]++
	return append([]int(nil), iterator.vector...), nil
}

type Pair[A, B any] struct {
	First  A
	Second B
}

type Product2Iterator[A, B any] struct {
	left    []A
	right   []B
	indices *IntTupleIterator
}

func NewProduct2Iterator[A, B any](left []A, right []B) *Product2Iterator[A, B] {
	return &Product2Iterator[A, B]{
		left: append([]A(nil), left...), right: append([]B(nil), right...),
		indices: NewIntTupleIterator([]int{len(left) - 1, len(right) - 1}),
	}
}

func (iterator *Product2Iterator[A, B]) HasNext() bool {
	return iterator != nil && iterator.indices.HasNext()
}

func (iterator *Product2Iterator[A, B]) Next() (Pair[A, B], error) {
	var zero Pair[A, B]
	if iterator == nil {
		return zero, fmt.Errorf("product iterator is nil")
	}
	indices, err := iterator.indices.Next()
	if err != nil {
		return zero, err
	}
	return Pair[A, B]{First: iterator.left[indices[0]], Second: iterator.right[indices[1]]}, nil
}
