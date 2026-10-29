package polynomial

// Source: src/math/polynomial/polynomial.cpp:2807-2952.

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialNewtonScope struct {
	refsConstructed, cConstructed bool
	kind, stage                   uint8
	product, aux, c               unumeral.Int
	temp, auxPoly, u              polynomialReference
	body, cleanup                 error
}

// PolynomialNewtonInterpolator is the real modular incremental Newton algorithm.
// Its heap owner is a charged Go adapter for the source stack or heap object.
type PolynomialNewtonInterpolator struct {
	self                   *PolynomialNewtonInterpolator
	owner                  *MultivariatePolynomialCore
	inputs, invs           []unumeral.Int
	vs                     polynomialReferenceVector
	scope                  polynomialNewtonScope
	resetCause             error
	header                 uint64
	resetStage, closeStage uint8
	inputDrain, invDrain   uint32
	closing, closed        bool
}

func NewPolynomialNewtonInterpolator(m *MultivariatePolynomialCore) (*PolynomialNewtonInterpolator, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	h := uint64(reflect.TypeOf(PolynomialNewtonInterpolator{}).Size())
	if e := m.integer.Budget().Reserve(h); e != nil {
		return nil, e
	}
	n := &PolynomialNewtonInterpolator{owner: m, header: h, invs: []unumeral.Int{{}}}
	n.self = n
	return n, nil
}
func (n *PolynomialNewtonInterpolator) open() error {
	if n == nil || n.self != n {
		return ErrPolynomialOwner
	}
	if n.closed || n.closing {
		return arena.ErrClosed
	}
	if n.scope.kind != 0 || n.resetStage != 0 {
		return ErrPolynomialPending
	}
	return n.owner.check()
}
func (n *PolynomialNewtonInterpolator) NumSamplePoints() (uint32, error) {
	if n == nil || n.self != n {
		return 0, ErrPolynomialOwner
	}
	if n.closed {
		return 0, arena.ErrClosed
	}
	return uint32(len(n.inputs)), nil
}

type PolynomialNewtonInputs struct{ owner *PolynomialNewtonInterpolator }

func (n *PolynomialNewtonInterpolator) Inputs() PolynomialNewtonInputs {
	return PolynomialNewtonInputs{owner: n}
}
func (v PolynomialNewtonInputs) Size() (uint32, error)              { return v.owner.NumSamplePoints() }
func (v PolynomialNewtonInputs) Get(i uint32) (unumeral.Int, error) { return v.owner.Input(i) }

