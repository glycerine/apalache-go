package numeral

// Source: src/util/mpz.h and src/util/mpz.cpp internal signed division wrappers/core.
// Knuth division translated from src/util/mpn.cpp.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var ErrDivideByZero = errors.New("mpnum: integer division by zero")

const (
	quotientOnly         = 0
	remainderOnly        = 1
	quotientAndRemainder = 2
)

// MachineDivRem truncates toward zero; the remainder has the numerator's sign.
// Either output may alias either input. If q==r the source publication order
// is retained: the general/small path ends in remainder, whereas the
// shorter-numerator limb guard copies remainder then resets quotient to zero.
func (m *IntegerManager) MachineDivRem(a, b, q, r *Int) error {
	return m.machineDivRemWithAllocationContext(a, b, q, r, integerAllocationInvocationContext{})
}
func (m *IntegerManager) machineDivRemWithAllocationContext(a, b, q, r *Int, context integerAllocationInvocationContext) (err error) {
	for _, n := range []*Int{a, b, q, r} {
		if e := m.check(n); e != nil {
			return e
		}
	}
	m.traceMPZDivision(a, b, q, r, "divrem", true)
	defer func() {
		if err == nil {
			m.traceMPZDivision(a, b, q, r, "divrem", false)
		}
	}()
	if b.Val == 0 {
		return ErrDivideByZero
	}
	if a.IsSmall() && b.IsSmall() {
		av, bv := int64(a.Val), int64(b.Val)
		qv, rv := av/bv, av%bv
		// Only INT_MIN/-1 promotes a small quotient. Allocate before publishing.
		var prepared NumericCellRef
		if qv > math.MaxInt32 && q.Cell == 0 {
			var e error
			prepared, e = m.allocateWithSourceContext(6, 0, nil, context)
			if e != nil {
				return e
			}
		}
		if prepared != 0 {
			q.Cell = prepared
			m.PublishCell(prepared)
		}
		if e := m.setInt64WithAllocationContext(q, qv, context); e != nil {
			return e
		}
		return m.setInt64WithAllocationContext(r, rv, context)
	}
	return m.quotRemWithAllocationContext(a, b, q, r, quotientAndRemainder, context)
}
func (m *IntegerManager) MachineDiv(a, b, c *Int) error {
	return m.machineDivWithAllocationContext(a, b, c, integerAllocationInvocationContext{})
}
func (m *IntegerManager) machineDivWithAllocationContext(a, b, c *Int, context integerAllocationInvocationContext) (err error) {
	for _, n := range []*Int{a, b, c} {
		if e := m.check(n); e != nil {
			return e
		}
	}
	m.traceMPZDivision(a, b, c, nil, "machine-div", true)
	defer func() {
		if err == nil {
			m.traceMPZDivision(a, b, c, nil, "machine-div", false)
		}
	}()
	if b.Val == 0 {
		return ErrDivideByZero
	}
	if a.IsSmall() && b.IsSmall() {
		return m.setInt64WithAllocationContext(c, int64(a.Val)/int64(b.Val), context)
	}
	var dummy Int
	if e := m.quotRemWithAllocationContext(a, b, c, &dummy, quotientOnly, context); e != nil {
		return e
	}
	return m.assertDivisionDummyZero(&dummy, 530)
}
func (m *IntegerManager) Rem(a, b, c *Int) error { return m.remWithPNReceipt(a, b, c, nil) }
func (m *IntegerManager) remWithPNReceipt(a, b, c *Int, pn *integerPNormalizeScope) error {
	return m.remWithAllocationContext(a, b, c, pnAllocationContext(pn))
}
func (m *IntegerManager) remWithAllocationContext(a, b, c *Int, context integerAllocationInvocationContext) (err error) {
	for _, n := range []*Int{a, b, c} {
		if e := m.check(n); e != nil {
			return e
		}
	}
	m.traceMPZDivision(a, b, c, nil, "rem", true)
	defer func() {
		if err == nil {
			m.traceMPZDivision(a, b, c, nil, "rem", false)
		}
	}()
	if b.Val == 0 {
		return ErrDivideByZero
	}
	if a.IsSmall() && b.IsSmall() {
		return m.setInt64WithAllocationContext(c, int64(a.Val)%int64(b.Val), context)
	}
	var dummy Int
	if e := m.quotRemWithAllocationContext(a, b, &dummy, c, remainderOnly, context); e != nil {
		return e
	}
	return m.assertDivisionDummyZero(&dummy, 547)
}

