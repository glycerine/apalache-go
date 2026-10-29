package arith

// Source: src/tactic/arith/pb2bv_tactic.cpp adjust/convert/is_eq_vector.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type pbConstraintKind uint8

const (
	pbEqual pbConstraintKind = iota
	pbGreaterEqual
	pbLessEqual
)

type pbTacticConfiguration struct {
	rewriter.DefaultConfiguration
	owner *PB2BVTactic
}

func (c *pbTacticConfiguration) MaxStepsExceeded(uint32) (bool, error) {
	return false, c.owner.checkpoint()
}
func (c *pbTacticConfiguration) GetSubst(term ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	t := c.owner
	yes, err := t.constraintCore(term)
	if err != nil || !yes {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if err = t.convert(term, t.saved, true, false); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	value, err := t.manager.RefGet(t.saved)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if err = t.tracePair("pb2bv_convert", "get_subst", 223, term, "\n-->\n", value, "\n"); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	return value, ast.ASTHandle{}, true, nil
}
func pbAdjust(positive *bool, kind *pbConstraintKind, c *big.Rat) {
	if !*positive {
		if *kind == pbLessEqual {
			*positive = true
			*kind = pbGreaterEqual
			c.Add(c, big.NewRat(1, 1))
		} else if *kind == pbGreaterEqual {
			*positive = true
			*kind = pbLessEqual
			c.Sub(c, big.NewRat(1, 1))
		}
	}
}
func (t *PB2BVTactic) numeral(h ast.ASTHandle) (q big.Rat, integer, found bool, err error) {
	text, integer, found, err := t.manager.ArithmeticNumeralText(h)
	if err != nil || !found {
		return q, integer, found, err
	}
	if _, ok := q.SetString(text); !ok {
		return q, false, false, ast.ErrASTDomain
	}
	return q, integer, true, nil
}
func (t *PB2BVTactic) addBoundsDependencies(h ast.ASTHandle) error {
	for _, lower := range []bool{true, false} {
		var dep ast.ASTDependency
		var err error
		if lower {
			dep, err = t.bounds.LowerDependency(h)
		} else {
			dep, err = t.bounds.UpperDependency(h)
		}
		if err != nil {
			return err
		}
		joined, err := t.manager.DependencyJoin(t.used, dep)
		if err != nil {
			return err
		}
		if err = t.manager.DependencyInc(joined); err != nil {
			return err
		}
		old := t.used
		t.used = joined
		if err = t.manager.DependencyDec(old); err != nil {
			return err
		}
	}
	return nil
}
func (t *PB2BVTactic) convert(term ast.ASTHandle, out ast.ASTRef, positive, root bool) (err error) {
	scope := &pbEncodingScope{owner: t}
	defer func() { err = errors.Join(err, scope.close()) }()
	value, err := t.convertCore(term, positive, root, scope)
	if err != nil {
		return err
	}
	return t.manager.SetRef(out, value)
}
func (t *PB2BVTactic) convertCore(term ast.ASTHandle, positive, root bool, s *pbEncodingScope) (ast.ASTHandle, error) {
	m := t.manager
	family, op, args, _, err := t.bounds.app(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(args) != 2 {
		return ast.ASTHandle{}, t.nonPB(term)
	}
	lhs, rhs := args[0], args[1]
	kind := pbEqual
	if family == m.BasicFamilyID() && op == ast.ASTOpEq {
		lc, e := t.bounds.constant(lhs)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rc, e := t.bounds.constant(rhs)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if lc && rc {
			if err = t.addBoundsDependencies(lhs); err != nil {
				return ast.ASTHandle{}, err
			}
			if err = t.addBoundsDependencies(rhs); err != nil {
				return ast.ASTHandle{}, err
			}
			first, e := t.integerLiteral(lhs, false)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			second, e := t.integerLiteral(rhs, !positive)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return s.hold(m.MkIff(first, second))
		}
	} else if family == t.bounds.family && op == 2 {
		kind = pbLessEqual
	} else if family == t.bounds.family && op == 3 {
		kind = pbGreaterEqual
	} else {
		return ast.ASTHandle{}, t.nonPB(term)
	}
	c, _, number, err := t.numeral(lhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if number {
		pbAdjust(&positive, &kind, &c)
		constant, e := t.bounds.constant(rhs)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !constant {
			return ast.ASTHandle{}, t.nonPB(term)
		}
		if err = t.addBoundsDependencies(rhs); err != nil {
			return ast.ASTHandle{}, err
		}
		if kind == pbEqual {
			sign := c.Sign() == 0
			if !positive {
				sign = !sign
			}
			return t.integerLiteral(rhs, sign)
		}
		if (c.Sign() == 0 && kind == pbLessEqual) || (c.Cmp(big.NewRat(1, 1)) == 0 && kind == pbGreaterEqual) {
			if err = t.traceDiscard(term, 649); err != nil {
				return ast.ASTHandle{}, err
			}
			return m.MkTrue()
		}
		if !positive || !((c.Sign() == 0 && kind == pbGreaterEqual) || (c.Cmp(big.NewRat(1, 1)) == 0 && kind == pbLessEqual)) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		return t.integerLiteral(rhs, kind == pbGreaterEqual)
	}
	c, integer, number, err := t.numeral(rhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !number || !integer {
		return ast.ASTHandle{}, t.nonPB(term)
	}
	pbAdjust(&positive, &kind, &c)
	constant, err := t.bounds.constant(lhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if constant {
		if err = t.addBoundsDependencies(lhs); err != nil {
			return ast.ASTHandle{}, err
		}
		if kind == pbEqual {
			if err = t.traceNumeral("pb2bv_bug", "convert", 673, "c: ", &c, "\n"); err != nil {
				return ast.ASTHandle{}, err
			}
			if c.Sign() != 0 && c.Cmp(big.NewRat(1, 1)) != 0 {
				if positive {
					return m.MkFalse()
				}
				return m.MkTrue()
			}
			sign := c.Sign() == 0
			if !positive {
				sign = !sign
			}
			return t.integerLiteral(lhs, sign)
		}
		if kind == pbLessEqual {
			if c.Cmp(big.NewRat(1, 1)) >= 0 {
				if err = t.traceDiscard(term, 691); err != nil {
					return ast.ASTHandle{}, err
				}
				return m.MkTrue()
			}
			if c.Sign() < 0 {
				return m.MkFalse()
			}
		} else {
			if c.Sign() <= 0 {
				if err = t.traceDiscard(term, 704); err != nil {
					return ast.ASTHandle{}, err
				}
				return m.MkTrue()
			}
			if c.Cmp(big.NewRat(1, 1)) > 0 {
				return m.MkFalse()
			}
		}
		if !positive || !((c.Sign() == 0 && kind == pbLessEqual) || (c.Cmp(big.NewRat(1, 1)) == 0 && kind == pbGreaterEqual)) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		return t.integerLiteral(lhs, kind == pbLessEqual)
	}
	family, op, terms, app, err := t.bounds.app(lhs)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !app || family != t.bounds.family || op != 6 {
		return ast.ASTHandle{}, t.nonPB(term)
	}
	// Validate every monomial before collecting any bound dependencies.
	for _, term := range terms {
		constant, e := t.bounds.constant(term)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if constant {
			continue
		}
		f, k, a, ok, e := t.bounds.app(term)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !ok || f != t.bounds.family || k != 9 || len(a) != 2 {
			return ast.ASTHandle{}, t.nonPB(term)
		}
		_, _, num, e := t.numeral(a[0])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		constant, e = t.bounds.constant(a[1])
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if !num || !constant {
			return ast.ASTHandle{}, t.nonPB(term)
		}
	}
	p := make(pbPolynomial, 0, len(terms))
	for _, term := range terms {
		constant, e := t.bounds.constant(term)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		weight := big.NewRat(1, 1)
		variable := term
		negative := false
		if !constant {
			_, _, a, _, e := t.bounds.app(term)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			n, _, _, e := t.numeral(a[0])
			if e != nil {
				return ast.ASTHandle{}, e
			}
			weight.Set(&n)
			variable = a[1]
			if weight.Sign() < 0 {
				weight.Neg(weight)
				c.Add(&c, weight)
				negative = true
			}
		}
		if err = t.addBoundsDependencies(variable); err != nil {
			return ast.ASTHandle{}, err
		}
		p = append(p, pbMonomial{weight: *weight, literal: pbLiteral{variable, negative}})
	}
	sort.SliceStable(p, func(i, j int) bool { return p[i].weight.Cmp(&p[j].weight) > 0 })
	if kind == pbGreaterEqual {
		return t.mkPBC(p, &c, root, s)
	}
	if kind == pbLessEqual {
		c.Neg(&c)
		for i := range p {
			c.Add(&c, &p[i].weight)
			p[i].literal.negative = !p[i].literal.negative
		}
		return t.mkPBC(p, &c, root, s)
	}
	eqVector, err := t.equalVector(p, &c)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if eqVector {
		if err = t.traceText("is_eq_vector", "convert", 796, "found eq vector\n"); err != nil {
			return ast.ASTHandle{}, err
		}
		eqs := make([]ast.ASTHandle, 0, len(p)/2)
		for i := 0; i < len(p); i += 2 {
			a, e := t.integerLiteral(p[i].literal.variable, false)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			b, e := t.integerLiteral(p[i+1].literal.variable, false)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			eq, e := s.hold(m.MkEq(a, b))
			if e != nil {
				return ast.ASTHandle{}, e
			}
			eqs = append(eqs, eq)
		}
		value, e := s.gate(ast.ASTOpAnd, eqs...)
		if e != nil || positive {
			return value, e
		}
		return s.gate(ast.ASTOpNot, value)
	}
	p2 := make(pbPolynomial, len(p))
	c2 := new(big.Rat).Neg(&c)
	for i := range p {
		p2[i].weight.Set(&p[i].weight)
		p2[i].literal = p[i].literal
		p2[i].literal.negative = !p2[i].literal.negative
		c2.Add(c2, &p[i].weight)
	}
	first, err := t.mkPBC(p, &c, false, s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	second, err := t.mkPBC(p2, c2, false, s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.traceEquality(term, p, &c, p2, c2, first, second); err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := s.gate(ast.ASTOpAnd, first, second)
	if err != nil || positive {
		return value, err
	}
	return s.gate(ast.ASTOpNot, value)
}
func (t *PB2BVTactic) equalVector(p pbPolynomial, c *big.Rat) (bool, error) {
	if err := t.tracePolynomial("is_eq_vector", "is_eq_vector", 586, "", p, c, ""); err != nil {
		return false, err
	}
	if len(p)%2 != 0 {
		return false, nil
	}
	n := len(p) / 2
	bound := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(n)), big.NewInt(1))
	if c.Cmp(new(big.Rat).SetInt(bound)) != 0 {
		return false, nil
	}
	for i := 0; i < n; i++ {
		a, b := &p[2*i], &p[2*i+1]
		if a.literal.negative == b.literal.negative || a.weight.Cmp(&b.weight) != 0 || a.weight.Cmp(new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(n-i-1)))) != 0 {
			return false, nil
		}
	}
	return true, nil
}
