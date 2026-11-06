package numeral

// Source: src/util/mpq_inf.h:26-279; src/util/mpq_inf.cpp:22-43.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	uhash "github.com/glycerine/g3/pkg/util/hash"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// MPQInf is the source rational pair. Initialize with NewMPQInf and never
// shallow-copy big components; use the manager's Set/Del operations.
type MPQInf struct{ First, Second Rational }

func NewMPQInf() MPQInf { return MPQInf{NewRational(), NewRational()} }

type MPQInfKind int32

const (
	MPQInfNegative MPQInfKind = -1
	MPQInfZero     MPQInfKind = 0
	MPQInfPositive MPQInfKind = 1
)

var ErrMPQInfOwner = errors.New("g3: invalid mpq_inf manager or value")
var ErrMPQInfKind = errors.New("g3: reached unreachable mpq_inf kind")

// MPQInfManager owns the concrete rational manager. Synchronized selects the
// original local versus reusable-scratch algorithms, not concurrent access.
type MPQInfManager struct {
	self           *MPQInfManager
	m              *RationalManager
	inf            float64
	header         uint64
	cause          error
	active, closed bool
	format         mpqInfFormattingScope
}

func NewMPQInfManager(config RationalConfig) (*MPQInfManager, error) {
	b := config.Integers.Cells.Budget
	header := uint64(reflect.TypeOf(MPQInfManager{}).Size())
	if e := b.Reserve(header); e != nil {
		return nil, e
	}
	m, e := NewRationalManager(config)
	if e != nil {
		b.Release(header)
		return nil, e
	}
	p := &MPQInfManager{m: m, inf: 0.0001, header: header}
	p.self = p
	return p, nil
}
func (p *MPQInfManager) open() error {
	if p == nil || p.self != p {
		return ErrMPQInfOwner
	}
	if p.closed {
		return arena.ErrClosed
	}
	if p.active {
		return ErrNumericPending
	}
	if p.cause != nil {
		return p.cause
	}
	return p.m.checkOwnerReady()
}
func (p *MPQInfManager) begin(values ...*MPQInf) error {
	if e := p.open(); e != nil {
		return e
	}
	for _, v := range values {
		if v == nil {
			return ErrMPQInfOwner
		}
	}
	p.active = true
	return nil
}
func (p *MPQInfManager) end(e *error) {
	p.active = false
	if *e != nil {
		p.cause = *e
	}
}
func (p *MPQInfManager) SetInf(inf float64) error {
	if e := p.open(); e != nil {
		return e
	}
	p.inf = inf
	return nil
}
func (p *MPQInfManager) GetMPQManager() (*RationalManager, error) {
	if e := p.open(); e != nil {
		return nil, e
	}
	return p.m, nil
}
func (p *MPQInfManager) Close() error {
	if p == nil || p.self != p {
		return ErrMPQInfOwner
	}
	if p.closed {
		return nil
	}
	if p.active {
		return ErrNumericPending
	}
	p.active = true
	defer func() { p.active = false }()
	if e := p.cleanupFormatting(); e != nil {
		return e
	}
	if e := p.m.CloseChecked(); e != nil {
		return e
	}
	p.m.BudgetField.Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}

