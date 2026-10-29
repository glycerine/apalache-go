package smtcore

// Source: src/smt/arith_eq_adapter.h and src/smt/arith_eq_adapter.cpp retained
// arithmetic equality-to-bound adapter.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type smtArithmeticEqualityPair struct {
	first, second ast.ASTHandle
}

// smtArithmeticEqualityAdapter converts equality and disequality events from
// congruence closure into the arithmetic equality atom handled by the owning
// theory. InternalizeEquality then creates the paired <= and >= atoms and the
// triangle clauses. Processed pairs follow theory scopes.
type smtArithmeticEqualityAdapter struct {
	owner     *SMTTheoryBase
	processed map[smtArithmeticEqualityPair]struct{}
	trail     []smtArithmeticEqualityPair
	limits    []int
	restart   []smtArithmeticEqualityPair
	hint      []ast.ASTParameter
	numAxioms uint64
	lazy      bool
	binary    bool
	trace     *diagnostic.TraceTags
}

func newSMTArithmeticEqualityAdapter(owner *SMTTheoryBase) (*smtArithmeticEqualityAdapter, error) {
	if owner == nil {
		return nil, ast.ErrASTHandle
	}
	return &smtArithmeticEqualityAdapter{
		owner:     owner,
		processed: make(map[smtArithmeticEqualityPair]struct{}),
	}, nil
}

