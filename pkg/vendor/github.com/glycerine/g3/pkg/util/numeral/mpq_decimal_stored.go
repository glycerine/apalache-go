package numeral

// Source: mpq.cpp:156-185, reached by algebraic_numbers.cpp:2733.

import umemory "github.com/glycerine/g3/pkg/util/memory"

// DisplayDecimalStoredInto borrows the concrete stored mpq representation:
// two project numeral records of three uint32 words, NUM followed by DEN.
// Reads occur at the actual get_numerator/get_denominator/is_neg statements.
// The view owns no header, numerator, denominator or numerical cleanup.
func (m *RationalManager) DisplayDecimalStoredInto(out RationalSourceStream, allocator *umemory.SmallAllocator, object umemory.ObjectRef, precision uint32, truncate bool) (cause error) {
	return m.DisplayDecimalStoredIntoWithInvocation(out, allocator, object, precision, truncate, nil)
}
func (m *RationalManager) DisplayDecimalStoredIntoWithInvocation(out RationalSourceStream, allocator *umemory.SmallAllocator, object umemory.ObjectRef, precision uint32, truncate bool, receipt *RationalDecimalInvocationReceipt) (cause error) {
	if e := m.checkOwnerReady(); e != nil {
		return e
	}
	if allocator == nil {
		return umemory.ErrAllocatorOwner
	}
	s, e := m.beginDecimalStreamWithInvocation(out, receipt)
	if e != nil {
		return e
	}
	defer func() { cause = s.finishCall(cause) }()
	z, c := m.Z, integerAllocationInvocationContext{Caller: &s.hooks}
	n, e := decimalStoredInteger(allocator, object, 0)
	if e != nil {
		return e
	}
	if e = z.copyFromOwnerWithAllocationContext(&s.values[1], z, &n, nil, c); e != nil {
		return e
	}
	d, e := decimalStoredInteger(allocator, object, 12)
	if e != nil {
		return e
	}
	if e = z.copyFromOwnerWithAllocationContext(&s.values[2], z, &d, nil, c); e != nil {
		return e
	}
	sign, e := allocator.ReadUint32(object, 0)
	if e != nil {
		return e
	}
	return s.runCopied(out, int32(sign) < 0, precision, truncate)
}
func decimalStoredInteger(allocator *umemory.SmallAllocator, object umemory.ObjectRef, offset uint64) (Int, error) {
	var words [3]uint32
	for i := range words {
		w, e := allocator.ReadUint32(object, offset+4*uint64(i))
		if e != nil {
			return Int{}, e
		}
		words[i] = w
	}
	return Int{Val: int32(words[0]), Cell: NumericCellRef(uint64(words[1]) | uint64(words[2])<<32)}, nil
}
