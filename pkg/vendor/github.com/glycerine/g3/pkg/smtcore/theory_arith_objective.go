package smtcore

// Source: src/smt/theory_arith_aux.h add_objective, value, maximize and mk_gt.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
)

// SMTArithmeticObjectiveValue is the concrete result of the arithmetic
// maximizer: an unbounded sentinel, or an exact tableau point. For inf_ext,
// Finite also retains the point's infinity coefficient; it must not be reduced
// to Unbounded. All big.Rat components are owned and cloned before mutation.
type SMTArithmeticObjectiveValue struct {
	Unbounded bool
	Finite    simplex.Infinitesimal
}

// AddObjective internalizes even a rejected nonlinear objective, as source
// does, then returns null_theory_var if it is outside the linear fragment.
func (t *SMTLinearArithmeticTheory) AddObjective(term ast.ASTHandle) (SMTTheoryVar, error) {
	s, err := t.State()
	if err != nil {
		return SMTNullTheoryVar, err
	}
	kind, err := s.manager.GetKind(term)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if kind != ast.ASTApp {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	sort, err := s.manager.GetSort(term)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	family, err := s.manager.GetFamilyID(sort)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	if family != t.FamilyID() {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	if err = s.Internalize(term, false); err != nil {
		return SMTNullTheoryVar, err
	}
	node := s.context.enodes[term]
	if node == nil || !t.IsAttachedToVariable(node) {
		return SMTNullTheoryVar, ast.ErrASTDomain
	}
	u, err := NewSMTTheoryOptimizationUtil(s.manager)
	if err != nil {
		return SMTNullTheoryVar, err
	}
	linear, err := u.IsLinear(term)
	if err != nil || !linear {
		return SMTNullTheoryVar, err
	}
	return node.TheoryVar(t.FamilyID()), nil
}

// ObjectiveValue reads the current tableau assignment without replacing its
// infinitesimal by the finite delta used for model publication.
func (t *SMTLinearArithmeticTheory) ObjectiveValue(variable SMTTheoryVar) (SMTArithmeticObjectiveValue, error) {
	if _, err := t.State(); err != nil {
		return SMTArithmeticObjectiveValue{}, err
	}
	if _, err := t.Enode(variable); err != nil {
		return SMTArithmeticObjectiveValue{}, err
	}
	if t.solver == nil || int(variable) >= len(t.simplexVariables) || t.simplexVariables[variable] >= t.solver.NumVariables() {
		return SMTArithmeticObjectiveValue{}, ast.ErrASTDomain
	}
	return SMTArithmeticObjectiveValue{Finite: t.solver.Value(t.simplexVariables[variable])}, nil
}

// objectiveBlocker returns a borrowed AST, following mk_gt. Integer values use
// the next integer; a real upper supremum r-epsilon is blocked by obj >= r.
func (t *SMTLinearArithmeticTheory) objectiveBlocker(variable SMTTheoryVar, value simplex.Infinitesimal) (ast.ASTHandle, error) {
	node, err := t.Enode(variable)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := t.state.manager.GetSort(node.owner)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := t.state.manager.GetDeclKind(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r := new(big.Rat).Set(&value.Main)
	op := smtArithOpGT
	var bound ast.ASTHandle
	if kind == 1 {
		next := floorSMTRational(r)
		next.Add(next, big.NewInt(1)) // ceil(r) when fractional, otherwise r+1.
		bound, err = t.state.manager.ArithmeticIntegerNumeralText(next.String())
		op = smtArithOpGE
	} else {
		bound, err = t.state.manager.ArithmeticRealNumeralText(r.RatString())
		if value.Epsilon.Sign() < 0 {
			op = smtArithOpGE
		}
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return t.state.manager.MkFamilyApp(t.FamilyID(), op, nil, []ast.ASTHandle{node.owner, bound})
}

// MaximizeObjective moves the current feasible assignment using the native
// tableau max_min algorithm. The caller must revalidate theory combination
// when hasShared is true, as opt_solver::maximize_objective does. The blocker
// is borrowed; a caller retaining it across manager operations must pin it.
func (t *SMTLinearArithmeticTheory) MaximizeObjective(variable SMTTheoryVar) (value SMTArithmeticObjectiveValue, blocker ast.ASTHandle, hasShared bool, err error) {
	value, err = t.ObjectiveValue(variable)
	if err != nil {
		return
	}
	if len(t.nonlinearColumns) != 0 {
		hasShared = true
		blocker, err = t.objectiveBlocker(variable, value.Finite)
		return
	}
	integers := make(map[uint32]bool, len(t.integerColumns)+len(t.atoms)+1)
	shared := make(map[uint32]bool)
	integers[0] = true
	for v, column := range t.simplexVariables {
		integers[column] = t.integer || t.integerColumns[column]
		var node *SMTEnode
		node, err = t.Enode(SMTTheoryVar(v))
		if err != nil {
			return
		}
		shared[column], err = t.sharedArithmeticNode(node)
		if err != nil {
			return
		}
	}
	for i := range t.atoms {
		if t.atoms[i].supported {
			integers[t.nextSimplex+uint32(i)] = t.integralAtom(&t.atoms[i])
		}
	}
	column := t.simplexVariables[variable]
	upper, bounded := t.solver.Upper(column)
	var result simplex.ArithmeticOptResult
	if bounded && value.Finite.Cmp(&upper) == 0 {
		// Source AT_BOUND does not enter the randomized row optimizer.
		result.Status = simplex.ArithmeticAtBound
	} else {
		t.statistics.maxMin++
		result, err = t.solver.ArithmeticMaxMin(column, true, simplex.ArithmeticOptConfig{
			IntegerColumns: integers, SharedColumns: shared,
			MaxEfforts: 10 + t.state.random.Bounded(20), Limit: &t.state.manager.Limit,
		})
		if err != nil {
			return
		}
	}
	if result.Status == simplex.ArithmeticUnbounded {
		value = SMTArithmeticObjectiveValue{Unbounded: true}
		blocker, err = t.state.manager.MkFalse()
		return
	}
	if result.Status == simplex.ArithmeticOptimized {
		bound := t.normalizeOptimizedBound(column, false, result.Value.Clone())
		explanation := smtArithmeticWeightedBoundExplanation(t.solver, result.Bounds)
		t.queueOptimizationBound(t.solver, smtArithmeticReplayBound{column: column, value: bound, explanation: explanation})
	}
	hasShared = result.HasShared
	value, err = t.ObjectiveValue(variable)
	if err == nil {
		blocker, err = t.objectiveBlocker(variable, value.Finite)
	}
	return
}