func (n *PolynomialNewtonInterpolator) Input(i uint32) (unumeral.Int, error) {
	if n == nil || n.self != n {
		return unumeral.Int{}, ErrPolynomialOwner
	}
	if n.closed || n.closing {
		return unumeral.Int{}, arena.ErrClosed
	}
	if e := n.owner.check(); e != nil {
		return unumeral.Int{}, e
	}
	if e := polynomialNewtonSourceAssertAt("src/util/vector.h", 309, "idx < size()", func() (bool, error) {
		return uint64(i) < uint64(len(n.inputs)), nil
	}); e != nil {
		return unumeral.Int{}, e
	}
	if uint64(i) >= uint64(len(n.inputs)) {
		return unumeral.Int{}, arena.ErrBounds
	}
	return n.inputs[i], nil
}
func (n *PolynomialNewtonInterpolator) pushNumeral(v *[]unumeral.Int, a *unumeral.Int) error {
	// Publish a constructed zero numeral before SET; a failed numeric copy
	// leaves its actual acquired resources in this owning entry.
	*v = append(*v, unumeral.Int{})
	return n.owner.modular.Copy(&(*v)[len(*v)-1], a)
}
func (n *PolynomialNewtonInterpolator) destroyNumerals(v *[]unumeral.Int, target uint32, cursor *uint32) error {
	if *cursor < target {
		*cursor = target
	}
	for uint64(*cursor) < uint64(len(*v)) {
		if e := n.owner.modular.Del(&(*v)[*cursor]); e != nil {
			return e
		}
		*cursor++
	}
	*v = (*v)[:target]
	*cursor = 0
	return nil
}
func (n *PolynomialNewtonInterpolator) scopeCleanup(owning, abort bool) error {
	s := &n.scope
	m := n.owner
	for {
		if s.kind == 1 {
			switch s.stage {
			case 0:
				if s.refsConstructed {
					if e := s.auxPoly.closeValue(m, owning); e != nil {
						return e
					}
				}
			case 1:
				if s.refsConstructed {
					if e := s.temp.closeValue(m, owning); e != nil {
						return e
					}
				}
			case 2:
				if e := m.modular.Del(&s.aux); e != nil {
					return e
				}
			case 3:
				if e := m.modular.Del(&s.product); e != nil {
					return e
				}
			default:
				if !abort && (s.auxPoly.assigning || s.temp.assigning) {
					return nil
				}
				if e := s.auxPoly.abort(m); e != nil {
					return e
				}
				if e := s.temp.abort(m); e != nil {
					return e
				}
				*s = polynomialNewtonScope{}
				return nil
			}
		} else if s.kind == 2 {
			switch s.stage {
			case 0:
				if s.cConstructed {
					if e := m.modular.Del(&s.c); e != nil {
						return e
					}
				}
			case 1:
				if e := s.auxPoly.closeValue(m, owning); e != nil {
					return e
				}
			case 2:
				if e := s.u.closeValue(m, owning); e != nil {
					return e
				}
			default:
				if !abort && (s.auxPoly.assigning || s.u.assigning) {
					return nil
				}
				if e := s.auxPoly.abort(m); e != nil {
					return e
				}
				if e := s.u.abort(m); e != nil {
					return e
				}
				*s = polynomialNewtonScope{}
				return nil
			}
		} else {
			return nil
		}
		s.stage++
	}
}
func (n *PolynomialNewtonInterpolator) finish(e error) error {
	n.scope.body = e
	d := n.scopeCleanup(false, false)
	if d != nil {
		n.scope.cleanup = d
	}
	return polynomialNewtonJoin(e, d)
}
func (n *PolynomialNewtonInterpolator) Add(input *unumeral.Int, output Polynomial) (err error) {
	if e := n.open(); e != nil {
		return e
	}
	m := n.owner
	if e := n.traceAddEntry(input, output); e != nil {
		return e
	}
	if e := polynomialNewtonScalarAssert(2834, "m().modular()", !m.modular.IntegerMode()); e != nil {
		return e
	}
	sz := uint32(len(n.inputs))
	var e error
	if sz == 0 {
		if e = n.pushNumeral(&n.inputs, input); e != nil {
			return e
		}
		return n.vs.push(m, output)
	}
	n.scope = polynomialNewtonScope{kind: 1}
	defer func() { err = n.finish(err) }()
	s := &n.scope
	first := n.inputs[0]
	if e := polynomialNewtonSourceAssert(2841, "!m().eq(input, m_inputs[0])", func() (bool, error) {
		same, x := m.modular.Equal(input, &first)
		return !same, x
	}); e != nil {
		return e
	}
	if e = m.modular.Sub(input, &first, &s.product); e != nil {
		return e
	}
	for i := uint32(1); i < sz; i++ {
		a := n.inputs[i]
		if e = polynomialNewtonSourceAssert(2844, "!m().eq(input, m_inputs[i])", func() (bool, error) {
			same, x := m.modular.Equal(input, &a)
			return !same, x
		}); e != nil {
			return e
		}
		if e = m.modular.Sub(input, &a, &s.aux); e != nil {
			return e
		}
		if e = m.modular.Mul(&s.product, &s.aux, &s.product); e != nil {
			return e
		}
	}
	if e = m.modular.Inv(&s.product); e != nil {
		return e
	}
	if e = n.pushNumeral(&n.inputs, input); e != nil {
		return e
	}
	if e = n.pushNumeral(&n.invs, &s.product); e != nil {
		return e
	}
	if e = n.traceNumeral(2851, "add", newtonIndexLabel("invs", sz), &s.product); e != nil {
		return e
	}
	if e = polynomialNewtonSourceAssert(2852, "m().eq(m_inputs[k], input)", func() (bool, error) {
		a := n.inputs[sz]
		return m.modular.Equal(&a, input)
	}); e != nil {
		return e
	}
	s.refsConstructed = true
	p, e := n.vs.get(m, sz-1)
	if e != nil {
		return e
	}
	if e = s.temp.assign(m, p); e != nil {
		return e
	}
	for j := int32(sz - 2); j >= 0; j-- {
		a := n.inputs[uint32(j)]
		if e = m.modular.Sub(input, &a, &s.aux); e != nil {
			return e
		}
		if e = n.assertNumeralNormalized(&s.aux, 2860, "m().is_p_normalized(aux)"); e != nil {
			return e
		}
		p, e = n.scale(&s.aux, s.temp.value)
		if e != nil {
			return e
		}
		if e = s.auxPoly.assign(m, p); e != nil {
			return e
		}
		v, e := n.vs.get(m, uint32(j))
		if e != nil {
			return e
		}
		p, e = m.Add(s.auxPoly.value, v)
		if e != nil {
			return e
		}
		if e = s.temp.assign(m, p); e != nil {
			return e
		}
		if e = n.assertPolynomialNormalized(s.temp.value, 2863, "pm.is_p_normalized(temp)"); e != nil {
			return e
		}
	}
	p, e = m.Sub(output, s.temp.value)
	if e != nil {
		return e
	}
	if e = s.auxPoly.assign(m, p); e != nil {
		return e
	}
	if e = n.assertPolynomialNormalized(s.auxPoly.value, 2867, "pm.is_p_normalized(aux_poly)"); e != nil {
		return e
	}
	a := n.invs[sz]
	p, e = n.scale(&a, s.auxPoly.value)
	if e != nil {
		return e
	}
	if e = s.auxPoly.assign(m, p); e != nil {
		return e
	}
	if e = n.assertPolynomialNormalized(s.auxPoly.value, 2869, "pm.is_p_normalized(aux_poly)"); e != nil {
		return e
	}
	if e = n.vs.push(m, s.auxPoly.value); e != nil {
		return e
	}
	return n.tracePolynomial(2871, "add", newtonIndexLabel("vs", sz), s.auxPoly.value)
}
func (n *PolynomialNewtonInterpolator) scale(a *unumeral.Int, p Polynomial) (Polynomial, error) {
	u, e := n.owner.monomials.Unit()
	if e != nil {
		return Polynomial{}, e
	}
	return n.owner.Scale(a, u, p)
}
func (n *PolynomialNewtonInterpolator) Mk(x uint32, r *PolynomialRef) (err error) {
	if e := n.open(); e != nil {
		return e
	}
	if r == nil || r.owner != n.owner {
		return ErrPolynomialOwner
	}
	if e := r.open(); e != nil {
		return e
	}
	m := n.owner
	if e := polynomialNewtonScalarAssert(2881, "m().modular()", !m.modular.IntegerMode()); e != nil {
		return e
	}
	n.scope = polynomialNewtonScope{kind: 2, refsConstructed: true}
	defer func() { err = n.finish(err) }()
	s := &n.scope
	rawNum := uint32(len(n.inputs))
	num := int32(rawNum)
	if num == math.MinInt32 {
		return arena.ErrBounds
	} // Reached signed d=num-1 UB domain.
	d := num - 1
	if e := polynomialNewtonScalarAssert(2886, "num > 0", num > 0); e != nil {
		return e
	}
	p, e := n.vs.get(m, uint32(d))
	if e != nil {
		return e
	}
	if e = s.u.assign(m, p); e != nil {
		return e
	}
	s.c = unumeral.Int{}
	s.cConstructed = true
	if d == math.MinInt32 {
		return arena.ErrBounds
	}
	for k := d - 1; k >= 0; k-- {
		if e = n.tracePolynomial(2890, "mk", "u: ", s.u.value); e != nil {
			return e
		}
		a := n.inputs[uint32(k)]
		if e = m.modular.Copy(&s.c, &a); e != nil {
			return e
		}
		if e = m.modular.Neg(&s.c); e != nil {
			return e
		}
		one := [1]unumeral.Int{unumeral.NewInt(1)}
		p, e = m.MkLinearTake(one[:], []uint32{x}, &s.c)
		if e != nil {
			return e
		}
		if e = s.auxPoly.assign(m, p); e != nil {
			return e
		}
		if e = n.tracePolynomial(2896, "mk", "(x - inputs[k]): ", s.auxPoly.value); e != nil {
			return e
		}
		p, e = m.Mul(s.u.value, s.auxPoly.value)
		if e != nil {
			return e
		}
		if e = s.auxPoly.assign(m, p); e != nil {
			return e
		}
		aPoly, e := n.vs.get(m, uint32(k))
		if e != nil {
			return e
		}
		p, e = m.Add(s.auxPoly.value, aPoly)
		if e != nil {
			return e
		}
		if e = s.u.assign(m, p); e != nil {
			return e
		}
	}
	if e = n.tracePolynomial(2901, "mk", "result u: ", s.u.value); e != nil {
		return e
	}
	return r.Assign(s.u.value)
}
func (n *PolynomialNewtonInterpolator) Reset() (err error) {
	if n != nil && n.self == n && n.resetCause != nil {
		return n.resetCause
	}
	defer func() {
		if err != nil && n != nil && n.self == n && n.resetStage != 0 {
			n.resetCause = err
		}
	}()
	if n == nil || n.self != n {
		return ErrPolynomialOwner
	}
	if n.closed || n.closing {
		return arena.ErrClosed
	}
	if n.scope.kind != 0 {
		return ErrPolynomialPending
	}
	if n.resetStage == 0 {
		n.resetStage = 1
	}
	if n.resetStage == 1 {
		if e := n.destroyNumerals(&n.inputs, 0, &n.inputDrain); e != nil {
			return e
		}
		n.resetStage = 2
	}
	if n.resetStage == 2 {
		if e := n.destroyNumerals(&n.invs, 1, &n.invDrain); e != nil {
			return e
		}
		n.resetStage = 3
	}
	if n.resetStage == 3 {
		if e := n.vs.reset(n.owner); e != nil {
			return e
		}
		n.resetStage = 4
	}
	if e := polynomialNewtonSourceAssert(2822, "m().is_zero(m_invs[0])", func() (bool, error) {
		a := n.invs[0]
		return n.owner.modular.IsZero(&a)
	}); e != nil {
		return e
	}
	n.resetStage = 0
	return nil
}

