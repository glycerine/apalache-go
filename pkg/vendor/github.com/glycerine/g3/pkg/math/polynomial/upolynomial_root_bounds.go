package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1324-1329,432-513.
// Source: src/math/polynomial/upolynomial.cpp:1298-1321.
// Source: src/math/polynomial/upolynomial.cpp:1906-1951 (Cauchy bound),
// src/math/polynomial/upolynomial.cpp:1995-2074 (Knuth/nonzero bounds), related root-bound bodies.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *UPolynomialManager) HasZeroRoots(v UPolynomialView) (bool, error) {
	if e := m.check(); e != nil {
		return false, e
	}
	if e := m.zeroRootsNonemptyAssertion(v.count, true); e != nil {
		return false, e
	}
	a, e := m.read(v, 0)
	if e != nil {
		return false, e
	}
	return m.modular.IsZero(a)
}
func (m *UPolynomialManager) RemoveZeroRoots(v UPolynomialView, out UPolynomial) error {
	d, e := m.descriptor(out)
	if e != nil {
		return e
	}
	return m.removeZeroRootsState(v, &d.state)
}
func (m *UPolynomialManager) removeZeroRootsState(v UPolynomialView, out *upolyVectorState) error {
	if e := m.zeroRootsNonemptyAssertion(v.count, false); e != nil {
		return e
	}
	a, e := m.read(v, 0)
	if e != nil {
		return e
	}
	zero, e := m.modular.IsZero(a)
	if e != nil {
		return e
	}
	if !zero {
		return m.setView(out, v)
	}
	i := uint32(0)
	for {
		enabled, e := refinementSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled && i >= v.count {
			if e := m.scalarDivisionAssertionFailure(1284, "i < sz"); e != nil {
				return e
			}
		}
		a, e = m.read(v, i)
		if e != nil {
			return e
		}
		zero, e = m.modular.IsZero(a)
		if e != nil {
			return e
		}
		if !zero {
			break
		}
		i++
	}
	n := v.count - i
	if e = m.reserve(out, n); e != nil {
		return e
	}
	for j := uint32(0); j < n; j++ {
		a, e = m.read(v, j+i)
		if e != nil {
			return e
		}
		if e = m.modular.Copy(&m.values(out)[j], a); e != nil {
			return e
		}
	}
	return m.setSize(out, n)
}
func (m *UPolynomialManager) HasOneHalfRoot(v UPolynomialView) (bool, error) {
	return m.hasOneHalfRootWithRemove(v, nil)
}
func (m *UPolynomialManager) hasOneHalfRootWithRemove(v UPolynomialView, parent *upolyRemoveOneHalfRootScope) (result bool, cause error) {
	if parent != nil && (!parent.matches() || parent.owner != m || !parent.active || !parent.executing || m.callbackDepth != 0) {
		return false, ErrUPolynomialOwner
	}
	if e := m.check(); e != nil {
		return false, e
	}
	if v.count == 0 {
		return true, nil
	}
	if v.count == 1 {
		return false, nil
	}
	k := uint32(1)
	q, e := m.beginOneHalfRoot()
	if e != nil {
		return false, e
	}
	q.target = v
	if parent != nil {
		if !parent.matches() || parent.owner != m || !parent.active || !parent.executing {
			return false, ErrUPolynomialOwner
		}
		q.removeParent = parent
		q.removeParentInvocation = parent.invocation
		parent.assertion = m.oneHalfRootReceipt()
	}
	z := m.config.Integer
	old, oldN := oneHalfRootCaller(z), oneHalfRootCaller(z.Naturals())
	setOneHalfRootCaller(z, m)
	setOneHalfRootCaller(z.Naturals(), m)
	defer func() {
		q.executing = false
		cause = q.finish(cause)
		if parent != nil && parent.matches() && parent.invocation == q.removeParentInvocation {
			parent.assertion = m.oneHalfRootReceipt()
			parent.updateHistory()
		}
		setOneHalfRootCaller(z, old)
		setOneHalfRootCaller(z.Naturals(), oldN)
	}()
	r, ak := &q.values[0], &q.values[1]
	context := q.context()
	a, e := m.read(v, v.count-1)
	if e != nil {
		return false, e
	}
	if e = numericIntegerModularManager_copyWithOneHalfRootContext(m.modular, r, a, context); e != nil {
		return false, e
	}
	for i := v.count - 1; i > 0; {
		i--
		a, e = m.read(v, i)
		if e != nil {
			return false, e
		}
		if e = numericIntegerModularManager_mul2kWithOneHalfRootContext(m.modular, a, k, ak, context); e != nil {
			return false, e
		}
		if e = numericIntegerModularManager_addWithOneHalfRootContext(m.modular, r, ak, r, context); e != nil {
			return false, e
		}
		k++
	}
	return m.modular.IsZero(r)
}
func (m *UPolynomialManager) RemoveOneHalfRoot(v UPolynomialView, out UPolynomial) (cause error) {
	d, e := m.descriptor(out)
	if e != nil {
		return e
	}
	q, e := m.beginRemoveOneHalfRoot()
	if e != nil {
		return e
	}
	q.target = v
	q.output = out
	z := m.config.Integer
	old, oldN := removeOneHalfRootCaller(z), removeOneHalfRootCaller(z.Naturals())
	setRemoveOneHalfRootCaller(z, m)
	setRemoveOneHalfRootCaller(z.Naturals(), m)
	defer func() {
		q.executing = false
		cause = q.finish(cause)
		setRemoveOneHalfRootCaller(z, old)
		setRemoveOneHalfRootCaller(z.Naturals(), oldN)
	}()
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled {
		yes, e := m.hasOneHalfRootWithRemove(v, q)
		if e != nil {
			return e
		}
		if !yes {
			if e := m.scalarDivisionAssertionFailure(1325, "has_one_half_root(sz, p)"); e != nil {
				return e
			}
		}
	}
	divisor := upolyDivisorSpan{immediate: true, pair: [2]unumeral.Int{unumeral.NewInt(-1), unumeral.NewInt(2)}}
	var power uint32
	if e = m.divRemCoreSpanWithSourceContext(v, &divisor, &power, &m.div2, &m.div1, nil, nil, q.context()); e != nil {
		return e
	}
	if e = m.reset(&m.div1); e != nil {
		return e
	}
	d.state, m.div2 = m.div2, d.state
	return nil
}

