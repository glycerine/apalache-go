package numeral

// Source: src/util/mpz.cpp power/is_power_of_two/ensure_capacity/normalize/
// machine_div2k/mul2k (_MP_INTERNAL 32-bit digits) and mpz.h copy wrappers.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var ErrNonterminatingPower = errors.New("mpnum: source power mask would not terminate")

type PowerDomainReason uint8

const (
	PowerMaskOverflow PowerDomainReason = 2
)

// PowerDomainError observes the actual mask-wrap boundary.
// Completed arithmetic/publication before that boundary remains visible.
type PowerDomainError struct {
	Exponent uint32
	Reason   PowerDomainReason
}

func (e *PowerDomainError) Error() string { return ErrNonterminatingPower.Error() }
func (e *PowerDomainError) Unwrap() error { return ErrNonterminatingPower }

// ensureOwnedCapacity translates ensure_capacity, including requested capacity
// six and INT_MIN promotion. Old ownership is released after replacement exists.
func (m *IntegerManager) EnsureOwnedCapacity(a *Int, capacity uint32) error {
	return m.ensureOwnedCapacityWithAllocationContext(a, capacity, integerAllocationInvocationContext{})
}
func (m *IntegerManager) ensureOwnedCapacityWithAllocationContext(a *Int, capacity uint32, context integerAllocationInvocationContext) error {
	if capacity <= 1 {
		return nil
	}
	if capacity < 6 {
		capacity = 6
	}
	if !a.IsSmall() && m.Cell(a.Cell).Capacity >= capacity {
		return nil
	}
	old := a.Cell
	size := uint32(1)
	if old != 0 {
		size = m.Cell(old).Size
	}
	ref, e := m.allocateWithSourceContext(capacity, size, nil, context)
	if e != nil {
		return e
	}
	if old == 0 {
		a.Cell = ref
		m.PublishCell(ref)
	} // source small pointer publication precedes the assertion and digit writes
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled && m.Cell(ref).Capacity != capacity {
			line := int32(1563)
			expression := "a.m_ptr->m_capacity == capacity"
			if old != 0 {
				line = 1586
				expression = "new_cell->m_capacity == capacity"
			}
			if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", line, expression); e != nil {
				return e
			}
		}
	}
	sign := a.Val
	if old != 0 {
		m.ResizeCell(ref, size)
	}
	if old != 0 {
		for i := uint32(0); i < size; i++ {
			m.writeDigit(ref, i, m.Digit(old, i))
		}
	} else {
		if a.Val == math.MinInt32 {
			m.writeDigit(ref, 0, m.Digit(m.IntMin.Cell, 0))
			sign = -1
		} else if a.Val < 0 {
			m.writeDigit(ref, 0, uint32(-a.Val))
			sign = -1
		} else {
			m.writeDigit(ref, 0, uint32(a.Val))
			sign = 1
		}
	}
	if old != 0 {
		if e := m.free(old); e != nil {
			return e
		}
	}
	if old != 0 {
		a.Cell = ref
		m.PublishCell(ref)
	}
	a.Val = sign
	if old == 0 {
		m.ResizeCell(ref, size)
	}
	return nil
}
func (m *IntegerManager) normalizeOwned(a *Int) error {
	size := m.Cell(a.Cell).Size
	for size > 0 && m.Digit(a.Cell, size-1) == 0 {
		size--
	}
	if size == 0 {
		return m.Reset(a)
	}
	if size == 1 && m.Digit(a.Cell, 0) <= math.MaxInt32 {
		v := int32(m.Digit(a.Cell, 0))
		if a.Val < 0 {
			v = -v
		}
		if e := m.Del(a); e != nil {
			return e
		}
		a.Val = v
		return nil
	}
	m.ResizeCell(a.Cell, size)
	return nil
}
func (m *IntegerManager) Power(a *Int, p uint32, out *Int) error {
	return m.powerWithReceipt(a, p, out, nil)
}
func (m *IntegerManager) powerWithReceipt(a *Int, p uint32, out *Int, receipt *integerPowerInvocationReceipt) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	return m.powerCoreWithContext(a, p, out, receipt, integerAllocationInvocationContext{})
}
func (m *IntegerManager) powerCore(a *Int, p uint32, out *Int) error {
	return m.powerCoreWithReceipt(a, p, out, nil)
}
func (m *IntegerManager) powerCoreWithReceipt(a *Int, p uint32, out *Int, receipt *integerPowerInvocationReceipt) error {
	return m.powerCoreWithContext(a, p, out, receipt, integerAllocationInvocationContext{})
}
func (m *IntegerManager) powerWithAllocationContext(a *Int, p uint32, out *Int, receipt *integerPowerInvocationReceipt, context integerAllocationInvocationContext) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	return m.powerCoreWithContext(a, p, out, receipt, context)
}
func (m *IntegerManager) powerCoreWithContext(a *Int, p uint32, out *Int, receipt *integerPowerInvocationReceipt, context integerAllocationInvocationContext) (cause error) {
	return m.powerCoreWithPhysicalContext(a, p, out, receipt, context, nil)
}
func (m *IntegerManager) powerWithPhysicalAllocationContext(a *Int, p uint32, out *Int, receipt *integerPowerInvocationReceipt, context integerAllocationInvocationContext, receiver *integerRuntimeValueReceiver) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	if e := receiver.validate(m, out); e != nil {
		return e
	}
	// This physical leaf represents the reached MPF powers-of-two specialization.
	if !a.IsSmall() || a.Val != 2 {
		return arena.ErrInvalidSlot
	}
	return m.powerCoreWithPhysicalContext(a, p, out, receipt, context, receiver)
}
func (m *IntegerManager) powerCoreWithPhysicalContext(a *Int, p uint32, out *Int, receipt *integerPowerInvocationReceipt, context integerAllocationInvocationContext, receiver *integerRuntimeValueReceiver) (cause error) {
	// A validated literal caller binding can precede its first allocating leaf.
	// This grants no roots; the allocation leaf prepares its actual record.
	rationalSetBinding := context.mpfRationalSet != nil && context.rationalSetFrame(m, true) != nil && context == context.mpfRationalSet.allocationContext()
	if context != (integerAllocationInvocationContext{}) && context.roots(m) == nil && !rationalSetBinding {
		return ErrNumericCallerOwner
	}
	if a.IsSmall() {
		switch a.Val {
		case 2:
			if p < 31 {
				e := m.Del(out)
				if write := receiver.publishCell(out); write != nil {
					return write
				}
				if e != nil {
					return e
				}
				out.Val = int32(uint32(1) << p)
				return receiver.publishValue(out)
			}
			size := p/32 + 1
			shift := p % 32
			if modularSourceDebug {
				d := diagnostic.BorrowProcessDebugContext()
				enabled, e := diagnostic.SourceAssertionsEnabled(d)
				if e != nil {
					return e
				}
				if enabled && size == 0 {
					if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1471, "sz > 0"); e != nil {
						return e
					}
				}
			}
			if e := m.allocateIfNeededWithPhysicalAllocationContext(out, size, context, receiver); e != nil {
				return e
			}
			if modularSourceDebug {
				d := diagnostic.BorrowProcessDebugContext()
				enabled, e := diagnostic.SourceAssertionsEnabled(d)
				if e != nil {
					return e
				}
				if enabled && m.Cell(out.Cell).Capacity < size {
					if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1473, "b.m_ptr->m_capacity >= sz"); e != nil {
						return e
					}
				}
			}
			// The physical Go span is guarded at its actual access, after
			// all source allocation and sign-publication actions.
			if size > m.Cell(out.Cell).Capacity {
				return arena.ErrBounds
			}
			m.ResizeCell(out.Cell, size)
			for i := uint32(0); i < size-1; i++ {
				m.writeDigit(out.Cell, i, 0)
			}
			// Explicit unsigned limb shift defines the intended bit at source signed 1<<31.
			m.writeDigit(out.Cell, size-1, uint32(1)<<shift)
			out.Val = 1
			return receiver.publishValue(out)
		case 0:
			if modularSourceDebug {
				d := diagnostic.BorrowProcessDebugContext()
				enabled, e := diagnostic.SourceAssertionsEnabled(d)
				if e != nil {
					return e
				}
				if enabled && p == 0 {
					if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1483, "p != 0"); e != nil {
						return e
					}
				}
			}
			return m.Reset(out)
		case 1:
			return m.setInt64WithAllocationContext(out, 1, context)
		}
	}
	if e := m.beginPowerRawScopeWithReceipt(receipt); e != nil {
		return e
	}
	s := &m.PowerRaw
	defer func() {
		if cause != nil && !s.endEntered && s.bodyCause == nil {
			s.bodyCause = cause
		}
	}()
	power := &s.Value
	if e := m.powerRawCallWithContext(0, a, nil, power, context); e != nil {
		return e
	}
	if e := m.setInt64WithAllocationContext(out, 1, context); e != nil {
		return e
	}
	for mask := uint32(1); mask <= p; mask <<= 1 {
		if mask == 0 {
			return &PowerDomainError{Exponent: p, Reason: PowerMaskOverflow}
		}
		if mask&p != 0 {
			if e := m.mulWithAllocationContext(out, power, out, context); e != nil {
				return e
			}
		}
		// The final square is deliberately retained, even when its value is unused.
		if e := m.powerRawCallWithContext(1, power, power, power, context); e != nil {
			return e
		}
	}
	return m.endPowerRawScope()
}
func (m *IntegerManager) IsPowerOfTwo(a *Int) (bool, uint32, error) {
	if e := m.check(a); e != nil {
		return false, 0, e
	}
	if a.Val <= 0 {
		return false, 0, nil
	}
	if a.IsSmall() {
		v := uint32(a.Val)
		if v&(v-1) != 0 {
			return false, 0, nil
		}
		return true, log2Uint32(v), nil
	}
	size := m.Cell(a.Cell).Size
	for i := uint32(0); i < size-1; i++ {
		if m.Digit(a.Cell, i) != 0 {
			return false, 0, nil
		}
	}
	v := m.Digit(a.Cell, size-1)
	if v == 0 || v&(v-1) != 0 {
		return false, 0, nil
	}
	k, e := m.Log2(a)
	return e == nil, k, e
}

