package polynomial

// Source: src/math/polynomial/polynomial.cpp:1885-2156,2159-2214.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type somRetainedMonomial struct {
	value    Monomial
	dropping bool
}

// buffer is the authoritative source class, not a shadow coefficient mirror.
// The reflected Go class header is real charged storage, not a Runtime ticket
// pretending to have the native class sizeof/global allocation geometry.
type PolynomialSOMBuffer struct {
	self                                            *PolynomialSOMBuffer
	storage                                         *MultivariatePolynomialCore
	buffer                                          polynomialSOM
	header                                          uint64
	retiredNumerals                                 []unumeral.Int
	retiredMonomials                                []somRetainedMonomial
	retiredNumeralCount, retiredMonomialCount       uint32
	numeralTransferred, monomialTransferred         uint32
	numeralCursor, monomialCursor                   uint32
	sourceCause, cleanupCause                       error
	sourceStage                                     uint8
	sourceStarted, sourceFinalized, closing, closed bool
	sortNumerals                                    polynomialNumerals
	sortActive                                      bool
	sortOriginalCause                               error
	generatedCall, calleeCall                       bool
	calleeGeneration                                uint64
	generatedValue                                  Monomial
}

func NewPolynomialSOMBuffer(m *MultivariatePolynomialCore) (*PolynomialSOMBuffer, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(PolynomialSOMBuffer{}).Size())
	if e := m.integer.Budget().Reserve(h); e != nil {
		return nil, e
	}
	b := &PolynomialSOMBuffer{storage: m, header: h}
	b.self = b

	b.buffer.positions.self = &b.buffer.positions
	// Borrowed transport is not source m_owner. The latter stays null until
	// the actual set_owner call, and empty Reset never dereferences it.
	b.buffer.positions.manager = m.monomials
	b.buffer.retirementOwner = b
	return b, nil
}
func (b *PolynomialSOMBuffer) ready() error {
	if b == nil || b.self != b {
		return ErrPolynomialOwner
	}
	if b.closed || b.closing || b.sourceStarted {
		return arena.ErrClosed
	}
	if b.sortActive {
		return ErrPolynomialPending
	}
	return b.storage.check()
}
func (b *PolynomialSOMBuffer) SetOwner(m *MultivariatePolynomialCore) error {
	if e := b.ready(); e != nil {
		return e
	}
	if m != b.storage {
		return ErrPolynomialOwner
	}
	b.buffer.owner = m
	return nil
}
func (b *PolynomialSOMBuffer) Size() (uint32, error) {
	if e := b.ready(); e != nil {
		return 0, e
	}
	return b.buffer.ms.size(b.storage)
}
func (b *PolynomialSOMBuffer) Empty() (bool, error) { n, e := b.Size(); return n == 0, e }
func (b *PolynomialSOMBuffer) Reset() error {
	if e := b.ready(); e != nil {
		return e
	}
	n, e := b.buffer.ms.size(b.storage)
	if e != nil || n == 0 {
		return e
	}
	if b.buffer.owner == nil {
		return ErrPolynomialOwner
	}
	return b.buffer.reset()
}
func (b *PolynomialSOMBuffer) Add(a *unumeral.Int, t Monomial) error {
	if e := b.ready(); e != nil {
		return e
	}
	if b.buffer.owner == nil {
		return ErrPolynomialOwner
	}
	return b.buffer.add(a, t, false)
}
func (b *PolynomialSOMBuffer) AddTake(a *unumeral.Int, t Monomial) error {
	if e := b.ready(); e != nil {
		return e
	}
	if b.buffer.owner == nil {
		return ErrPolynomialOwner
	}
	return b.buffer.add(a, t, true)
}

