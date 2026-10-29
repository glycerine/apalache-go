package numeral

// Source: src/util/mpfx.h and src/util/mpfx.cpp:28-83,90-100,133-249,
// 316-520. Concrete fixed-point numeral reached by subpaving's mpfx context.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type fixedPointException struct {
	message string
	parent  error
}

func (e *fixedPointException) Error() string                    { return e.message }
func (e *fixedPointException) Message() string                  { return e.message }
func (e *fixedPointException) ErrorCode() z3exception.ErrorCode { return z3exception.ErrorOK }
func (e *fixedPointException) HasErrorCode() bool               { return false }
func (e *fixedPointException) Unwrap() error                    { return e.parent }

var (
	ErrFixedPointOwner          = errors.New("mpfx: invalid numeral owner")
	ErrFixedPointClosed         = errors.New("mpfx: manager or numeral closed")
	ErrFixedPointDomain   error = &fixedPointException{message: "multi-precision fixed point (mpfx) exception"}
	ErrFixedPointOverflow error = &fixedPointException{message: "multi-precision fixed point (mpfx) overflow", parent: ErrFixedPointDomain}
	ErrFixedPointDivZero  error = &fixedPointException{message: "multi-precision fixed point (mpfx) division by zero", parent: ErrFixedPointDomain}
)

// FixedPointManager stores the source's fixed number of integer/fractional
// 32-bit words as a signed, scaled Go integer. Rounding initially points down.
type FixedPointManager struct {
	self                  *FixedPointManager
	intWords, fracWords   uint32
	fracBits              uint
	scale, magnitudeLimit big.Int
	roundUp               bool
	trace                 *diagnostic.TraceTags
	live                  uint32
	closed                bool
}

// FixedPoint is an owned concrete mpfx numeral. Its signed value is
// scaled / 2^(32*fracWords); zero has no negative representation.
type FixedPoint struct {
	self               *FixedPoint
	manager            *FixedPointManager
	scaled             big.Int
	sourceZeroSlot     bool
	sourceZeroNegative bool
	closed             bool
}

func (v *FixedPoint) sourceIsZero() bool {
	return v.scaled.Sign() == 0 && !v.sourceZeroSlot
}

func (v *FixedPoint) sourceNegative() bool {
	return v.scaled.Sign() < 0 || v.sourceZeroSlot && v.sourceZeroNegative
}

func NewFixedPointManager(intWords, fracWords uint32) (*FixedPointManager, error) {
	if intWords == 0 || fracWords == 0 || uint64(intWords)+uint64(fracWords) > 1<<20 {
		return nil, ErrFixedPointOwner
	}
	m := &FixedPointManager{intWords: intWords, fracWords: fracWords, fracBits: uint(fracWords) * 32}
	m.self = m
	m.scale.Lsh(big.NewInt(1), m.fracBits)
	m.magnitudeLimit.Lsh(big.NewInt(1), uint(intWords+fracWords)*32)
	return m, nil
}

func (m *FixedPointManager) check(v *FixedPoint) error {
	if m == nil || m.self != m || v == nil || v.self != v || v.manager != m {
		return ErrFixedPointOwner
	}
	if m.closed || v.closed {
		return ErrFixedPointClosed
	}
	return nil
}

func (m *FixedPointManager) pair(a, b *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	return m.check(b)
}

func (m *FixedPointManager) New() (*FixedPoint, error) {
	if m == nil || m.self != m {
		return nil, ErrFixedPointOwner
	}
	if m.closed {
		return nil, ErrFixedPointClosed
	}
	v := &FixedPoint{manager: m}
	v.self = v
	m.live++
	return v, nil
}

func (v *FixedPoint) Close() error {
	if v == nil || v.self != v || v.manager == nil || v.manager.self != v.manager {
		return ErrFixedPointOwner
	}
	if v.closed {
		return nil
	}
	v.scaled.SetInt64(0)
	v.sourceZeroSlot = false
	v.sourceZeroNegative = false
	v.closed = true
	v.manager.live--
	return nil
}

