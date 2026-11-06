package polynomial

// Source: src/math/polynomial/polynomial.cpp:1517-1610,7421-7434,7471-7478;
// src/util/mpz.cpp; src/util/mpzzp.h; src/util/scoped_numeral.h.

import (
	"reflect"
	"strings"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// PolynomialTextSink is a cold stream binding. Decimal bytes are borrowed only
// for WriteDecimal; each access checks their actual lexical/physical lifetime.
type PolynomialTextSink interface {
	MonomialTextSink
	WriteDecimal(*PolynomialDecimalView) error
}
type PolynomialDecimalView struct {
	frame      *polynomialDisplayFrame
	generation uint64
}

func (v *PolynomialDecimalView) Byte(i uint32) (byte, error) {
	if v == nil || v.frame == nil || !v.frame.inserting || v.generation != v.frame.generation {
		return 0, arena.ErrInvalidSlot
	}
	f := v.frame
	if f.big {
		if uint64(i) >= uint64(len(f.characters)) {
			return 0, arena.ErrBounds
		}
		return f.characters[i], nil
	}
	if i >= f.smallSize {
		return 0, arena.ErrBounds
	}
	return f.small[i], nil
}
func (v *PolynomialDecimalView) Length() (uint32, error) {
	if v == nil || v.frame == nil || !v.frame.inserting || v.generation != v.frame.generation {
		return 0, arena.ErrInvalidSlot
	}
	if !v.frame.big {
		return v.frame.smallSize, nil
	}
	for i := uint32(0); ; i++ {
		b, e := v.Byte(i)
		if e != nil {
			return 0, e
		}
		if b == 0 {
			return i, nil
		}
		if i == ^uint32(0) {
			return 0, arena.ErrBounds
		}
	}
}

type polynomialDisplayState struct {
	receipt *polynomialDisplayInvocationReceipt
	frames  []*polynomialDisplayFrame
	next    uint64
	current *polynomialDisplayFrame
}
type polynomialDisplayFrame struct {
	parent, younger                      *polynomialDisplayFrame
	youngerGeneration                    uint64
	owner                                *MultivariatePolynomialCore
	sink                                 PolynomialTextSink
	monomialSink                         MonomialTextSink
	generation                           uint64
	active, stopped                      bool
	numeral                              unumeral.Int
	constructed                          bool
	characters                           []byte
	characterCompleted, characterPending bool
	limbs                                unumeral.NaturalFormattingScope
	numeric                              polynomialNumericRoots
	small                                [12]byte
	smallSize                            uint32
	big, inserting                       bool
	view                                 PolynomialDecimalView
	bodyCause, sourceCause, cleanupCause error
	assembling                           bool
	assembly                             strings.Builder
}

func (m *MultivariatePolynomialCore) displayFrame(sink PolynomialTextSink) (frame *polynomialDisplayFrame, cause error) {
	defer func() {
		if frame != nil && cause == nil && m.display.receipt != nil && m.callbackDepth == 0 {
			*m.display.receipt = polynomialDisplayInvocationReceipt{frame, frame.generation}
		}
	}()
	s := &m.display
	for _, f := range s.frames {
		if !f.active {
			s.next++
			*f = polynomialDisplayFrame{owner: m, parent: s.current, sink: sink, generation: s.next, active: true}
			if owner := m.factorsOwner; owner != nil && owner.executing && m.callbackDepth == 0 {
				owner.displayReceipt = f
				owner.displayGeneration = f.generation
			}
			return f, nil
		}
	}

	if e := m.integer.Budget().Reserve(uint64(reflect.TypeOf(polynomialDisplayFrame{}).Size())); e != nil {
		return nil, e
	}
	s.next++
	f := &polynomialDisplayFrame{owner: m, parent: s.current, sink: sink, generation: s.next, active: true}
	if owner := m.factorsOwner; owner != nil && owner.executing && m.callbackDepth == 0 {
		owner.displayReceipt = f
		owner.displayGeneration = f.generation
	}
	s.frames = append(s.frames, f)
	return f, nil
}
func (f *polynomialDisplayFrame) WriteLiteral(s string) error {
	if f.assembling {
		f.assembly.WriteString(s)
		return nil
	}
	previous := f.owner.display.current
	f.owner.display.current = f
	f.owner.callbackDepth++
	defer func() { f.owner.callbackDepth--; f.owner.display.current = previous }()
	if f.monomialSink != nil {
		return f.monomialSink.WriteLiteral(s)
	}
	return f.sink.WriteLiteral(s)
}
func (f *polynomialDisplayFrame) WriteUnsigned(v uint32) error {
	previous := f.owner.display.current
	f.owner.display.current = f
	f.owner.callbackDepth++
	defer func() { f.owner.callbackDepth--; f.owner.display.current = previous }()
	if f.monomialSink != nil {
		return f.monomialSink.WriteUnsigned(v)
	}
	return f.sink.WriteUnsigned(v)
}
func (f *polynomialDisplayFrame) WriteVariable(v uint32) error {
	previous := f.owner.display.current
	f.owner.display.current = f
	f.owner.callbackDepth++
	defer func() { f.owner.callbackDepth--; f.owner.display.current = previous }()
	if f.monomialSink != nil {
		return f.monomialSink.WriteVariable(v)
	}
	return f.sink.WriteVariable(v)
}
func (m *MultivariatePolynomialCore) displayLiteral(sink PolynomialTextSink, s string) error {
	m.callbackDepth++
	defer func() { m.callbackDepth-- }()
	return sink.WriteLiteral(s)
}
func (f *polynomialDisplayFrame) numericCall(op uint8, a *unumeral.Int) error {
	z := f.owner.integer
	before, e := polynomialNumericBefore(z)
	if e != nil {
		return e
	}
	switch op {
	case 0:
		e = f.owner.modular.Copy(&f.numeral, a)
	case 1:
		e = f.owner.modular.Abs(&f.numeral)
	case 2:
		e = f.owner.modular.Neg(&f.numeral)
	}
	x := f.numeric.after(z, before)
	if e != nil {
		return e
	}
	return x
}
func (f *polynomialDisplayFrame) finishNumeral() error {
	if f.stopped {
		return f.sourceCause
	}
	if !f.constructed {
		return nil
	}
	_, e := upolynomialSourceNumeralDel(f.owner.integer, &f.numeral)
	if e != nil {
		f.stopped = true
		f.sourceCause = e
		return e
	}
	f.constructed = false
	return nil
}
func (f *polynomialDisplayFrame) finish(body error) error {
	defer func() {
		if f.stopped && f.parent != nil {
			f.parent.younger = f
			f.parent.youngerGeneration = f.generation
		}
	}()
	f.observeYoungerStop()
	if f.bodyCause == nil {
		f.bodyCause = body
	}
	f.sink = nil
	f.monomialSink = nil
	if f.stopped {
		return polynomialNewtonJoin(body, f.sourceCause)
	}
	if e := f.finishNumeral(); e != nil {
		return polynomialNewtonJoin(body, e)
	}
	if f.characterPending || f.limbs.Pending() || f.numeric.PendingObject() != (umemory.ObjectRef{}) || f.numeric.PendingAllocatorBlocks() != ([3]umemory.Block{}) {
		return body
	}
	f.active = false
	return body
}

// SMALL ostream decimal insertion is a fixed machine-integer transport adapter.
// It uses the same quotient/remainder and reversal order as prepareInteger.
func (f *polynomialDisplayFrame) smallDecimal(v int32) {
	n := int64(v)
	i := uint32(0)
	if n < 0 {
		f.small[i] = '-'
		i++
		n = -n
	}
	start := i
	for {
		f.small[i] = '0' + byte(n%10)
		i++
		n /= 10
		if n == 0 {
			break
		}
	}
	for l, r := start, i; l < r; {
		r--
		if l >= r {
			break
		}
		f.small[l], f.small[r] = f.small[r], f.small[l]
		l++
	}
	f.smallSize = i
	f.big = false
}
func (f *polynomialDisplayFrame) displayInteger(a *unumeral.Int) error {
	z := f.owner.integer
	if e := z.Check(a); e != nil {
		return e
	}
	if a.NativeCellToken() == 0 {
		f.smallDecimal(a.NativeScalar())
	} else {
		// Source emits BIG sign before constructing its char local.
		if a.NativeScalar() < 0 {
			if e := f.WriteLiteral("-"); e != nil {
				return e
			}
		}
		magnitude, e := z.BorrowMagnitude(a)
		if e != nil {
			return e
		}
		f.characters = nil
		f.characterPending = true
		n := uint32(11 * magnitude.Size())
		if uint64(n) > uint64(^uint(0)>>1) {
			return arena.ErrBounds
		}
		f.characters = make([]byte, n)
		f.characterCompleted = true
		e = magnitude.WriteCString(f.characters, &f.limbs)
		if e != nil && f.limbs.Completed() != 0 {
			f.stopped = true
			f.sourceCause = e
			if f.limbs.CleanupCause() != nil {
				f.sourceCause = f.limbs.CleanupCause()
			}
			return e
		}
		if e != nil {
			return f.destroyCharacters(e)
		}
		f.big = true
	}
	f.inserting = true
	f.view = PolynomialDecimalView{frame: f, generation: f.generation}
	var e error
	if f.assembling {
		var n uint32
		n, e = f.view.Length()
		if e == nil {
			for i := uint32(0); i < n; i++ {
				var b byte
				b, e = f.view.Byte(i)
				if e != nil {
					break
				}
				f.assembly.WriteByte(b)
			}
		}
	} else {
		previous := f.owner.display.current
		f.owner.display.current = f
		f.owner.callbackDepth++
		e = f.sink.WriteDecimal(&f.view)
		f.owner.callbackDepth--
		f.owner.display.current = previous
	}
	f.inserting = false
	if f.big {
		return f.destroyCharacters(e)
	}
	return e
}
func (f *polynomialDisplayFrame) destroyCharacters(body error) error {
	f.observeYoungerStop()
	if f.stopped {
		return polynomialNewtonJoin(body, f.sourceCause)
	}
	f.characters = nil
	f.characterCompleted = false
	f.characterPending = false
	return body
}
func (m *MultivariatePolynomialCore) CleanupDisplay() error {
	if m != nil && m.self == m && (m.renameTraceRunning || m.gcdTraceRunning || m.dividesTrace.running || m.quadraticTraceCallback() || m.evaluationTraceCallback()) {
		return ErrPolynomialPending
	}
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	for i := len(m.display.frames) - 1; i >= 0; i-- {
		f := m.display.frames[i]
		if !f.active {
			continue
		}
		if f.sink != nil || f.monomialSink != nil || f.inserting {
			return ErrPolynomialPending
		}
		if e := m.cleanupDisplayFrame(f); e != nil {
			return e
		}
	}
	return nil
}
func (m *MultivariatePolynomialCore) cleanupDisplayFrame(f *polynomialDisplayFrame) error {
	if !f.active {
		return nil
	}
	if f.sink != nil || f.monomialSink != nil || f.inserting {
		return ErrPolynomialPending
	}
	if e := f.limbs.CleanupAfterSource(m.integer.Naturals()); e != nil {
		f.cleanupCause = e
		return e
	}
	f.characters = nil
	f.characterCompleted = false
	f.characterPending = false
	if f.constructed {
		if e := m.integer.Del(&f.numeral); e != nil {
			f.cleanupCause = e
			return e
		}
		f.constructed = false
	}
	if e := f.numeric.cleanup(m.integer); e != nil {
		f.cleanupCause = e
		return e
	}
	f.active = false
	return nil
}
func (m *MultivariatePolynomialManager) CleanupDisplay() error {
	if e := m.identity(); e != nil {
		return e
	}
	return m.core.CleanupDisplay()
}
func (m *MultivariatePolynomialCore) displayPending() bool {
	for _, f := range m.display.frames {
		if f.active {
			return true
		}
	}
	return false
}
func (m *MultivariatePolynomialCore) closeDisplayMetadata() error {
	if m.displayPending() {
		return ErrPolynomialPending
	}
	m.integer.Budget().Release(uint64(len(m.display.frames)) * uint64(reflect.TypeOf(polynomialDisplayFrame{}).Size()))
	m.display.frames = nil
	return nil
}

func (m *MultivariatePolynomialCore) displayMonomialSize(a Monomial) (uint32, error) {
	l, e := m.monomials.location(a)
	if e != nil {
		return 0, e
	}
	return m.monomials.allocator.ReadUint32(l.raw, 12)
}
func (m *MultivariatePolynomialCore) displayConsistent(p Polynomial, sink PolynomialTextSink) (err error) {
	f, e := m.displayFrame(sink)
	if e != nil {
		return e
	}
	f.constructed = true
	defer func() { err = f.finish(err) }()
	n, e := m.readWord(p, 8)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		a, e := m.coefficientCore(p, i)
		if e != nil {
			return e
		}
		if e = f.numericCall(0, &a); e != nil {
			return e
		}
		if e = polynomialDisplaySourceAssert(1854, "m_manager.eq(a, p->a(i))", func() (bool, error) {
			a, e := m.coefficientCore(p, i)
			if e != nil {
				return false, e
			}
			before, e := polynomialNumericBefore(m.integer)
			if e != nil {
				return false, e
			}
			equal, e := m.modular.Equal(&f.numeral, &a)
			after := f.numeric.after(m.integer, before)
			if e != nil {
				return false, e
			}
			return equal, after
		}); e != nil {
			return e
		}
	}
	return nil
}
func (m *MultivariatePolynomialCore) DisplayInto(p Polynomial, sink PolynomialTextSink, star bool) error {
	if e := m.check(); e != nil {
		return e
	}
	return m.displayIntoCore(p, sink, star)
}

