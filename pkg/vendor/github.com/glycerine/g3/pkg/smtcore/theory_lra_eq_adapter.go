package smtcore

// Source: src/smt/arith_eq_adapter.cpp mk_axioms and scoped/restart lifetime,
// concretely consumed by src/smt/theory_lra.cpp.

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type smtLRAEqualityAdapter struct {
	owner        *SMTLRATheory
	manager      *ast.ASTManager
	processed    map[smtArithmeticEqualityPair]struct{}
	trail        []smtArithmeticEqualityPair
	limits       []int
	restart      []smtArithmeticEqualityPair
	hint         []ast.ASTParameter
	pendingRoots []ast.ASTRef
	numAxioms    uint64
}

func newSMTLRAEqualityAdapter(owner *SMTLRATheory) *smtLRAEqualityAdapter {
	return &smtLRAEqualityAdapter{owner: owner, processed: make(map[smtArithmeticEqualityPair]struct{})}
}

func (a *smtLRAEqualityAdapter) NewEquality(left, right SMTTheoryVar) error {
	if a == nil || a.owner == nil {
		return ast.ErrASTHandle
	}
	n1, err := a.owner.Enode(left)
	if err != nil {
		return err
	}
	n2, err := a.owner.Enode(right)
	if err != nil {
		return err
	}
	if err = smtArithEqTracePair(a.owner.lraTraceTags(), a.owner.state.manager, "new_eq_eh", 230, "arith_eq_adapter", left, right, n1, n2, false); err != nil {
		return err
	}
	if err = smtArithEqTrace(a.owner.lraTraceTags(), "arith_eq_adapter_bug", "new_eq_eh", 231, func() bool { return true }, func(out io.Writer) error {
		if err := smtArithEqDisplayBounded(a.owner.state.manager, out, n1.owner, 4294967295); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, "\n"); err != nil {
			return err
		}
		if err := smtArithEqDisplayBounded(a.owner.state.manager, out, n2.owner, 4294967295); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return err
	}
	return a.MkAxioms(left, right)
}

func (a *smtLRAEqualityAdapter) NewDisequality(left, right SMTTheoryVar) error {
	if a == nil || a.owner == nil {
		return ast.ErrASTHandle
	}
	n1, err := a.owner.Enode(left)
	if err != nil {
		return err
	}
	n2, err := a.owner.Enode(right)
	if err != nil {
		return err
	}
	if err = smtArithEqTracePair(a.owner.lraTraceTags(), a.owner.state.manager, "new_diseq_eh", 236, "arith_eq_adapter", left, right, n1, n2, true); err != nil {
		return err
	}
	return a.MkAxioms(left, right)
}