func (m *FixedPointManager) Close() error {
	if m == nil || m.self != m {
		return ErrFixedPointOwner
	}
	if m.live != 0 {
		return ErrFixedPointOwner
	}
	m.closed = true
	return nil
}

func (m *FixedPointManager) SetRounding(up bool) error {
	if m == nil || m.self != m {
		return ErrFixedPointOwner
	}
	if m.closed {
		return ErrFixedPointClosed
	}
	m.roundUp = up
	return nil
}

func (m *FixedPointManager) publish(dst *FixedPoint, scaled *big.Int) error {
	var abs big.Int
	abs.Abs(scaled)
	if abs.Cmp(&m.magnitudeLimit) >= 0 {
		return ErrFixedPointOverflow
	}
	dst.scaled.Set(scaled)
	dst.sourceZeroSlot = false
	dst.sourceZeroNegative = false
	return nil
}

// add_sub's fixed-width ::add writes all destination words before returning
// its carry. Preserve that physical publication when its caller throws.
func (m *FixedPointManager) publishAddOverflow(dst *FixedPoint, value *big.Int) {
	var magnitude big.Int
	magnitude.Abs(value)
	magnitude.Mod(&magnitude, &m.magnitudeLimit)
	negative := value.Sign() < 0
	if magnitude.Sign() == 0 {
		dst.scaled.SetInt64(0)
		dst.sourceZeroSlot = true
		dst.sourceZeroNegative = negative
		return
	}
	if negative {
		magnitude.Neg(&magnitude)
	}
	dst.scaled.Set(&magnitude)
	dst.sourceZeroSlot = false
	dst.sourceZeroNegative = false
}

// Product and numeral-conversion paths acquire the destination slot and set
// its sign before checking temporary result size. Old words stay in place.
func (m *FixedPointManager) publishSignBeforeWords(dst *FixedPoint, negative bool) {
	if dst.scaled.Sign() == 0 {
		dst.sourceZeroSlot = true
		dst.sourceZeroNegative = negative
		return
	}
	dst.scaled.Abs(&dst.scaled)
	if negative {
		dst.scaled.Neg(&dst.scaled)
	}
	dst.sourceZeroSlot = false
	dst.sourceZeroNegative = false
}

func (m *FixedPointManager) Reset(dst *FixedPoint) error {
	if err := m.check(dst); err != nil {
		return err
	}
	dst.scaled.SetInt64(0)
	dst.sourceZeroSlot = false
	dst.sourceZeroNegative = false
	return nil
}

func (m *FixedPointManager) SetInt64(dst *FixedPoint, n int64) error {
	if err := m.check(dst); err != nil {
		return err
	}
	var scaled big.Int
	scaled.Mul(big.NewInt(n), &m.scale)
	return m.publish(dst, &scaled)
}

func (m *FixedPointManager) SetUint64(dst *FixedPoint, n uint64) error {
	if err := m.check(dst); err != nil {
		return err
	}
	var scaled, val big.Int
	val.SetUint64(n)
	scaled.Mul(&val, &m.scale)
	return m.publish(dst, &scaled)
}

func (m *FixedPointManager) Copy(dst, src *FixedPoint) error {
	if err := m.pair(dst, src); err != nil {
		return err
	}
	dst.scaled.Set(&src.scaled)
	dst.sourceZeroSlot = src.sourceZeroSlot
	dst.sourceZeroNegative = src.sourceZeroNegative
	return nil
}

// roundMagnitude is the source sign/mode rule shared by fixed-point
// multiplication and division: an inexact quotient grows in magnitude when
// its sign differs from the selected plus-infinity direction.
func (m *FixedPointManager) roundMagnitude(num, den *big.Int) (*big.Int, error) {
	if den.Sign() == 0 {
		return nil, ErrFixedPointDivZero
	}
	negative := num.Sign() != den.Sign() && num.Sign() != 0
	var an, ad, q, rem big.Int
	an.Abs(num)
	ad.Abs(den)
	q.QuoRem(&an, &ad, &rem)
	if rem.Sign() != 0 && negative != m.roundUp {
		q.Add(&q, big.NewInt(1))
	}
	if negative {
		q.Neg(&q)
	}
	return &q, nil
}