func (m *IntegerManager) MachineDiv2k(a *Int, k uint32, out *Int) error {
	return m.shift(a, k, out, false)
}
func (m *IntegerManager) Mul2k(a *Int, k uint32, out *Int) error { return m.shift(a, k, out, true) }
func (m *IntegerManager) MachineDiv2kInPlace(a *Int, k uint32) error {
	if e := m.check(a); e != nil {
		return e
	}
	return m.machineDiv2kCore(a, k)
}
func (m *IntegerManager) Mul2kInPlace(a *Int, k uint32) error {
	if e := m.check(a); e != nil {
		return e
	}
	return m.mul2kCore(a, k)
}
func (m *IntegerManager) shift(a *Int, k uint32, out *Int, left bool) error {
	return m.shiftWithAllocationContext(a, k, out, left, integerAllocationInvocationContext{})
}
func (m *IntegerManager) shiftWithAllocationContext(a *Int, k uint32, out *Int, left bool, context integerAllocationInvocationContext) error {
	return m.shiftWithAllocationContextAndReceipt(a, k, out, left, context, nil)
}
func (m *IntegerManager) shiftWithAllocationContextAndReceipt(a *Int, k uint32, out *Int, left bool, context integerAllocationInvocationContext, receipt *IntegerCopyAllocationReceipt) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	// An explicit copy receipt keeps its own allocation authority; later
	// shift growth belongs to the supplied numerical caller context.
	copyContext := context
	if receipt != nil {
		copyContext = integerAllocationInvocationContext{}
	}
	if e := m.copyFromOwnerWithAllocationContext(out, m, a, receipt, copyContext); e != nil {
		return e
	}
	if left {
		return m.mul2kCoreWithAllocationContext(out, k, context)
	}
	return m.machineDiv2kCore(out, k)
}
func (m *IntegerManager) machineDiv2kCore(a *Int, k uint32) error {
	if k == 0 || a.Val == 0 {
		return nil
	}
	if a.IsSmall() {
		if k < 32 {
			twok := int32(uint32(1) << k)
			a.Val /= twok
		} else {
			a.Val = 0
		}
		return nil
	}
	digitShift := k / 32
	size := m.Cell(a.Cell).Size
	if digitShift >= size {
		return m.Reset(a)
	}
	bitShift := k % 32
	compShift := 32 - bitShift
	newSize := size - digitShift
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled && newSize < 1 {
			if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1649, "new_sz >= 1"); e != nil {
				return e
			}
		}
	}
	m.traceMachineDiv2k(bitShift, compShift, newSize, size)
	if newSize < size {
		i, j := uint32(0), digitShift
		if bitShift != 0 {
			for ; i < newSize-1; i, j = i+1, j+1 {
				v := m.Digit(a.Cell, j)>>bitShift | m.Digit(a.Cell, j+1)<<compShift
				m.writeDigit(a.Cell, i, v)
			}
			m.writeDigit(a.Cell, i, m.Digit(a.Cell, j)>>bitShift)
		} else {
			for ; i < newSize; i, j = i+1, j+1 {
				m.writeDigit(a.Cell, i, m.Digit(a.Cell, j))
			}
		}
	} else {
		if modularSourceDebug {
			d := diagnostic.BorrowProcessDebugContext()
			enabled, e := diagnostic.SourceAssertionsEnabled(d)
			if e != nil {
				return e
			}
			if enabled && newSize != size {
				if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1671, "new_sz == sz"); e != nil {
					return e
				}
			}
			enabled, e = diagnostic.SourceAssertionsEnabled(d)
			if e != nil {
				return e
			}
			if enabled && bitShift == 0 {
				if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1672, "bit_shift != 0"); e != nil {
					return e
				}
			}
		}
		var i uint32
		for ; i < newSize-1; i++ {
			v := m.Digit(a.Cell, i)>>bitShift | m.Digit(a.Cell, i+1)<<compShift
			m.writeDigit(a.Cell, i, v)
		}
		m.writeDigit(a.Cell, i, m.Digit(a.Cell, i)>>bitShift)
	}
	m.ResizeCell(a.Cell, newSize)
	return m.normalizeOwned(a)
}
func (m *IntegerManager) mul2kCore(a *Int, k uint32) error {
	return m.mul2kCoreWithAllocationContext(a, k, integerAllocationInvocationContext{})
}
func (m *IntegerManager) mul2kCoreWithAllocationContext(a *Int, k uint32, context integerAllocationInvocationContext) error {
	if k == 0 || a.Val == 0 {
		return nil
	}
	if a.IsSmall() && k < 32 {
		return m.setInt64WithAllocationContext(a, int64(a.Val)*(int64(1)<<k), context)
	}
	m.traceMul2kStart(a, k)
	wordShift, bitShift := k/32, k%32
	oldSize := uint32(1)
	if !a.IsSmall() {
		oldSize = m.Cell(a.Cell).Size
	}
	newSize := oldSize + wordShift + 1 // source unsigned32 addition
	if e := m.ensureOwnedCapacityWithAllocationContext(a, newSize, context); e != nil {
		return e
	}
	m.traceMul2kCapacity(a, wordShift, bitShift, oldSize, newSize)
	if modularSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled && a.IsSmall() {
			if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1711, "!is_small(a)"); e != nil {
				return e
			}
		}
	}
	// Guard the actual encoded backing access after source capacity preparation.
	if a.IsSmall() {
		return arena.ErrBounds
	}
	oldSize = m.Cell(a.Cell).Size
	cell := m.Cell(a.Cell)
	// Prepare only the physical digit access span. Retain the source cell size
	// through zero filling; publish newSize at the actual source statement.
	span := newSize
	if span > cell.Capacity {
		span = cell.Capacity
	}
	// Physical object extent is checked at the actual borrowed span.
	if uint64(span)*4+8 > cell.Backing.extent {
		return arena.ErrBounds
	}
	cell.span = span
	m.Store(a.Cell, cell)
	for i := oldSize; i < newSize; i++ {
		if i >= cell.Capacity || i >= span {
			return arena.ErrBounds
		}
		m.writeDigit(a.Cell, i, 0)
	}
	m.ResizeCell(a.Cell, newSize)
	if wordShift > 0 {
		j, i := oldSize, oldSize+wordShift
		for j > 0 {
			j--
			i--
			if i >= cell.Capacity || i >= span || j >= cell.Capacity || j >= span {
				return arena.ErrBounds
			}
			m.writeDigit(a.Cell, i, m.Digit(a.Cell, j))
		}
		for i > 0 {
			i--
			if i >= cell.Capacity || i >= span {
				return arena.ErrBounds
			}
			m.writeDigit(a.Cell, i, 0)
		}
	}
	if bitShift > 0 {
		if modularSourceDebug {
			d := diagnostic.BorrowProcessDebugContext()
			for i := uint32(0); i < wordShift; i++ {
				enabled, e := diagnostic.SourceAssertionsEnabled(d)
				if e != nil {
					return e
				}
				if enabled {
					if i >= cell.Capacity || i >= span {
						return arena.ErrBounds
					}
					if m.Digit(a.Cell, i) != 0 {
						if e = diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", 1734, "ds[i] == 0"); e != nil {
							return e
						}
					}
				}
			}
		}
		compShift := 32 - bitShift
		var previous uint32
		for i := wordShift; i < newSize; i++ {
			if i >= cell.Capacity || i >= span {
				return arena.ErrBounds
			}
			v := m.Digit(a.Cell, i)
			next := v >> compShift
			m.writeDigit(a.Cell, i, v<<bitShift|previous)
			previous = next
		}
	}
	if e := m.normalizeOwned(a); e != nil {
		return e
	}
	m.traceMul2kResult(a)
	return nil
}