func (m *UPolynomialManager) SignChanges(v UPolynomialView) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	var changes uint32
	previous := 0
	for i := uint32(0); i < v.count; i++ {
		sign, e := m.signAt(v, i)
		if e != nil {
			return 0, e
		}
		if sign == 0 {
			continue
		}
		if sign != previous && previous != 0 {
			changes++
		}
		previous = sign
	}
	return changes, nil
}
func (m *UPolynomialManager) DescartesBound(v UPolynomialView) (uint32, error) {
	return m.SignChanges(v)
}
func (m *UPolynomialManager) DescartesBoundZeroOne(v UPolynomialView) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	if v.count <= 1 {
		return 0, nil
	}
	q := &m.db
	if e := m.setView(q, v); e != nil {
		return 0, e
	}
	var changes uint32
	previous := 0
	for i := uint32(0); i < v.count; i++ {
		if e := m.checkpoint(); e != nil {
			return 0, e
		}
		k := uint32(1)
		for ; k < v.count-i; k++ {
			a, e := m.read(m.stateView(q).WithCount(v.count), k)
			if e != nil {
				return 0, e
			}
			prev, e := m.read(m.stateView(q).WithCount(v.count), k-1)
			if e != nil {
				return 0, e
			}
			if e = m.modular.Add(a, prev, a); e != nil {
				return 0, e
			}
		}
		sign, e := m.signAt(m.stateView(q).WithCount(v.count), k-1)
		if e != nil {
			return 0, e
		}
		if sign == 0 {
			continue
		}
		if sign != previous && previous != 0 {
			changes++
			if changes > 1 {
				return changes, nil
			}
		}
		previous = sign
	}
	return changes, nil
}
func (m *UPolynomialManager) DescartesBoundInterval(v UPolynomialView, bqm *unumeral.BinaryRationalManager, a, b *unumeral.BinaryRational) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	if bqm == nil || bqm.Integers() != m.config.Integer {
		return 0, ErrUPolynomialOwner
	}
	nonnegative, e := bqm.IsNonNeg(a)
	if e != nil {
		return 0, e
	}
	if nonnegative {
		aux := &m.dbab1
		if e = m.setView(aux, v); e != nil {
			return 0, e
		}
		if e = m.TranslateBinaryViewInPlace(m.stateView(aux).WithCount(v.count), a); e != nil {
			return 0, e
		}
		var difference unumeral.BinaryRational
		defer bqm.Del(&difference)
		if e = bqm.Sub(b, a, &difference); e != nil {
			return 0, e
		}
		if e = m.ComposeBinaryXViewInPlace(m.stateView(aux), &difference); e != nil {
			return 0, e
		}
		return m.DescartesBoundZeroOne(m.stateView(aux))
	}
	nonpositive, e := bqm.IsNonPos(b)
	if e != nil {
		return 0, e
	}
	if nonpositive {
		aux := &m.dbab2
		if e = m.setView(aux, v); e != nil {
			return 0, e
		}
		if e = m.PMinusXViewInPlace(m.stateView(aux)); e != nil {
			return 0, e
		}
		var mb, ma unumeral.BinaryRational
		defer bqm.Del(&mb)
		defer bqm.Del(&ma)
		if e = bqm.Copy(&mb, b); e != nil {
			return 0, e
		}
		if e = bqm.Neg(&mb); e != nil {
			return 0, e
		}
		if e = bqm.Copy(&ma, a); e != nil {
			return 0, e
		}
		if e = bqm.Neg(&ma); e != nil {
			return 0, e
		}
		return m.DescartesBoundInterval(m.stateView(aux), bqm, &mb, &ma)
	}
	hasZero, e := m.HasZeroRoots(v)
	if e != nil {
		return 0, e
	}
	zero := unumeral.NewBinaryInt(0)
	if !hasZero {
		r1, e := m.DescartesBoundInterval(v, bqm, a, &zero)
		if e != nil {
			return 0, e
		}
		if r1 > 1 {
			return r1, nil
		}
		r2, e := m.DescartesBoundInterval(v, bqm, &zero, b)
		if e != nil {
			return 0, e
		}
		if r1 == 0 {
			return r2, nil
		}
		if r2 == 0 {
			return r1, nil
		}
		return 2, nil
	}
	r1, e := m.DescartesBoundInterval(v, bqm, a, &zero)
	if e != nil {
		return 0, e
	}
	if r1 > 0 {
		return 2, nil
	}
	r2, e := m.DescartesBoundInterval(v, bqm, &zero, b)
	if e != nil {
		return 0, e
	}
	if r2 > 0 {
		return 2, nil
	}
	return 1, nil
}
func (m *UPolynomialManager) RootUpperBound(v UPolynomialView, out *unumeral.Int) (cause error) {
	if e := m.check(); e != nil {
		return e
	}
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && v.count == 0 {
		if e := m.scalarDivisionAssertionFailure(1913, "sz > 0"); e != nil {
			return e
		}
	}
	enabled, e = refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled {
		top, e := m.read(v, v.count-1)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(top)
		if e != nil {
			return e
		}
		if zero {
			if e := m.scalarDivisionAssertionFailure(1914, "!m().is_zero(p[sz - 1])"); e != nil {
				return e
			}
		}
	}
	q, e := m.beginRootUpperBound()
	if e != nil {
		return e
	}
	q.target = v
	context := q.context()
	z := m.config.Integer
	prior := rootUpperBoundCaller(z)
	setRootUpperBoundCaller(z, m)
	priorNatural := rootUpperBoundCaller(z.Naturals())
	setRootUpperBoundCaller(z.Naturals(), m)
	defer func() {
		q.executing = false
		cause = q.finish(cause)
		setRootUpperBoundCaller(z.Naturals(), priorNatural)
		setRootUpperBoundCaller(z, prior)
	}()
	maximum, minimum, an, r2 := &q.values[0], &q.values[1], &q.values[2], &q.values[3]
	top, e := m.read(v, v.count-1)
	if e != nil {
		return e
	}
	if e = numericIntegerModularManager_copyWithRootUpperContext(m.modular, an, top, context); e != nil {
		return e
	}
	if e = numericIntegerModularManager_absWithRootUpperContext(m.modular, an, context); e != nil {
		return e
	}
	q.constructed = 5
	q.sourceRemaining = 5
	c := &q.values[4]
	initialized := false
	for i := uint32(0); i < v.count; i++ {
		a, e := m.read(v, i)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		if e = numericIntegerModularManager_copyWithRootUpperContext(m.modular, c, a, context); e != nil {
			return e
		}
		if e = numericIntegerModularManager_absWithRootUpperContext(m.modular, c, context); e != nil {
			return e
		}
		if !initialized {
			if e = numericIntegerModularManager_copyWithRootUpperContext(m.modular, maximum, c, context); e != nil {
				return e
			}
			if e = numericIntegerModularManager_copyWithRootUpperContext(m.modular, minimum, c, context); e != nil {
				return e
			}
			initialized = true
			continue
		}
		greater, e := m.modular.Greater(c, maximum)
		if e != nil {
			return e
		}
		if greater {
			if e = numericIntegerModularManager_copyWithRootUpperContext(m.modular, maximum, c, context); e != nil {
				return e
			}
		}
		less, e := m.modular.Less(c, minimum)
		if e != nil {
			return e
		}
		if less {
			if e = numericIntegerModularManager_copyWithRootUpperContext(m.modular, minimum, c, context); e != nil {
				return e
			}
		}
	}
	one := unumeral.NewInt(1)
	if e = numericIntegerModularManager_addWithRootUpperContext(m.modular, minimum, maximum, out, context); e != nil {
		return e
	}
	if e = numericIntegerModularManager_divWithRootUpperContext(m.modular, out, minimum, out, context); e != nil {
		return e
	}
	if e = numericIntegerModularManager_addWithRootUpperContext(m.modular, out, &one, out, context); e != nil {
		return e
	}
	if e = numericIntegerModularManager_addWithRootUpperContext(m.modular, an, maximum, r2, context); e != nil {
		return e
	}
	if e = numericIntegerModularManager_divWithRootUpperContext(m.modular, r2, an, r2, context); e != nil {
		return e
	}
	if e = numericIntegerModularManager_addWithRootUpperContext(m.modular, r2, &one, r2, context); e != nil {
		return e
	}
	less, e := m.modular.Less(r2, out)
	if e != nil {
		return e
	}
	if less {
		*out, *r2 = *r2, *out
	}
	return m.rootUpperOrderingAssertion(out, r2)
}

