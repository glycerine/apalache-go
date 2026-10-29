package polynomial

// Source: src/math/polynomial/upolynomial.cpp:663-826;

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"io"
)

// modularImage implements the lexical scoped_set_zp body. The old modulus
// copy goes through mpzzp Copy, including source normalization. In modular
// GCD the entering mode is always Z. Cleanup on Go failure is explicit safe
// ownership adaptation; a restoration failure stays visible.

func (m *UPolynomialManager) modularImage(u, v, q *upolyVectorState, p, lc *unumeral.Int, du, dv uint32) (bool, error) {
	return m.modularImageWithSourceContext(u, v, q, p, lc, du, dv, integerAllocationInvocationContext{})
}
func (m *UPolynomialManager) modularImageWithSourceContext(u, v, q *upolyVectorState, p, lc *unumeral.Int, du, dv uint32, context integerAllocationInvocationContext) (bad bool, err error) {
	oldMode, e := m.modular.Modular()
	if e != nil {
		return false, e
	}
	var localOld, localC unumeral.Int
	old, c := &localOld, &localC
	sf := context.squareFreeCallFrame(m.config.Integer)
	var image *upolySquareFreeImageScope
	var frame *upolyFactorImageScope
	if sf != nil {
		image = &sf.image
		if image.savedConstructed || image.classConstructed || image.cConstructed {
			return false, ErrUPolynomialCleanupPending
		}
		*image = upolySquareFreeImageScope{oldMode: oldMode, savedConstructed: true}
		old, c = &image.old, &image.c
		defer func() { err = image.finish(sf, err, false) }()
	} else if owner := context.factorFrontendFrame(m.config.Integer); owner != nil {
		frame = &owner.image
		if frame.savedConstructed || frame.classConstructed || frame.cConstructed || frame.invocation == ^uint64(0) {
			return false, ErrUPolynomialCleanupPending
		}
		*frame = upolyFactorImageScope{owner: owner, invocation: frame.invocation + 1, oldMode: oldMode, savedConstructed: true, executing: true}
		old, c = &frame.old, &frame.c
		defer func() { frame.executing = false; err = frame.finish(err) }()
	} else {
		defer m.modular.Del(old)
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(old, &(*m.modular.BorrowModulus())); e != nil {
		return false, e
	}
	if frame == nil && image == nil {
		defer func() {
			var restore error
			if oldMode {
				restore = m.modular.SetZP(old)
			} else {
				restore = m.modular.SetZ()
			}
			if restore != nil {
				err = restore
			}
		}()
	}

	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).SetZP(p); e != nil {
		return false, e
	}
	if frame != nil {
		frame.classConstructed = true
	} else if image != nil {
		image.classConstructed = true
	}
	U, V := &m.mgcd[2], &m.mgcd[3]
	if e = m.setViewWithSourceContext(U, m.stateView(u), context); e != nil {
		return false, e
	}
	if e = m.setViewWithSourceContext(V, m.stateView(v), context); e != nil {
		return false, e
	}
	ud, vd := uint32(0), uint32(0)
	if U.size != 0 {
		ud = U.size - 1
	}
	if V.size != 0 {
		vd = V.size - 1
	}
	if ud < du {
		return true, m.traceModGCDLiteral(758, "bad prime, leading coefficient vanished\n")
	}
	if vd < dv {
		return true, m.traceModGCDLiteral(762, "bad prime, leading coefficient vanished\n")
	}
	if e = m.euclidGCDWithSourceContext(m.stateView(U), m.stateView(V), q, context); e != nil {
		return false, e
	}
	if e = m.monicLocalWithSourceContext(m.stateView(q), context); e != nil {
		return false, e
	}
	if frame != nil {
		frame.cConstructed = true
	} else if image != nil {
		image.cConstructed = true
	} else {
		defer m.modular.Del(c)
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(c, lc); e != nil {
		return false, e
	}
	return false, m.mulScalarViewWithSourceContext(m.stateView(q), c, context)
}
func (m *UPolynomialManager) combineImages(c1 *upolyVectorState, b1 *unumeral.Int, c2 *upolyVectorState, b2 *unumeral.Int) error {
	return m.combineImagesWithSourceContext(c1, b1, c2, b2, integerAllocationInvocationContext{})
}
func (m *UPolynomialManager) combineImagesWithSourceContext(c1 *upolyVectorState, b1 *unumeral.Int, c2 *upolyVectorState, b2 *unumeral.Int, context integerAllocationInvocationContext) (cause error) {
	if m.pendingCRATrace != nil && m.pendingCRATrace.pending() {
		return ErrUPolynomialCleanupPending
	}
	for i, bound := range []*unumeral.Int{b1, b2} {
		enabled, e := refinementSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled {
			even, e := m.config.Integer.IsEven(bound)
			if e != nil {
				return e
			}
			if even {
				expression := "!m().m().is_even(b1)"
				if i == 1 {
					expression = "!m().m().is_even(b2)"
				}
				if e := m.scalarDivisionAssertionFailure(int32(664+i), expression); e != nil {
					return e
				}
			}
		}
	}
	R := &m.cra
	if e := m.reset(R); e != nil {
		return e
	}
	var trace *upolyCRATrace
	local := &m.craValues
	values := local
	var frame *upolyFactorCRAScope
	if owner := context.factorFrontendFrame(m.config.Integer); owner != nil {
		frame = &owner.cra
		if frame.constructed != 0 || frame.executing || frame.invocation == ^uint64(0) {
			return ErrUPolynomialCleanupPending
		}
		*frame = upolyFactorCRAScope{owner: owner, invocation: frame.invocation + 1, constructed: 3, peak: 3, executing: true}
		values = &frame.values
		defer func() { frame.executing = false; cause = frame.finish(cause) }()
	} else {
		// Persistent Go storage is not a C++ member construction. Each call
		// admits its actual first three NUM declarations after the R reset.
		*local = [12]unumeral.Int{}
		trace = &m.craTrace
		if trace.invocation == ^uint64(0) {
			return ErrUPolynomialOwner
		}
		*trace = upolyCRATrace{invocation: trace.invocation + 1, owner: m, executing: true, declared: 3, values: local}
		if parent := context.squareFreeCallFrame(m.config.Integer); parent != nil {
			parent.craInvocation = trace.invocation
			trace.parent = parent
			trace.parentInvocation = parent.invocation
		}
		defer func() { cause = trace.finishDefault(local, cause) }()
	}
	if frame != nil && m.modular.BorrowTraceTags().ShouldTrace("CRA") {
		trace = &frame.trace
		if trace.pending() {
			return ErrUPolynomialCleanupPending
		}
		*trace = upolyCRATrace{owner: m, executing: true, declared: 3, values: values}
		if frame != nil {
			trace.frontend = frame.owner
			defer func() {
				trace.executing = false
				if !trace.pending() {
					trace.values = nil
				}
			}()
		}
	}
	inv1, inv2, g, a1, a2, bound, lower, upper, newA, tmp1, tmp2, tmp3 := &values[0], &values[1], &values[2], &values[3], &values[4], &values[5], &values[6], &values[7], &values[8], &values[9], &values[10], &values[11]
	construct := func(n uint8) {
		if trace != nil {
			trace.declared = n
		}
		if frame != nil {
			frame.constructed = n
			frame.peak = n
		}
	}

	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).ExtendedGCD(b1, b2, inv1, inv2, g); e != nil {
		return e
	}
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled {
		one, e := m.modular.IsOne(g)
		if e != nil {
			return e
		}
		if !one {
			if e := m.scalarDivisionAssertionFailure(671, "m().is_one(g)"); e != nil {
				return e
			}
		}
	}
	z := m.config.Integer
	if e := z.WithAllocationContext((context).utility(z)).Mod(inv1, b2, inv1); e != nil {
		return e
	}
	if e := z.WithAllocationContext((context).utility(z)).Mod(inv2, b1, inv2); e != nil {
		return e
	}
	if e := m.traceCRAPair(trace, 677, inv1, inv2); e != nil {
		return e
	}
	construct(5)
	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(b2, inv2, a1); e != nil {
		return e
	}
	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(b1, inv1, a2); e != nil {
		return e
	}
	if e := m.traceCRAPair(trace, 682, a1, a2); e != nil {
		return e
	}
	construct(6)
	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(b1, b2, bound); e != nil {
		return e
	}
	construct(12)
	two := unumeral.NewInt(2)
	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Div(bound, &two, upper); e != nil {
		return e
	}
	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(lower, upper); e != nil {
		return e
	}
	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Neg(lower); e != nil {
		return e
	}
	if e := m.traceCRAPair(trace, 692, lower, upper); e != nil {
		return e
	}
	zero := unumeral.NewInt(0)
	n1, n2 := c1.size, c2.size
	n := n1
	if n2 < n {
		n = n2
	}
	add := func(x, y *unumeral.Int) error {
		if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(x, a1, tmp1); e != nil {
			return e
		}
		if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Mul(y, a2, tmp2); e != nil {
			return e
		}
		if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Add(tmp1, tmp2, tmp3); e != nil {
			return e
		}
		if e := z.WithAllocationContext((context).utility(z)).Mod(tmp3, bound, newA); e != nil {
			return e
		}
		greater, e := m.modular.Greater(newA, upper)
		if e != nil {
			return e
		}
		if greater {
			if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Sub(newA, bound, newA); e != nil {
				return e
			}
		}
		dst, e := m.pushDefault(R)
		if e != nil {
			return e
		}
		return m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(dst, newA)
	}
	i := uint32(0)
	for ; i < n; i++ {
		if e := add(&m.values(c1)[i], &m.values(c2)[i]); e != nil {
			return e
		}
	}
	for ; i < n1; i++ {
		if e := add(&m.values(c1)[i], &zero); e != nil {
			return e
		}
	}
	for ; i < n2; i++ {
		if e := add(&zero, &m.values(c2)[i]); e != nil {
			return e
		}
	}
	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(b2, bound); e != nil {
		return e
	}
	*R, *c2 = *c2, *R
	return nil
}
func (m *UPolynomialManager) modularGCD(a, b UPolynomialView, out *upolyVectorState) error {
	return m.modularGCDWithSourceContext(a, b, out, integerAllocationInvocationContext{})
}
func (m *UPolynomialManager) modularGCDWithSourceContext(a, b UPolynomialView, out *upolyVectorState, context integerAllocationInvocationContext) (cause error) {
	if e := m.traceModGCDEntry(a, b, context); e != nil {
		return e
	}
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && (a.count == 0 || b.count == 0) {
		if e := m.scalarDivisionAssertionFailure(727, "sz_u > 0 && sz_v > 0"); e != nil {
			return e
		}
	}
	enabled, e = refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled {
		finite, e := m.modular.Modular()
		if e != nil {
			return e
		}
		if finite {
			if e := m.scalarDivisionAssertionFailure(728, "!m().modular()"); e != nil {
				return e
			}
		}
	}
	values := &m.modGCDScalars.values

	var frame *upolyFactorModularGCDScope
	if owner := context.factorFrontendFrame(m.config.Integer); owner != nil {
		frame = &owner.modularGCD
		if frame.constructed != 0 || frame.invocation == ^uint64(0) {
			return ErrUPolynomialCleanupPending
		}
		*frame = upolyFactorModularGCDScope{owner: owner, invocation: frame.invocation + 1, constructed: 2, executing: true}
		values = &frame.values
		defer func() { frame.executing = false; cause = frame.finish(cause) }()
	} else {
		if m.modGCDScalars.ownRemaining != 0 || m.modGCDScalars.executing {
			return ErrUPolynomialCleanupPending
		}
		if m.modGCDScalars.invocation == ^uint64(0) {
			return ErrUPolynomialOwner
		}
		m.modGCDScalars = upolyModGCDTraceScalars{invocation: m.modGCDScalars.invocation + 1, constructed: 2, executing: true}
		if parent := context.squareFreeCallFrame(m.config.Integer); parent != nil {
			parent.gcdInvocation = m.modGCDScalars.invocation
			m.modGCDScalars.parent = parent
			m.modGCDScalars.parentInvocation = parent.invocation
		}
		defer func() { cause = m.modGCDScalars.finish(m, cause); m.modGCDScalars.executing = false }()
	}

	cu, cv, cg, lcg, p, bound := &values[0], &values[1], &values[2], &values[3], &values[4], &values[5]
	u, v, q, C := &m.mgcd[0], &m.mgcd[1], &m.mgcd[4], &m.mgcd[5]
	if e := m.primitiveStateWithSourceContext(a, u, cu, context); e != nil {
		return e
	}
	if e := m.primitiveStateWithSourceContext(b, v, cv, context); e != nil {
		return e
	}
	if frame != nil {
		frame.constructed = 3
	} else {
		m.modGCDScalars.constructed = 3
	}
	if e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Gcd(cu, cv, cg); e != nil {
		return e
	}
	du, dv := a.count-1, b.count-1
	lu, e := m.read(m.stateView(u), du)
	if e != nil {
		return e
	}
	lv, e := m.read(m.stateView(v), dv)
	if e != nil {
		return e
	}
	if frame != nil {
		frame.constructed = 4
	} else {
		m.modGCDScalars.constructed = 4
	}
	if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Gcd(lu, lv, lcg); e != nil {
		return e
	}
	if frame != nil {
		frame.constructed = 6
	} else {
		m.modGCDScalars.constructed = 6
	}
	for i, prime := range upolyBigPrimes {
		if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).SetUint32(p, prime); e != nil {
			return e
		}
		if e = m.traceModGCDPrime(p); e != nil {
			return e
		}
		bad, e := m.modularImageWithSourceContext(u, v, q, p, lcg, du, dv, context)
		if e != nil {
			return e
		}
		if bad {
			continue
		}
		if e = m.traceModGCDPolynomial(772, "new q:\n", m.stateView(q), context); e != nil {
			return e
		}
		if e = m.trim(q); e != nil {
			return e
		}
		if q.size <= 1 {
			if e = m.traceModGCDLiteral(774, "done, modular gcd is one\n"); e != nil {
				return e
			}
			if e = m.reset(out); e != nil {
				return e
			}
			dst, e := m.pushDefault(out)
			if e != nil {
				return e
			}
			return m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(dst, cg)
		}
		discard := i == 0
		if !discard {
			discard = q.size < C.size
			if !discard {
				discard, e = m.config.Integer.IsEven(p)
				if e != nil {
					return e
				}
			}
			if !discard {
				discard, e = m.config.Integer.IsEven(bound)
				if e != nil {
					return e
				}
			}
		}
		if discard {
			if i != 0 {
				if e = m.traceModGCDLiteral(786, "discarding image\n"); e != nil {
					return e
				}
			}
			if e = m.setViewWithSourceContext(C, m.stateView(q), context); e != nil {
				return e
			}
			if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Copy(bound, p); e != nil {
				return e
			}
		} else {
			if e = m.combineImagesWithSourceContext(q, p, C, bound, context); e != nil {
				return e
			}
			if e = m.traceModGCDPolynomial(792, "new combined:\n", m.stateView(C), context); e != nil {
				return e
			}
		}
		if e = m.primitiveWithSourceContext(m.stateView(C), q, context); e != nil {
			return e
		}
		if e = m.traceModGCDPolynomial(796, "candidate:\n", m.stateView(q), context); e != nil {
			return e
		}
		enabled, e := refinementSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled && q.size == 0 {
			if e := m.scalarDivisionAssertionFailure(797, "candidate.size() > 0"); e != nil {
				return e
			}
		}
		last, e := m.read(m.stateView(q), q.size-1)
		if e != nil {
			return e
		}
		ok, e := m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Divides(last, lcg)
		if e != nil {
			return e
		}
		if ok {
			ok, e = m.dividesViewsWithSourceContext(m.stateView(u), m.stateView(q), context)
			if e != nil {
				return e
			}
		}
		if ok {
			ok, e = m.dividesViewsWithSourceContext(m.stateView(v), m.stateView(q), context)
			if e != nil {
				return e
			}
		}
		if ok {
			if e = m.traceModGCDLiteral(802, "found GCD\n"); e != nil {
				return e
			}
			if e = m.mulScalarStateWithSourceContext(q, cg, context); e != nil {
				return e
			}
			if e = m.flipSign(q); e != nil {
				return e
			}
			*q, *out = *out, *q
			return m.traceModGCDPolynomial(806, "r: ", m.stateView(out), context)
		}
	}
	return m.euclidGCDWithSourceContext(a, b, out, context)
}
func (m *UPolynomialManager) gcd(a, b UPolynomialView, out *upolyVectorState) error {
	return m.gcdWithSourceContext(a, b, out, integerAllocationInvocationContext{})
}
func (m *UPolynomialManager) gcdWithSourceContext(a, b UPolynomialView, out *upolyVectorState, context integerAllocationInvocationContext) error {
	done, e := m.gcdEmpty(a, b, out, 859)
	if done {
		return e
	}
	finite, e := m.modular.Modular()
	if e != nil {
		return e
	}
	if !finite {
		return m.modularGCDWithSourceContext(a, b, out, context)
	}
	return m.euclidGCDWithSourceContext(a, b, out, context)
}
func (m *UPolynomialManager) GCDViews(a, b UPolynomialView, g UPolynomial) error {
	out, e := m.descriptor(g)
	if e != nil {
		return e
	}
	return m.gcd(a, b, &out.state)
}
func (m *UPolynomialManager) GCD(a, b, g UPolynomial) error {
	av, e := m.wholeView(a)
	if e != nil {
		return e
	}
	bv, e := m.wholeView(b)
	if e != nil {
		return e
	}
	return m.GCDViews(av, bv, g)
}

