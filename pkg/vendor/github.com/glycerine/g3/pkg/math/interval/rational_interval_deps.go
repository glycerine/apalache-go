package interval

// Source: src/math/interval/interval.h:78-104 and
// src/math/interval/interval_def.h:710-725,776-823,834-850,928-1019,
// 1219-1268,1494-1515,1593-1664.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type BoundDeps uint8

const (
	DepLower1 BoundDeps = 1 << iota
	DepUpper1
	DepLower2
	DepUpper2
)

type IntervalDeps struct{ Lower, Upper BoundDeps }

func (d BoundDeps) HasLower1() bool { return d&DepLower1 != 0 }
func (d BoundDeps) HasUpper1() bool { return d&DepUpper1 != 0 }
func (d BoundDeps) HasLower2() bool { return d&DepLower2 != 0 }
func (d BoundDeps) HasUpper2() bool { return d&DepUpper2 != 0 }

func (d BoundDeps) FirstToSecond() (BoundDeps, error) {
	if d&(DepLower2|DepUpper2) != 0 {
		return 0, ErrExtendedDomain
	}
	return d << 2, nil
}

func (a *RationalInterval) checkDepsPair(b *RationalInterval) error {
	if err := a.open(); err != nil {
		return err
	}
	if err := b.open(); err != nil {
		return err
	}
	if a.numbers != b.numbers {
		return arena.ErrInvalidSlot
	}
	return nil
}

func (a *RationalInterval) NegDeps() (IntervalDeps, error) {
	if err := a.open(); err != nil {
		return IntervalDeps{}, err
	}
	var d IntervalDeps
	if !a.upperInf {
		d.Lower = DepUpper1
	}
	if !a.lowerInf {
		d.Upper = DepLower1
	}
	return d, nil
}

func (a *RationalInterval) AddDeps(b *RationalInterval) (IntervalDeps, error) {
	if err := a.checkDepsPair(b); err != nil {
		return IntervalDeps{}, err
	}
	return IntervalDeps{DepLower1 | DepLower2, DepUpper1 | DepUpper2}, nil
}

func (a *RationalInterval) SubDeps(b *RationalInterval) (IntervalDeps, error) {
	if err := a.checkDepsPair(b); err != nil {
		return IntervalDeps{}, err
	}
	return IntervalDeps{DepLower1 | DepUpper2, DepUpper1 | DepLower2}, nil
}

func (a *RationalInterval) MulScalarDeps(k *numeral.Rational) (IntervalDeps, error) {
	if err := a.open(); err != nil {
		return IntervalDeps{}, err
	}
	z, err := a.numbers.IsZero(k)
	if err != nil {
		return IntervalDeps{}, err
	}
	if z {
		return IntervalDeps{}, nil
	}
	neg, err := a.numbers.IsNeg(k)
	if err != nil {
		return IntervalDeps{}, err
	}
	if neg {
		return IntervalDeps{DepUpper1, DepLower1}, nil
	}
	return IntervalDeps{DepLower1, DepUpper1}, nil
}

func (a *RationalInterval) MulDeps(b *RationalInterval) (IntervalDeps, error) {
	if err := a.checkDepsPair(b); err != nil {
		return IntervalDeps{}, err
	}
	zero, err := a.isSingletonZero()
	if err != nil {
		return IntervalDeps{}, err
	}
	if zero {
		return IntervalDeps{DepLower1 | DepUpper1, DepLower1 | DepUpper1}, nil
	}
	zero, err = b.isSingletonZero()
	if err != nil {
		return IntervalDeps{}, err
	}
	if zero {
		return IntervalDeps{DepLower2 | DepUpper2, DepLower2 | DepUpper2}, nil
	}
	ac, err := a.signClass()
	if err != nil {
		return IntervalDeps{}, err
	}
	bc, err := b.signClass()
	if err != nil {
		return IntervalDeps{}, err
	}
	switch ac {
	case 'N':
		switch bc {
		case 'N':
			return IntervalDeps{DepUpper1 | DepUpper2, DepLower1 | DepLower2 | DepUpper1}, nil
		case 'M':
			return IntervalDeps{DepLower1 | DepUpper2 | DepUpper1, DepLower1 | DepLower2 | DepUpper1}, nil
		default:
			return IntervalDeps{DepLower1 | DepUpper2 | DepUpper1, DepUpper1 | DepLower2}, nil
		}
	case 'M':
		switch bc {
		case 'N':
			return IntervalDeps{DepUpper1 | DepLower2 | DepUpper2, DepLower1 | DepLower2 | DepUpper2}, nil
		case 'M':
			return IntervalDeps{DepLower1 | DepUpper1 | DepLower2 | DepUpper2, DepLower1 | DepUpper1 | DepLower2 | DepUpper2}, nil
		default:
			return IntervalDeps{DepLower1 | DepUpper2 | DepLower2, DepUpper1 | DepUpper2 | DepLower2}, nil
		}
	default:
		switch bc {
		case 'N':
			return IntervalDeps{DepUpper1 | DepLower2 | DepLower1, DepLower1 | DepUpper2}, nil
		case 'M':
			return IntervalDeps{DepUpper1 | DepLower2 | DepLower1, DepUpper1 | DepUpper2 | DepLower1}, nil
		default:
			return IntervalDeps{DepLower1 | DepLower2, DepUpper1 | DepUpper2 | DepLower1}, nil
		}
	}
}

