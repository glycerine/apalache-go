package smtcore

// Source: theory_pb::arg_t and pb_rewriter_util<pb_lit_rewriter_util>.

import (
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtPBTerm struct {
	literal SMTLiteral
	coeff   big.Rat
}

type smtPBArgs struct {
	terms []smtPBTerm
	bound big.Rat
}

func (a *smtPBArgs) unique() {
	for i := range a.terms {
		if a.terms[i].literal.Sign() {
			a.terms[i].literal = a.terms[i].literal.Negated()
			a.bound.Sub(&a.bound, &a.terms[i].coeff)
			a.terms[i].coeff.Neg(&a.terms[i].coeff)
		}
	}
	for i := 0; i < len(a.terms); {
		l := a.terms[i].literal
		if l == SMTTrueLiteral {
			a.bound.Sub(&a.bound, &a.terms[i].coeff)
		}
		if l == SMTTrueLiteral || l == SMTFalseLiteral {
			a.terms[i] = a.terms[len(a.terms)-1]
			a.terms = a.terms[:len(a.terms)-1]
		} else {
			i++
		}
	}
	sort.Slice(a.terms, func(i, j int) bool { return a.terms[i].literal.Index() < a.terms[j].literal.Index() })
	out := make([]smtPBTerm, 0, len(a.terms))
	for _, term := range a.terms {
		if len(out) > 0 && out[len(out)-1].literal == term.literal {
			out[len(out)-1].coeff.Add(&out[len(out)-1].coeff, &term.coeff)
		} else {
			var c big.Rat
			c.Set(&term.coeff)
			out = append(out, smtPBTerm{term.literal, c})
		}
	}
	a.terms = nil
	for _, term := range out {
		if term.coeff.Sign() != 0 {
			a.terms = append(a.terms, term)
		}
	}
}

func smtPBLCM(a, b *big.Int) *big.Int {
	g := new(big.Int).GCD(nil, nil, a, b)
	return new(big.Int).Mul(new(big.Int).Quo(a, g), b)
}

func smtPBFloor(q *big.Rat) *big.Int {
	n, r := new(big.Int), new(big.Int)
	n.QuoRem(q.Num(), q.Denom(), r)
	if q.Sign() < 0 && r.Sign() != 0 {
		n.Sub(n, big.NewInt(1))
	}
	return n
}

func smtPBCeil(q *big.Rat) *big.Int {
	n := smtPBFloor(new(big.Rat).Neg(q))
	return n.Neg(n)
}

func (a *smtPBArgs) unit(bound int64) {
	for i := range a.terms {
		a.terms[i].coeff.SetInt64(1)
	}
	a.bound.SetInt64(bound)
}

func (a *smtPBArgs) normalize(eq bool) lbool.LBool {
	sum := new(big.Rat)
	for i := range a.terms {
		term := &a.terms[i]
		if term.coeff.Sign() < 0 {
			a.bound.Sub(&a.bound, &term.coeff)
			term.coeff.Neg(&term.coeff)
			term.literal = term.literal.Negated()
		}
		sum.Add(sum, &term.coeff)
	}
	if !eq && a.bound.Sign() <= 0 {
		a.terms = nil
		a.bound.SetInt64(0)
		return lbool.True
	}
	if eq && a.bound.Sign() == 0 && len(a.terms) == 0 {
		return lbool.True
	}
	// With no literals there will be no assignment callback. Fold the
	// impossible equality now instead of leaving an inert constraint.
	if eq && len(a.terms) == 0 && a.bound.Sign() != 0 {
		return lbool.False
	}
	if sum.Cmp(&a.bound) < 0 {
		a.terms = nil
		a.bound.SetInt64(1)
		return lbool.False
	}
	if eq && a.bound.Cmp(sum) == 0 {
		a.unit(int64(len(a.terms)))
		return lbool.Undef
	}
	den := new(big.Int).Set(a.bound.Denom())
	for i := range a.terms {
		den = smtPBLCM(den, a.terms[i].coeff.Denom())
	}
	if den.Cmp(big.NewInt(1)) != 0 {
		factor := new(big.Rat).SetInt(den)
		a.bound.Mul(&a.bound, factor)
		for i := range a.terms {
			a.terms[i].coeff.Mul(&a.terms[i].coeff, factor)
		}
	}
	if eq {
		return lbool.Undef
	}
	sum.SetInt64(0)
	for i := range a.terms {
		if a.terms[i].coeff.Cmp(&a.bound) > 0 {
			a.terms[i].coeff.Set(&a.bound)
		}
		sum.Add(sum, &a.terms[i].coeff)
	}
	if sum.Cmp(&a.bound) == 0 {
		a.unit(int64(len(a.terms)))
	}
	g := new(big.Int)
	for i := range a.terms {
		if a.terms[i].coeff.Cmp(&a.bound) != 0 {
			g.GCD(nil, nil, g, a.terms[i].coeff.Num())
		}
	}
	if g.Sign() == 0 {
		a.unit(1)
	} else if g.Cmp(big.NewInt(1)) > 0 {
		bound := new(big.Rat).Set(&a.bound)
		k := smtPBCeil(new(big.Rat).Quo(bound, new(big.Rat).SetInt(g)))
		for i := range a.terms {
			if a.terms[i].coeff.Cmp(bound) == 0 {
				a.terms[i].coeff.SetInt(k)
			} else {
				a.terms[i].coeff.Quo(&a.terms[i].coeff, new(big.Rat).SetInt(g))
			}
		}
		a.bound.SetInt(k)
	}
	if a.bound.Cmp(big.NewRat(1, 1)) != 0 && len(a.terms) > 0 {
		min, max := new(big.Rat).Set(&a.terms[0].coeff), new(big.Rat).Set(&a.terms[0].coeff)
		for i := range a.terms {
			c := &a.terms[i].coeff
			if c.Cmp(min) < 0 {
				min.Set(c)
			}
			if c.Cmp(max) > 0 {
				max.Set(c)
			}
		}
		n0 := new(big.Rat).Quo(&a.bound, max)
		n1 := smtPBFloor(n0)
		n2 := smtPBCeil(new(big.Rat).Quo(&a.bound, min))
		n2.Sub(n2, big.NewInt(1))
		if n1.Cmp(n2) == 0 && !n0.IsInt() {
			a.unit(1)
			a.bound.SetInt(n1.Add(n1, big.NewInt(1)))
		}
	}
	return lbool.Undef
}

func (a *smtPBArgs) prune(eq bool) {
	if eq {
		return
	}
	sum := new(big.Rat)
	count := 0
	for i := 0; sum.Cmp(&a.bound) < 0 && i < len(a.terms); i++ {
		if a.terms[i].coeff.Cmp(&a.bound) < 0 {
			sum.Add(sum, &a.terms[i].coeff)
			count++
		}
	}
	if count > 0 && sum.Cmp(&a.bound) < 0 {
		for i := 0; i < len(a.terms); {
			if a.terms[i].coeff.Cmp(&a.bound) < 0 {
				a.terms[i] = a.terms[len(a.terms)-1]
				a.terms = a.terms[:len(a.terms)-1]
			} else {
				i++
			}
		}
		a.unique()
		a.normalize(false)
	}
}

func (a *smtPBArgs) negate() error {
	sum := new(big.Rat)
	for i := range a.terms {
		a.terms[i].literal = a.terms[i].literal.Negated()
		sum.Add(sum, &a.terms[i].coeff)
	}
	a.bound.Sub(sum, &a.bound)
	a.bound.Add(&a.bound, big.NewRat(1, 1))
	// VERIFY evaluates normalize in every build, then checks its result
	// only in debug builds. Preserve normalization before reporting failure.
	truth := a.normalize(false)
	if smtcoreSourceDebug && truth != lbool.Undef {
		return ast.ErrASTDomain
	}
	return nil
}

func (a *smtPBArgs) copy() smtPBArgs {
	var out smtPBArgs
	out.bound.Set(&a.bound)
	for i := range a.terms {
		var c big.Rat
		c.Set(&a.terms[i].coeff)
		out.terms = append(out.terms, smtPBTerm{a.terms[i].literal, c})
	}
	return out
}
