package smtcore

// Source: src/smt/old_interval.cpp and src/smt/old_interval.h retained
// rational interval operations used by nonlinear arithmetic.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type smtOldInterval struct {
	manager              *smtArithmeticBoundDependencyManager
	lower, upper         smtExtendedRational
	lowerOpen, upperOpen bool
	lowerDep, upperDep   *smtArithmeticBoundDependency
}

func newSMTOldInterval(manager *smtArithmeticBoundDependencyManager) (*smtOldInterval, error) {
	if manager == nil {
		return nil, ast.ErrASTDomain
	}
	return &smtOldInterval{manager: manager, lower: smtInfiniteExtended(false), upper: smtInfiniteExtended(true), lowerOpen: true, upperOpen: true}, nil
}

func newSMTOldIntervalBounds(manager *smtArithmeticBoundDependencyManager, lower *big.Rat, lowerOpen bool, lowerDep *smtArithmeticBoundDependency, upper *big.Rat, upperOpen bool, upperDep *smtArithmeticBoundDependency) (*smtOldInterval, error) {
	if manager == nil || lower == nil || upper == nil || lower.Cmp(upper) > 0 || lower.Cmp(upper) == 0 && lowerOpen && upperOpen {
		return nil, ast.ErrASTDomain
	}
	return &smtOldInterval{manager: manager, lower: smtFiniteExtended(lower), upper: smtFiniteExtended(upper), lowerOpen: lowerOpen, upperOpen: upperOpen, lowerDep: lowerDep, upperDep: upperDep}, nil
}

func newSMTOldIntervalPoint(manager *smtArithmeticBoundDependencyManager, value *big.Rat, lowerDep, upperDep *smtArithmeticBoundDependency) (*smtOldInterval, error) {
	return newSMTOldIntervalBounds(manager, value, false, lowerDep, value, false, upperDep)
}

func newSMTOldIntervalHalfBound(manager *smtArithmeticBoundDependencyManager, value *big.Rat, open, lower bool, dependency *smtArithmeticBoundDependency) (*smtOldInterval, error) {
	result, err := newSMTOldInterval(manager)
	if err != nil || value == nil {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return nil, err
	}
	if lower {
		result.lower, result.lowerOpen, result.lowerDep = smtFiniteExtended(value), open, dependency
	} else {
		result.upper, result.upperOpen, result.upperDep = smtFiniteExtended(value), open, dependency
	}
	return result, nil
}

func (i *smtOldInterval) clone() *smtOldInterval {
	result := *i
	// A big.Rat struct copy shares its limb storage. Calling Set on that
	// shallow copy can reuse the shared storage, so start with fresh values.
	result.lower = i.lower.clone()
	result.upper = i.upper.clone()
	return &result
}
func (i *smtOldInterval) valid(other *smtOldInterval) bool {
	return i != nil && other != nil && i.manager != nil && i.manager == other.manager
}

func (i *smtOldInterval) add(other *smtOldInterval) error {
	if !i.valid(other) {
		return ast.ErrASTDomain
	}
	if err := i.lower.add(&other.lower); err != nil {
		return err
	}
	if err := i.upper.add(&other.upper); err != nil {
		return err
	}
	i.lowerOpen = i.lowerOpen || other.lowerOpen
	i.upperOpen = i.upperOpen || other.upperOpen
	if i.lower.infinite() {
		i.lowerDep = nil
	} else {
		i.lowerDep = i.manager.Join(i.lowerDep, other.lowerDep)
	}
	if i.upper.infinite() {
		i.upperDep = nil
	} else {
		i.upperDep = i.manager.Join(i.upperDep, other.upperDep)
	}
	return nil
}