func (a *smtArithmeticEqualityAdapter) traceTags() *diagnostic.TraceTags {
	if a != nil && a.trace != nil {
		return a.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (a *smtArithmeticEqualityAdapter) NewEquality(left, right SMTTheoryVar) error {
	return a.newRelation(left, right, false)
}

func (a *smtArithmeticEqualityAdapter) NewDisequality(left, right SMTTheoryVar) error {
	return a.newRelation(left, right, true)
}

func (a *smtArithmeticEqualityAdapter) newRelation(left, right SMTTheoryVar, disequality bool) error {
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
	function, line := "new_eq_eh", 230
	if disequality {
		function, line = "new_diseq_eh", 236
	}
	if err = smtArithEqTracePair(a.traceTags(), a.owner.state.manager, function, line, "arith_eq_adapter", left, right, n1, n2, disequality); err != nil {
		return err
	}
	if !disequality {
		if err = smtArithEqTrace(a.traceTags(), "arith_eq_adapter_bug", function, 231, func() bool { return true }, func(out io.Writer) error {
			for _, node := range []*SMTEnode{n1, n2} {
				if err := smtArithEqDisplayBounded(a.owner.state.manager, out, node.owner, ^uint32(0)); err != nil {
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
	}
	return a.MkAxioms(left, right)
}

func (a *smtArithmeticEqualityAdapter) pair(left, right *SMTEnode) (smtArithmeticEqualityPair, error) {
	first, err := left.OwnerID()
	if err != nil {
		return smtArithmeticEqualityPair{}, err
	}
	second, err := right.OwnerID()
	if err != nil {
		return smtArithmeticEqualityPair{}, err
	}
	if first > second {
		left, right = right, left
	}
	return smtArithmeticEqualityPair{first: left.Owner(), second: right.Owner()}, nil
}

func (a *smtArithmeticEqualityAdapter) MkAxioms(left, right SMTTheoryVar) error {
	if a == nil || a.owner == nil {
		return ast.ErrASTDomain
	}
	leftNode, err := a.owner.Enode(left)
	if err != nil {
		return err
	}
	rightNode, err := a.owner.Enode(right)
	if err != nil {
		return err
	}
	if err = smtArithEqAssertion(84, "n1 != n2", leftNode != rightNode); err != nil {
		return err
	}
	if left == right {
		return ast.ErrASTDomain
	}
	state, err := a.owner.State()
	if err != nil {
		return err
	}
	if err = smtArithEqTrace(a.traceTags(), "arith_eq_adapter_mk_axioms", "mk_axioms", 86, func() bool { return true }, func(out io.Writer) error {
		first, firstErr := leftNode.OwnerID()
		if firstErr != nil {
			return firstErr
		}
		second, secondErr := rightNode.OwnerID()
		if secondErr != nil {
			return secondErr
		}
		if err := smtWriteTheoryText(out, fmt.Sprintf("#%d #%d\n", first, second)); err != nil {
			return err
		}
		for _, expression := range []ast.ASTHandle{leftNode.owner, rightNode.owner} {
			if err := smtArithEqDisplayExpression(state.manager, out, expression); err != nil {
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
	if leftNode == rightNode || leftNode.Owner() == rightNode.Owner() {
		return nil
	}
	leftValue, err := state.manager.IsValue(leftNode.Owner())
	if err != nil {
		return err
	}
	rightValue, err := state.manager.IsValue(rightNode.Owner())
	if err != nil {
		return err
	}
	if leftValue && rightValue {
		return nil
	}
	if err = smtArithEqTrace(a.traceTags(), "arith_eq_adapter_relevancy", "mk_axioms", 102, func() bool {
		return state.relevancy != nil && !(state.relevancy.IsRelevant(leftNode.owner) && state.relevancy.IsRelevant(rightNode.owner))
	}, func(out io.Writer) error {
		leftID, leftErr := leftNode.OwnerID()
		if leftErr != nil {
			return leftErr
		}
		rightID, rightErr := rightNode.OwnerID()
		if rightErr != nil {
			return rightErr
		}
		if err := smtWriteTheoryText(out, fmt.Sprintf("is_relevant(n1): #%d %d\nis_relevant(n2): #%d %d\n", leftID, smtDisplayBool(state.relevancy.IsRelevant(leftNode.owner)), rightID, smtDisplayBool(state.relevancy.IsRelevant(rightNode.owner)))); err != nil {
			return err
		}
		for _, expression := range []ast.ASTHandle{leftNode.owner, rightNode.owner} {
			if err := smtArithEqDisplayExpression(state.manager, out, expression); err != nil {
				return err
			}
			if err := smtWriteTheoryText(out, "\n"); err != nil {
				return err
			}
		}
		return state.DisplayContext(out)
	}); err != nil {
		return err
	}
	pair, err := a.pair(leftNode, rightNode)
	if err != nil {
		return err
	}
	if _, found := a.processed[pair]; found {
		return nil
	}
	firstID, err := state.manager.GetASTID(pair.first)
	if err != nil {
		return err
	}
	secondID, err := state.manager.GetASTID(pair.second)
	if err != nil {
		return err
	}
	if err = smtArithEqTrace(a.traceTags(), "arith_eq_adapter_profile", "mk_axioms", 116, func() bool { return true }, func(out io.Writer) error {
		return smtWriteTheoryText(out, fmt.Sprintf("mk #%d #%d %d %d\n", firstID, secondID, len(a.processed), state.scopeLevel))
	}); err != nil {
		return err
	}
	if err = smtArithEqTrace(a.traceTags(), "arith_eq_adapter_profile_detail", "mk_axioms", 121, func() bool { return true }, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, "mk_detail "); err != nil {
			return err
		}
		if err := smtArithEqDisplayBounded(state.manager, out, pair.first, 5); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, " "); err != nil {
			return err
		}
		if err := smtArithEqDisplayBounded(state.manager, out, pair.second, 5); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return err
	}
	a.processed[pair] = struct{}{}
	a.trail = append(a.trail, pair)
	booleanStart := len(state.boolExpr)
	equality, err := state.manager.MkEq(pair.first, pair.second)
	if err == nil {
		err = smtArithEqAssertion(128, "!m.is_false(t1_eq_t2)", !state.manager.IsFalse(equality))
	}
	if err == nil {
		err = smtArithEqTrace(a.traceTags(), "arith_eq_adapter_bug", "mk_axioms", 130, func() bool { return true }, func(out io.Writer) error {
			for _, expression := range []ast.ASTHandle{equality, pair.first, pair.second} {
				if displayErr := smtArithEqDisplayBounded(state.manager, out, expression, ^uint32(0)); displayErr != nil {
					return displayErr
				}
				if writeErr := smtWriteTheoryText(out, "\n"); writeErr != nil {
					return writeErr
				}
			}
			return nil
		})
	}
	if err == nil {
		err = state.Internalize(equality, true)
	}
	if err != nil {
		delete(a.processed, pair)
		a.trail = a.trail[:len(a.trail)-1]
		return err
	}
	generated := append([]ast.ASTHandle(nil), state.boolExpr[booleanStart:]...)
	var bounds []ast.ASTHandle
	for _, expression := range generated {
		if expression != equality {
			bounds = append(bounds, expression)
		}
	}
	if len(bounds) >= 2 {
		if err = smtArithEqTrace(a.traceTags(), "arith_eq_adapter_perf", "mk_axioms", 174, func() bool { return true }, func(out io.Writer) error {
			for _, expression := range []ast.ASTHandle{equality, bounds[0], bounds[1]} {
				if err := smtArithEqDisplayExpression(state.manager, out, expression); err != nil {
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
		eqLit, litErr := state.Literal(equality)
		if litErr != nil {
			return litErr
		}
		leLit, litErr := state.Literal(bounds[0])
		if litErr != nil {
			return litErr
		}
		geLit, litErr := state.Literal(bounds[1])
		if litErr != nil {
			return litErr
		}
		if err = smtArithEqAssertion(190, "ctx.lit_internalized(le)", leLit != SMTNullLiteral); err != nil {
			return err
		}
		if err = smtArithEqAssertion(191, "ctx.lit_internalized(ge)", geLit != SMTNullLiteral); err != nil {
			return err
		}
		if err = smtArithEqTrace(a.traceTags(), "interface_eq", "mk_axioms", 182, func() bool { return true }, func(out io.Writer) error {
			data, dataErr := state.BooleanData(eqLit.Var())
			if dataErr != nil {
				return dataErr
			}
			if err := smtWriteTheoryText(out, "core should try true phase first for the equality: "); err != nil {
				return err
			}
			if err := smtArithEqDisplayLiteral(out, eqLit); err != nil {
				return err
			}
			return smtWriteTheoryText(out, fmt.Sprintf("\n#%d == #%d\ntry_true_first: %d\n", firstID, secondID, smtDisplayBool(data.TryTrueFirst())))
		}); err != nil {
			return err
		}
		if err = smtArithEqTrace(a.traceTags(), "arith_eq_adapter_bug", "mk_axioms", 186, func() bool { return true }, func(out io.Writer) error {
			for index, expression := range bounds[:2] {
				if index == 0 {
					if err := smtWriteTheoryText(out, "le: "); err != nil {
						return err
					}
				} else if err := smtWriteTheoryText(out, "\nge: "); err != nil {
					return err
				}
				if err := smtArithEqDisplayExpression(state.manager, out, expression); err != nil {
					return err
				}
			}
			return smtWriteTheoryText(out, "\n")
		}); err != nil {
			return err
		}
		if err = smtArithEqTrace(a.traceTags(), "arith_eq_adapter", "mk_axioms", 208, func() bool { return true }, func(out io.Writer) error {
			expressions := []ast.ASTHandle{bounds[0], bounds[1], equality}
			literals := []SMTLiteral{leLit, geLit, eqLit}
			if err := smtWriteTheoryText(out, "internalizing: "); err != nil {
				return err
			}
			for i := range expressions {
				if err := smtWriteTheoryText(out, " "); err != nil {
					return err
				}
				if err := smtArithEqDisplayExpression(state.manager, out, expressions[i]); err != nil {
					return err
				}
				if err := smtWriteTheoryText(out, ": "); err != nil {
					return err
				}
				if err := smtArithEqDisplayLiteral(out, literals[i]); err != nil {
					return err
				}
			}
			return smtWriteTheoryText(out, "\n")
		}); err != nil {
			return err
		}
	}
	a.numAxioms++
	if err = smtArithEqTrace(a.traceTags(), "arith_eq_adapter_profile", "mk_axioms", 179, func() bool { return true }, func(out io.Writer) error {
		return smtWriteTheoryText(out, fmt.Sprintf("insert #%d #%d\n", firstID, secondID))
	}); err != nil {
		return err
	}
	if a.binary {
		if err = smtArithEqTrace(a.traceTags(), "arith_eq_adapter", "mk_axioms", 214, func() bool { return true }, func(out io.Writer) error {
			return smtWriteTheoryText(out, "adding binary bounds...\n")
		}); err != nil {
			return err
		}
	}
	if !a.lazy && state.searching && state.scopeLevel > state.searchBaseLevel &&
		leftNode.InternalScopeLevel() <= state.searchBaseLevel && rightNode.InternalScopeLevel() <= state.searchBaseLevel {
		a.restart = append(a.restart, pair)
	}
	return smtArithEqTraceContext(a.traceTags(), "arith_eq_adapter_detail", "mk_axioms", 226, state)
}

func (a *smtArithmeticEqualityAdapter) AddRelevancy(first, second ast.ASTHandle, targets ...ast.ASTHandle) error {
	if a == nil || a.owner == nil {
		return ast.ErrASTHandle
	}
	state, err := a.owner.State()
	if err != nil {
		return err
	}
	if state.relevancy == nil {
		return nil
	}
	for _, target := range targets {
		if err = state.relevancy.AddPairDependency(first, second, target); err != nil {
			return err
		}
	}
	return nil
}

func (a *smtArithmeticEqualityAdapter) InitializeSearch() error {
	if a == nil || a.owner == nil {
		return ast.ErrASTHandle
	}
	a.restart = nil
	return nil
}

func (a *smtArithmeticEqualityAdapter) Restart() error {
	if a == nil || a.owner == nil {
		return ast.ErrASTHandle
	}
	state, err := a.owner.State()
	if err != nil {
		return err
	}
	pairs := append([]smtArithmeticEqualityPair(nil), a.restart...)
	a.restart = nil
	for _, pair := range pairs {
		if state.inconsistent {
			break
		}
		left := state.context.enodes[pair.first]
		right := state.context.enodes[pair.second]
		if left == nil || right == nil {
			return ast.ErrASTDomain
		}
		leftVar := left.TheoryVar(a.owner.FamilyID())
		rightVar := right.TheoryVar(a.owner.FamilyID())
		if leftVar == SMTNullTheoryVar || rightVar == SMTNullTheoryVar {
			return ast.ErrASTDomain
		}
		if err = a.MkAxioms(leftVar, rightVar); err != nil {
			return err
		}
	}
	return nil
}

func (a *smtArithmeticEqualityAdapter) PushScope() error {
	if a == nil || a.owner == nil {
		return ast.ErrASTHandle
	}
	a.limits = append(a.limits, len(a.trail))
	return nil
}

func (a *smtArithmeticEqualityAdapter) PopScopes(count uint32) error {
	if a == nil || count == 0 || int(count) > len(a.limits) {
		return ast.ErrASTDomain
	}
	level := len(a.limits) - int(count)
	limit := a.limits[level]
	for index := len(a.trail); index > limit; index-- {
		delete(a.processed, a.trail[index-1])
	}
	a.trail = a.trail[:limit]
	a.limits = a.limits[:level]
	return nil
}

func (a *smtArithmeticEqualityAdapter) NumAxioms() uint64 {
	if a == nil {
		return 0
	}
	return a.numAxioms
}

func (a *smtArithmeticEqualityAdapter) ResetStatistics() {
	if a != nil {
		a.numAxioms = 0
	}
}

// Source arith_eq_adapter.cpp:269-279. Its native hash-table iteration order
// is unspecified; only each canonical pair's literal record is prescribed.
// Context enodes retain the borrowed AST handles through the pair's scope.
func (a *smtArithmeticEqualityAdapter) DisplayAlreadyProcessed(out io.Writer) error {
	if a == nil || a.owner == nil || out == nil {
		return ast.ErrASTHandle
	}
	state, err := a.owner.State()
	if err != nil {
		return err
	}
	for pair := range a.processed {
		first, err := state.manager.GetASTID(pair.first)
		if err != nil {
			return err
		}
		second, err := state.manager.GetASTID(pair.second)
		if err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, fmt.Sprintf("eq_adapter: #%d #%d\n", first, second)); err != nil {
			return err
		}
	}
	return nil
}

// Source theory_arith_core.h:1475, before propagate_core in final_check_eh.
func (a *smtArithmeticEqualityAdapter) traceFinalCheckPairs() error {
	if a == nil {
		return ast.ErrASTHandle
	}
	tags := a.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	var operandErr error
	err := tags.EmitConditional("arith_eq_adapter_info", "final_check_eh", "src/smt/theory_arith_core.h", 1475, func() bool { return true }, func(out io.Writer) error {
		operandErr = a.DisplayAlreadyProcessed(assertedTraceWriter{out})
		return operandErr
	})
	if operandErr != nil {
		return operandErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
