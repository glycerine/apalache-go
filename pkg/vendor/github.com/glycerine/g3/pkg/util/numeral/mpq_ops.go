package numeral

// Source: src/util/mpq.h mixed arithmetic and fused operations.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrRationalNotInteger = errors.New("mpnum: operation requires integer-valued rational operands")

// AddInt ports add(mpq,mpz,mpq), including the source integer fast branch
// and rat_add's single local/cached cross-product temporary.
func (m *RationalManager) AddInt(a *Rational, b *Int, out *Rational) (err error) {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.Z.check(b); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	m.traceMPQMixed(a, b, out, "+", "mpq", false, true)
	defer func() {
		if err == nil {
			m.traceMPQMixed(a, b, out, "+", "mpq", false, false)
		}
	}()
	if isIntegerRational(a) {
		if e := m.Z.Add(&a.Num, b, &out.Num); e != nil {
			return e
		}
		return m.resetDenominator(out)
	} else {
		m.traceMPQMixed(a, b, out, "+", "rat_mpq", false, true)
		defer func() {
			if err == nil {
				m.traceMPQMixed(a, b, out, "+", "rat_mpq", false, false)
			}
		}()
		tmp := &m.AddTmp1
		if m.Synchronized {
			m.addIntPending = true
			tmp = &m.addIntTemporary
		}
		if e := m.Z.Mul(b, &a.Den, tmp); e != nil {
			return e
		}
		if e := m.Z.Copy(&out.Den, &a.Den); e != nil {
			return e
		}
		if e := m.Z.Add(&a.Num, tmp, &out.Num); e != nil {
			return e
		}
		if e := m.normalize(out); e != nil {
			return e
		}
		if m.Synchronized {
			if _, e := sourceNumeralDel(m.Z, tmp); e != nil {
				return e
			}
			m.addIntPending = false
		}
	}
	return nil
}

// CleanupAddIntTemporary completes the owning cleanup of a synchronized
// rat_add scratch value after its source body or explicit del stops.
func (m *RationalManager) CleanupAddIntTemporary() error {
	if m == nil || m.self != m || m.Z == nil {
		return umemory.ErrRuntimeOwner
	}
	if !m.addIntPending {
		return nil
	}
	if e := m.Z.Del(&m.addIntTemporary); e != nil {
		return e
	}
	m.addIntPending = false
	return nil
}

// MulInts implements the inherited integer-product rational-result overload.
func (m *RationalManager) MulInts(a, b *Int, out *Rational) error {
	if e := m.Z.check(a); e != nil {
		return e
	}
	if e := m.Z.check(b); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	if e := m.Z.Mul(a, b, &out.Num); e != nil {
		return e
	}
	return m.resetDenominator(out)
}
func (m *RationalManager) MulInt(a *Int, b, out *Rational) error {
	return m.mulIntWithAllocationContext(a, b, out, integerAllocationInvocationContext{})
}
func (m *RationalManager) mulIntWithAllocationContext(a *Int, b, out *Rational, context integerAllocationInvocationContext) (err error) {
	return m.mulIntWithAllocationContextAndReceipt(a, b, out, context, nil)
}
func (m *RationalManager) mulIntWithAllocationContextAndReceipt(a *Int, b, out *Rational, context integerAllocationInvocationContext, receipt *RationalNormalizeInvocationReceipt) (err error) {
	if e := m.Z.check(a); e != nil {
		return e
	}
	if e := m.check(b); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	m.traceMPQMixed(b, a, out, "*", "mpq", true, true)
	defer func() {
		if err == nil {
			m.traceMPQMixed(b, a, out, "*", "mpq", true, false)
		}
	}()
	integer := isIntegerRational(b)
	if !integer {
		m.traceMPQMixed(b, a, out, "*", "rat_mpq", true, true)
		defer func() {
			if err == nil {
				m.traceMPQMixed(b, a, out, "*", "rat_mpq", true, false)
			}
		}()
	}
	if e := m.Z.mulWithAllocationContext(a, &b.Num, &out.Num, context); e != nil {
		return e
	}
	if integer {
		return m.resetDenominator(out)
	}
	if e := m.Z.copyFromOwnerWithAllocationContext(&out.Den, m.Z, &b.Den, nil, context); e != nil {
		return e
	}
	return m.normalizeWithAllocationContextAndReceipt(out, context, receipt)
}
func (m *RationalManager) DivInt(a *Rational, b *Int, out *Rational) error {
	return m.divIntWithAllocationContext(a, b, out, integerAllocationInvocationContext{})
}
func (m *RationalManager) divIntWithAllocationContext(a *Rational, b *Int, out *Rational, context integerAllocationInvocationContext) (err error) {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.Z.check(b); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	m.traceMPQMixed(a, b, out, "/", "mpq", false, true)
	defer func() {
		if err == nil {
			m.traceMPQMixed(a, b, out, "/", "mpq", false, false)
		}
	}()
	if e := m.Z.copyFromOwnerWithAllocationContext(&out.Num, m.Z, &a.Num, nil, context); e != nil {
		return e
	}
	if e := m.Z.mulWithAllocationContext(&a.Den, b, &out.Den, context); e != nil {
		return e
	}
	if b.Val < 0 {
		if e := m.Z.negWithAllocationContext(&out.Num, context); e != nil {
			return e
		}
		if e := m.Z.negWithAllocationContext(&out.Den, context); e != nil {
			return e
		}
	}
	return m.normalizeWithAllocationContext(out, context)
}

