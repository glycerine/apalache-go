package arith

// Source: src/tactic/arith/linear_equation.h:29-79;
// src/tactic/arith/linear_equation.cpp:25-58,90-205,273-280.

import (
	"errors"
	"io"
	"sort"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

const noEquationPosition = ^uint32(0)

// LinearEquation owns its exact integer coefficients and borrows its manager.
// Variable IDs and approximate coefficients use native Go slices.
type LinearEquation struct {
	manager    *numeral.RationalManager
	as         []numeral.Int
	approx     []float64
	xs         []uint32
	closeIndex int
	closed     bool
}

func (e *LinearEquation) Size() uint32 {
	if e == nil || e.closed {
		return 0
	}
	return uint32(len(e.xs))
}

func (e *LinearEquation) Pos(x uint32) uint32 {
	if e == nil || e.closed {
		return noEquationPosition
	}
	i := sort.Search(len(e.xs), func(i int) bool { return e.xs[i] >= x })
	if i == len(e.xs) || e.xs[i] != x {
		return noEquationPosition
	}
	return uint32(i)
}

func (e *LinearEquation) Var(i uint32) (uint32, error) {
	if e == nil || e.closed {
		return 0, arena.ErrClosed
	}
	if i >= uint32(len(e.xs)) {
		return 0, arena.ErrBounds
	}
	return e.xs[i], nil
}

func (e *LinearEquation) Approx(i uint32) (float64, error) {
	if e == nil || e.closed {
		return 0, arena.ErrClosed
	}
	if i >= uint32(len(e.approx)) {
		return 0, arena.ErrBounds
	}
	return e.approx[i], nil
}

// CopyCoefficient makes the caller own an independent numeral, matching the
// source get_a(mpz&) copy path. The equation keeps its own coefficient.
func (e *LinearEquation) CopyCoefficient(i uint32, out *numeral.Int) error {
	if e == nil || e.closed {
		return arena.ErrClosed
	}
	if i >= uint32(len(e.as)) {
		return arena.ErrBounds
	}
	return e.manager.Integers().Copy(out, &e.as[i])
}

func (e *LinearEquation) Close() error {
	if e == nil || e.closed {
		return nil
	}
	z := e.manager.Integers()
	for e.closeIndex < len(e.as) {
		if err := z.Del(&e.as[e.closeIndex]); err != nil {
			return err
		}
		e.closeIndex++
	}
	e.as, e.approx, e.xs = nil, nil, nil
	e.closed = true
	return nil
}

// LinearEquationManager borrows the rational numeral manager and owns two
// concrete integer numeral buffers used by the reached arithmetic tactic.
type LinearEquationManager struct {
	self      *LinearEquationManager
	manager   *numeral.RationalManager
	trace     *diagnostic.TraceTags
	intBuffer *numeral.ScopedIntVector
	valBuffer *numeral.ScopedIntVector
	mark      []bool
	varBuffer []uint32
	closed    bool
}

func NewLinearEquationManager(m *numeral.RationalManager) (*LinearEquationManager, error) {
	if m == nil {
		return nil, arena.ErrInvalidSlot
	}
	if e := m.CheckOwnerReady(); e != nil {
		return nil, e
	}
	ib, e := numeral.NewScopedIntVector(m.Integers())
	if e != nil {
		return nil, e
	}
	vb, e := numeral.NewScopedIntVector(m.Integers())
	if e != nil {
		_ = ib.Close()
		return nil, e
	}
	l := &LinearEquationManager{manager: m, intBuffer: ib, valBuffer: vb}
	l.self = l
	return l, nil
}

func (l *LinearEquationManager) open() error {
	if l == nil || l.self != l || l.manager == nil {
		return arena.ErrInvalidSlot
	}
	if l.closed {
		return arena.ErrClosed
	}
	return l.manager.CheckOwnerReady()
}

// MkInt follows the source mutable-input constructor. In the non-normalized
// branch it combines repeated IDs, drops zero sums and refuses rows of size
// zero or one. The normalized branch trusts the caller's unique-ID contract.
func (l *LinearEquationManager) MkInt(as []numeral.Int, xs []uint32, normalized bool) (*LinearEquation, error) {
	if e := l.open(); e != nil {
		return nil, e
	}
	if len(as) != len(xs) {
		return nil, arena.ErrInvalidSlot
	}
	z := l.manager.Integers()
	size := len(xs)
	if !normalized {
		for i, x := range xs {
			if x == noEquationPosition || uint64(x)+1 > uint64(int(^uint(0)>>1)) {
				return nil, arena.ErrBounds
			}
			if int(x) >= len(l.mark) {
				l.mark = append(l.mark, make([]bool, int(x)+1-len(l.mark))...)
			}
			if e := l.valBuffer.Reserve(x + 1); e != nil {
				return nil, e
			}
			slot, e := l.valBuffer.BorrowSlot(x)
			if e != nil {
				return nil, e
			}
			if l.mark[x] {
				if e = z.Add(slot, &as[i], slot); e != nil {
					return nil, e
				}
			} else {
				if e = z.Copy(slot, &as[i]); e != nil {
					return nil, e
				}
				l.mark[x] = true
			}
		}
		j := 0
		for _, x := range xs {
			if !l.mark[x] {
				continue
			}
			slot, e := l.valBuffer.BorrowSlot(x)
			if e != nil {
				return nil, e
			}
			zero, e := z.IsZero(slot)
			if e != nil {
				return nil, e
			}
			if !zero {
				xs[j] = x
				if e = z.Copy(&as[j], slot); e != nil {
					return nil, e
				}
				j++
			}
			l.mark[x] = false
		}
		size = j
		if size <= 1 {
			return nil, nil
		}
	} else if e := l.assertUniqueNormalizedEquationVars(xs); e != nil {
		return nil, e
	}
	for i := 0; i < size; i++ {
		x := xs[i]
		if x == noEquationPosition {
			return nil, arena.ErrBounds
		}
		if e := l.valBuffer.Reserve(x + 1); e != nil {
			return nil, e
		}
		slot, e := l.valBuffer.BorrowSlot(x)
		if e != nil {
			return nil, e
		}
		if e = z.Swap(slot, &as[i]); e != nil {
			return nil, e
		}
	}
	sort.Slice(xs[:size], func(i, j int) bool { return xs[i] < xs[j] })
	for i := 0; i < size; i++ {
		slot, e := l.valBuffer.BorrowSlot(xs[i])
		if e != nil {
			return nil, e
		}
		if e = z.Swap(&as[i], slot); e != nil {
			return nil, e
		}
	}
	return l.mkCore(as[:size], xs[:size])
}

// MkRational clears denominators in the caller's mutable coefficients, then
// delegates to the exact integer constructor as the source overload does.
func (l *LinearEquationManager) MkRational(as []numeral.Rational, xs []uint32, normalized bool) (out *LinearEquation, err error) {
	if err = l.open(); err != nil {
		return nil, err
	}
	if len(as) != len(xs) || len(as) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if err = l.assertRationalEquationArity(len(as)); err != nil {
		return nil, err
	}
	z := l.manager.Integers()
	lcm, denominator := numeral.NewInt(0), numeral.NewInt(0)
	defer func() { err = errors.Join(err, z.Del(&denominator), z.Del(&lcm)) }()
	if err = l.manager.Denominator(&as[0], &lcm); err != nil {
		return nil, err
	}
	for i := 1; i < len(as); i++ {
		if err = l.manager.Denominator(&as[i], &denominator); err != nil {
			return nil, err
		}
		if err = z.LCM(&denominator, &lcm, &lcm); err != nil {
			return nil, err
		}
	}
	if err = l.traceEquationLCM(&lcm); err != nil {
		return nil, err
	}
	if err = l.intBuffer.Reset(); err != nil {
		return nil, err
	}
	for i := range as {
		if err = l.traceEquationRational(i, &as[i], false); err != nil {
			return nil, err
		}
		if err = l.manager.MulInt(&lcm, &as[i], &as[i]); err != nil {
			return nil, err
		}
		if err = l.traceEquationRational(i, &as[i], true); err != nil {
			return nil, err
		}
		if err = l.intBuffer.Push(&as[i].Num); err != nil {
			return nil, err
		}
	}
	values, err := l.intBuffer.BorrowedValues()
	if err != nil {
		return nil, err
	}
	return l.MkInt(values, xs, normalized)
}

// Combine constructs b1*eq1+b2*eq2, omitting exact cancellations. The
// reusable buffer owns each intermediate until the next call or Close.
func (l *LinearEquationManager) Combine(b1 *numeral.Int, eq1 *LinearEquation, b2 *numeral.Int, eq2 *LinearEquation) (out *LinearEquation, err error) {
	if err = l.open(); err != nil {
		return nil, err
	}
	if eq1 == nil || eq2 == nil || eq1.closed || eq2.closed || eq1.manager != l.manager || eq2.manager != l.manager {
		return nil, arena.ErrInvalidSlot
	}
	if b1 == nil || b2 == nil {
		return nil, arena.ErrInvalidSlot
	}
	if err = l.assertNonzeroEquationMultiplier(b1, 209, "!m.is_zero(b1)"); err != nil {
		return nil, err
	}
	if err = l.assertNonzeroEquationMultiplier(b2, 210, "!m.is_zero(b2)"); err != nil {
		return nil, err
	}
	z := l.manager.Integers()
	if err = l.intBuffer.Reset(); err != nil {
		return nil, err
	}
	l.varBuffer = l.varBuffer[:0]
	appendScaled := func(a *numeral.Int, b *numeral.Int, x uint32) error {
		if e := l.intBuffer.Push(a); e != nil {
			return e
		}
		slot, e := l.intBuffer.BorrowSlot(l.intBuffer.Size() - 1)
		if e != nil {
			return e
		}
		if e = z.Mul(slot, b, slot); e != nil {
			return e
		}
		l.varBuffer = append(l.varBuffer, x)
		return nil
	}
	tmp, newA := numeral.NewInt(0), numeral.NewInt(0)
	defer func() { err = errors.Join(err, z.Del(&newA), z.Del(&tmp)) }()
	i, j := 0, 0
	for i < len(eq1.xs) || j < len(eq2.xs) {
		if j == len(eq2.xs) || (i < len(eq1.xs) && eq1.xs[i] < eq2.xs[j]) {
			if err = appendScaled(&eq1.as[i], b1, eq1.xs[i]); err != nil {
				return nil, err
			}
			i++
		} else if i == len(eq1.xs) || eq2.xs[j] < eq1.xs[i] {
			if err = appendScaled(&eq2.as[j], b2, eq2.xs[j]); err != nil {
				return nil, err
			}
			j++
		} else {
			if err = z.Mul(&eq1.as[i], b1, &tmp); err != nil {
				return nil, err
			}
			if err = z.AddMul(&tmp, b2, &eq2.as[j], &newA); err != nil {
				return nil, err
			}
			zero, e := z.IsZero(&newA)
			if e != nil {
				return nil, e
			}
			if !zero {
				if err = l.intBuffer.Push(&newA); err != nil {
					return nil, err
				}
				l.varBuffer = append(l.varBuffer, eq1.xs[i])
			}
			i++
			j++
		}
	}
	if l.intBuffer.Size() == 0 {
		return nil, nil
	}
	values, err := l.intBuffer.BorrowedValues()
	if err != nil {
		return nil, err
	}
	return l.mkCore(values, l.varBuffer)
}

func (l *LinearEquationManager) mkCore(as []numeral.Int, xs []uint32) (out *LinearEquation, err error) {
	if len(as) == 0 || len(as) != len(xs) {
		return nil, arena.ErrInvalidSlot
	}
	if err = l.assertSortedEquationVars(xs); err != nil {
		return nil, err
	}
	z := l.manager.Integers()
	if err = l.traceEquationCore(as, xs, nil); err != nil {
		return nil, err
	}
	g := numeral.NewInt(0)
	defer func() { err = errors.Join(err, z.Del(&g)) }()
	if err = z.Copy(&g, &as[0]); err != nil {
		return nil, err
	}
	for i := 1; i < len(as); i++ {
		one, e := z.IsOne(&g)
		if e != nil {
			return nil, e
		}
		if one {
			break
		}
		negative, e := z.IsNeg(&as[i])
		if e != nil {
			return nil, e
		}
		if negative {
			if e = z.Neg(&as[i]); e != nil {
				return nil, e
			}
		}
		e = z.GCD(&g, &as[i], &g)
		if negative {
			e = errors.Join(e, z.Neg(&as[i]))
		}
		if e != nil {
			return nil, e
		}
	}
	one, e := z.IsOne(&g)
	if e != nil {
		return nil, e
	}
	if !one {
		for i := range as {
			if e = z.Div(&as[i], &g, &as[i]); e != nil {
				return nil, e
			}
		}
	}
	if err = l.traceEquationCore(as, xs, &g); err != nil {
		return nil, err
	}
	eq := &LinearEquation{manager: l.manager, as: make([]numeral.Int, len(as)),
		approx: make([]float64, len(as)), xs: append([]uint32(nil), xs...)}
	defer func() {
		if err != nil {
			err = errors.Join(err, eq.Close())
		}
	}()
	for i := range as {
		if err = z.Copy(&eq.as[i], &as[i]); err != nil {
			return nil, err
		}
		if eq.approx[i], err = z.Float64(&as[i]); err != nil {
			return nil, err
		}
	}
	return eq, nil
}

func (l *LinearEquationManager) Display(out io.Writer, eq *LinearEquation) error {
	if e := l.open(); e != nil {
		return e
	}
	if out == nil || eq == nil || eq.closed {
		return arena.ErrInvalidSlot
	}
	for i, x := range eq.xs {
		if i != 0 {
			if _, e := io.WriteString(out, " + "); e != nil {
				return e
			}
		}
		a, e := l.manager.Integers().String(&eq.as[i])
		if e != nil {
			return e
		}
		if _, e = io.WriteString(out, a+"*x"+strconv.FormatUint(uint64(x), 10)); e != nil {
			return e
		}
	}
	_, e := io.WriteString(out, " = 0")
	return e
}

func (l *LinearEquationManager) Close() error {
	if l == nil || l.self != l {
		return arena.ErrInvalidSlot
	}
	if l.closed {
		return nil
	}
	if e := l.valBuffer.Close(); e != nil {
		return e
	}
	if e := l.intBuffer.Close(); e != nil {
		return e
	}
	l.mark, l.varBuffer = nil, nil
	l.closed = true
	return nil
}