func (m *IntegerManager) assertDivisionDummyZero(dummy *Int, line int32) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled {
		return e
	}
	if dummy.Val != 0 {
		return diagnostic.DebugAssertionFailure(d, "src/util/mpz.cpp", line, "is_zero(dummy)")
	}
	return nil
}

// Div implements the source Euclidean quotient adjustment for a negative
// numerator. The source rereads b after output publication; when c==b this
// preserves a known source alias defect (documented in ../PORT_PROGRESS.md#audit-mpnum-port-status-md).
func (m *IntegerManager) Div(a, b, c *Int) error {
	return m.divWithAllocationContext(a, b, c, integerAllocationInvocationContext{})
}
func (m *IntegerManager) divWithAllocationContext(a, b, c *Int, context integerAllocationInvocationContext) (err error) {
	for _, n := range []*Int{a, b, c} {
		if e := m.check(n); e != nil {
			return e
		}
	}
	m.traceMPZDivision(a, b, c, nil, "div", true)
	defer func() {
		if err == nil {
			m.traceMPZDivision(a, b, c, nil, "div", false)
		}
	}()
	if a.Val < 0 {
		var tmp *Int
		var raw *integerScalarRawScope
		if context.Caller != nil && context.Caller.hasScalarRaw() {
			var beginErr error
			raw, beginErr = context.Caller.callScalarRaw(m, "division")
			if beginErr != nil {
				return beginErr
			}
			if raw == nil {
				return ErrNumericOwner
			}
			if e := m.BeginScalarRaw(raw, context.Caller, 1); e != nil {
				return e
			}
			tmp = &raw.Values[0]
			defer func() {
				raw.executing = false
				if err == nil {
					raw.BodySucceeded = true
					err = m.FinishScalarRawSource(raw)
				}
			}()
		} else {
			public := &m.DivisionRaw
			receipt, e := m.beginRawInvocation(public, 1)
			if e != nil {
				return e
			}
			tmp = &public.Values[0]
			context.recordRationalSetRaw(m, receipt)
			context.recordConversionRaw(m, receipt)
			context.recordFormatRaw(m, receipt)
			context.recordRationalServiceRaw(m, receipt)
			context.recordCaller(m, "division", NumericInvocationReceipt{Raw: receipt})
			context = context.withRawInvocation(receipt)
			defer func() {
				public.executing = false
				public.BodyCause = err
				if err == nil {
					public.BodySucceeded = true
					err = m.finishRawSource(public)
				}
			}()
		}
		if e := m.machineDivRemWithAllocationContext(a, b, c, tmp, context); e != nil {
			return e
		}
		if tmp.Val != 0 {
			one := NewInt(1)
			return m.addSubWithAllocationContext(c, &one, c, b.Val >= 0, context)
		}
		return nil
	}
	return m.machineDivWithAllocationContext(a, b, c, context)
}