func (i *smtOldInterval) neg() {
	i.lower, i.upper = i.upper, i.lower
	i.lowerOpen, i.upperOpen = i.upperOpen, i.lowerOpen
	i.lowerDep, i.upperDep = i.upperDep, i.lowerDep
	i.lower.neg()
	i.upper.neg()
}
func (i *smtOldInterval) sub(other *smtOldInterval) error {
	if !i.valid(other) {
		return ast.ErrASTDomain
	}
	copy := other.clone()
	copy.neg()
	return i.add(copy)
}

func (i *smtOldInterval) containsZero() bool {
	return i != nil && (i.lower.sign() < 0 || i.lower.zero() && !i.lowerOpen) && (i.upper.sign() > 0 || i.upper.zero() && !i.upperOpen)
}
func (i *smtOldInterval) minusInfinity() bool { return i == nil || i.lower.infinite() }
func (i *smtOldInterval) plusInfinity() bool  { return i == nil || i.upper.infinite() }
func (i *smtOldInterval) lowerValue() (*big.Rat, error) {
	if i == nil || i.lower.infinite() {
		return nil, ast.ErrASTDomain
	}
	return new(big.Rat).Set(&i.lower.value), nil
}
func (i *smtOldInterval) upperValue() (*big.Rat, error) {
	if i == nil || i.upper.infinite() {
		return nil, ast.ErrASTDomain
	}
	return new(big.Rat).Set(&i.upper.value), nil
}
func (i *smtOldInterval) equal(other *smtOldInterval) bool {
	return i != nil && other != nil && i.lower.equal(&other.lower) && i.upper.equal(&other.upper) && i.lowerOpen == other.lowerOpen && i.upperOpen == other.upperOpen
}
func (i *smtOldInterval) contains(value *big.Rat) bool {
	if i == nil || value == nil {
		return false
	}
	if !i.lower.infinite() {
		c := value.Cmp(&i.lower.value)
		if c < 0 || c == 0 && i.lowerOpen {
			return false
		}
	}
	if !i.upper.infinite() {
		c := value.Cmp(&i.upper.value)
		if c > 0 || c == 0 && i.upperOpen {
			return false
		}
	}
	return true
}
func (i *smtOldInterval) zero() bool { return i != nil && i.lower.zero() && i.upper.zero() }
func (i *smtOldInterval) positiveOne() bool {
	return i.lower.sign() > 0 || i.lower.zero() && i.lowerOpen
}
func (i *smtOldInterval) negativeOne() bool {
	return i.upper.sign() < 0 || i.upper.zero() && i.upperOpen
}
func (i *smtOldInterval) positive() bool     { return i.lower.sign() >= 0 }
func (i *smtOldInterval) positiveZero() bool { return i.lower.zero() && !i.lowerOpen }
func (i *smtOldInterval) negative() bool     { return i.upper.sign() <= 0 }
func (i *smtOldInterval) negativeZero() bool { return i.upper.zero() && !i.upperOpen }
func (i *smtOldInterval) mixed() bool        { return i.lower.sign() < 0 && i.upper.sign() > 0 }

func smtExtendedProduct(left, right *smtExtendedRational) smtExtendedRational {
	result := left.clone()
	result.mul(right)
	return result
}
func (i *smtOldInterval) join3(a, b, c *smtArithmeticBoundDependency) *smtArithmeticBoundDependency {
	return i.manager.Join(i.manager.Join(a, b), c)
}
func (i *smtOldInterval) join4(a, b, c, d *smtArithmeticBoundDependency) *smtArithmeticBoundDependency {
	return i.manager.Join(i.manager.Join(a, b), i.manager.Join(c, d))
}
func (i *smtOldInterval) joinOptional(a, b, first, second *smtArithmeticBoundDependency) *smtArithmeticBoundDependency {
	if first == a || first == b || second == a || second == b || first == nil || second == nil {
		return i.manager.Join(a, b)
	}
	return i.join3(a, b, first)
}