func (a *RationalInterval) PowerDeps(n uint32) (IntervalDeps, error) {
	if err := a.open(); err != nil {
		return IntervalDeps{}, err
	}
	if n == 1 {
		return IntervalDeps{DepLower1, DepUpper1}, nil
	}
	if n%2 != 0 {
		var d IntervalDeps
		if !a.lowerInf {
			d.Lower = DepLower1
		}
		if !a.upperInf {
			d.Upper = DepUpper1
		}
		return d, nil
	}
	e := RationalExtended{Numbers: a.numbers}
	pos, err := e.IsPos(&a.lower, a.lowerKind())
	if err != nil {
		return IntervalDeps{}, err
	}
	if pos {
		if a.upperInf {
			return IntervalDeps{DepLower1, 0}, nil
		}
		return IntervalDeps{DepLower1, DepLower1 | DepUpper1}, nil
	}
	neg, err := e.IsNeg(&a.upper, a.upperKind())
	if err != nil {
		return IntervalDeps{}, err
	}
	if neg {
		if a.lowerInf {
			return IntervalDeps{DepUpper1, 0}, nil
		}
		return IntervalDeps{DepUpper1, DepLower1 | DepUpper1}, nil
	}
	return IntervalDeps{0, DepLower1 | DepUpper1}, nil
}

func (a *RationalInterval) InvDeps() (IntervalDeps, error) {
	if err := a.open(); err != nil {
		return IntervalDeps{}, err
	}
	z, err := a.ContainsZero()
	if err != nil {
		return IntervalDeps{}, err
	}
	if z {
		return IntervalDeps{}, ErrExtendedDomain
	}
	e := RationalExtended{Numbers: a.numbers}
	pos, err := e.IsPos(&a.lower, a.lowerKind())
	if err != nil {
		return IntervalDeps{}, err
	}
	lz, err := e.IsZero(&a.lower, a.lowerKind())
	if err != nil {
		return IntervalDeps{}, err
	}
	if pos || lz && a.lowerOpen {
		return IntervalDeps{DepLower1 | DepUpper1, DepLower1}, nil
	}
	neg, err := e.IsNeg(&a.upper, a.upperKind())
	if err != nil {
		return IntervalDeps{}, err
	}
	uz, err := e.IsZero(&a.upper, a.upperKind())
	if err != nil {
		return IntervalDeps{}, err
	}
	if neg || uz && a.upperOpen {
		return IntervalDeps{DepUpper1, DepLower1 | DepUpper1}, nil
	}
	return IntervalDeps{}, ErrExtendedDomain
}

func (a *RationalInterval) DivDeps(b *RationalInterval) (IntervalDeps, error) {
	if err := a.checkDepsPair(b); err != nil {
		return IntervalDeps{}, err
	}
	z, err := b.ContainsZero()
	if err != nil {
		return IntervalDeps{}, err
	}
	if z {
		return IntervalDeps{}, ErrExtendedDomain
	}
	e := RationalExtended{Numbers: a.numbers}
	neg, err := e.IsNeg(&b.upper, b.upperKind())
	if err != nil {
		return IntervalDeps{}, err
	}
	uz, err := e.IsZero(&b.upper, b.upperKind())
	if err != nil {
		return IntervalDeps{}, err
	}
	dn := neg || uz && b.upperOpen
	zero, err := a.isSingletonZero()
	if err != nil {
		return IntervalDeps{}, err
	}
	if zero {
		if dn {
			return IntervalDeps{DepUpper1 | DepUpper2, DepLower1 | DepUpper2}, nil
		}
		return IntervalDeps{DepLower1 | DepLower2, DepUpper1 | DepLower2}, nil
	}
	ac, err := a.signClass()
	if err != nil {
		return IntervalDeps{}, err
	}
	if dn {
		switch ac {
		case 'N':
			return IntervalDeps{DepUpper1 | DepLower2 | DepUpper2, DepLower1 | DepUpper2}, nil
		case 'M':
			return IntervalDeps{DepUpper1 | DepUpper2, DepLower1 | DepUpper2}, nil
		default:
			return IntervalDeps{DepUpper1 | DepUpper2, DepLower1 | DepLower2 | DepUpper2}, nil
		}
	}
	switch ac {
	case 'N':
		return IntervalDeps{DepLower1 | DepLower2, DepUpper1 | DepLower2 | DepUpper2}, nil
	case 'M':
		return IntervalDeps{DepLower1 | DepLower2, DepUpper1 | DepLower2}, nil
	default:
		return IntervalDeps{DepLower1 | DepLower2 | DepUpper2, DepUpper1 | DepLower2}, nil
	}
}
