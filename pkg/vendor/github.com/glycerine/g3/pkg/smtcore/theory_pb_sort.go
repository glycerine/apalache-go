package smtcore

// Source: src/util/sorting_network.h, retained psort_nw<theory_pb::psort_expr>
// instantiation reached by src/smt/theory_pb.cpp:compile_ineq.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

const (
	smtPBSortLE uint8 = iota
	smtPBSortGE
	smtPBSortGEFull
	smtPBSortLEFull
)

// This specialization operates on SAT literals. Its auxiliary AST applications
// name Boolean variables; their definitions are supplied by the network clauses,
// without recursively internalizing them as ordinary Boolean gates.
type smtPBSort struct {
	theory                              *SMTPseudoBooleanTheory
	mode                                uint8
	variables, clauses, clauseVariables uint32
	err                                 error
}

func (s *smtPBSort) fresh() SMTLiteral {
	if s.err != nil {
		return SMTNullLiteral
	}
	s.variables++
	e, err := s.theory.state.manager.MkFreshPseudoBooleanAux()
	if err != nil {
		s.err = err
		return SMTNullLiteral
	}
	v, err := s.theory.state.MkBoolVar(e)
	s.err = err
	return NewSMTLiteral(v, false)
}

func (s *smtPBSort) extreme(a, b SMTLiteral, maximum bool) SMTLiteral {
	if a == b {
		return a
	}
	if s.err != nil {
		return SMTNullLiteral
	}
	s.variables++
	state := s.theory.state
	args, err := state.clauseLiteralExpressions([]SMTLiteral{a, b})
	if err != nil {
		s.err = err
		return SMTNullLiteral
	}
	var e ast.ASTHandle
	if maximum {
		e, err = state.manager.MkOr(args[0], args[1])
	} else {
		e, err = state.manager.MkAnd(args[0], args[1])
	}
	if err != nil {
		s.err = err
		return SMTNullLiteral
	}
	v, found := state.BoolVar(e)
	if !found {
		v, err = state.MkBoolVar(e)
	}
	s.err = err
	return NewSMTLiteral(v, false)
}

func (s *smtPBSort) add(ls ...SMTLiteral) {
	if s.err != nil {
		return
	}
	s.clauses++
	s.clauseVariables += uint32(len(ls))
	s.trace("add_clause", 561, func(out io.Writer) error { return smtPBSortTraceVector(out, "", ls) })
	if s.err != nil {
		return
	}
	s.err = s.theory.compilationClause(ls, SMTClauseAux)
}

func (s *smtPBSort) compare(a, b, hi, lo SMTLiteral) {
	if s.mode == smtPBSortLE || s.mode == smtPBSortLEFull {
		s.add(a.Negated(), hi)
		s.add(b.Negated(), hi)
		s.add(a.Negated(), b.Negated(), lo)
	} else {
		s.add(lo.Negated(), a)
		s.add(lo.Negated(), b)
		s.add(hi.Negated(), a, b)
	}
}

func smtPBSortNot(xs []SMTLiteral) []SMTLiteral {
	ys := make([]SMTLiteral, len(xs))
	for i, x := range xs {
		ys[i] = x.Negated()
	}
	return ys
}

func (s *smtPBSort) ge(full bool, k uint32, xs []SMTLiteral) SMTLiteral {
	n := uint32(len(xs))
	if k > n {
		return SMTFalseLiteral
	}
	if k == 0 {
		return SMTTrueLiteral
	}
	if 2*k > n {
		return s.le(full, n-k, s.dualize(k, n, xs))
	}
	s.mode = smtPBSortGE
	if full {
		s.mode = smtPBSortGEFull
	}
	out := s.card(k, xs)
	if s.err != nil {
		return SMTNullLiteral
	}
	return out[k-1]
}