// Literal records require no numeral insertion child, but borrow the actual
// receiving managers throughout header, body and footer callbacks.
func (m *UPolynomialManager) traceModGCDLiteral(line int, text string) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("mgcd") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	_ = tags.EmitConditional("mgcd", "mod_gcd", "src/math/polynomial/upolynomial.cpp", line, func() bool { return true }, func(raw io.Writer) error {
		_ = linearWriteText(factorizationTraceWriter{raw}, text)
		return nil
	})
	return nil
}

// The retained source prime table contains only positive SMALL numerals
// (39103..41549). Source integer insertion therefore takes AppendInt32 and
// constructs no character-buffer child or retained insertion receipt.
func (m *UPolynomialManager) traceModGCDPrime(p *unumeral.Int) error {
	tags := m.modular.BorrowTraceTags()
	if !tags.ShouldTrace("mgcd") {
		return nil
	}
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{m}, tags); e != nil {
		return e
	}
	defer borrow.unbind()
	var computation error
	_ = tags.EmitConditional("mgcd", "mod_gcd", "src/math/polynomial/upolynomial.cpp", 752, func() bool { return true }, func(raw io.Writer) error {
		out := factorizationTraceWriter{raw}
		_ = linearWriteText(out, "trying prime: ")
		if e := m.config.Integer.InsertSourceIntegerTracked(factorizationTraceStream{out}, p, nil, nil); e != nil {
			computation = e
			return e
		}
		_ = linearWriteText(out, "\n")
		return nil
	})
	return computation
}