// sourceClose is the outer lexical boundary. An actual stopped younger
// destructor prevents reaching class member destruction. Owning acknowledgement
// belongs to the caller's later Close/CleanupNewtonInterpolation operation.
func (n *PolynomialNewtonInterpolator) sourceClose() error {
	if n.scope.cleanup != nil {
		return n.scope.cleanup
	}
	return n.closeMode(false, false)
}

func (n *PolynomialNewtonInterpolator) Close() error { return n.closeMode(true, true) }
func (n *PolynomialNewtonInterpolator) closeMode(owning, abort bool) error {
	if n == nil || n.self != n {
		return ErrPolynomialOwner
	}
	if n.closed {
		return nil
	}
	n.closing = true
	if e := n.scopeCleanup(owning, false); e != nil {
		return e
	}
	if n.closeStage == 0 {
		if e := n.vs.resetMode(n.owner, owning); e != nil {
			return e
		}
		n.vs.values = nil
		n.closeStage++
	}
	if n.closeStage == 1 {
		if e := n.destroyNumerals(&n.invs, 0, &n.invDrain); e != nil {
			return e
		}
		n.invs = nil
		n.closeStage++
	}
	if n.closeStage == 2 {
		if e := n.destroyNumerals(&n.inputs, 0, &n.inputDrain); e != nil {
			return e
		}
		n.inputs = nil
		n.closeStage++
	}
	// Actual pending numeric/scope resources require separate owning abort
	// after all completed class member destructors.
	if !abort && n.scope.kind != 0 {
		return nil
	}
	if e := n.scopeCleanup(owning, true); e != nil {
		return e
	}
	n.owner.integer.Budget().Release(n.header)
	n.header = 0
	n.closed = true
	return nil
}