func (s *smtPBSort) le(full bool, k uint32, xs []SMTLiteral) SMTLiteral {
	n := uint32(len(xs))
	if k >= n {
		return SMTTrueLiteral
	}
	if 2*k > n {
		return s.ge(full, n-k, s.dualize(k, n, xs))
	}
	if k == 1 {
		return s.atMostOne(full, xs)
	}
	s.mode = smtPBSortLE
	if full {
		s.mode = smtPBSortLEFull
	}
	out := s.card(k+1, xs)
	if s.err != nil {
		return SMTNullLiteral
	}
	return out[k].Negated()
}

func (s *smtPBSort) or(xs []SMTLiteral) SMTLiteral {
	if len(xs) == 1 {
		return xs[0]
	}
	y := s.fresh()
	s.add(append(append([]SMTLiteral{}, xs...), y.Negated())...)
	for _, x := range xs {
		s.add(x.Negated(), y)
	}
	return y
}

func (s *smtPBSort) atMostOne(full bool, xs []SMTLiteral) SMTLiteral {
	s.trace("mk_at_most_1", 340, func(out io.Writer) error {
		kind := "partial"
		if full {
			kind = "full"
		}
		_, _ = fmt.Fprintf(out, "%s ", kind)
		return smtPBSortTraceVector(out, "", xs)
	})
	in := append([]SMTLiteral{}, xs...)
	result := s.fresh()
	ands := []SMTLiteral{result}
	incSize := 4
	for len(in) != 0 {
		var ors []SMTLiteral
		n := len(in)
		if n+1 == incSize {
			incSize++
		}
		for i := 0; i < n; i += incSize {
			group := in[i:min(n, i+incSize)]
			if !s.require(len(group) > 0) {
				return SMTNullLiteral
			}
			if len(group) > 1 {
				for j, a := range group {
					for _, b := range group[j+1:] {
						s.add(result.Negated(), a.Negated(), b.Negated())
					}
				}
				if full {
					y := s.fresh()
					for j := range group {
						ls := []SMTLiteral{y}
						for k, x := range group {
							if k != j {
								ls = append(ls, x)
							}
						}
						s.add(ls...)
					}
					ands = append(ands, y.Negated())
				}
			}
			if n > incSize {
				ors = append(ors, s.or(group))
			}
		}
		if n <= incSize {
			break
		}
		in = ors
	}
	if full {
		s.add(ands...)
	}
	return result
}

func (s *smtPBSort) card(k uint32, xs []SMTLiteral) (out []SMTLiteral) {
	n := uint32(len(xs))
	s.traceCard(598, k, n, nil, nil, false)
	defer func() { s.traceCard(612, k, n, xs, out, true) }()
	if s.err != nil {
		return nil
	}
	if n <= k {
		return s.sort(xs)
	}
	if s.useDirectCard(k, n) {
		return s.directSort(k, xs)
	}
	l := n / 2
	a, b := s.card(k, xs[:l]), s.card(k, xs[l:])
	return s.smallMerge(k, a, b)
}

func (s *smtPBSort) sort(xs []SMTLiteral) (out []SMTLiteral) {
	n := uint32(len(xs))
	s.traceSorting(744, xs, nil, false)
	defer func() { s.traceSorting(769, xs, out, true) }()
	if s.err != nil {
		return nil
	}
	if n < 2 {
		return append([]SMTLiteral{}, xs...)
	}
	if n == 2 {
		return s.merge(xs[:1], xs[1:])
	}
	if s.useDirectSort(n) {
		return s.directSort(n, xs)
	}
	l := n / 2
	a, b := s.sort(xs[:l]), s.sort(xs[l:])
	return s.merge(a, b)
}

func smtPBSortSplit(xs []SMTLiteral) (even, odd []SMTLiteral) {
	for i, x := range xs {
		if i%2 == 0 {
			even = append(even, x)
		} else {
			odd = append(odd, x)
		}
	}
	return
}