// Only transitions of the borrowed core's concrete named creation fields
// caused by this invocation become this class's owning-abort obligations.
// Source member destruction never aborts a returned local or failed callee.
func (b *PolynomialSOMBuffer) AddMul(a *unumeral.Int, t Monomial, p Polynomial) error {
	return b.addInput(a, t, polynomialSOMInput{polynomial: p}, true)
}
func (b *PolynomialSOMBuffer) AddMulBuffer(a *unumeral.Int, t Monomial, p *PolynomialSOMBuffer) error {
	return b.addInput(a, t, polynomialSOMInput{buffer: p, fromBuffer: true}, true)
}
func (b *PolynomialSOMBuffer) AddScaledBuffer(a *unumeral.Int, p *PolynomialSOMBuffer) error {
	if e := b.ready(); e != nil {
		return e
	}
	if b.buffer.owner == nil {
		return ErrPolynomialOwner
	}
	u, e := b.storage.monomials.Unit()
	if e != nil {
		return e
	}
	return b.addInput(a, u, polynomialSOMInput{buffer: p, fromBuffer: true}, true)
}
func (b *PolynomialSOMBuffer) AddPolynomial(p Polynomial) error {
	return b.addInput(nil, Monomial{}, polynomialSOMInput{polynomial: p}, false)
}
func (b *PolynomialSOMBuffer) addInput(a *unumeral.Int, t Monomial, p polynomialSOMInput, multiply bool) (result error) {
	if e := b.ready(); e != nil {
		return e
	}
	if b.buffer.owner == nil {
		return ErrPolynomialOwner
	}
	m := b.storage
	oldGenerated, oldCallee := m.generatedActive, m.generatedCalleePending
	defer func() {
		if !oldGenerated && m.generatedActive {
			b.generatedCall = true
			b.generatedValue = m.generatedMonomial
		}
		if !oldCallee && m.generatedCalleePending {
			b.calleeCall = true
			b.calleeGeneration = m.generatedCalleeGeneration
		}
	}()
	return b.buffer.addSourceInput(a, t, p, multiply)
}
func (b *PolynomialSOMBuffer) cleanupGeneratedCall() error {
	m := b.storage
	if b.generatedCall {
		if m.generatedActive && m.generatedMonomial == b.generatedValue {
			if e := m.cleanupGeneratedMonomial(); e != nil {
				return e
			}
		}
		b.generatedCall = false
		b.generatedValue = Monomial{}
	}
	if b.calleeCall {
		// The receipt names the actual factory continuation this body started.
		// Explicit callee completion may have relinquished it before a new,
		// unrelated construction reused the same monomial-manager owner.
		c := &m.monomials.creation
		if b.calleeGeneration != 0 && m.generatedCalleeGeneration == b.calleeGeneration && c.generation == b.calleeGeneration && c.parent == m {
			if e := m.monomials.CleanupCreation(); e != nil {
				return e
			}
		}
		b.calleeCall = false
		b.calleeGeneration = 0
	}
	return nil
}
func (b *PolynomialSOMBuffer) Coefficient(i uint32) (unumeral.Int, error) {
	if e := b.ready(); e != nil {
		return unumeral.Int{}, e
	}
	if polynomialSourceDebug {
		n, e := b.buffer.as.size(b.storage)
		if e != nil {
			return unumeral.Int{}, e
		}
		if i >= n {
			return unumeral.Int{}, ErrPolynomialAssertion
		}
	}
	return b.buffer.as.coefficient(b.storage, i)
}
func (b *PolynomialSOMBuffer) Monomial(i uint32) (Monomial, error) {
	if e := b.ready(); e != nil {
		return Monomial{}, e
	}
	if polynomialSourceDebug {
		n, e := b.buffer.ms.size(b.storage)
		if e != nil {
			return Monomial{}, e
		}
		if i >= n {
			return Monomial{}, ErrPolynomialAssertion
		}
	}
	return b.buffer.ms.monomial(b.storage, i)
}
func (b *PolynomialSOMBuffer) Mk(normalize bool) (Polynomial, error) {
	if e := b.ready(); e != nil {
		return Polynomial{}, e
	}
	if b.buffer.owner == nil {
		return Polynomial{}, ErrPolynomialOwner
	}
	return b.buffer.mk(normalize)
}