func (i *smtOldInterval) mul(other *smtOldInterval) error {
	if !i.valid(other) {
		return ast.ErrASTDomain
	}
	if i.zero() {
		return nil
	}
	if other.zero() {
		*i = *other.clone()
		i.lowerDep = i.manager.Join(i.lowerDep, i.upperDep)
		i.upperDep = i.lowerDep
		return nil
	}
	a, b, c, d := i.lower, i.upper, other.lower, other.upper
	ao, bo, co, do := i.lowerOpen, i.upperOpen, other.lowerOpen, other.upperOpen
	ad, bd, cd, dd := i.lowerDep, i.upperDep, other.lowerDep, other.upperDep
	set := func(lower smtExtendedRational, lo bool, ld *smtArithmeticBoundDependency, upper smtExtendedRational, uo bool, ud *smtArithmeticBoundDependency) {
		i.lower, i.lowerOpen, i.lowerDep = lower, lo, ld
		i.upper, i.upperOpen, i.upperDep = upper, uo, ud
		if i.lower.infinite() {
			i.lowerDep = nil
		}
		if i.upper.infinite() {
			i.upperDep = nil
		}
	}
	if i.negative() {
		if other.negative() {
			set(smtExtendedProduct(&b, &d), !(i.negativeZero() || other.negativeZero()) && (bo || do), i.manager.Join(bd, dd), smtExtendedProduct(&a, &c), ao || co, i.joinOptional(ad, cd, bd, dd))
		} else if other.mixed() {
			set(smtExtendedProduct(&a, &d), ao || do, i.join3(ad, dd, bd), smtExtendedProduct(&a, &c), ao || co, i.join3(ad, cd, bd))
		} else {
			n0 := i.negativeZero()
			set(smtExtendedProduct(&a, &d), ao || do, i.joinOptional(ad, dd, bd, cd), smtExtendedProduct(&b, &c), !(n0 || other.positiveZero()) && (bo || co), i.manager.Join(bd, cd))
		}
	} else if i.mixed() {
		if other.negative() {
			set(smtExtendedProduct(&b, &c), bo || co, i.join3(bd, cd, dd), smtExtendedProduct(&a, &c), ao || co, i.join3(ad, cd, dd))
		} else if other.mixed() {
			adn, bcn, acn, bdn := smtExtendedProduct(&a, &d), smtExtendedProduct(&b, &c), smtExtendedProduct(&a, &c), smtExtendedProduct(&b, &d)
			adOpen, bcOpen, acOpen, bdOpen := ao || do, bo || co, ao || co, bo || do
			lower, lowerOpen := adn, adOpen
			if adn.cmp(&bcn) > 0 || adn.equal(&bcn) && adOpen && !bcOpen {
				lower, lowerOpen = bcn, bcOpen
			}
			upper, upperOpen := acn, acOpen
			if acn.cmp(&bdn) < 0 || acn.equal(&bdn) && acOpen && !bdOpen {
				upper, upperOpen = bdn, bdOpen
			}
			deps := i.join4(ad, bd, cd, dd)
			set(lower, lowerOpen, deps, upper, upperOpen, deps)
		} else {
			set(smtExtendedProduct(&a, &d), ao || do, i.join3(ad, dd, cd), smtExtendedProduct(&b, &d), bo || do, i.join3(bd, dd, cd))
		}
	} else {
		if !i.positive() {
			return ast.ErrASTDomain
		}
		if other.negative() {
			p0 := i.positiveZero()
			set(smtExtendedProduct(&b, &c), bo || co, i.joinOptional(bd, cd, ad, dd), smtExtendedProduct(&a, &d), !(p0 || other.negativeZero()) && (ao || do), i.manager.Join(ad, dd))
		} else if other.mixed() {
			set(smtExtendedProduct(&b, &c), bo || co, i.join3(bd, cd, ad), smtExtendedProduct(&b, &d), bo || do, i.join3(bd, dd, ad))
		} else {
			set(smtExtendedProduct(&a, &c), !(i.positiveZero() || other.positiveZero()) && (ao || co), i.manager.Join(ad, cd), smtExtendedProduct(&b, &d), bo || do, i.joinOptional(bd, dd, ad, cd))
		}
	}
	return nil
}

