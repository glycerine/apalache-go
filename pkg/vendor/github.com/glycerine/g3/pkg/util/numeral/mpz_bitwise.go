package numeral

// Source: src/util/mpz.cpp bitwise_or/and/xor/not, _MP_INTERNAL branch.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// BitwiseOperation identifies the source binary chunk loop.
type BitwiseOperation uint8

const (
	BitwiseOrOperation BitwiseOperation = iota
	BitwiseAndOperation
	BitwiseXorOperation
)

var ErrNonterminatingBitwise = errors.New("mpnum: source bitwise quotient continuation does not terminate")

type BitwiseNonterminationError struct{ Operation BitwiseOperation }

func (e *BitwiseNonterminationError) Error() string { return ErrNonterminatingBitwise.Error() }
func (e *BitwiseNonterminationError) Unwrap() error { return ErrNonterminatingBitwise }

func bitwiseSourceNonnegative(a *Int, line int32, expression string) error {
	if !modularSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled || a.Val >= 0 {
		return e
	}
	return diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", line, expression)
}

func (m *IntegerManager) BitwiseOr(a, b, out *Int) error  { return m.bitwiseBinary(a, b, out, 0) }
func (m *IntegerManager) BitwiseAnd(a, b, out *Int) error { return m.bitwiseBinary(a, b, out, 1) }
func (m *IntegerManager) BitwiseXor(a, b, out *Int) error { return m.bitwiseBinary(a, b, out, 2) }

// bitwiseBinary preserves the source arithmetic chunk algorithm, including its
// final multiplier/division actions. Work is owner-budgeted source temporaries;
// errors retain completed caller publications, including aliased operands.
func (m *IntegerManager) bitwiseBinary(a, b, out *Int, op uint8) error {
	return m.bitwiseBinaryWithAllocationContext(a, b, out, op, integerAllocationInvocationContext{})
}

// Source: src/util/mpz.cpp:1018-1148.
func (m *IntegerManager) bitwiseBinaryWithAllocationContext(a, b, out *Int, op uint8, context integerAllocationInvocationContext) (err error) {
	for _, v := range []*Int{a, b, out} {
		if e := m.check(v); e != nil {
			return e
		}
	}
	first, second := int32(1019), int32(1020)
	if op == 1 {
		first, second = 1069, 1070
	} else if op == 2 {
		first, second = 1107, 1108
	}
	if e := bitwiseSourceNonnegative(a, first, "is_nonneg(a)"); e != nil {
		return e
	}
	if e := bitwiseSourceNonnegative(b, second, "is_nonneg(b)"); e != nil {
		return e
	}
	if op == 0 {
		m.traceBitwiseOrStart(a, b)
	}
	if a.IsSmall() && b.IsSmall() {
		var v int32
		switch op {
		case 0:
			v = a.Val | b.Val
		case 1:
			v = a.Val & b.Val
		case 2:
			v = a.Val ^ b.Val
		}
		if op == 2 {
			return m.setInt64WithAllocationContext(out, int64(v), context)
		}
		if e := m.Del(out); e != nil {
			return e
		}
		out.Val = v
		return nil
	}
	raw, name := &m.BitwiseOrRaw, "bitwise_or"
	switch op {
	case 1:
		raw, name = &m.BitwiseAndRaw, "bitwise_and"
	case 2:
		raw, name = &m.BitwiseXorRaw, "bitwise_xor"
	}
	receipt, e := m.beginRawInvocation(raw, 6)
	if e != nil {
		return e
	}
	context.recordRationalServiceRaw(m, receipt)
	context.recordCaller(m, name, NumericInvocationReceipt{Raw: receipt})
	context = context.withRawInvocation(receipt)
	a1, b1, a2, b2, mult, tmp := &raw.Values[0], &raw.Values[1], &raw.Values[2], &raw.Values[3], &raw.Values[4], &raw.Values[5]
	defer func() {
		raw.executing = false
		raw.BodyCause = err
		if err == nil {
			raw.BodySucceeded = true
			err = m.finishRawSource(raw)
		}
	}()
	if e := m.copyFromOwnerWithAllocationContext(a1, m, a, nil, context); e != nil {
		return e
	}
	if e := m.copyFromOwnerWithAllocationContext(b1, m, b, nil, context); e != nil {
		return e
	}
	if e := m.setInt64WithAllocationContext(mult, 1, context); e != nil {
		return e
	}
	if e := m.Reset(out); e != nil {
		return e
	}
	for a1.Val != 0 && b1.Val != 0 {
		// Both quotients remain -1 forever. Stop only at this actual next
		// continuation, after all preceding source actions and their failures.
		if a1.IsSmall() && b1.IsSmall() && a1.Val == -1 && b1.Val == -1 {
			return &BitwiseNonterminationError{Operation: BitwiseOperation(op)}
		}
		if op == 0 {
			m.traceBitwiseOrOperands(a1, b1)
		}
		if e := m.modWithAllocationContext(a1, &m.Two64, a2, context); e != nil {
			return e
		}
		if e := m.modWithAllocationContext(b1, &m.Two64, b2, context); e != nil {
			return e
		}
		if op == 0 {
			m.traceBitwiseOrChunks(a2, b2)
		}
		u, e := m.Uint64(a2)
		if e != nil {
			return e
		}
		v, e := m.Uint64(b2)
		if e != nil {
			return e
		}
		bUnsigned := v
		switch op {
		case 0:
			v = u | v
		case 1:
			v = u & v
		case 2:
			v = u ^ v
		}
		if op == 0 {
			m.traceBitwiseOrUnsigned(u, bUnsigned)
		}
		if e = m.setUint64WithAllocationContext(tmp, v, context); e != nil {
			return e
		}
		if e = m.mulWithAllocationContext(tmp, mult, tmp, context); e != nil {
			return e
		}
		if e = m.addSubWithAllocationContext(out, tmp, out, false, context); e != nil {
			return e
		}
		if e = m.mulWithAllocationContext(mult, &m.Two64, mult, context); e != nil {
			return e
		}
		if e = m.divWithAllocationContext(a1, &m.Two64, a1, context); e != nil {
			return e
		}
		if e = m.divWithAllocationContext(b1, &m.Two64, b1, context); e != nil {
			return e
		}
	}
	if op != 1 {
		if a1.Val != 0 {
			if e := m.mulWithAllocationContext(a1, mult, a1, context); e != nil {
				return e
			}
			if e := m.addSubWithAllocationContext(out, a1, out, false, context); e != nil {
				return e
			}
		}
		if b1.Val != 0 {
			if e := m.mulWithAllocationContext(b1, mult, b1, context); e != nil {
				return e
			}
			if e := m.addSubWithAllocationContext(out, b1, out, false, context); e != nil {
				return e
			}
		}
	}
	return nil
}