func (m *FixedPointManager) SetRatio(dst *FixedPoint, num int64, den uint64) error {
	if err := m.check(dst); err != nil {
		return err
	}
	var numerator, denominator big.Int
	numerator.Mul(big.NewInt(num), &m.scale)
	denominator.SetUint64(den)
	var absNum, scaledDen big.Int
	absNum.Abs(&numerator)
	scaledDen.Mul(&denominator, &m.scale)
	if absNum.Cmp(&m.magnitudeLimit) >= 0 || scaledDen.Cmp(&m.magnitudeLimit) >= 0 {
		return ErrFixedPointOverflow // source constructs both integer operands first
	}
	if den == 0 {
		return ErrFixedPointDivZero
	}
	m.traceFixedIntegerDivision(num, den, dst, false)
	q, err := m.roundMagnitude(&numerator, &denominator)
	if err != nil {
		return err
	}
	if err := m.publish(dst, q); err != nil {
		return err
	}
	m.traceFixedIntegerDivision(num, den, dst, true)
	return nil
}

func (m *FixedPointManager) Compare(a, b *FixedPoint) (int, error) {
	if err := m.pair(a, b); err != nil {
		return 0, err
	}
	return compareFixedPointSource(a, b), nil
}

func compareFixedPointSource(a, b *FixedPoint) int {
	if cmp := a.scaled.Cmp(&b.scaled); cmp != 0 {
		return cmp
	}
	if a.scaled.Sign() != 0 {
		return 0
	}
	// Source is_zero uses the word-slot ID, so the two zero-word slots
	// remain ordered on either side of the canonical unallocated zero.
	rank := func(v *FixedPoint) int {
		if !v.sourceZeroSlot {
			return 0
		}
		if v.sourceZeroNegative {
			return -1
		}
		return 1
	}
	if rank(a) < rank(b) {
		return -1
	}
	if rank(a) > rank(b) {
		return 1
	}
	return 0
}

func (m *FixedPointManager) IsZero(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	return a.sourceIsZero(), nil
}

func (m *FixedPointManager) IsInt(a *FixedPoint) (bool, error) {
	if err := m.check(a); err != nil {
		return false, err
	}
	var rem big.Int
	rem.Rem(&a.scaled, &m.scale)
	return rem.Sign() == 0, nil
}

func (m *FixedPointManager) Neg(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	if a.sourceZeroSlot {
		a.sourceZeroNegative = !a.sourceZeroNegative
	} else {
		a.scaled.Neg(&a.scaled)
	}
	return nil
}

func (m *FixedPointManager) Abs(a *FixedPoint) error {
	if err := m.check(a); err != nil {
		return err
	}
	a.scaled.Abs(&a.scaled)
	a.sourceZeroNegative = false
	return nil
}

// addSubZeroSlot preserves the source's allocated zero-word result when its
// shortcut copies a zero-word operand or equal-sign zero words are added.
// Capture this before publication because out may alias either operand.
func addSubZeroSlot(a, b *FixedPoint, sub bool) (bool, bool) {
	if a.sourceIsZero() {
		return b.sourceZeroSlot, b.sourceNegative() != sub
	}
	if b.sourceIsZero() {
		return a.sourceZeroSlot, a.sourceNegative()
	}
	if a.sourceZeroSlot && b.sourceZeroSlot && a.sourceNegative() == (b.sourceNegative() != sub) {
		return true, a.sourceNegative()
	}
	return false, false
}