func (i *smtOldInterval) div(other *smtOldInterval) error {
	if !i.valid(other) || other.containsZero() {
		return ast.ErrASTDomain
	}
	if i.zero() {
		dependency := other.lowerDep
		if !other.positiveOne() {
			dependency = other.upperDep
		}
		i.lowerDep = i.manager.Join(i.lowerDep, dependency)
		i.upperDep = i.manager.Join(i.upperDep, dependency)
		return nil
	}
	copy := other.clone()
	if err := copy.inverse(); err != nil {
		return err
	}
	return i.mul(copy)
}

func (i *smtOldInterval) inverse() error {
	if i == nil || i.manager == nil || i.containsZero() {
		return ast.ErrASTDomain
	}
	if i.positiveOne() {
		newLower := i.upper
		if err := newLower.inverse(); err != nil {
			return err
		}
		newUpper := smtInfiniteExtended(true)
		if !i.lower.zero() {
			newUpper = i.lower
			if err := newUpper.inverse(); err != nil {
				return err
			}
		}
		i.lower, i.upper = newLower, newUpper
		i.lowerOpen, i.upperOpen = i.upperOpen, i.lowerOpen
		newUpperDep := i.lowerDep
		i.lowerDep = i.manager.Join(i.lowerDep, i.upperDep)
		i.upperDep = newUpperDep
		return nil
	}
	if i.negativeOne() {
		newUpper := i.lower
		if err := newUpper.inverse(); err != nil {
			return err
		}
		newLower := smtInfiniteExtended(false)
		if !i.upper.zero() {
			newLower = i.upper
			if err := newLower.inverse(); err != nil {
				return err
			}
		}
		i.lower, i.upper = newLower, newUpper
		i.lowerOpen, i.upperOpen = i.upperOpen, i.lowerOpen
		newLowerDep := i.upperDep
		i.upperDep = i.manager.Join(i.lowerDep, i.upperDep)
		i.lowerDep = newLowerDep
		return nil
	}
	return ast.ErrASTDomain
}

func (i *smtOldInterval) power(exponent uint32) {
	if i == nil || exponent == 1 {
		return
	}
	if exponent%2 != 0 {
		i.lower.power(exponent)
		i.upper.power(exponent)
		return
	}
	if i.lower.sign() > 0 {
		i.lower.power(exponent)
		i.upper.power(exponent)
		if !i.upper.infinite() {
			i.upperDep = i.manager.Join(i.lowerDep, i.upperDep)
		}
		return
	}
	if i.upper.sign() < 0 {
		i.lower, i.upper = i.upper, i.lower
		i.lowerOpen, i.upperOpen = i.upperOpen, i.lowerOpen
		i.lowerDep, i.upperDep = i.upperDep, i.lowerDep
		i.lower.power(exponent)
		i.upper.power(exponent)
		if !i.upper.infinite() {
			i.upperDep = i.manager.Join(i.lowerDep, i.upperDep)
		}
		return
	}
	i.lower.power(exponent)
	i.upper.power(exponent)
	if i.lower.cmp(&i.upper) > 0 || i.lower.equal(&i.upper) && !i.lowerOpen && i.upperOpen {
		i.upper, i.upperOpen = i.lower, i.lowerOpen
	}
	if !i.upper.infinite() {
		i.upperDep = i.manager.Join(i.lowerDep, i.upperDep)
	}
	i.lower = smtFiniteExtended(nil)
	i.lowerOpen, i.lowerDep = false, nil
}

func (i *smtOldInterval) String() string {
	if i == nil {
		return "<nil>"
	}
	left, right := "[", "]"
	if i.lowerOpen {
		left = "("
	}
	if i.upperOpen {
		right = ")"
	}
	return left + i.lower.String() + ", " + i.upper.String() + right
}