// CleanupValue joins retained younger arithmetic scopes before releasing this
// caller-owned pair. It is owning cleanup, not source failure resumability.
func (p *MPQInfManager) CleanupValue(a *MPQInf) error {
	if p == nil || p.self != p || a == nil {
		return ErrMPQInfOwner
	}
	if p.active {
		return ErrNumericPending
	}
	if p.closed {
		return arena.ErrClosed
	}
	p.active = true
	defer func() { p.active = false }()
	if e := p.cleanupFormatting(); e != nil {
		return e
	}
	if e := p.m.cleanupRawArithmetic(); e != nil {
		return e
	}
	if e := p.m.lexical.cleanup(p.m.Z); e != nil {
		return e
	}
	if e := p.m.Del(&a.First); e != nil {
		return e
	}
	return p.m.Del(&a.Second)
}
func (p *MPQInfManager) Reset(a *MPQInf) (e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.Reset(&a.First); e != nil {
		return
	}
	return p.m.Reset(&a.Second)
}
func (p *MPQInfManager) Del(a *MPQInf) (e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.Del(&a.First); e != nil {
		return
	}
	return p.m.Del(&a.Second)
}
func (p *MPQInfManager) Swap(a, b *MPQInf) (e error) {
	if e = p.begin(a, b); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.Swap(&a.First, &b.First); e != nil {
		return
	}
	return p.m.Swap(&a.Second, &b.Second)
}
func (p *MPQInfManager) Set(a, b *MPQInf) (e error) {
	if e = p.begin(a, b); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.Copy(&a.First, &b.First); e != nil {
		return
	}
	return p.m.Copy(&a.Second, &b.Second)
}
func (p *MPQInfManager) SetRational(a *MPQInf, r *Rational) (e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.Copy(&a.First, r); e != nil {
		return
	}
	return p.m.Reset(&a.Second)
}
func (p *MPQInfManager) SetPair(a *MPQInf, r, i *Rational) (e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.Copy(&a.First, r); e != nil {
		return
	}
	return p.m.Copy(&a.Second, i)
}
func (p *MPQInfManager) SetKind(a *MPQInf, r *Rational, k MPQInfKind) (e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.Copy(&a.First, r); e != nil {
		return
	}
	switch k {
	case MPQInfNegative:
		return p.m.SetInt32(&a.Second, -1)
	case MPQInfZero:
		return p.m.Reset(&a.Second)
	case MPQInfPositive:
		return p.m.SetInt32(&a.Second, 1)
	}
	return nil // Source switch has no default; unknown kind preserves second.
}
func (p *MPQInfManager) Hash(a *MPQInf) (h uint32, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	first, e := p.m.Hash(&a.First)
	if e != nil {
		return
	}
	second, e := p.m.Hash(&a.Second)
	return uhash.UintPair(first, second), e
}
func (p *MPQInfManager) IsInt(a *MPQInf) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	yes, e = p.m.IsInt(&a.First)
	if e != nil || !yes {
		return
	}
	return p.m.IsZero(&a.Second)
}
func (p *MPQInfManager) IsRational(a *MPQInf) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	return p.m.IsZero(&a.Second)
}
func (p *MPQInfManager) IsZero(a *MPQInf) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	yes, e = p.m.IsZero(&a.First)
	if e != nil || !yes {
		return
	}
	return p.m.IsZero(&a.Second)
}
func (p *MPQInfManager) sign(a *MPQInf, positive bool) (bool, error) {
	var yes bool
	var e error
	if positive {
		yes, e = p.m.IsPos(&a.First)
	} else {
		yes, e = p.m.IsNeg(&a.First)
	}
	if e != nil || yes {
		return yes, e
	}
	yes, e = p.m.IsZero(&a.First)
	if e != nil || !yes {
		return false, e
	}
	if positive {
		return p.m.IsPos(&a.Second)
	}
	return p.m.IsNeg(&a.Second)
}
func (p *MPQInfManager) IsPos(a *MPQInf) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	return p.sign(a, true)
}
func (p *MPQInfManager) IsNeg(a *MPQInf) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	return p.sign(a, false)
}
func (p *MPQInfManager) GetRational(a *MPQInf, r *Rational) (e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	return p.m.Copy(r, &a.First)
}
func (p *MPQInfManager) GetInfinitesimal(a *MPQInf, r *Rational) (e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	return p.m.Copy(r, &a.Second)
}
func (p *MPQInfManager) GetDouble(a *MPQInf) (value float64, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	value, e = p.m.Float64(&a.First)
	if e != nil {
		return
	}
	yes, e := p.m.IsPos(&a.Second)
	if e != nil {
		return
	}
	if yes {
		return value + p.inf, nil
	}
	yes, e = p.m.IsNeg(&a.Second)
	if e != nil {
		return
	}
	if yes {
		return value - p.inf, nil
	}
	return value, nil
}
func (p *MPQInfManager) eq(a, b *MPQInf) (bool, error) {
	yes, e := p.m.Equal(&a.First, &b.First)
	if e != nil || !yes {
		return false, e
	}
	return p.m.Equal(&a.Second, &b.Second)
}
func (p *MPQInfManager) lt(a, b *MPQInf) (bool, error) {
	n, e := p.m.Compare(&a.First, &b.First)
	if e != nil || n < 0 {
		return n < 0, e
	}
	n, e = p.m.Compare(&a.Second, &b.Second)
	if e != nil || n >= 0 {
		return false, e
	}
	return p.m.Equal(&a.First, &b.First)
}
func (p *MPQInfManager) Eq(a, b *MPQInf) (yes bool, e error) {
	if e = p.begin(a, b); e != nil {
		return
	}
	defer p.end(&e)
	return p.eq(a, b)
}

