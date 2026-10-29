package smtcore

// Source: src/smt/theory_dense_diff_logic_def.h:841-1130 optimization.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source explicitly shares objective syntax with sparse difference logic.
// Only the concrete term/compiler/core storage is shared; dense maximization
// builds a fresh tableau over the dense edge list on every call.
func (t *SMTDenseDiffTheory) AddObjective(term ast.ASTHandle) (SMTTheoryVar, error) {
	return t.syntax.addDiffObjective(term, t)
}
func (t *SMTDenseDiffTheory) Close() error {
	if t == nil {
		return nil
	}
	return t.syntax.closeDiffOptimization()
}
func (t *SMTDenseDiffTheory) ObjectiveValue(v SMTTheoryVar) (SMTArithmeticObjectiveValue, error) {
	objective, err := t.syntax.diffObjective(v)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, err
	}
	value := simplex.RationalInfinitesimal(&objective.constant)
	for _, term := range objective.terms {
		// This source revision indexes the assignment by objective index v.
		if v < 0 || int(v) >= len(t.values) {
			return SMTArithmeticObjectiveValue{}, ast.ErrASTDomain
		}
		point := &t.values[v]
		value.Main.Add(&value.Main, new(big.Rat).Mul(&term.coefficient, &point.rational))
		value.Epsilon.Add(&value.Epsilon, new(big.Rat).Mul(&term.coefficient, &point.infinitesimal))
	}
	return SMTArithmeticObjectiveValue{Finite: value}, nil
}
func (t *SMTDenseDiffTheory) MakeObjectiveLowerBound(_ *tactic.FilterModelConverter, v SMTTheoryVar, value simplex.Infinitesimal) (ast.ASTHandle, error) {
	if value.Infinity.Sign() != 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return t.syntax.diffObjectiveInequality(v, value, false)
}

func (t *SMTDenseDiffTheory) MaximizeObjective(v SMTTheoryVar) (SMTArithmeticObjectiveValue, ast.ASTHandle, bool, error) {
	objective, err := t.syntax.diffObjective(v)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	state, err := t.State()
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	nodes, edges := t.NumVariables(), len(t.matrix.edges)
	if len(t.values) != nodes {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	solver := simplex.NewSolver(&state.manager.Limit)
	solver.EnsureVariable(uint32(nodes + edges + len(t.syntax.optimization.objectives)))
	for i := range t.values {
		point := &t.values[i]
		solver.SetValue(uint32(i), simplex.NewInfinitesimal(&point.rational, &point.infinitesimal))
	}
	for _, node := range t.variables {
		value, numeral, err := t.syntax.numeral(node.owner)
		if err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
		if numeral && value.Sign() == 0 {
			// Source sets bounds on v (objective index), not the zero node index.
			zero := simplex.Infinitesimal{}
			if err = solver.SetLower(uint32(v), zero); err != nil {
				return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
			}
			if err = solver.SetUpper(uint32(v), zero); err != nil {
				return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
			}
			break
		}
	}
	for i := range t.matrix.edges {
		edge := &t.matrix.edges[i]
		if edge.source < 0 || edge.target < 0 {
			continue
		} // Null edge zero is retained.
		base := uint32(nodes + i)
		if _, err = solver.AddIntegerRow(base, []uint32{uint32(edge.target), uint32(edge.source), base}, []big.Int{*big.NewInt(1), *big.NewInt(-1), *big.NewInt(-1)}); err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
		if err = solver.SetUpper(base, simplex.NewInfinitesimal(&edge.offset.rational, &edge.offset.infinitesimal)); err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
	}
	base := uint32(nodes+edges) + uint32(v)
	var variables []uint32
	var coefficients []big.Rat
	for _, term := range objective.terms {
		variables = append(variables, uint32(term.variable))
		coefficients = append(coefficients, *new(big.Rat).Set(&term.coefficient))
	}
	variables = append(variables, base)
	coefficients = append(coefficients, *big.NewRat(1, 1))
	row, err := solver.AddRationalRow(base, variables, coefficients)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	status, err := solver.MakeFeasible()
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	if status == lbool.False {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	if status != lbool.Undef {
		status, err = solver.Minimize(base)
		if err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
	}
	if status != lbool.True {
		blocker, err := state.manager.MkFalse()
		return SMTArithmeticObjectiveValue{Unbounded: true}, blocker, false, err
	}
	value := solver.Value(base)
	value.Neg()
	for len(objective.core) > 0 {
		last := len(objective.core) - 1
		if err = state.manager.DecRef(objective.core[last]); err != nil {
			return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
		}
		objective.core = objective.core[:last]
	}
	entries, err := solver.RowEntries(row)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	var literals []SMTLiteral
	for _, entry := range entries {
		if int(entry.Variable) >= nodes && int(entry.Variable) < nodes+edges {
			literal := t.matrix.edges[int(entry.Variable)-nodes].literal
			if literal != SMTNullLiteral {
				literals = append(literals, literal)
			}
		}
	}
	if err = t.syntax.saveObjectiveCore(v, literals); err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	for i := 0; i < nodes; i++ {
		point := solver.Value(uint32(i))
		// Preserve the source's explicit TBD: dense maximization publishes the
		// rational part only, unlike the sparse backend's finite-delta assignment.
		t.values[i].rational.Set(&point.Main)
		t.values[i].infinitesimal.SetInt64(0)
	}
	blocker, err := t.syntax.diffObjectiveInequality(v, value, true)
	value.Main.Add(&value.Main, &objective.constant)
	return SMTArithmeticObjectiveValue{Finite: value}, blocker, false, err
}
