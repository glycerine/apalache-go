package fp

// Source: dl_mk_bit_blast.cpp:94-112 assigns bits from low to high within
// each original predicate column. C++ rewrites rules without migrating stored
// facts; these concrete tuple conversions preserve the existing Go finite
// relation API when that transformation is activated.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/muz/rel"
)

func datalogBitBlastFactWidth(widths []uint32) (int, error) {
	count := 0
	maxInt := int(^uint(0) >> 1)
	for column, width := range widths {
		// Finite table signatures store the domain cardinality in uint64,
		// so a 64-bit original domain cannot be represented by this path.
		if width == 0 || width > 63 {
			return 0, fmt.Errorf("%w: bit-blasted column %d has unsupported width %d", rel.ErrTable, column, width)
		}
		if count > maxInt-int(width) {
			return 0, fmt.Errorf("%w: bit-blasted tuple arity overflows", rel.ErrTable)
		}
		count += int(width)
	}
	return count, nil
}

// encodeDatalogBitBlastFact expands an all-bit-vector finite tuple into Boolean
// columns. The result owns its slice and does not borrow the input tuple.
func encodeDatalogBitBlastFact(widths []uint32, fact rel.Fact) (rel.Fact, error) {
	count, err := datalogBitBlastFactWidth(widths)
	if err != nil {
		return nil, err
	}
	if len(fact) != len(widths) {
		return nil, fmt.Errorf("%w: bit-vector tuple arity %d, expected %d", rel.ErrTable, len(fact), len(widths))
	}
	for column, width := range widths {
		if uint64(fact[column]) >= uint64(1)<<width {
			return nil, fmt.Errorf("%w: bit-vector tuple column %d exceeds width %d", rel.ErrTable, column, width)
		}
	}
	result := make(rel.Fact, count)
	offset := 0
	for column, width := range widths {
		value := uint64(fact[column])
		for bit := uint32(0); bit < width; bit++ {
			result[offset] = rel.Element((value >> bit) & 1)
			offset++
		}
	}
	return result, nil
}

// decodeDatalogBitBlastFact reconstructs the original finite tuple from its
// Boolean columns, rejecting malformed rows before publishing a result.
func decodeDatalogBitBlastFact(widths []uint32, fact rel.Fact) (rel.Fact, error) {
	count, err := datalogBitBlastFactWidth(widths)
	if err != nil {
		return nil, err
	}
	if len(fact) != count {
		return nil, fmt.Errorf("%w: Boolean tuple arity %d, expected %d", rel.ErrTable, len(fact), count)
	}
	for column, bit := range fact {
		if bit > 1 {
			return nil, fmt.Errorf("%w: bit-blasted tuple column %d is not Boolean", rel.ErrTable, column)
		}
	}
	result := make(rel.Fact, len(widths))
	offset := 0
	for column, width := range widths {
		value := uint64(0)
		for bit := uint32(0); bit < width; bit++ {
			value |= uint64(fact[offset]) << bit
			offset++
		}
		result[column] = rel.Element(value)
	}
	return result, nil
}