func (m *UPolynomialManager) KnuthPositiveRootUpperBound(v UPolynomialView) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	if v.count == 0 {
		return 0, nil
	}
	n := v.count - 1
	a, e := m.read(v, n)
	if e != nil {
		return 0, e
	}
	positive, e := m.modular.IsPos(a)
	if e != nil {
		return 0, e
	}
	var logarithm uint32
	if positive {
		logarithm, e = m.modular.Log2(a)
	} else {
		logarithm, e = m.modular.MLog2(a)
	}
	if e != nil {
		return 0, e
	}
	maximum := uint32(0)
	for k := uint32(1); k <= n; k++ {
		c, e := m.read(v, n-k)
		if e != nil {
			return 0, e
		}
		zero, e := m.modular.IsZero(c)
		if e != nil {
			return 0, e
		}
		if zero {
			continue
		}
		pos, e := m.modular.IsPos(c)
		if e != nil {
			return 0, e
		}
		if pos == positive {
			continue
		}
		var log uint32
		if pos {
			log, e = m.modular.Log2(c)
		} else {
			log, e = m.modular.MLog2(c)
		}
		if e != nil {
			return 0, e
		}
		if logarithm > log {
			continue
		}
		current := log + 1 - logarithm
		remainder := current % k
		current /= k
		if remainder != 0 {
			current++
		}
		if current > maximum {
			maximum = current
		}
	}
	return maximum + 1, nil
}
func (m *UPolynomialManager) KnuthNegativeRootUpperBound(v UPolynomialView) (uint32, error) {
	return m.knuthNegativeRootUpperBoundWithReceipt(v, nil)
}
func (m *UPolynomialManager) knuthNegativeRootUpperBoundWithReceipt(v UPolynomialView, receipt *upolyKnuthInvocationReceipt) (bound uint32, cause error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	q, e := m.beginKnuth(v, 1)
	if e != nil {
		return 0, e
	}
	if receipt != nil {
		*receipt = m.knuthReceipt()
	}
	z := m.config.Integer
	old, oldN := knuthCaller(z), knuthCaller(z.Naturals())
	setKnuthCaller(z, m)
	setKnuthCaller(z.Naturals(), m)
	defer func() {
		q.executing = false
		cause = q.finish(cause)
		setKnuthCaller(z, old)
		setKnuthCaller(z.Naturals(), oldN)
	}()
	return m.knuthNegativeWithContext(v, q.context())
}
func (m *UPolynomialManager) knuthNegativeWithContext(v UPolynomialView, context integerAllocationInvocationContext) (uint32, error) {
	if q := context.knuthFrame(m.config.Integer); q != nil {
		q.stage = 1
	}
	if e := m.pMinusXWithSourceContext(v, context); e != nil {
		return 0, e
	}
	if q := context.knuthFrame(m.config.Integer); q != nil {
		q.stage = 2
	}
	r, e := m.KnuthPositiveRootUpperBound(v)
	if e != nil {
		return 0, e
	}
	if q := context.knuthFrame(m.config.Integer); q != nil {
		q.stage = 3
	}
	if e = m.pMinusXWithSourceContext(v, context); e != nil {
		return 0, e
	}
	return r, nil
}
func (m *UPolynomialManager) NonzeroRootLowerBound(v UPolynomialView) (uint32, error) {
	return m.nonzeroRootLowerBoundWithReceipt(v, nil)
}
func (m *UPolynomialManager) nonzeroRootLowerBoundWithReceipt(v UPolynomialView, receipt *upolyKnuthInvocationReceipt) (bound uint32, cause error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	q, e := m.beginKnuth(v, 2)
	if e != nil {
		return 0, e
	}
	if receipt != nil {
		*receipt = m.knuthReceipt()
		defer func() { receipt.history = q.history }()
	}
	z := m.config.Integer
	old, oldN := knuthCaller(z), knuthCaller(z.Naturals())
	setKnuthCaller(z, m)
	setKnuthCaller(z.Naturals(), m)
	defer func() {
		q.executing = false
		cause = q.finish(cause)
		setKnuthCaller(z, old)
		setKnuthCaller(z.Naturals(), oldN)
	}()
	return m.nonzeroWithContext(v, q.context())
}
func (m *UPolynomialManager) nonzeroWithContext(v UPolynomialView, context integerAllocationInvocationContext) (uint32, error) {
	if e := m.check(); e != nil {
		return 0, e
	}
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return 0, e
	}
	if enabled && v.count == 0 {
		if e := m.scalarDivisionAssertionFailure(2053, "sz > 0"); e != nil {
			return 0, e
		}
	}
	i := uint32(0)
	for {
		enabled, e := refinementSourceAssertionsEnabled()
		if e != nil {
			return 0, e
		}
		if enabled && i >= v.count {
			if e := m.scalarDivisionAssertionFailure(2057, "i < sz"); e != nil {
				return 0, e
			}
		}
		a, e := m.read(v, i)
		if e != nil {
			return 0, e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return 0, e
		}
		if !zero {
			break
		}
		i++
	}
	enabled, e = refinementSourceAssertionsEnabled()
	if e != nil {
		return 0, e
	}
	if enabled && i >= v.count {
		if e := m.scalarDivisionAssertionFailure(2062, "i < sz"); e != nil {
			return 0, e
		}
	}
	enabled, e = refinementSourceAssertionsEnabled()
	if e != nil {
		return 0, e
	}
	if enabled {
		a, e := m.read(v, i)
		if e != nil {
			return 0, e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return 0, e
		}
		if zero {
			if e := m.scalarDivisionAssertionFailure(2063, "!m().is_zero(p[i])"); e != nil {
				return 0, e
			}
		}
	}
	// The successful indexed read establishes physical representability of start+i.
	nz := v
	nz.start += i
	nz.count -= i
	if q := context.knuthFrame(m.config.Integer); q != nil {
		q.stage = 4
	}
	if e := m.ReciprocalViewInPlace(nz); e != nil {
		return 0, e
	}
	k1, e := m.KnuthPositiveRootUpperBound(nz)
	if e != nil {
		return 0, e
	}
	k2, e := m.knuthNegativeWithContext(nz, context)
	if e != nil {
		return 0, e
	}
	if q := context.knuthFrame(m.config.Integer); q != nil {
		q.stage = 5
	}
	if e = m.ReciprocalViewInPlace(nz); e != nil {
		return 0, e
	}
	if k2 > k1 {
		return k2, nil
	}
	return k1, nil
}