func (s *smtPBSort) merge(as, bs []SMTLiteral) (out []SMTLiteral) {
	a, b := uint32(len(as)), uint32(len(bs))
	s.traceMerge("merge", 640, 0, as, bs, nil, false)
	defer func() { s.traceMerge("merge", 675, 0, as, bs, out, true) }()
	if s.err != nil {
		return nil
	}
	if a == 1 && b == 1 {
		hi, lo := s.extreme(as[0], bs[0], true), s.extreme(as[0], bs[0], false)
		s.compare(as[0], bs[0], hi, lo)
		return []SMTLiteral{hi, lo}
	}
	if a == 0 {
		return append([]SMTLiteral{}, bs...)
	}
	if b == 0 {
		return append([]SMTLiteral{}, as...)
	}
	if s.useDirectMerge(a, b, a+b) {
		return s.directMerge(a+b, as, bs)
	}
	if a%2 == 0 && b%2 == 1 {
		return s.merge(bs, as)
	}
	ea, oa := smtPBSortSplit(as)
	eb, ob := smtPBSortSplit(bs)
	if !s.require((a > 1 || b > 1) && len(ea) != 0 && len(eb) != 0) {
		return nil
	}
	x, y := s.merge(ea, eb), s.merge(oa, ob)
	return s.interleave(x, y)
}

func (s *smtPBSort) interleave(as, bs []SMTLiteral) []SMTLiteral {
	s.traceMerge("interleave", 712, 0, as, bs, nil, false)
	if !s.require(len(as) >= len(bs) && len(as) <= len(bs)+2 && len(as) != 0) {
		return nil
	}
	out := []SMTLiteral{as[0]}
	sz := min(len(as)-1, len(bs))
	for i := 0; i < sz; i++ {
		hi, lo := s.extreme(as[i+1], bs[i], true), s.extreme(as[i+1], bs[i], false)
		s.compare(as[i+1], bs[i], hi, lo)
		out = append(out, hi, lo)
	}
	if len(as) == len(bs) {
		out = append(out, bs[sz])
	} else if len(as) == len(bs)+2 {
		out = append(out, as[sz+1])
	}
	if !s.require(len(out) == len(as)+len(bs)) {
		return nil
	}
	s.traceMerge("interleave", 732, 0, as, bs, out, true)
	return out
}

func (s *smtPBSort) maxClauses(a, b, y SMTLiteral) {
	if s.mode != smtPBSortGE {
		s.add(a.Negated(), y)
		s.add(b.Negated(), y)
	}
	if s.mode != smtPBSortLE {
		s.add(y.Negated(), a, b)
	}
}

func (s *smtPBSort) smallMerge(c uint32, as, bs []SMTLiteral) (out []SMTLiteral) {
	a, b := uint32(len(as)), uint32(len(bs))
	s.traceMerge("smerge", 805, c, as, bs, nil, false)
	defer func() {
		s.traceMerge("smerge", 879, c, as, bs, out, true)
		s.require(uint32(len(out)) == min(a+b, c))
	}()
	if s.err != nil {
		return nil
	}
	if a == 1 && b == 1 && c == 1 {
		y := s.extreme(as[0], bs[0], true)
		s.maxClauses(as[0], bs[0], y)
		return []SMTLiteral{y}
	}
	if a == 0 {
		return append([]SMTLiteral{}, bs[:min(c, b)]...)
	}
	if b == 0 {
		return append([]SMTLiteral{}, as[:min(c, a)]...)
	}
	if a > c {
		return s.smallMerge(c, as[:c], bs)
	}
	if b > c {
		return s.smallMerge(c, as, bs[:c])
	}
	if a+b <= c {
		return s.merge(as, bs)
	}
	if s.useDirectMerge(a, b, c) {
		return s.directMerge(c, as, bs)
	}
	ea, oa := smtPBSortSplit(as)
	eb, ob := smtPBSortSplit(bs)
	if !s.require(len(ea) != 0 && len(eb) != 0) {
		return nil
	}
	c1, c2 := (c+1)/2, (c-1)/2
	if c%2 == 0 {
		c1, c2 = 1+c/2, c/2
	}
	x, y := s.smallMerge(c1, ea, eb), s.smallMerge(c2, oa, ob)
	if !s.require(uint32(len(x)) == min(uint32(len(ea)+len(eb)), c1) && uint32(len(y)) == min(uint32(len(oa)+len(ob)), c2)) {
		return nil
	}
	var last SMTLiteral
	if c%2 == 0 {
		a, b := x[len(x)-1], y[len(y)-1]
		x, y = x[:len(x)-1], y[:len(y)-1]
		last = s.extreme(a, b, true)
		s.maxClauses(a, b, last)
	}
	out = s.interleave(x, y)
	if c%2 == 0 {
		out = append(out, last)
	}
	return out
}

