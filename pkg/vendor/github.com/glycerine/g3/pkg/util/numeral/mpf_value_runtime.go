package numeral

// Source: src/util/mpf.cpp:316-333; src/util/mpf.h:88-90;
// src/util/mpz.cpp:big_set; src/util/vector.h:423-435.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func (m *ArbitraryFloatManager) captureRuntimeValue(block umemory.Block, offset uint64, own bool) (ArbitraryFloat, error) {
	var zero ArbitraryFloat
	if own {
		if m == nil || m.self != m || m.integer == nil || m.integer.self != m.integer {
			return zero, umemory.ErrRuntimeOwner
		}
		if !m.constructorComplete || m.closed {
			return zero, arena.ErrClosed
		}
		if m.valueExecuting {
			return zero, ErrNumericPending
		}
	} else if e := m.valueGuard(&zero); e != nil {
		return zero, e
	}
	extent, e := m.memory.Size(block)
	if e != nil {
		return zero, e
	}
	if offset > extent || extent-offset < 24 {
		return zero, arena.ErrBounds
	}
	var words [6]uint32
	for i := range words {
		words[i], e = m.memory.ReadUint32(block, offset+uint64(i)*4)
		if e != nil {
			return zero, e
		}
	}
	return CaptureArbitraryFloatWords(words), nil
}

// CaptureRuntimeValue is a borrowed scalar read of sole Runtime row bytes; it
// does not dereference the encoded cell or create another owning numeral.
func (m *ArbitraryFloatManager) CaptureRuntimeValue(block umemory.Block, offset uint64) (ArbitraryFloat, error) {
	return m.captureRuntimeValue(block, offset, false)
}

func (m *ArbitraryFloatManager) writeRuntimeExponent(block umemory.Block, offset uint64, exponent int64) error {
	if e := m.memory.WriteUint32(block, offset+4, uint32(exponent)); e != nil {
		return e
	}
	return m.memory.WriteUint32(block, offset+8, uint32(uint64(exponent)>>32))
}

func (m *ArbitraryFloatManager) physicalCopySignificand(out, source *Int, receiver *integerRuntimeValueReceiver) error {
	return m.physicalCopySignificandFromOwner(out, m.integer, source, receiver)
}
func (m *ArbitraryFloatManager) physicalCopySignificandFromOwner(out *Int, sourceManager *IntegerManager, source *Int, receiver *integerRuntimeValueReceiver) error {
	old := m.valueCopy
	e := m.integer.copyFromOwnerWithPhysicalFloatCaller(out, sourceManager, source, &m.valueCopy, integerAllocationInvocationContext{}, receiver, &m.valueBinding)
	m.acknowledgeValueCopyReturn(old)
	return e
}

// CopyToRuntime publishes format/exponent before SET. The optional concrete
// receiver publishes sign, poison/null and returned cell at the existing
// Integer leaf sites, before real allocation callbacks can observe the row.
func (m *ArbitraryFloatManager) CopyToRuntime(block umemory.Block, offset uint64, source *ArbitraryFloat) error {
	return m.CopyToRuntimeWithReceipt(block, offset, source, nil)
}
func (m *ArbitraryFloatManager) CopyToRuntimeWithReceipt(block umemory.Block, offset uint64, source *ArbitraryFloat, receipt *ArbitraryFloatValueCopyReceipt) error {
	return m.CopyToRuntimeFromOwnerWithReceipt(block, offset, m, source, receipt)
}