// RatDiv deliberately retains raw denominator sign and actual Copy aliases.
func (m *RationalManager) RatDiv(a, b *Int, out *Rational) error {
	if e := m.Z.check(a); e != nil {
		return e
	}
	if e := m.Z.check(b); e != nil {
		return e
	}
	if e := m.check(out); e != nil {
		return e
	}
	if e := m.Z.Copy(&out.Num, a); e != nil {
		return e
	}
	if e := m.Z.Copy(&out.Den, b); e != nil {
		return e
	}
	return m.normalize(out)
}
func (m *RationalManager) AddMul(a, b, c, out *Rational) error { return m.fused(a, b, c, out, false) }
func (m *RationalManager) SubMul(a, b, c, out *Rational) error { return m.fused(a, b, c, out, true) }
func (m *RationalManager) fused(a, b, c, out *Rational, subtract bool) error {
	return m.fusedWithAllocationContext(a, b, c, out, subtract, integerAllocationInvocationContext{})
}
func (m *RationalManager) fusedWithAllocationContext(a, b, c, out *Rational, subtract bool, context integerAllocationInvocationContext) error {
	for _, q := range []*Rational{a, b, c, out} {
		if e := m.check(q); e != nil {
			return e
		}
	}
	if isIntegerRational(b) && b.Num.IsSmall() && (b.Num.Val == 1 || b.Num.Val == -1) {
		if subtract == (b.Num.Val == 1) {
			return m.addSubWithAllocationContext(a, c, out, true, context)
		}
		return m.addSubWithAllocationContext(a, c, out, false, context)
	}
	tmp := &m.AddMulTmp
	if m.Synchronized {
		m.beginFusedTemporary()
		tmp = &m.fusedTemporary
		defer func() { m.fusedExecuting = false }()
	}
	if e := m.mulWithAllocationContext(b, c, tmp, context); e != nil {
		return e
	}
	if subtract {
		if e := m.addSubWithAllocationContext(a, tmp, out, true, context); e != nil {
			return e
		}
	} else if e := m.addSubWithAllocationContext(a, tmp, out, false, context); e != nil {
		return e
	}
	if m.Synchronized {
		return m.finishFusedTemporarySource()
	}
	return nil
}
func (m *RationalManager) AddMulInt(a *Rational, b *Int, c, out *Rational) error {
	return m.fusedInt(a, b, c, out, false)
}
func (m *RationalManager) SubMulInt(a *Rational, b *Int, c, out *Rational) error {
	return m.fusedInt(a, b, c, out, true)
}
func (m *RationalManager) fusedInt(a *Rational, b *Int, c, out *Rational, subtract bool) error {
	for _, q := range []*Rational{a, c, out} {
		if e := m.check(q); e != nil {
			return e
		}
	}
	if e := m.Z.check(b); e != nil {
		return e
	}
	if b.IsSmall() && (b.Val == 1 || b.Val == -1) {
		if subtract == (b.Val == 1) {
			return m.Sub(a, c, out)
		}
		return m.Add(a, c, out)
	}
	tmp := &m.AddMulTmp
	if m.Synchronized {
		m.beginFusedTemporary()
		tmp = &m.fusedTemporary
		defer func() { m.fusedExecuting = false }()
	}
	if e := m.MulInt(b, c, tmp); e != nil {
		return e
	}
	if subtract {
		if e := m.Sub(a, tmp, out); e != nil {
			return e
		}
	} else if e := m.Add(a, tmp, out); e != nil {
		return e
	}
	if m.Synchronized {
		return m.finishFusedTemporarySource()
	}
	return nil
}