func (p *MPQInfManager) EqRational(a *MPQInf, b *Rational) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	yes, e = p.m.Equal(&a.First, b)
	if e != nil || !yes {
		return
	}
	return p.m.IsZero(&a.Second)
}
func (p *MPQInfManager) EqKind(a *MPQInf, b *Rational, k MPQInfKind) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	yes, e = p.m.Equal(&a.First, b)
	if e != nil || !yes {
		return
	}
	switch k {
	case MPQInfNegative:
		return p.m.IsMinusOne(&a.Second)
	case MPQInfZero:
		return p.m.IsZero(&a.Second)
	case MPQInfPositive:
		return p.m.IsOne(&a.Second)
	}
	return false, ErrMPQInfKind
}
func (p *MPQInfManager) compareRational(a *MPQInf, b *Rational, greater bool) (bool, error) {
	var n int
	var e error
	if greater {
		n, e = p.m.Compare(b, &a.First)
	} else {
		n, e = p.m.Compare(&a.First, b)
	}
	if e != nil || n < 0 {
		return n < 0, e
	}
	var yes bool
	if greater {
		yes, e = p.m.IsPos(&a.Second)
	} else {
		yes, e = p.m.IsNeg(&a.Second)
	}
	if e != nil || !yes {
		return false, e
	}
	return p.m.Equal(&a.First, b)
}
func (p *MPQInfManager) compareKind(a *MPQInf, b *Rational, k MPQInfKind, greater bool) (bool, error) {
	var n int
	var e error
	if greater {
		n, e = p.m.Compare(b, &a.First)
	} else {
		n, e = p.m.Compare(&a.First, b)
	}
	if e != nil || n < 0 {
		return n < 0, e
	}
	yes, e := p.m.Equal(&a.First, b)
	if e != nil || !yes {
		return false, e
	}
	if k == MPQInfZero {
		if greater {
			return p.m.IsPos(&a.Second)
		}
		return p.m.IsNeg(&a.Second)
	}
	if k != MPQInfNegative && k != MPQInfPositive {
		return false, ErrMPQInfKind
	}
	literal := Rational{Num: NewInt(int32(k)), Den: NewInt(1)}
	if greater {
		n, e = p.m.Compare(&literal, &a.Second)
	} else {
		n, e = p.m.Compare(&a.Second, &literal)
	}
	return n < 0, e
}
func (p *MPQInfManager) LtRational(a *MPQInf, b *Rational) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	return p.compareRational(a, b, false)
}
func (p *MPQInfManager) GtRational(a *MPQInf, b *Rational) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	return p.compareRational(a, b, true)
}
func (p *MPQInfManager) LeRational(a *MPQInf, b *Rational) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	yes, e = p.compareRational(a, b, true)
	if e != nil {
		return false, e
	}
	return !yes, nil
}
func (p *MPQInfManager) GeRational(a *MPQInf, b *Rational) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	yes, e = p.compareRational(a, b, false)
	if e != nil {
		return false, e
	}
	return !yes, nil
}
func (p *MPQInfManager) LtKind(a *MPQInf, b *Rational, k MPQInfKind) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	return p.compareKind(a, b, k, false)
}
func (p *MPQInfManager) GtKind(a *MPQInf, b *Rational, k MPQInfKind) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	return p.compareKind(a, b, k, true)
}
func (p *MPQInfManager) LeKind(a *MPQInf, b *Rational, k MPQInfKind) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	yes, e = p.compareKind(a, b, k, true)
	if e != nil {
		return false, e
	}
	return !yes, nil
}
func (p *MPQInfManager) GeKind(a *MPQInf, b *Rational, k MPQInfKind) (yes bool, e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	yes, e = p.compareKind(a, b, k, false)
	if e != nil {
		return false, e
	}
	return !yes, nil
}
func (p *MPQInfManager) Lt(a, b *MPQInf) (yes bool, e error) {
	if e = p.begin(a, b); e != nil {
		return
	}
	defer p.end(&e)
	return p.lt(a, b)
}
func (p *MPQInfManager) Gt(a, b *MPQInf) (yes bool, e error) {
	if e = p.begin(a, b); e != nil {
		return
	}
	defer p.end(&e)
	return p.lt(b, a)
}
func (p *MPQInfManager) Le(a, b *MPQInf) (yes bool, e error) {
	if e = p.begin(a, b); e != nil {
		return
	}
	defer p.end(&e)
	yes, e = p.lt(b, a)
	if e != nil {
		return false, e
	}
	return !yes, nil
}
func (p *MPQInfManager) Ge(a, b *MPQInf) (yes bool, e error) {
	if e = p.begin(a, b); e != nil {
		return
	}
	defer p.end(&e)
	yes, e = p.lt(a, b)
	if e != nil {
		return false, e
	}
	return !yes, nil
}
func (p *MPQInfManager) Add(a, b, c *MPQInf) (e error) {
	if e = p.begin(a, b, c); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.Add(&a.First, &b.First, &c.First); e != nil {
		return
	}
	return p.m.Add(&a.Second, &b.Second, &c.Second)
}
func (p *MPQInfManager) Sub(a, b, c *MPQInf) (e error) {
	if e = p.begin(a, b, c); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.Sub(&a.First, &b.First, &c.First); e != nil {
		return
	}
	return p.m.Sub(&a.Second, &b.Second, &c.Second)
}
func (p *MPQInfManager) AddRational(a *MPQInf, b *Rational, c *MPQInf) (e error) {
	if e = p.begin(a, c); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.Add(&a.First, b, &c.First); e != nil {
		return
	}
	return p.m.Copy(&c.Second, &a.Second)
}
func (p *MPQInfManager) SubRational(a *MPQInf, b *Rational, c *MPQInf) (e error) {
	if e = p.begin(a, c); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.Sub(&a.First, b, &c.First); e != nil {
		return
	}
	return p.m.Copy(&c.Second, &a.Second)
}
func (p *MPQInfManager) MulRational(a *MPQInf, b *Rational, c *MPQInf) (e error) {
	if e = p.begin(a, c); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.Mul(&a.First, b, &c.First); e != nil {
		return
	}
	return p.m.Mul(&a.Second, b, &c.Second)
}
func (p *MPQInfManager) DivRational(a *MPQInf, b *Rational, c *MPQInf) (e error) {
	if e = p.begin(a, c); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.Div(&a.First, b, &c.First); e != nil {
		return
	}
	return p.m.Div(&a.Second, b, &c.Second)
}
func (p *MPQInfManager) MulInteger(a *MPQInf, b *Int, c *MPQInf) (e error) {
	if e = p.begin(a, c); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.MulInt(b, &a.First, &c.First); e != nil {
		return
	}
	return p.m.MulInt(b, &a.Second, &c.Second)
}
func (p *MPQInfManager) DivInteger(a *MPQInf, b *Int, c *MPQInf) (e error) {
	if e = p.begin(a, c); e != nil {
		return
	}
	defer p.end(&e)
	if e = p.m.DivInt(&a.First, b, &c.First); e != nil {
		return
	}
	return p.m.DivInt(&a.Second, b, &c.Second)
}
func (p *MPQInfManager) Inc(a *MPQInf) (e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	return p.m.Inc(&a.First)
}
func (p *MPQInfManager) Dec(a *MPQInf) (e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	return p.m.Dec(&a.First)
}
func (p *MPQInfManager) neg(a *MPQInf) error {
	if e := p.m.Neg(&a.First); e != nil {
		return e
	}
	return p.m.Neg(&a.Second)
}
func (p *MPQInfManager) Neg(a *MPQInf) (e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	return p.neg(a)
}
func (p *MPQInfManager) Abs(a *MPQInf) (e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	yes, e := p.sign(a, false)
	if e != nil || !yes {
		return e
	}
	return p.neg(a)
}
func (p *MPQInfManager) Ceil(a *MPQInf, b *Rational) (e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	integer, e := p.m.IsInt(&a.First)
	if e != nil {
		return
	}
	if !integer {
		return p.m.Ceil(&a.First, b)
	}
	positive, e := p.m.IsPos(&a.Second)
	if e != nil {
		return
	}
	if !positive {
		return p.m.Copy(b, &a.First)
	}
	one := Rational{Num: NewInt(1), Den: NewInt(1)}
	return p.m.Add(&a.First, &one, b)
}
func (p *MPQInfManager) Floor(a *MPQInf, b *Rational) (e error) {
	if e = p.begin(a); e != nil {
		return
	}
	defer p.end(&e)
	integer, e := p.m.IsInt(&a.First)
	if e != nil {
		return
	}
	if !integer {
		return p.m.Floor(&a.First, b)
	}
	// Preserve the original test on FIRST, even when epsilon is zero or positive.
	negative, e := p.m.IsNeg(&a.First)
	if e != nil {
		return
	}
	if !negative {
		return p.m.Copy(b, &a.First)
	}
	one := Rational{Num: NewInt(1), Den: NewInt(1)}
	return p.m.Sub(&a.First, &one, b)
}

