package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1594-1602.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Concrete source overload dispatch: borrowed numerical parameters do not
// escape the call, and their fields are reread at the actual source sites.

func (m *UPolynomialManager) translate(v UPolynomialView, kind uint8, k uint32, z *unumeral.Int, b *unumeral.BinaryRational, q *unumeral.Rational) error {
	if e := m.check(); e != nil {
		return e
	}
	if v.count <= 1 {
		return nil
	}
	if kind == 3 {
		if b == nil {
			return ErrUPolynomialOwner
		}
		if e := m.ComposePowerTwoDegreeDivideXViewInPlace(v, b.K()); e != nil {
			return e
		}
	}
	if kind == 4 {
		if q == nil {
			return ErrUPolynomialOwner
		}
		if e := m.ComposeIntegerDegreeDivideXViewInPlace(v, &(*q.BorrowDenominator())); e != nil {
			return e
		}
	}
	var aux unumeral.Int
	if kind == 1 {
		defer m.modular.Del(&aux)
	}
	n := v.count - 1
	for i := uint32(1); i <= n; i++ {
		if e := m.checkpoint(); e != nil {
			return e
		}
		for index := n - i; index <= n-1; index++ {
			a, e := m.read(v, index)
			if e != nil {
				return e
			}
			next, e := m.read(v, index+1)
			if e != nil {
				return e
			}
			switch kind {
			case 0:
				e = m.modular.Add(a, next, a)
			case 1:
				// The C++ inner unsigned k shadows the shift argument.
				e = m.modular.Mul2k(next, index, &aux)
				if e == nil {
					e = m.modular.Add(a, &aux, a)
				}
			case 2:
				e = m.modular.AddMul(a, z, next, a)
			case 3:
				if index > n-i {
					e = m.modular.Mul2kInPlace(a, b.K())
				}
				if e == nil {
					e = m.modular.AddMul(a, &(*b.BorrowNumerator()), next, a)
				}
			case 4:
				if index > n-i {
					e = m.modular.Mul(a, &(*q.BorrowDenominator()), a)
				}
				if e == nil {
					e = m.modular.AddMul(a, &(*q.BorrowNumerator()), next, a)
				}
			}
			if e != nil {
				return e
			}
		}
		if kind == 3 || kind == 4 {
			top, e := m.read(v, n)
			if e != nil {
				return e
			}
			if kind == 3 {
				e = m.modular.Mul2kInPlace(top, b.K())
			} else {
				e = m.modular.Mul(top, &(*q.BorrowDenominator()), top)
			}
			if e != nil {
				return e
			}
		}
	}
	return nil
}
func (m *UPolynomialManager) TranslateOneViewInPlace(v UPolynomialView) error {
	return m.translate(v, 0, 0, nil, nil, nil)
}
func (m *UPolynomialManager) TranslatePowerTwoViewInPlace(v UPolynomialView, k uint32) error {
	return m.translate(v, 1, k, nil, nil, nil)
}
func (m *UPolynomialManager) TranslateIntegerViewInPlace(v UPolynomialView, c *unumeral.Int) error {
	return m.translate(v, 2, 0, c, nil, nil)
}
func (m *UPolynomialManager) TranslateBinaryViewInPlace(v UPolynomialView, b *unumeral.BinaryRational) error {
	return m.translate(v, 3, 0, nil, b, nil)
}
func (m *UPolynomialManager) TranslateRationalViewInPlace(v UPolynomialView, b *unumeral.Rational) error {
	return m.translate(v, 4, 0, nil, nil, b)
}
func (m *UPolynomialManager) translated(v UPolynomialView, out UPolynomial, kind uint8, k uint32, z *unumeral.Int, b *unumeral.BinaryRational, q *unumeral.Rational) error {
	d, e := m.descriptor(out)
	if e != nil {
		return e
	}
	if e = m.setView(&d.state, v); e != nil {
		return e
	}
	return m.translate(m.stateView(&d.state).WithCount(v.count), kind, k, z, b, q)
}
func (m *UPolynomialManager) TranslateOne(v UPolynomialView, out UPolynomial) error {
	return m.translated(v, out, 0, 0, nil, nil, nil)
}
func (m *UPolynomialManager) TranslatePowerTwo(v UPolynomialView, k uint32, out UPolynomial) error {
	return m.translated(v, out, 1, k, nil, nil, nil)
}
func (m *UPolynomialManager) TranslateInteger(v UPolynomialView, c *unumeral.Int, out UPolynomial) error {
	return m.translated(v, out, 2, 0, c, nil, nil)
}
func (m *UPolynomialManager) TranslateBinary(v UPolynomialView, b *unumeral.BinaryRational, out UPolynomial) error {
	return m.translated(v, out, 3, 0, nil, b, nil)
}
func (m *UPolynomialManager) TranslateRational(v UPolynomialView, b *unumeral.Rational, out UPolynomial) error {
	return m.translated(v, out, 4, 0, nil, nil, b)
}
func (m *UPolynomialManager) ComposeTwoDegreeDivideXViewInPlace(v UPolynomialView) error {
	if e := m.check(); e != nil {
		return e
	}
	if v.count <= 1 {
		return nil
	}
	k := v.count - 1
	for i := uint32(0); i < v.count-1; i++ {
		a, e := m.read(v, i)
		if e != nil {
			return e
		}
		if e = m.modular.Mul2kInPlace(a, k); e != nil {
			return e
		}
		k--
	}
	return nil
}
func (m *UPolynomialManager) ComposePowerTwoDegreeDivideXViewInPlace(v UPolynomialView, k uint32) error {
	if e := m.check(); e != nil {
		return e
	}
	if v.count <= 1 {
		return nil
	}
	ki := k * v.count
	for i := uint32(0); i < v.count; i++ {
		ki -= k
		a, e := m.read(v, i)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return e
		}
		if !zero {
			if e = m.modular.Mul2kInPlace(a, ki); e != nil {
				return e
			}
		}
	}
	return nil
}
func (m *UPolynomialManager) ComposePowerTwoXViewInPlace(v UPolynomialView, k uint32) error {
	if e := m.check(); e != nil {
		return e
	}
	if v.count <= 1 {
		return nil
	}
	ki := k
	for i := uint32(1); i < v.count; i++ {
		a, e := m.read(v, i)
		if e != nil {
			return e
		}
		if e = m.modular.Mul2kInPlace(a, ki); e != nil {
			return e
		}
		ki += k
	}
	return nil
}
func (m *UPolynomialManager) ComposeIntegerDegreeDivideXViewInPlace(v UPolynomialView, a *unumeral.Int) error {
	if e := m.check(); e != nil {
		return e
	}
	if v.count <= 1 {
		return nil
	}
	var ai unumeral.Int
	defer m.modular.Del(&ai)
	if e := m.modular.Copy(&ai, a); e != nil {
		return e
	}
	for i := v.count - 1; i > 0; {
		i--
		x, e := m.read(v, i)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(x)
		if e != nil {
			return e
		}
		if !zero {
			if e = m.modular.Mul(x, &ai, x); e != nil {
				return e
			}
		}
		if e = m.modular.Mul(&ai, a, &ai); e != nil {
			return e
		}
	}
	return nil
}
func (m *UPolynomialManager) ComposeIntegerXViewInPlace(v UPolynomialView, b *unumeral.Int) error {
	if e := m.check(); e != nil {
		return e
	}
	if v.count <= 1 {
		return nil
	}
	var bi unumeral.Int
	defer m.modular.Del(&bi)
	if e := m.modular.Copy(&bi, b); e != nil {
		return e
	}
	for i := uint32(1); i < v.count; i++ {
		a, e := m.read(v, i)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return e
		}
		if !zero {
			if e = m.modular.Mul(a, &bi, a); e != nil {
				return e
			}
		}
		if e = m.modular.Mul(&bi, b, &bi); e != nil {
			return e
		}
	}
	return nil
}
func (m *UPolynomialManager) ComposeBinaryXViewInPlace(v UPolynomialView, b *unumeral.BinaryRational) error {
	if e := m.check(); e != nil {
		return e
	}
	if v.count <= 1 {
		return nil
	}
	if b == nil {
		return ErrUPolynomialOwner
	}
	var ci unumeral.Int
	defer m.modular.Del(&ci)
	if e := m.modular.SetInt32(&ci, 1); e != nil {
		return e
	}
	k := b.K()
	ki := k * v.count
	for i := uint32(0); i < v.count; i++ {
		ki -= k
		a, e := m.read(v, i)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return e
		}
		if !zero {
			if e = m.modular.Mul2kInPlace(a, ki); e != nil {
				return e
			}
			if e = m.modular.Mul(a, &ci, a); e != nil {
				return e
			}
		}
		if e = m.modular.Mul(&ci, &(*b.BorrowNumerator()), &ci); e != nil {
			return e
		}
	}
	return m.binaryCompositionExponentAssertion(ki)
}
func (m *UPolynomialManager) ComposeRationalXViewInPlace(v UPolynomialView, q *unumeral.Rational) error {
	if e := m.check(); e != nil {
		return e
	}
	if v.count <= 1 {
		return nil
	}
	if q == nil {
		return ErrUPolynomialOwner
	}
	var bc unumeral.Int
	defer m.modular.Del(&bc)
	if e := m.modular.Power(&(*q.BorrowDenominator()), v.count-1, &bc); e != nil {
		return e
	}
	for i := uint32(0); i < v.count; i++ {
		a, e := m.read(v, i)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return e
		}
		if !zero {
			if e = m.modular.Mul(a, &bc, a); e != nil {
				return e
			}
		}
		if i < v.count-1 {
			if e = m.modular.Div(&bc, &(*q.BorrowDenominator()), &bc); e != nil {
				return e
			}
			if e = m.modular.Mul(&bc, &(*q.BorrowNumerator()), &bc); e != nil {
				return e
			}
		}
	}
	return nil
}
func (m *UPolynomialManager) PMinusXViewInPlace(v UPolynomialView) (cause error) {
	if e := m.check(); e != nil {
		return e
	}
	q, e := m.beginKnuth(v, 3)
	if e != nil {
		return e
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
	q.stage = 1
	return m.pMinusXWithSourceContext(v, q.context())
}
func (m *UPolynomialManager) pMinusXWithSourceContext(v UPolynomialView, context integerAllocationInvocationContext) error {
	if context.knuth != nil {
		q := context.knuthFrame(m.config.Integer)
		if q == nil || q.owner != m || v.origin != q.target.origin || v.backing != q.target.backing || v.generation != q.target.generation || v.start < q.target.start || uint64(v.start)+uint64(v.count) > uint64(q.target.start)+uint64(q.target.count) {
			return ErrUPolynomialOwner
		}
	}

	if e := m.check(); e != nil {
		return e
	}
	for i := uint32(0); i < v.count; i++ {
		a, e := m.read(v, i)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return e
		}
		if zero || i%2 == 0 {
			continue
		}
		if q := context.knuthFrame(m.config.Integer); q != nil {
			q.cursor = i
		}
		if e = m.modular.WithAllocationContext((context).utility(m.modular.Integers())).Neg(a); e != nil {
			return e
		}
	}
	return nil
}
func (m *UPolynomialManager) ReciprocalViewInPlace(v UPolynomialView) error {
	if e := m.check(); e != nil {
		return e
	}
	if v.count <= 1 {
		return nil
	}
	i, j := uint32(0), v.count-1
	if e := m.reciprocalIndexAssertion(j); e != nil {
		return e
	}
	for {
		a, e := m.read(v, i)
		if e != nil {
			return e
		}
		b, e := m.read(v, j)
		if e != nil {
			return e
		}
		*a, *b = *b, *a
		i++
		j--
		if i >= j {
			return nil
		}
	}
}