func (m *RationalManager) beginFusedTemporary() {
	m.fusedTemporary = NewRational()
	m.fusedPending = true
	m.fusedExecuting = true
	m.fusedCursor = 0
}

func (m *RationalManager) finishFusedTemporarySource() error {
	for m.fusedCursor < 2 {
		part := &m.fusedTemporary.Num
		if m.fusedCursor == 1 {
			part = &m.fusedTemporary.Den
		}
		if _, e := sourceNumeralDel(m.Z, part); e != nil {
			return e
		}
		m.fusedCursor++
	}
	m.fusedTemporary = Rational{}
	m.fusedPending = false
	return nil
}

func (m *RationalManager) CleanupFusedTemporary() error {
	if m == nil || m.self != m || m.Z == nil {
		return umemory.ErrRuntimeOwner
	}
	if !m.fusedPending {
		return nil
	}
	if m.fusedExecuting {
		return ErrNumericPending
	}
	for m.fusedCursor < 2 {
		part := &m.fusedTemporary.Num
		if m.fusedCursor == 1 {
			part = &m.fusedTemporary.Den
		}
		if e := m.Z.Del(part); e != nil {
			return e
		}
		m.fusedCursor++
	}
	m.fusedTemporary = Rational{}
	m.fusedPending = false
	return nil
}

const (
	qIntegerDiv uint8 = iota
	qIntegerMachineDiv
	qIntegerRem
)

func (m *RationalManager) integerOperandsAt(a, b *Rational, line int32) error {
	if e := m.check(a); e != nil {
		return e
	}
	if e := m.check(b); e != nil {
		return e
	}
	if !modularSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil {
		return e
	}
	if enabled && (!isIntegerRational(a) || !isIntegerRational(b)) {
		expression := "is_int(a) && is_int(b)"
		if line == 610 {
			expression = "is_int(r1) && is_int(r2)"
		}
		return diagnostic.DebugAssertionFailure(d, "src/util/mpq.h", line, expression)
	}
	return nil
}
func (m *RationalManager) integerOpWithAllocationContext(a, b *Rational, out *Int, operation uint8, context integerAllocationInvocationContext) error {
	line := int32(524)
	switch operation {
	case qIntegerMachineDiv:
		line = 513
	case qIntegerRem:
		line = 537
	}
	if e := m.integerOperandsAt(a, b, line); e != nil {
		return e
	}
	// Direct output identity preserves the source base-manager's divisor/output
	// alias quirks for Euclidean division and remainder.
	switch operation {
	case qIntegerDiv:
		return m.Z.divWithAllocationContext(&a.Num, &b.Num, out, context)
	case qIntegerMachineDiv:
		return m.Z.machineDivWithAllocationContext(&a.Num, &b.Num, out, context)
	default:
		return m.Z.remWithAllocationContext(&a.Num, &b.Num, out, context)
	}
}
func (m *RationalManager) integerOpRational(a, b, out *Rational, operation uint8) error {
	return m.integerOpRationalWithAllocationContext(a, b, out, operation, integerAllocationInvocationContext{})
}
func (m *RationalManager) integerOpRationalWithAllocationContext(a, b, out *Rational, operation uint8, context integerAllocationInvocationContext) error {
	if e := m.check(out); e != nil {
		return e
	}
	if e := m.integerOpWithAllocationContext(a, b, &out.Num, operation, context); e != nil {
		return e
	}
	return m.resetDenominator(out)
}
func (m *RationalManager) IDiv(a, b, out *Rational) error {
	return m.integerOpRational(a, b, out, qIntegerDiv)
}
func (m *RationalManager) MachineIDiv(a, b, out *Rational) error {
	return m.integerOpRational(a, b, out, qIntegerMachineDiv)
}
func (m *RationalManager) Rem(a, b, out *Rational) error {
	return m.integerOpRational(a, b, out, qIntegerRem)
}
func (m *RationalManager) Mod(a, b, out *Rational) error {
	return m.modWithAllocationContext(a, b, out, integerAllocationInvocationContext{})
}
func (m *RationalManager) modWithAllocationContext(a, b, out *Rational, context integerAllocationInvocationContext) error {
	if e := m.check(out); e != nil {
		return e
	}
	if e := m.integerOperandsAt(a, b, 550); e != nil {
		return e
	}
	if e := m.Z.modWithAllocationContext(&a.Num, &b.Num, &out.Num, context); e != nil {
		return e
	}
	return m.resetDenominator(out)
}
