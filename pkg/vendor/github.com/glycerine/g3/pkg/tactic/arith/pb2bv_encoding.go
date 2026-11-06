package arith

// Source: src/tactic/arith/pb2bv_tactic.cpp bitblast_pbc/mk_pbc/split.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type pbLiteral struct {
	variable ast.ASTHandle
	negative bool
}
type pbMonomial struct {
	weight  big.Rat
	literal pbLiteral
}
type pbPolynomial []pbMonomial
type pbBitBinding struct{ declaration, positive, negative ast.ASTHandle }

// This concrete lexical trail keeps the source expr_ref/vector results alive
// until the caller publishes its output. It owns no solver or callbacks.
type pbEncodingScope struct {
	owner *PB2BVTactic
	refs  []ast.ASTRef
}

func (s *pbEncodingScope) close() (err error) {
	for i := len(s.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, s.owner.manager.DelRef(&s.refs[i]))
	}
	s.refs = nil
	return err
}
func (s *pbEncodingScope) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return h, err
	}
	r, e := s.owner.manager.NewRef(h)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	s.refs = append(s.refs, r)
	return h, nil
}
func (s *pbEncodingScope) gate(op int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	m := s.owner.manager
	r, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s.refs = append(s.refs, r)
	switch op {
	case ast.ASTOpAnd:
		err = s.owner.boolean.MkAnd(args, r)
	case ast.ASTOpNot:
		err = s.owner.boolean.MkNot(args[0], r)
	case ast.ASTOpIte:
		err = s.owner.boolean.MkIte(args[0], args[1], args[2], r)
	default:
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.RefGet(r)
}
func (t *PB2BVTactic) integerLiteral(v ast.ASTHandle, negative bool) (ast.ASTHandle, error) {
	m := t.manager
	decl, err := m.GetDecl(v)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if i, ok := t.bits[decl]; ok {
		if negative {
			return t.entries[i].negative, nil
		}
		return t.entries[i].positive, nil
	}
	b, err := m.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	fresh, err := m.MkFreshConstNullable(nil, b)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	not, err := m.MkNot(fresh)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, h := range []ast.ASTHandle{decl, fresh} {
		if err = m.IncRef(h); err != nil {
			return ast.ASTHandle{}, err
		}
		t.heldKeys = append(t.heldKeys, h)
	}
	if err = m.IncRef(not); err != nil {
		return ast.ASTHandle{}, err
	}
	t.heldNots = append(t.heldNots, not)
	t.bits[decl] = len(t.entries)
	t.entries = append(t.entries, pbBitBinding{decl, fresh, not})
	if negative {
		return not, nil
	}
	return fresh, nil
}
func (t *PB2BVTactic) monomialLiteral(l pbLiteral) (ast.ASTHandle, error) {
	return t.integerLiteral(l.variable, l.negative)
}

func (t *PB2BVTactic) allClauses(p pbPolynomial, c *big.Rat, s *pbEncodingScope) (ast.ASTHandle, error) {
	m := t.manager
	n := len(p)
	sums := make([]big.Rat, n)
	lits := make([]ast.ASTHandle, n)
	for i := n - 1; i >= 0; i-- {
		sums[i].Set(&p[i].weight)
		if i+1 < n {
			sums[i].Add(&sums[i], &sums[i+1])
		}
	}
	for i := range p {
		h, err := t.monomialLiteral(p[i].literal)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		lits[i] = h
	}
	var clauses, clause []ast.ASTHandle
	var process func(int, *big.Rat) error
	process = func(index int, bound *big.Rat) error {
		if bound.Sign() <= 0 {
			return nil
		}
		if index == n || sums[index].Cmp(bound) < 0 {
			var h ast.ASTHandle
			var err error
			switch len(clause) {
			case 0:
				h, err = m.MkFalse()
			case 1:
				h = clause[0]
			default:
				h, err = s.hold(m.MkOrArgs(clause))
			}
			if err != nil {
				return err
			}
			clauses = append(clauses, h)
			return nil
		}
		if err := t.checkpoint(); err != nil {
			return err
		}
		clause = append(clause, lits[index])
		if err := process(index+1, bound); err != nil {
			return err
		}
		clause = clause[:len(clause)-1]
		return process(index+1, new(big.Rat).Sub(bound, &p[index].weight))
	}
	if err := process(0, c); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.gate(ast.ASTOpAnd, clauses...)
}
func (t *PB2BVTactic) bitblast(p pbPolynomial, c *big.Rat, s *pbEncodingScope) (ast.ASTHandle, error) {
	m := t.manager
	cardinality := true
	for i := range p {
		if p[i].weight.Cmp(big.NewRat(1, 1)) != 0 {
			cardinality = false
			break
		}
	}
	n := len(p)
	if cardinality && big.NewRat(int64(n), 1).Cmp(c) < 0 {
		return m.MkFalse()
	}
	if cardinality && (c.Cmp(big.NewRat(1, 1)) == 0 || c.Cmp(big.NewRat(int64(n), 1)) == 0) {
		args := make([]ast.ASTHandle, n)
		for i := range p {
			h, err := t.monomialLiteral(p[i].literal)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			args[i] = h
		}
		if c.Cmp(big.NewRat(1, 1)) == 0 {
			return s.hold(m.MkOrArgs(args))
		}
		return s.gate(ast.ASTOpAnd, args...)
	}
	if uint64(n) <= uint64(t.allClausesLimit) {
		return t.allClauses(p, c, s)
	}
	if cardinality {
		if !c.IsInt() || !c.Num().IsUint64() || c.Num().Uint64() > uint64(^uint32(0)) || c.Sign() <= 0 || c.Cmp(big.NewRat(int64(n), 1)) >= 0 {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		k := uint32(c.Num().Uint64())
		rows := uint32(n) - k + 1
		// Source multiplies unsigned operands before assigning to uint64.
		cost := uint64(k * rows)
		if cost <= uint64(t.cardinalityLimit) {
			truth, err := m.MkTrue()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			falsity, err := m.MkFalse()
			if err != nil {
				return ast.ASTHandle{}, err
			}
			tmp := make([]ast.ASTHandle, rows)
			for i := range tmp {
				tmp[i] = truth
			}
			for i := uint32(0); i < k; i++ {
				for j := uint32(0); j < rows; j++ {
					lit, err := t.monomialLiteral(p[i+j].literal)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					otherwise := falsity
					if j != 0 {
						otherwise = tmp[j-1]
					}
					tmp[j], err = s.gate(ast.ASTOpIte, lit, tmp[j], otherwise)
					if err != nil {
						return ast.ASTHandle{}, err
					}
				}
			}
			result := tmp[len(tmp)-1]
			if err := t.traceExpression("pb2bv_bv", "bitblast_pbc", 421, "BV Cardinality: ", result, "\n"); err != nil {
				return ast.ASTHandle{}, err
			}
			return result, nil
		}
	}
	if err := t.tracePolynomial("pb2bv_bv_detail", "bitblast_pbc", 427, "encoding:\n", p, c, ""); err != nil {
		return ast.ASTHandle{}, err
	}
	var sum big.Rat
	for i := range p {
		if p[i].weight.Sign() <= 0 {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		sum.Add(&sum, &p[i].weight)
	}
	if sum.Cmp(c) < 0 {
		return m.MkFalse()
	}
	bits, err := lia2pbBits(&sum)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.traceBitWidth(bits, &sum, n); err != nil {
		return ast.ASTHandle{}, err
	}
	args := make([]ast.ASTHandle, 0, n)
	for i := range p {
		lit, err := t.monomialLiteral(p[i].literal)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		weight, err := s.hold(m.BitVectorNumeralRat(&p[i].weight, bits))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		zero, err := s.hold(m.BitVectorNumeralRat(new(big.Rat), bits))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		term, err := s.hold(m.MkIte(lit, weight, zero))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args = append(args, term)
	}
	family, err := m.RegisterBitVectorPlugin()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	lhs, err := s.hold(m.MkFamilyApp(family, 4, nil, args))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rhs, err := s.hold(m.BitVectorNumeralRat(c, bits))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return s.hold(m.MkBitVectorULE(rhs, lhs))
}
func (t *PB2BVTactic) split(p pbPolynomial, c *big.Rat) (pbPolynomial, pbPolynomial, error) {
	if len(p) <= 2 || c.Cmp(big.NewRat(1, 1)) == 0 || p[0].weight.Cmp(c) != 0 || p[1].weight.Cmp(c) != 0 {
		return p, nil, nil
	}
	i := 2
	for i < len(p) && p[i].weight.Cmp(c) == 0 {
		i++
	}
	if i == len(p) {
		return p, nil, nil
	}
	clause := make(pbPolynomial, 0, i+1)
	for j := 0; j < i; j++ {
		clause = append(clause, pbMonomial{weight: *big.NewRat(1, 1), literal: p[j].literal})
	}
	integer, err := t.manager.MkFamilySort(t.bounds.family, 1, nil)
	if err != nil {
		return nil, nil, err
	}
	fresh, err := t.manager.MkFreshConstNullable(nil, integer)
	if err != nil {
		return nil, nil, err
	}
	if err = t.manager.IncRef(fresh); err != nil {
		return nil, nil, err
	}
	t.temporaryInts = append(t.temporaryInts, fresh)
	clause = append(clause, pbMonomial{weight: *big.NewRat(1, 1), literal: pbLiteral{fresh, true}})
	remaining := make(pbPolynomial, 1, len(p)-i+1)
	remaining[0].weight.Set(c)
	remaining[0].literal = pbLiteral{fresh, false}
	remaining = append(remaining, p[i:]...)
	return remaining, clause, nil
}
func (t *PB2BVTactic) mkPBC(p pbPolynomial, c *big.Rat, split bool, s *pbEncodingScope) (ast.ASTHandle, error) {
	if err := t.tracePolynomial("mk_pbc", "mk_pbc", 507, "", p, c, ""); err != nil {
		return ast.ASTHandle{}, err
	}
	if c.Sign() <= 0 {
		return t.manager.MkTrue()
	}
	if len(p) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	gcd := new(big.Int).Set(p[0].weight.Num())
	if p[0].weight.Cmp(c) > 0 {
		gcd.Set(c.Num())
	}
	for i := range p {
		if !p[i].weight.IsInt() {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		if p[i].weight.Cmp(c) > 0 {
			p[i].weight.Set(c)
		}
		gcd.GCD(nil, nil, gcd, p[i].weight.Num())
	}
	if gcd.Sign() <= 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	g := new(big.Rat).SetInt(gcd)
	if gcd.Cmp(big.NewInt(1)) != 0 {
		for i := range p {
			p[i].weight.Quo(&p[i].weight, g)
		}
		q := new(big.Rat).Quo(c, g)
		v := new(big.Int).Quo(q.Num(), q.Denom())
		if new(big.Int).Mod(q.Num(), q.Denom()).Sign() != 0 {
			v.Add(v, big.NewInt(1))
		}
		c.SetInt(v)
	}
	if err := t.tracePolynomial("mk_pbc", "mk_pbc", 528, "GCD = "+g.RatString()+"; Normalized: ", p, c, "\n"); err != nil {
		return ast.ASTHandle{}, err
	}
	// The source's a_sum loop adds m_c rather than the coefficients. For any
	// nonempty polynomial this branch succeeds immediately; preserve it.
	if err := t.tracePolynomial("split_bug", "mk_pbc", 542, "", p, c, ""); err != nil {
		return ast.ASTHandle{}, err
	}
	var clause pbPolynomial
	var err error
	if split {
		p, clause, err = t.split(p, c)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if err = t.traceSplit(p, c, clause); err != nil {
		return ast.ASTHandle{}, err
	}
	first, err := t.bitblast(p, c, s)
	if err != nil || len(clause) == 0 {
		return first, err
	}
	second, err := t.bitblast(clause, big.NewRat(1, 1), s)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.tracePair("split_bug", "mk_pbc", 554, first, "\nAND\n", second, "\n"); err != nil {
		return ast.ASTHandle{}, err
	}
	return s.gate(ast.ASTOpAnd, first, second)
}