func (s *smtPBSort) directMerge(c uint32, as, bs []SMTLiteral) []SMTLiteral {
	a, b := uint32(len(as)), uint32(len(bs))
	s.traceMerge("dsmerge", 923, c, as, bs, nil, false)
	if !s.require(a <= c && b <= c && a+b >= c) {
		return nil
	}
	out := make([]SMTLiteral, c)
	for i := range out {
		out[i] = s.fresh()
	}
	if s.mode != smtPBSortGE {
		for i, x := range as {
			s.add(x.Negated(), out[i])
		}
		for i, x := range bs {
			s.add(x.Negated(), out[i])
		}
		for i := uint32(1); i <= a; i++ {
			for j := uint32(1); j <= b && i+j <= c; j++ {
				s.add(as[i-1].Negated(), bs[j-1].Negated(), out[i+j-1])
			}
		}
	}
	if s.mode != smtPBSortLE {
		for k := uint32(0); k < c; k++ {
			if a <= k {
				s.add(out[k].Negated(), bs[k-a])
			}
			if b <= k {
				s.add(out[k].Negated(), as[k-b])
			}
			for i := uint32(0); i < min(a, k+1); i++ {
				j := k - i
				if !s.require(i+j == k) {
					return nil
				}
				if j < b {
					s.add(out[k].Negated(), as[i], bs[j])
				}
			}
		}
	}
	return out
}

func (s *smtPBSort) directSort(m uint32, xs []SMTLiteral) []SMTLiteral {
	s.trace("dsorting", 982, func(out io.Writer) error {
		_, err := fmt.Fprintf(out, "dsorting m: %d n: %d\n", m, len(xs))
		return err
	})
	if !s.require(m <= uint32(len(xs))) {
		return nil
	}
	out := make([]SMTLiteral, m)
	for i := range out {
		out[i] = s.fresh()
	}
	if s.mode != smtPBSortGE {
		for k := uint32(1); k <= m; k++ {
			s.subset(true, k, 0, []SMTLiteral{out[k-1]}, xs)
		}
	}
	if s.mode != smtPBSortLE {
		for k := uint32(1); k <= m; k++ {
			s.subset(false, uint32(len(xs))-k+1, 0, []SMTLiteral{out[k-1].Negated()}, xs)
		}
	}
	return out
}

func (s *smtPBSort) subset(negative bool, k, offset uint32, ls, xs []SMTLiteral) {
	s.trace("add_subset", 1017, func(out io.Writer) error {
		_, _ = fmt.Fprintf(out, "k:%d offset: %d n: %d ", k, offset, len(xs))
		return smtPBSortTraceVector(out, "", ls)
	})
	if !s.require(k+offset <= uint32(len(xs))) {
		return
	}
	if k == 0 {
		s.add(ls...)
		return
	}
	for i := offset; i < uint32(len(xs))-k+1; i++ {
		x := xs[i]
		if negative {
			x = x.Negated()
		}
		s.subset(negative, k-1, i+1, append(ls, x), xs)
	}
}