func (m *MultivariatePolynomialCore) displayIntoCore(p Polynomial, sink PolynomialTextSink, star bool) error {
	if sink == nil {
		return ErrPolynomialOwner
	}
	if e := polynomialDisplaySourceAssert(7426, "m_imp->consistent_coeffs(p)", func() (bool, error) {
		e := m.displayConsistent(p, sink)
		return e == nil, e
	}); e != nil {
		return e
	}
	return m.displayPolynomialDirect(p, sink, star)
}

// displayPolynomialDirect implements polynomial::display, without the public
// manager::display consistent_coeffs assertion.
func (m *MultivariatePolynomialCore) displayPolynomialDirect(p Polynomial, sink PolynomialTextSink, star bool) error {
	n, e := m.readWord(p, 8)
	if e != nil {
		return e
	}
	if n == 0 {
		return m.displayLiteral(sink, "0")
	}
	for i := uint32(0); ; i++ {
		n, e = m.readWord(p, 8)
		if e != nil {
			return e
		}
		if i >= n {
			return nil
		}
		if e = m.displayOrdinaryTerm(p, i, sink, star); e != nil {
			return e
		}
	}
}
func (m *MultivariatePolynomialCore) displayOrdinaryTerm(p Polynomial, i uint32, sink PolynomialTextSink, star bool) (err error) {
	e := m.displayBindCoefficient(p, i)
	if e != nil {
		return e
	}
	f, e := m.displayFrame(sink)
	if e != nil {
		return e
	}
	f.constructed = true
	defer func() { err = f.finish(err) }()
	a, e := m.displayCoefficient(p, i)
	if e != nil {
		return e
	}
	if e = f.numericCall(0, &a); e != nil {
		return e
	}
	if e = f.numericCall(1, nil); e != nil {
		return e
	}
	a, e = m.displayCoefficient(p, i)
	if e != nil {
		return e
	}
	negative, e := m.modular.IsNeg(&a)
	if e != nil {
		return e
	}
	if i > 0 {
		s := " + "
		if negative {
			s = " - "
		}
		if e = f.WriteLiteral(s); e != nil {
			return e
		}
	} else if negative {
		if e = f.WriteLiteral("- "); e != nil {
			return e
		}
	}
	mon, e := m.monomialCore(p, i)
	if e != nil {
		return e
	}
	sz, e := m.displayMonomialSize(mon)
	if e != nil {
		return e
	}
	if sz == 0 {
		v, e := f.stringInteger(&f.numeral)
		if e != nil {
			return e
		}
		return f.WriteLiteral(v)
	}
	one, e := m.modular.IsOne(&f.numeral)
	if e != nil {
		return e
	}
	if one {
		return m.monomials.DisplayInto(mon, f, star)
	}
	v, e := f.stringInteger(&f.numeral)
	if e != nil {
		return e
	}
	if e = f.WriteLiteral(v); e != nil {
		return e
	}
	s := " "
	if star {
		s = "*"
	}
	if e = f.WriteLiteral(s); e != nil {
		return e
	}
	mon, e = m.monomialCore(p, i)
	if e != nil {
		return e
	}
	return m.monomials.DisplayInto(mon, f, star)
}
func (m *MultivariatePolynomialCore) displayNumberSMT2(p Polynomial, i uint32, sink PolynomialTextSink) (err error) {
	a, e := m.displayCoefficient(p, i)
	if e != nil {
		return e
	}
	neg, e := m.modular.IsNeg(&a)
	if e != nil {
		return e
	}
	if neg {
		if e = m.displayLiteral(sink, "(- "); e != nil {
			return e
		}
	}
	f, e := m.displayFrame(sink)
	if e != nil {
		return e
	}
	defer func() { err = f.finish(err) }()
	if neg {
		f.constructed = true
		a, e = m.displayCoefficient(p, i)
		if e != nil {
			return e
		}
		if e = f.numericCall(0, &a); e != nil {
			return e
		}
		if e = f.numericCall(2, nil); e != nil {
			return e
		}
		if e = f.displayInteger(&f.numeral); e != nil {
			return e
		}
		return f.WriteLiteral(")")
	}
	return f.displayInteger(&a)
}
func (m *MultivariatePolynomialCore) displaySMT2Term(p Polynomial, i uint32, sink PolynomialTextSink) error {
	if e := polynomialDisplaySourceAssert(1573, "i < m_size", func() (bool, error) {
		n, e := m.readWord(p, 8)
		return i < n, e
	}); e != nil {
		return e
	}
	mon, e := m.monomialCore(p, i)
	if e != nil {
		return e
	}
	if e = m.displayBindCoefficient(p, i); e != nil {
		return e
	}
	sz, e := m.displayMonomialSize(mon)
	if e != nil {
		return e
	}
	if sz == 0 {
		return m.displayNumberSMT2(p, i, sink)
	}
	a, e := m.displayCoefficient(p, i)
	if e != nil {
		return e
	}
	one, e := m.modular.IsOne(&a)
	if e != nil {
		return e
	}
	if !one {
		if e = m.displayLiteral(sink, "(* "); e != nil {
			return e
		}
		if e = m.displayNumberSMT2(p, i, sink); e != nil {
			return e
		}
	}
	f, e := m.displayFrame(sink)
	if e != nil {
		return e
	}
	e = m.monomials.DisplayInto(mon, f, false)
	e = f.finish(e)
	if e != nil {
		return e
	}
	if !one {
		return m.displayLiteral(sink, ")")
	}
	return nil
}
func (m *MultivariatePolynomialCore) DisplaySMT2Into(p Polynomial, sink PolynomialTextSink) error {
	if e := m.check(); e != nil {
		return e
	}
	if sink == nil {
		return ErrPolynomialOwner
	}
	n, e := m.readWord(p, 8)
	if e != nil {
		return e
	}
	if n == 0 {
		return m.displayLiteral(sink, "0")
	}
	n, e = m.readWord(p, 8)
	if e != nil {
		return e
	}
	if n == 1 {
		return m.displaySMT2Term(p, 0, sink)
	}
	if e = m.displayLiteral(sink, "(+"); e != nil {
		return e
	}
	for i := uint32(0); ; i++ {
		n, e = m.readWord(p, 8)
		if e != nil {
			return e
		}
		if i >= n {
			break
		}
		if e = m.displayLiteral(sink, " "); e != nil {
			return e
		}
		if e = m.displaySMT2Term(p, i, sink); e != nil {
			return e
		}
	}
	return m.displayLiteral(sink, ")")
}
func (m *MultivariatePolynomialManager) DisplayInto(p Polynomial, sink PolynomialTextSink, star bool) error {
	if e := m.ready(); e != nil {
		return e
	}
	return m.core.DisplayInto(p, sink, star)
}
func (m *MultivariatePolynomialManager) DisplaySMT2Into(p Polynomial, sink PolynomialTextSink) error {
	if e := m.ready(); e != nil {
		return e
	}
	return m.core.DisplaySMT2Into(p, sink)
}
func (v *PolynomialRefVector) DisplayInto(sink PolynomialTextSink) error {
	if e := v.open(); e != nil {
		return e
	}
	n := uint32(len(v.state.values))
	for i := uint32(0); i < n; i++ {
		p, e := v.state.get(v.owner, i)
		if e != nil {
			return e
		}
		if e = v.owner.DisplayInto(p, sink, false); e != nil {
			return e
		}
		if e = v.owner.displayLiteral(sink, "\n"); e != nil {
			return e
		}
	}
	return nil
}

