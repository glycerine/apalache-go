package smtcore

// Source: src/smt/theory_arith_int.h branch_infeasible_int_var,
// constrain_free_vars, mk_polynomial_ge and mk_gomory_cut.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTLinearArithmeticTheory) integerColumnExpression(refs *smtArithmeticDivisionCall, solver *simplex.Solver, column uint32) (ast.ASTHandle, error) {
	// Keep a comparison column's defining polynomial even when it is fixed.
	// Replacing it with its current endpoint would require the other endpoint
	// as an additional equality premise, absent from the source cut reason.
	return t.integerColumnDefinition(refs, solver, column)
}

// Replay comparison columns represent coefficient-only polynomials. The
// distinguished constant column has no defining atom and remains a numeral.
func (t *SMTLinearArithmeticTheory) integerColumnDefinition(refs *smtArithmeticDivisionCall, solver *simplex.Solver, column uint32) (ast.ASTHandle, error) {
	m := t.state.manager
	if variable, found := t.theoryVariable(column); found {
		node, err := t.Enode(variable)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return node.owner, nil
	}
	if column < t.nextSimplex || int(column-t.nextSimplex) >= len(t.atoms) {
		if value, fixed := smtFixedSimplexColumn(solver, column); fixed {
			return refs.hold(m.ArithmeticNumeralRat(value, value.IsInt()))
		}
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	form := &t.atoms[column-t.nextSimplex].form
	columns := make([]uint32, 0, len(form.terms))
	for column := range form.terms {
		columns = append(columns, column)
	}
	sort.Slice(columns, func(i, j int) bool { return columns[i] < columns[j] })
	var terms []ast.ASTHandle
	for _, column := range columns {
		term, err := t.integerColumnDefinition(refs, solver, column)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		coefficient := form.terms[column]
		if coefficient.Cmp(big.NewRat(1, 1)) != 0 {
			sort, err := m.GetSort(term)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			kind, err := m.GetDeclKind(sort)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			numeral, err := refs.hold(m.ArithmeticNumeralRat(coefficient, kind == 1 && coefficient.IsInt()))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			term, err = refs.app(smtArithOpMul, numeral, term)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		terms = append(terms, term)
	}
	if len(terms) == 0 {
		return refs.hold(m.ArithmeticNumeralRat(new(big.Rat), true))
	}
	if len(terms) == 1 {
		return terms[0], nil
	}
	return refs.app(smtArithOpAdd, terms...)
}

func (t *SMTLinearArithmeticTheory) integerColumnBound(refs *smtArithmeticDivisionCall, solver *simplex.Solver, column uint32, lower bool, value *big.Rat) (ast.ASTHandle, error) {
	expression, err := t.integerColumnExpression(refs, solver, column)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := t.state.manager.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := t.state.manager.GetDeclKind(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	numeral, err := refs.hold(t.state.manager.ArithmeticNumeralRat(value, kind == 1))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op := smtArithOpLE
	if lower {
		op = smtArithOpGE
	}
	return refs.app(op, expression, numeral)
}

func (t *SMTLinearArithmeticTheory) exposeIntegerBound(bound ast.ASTHandle) error {
	if err := t.state.Internalize(bound, true); err != nil {
		return err
	}
	return t.state.markRelevant(bound)
}

func (t *SMTLinearArithmeticTheory) branchIntegerColumn(column uint32) (result error) {
	refs := smtArithmeticDivisionCall{theory: t}
	defer func() { result = errors.Join(result, refs.close()) }()
	value := t.solver.Value(column)
	ceil := floorSMTRational(&value.Main)
	if !value.Main.IsInt() || value.Epsilon.Sign() > 0 {
		ceil.Add(ceil, big.NewInt(1))
	}
	bound, err := t.integerColumnBound(&refs, t.solver, column, true, new(big.Rat).SetInt(ceil))
	if err != nil {
		return err
	}
	t.statistics.branches++
	return t.exposeIntegerBound(bound)
}

func (t *SMTLinearArithmeticTheory) gomoryIntegerColumn(column uint32) (result error) {
	refs := smtArithmeticDivisionCall{theory: t}
	defer func() { result = errors.Join(result, refs.close()) }()
	row, basic := t.solver.BasicRow(column)
	if !basic {
		return ast.ErrASTDomain
	}
	entries, err := t.solver.RowEntries(row)
	if err != nil {
		return err
	}
	var freeBounds []ast.ASTHandle
	for _, entry := range entries {
		if entry.Variable == column {
			continue
		}
		_, lower := t.solver.Lower(entry.Variable)
		_, upper := t.solver.Upper(entry.Variable)
		if !lower && !upper {
			bound, err := t.integerColumnBound(&refs, t.solver, entry.Variable, true, new(big.Rat))
			if err != nil {
				return err
			}
			freeBounds = append(freeBounds, bound)
		}
	}
	// Construct every expression before internalization can allocate theory
	// columns and change the replay-only comparison-column index range.
	if len(freeBounds) != 0 {
		for _, bound := range freeBounds {
			if err = t.exposeIntegerBound(bound); err != nil {
				return err
			}
		}
		return nil
	}
	cut, ready, err := t.solver.ArithmeticGomory(row, t.arithmeticIntegerMask())
	if err != nil || !ready {
		return err
	}
	t.statistics.gomoryCuts++
	m := t.state.manager
	proofs, err := m.ProofsEnabled()
	if err != nil {
		return err
	}
	antecedents, explained, err := smtArithmeticExplainBounds(t.solver, cut.Bounds, proofs, t.equalityBounds)
	if err != nil {
		return err
	}
	if !explained {
		antecedents, err = t.allArithmeticAntecedents()
		if err != nil {
			return err
		}
	}
	parameters, err := antecedents.parameters(m, "gomory-cut")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(m, parameters)) }()
	if cut.Conflict {
		t.recordArithmeticConflict()
		return t.setArithmeticConflict(&antecedents, parameters...)
	}
	var bound ast.ASTHandle
	if len(cut.Terms) == 1 {
		bound, err = t.integerColumnBound(&refs, t.solver, cut.Terms[0].Column, cut.Lower, &cut.Bound)
	} else {
		bound, err = t.integerPolynomialGE(&refs, cut.Terms, &cut.Bound)
	}
	if err != nil {
		return err
	}
	if err = t.exposeIntegerBound(bound); err != nil {
		return err
	}
	literal, err := t.state.Literal(bound)
	if err != nil {
		return err
	}
	if assignment, e := t.state.Assignment(literal); e != nil || assignment == lbool.True {
		return e
	}
	owner, err := NewSMTExtTheoryPropagationJustification(m, t.FamilyID(), antecedents.literals, antecedents.equalities, literal, parameters)
	if err != nil {
		return err
	}
	owner.returnToOwner = true
	justification, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return errors.Join(err, owner.Release(m))
	}
	if err = t.state.AssignLiteral(literal, justification); err != nil {
		return errors.Join(err, owner.Release(m))
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	return nil
}

func (t *SMTLinearArithmeticTheory) branchOrCutIntegerColumn(column uint32) (SMTFinalCheckStatus, error) {
	t.branchCutCounter++
	// Strategy advancement is progress even when a cut is postponed because
	// a row still has free or infinitesimal-valued nonbasic columns.
	t.state.theoryProgress++
	if t.branchCutRatio == 0 {
		return SMTFinalCheckGiveUp, nil
	}
	if t.branchCutCounter%t.branchCutRatio != 0 {
		if t.integerEqualityBranching {
			if progress, err := t.branchIntegerEquality(); err != nil || progress {
				return SMTFinalCheckContinue, err
			}
		}
		// Source reselects after the optional equality split attempt.
		if selected, found := t.solver.ArithmeticIntegerBase(t.arithmeticIntegerMask(), &t.nonlinearRandom); found {
			column = selected
		}
		return SMTFinalCheckContinue, t.branchIntegerColumn(column)
	}
	integers := t.arithmeticIntegerMask()
	t.solver.MoveArithmeticNonBasesToBounds(integers)
	status, err := t.solver.MakeFeasible()
	if err != nil || status == lbool.Undef {
		return SMTFinalCheckGiveUp, err
	}
	if status == lbool.False {
		return SMTFinalCheckContinue, t.derivedBoundConflict(t.solver)
	}
	column, found := t.solver.ArithmeticIntegerBase(integers, &t.nonlinearRandom)
	if !found {
		return SMTFinalCheckDone, nil
	}
	return SMTFinalCheckContinue, t.gomoryIntegerColumn(column)
}