// Source cost arithmetic is unsigned, including intermediate wrapping. Costs
// retain the source's estimates (which intentionally differ from actual counts).
func smtPBSortCeil(n uint32) uint32 { return n/2 + n%2 }
func (s *smtPBSort) costDirectSort(m, n uint32) uint32 {
	if !s.require(m <= n && n < 10) {
		return 0
	}
	c := 5 * m
	if s.mode != smtPBSortGE {
		c += uint32(1) << (n - 1)
	}
	if s.mode != smtPBSortLE {
		c += uint32(1) << (n - 1)
	}
	return c
}
func (s *smtPBSort) costDirectMerge(a, b, c uint32) uint32 {
	v := 5 * c
	if s.mode != smtPBSortGE {
		v += a + b + min(a, c)*min(b, c)/2
	}
	if s.mode != smtPBSortLE {
		v += min(a, c) * min(b, c) / 2
	}
	return v
}
func (s *smtPBSort) useDirectSort(n uint32) bool {
	if !s.require(n > 2) {
		return false
	}
	return n < 10 && s.costDirectSort(n, n) < s.costSortRecursive(n)
}
func (s *smtPBSort) costSort(n uint32) uint32 {
	if n < 2 {
		return 0
	}
	if n == 2 {
		return 13
	}
	if s.useDirectSort(n) {
		return s.costDirectSort(n, n)
	}
	return s.costSortRecursive(n)
}
func (s *smtPBSort) costSortRecursive(n uint32) uint32 {
	if !s.require(n > 2) {
		return 0
	}
	l := n / 2
	return s.costSort(l) + s.costSort(n-l) + s.costMerge(l, n-l)
}
func (s *smtPBSort) useDirectCard(k, n uint32) bool {
	return n < 10 && s.costDirectSort(k, n) < s.costCardRecursive(k, n)
}
func (s *smtPBSort) costCard(k, n uint32) uint32 {
	if n <= k {
		return s.costSort(n)
	}
	if s.useDirectCard(k, n) {
		return s.costDirectSort(k, n)
	}
	return s.costCardRecursive(k, n)
}
func (s *smtPBSort) costCardRecursive(k, n uint32) uint32 {
	l := n / 2
	// Source vc_card_rec calls vc_smerge(k,l,n-l), whose signature is
	// (a,b,c), unlike the executable smerge(c,a,b) call above.
	return s.costCard(k, l) + s.costCard(k, n-l) + s.costSmallMerge(n-l, k, l)
}
func (s *smtPBSort) costMerge(a, b uint32) uint32 {
	if a == 1 && b == 1 {
		return 13
	}
	if a == 0 || b == 0 {
		return 0
	}
	if s.useDirectMerge(a, b, a+b) {
		return s.costDirectMerge(a, b, a+b)
	}
	return s.costMerge(smtPBSortCeil(a), smtPBSortCeil(b)) + s.costMerge(a/2, b/2) + 13*min(smtPBSortCeil(a)+smtPBSortCeil(b)-1, a/2+b/2)
}
func (s *smtPBSort) costMax() uint32 {
	c := uint32(5)
	if s.mode != smtPBSortGE {
		c += 2
	}
	if s.mode != smtPBSortLE {
		c++
	}
	return c
}
func (s *smtPBSort) costSmallMerge(c, a, b uint32) uint32 {
	if a == 1 && b == 1 && c == 1 {
		return s.costMax()
	}
	if a == 0 || b == 0 {
		return 0
	}
	if a > c {
		return s.costSmallMerge(c, c, b)
	}
	if b > c {
		return s.costSmallMerge(c, a, c)
	}
	if a+b <= c {
		return s.costMerge(a, b)
	}
	if s.useDirectMerge(a, b, c) {
		return s.costDirectMerge(a, b, c)
	}
	return s.costSmallMergeRecursive(a, b, c)
}
func (s *smtPBSort) costSmallMergeRecursive(a, b, c uint32) uint32 {
	c1, c2 := (c+1)/2, (c-1)/2
	if c%2 == 0 {
		c1, c2 = 1+c/2, c/2
	}
	return s.costSmallMerge(c1, smtPBSortCeil(a), smtPBSortCeil(b)) + s.costSmallMerge(c2, a/2, b/2) + 13*min(smtPBSortCeil(a)+smtPBSortCeil(b)-1, a/2+b/2) + s.costMax()
}
func (s *smtPBSort) useDirectMerge(a, b, c uint32) bool {
	return a < 1<<15 && b < 1<<15 && s.costDirectMerge(a, b, a+b) < s.costSmallMergeRecursive(a, b, c)
}
