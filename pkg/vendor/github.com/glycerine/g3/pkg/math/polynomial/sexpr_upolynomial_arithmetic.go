package polynomial

// Source: src/math/polynomial/sexpr2upolynomial.cpp:39-85.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// One accepted recursive arithmetic call at a time; completed inner calls have
// ended before the enclosing operator starts. Only issued children survive.
type sexprPolynomialArithmeticScope struct {
	owner                       *SexprPolynomialConverter
	invocation                  uint64
	active, executing, cleaning bool
	previous                    *sexprPolynomialArithmeticScope
	hooks                       unumeral.NumericInvocationHooks
	numeric                     unumeral.NumericAllocationRoots
	raw                         []unumeral.IntegerRawInvocationReceipt
	addMul                      unumeral.IntegerAddMulInvocationReceipt
	normalize                   unumeral.IntegerNormalizeInvocationReceipt
	integer, natural            unumeral.NumericCallerBinding
}

func (s *sexprPolynomialArithmeticScope) NumericExecuting() bool { return s.executing || s.cleaning }
func (s *sexprPolynomialArithmeticScope) NumericBound(executing bool) bool {
	c := s.owner
	return c != nil && c.self == c && !c.closed && s == &c.arithmetic && s.active && (!executing || s.executing && !s.cleaning && c.polynomial.callbackDepth == 0 && !c.polynomial.config.Integer.RuntimeOwner().FailureTransportActive() && !c.polynomial.config.Integer.BorrowTraceTags().OutputCallbackActive())
}
func (s *sexprPolynomialArithmeticScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !s.NumericBound(true) || s.owner.polynomial.config.Integer != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.numeric
}
func (s *sexprPolynomialArithmeticScope) NumericRecord(z *unumeral.IntegerManager, _ string, r unumeral.NumericInvocationReceipt) {
	if s.NumericBound(true) && s.owner.polynomial.config.Integer == z && r.Raw != nil {
		s.raw = append(s.raw, *r.Raw)
	}
}
func (s *sexprPolynomialArithmeticScope) NumericNormalize(m *unumeral.IntegerModularManager, a unumeral.Int) (unumeral.Int, error) {
	if !s.NumericBound(true) || m != s.owner.polynomial.modular {
		return a, ErrUPolynomialOwner
	}
	e := m.PNormalizeWithFactorHistory(&a, &s.normalize, unumeral.NumericAllocationContext{}.WithCaller(&s.hooks))
	return a, e
}
func (s *sexprPolynomialArithmeticScope) pending() bool {
	if s.owner == nil || !s.active {
		return false
	}
	m := s.owner.polynomial
	z := m.config.Integer
	if unumeral.NumericAllocationPending(&s.numeric) || z.MatchesAddMulInvocation(&s.addMul) || m.modular.MatchesPNormalizeInvocation(&s.normalize) {
		return true
	}
	for i := range s.raw {
		if z.MatchesRawInvocation(&s.raw[i]) {
			return true
		}
	}
	return false
}
func (s *sexprPolynomialArithmeticScope) bind() error {
	m := s.owner.polynomial
	s.previous = m.sexprArithmetic
	m.sexprArithmetic = s
	z := m.config.Integer
	if e := z.BindCaller(&s.integer, s, 20); e != nil {
		m.sexprArithmetic = s.previous
		s.previous = nil
		return e
	}
	if e := z.Natural.BindCaller(&s.natural, s, 20); e != nil {
		_ = s.integer.Unbind()
		m.sexprArithmetic = s.previous
		s.previous = nil
		return e
	}
	return nil
}
func (s *sexprPolynomialArithmeticScope) unbind() {
	if e := s.natural.Unbind(); e != nil {
		panic(e)
	}
	if e := s.integer.Unbind(); e != nil {
		panic(e)
	}
	m := s.owner.polynomial
	if m.sexprArithmetic != s {
		panic(ErrUPolynomialOwner)
	}
	m.sexprArithmetic = s.previous
	s.previous = nil
}
func (c integerAllocationInvocationContext) sexprArithmeticFrame(z *unumeral.IntegerManager) *sexprPolynomialArithmeticScope {
	s := c.sexprArithmetic
	if s == nil || !s.NumericBound(true) || s.owner.polynomial.config.Integer != z || c.sexprArithmeticInvocation == 0 || c.sexprArithmeticInvocation != s.invocation {
		return nil
	}
	return s
}
func (m *UPolynomialManager) sexprArithmeticExecuting() bool {
	return m != nil && m.self == m && m.sexprArithmetic != nil && m.sexprArithmetic.NumericExecuting()
}

func (c *SexprPolynomialConverter) arithmeticCall(ctx *UPolynomialRationalContext, op string, p, arg UPolynomial, exponent uint32) error {
	m := c.polynomial
	s := &c.arithmetic
	if s.active || s.invocation == ^uint64(0) {
		return ErrSexprPolynomialPending
	}
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	a, e := m.wholeView(p)
	if e != nil {
		return e
	}
	var b UPolynomialView
	if op == "+" || op == "-" || op == "*" {
		b, e = m.wholeView(arg)
		if e != nil {
			return e
		}
	}
	*s = sexprPolynomialArithmeticScope{owner: c, invocation: s.invocation + 1, active: true, executing: true}
	s.hooks = unumeral.NumericInvocationHooks{Generation: s.invocation, Policy: s}
	if e := s.bind(); e != nil {
		s.executing = false
		s.active = false
		return e
	}
	defer func() {
		s.unbind()
		s.executing = false
		if !s.pending() {
			s.active = false
		} else {
			m.pendingSexprArithmetic = s
		}
	}()
	context := integerAllocationInvocationContext{sexprArithmetic: s, sexprArithmeticInvocation: s.invocation}
	return ctx.borrow.WithReceivingInteger(m.config.Integer, func() error {
		switch op {
		case "+":
			return m.addSubStateWithSourceContext(b, a, &d.state, false, context)
		case "-":
			return m.addSubStateWithSourceContext(a, b, &d.state, true, context)
		case "*":
			return m.mulWithSourceContext(b, a, &d.state, &s.addMul, context)
		case "neg":
			return m.negViewInPlaceWithSourceContext(a, context)
		case "^":
			return m.powerStateWithSourceContext(a, exponent, &d.state, &s.addMul, context)
		}
		return ErrUPolynomialOwner
	})
}

func (c *SexprPolynomialConverter) cleanupArithmetic() error {
	s := &c.arithmetic
	if !s.active {
		return nil
	}
	if !s.NumericBound(false) {
		return ErrUPolynomialOwner
	}
	if s.NumericExecuting() {
		return ErrSexprPolynomialPending
	}
	s.cleaning = true
	if e := s.bind(); e != nil {
		s.cleaning = false
		return e
	}
	defer func() { s.unbind(); s.cleaning = false }()
	m := c.polynomial
	z := m.config.Integer
	for _, abort := range []bool{false, true} {
		for i := len(s.raw) - 1; i >= 0; i-- {
			if e := z.CleanupRawInvocation(&s.raw[i], abort); e != nil {
				return e
			}
		}
		if e := z.CleanupAddMulInvocation(&s.addMul, abort); e != nil {
			return e
		}
	}
	if e := m.modular.CleanupPNormalizeInvocation(&s.normalize, true); e != nil {
		return e
	}
	if e := s.numeric.Cleanup(z); e != nil {
		return e
	}
	s.active = false
	s.raw = nil
	if m.pendingSexprArithmetic == s {
		m.pendingSexprArithmetic = nil
	}
	return nil
}