func (n *PolynomialNewtonInterpolator) assertNumeralNormalized(a *unumeral.Int, line int32, expression string) error {
	return polynomialNewtonSourceAssert(line, expression, func() (bool, error) {
		return n.owner.modular.IsPNormalized(a)
	})
}
func (n *PolynomialNewtonInterpolator) assertPolynomialNormalized(p Polynomial, outerLine int32, outerExpression string) error {
	return polynomialNewtonSourceAssert(outerLine, outerExpression, func() (bool, error) {
		count, e := n.owner.readWord(p, 8)
		if e != nil {
			return false, e
		}
		for i := uint32(0); i < count; i++ {
			a, e := n.owner.coefficientCore(p, i)
			if e != nil {
				return false, e
			}
			if e = n.assertNumeralNormalized(&a, 2792, "m().is_p_normalized(p->a(i))"); e != nil {
				return false, e
			}
		}
		return true, nil
	})
}

// NewtonInterpolation preserves the actual method-local interpolator until its
// reached member destruction succeeds; its field is not reusable warm scratch.
func (m *MultivariatePolynomialCore) NewtonInterpolation(x, d uint32, inputs []unumeral.Int, outputs []Polynomial, r *PolynomialRef) (err error) {
	if e := m.check(); e != nil {
		return e
	}
	if m.newtonLocal != nil {
		return ErrPolynomialPending
	}
	if e := polynomialNewtonScalarAssert(2916, "m().modular()", !m.modular.IntegerMode()); e != nil {
		return e
	}
	n, e := NewPolynomialNewtonInterpolator(m)
	if e != nil {
		return e
	}
	m.newtonLocal = n
	defer func() {
		c := n.sourceClose()
		if n.closed {
			m.newtonLocal = nil
		}
		err = polynomialNewtonJoin(err, c)
	}()
	for i := uint32(0); i <= d; i++ {
		if uint64(i) >= uint64(len(inputs)) || uint64(i) >= uint64(len(outputs)) {
			return arena.ErrBounds
		}
		if e = n.Add(&inputs[i], outputs[i]); e != nil {
			return e
		}
		if i == ^uint32(0) {
			return arena.ErrBounds
		}
	}
	return n.Mk(x, r)
}
func (m *MultivariatePolynomialCore) CleanupNewtonInterpolation() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.newtonLocal == nil {
		return nil
	}
	if e := m.newtonLocal.Close(); e != nil {
		return e
	}
	m.newtonLocal = nil
	return nil
}
