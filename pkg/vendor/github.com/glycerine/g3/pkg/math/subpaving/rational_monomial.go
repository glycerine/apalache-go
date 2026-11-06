package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:657-664,690-724,
// concrete config_mpq monomial definitions and watches.

import (
	"math"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
)

type RationalPower struct {
	Variable uint32
	Degree   uint32
}

type RationalMonomial struct {
	variable uint32
	powers   []RationalPower
	visited  uint64
}

// DefineMonomial sorts and merges powers before registering one watch per
// factor. The source classifies the product as integer if any factor is integer.
func (c *RationalContext) DefineMonomial(powers []RationalPower) (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	if c.root != nil || len(powers) == 0 || len(c.variables) >= math.MaxUint32 {
		return 0, arena.ErrInvalidSlot
	}
	for _, p := range powers {
		if p.Variable >= uint32(len(c.variables)) {
			return 0, arena.ErrInvalidSlot
		}
	}
	merged := append([]RationalPower(nil), powers...)
	sort.Slice(merged, func(i, j int) bool { return merged[i].Variable < merged[j].Variable })
	n := 0
	for _, p := range merged {
		if n > 0 && merged[n-1].Variable == p.Variable {
			merged[n-1].Degree += p.Degree
		} else {
			merged[n] = p
			n++
		}
	}
	merged = merged[:n]
	integer := false
	for _, p := range merged {
		integer = integer || c.variables[p.Variable]
	}
	x, err := c.NewVariable(integer)
	if err != nil {
		return 0, err
	}
	if c.monomials == nil {
		c.monomials = make(map[uint32]*RationalMonomial)
	}
	c.monomials[x] = &RationalMonomial{variable: x, powers: merged}
	for _, p := range merged {
		c.watchOrder[p.Variable] = append(c.watchOrder[p.Variable], rationalWatch{definition: x})
	}
	return x, nil
}