// Concrete lexical storage retains actual source objects at a stopped call.
// The manager header accounts this control storage; strings account backing.
// out is the implicit return slot, borrowed during execution and owned by this
// scope on failure. Successful ToString transfers that ownership to the caller.
type mpqInfFormattingScope struct {
	active                       bool
	out                          *utext.OwnedString
	number, denominator, display utext.OwnedString
	stream                       utext.OwnedStringStream
	tmp                          Rational
	tmpLive                      bool
}

// formatInteger follows mpz.cpp:1432-1435 without the retained text assembly
// adapter: display into the actual local output stream, copy str(), destroy it.
func (p *MPQInfManager) formatInteger(a *Int, dst *utext.OwnedString) error {
	s := &p.format
	if e := s.stream.Construct(p.m.BudgetField); e != nil {
		return e
	}
	if e := p.m.Z.insertSourceInteger(&s.stream, a); e != nil {
		return e
	}
	if e := s.stream.Str(dst); e != nil {
		return e
	}
	s.stream.OwnAbort()
	s.stream = utext.OwnedStringStream{}
	return nil
}

// mpq.cpp:123-127. The concrete Go profile selects left-to-right evaluation of
// the C++ concatenation operands. Numerator, slash and denominator retain their
// distinct source lifetimes; C++'s unspecified operand evaluation order is not
// claimed as a native allocation sequence across all compilers.
func (p *MPQInfManager) formatRational(a *Rational, dst *utext.OwnedString) error {
	integer, e := p.m.IsInt(a)
	if e != nil {
		return e
	}
	if e = p.formatInteger(&a.Num, dst); e != nil {
		return e
	}
	if integer {
		return nil
	}
	if e = dst.AppendCString("/"); e != nil {
		return e
	}
	s := &p.format
	if e = p.formatInteger(&a.Den, &s.denominator); e != nil {
		return e
	}
	if e = dst.Append(s.denominator.View()); e != nil {
		return e
	}
	s.denominator.SourceDestroy()
	s.denominator = utext.OwnedString{}
	return nil
}