// BitwiseNot complements within the specified source unsigned bit width.
// The source small path uses a signed shift at width63; unsigned mask arithmetic
// defines its intended low63 bits without signed overflow. The general path
// retains all 64-bit source chunk actions, including the final unused multiplier.
func (m *IntegerManager) BitwiseNot(width uint32, a, out *Int) error {
	return m.bitwiseNotWithAllocationContext(width, a, out, integerAllocationInvocationContext{})
}
func (m *IntegerManager) bitwiseNotWithAllocationContext(width uint32, a, out *Int, context integerAllocationInvocationContext) (err error) {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	if e := bitwiseSourceNonnegative(a, 1152, "is_nonneg(a)"); e != nil {
		return e
	}
	if a.IsSmall() && width <= 63 {
		mask := (uint64(1) << width) - 1
		return m.setInt64WithAllocationContext(out, int64(^uint64(a.Val)&mask), context)
	}
	raw := &m.BitwiseNotRaw
	receipt, e := m.beginRawInvocation(raw, 4)
	if e != nil {
		return e
	}
	context.recordRationalServiceRaw(m, receipt)
	context.recordCaller(m, "bitwise_not", NumericInvocationReceipt{Raw: receipt})
	context = context.withRawInvocation(receipt)
	a1, a2, mult, tmp := &raw.Values[0], &raw.Values[1], &raw.Values[2], &raw.Values[3]
	defer func() {
		raw.executing = false
		raw.BodyCause = err
		if err == nil {
			raw.BodySucceeded = true
			err = m.finishRawSource(raw)
			if err == nil {
				m.traceBitwiseNotFinal(width, a, out)
			}
		}
	}()
	if e := m.copyFromOwnerWithAllocationContext(a1, m, a, nil, context); e != nil {
		return e
	}
	if e := m.setInt64WithAllocationContext(mult, 1, context); e != nil {
		return e
	}
	if e := m.setInt64WithAllocationContext(out, 0, context); e != nil {
		return e
	}
	for width > 0 {
		if e := m.modWithAllocationContext(a1, &m.Two64, a2, context); e != nil {
			return e
		}
		n, e := m.Uint64(a2)
		if e != nil {
			return e
		}
		v := ^n
		if modularSourceDebug {
			d := diagnostic.BorrowProcessDebugContext()
			enabled, e := diagnostic.SourceAssertionsEnabled(d)
			if e != nil {
				return e
			}
			if enabled && ^v != n {
				if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1166, "~v == n"); e != nil {
					return e
				}
			}
		}
		if width < 64 {
			v &= (uint64(1) << width) - 1
		}
		m.traceBitwiseNotChunk(width, v, n)
		if e = m.setUint64WithAllocationContext(tmp, v, context); e != nil {
			return e
		}
		if modularSourceDebug {
			d := diagnostic.BorrowProcessDebugContext()
			enabled, e := diagnostic.SourceAssertionsEnabled(d)
			if e != nil {
				return e
			}
			if enabled {
				got, e := m.Uint64(tmp)
				if e != nil {
					return e
				}
				if got != v {
					if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1173, "get_uint64(tmp) == v"); e != nil {
						return e
					}
				}
			}
		}
		if e = m.mulWithAllocationContext(tmp, mult, tmp, context); e != nil {
			return e
		}
		if e = m.addSubWithAllocationContext(out, tmp, out, false, context); e != nil {
			return e
		}
		if e = m.mulWithAllocationContext(mult, &m.Two64, mult, context); e != nil {
			return e
		}
		if e = m.divWithAllocationContext(a1, &m.Two64, a1, context); e != nil {
			return e
		}
		if width < 64 {
			width = 0
		} else {
			width -= 64
		}
	}
	return nil
}