func (m *FixedPointManager) Add(a, b, out *FixedPoint) error {
	if err := m.pair(a, b); err != nil {
		return err
	}
	if err := m.check(out); err != nil {
		return err
	}
	m.traceFixedAddSubOuter(a, b, out, false, true)
	inner := !a.sourceIsZero() && !b.sourceIsZero()
	retainsSlot, negative := addSubZeroSlot(a, b, false)
	if inner {
		m.traceFixedAddSubInner(a, b, out, false, true)
	}
	var v big.Int
	v.Add(&a.scaled, &b.scaled)
	if err := m.publish(out, &v); err != nil {
		m.publishAddOverflow(out, &v)
		return err
	}
	if v.Sign() == 0 && retainsSlot {
		out.sourceZeroSlot = true
		out.sourceZeroNegative = negative
	}
	if inner {
		m.traceFixedAddSubInner(a, b, out, false, false)
	}
	m.traceFixedAddSubOuter(a, b, out, false, false)
	return nil
}

func (m *FixedPointManager) Sub(a, b, out *FixedPoint) error {
	if err := m.pair(a, b); err != nil {
		return err
	}
	if err := m.check(out); err != nil {
		return err
	}
	m.traceFixedAddSubOuter(a, b, out, true, true)
	inner := !a.sourceIsZero() && !b.sourceIsZero()
	retainsSlot, negative := addSubZeroSlot(a, b, true)
	if inner {
		m.traceFixedAddSubInner(a, b, out, true, true)
	}
	var v big.Int
	v.Sub(&a.scaled, &b.scaled)
	if err := m.publish(out, &v); err != nil {
		m.publishAddOverflow(out, &v)
		return err
	}
	if v.Sign() == 0 && retainsSlot {
		out.sourceZeroSlot = true
		out.sourceZeroNegative = negative
	}
	if inner {
		m.traceFixedAddSubInner(a, b, out, true, false)
	}
	m.traceFixedAddSubOuter(a, b, out, true, false)
	return nil
}

func (m *FixedPointManager) Mul(a, b, out *FixedPoint) error {
	if err := m.pair(a, b); err != nil {
		return err
	}
	if err := m.check(out); err != nil {
		return err
	}
	m.traceFixedProduct(a, b, out, false, false)
	retainsSlot := !a.sourceIsZero() && !b.sourceIsZero()
	negative := a.sourceNegative() != b.sourceNegative()
	var product big.Int
	product.Mul(&a.scaled, &b.scaled)
	q, err := m.roundMagnitude(&product, &m.scale)
	if err != nil {
		return err
	}
	if err := m.publish(out, q); err != nil {
		m.publishSignBeforeWords(out, negative)
		return err
	}
	if q.Sign() == 0 && retainsSlot {
		// Source enters the nonzero-operand branch, allocates c, then copies
		// zero product words without calling reset.
		out.sourceZeroSlot = true
		out.sourceZeroNegative = negative
	}
	m.traceFixedProduct(a, b, out, false, true)
	return nil
}

func (m *FixedPointManager) Div(a, b, out *FixedPoint) error {
	if err := m.pair(a, b); err != nil {
		return err
	}
	if err := m.check(out); err != nil {
		return err
	}
	if b.scaled.Sign() == 0 {
		return ErrFixedPointDivZero
	}
	m.traceFixedProduct(a, b, out, true, false)
	negative := a.sourceNegative() != b.sourceNegative()
	var numerator big.Int
	numerator.Mul(&a.scaled, &m.scale)
	q, err := m.roundMagnitude(&numerator, &b.scaled)
	if err != nil {
		return err
	}
	if err := m.publish(out, q); err != nil {
		m.publishSignBeforeWords(out, negative)
		return err
	}
	m.traceFixedProduct(a, b, out, true, true)
	return nil
}

// Rational returns an independent exact value for testing and concrete
// subpaving boundary conversions.
func (m *FixedPointManager) Rational(a *FixedPoint) (*big.Rat, error) {
	if err := m.check(a); err != nil {
		return nil, err
	}
	return new(big.Rat).SetFrac(new(big.Int).Set(&a.scaled), new(big.Int).Set(&m.scale)), nil
}