func (b *PolynomialSOMBuffer) ownsMonomial(i uint32) bool {
	a := &b.buffer
	if b.generatedCall && b.storage.generatedActive && b.storage.generatedBuffer == a && b.storage.generatedIndex == i {
		return false
	}
	if i < a.cleanupIndex || i < a.transferred || (a.resetActive && i < a.resetIndex) {
		return false
	}
	return !(a.borrowedAppend && i == a.borrowedAppendIndex) && !(a.unreferencedConsumed && i == a.unreferencedIndex) && (!a.removeActive || i < a.removePrefix || i >= a.removeSuffix)
}
func (b *PolynomialSOMBuffer) retireMonomials() error {
	a := &b.buffer
	n, e := a.ms.size(b.storage)
	if e != nil {
		return e
	}
	for b.monomialTransferred < n {
		i := b.monomialTransferred
		if b.ownsMonomial(i) {
			t, e := a.ms.monomial(b.storage, i)
			if e != nil {
				return e
			}
			dropping := false
			if (a.resetActive && i == a.resetIndex && a.resetStage == 3) || (a.removeActive && a.removeStopped && i == a.removeSuffix) {
				l, e := b.storage.monomials.location(t)
				if e != nil {
					return e
				}
				dropping = l.stage != 0
			}
			b.retiredMonomials = append(b.retiredMonomials, somRetainedMonomial{t, dropping})
			b.retiredMonomialCount++
		}
		b.monomialTransferred++
	}
	return nil
}
func (b *PolynomialSOMBuffer) retireNumerals() error {
	a := &b.buffer.as
	for b.numeralTransferred < uint32(len(a.values)) {
		v, e := a.coefficient(b.storage, b.numeralTransferred)
		if e != nil {
			return e
		}
		if v.NativeCellToken() != 0 {
			b.retiredNumerals = append(b.retiredNumerals, v)
			b.retiredNumeralCount++
		}
		b.numeralTransferred++
	}
	return nil
}

// som_buffer has an implicit destructor: no Reset, no numeral Del and no
// monomial DEC. Its reverse members are ms, as, then monomial2pos.
func (b *PolynomialSOMBuffer) SourceFinalize() error {
	if b == nil || b.self != b {
		return ErrPolynomialOwner
	}
	if b.buffer.normalizeScope.active || b.buffer.normalizeScope.executing || b.buffer.normalizeScope.finishing {
		return ErrPolynomialPending
	}
	if b.closed || b.closing {
		return arena.ErrClosed
	}
	if b.sourceCause != nil {
		return b.sourceCause
	}
	if b.sourceFinalized {
		return nil
	}
	b.sourceStarted = true
	fail := func(e error) error { b.sourceCause = e; return e }
	if b.sourceStage == 0 {
		if e := b.retireMonomials(); e != nil {
			return fail(e)
		}
		b.buffer.ms = nil
		b.sourceStage++
	}
	if b.sourceStage == 1 {
		if e := b.retireNumerals(); e != nil {
			return fail(e)
		}
		b.buffer.as.values = nil
		b.sourceStage++
	}
	if b.sourceStage == 2 {
		if e := b.buffer.positions.SourceFinalize(); e != nil {
			return fail(e)
		}
		b.sourceStage++
	}
	b.sourceFinalized = true
	return nil
}
func (b *PolynomialSOMBuffer) CloseChecked() error {
	if b == nil || b.self != b {
		return ErrPolynomialOwner
	}
	if b.buffer.normalizeScope.executing || b.buffer.normalizeScope.finishing {
		return ErrPolynomialPending
	}
	if b.closed {
		return nil
	}
	b.closing = true
	if e := b.cleanupGeneratedCall(); e != nil {
		b.cleanupCause = e
		return e
	}
	if b.sourceStarted {
		if e := b.retireMonomials(); e != nil {
			return e
		}
		if e := b.retireNumerals(); e != nil {
			return e
		}
		// Authority has moved to and been consumed from the named abort records.
		// Raw pointer bytes are source-stale and must not be deleted a second time.
		n, e := b.buffer.ms.size(b.storage)
		if e != nil {
			return e
		}
		a, e := b.buffer.as.size(b.storage)
		if e != nil {
			return e
		}
		if a > n {
			n = a
		}
		b.buffer.cleanupIndex = n
		clear(b.buffer.as.values)
		b.buffer.as.values = nil
	}
	for b.numeralCursor < b.retiredNumeralCount {
		if e := b.storage.integer.Del(&b.retiredNumerals[b.numeralCursor]); e != nil {
			return e
		}
		b.numeralCursor++
	}
	for b.monomialCursor < b.retiredMonomialCount {
		a := &b.retiredMonomials[b.monomialCursor]
		if a.value != (Monomial{}) {
			if a.dropping || b.storage.monomials.sourceFinalizing {
				if e := b.storage.monomials.CleanupValue(&a.value); e != nil {
					return e
				}
			} else {
				e := b.storage.monomials.decRefCore(a.value)
				if e != nil {
					l, x := b.storage.monomials.location(a.value)
					if x == nil && l.stage != 0 {
						a.dropping = true
					}
					return e
				}
				a.value = Monomial{}
			}
		}
		b.monomialCursor++
	}

	if e := b.sortNumerals.close(b.storage); e != nil {
		return e
	}
	b.sortActive = false
	if b.buffer.owner != nil {
		if e := b.buffer.close(); e != nil {
			b.cleanupCause = e
			return e
		}
	} else {
		if e := b.buffer.ms.close(b.storage); e != nil {
			return e
		}
		if e := b.buffer.as.close(b.storage); e != nil {
			return e
		}
		if e := b.buffer.positions.CloseChecked(); e != nil {
			return e
		}
	}
	b.storage.integer.Budget().Release(b.header)
	b.header = 0
	b.retiredNumerals = nil
	b.retiredMonomials = nil
	b.cleanupCause = nil
	b.closed = true
	return nil
}