// The source MPF is borrowed from an explicitly named owner. Its compressed
// significand is read by that owner's Integer leaf, never inferred from IDs.
func (m *ArbitraryFloatManager) CopyToRuntimeFromOwnerWithReceipt(block umemory.Block, offset uint64, sourceManager *ArbitraryFloatManager, source *ArbitraryFloat, receipt *ArbitraryFloatValueCopyReceipt) (cause error) {
	out, e := m.captureRuntimeValue(block, offset, false)
	if e != nil {
		return e
	}
	if sourceManager == nil || source == nil {
		return umemory.ErrRuntimeOwner
	}
	if e := sourceManager.valueGuard(source); e != nil {
		return e
	}
	if e := sourceManager.integer.check(&source.significand); e != nil {
		return e
	}
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	if e := m.beginValueCopyBinding(receipt, sourceManager.integer); e != nil {
		return e
	}
	defer func() { m.finishValueCopyBinding(cause) }()
	if e = m.memory.WriteUint32(block, offset, source.format); e != nil {
		return e
	}
	if e = m.writeRuntimeExponent(block, offset, source.exponent); e != nil {
		return e
	}
	receiver := integerRuntimeValueReceiver{runtime: m.memory, block: block, offset: offset + 12}
	return m.physicalCopySignificandFromOwner(&out.significand, sourceManager.integer, &source.significand, &receiver)
}

// SetComponentsToRuntime uses the source mpz overload's opposite exponent
// ordering: write format, SET the actual row's significand, then exponent.
func (m *ArbitraryFloatManager) SetComponentsToRuntime(block umemory.Block, offset uint64, ebits, sbits uint32, sign bool, exponent int64, significand *Int) error {
	return m.SetComponentsToRuntimeWithReceipt(block, offset, ebits, sbits, sign, exponent, significand, nil)
}
func (m *ArbitraryFloatManager) SetComponentsToRuntimeWithReceipt(block umemory.Block, offset uint64, ebits, sbits uint32, sign bool, exponent int64, significand *Int, receipt *ArbitraryFloatValueCopyReceipt) error {
	if m == nil {
		return umemory.ErrRuntimeOwner
	}
	return m.SetComponentsToRuntimeFromOwnerWithReceipt(block, offset, ebits, sbits, sign, exponent, m.integer, significand, receipt)
}
func (m *ArbitraryFloatManager) SetComponentsToRuntimeFromOwnerWithReceipt(block umemory.Block, offset uint64, ebits, sbits uint32, sign bool, exponent int64, sourceManager *IntegerManager, significand *Int, receipt *ArbitraryFloatValueCopyReceipt) (cause error) {
	out, e := m.captureRuntimeValue(block, offset, false)
	if e != nil {
		return e
	}
	if sourceManager == nil || significand == nil {
		return umemory.ErrRuntimeOwner
	}
	if e := sourceManager.check(significand); e != nil {
		return e
	}
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	if e := m.beginValueCopyBinding(receipt, sourceManager); e != nil {
		return e
	}
	defer func() { m.finishValueCopyBinding(cause) }()
	if e = m.memory.WriteUint32(block, offset, arbitraryFloatFormat(ebits, sbits, sign)); e != nil {
		return e
	}
	receiver := integerRuntimeValueReceiver{runtime: m.memory, block: block, offset: offset + 12}
	if e = m.physicalCopySignificandFromOwner(&out.significand, sourceManager, significand, &receiver); e != nil {
		return e
	}
	return m.writeRuntimeExponent(block, offset, exponent)
}

// Deletion has no allocating publication gap. Always write its actual returned
// NUM prefix, including a consumed release that also returns an owning error.
func (m *ArbitraryFloatManager) delRuntimeValue(block umemory.Block, offset uint64, own bool) error {
	out, e := m.captureRuntimeValue(block, offset, own)
	if e != nil {
		return e
	}
	if own {
		e = m.CleanupValue(&out)
	} else {
		e = m.Del(&out)
	}
	r := integerRuntimeValueReceiver{runtime: m.memory, block: block, offset: offset + 12}
	if pub := r.publishValue(&out.significand); pub != nil {
		if e != nil {
			return e
		}
		return pub
	}
	if pub := r.publishCell(&out.significand); pub != nil {
		if e != nil {
			return e
		}
		return pub
	}
	return e
}

func (m *ArbitraryFloatManager) DelRuntimeValue(block umemory.Block, offset uint64) error {
	return m.delRuntimeValue(block, offset, false)
}
func (m *ArbitraryFloatManager) CleanupRuntimeValue(block umemory.Block, offset uint64) error {
	return m.delRuntimeValue(block, offset, true)
}