// mpq_inf.cpp:22-43. tmp is raw (not a scoped numeral); its explicit del is
// reached only after copy, abs, conversion and append all succeed.
func (p *MPQInfManager) formatPair(a *MPQInf, dst *utext.OwnedString) error {
	s := &p.format
	s.active = true
	s.out = dst
	zero, e := p.m.IsZero(&a.Second)
	if e != nil {
		return e
	}
	if zero {
		return p.formatRational(&a.First, dst)
	}
	if e = dst.Construct(p.m.BudgetField, "("); e != nil {
		return e
	}
	if e = p.formatRational(&a.First, &s.number); e != nil {
		return e
	}
	if e = dst.Append(s.number.View()); e != nil {
		return e
	}
	s.number.SourceDestroy()
	s.number = utext.OwnedString{}
	negative, e := p.m.IsNeg(&a.Second)
	if e != nil {
		return e
	}
	suffix := " +e*"
	if negative {
		suffix = " -e*"
	}
	if e = dst.AppendCString(suffix); e != nil {
		return e
	}
	s.tmp = NewRational()
	s.tmpLive = true
	if e = p.m.Copy(&s.tmp, &a.Second); e != nil {
		return e
	}
	if e = p.m.Abs(&s.tmp); e != nil {
		return e
	}
	if e = p.formatRational(&s.tmp, &s.number); e != nil {
		return e
	}
	if e = dst.Append(s.number.View()); e != nil {
		return e
	}
	s.number.SourceDestroy()
	s.number = utext.OwnedString{}
	if e = p.m.Del(&s.tmp); e != nil {
		return e
	}
	s.tmpLive = false
	return dst.AppendCString(")")
}

