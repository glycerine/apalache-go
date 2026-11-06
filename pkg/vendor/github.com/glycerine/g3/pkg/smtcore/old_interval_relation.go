package smtcore

// Source: src/muz/rel/dl_interval_relation.cpp:125-190.
// The relation plugin uses the same old intervals as arithmetic, without
// attaching bound dependencies.

import "math/big"

// RelationInterval is an exact rational interval. A nil lower or upper bound
// denotes negative or positive infinity, respectively.
type RelationInterval struct{ value *smtOldInterval }

func NewRelationInterval(lower *big.Rat, lowerOpen bool, upper *big.Rat, upperOpen bool) RelationInterval {
	m := &smtArithmeticBoundDependencyManager{}
	i, _ := newSMTOldInterval(m)
	if lower != nil {
		i.lower, i.lowerOpen = smtFiniteExtended(lower), lowerOpen
	}
	if upper != nil {
		i.upper, i.upperOpen = smtFiniteExtended(upper), upperOpen
	}
	return RelationInterval{i}
}

// Bounds returns independent copies of the finite endpoints.
func (i RelationInterval) Bounds() (lower *big.Rat, lowerOpen bool, upper *big.Rat, upperOpen bool) {
	if !i.value.lower.infinite() {
		lower = new(big.Rat).Set(&i.value.lower.value)
	}
	if !i.value.upper.infinite() {
		upper = new(big.Rat).Set(&i.value.upper.value)
	}
	return lower, i.value.lowerOpen, upper, i.value.upperOpen
}

func (i RelationInterval) Contains(value *big.Rat) bool { return i.value.contains(value) }

// Equal compares endpoints and their openness.
func (i RelationInterval) Equal(other RelationInterval) bool { return i.value.equal(other.value) }

// EmptyBounds follows the relation plugin's check for reversed endpoints.
func (i RelationInterval) EmptyBounds() bool { return i.value.upper.cmp(&i.value.lower) < 0 }

// Hull returns the least interval produced by the source union operation.
func (i RelationInterval) Hull(other RelationInterval) RelationInterval {
	a, b := i.value, other.value
	r := a.clone()
	if b.lower.cmp(&r.lower) < 0 || b.lower.equal(&r.lower) && r.lowerOpen {
		r.lower, r.lowerOpen = b.lower.clone(), b.lowerOpen
	}
	if b.upper.cmp(&r.upper) > 0 || b.upper.equal(&r.upper) && r.upperOpen {
		r.upper, r.upperOpen = b.upper.clone(), b.upperOpen
	}
	return RelationInterval{r}
}

// Widen follows the source's endpoint and openness rules.
func (i RelationInterval) Widen(other RelationInterval) RelationInterval {
	a, b := i.value, other.value
	r := a.clone()
	if b.lower.cmp(&r.lower) < 0 || b.lower.equal(&r.lower) && r.lowerOpen && !b.lowerOpen {
		r.lower, r.lowerOpen = smtInfiniteExtended(false), true
	}
	if b.upper.cmp(&r.upper) > 0 || b.upper.equal(&r.upper) && !r.upperOpen && b.upperOpen {
		r.upper, r.upperOpen = smtInfiniteExtended(true), true
	}
	return RelationInterval{r}
}

// Meet returns the intersection and reports a disjoint intersection.
func (i RelationInterval) Meet(other RelationInterval) (RelationInterval, bool) {
	a, b := i.value, other.value
	if a.upper.cmp(&a.lower) < 0 || b.minusInfinity() && b.plusInfinity() {
		return RelationInterval{a.clone()}, false
	}
	if b.upper.cmp(&b.lower) < 0 || a.minusInfinity() && a.plusInfinity() {
		return RelationInterval{b.clone()}, false
	}
	r := a.clone()
	if b.lower.cmp(&r.lower) > 0 || b.lower.equal(&r.lower) && !r.lowerOpen {
		r.lower, r.lowerOpen = b.lower.clone(), b.lowerOpen
	}
	if b.upper.cmp(&r.upper) < 0 || b.upper.equal(&r.upper) && !r.upperOpen {
		r.upper, r.upperOpen = b.upper.clone(), b.upperOpen
	}
	cmp := r.lower.cmp(&r.upper)
	if cmp > 0 || cmp == 0 && (r.lowerOpen || r.upperOpen) {
		return NewRelationInterval(nil, true, nil, true), true
	}
	return RelationInterval{r}, false
}