func (a *smtLRAEqualityAdapter) MkAxioms(left, right SMTTheoryVar) (result error) {
	if a == nil || a.owner == nil || len(a.pendingRoots) != 0 {
		return ast.ErrASTHandle
	}
	t := a.owner
	s, err := t.State()
	if err != nil {
		return err
	}
	m := s.manager
	a.manager = m
	n1, err := t.Enode(left)
	if err != nil {
		return err
	}
	n2, err := t.Enode(right)
	if err != nil {
		return err
	}
	if err = smtArithEqAssertion(84, "n1 != n2", n1 != n2); err != nil {
		return err
	}
	if err = smtArithEqTrace(t.lraTraceTags(), "arith_eq_adapter_mk_axioms", "mk_axioms", 86, func() bool { return true }, func(out io.Writer) error {
		id1, err := n1.OwnerID()
		if err != nil {
			return err
		}
		id2, err := n2.OwnerID()
		if err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, fmt.Sprintf("#%d #%d\n", id1, id2)); err != nil {
			return err
		}
		if err = smtArithEqDisplayExpression(m, out, n1.owner); err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, "\n"); err != nil {
			return err
		}
		if err = smtArithEqDisplayExpression(m, out, n2.owner); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return err
	}
	if n1 == n2 || n1.owner == n2.owner {
		return nil
	}
	id1, err := n1.OwnerID()
	if err != nil {
		return err
	}
	id2, err := n2.OwnerID()
	if err != nil {
		return err
	}
	if id1 > id2 {
		n1, n2 = n2, n1
	}
	first, err := m.IsValue(n1.owner)
	if err != nil {
		return err
	}
	second, err := m.IsValue(n2.owner)
	if err != nil {
		return err
	}
	if first && second {
		return nil
	}
	if err = smtArithEqTrace(t.lraTraceTags(), "arith_eq_adapter_relevancy", "mk_axioms", 102, func() bool {
		return s.relevancy != nil && !(s.relevancy.IsRelevant(n1.owner) && s.relevancy.IsRelevant(n2.owner))
	}, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, fmt.Sprintf("is_relevant(n1): #%d %d\nis_relevant(n2): #%d %d\n", id1, smtDisplayBool(s.relevancy.IsRelevant(n1.owner)), id2, smtDisplayBool(s.relevancy.IsRelevant(n2.owner)))); err != nil {
			return err
		}
		if err := smtArithEqDisplayExpression(m, out, n1.owner); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, "\n"); err != nil {
			return err
		}
		if err := smtArithEqDisplayExpression(m, out, n2.owner); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, "\n"); err != nil {
			return err
		}
		return s.DisplayContext(out)
	}); err != nil {
		return err
	}
	pair := smtArithmeticEqualityPair{n1.owner, n2.owner}
	if _, found := a.processed[pair]; found {
		return nil
	}
	if err = smtArithEqTrace(t.lraTraceTags(), "arith_eq_adapter_profile", "mk_axioms", 116, func() bool { return true }, func(out io.Writer) error {
		return smtWriteTheoryText(out, fmt.Sprintf("mk #%d #%d %d %d\n", id1, id2, len(a.processed), s.scopeLevel))
	}); err != nil {
		return err
	}
	if err = smtArithEqTrace(t.lraTraceTags(), "arith_eq_adapter_profile_detail", "mk_axioms", 121, func() bool { return true }, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, "mk_detail "); err != nil {
			return err
		}
		if err := smtArithEqDisplayBounded(m, out, n1.owner, 5); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, " "); err != nil {
			return err
		}
		if err := smtArithEqDisplayBounded(m, out, n2.owner, 5); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return err
	}
	var roots []ast.ASTRef
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			if err := m.DelRef(&roots[i]); err != nil {
				a.pendingRoots = append(a.pendingRoots, roots[:i+1]...)
				result = errors.Join(result, err)
				return
			}
		}
	}()
	pin := func(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		r, err := m.NewRef(h)
		if err == nil {
			roots = append(roots, r)
		}
		return h, err
	}
	equality, err := pin(s.mkEqualityAtom(n1.owner, n2.owner))
	if err != nil {
		return err
	}
	if err = smtArithEqAssertion(128, "!m.is_false(t1_eq_t2)", !m.IsFalse(equality)); err != nil {
		return err
	}
	if err = smtArithEqTrace(t.lraTraceTags(), "arith_eq_adapter_bug", "mk_axioms", 130, func() bool { return true }, func(out io.Writer) error {
		for _, expression := range []ast.ASTHandle{equality, n1.owner, n2.owner} {
			if err := smtArithEqDisplayBounded(m, out, expression, 4294967295); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, "\n"); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	lhs, rhs := n1.owner, n2.owner
	if _, number, err := t.numeral(lhs); err != nil {
		return err
	} else if number {
		lhs, rhs = rhs, lhs
	}
	_, number, err := t.numeral(rhs)
	if err != nil {
		return err
	}
	if !number {
		sort, err := m.GetSort(lhs)
		if err != nil {
			return err
		}
		kind, err := m.GetDeclKind(sort)
		if err != nil {
			return err
		}
		minusOne, err := pin(m.ArithmeticNumeralRat(big.NewRat(-1, 1), kind == 1))
		if err != nil {
			return err
		}
		zero, err := pin(m.ArithmeticNumeralRat(new(big.Rat), kind == 1))
		if err != nil {
			return err
		}
		negative, err := pin(m.MkFamilyApp(t.FamilyID(), smtArithOpMul, nil, []ast.ASTHandle{minusOne, rhs}))
		if err != nil {
			return err
		}
		lhs, err = pin(m.MkFamilyApp(t.FamilyID(), smtArithOpAdd, nil, []ast.ASTHandle{lhs, negative}))
		if err != nil {
			return err
		}
		rhs = zero
	}
	le, err := pin(m.MkFamilyApp(t.FamilyID(), smtArithOpLE, nil, []ast.ASTHandle{lhs, rhs}))
	if err != nil {
		return err
	}
	ge, err := pin(m.MkFamilyApp(t.FamilyID(), smtArithOpGE, nil, []ast.ASTHandle{lhs, rhs}))
	if err != nil {
		return err
	}
	if err = smtArithEqTrace(t.lraTraceTags(), "arith_eq_adapter_perf", "mk_axioms", 174, func() bool { return true }, func(out io.Writer) error {
		for _, expression := range []ast.ASTHandle{equality, le, ge} {
			if err := smtArithEqDisplayExpression(m, out, expression); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, "\n"); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	// Insert before recursive equality internalization, as the source does.
	position := len(a.trail)
	a.trail = append(a.trail, pair)
	a.processed[pair] = struct{}{}
	a.numAxioms++
	if err = smtArithEqTrace(t.lraTraceTags(), "arith_eq_adapter_profile", "mk_axioms", 179, func() bool { return true }, func(out io.Writer) error {
		return smtWriteTheoryText(out, fmt.Sprintf("insert #%d #%d\n", id1, id2))
	}); err != nil {
		return err
	}
	defer func() {
		if result != nil {
			delete(a.processed, pair)
			copy(a.trail[position:], a.trail[position+1:])
			a.trail = a.trail[:len(a.trail)-1]
		}
	}()
	if err = s.Internalize(equality, true); err != nil {
		return err
	}
	if err = s.Internalize(le, true); err != nil {
		return err
	}
	if err = s.Internalize(ge, true); err != nil {
		return err
	}
	eqLit, err := s.Literal(equality)
	if err != nil {
		return err
	}
	if err = smtArithEqTrace(t.lraTraceTags(), "interface_eq", "mk_axioms", 182, func() bool { return true }, func(out io.Writer) error {
		data, dataErr := s.BooleanData(eqLit.Var())
		if dataErr != nil {
			return dataErr
		}
		if err := smtWriteTheoryText(out, "core should try true phase first for the equality: "); err != nil {
			return err
		}
		if err := smtArithEqDisplayLiteral(out, eqLit); err != nil {
			return err
		}
		return smtWriteTheoryText(out, fmt.Sprintf("\n#%d == #%d\ntry_true_first: %d\n", id1, id2, smtDisplayBool(data.TryTrueFirst())))
	}); err != nil {
		return err
	}
	if err = smtArithEqTrace(t.lraTraceTags(), "arith_eq_adapter_bug", "mk_axioms", 186, func() bool { return true }, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, "le: "); err != nil {
			return err
		}
		if err := smtArithEqDisplayExpression(m, out, le); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, "\nge: "); err != nil {
			return err
		}
		if err := smtArithEqDisplayExpression(m, out, ge); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return err
	}
	leLit, err := s.Literal(le)
	if err != nil {
		return err
	}
	if err = smtArithEqAssertion(190, "ctx.lit_internalized(le)", leLit != SMTNullLiteral); err != nil {
		return err
	}
	geLit, err := s.Literal(ge)
	if err != nil {
		return err
	}
	if err = smtArithEqAssertion(191, "ctx.lit_internalized(ge)", geLit != SMTNullLiteral); err != nil {
		return err
	}
	data, err := s.BooleanData(eqLit.Var())
	if err != nil {
		return err
	}
	if data.TryTrueFirst() {
		if _, err = s.setScopedTrueFirst(leLit.Var()); err != nil {
			return err
		}
		if _, err = s.setScopedTrueFirst(geLit.Var()); err != nil {
			return err
		}
	}
	proofs, err := m.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs && len(a.hint) == 0 {
		symbol, err := m.Symbols.Intern("triangle-eq")
		if err != nil {
			return err
		}
		p, err := m.NewParameterSymbol(symbol)
		if err != nil {
			return err
		}
		a.hint = append(a.hint, p)
	}
	clauses := [][]SMTLiteral{{eqLit.Negated(), leLit}, {eqLit.Negated(), geLit}, {eqLit, leLit.Negated(), geLit.Negated()}}
	for _, clause := range clauses {
		if err = s.addTheoryAxiomWithParameters(t.FamilyID(), clause, a.hint); err != nil {
			return err
		}
	}
	if err = smtArithEqTrace(t.lraTraceTags(), "arith_eq_adapter", "mk_axioms", 208, func() bool { return true }, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, "internalizing:  "); err != nil {
			return err
		}
		if err := smtArithEqDisplayExpression(m, out, le); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, ": "); err != nil {
			return err
		}
		if err := smtArithEqDisplayLiteral(out, leLit); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, " "); err != nil {
			return err
		}
		if err := smtArithEqDisplayExpression(m, out, ge); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, ": "); err != nil {
			return err
		}
		if err := smtArithEqDisplayLiteral(out, geLit); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, " "); err != nil {
			return err
		}
		if err := smtArithEqDisplayExpression(m, out, equality); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, ": "); err != nil {
			return err
		}
		if err := smtArithEqDisplayLiteral(out, eqLit); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return err
	}
	if s.relevancy != nil {
		for _, target := range []ast.ASTHandle{equality, le, ge} {
			if err = s.relevancy.AddPairDependency(n1.owner, n2.owner, target); err != nil {
				return err
			}
		}
	}
	// lazy_adapter stays false on retained LRA parameter paths. The sole
	// add_binary_bounds writer selects RDL, so that branch is not instantiated.
	if s.scopeLevel > s.searchBaseLevel && n1.InternalScopeLevel() <= s.searchBaseLevel && n2.InternalScopeLevel() <= s.searchBaseLevel {
		a.restart = append(a.restart, pair)
	}
	return smtArithEqTraceContext(t.lraTraceTags(), "arith_eq_adapter_detail", "mk_axioms", 226, s)
}

