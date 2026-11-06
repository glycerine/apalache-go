package numeral

// Source: src/util/mpf.cpp:23-59,316-333;
// src/util/mpf.h:40-62,88-90,107-114,196-199.

import (
	"github.com/glycerine/g3/pkg/arena"
	uhash "github.com/glycerine/g3/pkg/util/hash"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// ArbitraryFloat is the actual pointer-free MPF numeral. An initialized BIG
// significand belongs to its manager: use Copy, not Go assignment, to own a
// second value. Zero is the literal default constructor. The 15/16/1 bitfield
// widths are packed separately from the full signed 64-bit exponent.

type ArbitraryFloat struct {
	format      uint32
	significand Int
	exponent    int64
}

func (a ArbitraryFloat) EBits() uint32   { return a.format & 0x7fff }
func (a ArbitraryFloat) SBits() uint32   { return (a.format >> 15) & 0xffff }
func (a ArbitraryFloat) Sign() bool      { return a.format>>31 != 0 }
func (a ArbitraryFloat) Exponent() int64 { return a.exponent }

func arbitraryFloatFormat(ebits, sbits uint32, sign bool) uint32 {
	v := ebits&0x7fff | (sbits&0xffff)<<15
	if sign {
		v |= 1 << 31
	}
	return v
}

func (m *ArbitraryFloatManager) valueGuard(a *ArbitraryFloat) error {
	if a == nil {
		return umemory.ErrRuntimeOwner
	}
	return m.valueOwnerGuard()
}

func (m *ArbitraryFloatManager) valueOwnerGuard() error {
	if m == nil || m.self != m || m.integer == nil || m.integer.self != m.integer {
		return umemory.ErrRuntimeOwner
	}
	if !m.constructorComplete || m.closed || m.sourceEnded {
		return arena.ErrClosed
	}
	if m.formattedLiteral.finishing || m.formattedLiteral.executing && m.memory != nil && m.memory.FailureTransportActive() || m.signedFraction.finishing || m.signedFraction.executing && m.memory != nil && m.memory.FailureTransportActive() || m.hexfloatComponents.finishing || m.hexfloatComponents.executing && m.memory != nil && m.memory.FailureTransportActive() || m.binaryString.finishing || m.binaryString.executing && m.memory != nil && m.memory.FailureTransportActive() || m.decimalStream.finishing || m.decimalStream.executing && m.memory != nil && m.memory.FailureTransportActive() || m.integer.decimalStreamExecuting(true) || m.smt2Stream.finishing || m.smt2Stream.executing && m.memory != nil && m.memory.FailureTransportActive() || m.integer.smt2StreamExecuting(true) || m.rationalString.finishing || m.rationalString.executing && m.memory != nil && m.memory.FailureTransportActive() || m.integer.rationalStringExecuting(true) || m.valueExecuting || m.ieeeBV.finishing || m.ieeeBV.executing && m.memory != nil && m.memory.FailureTransportActive() || m.sbv.finishing || m.sbv.executing && m.memory != nil && m.memory.FailureTransportActive() || m.toMPZ.finishing || m.integer.mpfToMPZExecuting(true) || m.rem.finishing || m.rem.executing && m.memory != nil && m.memory.FailureTransportActive() || m.partialRemainder.finishing || m.partialRemainder.executing && m.memory != nil && m.memory.FailureTransportActive() || m.renormalize.finishing || m.renormalize.executing && m.memory != nil && m.memory.FailureTransportActive() || m.sqrt.finishing || m.sqrt.executing && m.memory != nil && m.memory.FailureTransportActive() || m.fma.finishing || m.fma.executing && m.memory != nil && m.memory.FailureTransportActive() || m.div.finishing || m.div.executing && m.memory != nil && m.memory.FailureTransportActive() || m.mul.finishing || m.mul.executing && m.memory != nil && m.memory.FailureTransportActive() || m.addSub.finishing || m.addSub.executing && m.memory != nil && m.memory.FailureTransportActive() || m.integralRound.finishing || m.integralRound.executing && m.memory != nil && m.memory.FailureTransportActive() || m.integerQuery.finishing || m.integer.mpfBorrowedSourceCallbackActive() {
		return ErrNumericPending
	}
	return nil
}

// Hash intentionally omits format and sign, as the source does. hash_ull's
// uint32 result passes through the source implicit mpz(int) constructor before
// the second integer hash; its bits are preserved even above INT_MAX.
func (m *ArbitraryFloatManager) Hash(a *ArbitraryFloat) (uint32, error) {
	if e := m.valueGuard(a); e != nil {
		return 0, e
	}
	h, e := m.integer.Hash(&a.significand)
	if e != nil {
		return 0, e
	}
	v := NewInt(int32(uhash.Uint64(uint64(a.exponent))))
	k, e := m.integer.Hash(&v)
	if e != nil {
		return 0, e
	}
	return uhash.UintPair(h, k), nil
}

// EqCore preserves each source short circuit. A different format or sign does
// not read either significand; exponent comparison follows the integer leaf.
func (m *ArbitraryFloatManager) EqCore(a, b *ArbitraryFloat) (bool, error) {
	if e := m.valueGuard(a); e != nil {
		return false, e
	}
	if b == nil {
		return false, umemory.ErrRuntimeOwner
	}
	if a.EBits() != b.EBits() || a.SBits() != b.SBits() || a.Sign() != b.Sign() {
		return false, nil
	}
	c, e := m.integer.Compare(&a.significand, &b.significand)
	if e != nil {
		return false, e
	}
	return c == 0 && a.exponent == b.exponent, nil
}

func (m *ArbitraryFloatManager) copyValueSignificand(dst, src *Int) error {
	old := m.valueCopy
	e := m.integer.copyFromOwnerWithPhysicalFloatCaller(dst, m.integer, src, &m.valueCopy, integerAllocationInvocationContext{}, nil, &m.valueBinding)
	m.acknowledgeValueCopyReturn(old)
	return e
}

func (m *ArbitraryFloatManager) acknowledgeValueCopyReturn(old integerCopyAllocationReceipt) {
	if m.valueCopy != old {
		m.valueCopyAcknowledged = false
		// The actual leaf can complete without retained roots. Acknowledge its
		// exact returned tuple, rather than later demanding an active frame.
		r := &m.valueCopy
		if r.manager == m.integer && m.integer.validCopyAllocationScope(r.Frame) &&
			r.invocation != 0 && r.Frame.invocation == r.invocation &&
			!r.Frame.Executing && !r.Frame.active {
			m.valueCopyAcknowledged = true
		}
	}
}

// SetComponents translates the mpz-significand overload, which has no powers2
// assertion. Its exponent assignment happens only after successful SET.
func (m *ArbitraryFloatManager) SetComponents(out *ArbitraryFloat, ebits, sbits uint32, sign bool, exponent int64, significand *Int) (cause error) {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	if significand == nil {
		return umemory.ErrRuntimeOwner
	}
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	if e := m.beginValueCopyBinding(nil, m.integer); e != nil {
		return e
	}
	defer func() { m.finishValueCopyBinding(cause) }()
	out.format = arbitraryFloatFormat(ebits, sbits, sign)
	if e := m.copyValueSignificand(&out.significand, significand); e != nil {
		return e
	}
	out.exponent = exponent
	return nil
}

// Copy assigns all scalar fields before the actual significand SET, unlike
// SetComponents. A refused leaf therefore retains this literal source prefix.
func (m *ArbitraryFloatManager) Copy(out, source *ArbitraryFloat) error {
	return m.CopyWithReceipt(out, source, nil)
}

// CopyWithReceipt is the same scalar source SET engine. The opaque child is
// issued only at its actual allocation leaf, as in the physical Runtime route.
func (m *ArbitraryFloatManager) CopyWithReceipt(out, source *ArbitraryFloat, receipt *ArbitraryFloatValueCopyReceipt) error {
	return m.copyWithValueReceipt(out, source, receipt)
}
func (m *ArbitraryFloatManager) copyWithValueReceipt(out, source *ArbitraryFloat, receipt *ArbitraryFloatValueCopyReceipt) error {
	return m.copyWithValueReceiptAndContext(out, source, receipt, integerAllocationInvocationContext{})
}
func (m *ArbitraryFloatManager) copyWithValueReceiptAndContext(out, source *ArbitraryFloat, receipt *ArbitraryFloatValueCopyReceipt, context integerAllocationInvocationContext) error {
	return m.copyWithValueReceiptAndChain(out, source, receipt, context, nil)
}
func (m *ArbitraryFloatManager) copyWithValueReceiptAndChain(out, source *ArbitraryFloat, receipt *ArbitraryFloatValueCopyReceipt, context integerAllocationInvocationContext, chain *ArbitraryFloatValueCopyChain) (cause error) {
	if e := m.valueGuard(out); e != nil {
		return e
	}
	if source == nil {
		return umemory.ErrRuntimeOwner
	}
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	if e := m.beginValueCopyBinding(receipt, m.integer); e != nil {
		return e
	}
	m.valueBinding.conversionContext = context
	m.valueBinding.chain = chain
	defer func() { m.finishValueCopyBinding(cause) }()
	out.format = source.format
	out.exponent = source.exponent
	return m.copyValueSignificand(&out.significand, &source.significand)
}

func (m *ArbitraryFloatManager) Del(a *ArbitraryFloat) error {
	if e := m.valueGuard(a); e != nil {
		return e
	}
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	return m.integer.Del(&a.significand)
}

// CleanupValue is the separate owning NUM phase. It consumes the actual row
// numeral, including a failed source SET's poison/null tag; it does not replay
// Del or acquire authority over an unjoined allocation. The enclosing manager
// acknowledges its exact retained copy child at actual context destruction.
func (m *ArbitraryFloatManager) CleanupValue(a *ArbitraryFloat) error {
	if m == nil || m.self != m || m.integer == nil || m.integer.self != m.integer || a == nil {
		return umemory.ErrRuntimeOwner
	}
	if !m.constructorComplete || m.closed {
		return arena.ErrClosed
	}
	if m.formattedLiteral.finishing || m.formattedLiteral.executing && m.memory != nil && m.memory.FailureTransportActive() || m.signedFraction.finishing || m.signedFraction.executing && m.memory != nil && m.memory.FailureTransportActive() || m.hexfloatComponents.finishing || m.hexfloatComponents.executing && m.memory != nil && m.memory.FailureTransportActive() || m.binaryString.finishing || m.binaryString.executing && m.memory != nil && m.memory.FailureTransportActive() || m.decimalStream.finishing || m.decimalStream.executing && m.memory != nil && m.memory.FailureTransportActive() || m.integer.decimalStreamExecuting(true) || m.smt2Stream.finishing || m.smt2Stream.executing && m.memory != nil && m.memory.FailureTransportActive() || m.integer.smt2StreamExecuting(true) || m.rationalString.finishing || m.rationalString.executing && m.memory != nil && m.memory.FailureTransportActive() || m.integer.rationalStringExecuting(true) || m.valueExecuting || m.ieeeBV.finishing || m.ieeeBV.executing && m.memory != nil && m.memory.FailureTransportActive() || m.sbv.finishing || m.sbv.executing && m.memory != nil && m.memory.FailureTransportActive() || m.toMPZ.finishing || m.integer.mpfToMPZExecuting(true) || m.rem.finishing || m.rem.executing && m.memory != nil && m.memory.FailureTransportActive() || m.partialRemainder.finishing || m.partialRemainder.executing && m.memory != nil && m.memory.FailureTransportActive() || m.renormalize.finishing || m.renormalize.executing && m.memory != nil && m.memory.FailureTransportActive() || m.sqrt.finishing || m.sqrt.executing && m.memory != nil && m.memory.FailureTransportActive() || m.fma.finishing || m.fma.executing && m.memory != nil && m.memory.FailureTransportActive() || m.div.finishing || m.div.executing && m.memory != nil && m.memory.FailureTransportActive() || m.mul.finishing || m.mul.executing && m.memory != nil && m.memory.FailureTransportActive() || m.addSub.finishing || m.addSub.executing && m.memory != nil && m.memory.FailureTransportActive() || m.integralRound.finishing || m.integralRound.executing && m.memory != nil && m.memory.FailureTransportActive() || m.integerQuery.finishing || m.integer.mpfBorrowedSourceCallbackActive() {
		return ErrNumericPending
	}
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	return m.integer.CleanupRawNumeral(&a.significand)
}

// ValueWords captures the actual scalar record without dereferencing its
// significand. Runtime vectors store these six words, never a shadow slice of
// owning numerals. The 24-byte compressed encoding differs from Go/native ABI.
func (m *ArbitraryFloatManager) ValueWords(a *ArbitraryFloat) ([6]uint32, error) {
	if e := m.valueGuard(a); e != nil {
		return [6]uint32{}, e
	}
	return [6]uint32{a.format, uint32(a.exponent), uint32(uint64(a.exponent) >> 32), uint32(a.significand.Val), uint32(a.significand.Cell), uint32(uint64(a.significand.Cell) >> 32)}, nil
}

// CaptureArbitraryFloatWords borrows the encoded record. It performs no cell
// read, allocation, NUM ownership transfer or cloning of a BIG significand.
func CaptureArbitraryFloatWords(w [6]uint32) ArbitraryFloat {
	return ArbitraryFloat{format: w[0], exponent: int64(uint64(w[1]) | uint64(w[2])<<32), significand: Int{Val: int32(w[3]), Cell: NumericCellRef(uint64(w[4]) | uint64(w[5])<<32)}}
}