func displayDeallocatePublished(r *umemory.Runtime, b *umemory.Block) error {
	if *b == (umemory.Block{}) {
		return nil
	}
	return r.Deallocate(b)
}

func (f *polynomialDisplayFrame) stringInteger(a *unumeral.Int) (string, error) {
	if e := f.owner.modular.AssertNormalized(a); e != nil {
		return "", e
	}
	f.assembling = true
	f.assembly.Reset()
	e := f.displayInteger(a)
	f.assembling = false
	out := f.assembly.String()
	f.assembly.Reset()
	return out, e
}

func (f *polynomialDisplayFrame) observeYoungerStop() {
	if f.stopped {
		return
	}
	if y := f.younger; y != nil && y.generation == f.youngerGeneration && y.active && y.stopped {
		f.stopped = true
		f.sourceCause = y.sourceCause
	}
}

// Source a(i) forms a reference; numeral words are loaded only when the
// reached manager operation uses that reference. The codec has no native as
// pointer, so checked provenance substitutes for its lifetime guard.
func (m *MultivariatePolynomialCore) displayBindCoefficient(p Polynomial, i uint32) error {
	if polynomialSourceDebug {
		n, e := m.readWord(p, 8)
		if e != nil {
			return e
		}
		if i >= n {
			return ErrPolynomialAssertion
		}
	}
	_, e := m.polynomialLocation(p)
	return e
}
func (m *MultivariatePolynomialCore) displayCoefficient(p Polynomial, i uint32) (unumeral.Int, error) {
	l, e := m.polynomialLocation(p)
	if e != nil {
		return unumeral.Int{}, e
	}
	if i >= l.initializedCoefficients {
		return unumeral.Int{}, ErrPolynomialPartial
	}
	o := uint64(12) + 12*uint64(i)
	v, e := m.readWord(p, o)
	if e != nil {
		return unumeral.Int{}, e
	}
	r, e := m.read64(p, o+4)
	return unumeral.IntFromNativeWords([3]uint32{uint32(v), uint32(r), uint32(uint64(r) >> 32)}), e
}

func (m *MultivariatePolynomialCore) DisplayMonomialInto(a Monomial, sink MonomialTextSink, star bool) error {
	if e := m.check(); e != nil {
		return e
	}
	if sink == nil {
		return ErrPolynomialOwner
	}
	f, e := m.displayFrame(nil)
	if e != nil {
		return e
	}
	f.monomialSink = sink
	e = m.monomials.DisplayInto(a, f, star)
	return f.finish(e)
}
func (m *MultivariatePolynomialManager) DisplayMonomialInto(a Monomial, sink MonomialTextSink, star bool) error {
	if e := m.ready(); e != nil {
		return e
	}
	return m.core.DisplayMonomialInto(a, sink, star)
}