// Source: src/math/polynomial/polynomial.cpp::som_buffer::graded_lex_max_pos.
func (b *PolynomialSOMBuffer) GradedLexMaxPosition() (uint32, error) {
	if e := b.ready(); e != nil {
		return 0, e
	}
	if b.buffer.owner == nil {
		return 0, ErrPolynomialOwner
	}
	n, e := b.buffer.as.size(b.storage)
	if e != nil {
		return 0, e
	}
	max := ^uint32(0)
	for i := uint32(0); i < n; i++ {
		a, e := b.buffer.as.coefficient(b.storage, i)
		if e != nil {
			return 0, e
		}
		zero, e := b.storage.modular.IsZero(&a)
		if e != nil {
			return 0, e
		}
		if zero {
			continue
		}
		if max == ^uint32(0) {
			max = i
			continue
		}
		x, e := b.buffer.ms.monomial(b.storage, i)
		if e != nil {
			return 0, e
		}
		y, e := b.buffer.ms.monomial(b.storage, max)
		if e != nil {
			return 0, e
		}
		c, e := b.storage.monomials.GradedLex(x, y)
		if e != nil {
			return 0, e
		}
		if c > 0 {
			max = i
		}
	}
	return max, nil
}

// Source: src/math/polynomial/polynomial.cpp::som_buffer::display.
// The borrowed sink transports the actual numeral formatter output; no
// retained numeral text or shadow coefficient storage is introduced.
func (b *PolynomialSOMBuffer) DisplayInto(out MonomialTextSink) error {
	if e := b.ready(); e != nil {
		return e
	}
	if polynomialSourceDebug {
		n, e := b.buffer.ms.size(b.storage)
		if e != nil {
			return e
		}
		a, e := b.buffer.as.size(b.storage)
		if e != nil {
			return e
		}
		if e = polynomialSOMSourceAssert(2149, "m_tmp_ms.size() == m_tmp_as.size()", n == a); e != nil {
			return e
		}
	}
	if b.buffer.owner == nil {
		return ErrPolynomialOwner
	}
	for i := uint32(0); ; i++ {
		if e := b.ready(); e != nil {
			return e
		}
		n, e := b.buffer.as.size(b.storage)
		if e != nil {
			return e
		}
		if i >= n {
			break
		}
		if i > 0 {
			if e = monomialSinkLiteral(out, " + "); e != nil {
				return e
			}
		}
		a, e := b.buffer.as.coefficient(b.storage, i)
		if e != nil {
			return e
		}
		value, e := b.storage.integer.String(&a)
		if e != nil {
			return e
		}
		if e = monomialSinkLiteral(out, value); e != nil {
			return e
		}
		if e = monomialSinkLiteral(out, "*"); e != nil {
			return e
		}
		term, e := b.buffer.ms.monomial(b.storage, i)
		if e != nil {
			return e
		}
		if e = b.storage.monomials.DisplayInto(term, out, false); e != nil {
			return e
		}
	}
	return monomialSinkLiteral(out, "\n")
}

func (b *PolynomialSOMBuffer) AddScalar(a *unumeral.Int) error {
	if e := b.ready(); e != nil {
		return e
	}
	if b.buffer.owner == nil {
		return ErrPolynomialOwner
	}
	u, e := b.storage.monomials.Unit()
	if e != nil {
		return e
	}
	return b.buffer.add(a, u, false)
}
func (b *PolynomialSOMBuffer) AddMonomialBuffer(t Monomial, p *PolynomialSOMBuffer) error {
	one := unumeral.NewInt(1) // source completed stack numeral one(1), no heap payload.
	return b.AddMulBuffer(&one, t, p)
}
func (b *PolynomialSOMBuffer) ModD(d *PolynomialVariableDegrees) error {
	if e := b.ready(); e != nil {
		return e
	}
	if b.buffer.owner == nil {
		return ErrPolynomialOwner
	}
	return b.buffer.modD(d)
}
