package polynomial

// Source: algebraic_numbers.cpp:467-474,1905-1924. Copy reads source fields as their
// consuming callees are reached, without decoding the entire source cell.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

func (m *AlgebraicRootCore) copySourcePolynomialFields(l *algebraicLocation) (algebraicRecord, error) {
	// The source pointer/count arguments have no claimed relative evaluation
	// order. Neither argument reaches the packed flags or interval payload.
	var w [4]uint32
	for i := range w {
		v, e := m.config.Allocator.ReadUint32(l.raw, uint64(i)*4)
		if e != nil {
			return algebraicRecord{}, e
		}
		w[i] = v
	}
	size, e := m.config.Allocator.ReadUint32(l.raw, 20)
	if e != nil {
		return algebraicRecord{}, e
	}
	return algebraicRecord{poly: UPolynomial{uint64(w[0]) | uint64(w[1])<<32, w[2], w[3]}, size: size}, nil
}
func (m *AlgebraicRootCore) copySourceIntervalFields(l *algebraicLocation) (unumeral.BinaryInterval, error) {
	var w [8]uint32
	for i := range w {
		v, e := m.config.Allocator.ReadUint32(l.raw, 24+uint64(i)*4)
		if e != nil {
			return unumeral.BinaryInterval{}, e
		}
		w[i] = v
	}
	return unumeral.BinaryIntervalFromBorrowedBounds(
		unumeral.BinaryRationalFromBorrowedNumeral(algebraicWordsInt(w[:3]), w[3]),
		unumeral.BinaryRationalFromBorrowedNumeral(algebraicWordsInt(w[4:7]), w[7])), nil
}
func (m *AlgebraicRootCore) copySourceFlags(l *algebraicLocation) (uint32, error) {
	return m.config.Allocator.ReadUint32(l.raw, 16)
}