func (a *smtLRAEqualityAdapter) InitializeSearch() error { a.restart = nil; return nil }
func (a *smtLRAEqualityAdapter) Restart() error {
	s, err := a.owner.State()
	if err != nil {
		return err
	}
	pairs := append([]smtArithmeticEqualityPair(nil), a.restart...)
	a.restart = nil
	for _, pair := range pairs {
		if s.inconsistent {
			break
		}
		left, right := s.context.enodes[pair.first], s.context.enodes[pair.second]
		if left == nil || right == nil {
			return ast.ErrASTDomain
		}
		if err = a.MkAxioms(left.TheoryVar(a.owner.FamilyID()), right.TheoryVar(a.owner.FamilyID())); err != nil {
			return err
		}
	}
	return nil
}
func (a *smtLRAEqualityAdapter) PushScope() error {
	a.limits = append(a.limits, len(a.trail))
	return nil
}
func (a *smtLRAEqualityAdapter) PopScopes(n uint32) error {
	if n == 0 || int(n) > len(a.limits) {
		return ast.ErrASTDomain
	}
	level := len(a.limits) - int(n)
	limit := a.limits[level]
	for _, pair := range a.trail[limit:] {
		delete(a.processed, pair)
	}
	a.trail = a.trail[:limit]
	a.limits = a.limits[:level]
	return nil
}
func (a *smtLRAEqualityAdapter) NumAxioms() uint64 { return a.numAxioms }
func (a *smtLRAEqualityAdapter) Close() error {
	for len(a.pendingRoots) > 0 {
		i := len(a.pendingRoots) - 1
		if err := a.manager.DelRef(&a.pendingRoots[i]); err != nil {
			return err
		}
		a.pendingRoots = a.pendingRoots[:i]
	}
	if len(a.hint) != 0 {
		if err := smtArithmeticReleaseParameters(a.manager, a.hint); err != nil {
			return err
		}
		a.hint = nil
	}
	a.processed = nil
	a.trail = nil
	a.limits = nil
	a.restart = nil
	return nil
}