// ToString constructs the native return string in caller-provided fresh
// storage. On success the caller owns it and must SourceDestroy it. On failure
// CleanupFormatting, CleanupValue or Close releases the retained return slot;
// the caller must not dispose that slot before owning cleanup joins its scope.
func (p *MPQInfManager) ToString(a *MPQInf, dst *utext.OwnedString) (e error) {
	if e = p.begin(a); e != nil {
		return e
	}
	defer p.end(&e)
	if dst == nil || dst.Self != nil {
		return utext.ErrOwnedStringOwner
	}
	if e = p.formatPair(a, dst); e != nil {
		return e
	}
	p.format = mpqInfFormattingScope{}
	return nil
}

// Display follows mpq_inf.h:275-277: construct the whole return string, insert
// it once (width applies to the whole string), then destroy the temporary.
func (p *MPQInfManager) Display(out *utext.NumericTextStream, a *MPQInf) (e error) {
	if e = p.begin(a); e != nil {
		return e
	}
	defer p.end(&e)
	s := &p.format
	if e = p.formatPair(a, &s.display); e != nil {
		return e
	}
	if out == nil {
		return utext.ErrNumericStreamOwner
	}
	if e = s.display.AppendTo(out); e != nil {
		return e
	}
	s.display.SourceDestroy()
	p.format = mpqInfFormattingScope{}
	return nil
}

func (p *MPQInfManager) cleanupFormatting() error {
	s := &p.format
	if !s.active {
		return nil
	}
	// Join the younger numerical allocation/character scopes before raw tmp or
	// any older return-string object. No formatting/output source work is replayed.
	if e := p.m.Z.Natural.cleanupOutputFormatting(); e != nil {
		return e
	}
	if e := p.m.cleanupRawArithmetic(); e != nil {
		return e
	}
	if e := p.m.lexical.cleanup(p.m.Z); e != nil {
		return e
	}
	s.stream.OwnAbort()
	s.denominator.OwnAbort()
	s.number.OwnAbort()
	if s.tmpLive {
		if e := p.m.Del(&s.tmp); e != nil {
			return e
		}
		s.tmpLive = false
	}
	if s.out != nil {
		s.out.OwnAbort()
	}
	p.format = mpqInfFormattingScope{}
	return nil
}

// CleanupFormatting joins the actual retained lexical scope after failure.
// The parent cause remains sticky; owning cleanup does not resume source work.
func (p *MPQInfManager) CleanupFormatting() error {
	if p == nil || p.self != p {
		return ErrMPQInfOwner
	}
	if p.closed {
		return arena.ErrClosed
	}
	if p.active {
		return ErrNumericPending
	}
	p.active = true
	defer func() { p.active = false }()
	return p.cleanupFormatting()
}