// Mod implements source nonnegative modulus, including its reread of an
// overwritten divisor when c==b (the source alias defect).
func (m *IntegerManager) Mod(a, b, c *Int) error {
	return m.modWithAllocationContext(a, b, c, integerAllocationInvocationContext{})
}
func (m *IntegerManager) modWithAllocationContext(a, b, c *Int, context integerAllocationInvocationContext) (err error) {
	for _, n := range []*Int{a, b, c} {
		if e := m.check(n); e != nil {
			return e
		}
	}
	m.traceMPZDivision(a, b, c, nil, "mod", true)
	defer func() {
		if err == nil {
			m.traceMPZDivision(a, b, c, nil, "mod", false)
		}
	}()
	if e := m.remWithAllocationContext(a, b, c, context); e != nil {
		return e
	}
	if c.Val < 0 {
		if b.Val > 0 {
			return m.addSubWithAllocationContext(c, b, c, false, context)
		}
		return m.addSubWithAllocationContext(c, b, c, true, context)
	}
	return nil
}

func (m *IntegerManager) normalizedTmpSize(idx int, size uint32) uint32 {
	for size > 0 && m.Digit(m.Tmp[idx], size-1) == 0 {
		size--
	}
	return size
}
func (m *IntegerManager) quotRemCore(a, b, q, r *Int, mode int) error {
	return m.quotRemWithPNReceipt(a, b, q, r, mode, nil)
}
func (m *IntegerManager) quotRemWithPNReceipt(a, b, q, r *Int, mode int, pn *integerPNormalizeScope) error {
	return m.quotRemWithAllocationContext(a, b, q, r, mode, pnAllocationContext(pn))
}
func (m *IntegerManager) quotRemWithAllocationContext(a, b, q, r *Int, mode int, context integerAllocationInvocationContext) error {
	sa, ca, e := m.signCell(a, 0)
	if e != nil {
		return e
	}
	sb, cb, e := m.signCell(b, 1)
	if e != nil {
		return e
	}
	na, nb := m.Cell(ca).Size, m.Cell(cb).Size
	if nb > na {
		if mode != quotientOnly {
			if e := m.copyFromOwnerWithAllocationContext(r, m, a, nil, context); e != nil {
				return e
			}
		}
		if mode != remainderOnly {
			return m.Reset(q)
		}
		return nil
	}
	qsize, rsize := na-nb+1, nb
	if e := m.ensureTmpWithAllocationContext(0, qsize, context); e != nil {
		return e
	}
	if e := m.ensureTmpWithAllocationContext(1, rsize, context); e != nil {
		return e
	}
	if e := m.prepareTmpSpan(0, qsize); e != nil {
		return e
	}
	if e := m.prepareTmpSpan(1, rsize); e != nil {
		return e
	}
	if e := m.divideCellsWithAllocationContext(ca, cb, context); e != nil {
		return e
	}
	sign := int32(1)
	if sa != sb {
		sign = -1
	}
	if mode != remainderOnly {
		if e := m.publishTmpAtWithAllocationContext(0, q, sign, qsize, context); e != nil {
			return e
		}
	}
	if mode != quotientOnly {
		return m.publishTmpAtWithAllocationContext(1, r, sa, rsize, context)
	}
	return nil
}

// divideCells borrows ephemeral indexed views; no arena backing slice escapes.
func (m *IntegerManager) divideCells(a, b NumericCellRef) error {
	return m.divideCellsWithPNReceipt(a, b, nil)
}
func (m *IntegerManager) divideCellsWithPNReceipt(a, b NumericCellRef, pn *integerPNormalizeScope) error {
	return m.divideCellsWithAllocationContext(a, b, pnAllocationContext(pn))
}
func (m *IntegerManager) divideCellsWithAllocationContext(a, b NumericCellRef, context integerAllocationInvocationContext) error {
	ca, cb := m.Cell(a), m.Cell(b)
	cq, cr := m.Cell(m.Tmp[0]), m.Cell(m.Tmp[1])
	_, e := m.Natural.divide(limbView{allocator: m.Allocator, backing: ca.Backing, size: ca.Size}, limbView{allocator: m.Allocator, backing: cb.Backing, size: cb.Size}, limbView{allocator: m.Allocator, backing: cq.Backing, size: cq.span}, limbView{allocator: m.Allocator, backing: cr.Backing, size: cr.span})
	return e
}
