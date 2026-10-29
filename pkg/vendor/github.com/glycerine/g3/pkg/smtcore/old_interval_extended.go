package smtcore

// Source: src/smt/old_interval.cpp:20-158 and src/smt/old_interval.h:25-64.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type smtExtendedRationalKind uint8

const (
	smtExtendedFinite smtExtendedRationalKind = iota
	smtExtendedMinusInfinity
	smtExtendedPlusInfinity
)

// smtExtendedRational is the concrete rational extended numeral used by the
// retained nonlinear arithmetic interval implementation.
type smtExtendedRational struct {
	kind  smtExtendedRationalKind
	value big.Rat
}

func smtFiniteExtended(value *big.Rat) smtExtendedRational {
	var result smtExtendedRational
	if value != nil {
		result.value.Set(value)
	}
	return result
}

func smtInfiniteExtended(positive bool) smtExtendedRational {
	if positive {
		return smtExtendedRational{kind: smtExtendedPlusInfinity}
	}
	return smtExtendedRational{kind: smtExtendedMinusInfinity}
}

func (n *smtExtendedRational) set(other *smtExtendedRational) *smtExtendedRational {
	n.kind = other.kind
	n.value.Set(&other.value)
	return n
}

func (n *smtExtendedRational) clone() smtExtendedRational {
	var result smtExtendedRational
	result.set(n)
	return result
}

func (n *smtExtendedRational) infinite() bool { return n.kind != smtExtendedFinite }
func (n *smtExtendedRational) sign() int {
	if n.kind == smtExtendedMinusInfinity {
		return -1
	}
	if n.kind == smtExtendedPlusInfinity {
		return 1
	}
	return n.value.Sign()
}
func (n *smtExtendedRational) zero() bool { return n.kind == smtExtendedFinite && n.value.Sign() == 0 }

func (n *smtExtendedRational) neg() *smtExtendedRational {
	switch n.kind {
	case smtExtendedMinusInfinity:
		n.kind = smtExtendedPlusInfinity
	case smtExtendedPlusInfinity:
		n.kind = smtExtendedMinusInfinity
	default:
		n.value.Neg(&n.value)
	}
	return n
}

func (n *smtExtendedRational) add(other *smtExtendedRational) error {
	if n.infinite() && other.infinite() && n.kind != other.kind {
		return ast.ErrASTDomain
	}
	if n.infinite() {
		return nil
	}
	if other.infinite() {
		n.kind = other.kind
		n.value.SetInt64(0)
		return nil
	}
	n.value.Add(&n.value, &other.value)
	return nil
}

func (n *smtExtendedRational) sub(other *smtExtendedRational) error {
	if n.infinite() && other.infinite() && n.kind == other.kind {
		return ast.ErrASTDomain
	}
	if n.infinite() {
		return nil
	}
	if other.infinite() {
		if other.kind == smtExtendedMinusInfinity {
			n.kind = smtExtendedPlusInfinity
		} else {
			n.kind = smtExtendedMinusInfinity
		}
		n.value.SetInt64(0)
		return nil
	}
	n.value.Sub(&n.value, &other.value)
	return nil
}

func (n *smtExtendedRational) mul(other *smtExtendedRational) {
	if n.zero() || other.zero() {
		n.kind = smtExtendedFinite
		n.value.SetInt64(0)
		return
	}
	if n.infinite() || other.infinite() {
		if n.sign() == other.sign() {
			n.kind = smtExtendedPlusInfinity
		} else {
			n.kind = smtExtendedMinusInfinity
		}
		n.value.SetInt64(0)
		return
	}
	n.value.Mul(&n.value, &other.value)
}

func (n *smtExtendedRational) inverse() error {
	if n.zero() {
		return ast.ErrASTDomain
	}
	if n.infinite() {
		n.kind = smtExtendedFinite
		n.value.SetInt64(0)
		return nil
	}
	n.value.Inv(&n.value)
	return nil
}

func (n *smtExtendedRational) power(exponent uint32) {
	if n.kind == smtExtendedMinusInfinity && exponent%2 == 0 {
		n.kind = smtExtendedPlusInfinity
	}
	if n.infinite() {
		return
	}
	power := new(big.Int).SetUint64(uint64(exponent))
	numerator := new(big.Int).Exp(n.value.Num(), power, nil)
	denominator := new(big.Int).Exp(n.value.Denom(), power, nil)
	n.value.SetFrac(numerator, denominator)
}

func (n *smtExtendedRational) cmp(other *smtExtendedRational) int {
	if n.kind == other.kind {
		if n.infinite() {
			return 0
		}
		return n.value.Cmp(&other.value)
	}
	if n.kind == smtExtendedMinusInfinity || other.kind == smtExtendedPlusInfinity {
		return -1
	}
	return 1
}

func (n *smtExtendedRational) equal(other *smtExtendedRational) bool { return n.cmp(other) == 0 }

func (n *smtExtendedRational) String() string {
	switch n.kind {
	case smtExtendedMinusInfinity:
		return "-oo"
	case smtExtendedPlusInfinity:
		return "oo"
	default:
		return n.value.RatString()
	}
}
