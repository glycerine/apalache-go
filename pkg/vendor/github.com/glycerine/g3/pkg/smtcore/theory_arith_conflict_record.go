package smtcore

// Source: src/smt/theory_arith_aux.h enable_record_conflict,
// record_conflict, and conflict_minimize.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
)

// EnableRecordConflict watches one arithmetic bound while the optimizer checks
// it as an assumption. A conflict that depends on that bound can then produce
// a tighter objective upper bound from the weighted remaining premises.
func (t *SMTLinearArithmeticTheory) EnableRecordConflict(bound ast.ASTHandle) error {
	if t == nil {
		return ast.ErrASTHandle
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	t.recordConflict = false
	t.recordConflictBound = SMTNullLiteral
	t.conflictUpper = simplex.Infinitesimal{}
	t.conflictUpper.Infinity.SetInt64(-1)
	t.conflictUpperFinite = false
	t.boundPropagation = 0 // source enable_record_conflict sets BP_NONE.
	if bound == (ast.ASTHandle{}) {
		return nil
	}
	if !t.infinite {
		return ast.ErrASTDomain
	}
	if err = state.Internalize(bound, false); err != nil {
		return err
	}
	literal, err := state.Literal(bound)
	if err != nil {
		return err
	}
	t.recordConflict = true
	t.recordConflictBound = literal
	return nil
}

// ConflictMinimize returns the best finite bound derived for the currently
// watched objective, if its assumption participated in the latest conflict.
func (t *SMTLinearArithmeticTheory) ConflictMinimize() (simplex.Infinitesimal, bool) {
	if t == nil || !t.conflictUpperFinite {
		return simplex.Infinitesimal{}, false
	}
	return t.conflictUpper.Clone(), true
}

func (t *SMTLinearArithmeticTheory) recordWatchedConflict(antecedents *smtArithmeticAntecedents) (resultErr error) {
	if t == nil || !t.recordConflict || antecedents == nil {
		return nil
	}
	watch := -1
	for i, literal := range antecedents.literals {
		if literal.Var() == t.recordConflictBound.Var() {
			watch = i
			break
		}
	}
	if watch < 0 || len(antecedents.literalCoefficients) != len(antecedents.literals) || len(antecedents.equalityCoefficients) != len(antecedents.equalities) {
		return nil
	}
	state := t.state
	if state.smtParams == nil {
		return ast.ErrASTDomain
	}
	farkas, err := NewSMTFarkasUtil(state.manager, state.smtParams.Context, state.smtParams.Globals)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, farkas.Close()) }()
	watchCoefficient := new(big.Rat).Abs(antecedents.literalCoefficients[watch])
	added := false
	for i, literal := range antecedents.literals {
		if i == watch {
			continue
		}
		expression, cause := state.literalExpression(literal)
		if cause != nil {
			return cause
		}
		coefficient := new(big.Rat).Abs(antecedents.literalCoefficients[i])
		if err = farkas.Add(coefficient, expression); err != nil {
			return err
		}
		added = true
	}
	for i, pair := range antecedents.equalities {
		expression, cause := state.manager.MkEq(pair.First.Owner(), pair.Second.Owner())
		if cause != nil {
			return cause
		}
		coefficient := new(big.Rat).Abs(antecedents.equalityCoefficients[i])
		if err = farkas.Add(coefficient, expression); err != nil {
			return err
		}
		added = true
	}
	if !added || watchCoefficient.Sign() == 0 {
		return nil
	}
	consequenceRef, err := state.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, state.manager.DelRef(&consequenceRef)) }()
	if err = farkas.Get(consequenceRef); err != nil {
		return err
	}
	consequence, err := state.manager.RefGet(consequenceRef)
	if err != nil {
		return err
	}
	operation, left, right, matched, err := farkas.operation(consequence)
	if err != nil {
		return err
	}
	if negated, child, cause := state.manager.IsNot(consequence); cause != nil {
		return cause
	} else if negated {
		operation, left, right, matched, err = farkas.operation(child)
		if err != nil {
			return err
		}
		operation = map[int32]int32{
			smtArithOpLE: smtArithOpGT,
			smtArithOpLT: smtArithOpGE,
			smtArithOpGE: smtArithOpLT,
			smtArithOpGT: smtArithOpLE,
		}[operation]
	}
	if !matched || operation != smtArithOpLE && operation != smtArithOpLT && operation != smtArithOpGE && operation != smtArithOpGT && operation != ast.ASTOpEq {
		return nil
	}
	formLeft, linear, err := t.linearize(left)
	if err != nil || !linear {
		return err
	}
	formRight, linear, err := t.linearize(right)
	if err != nil || !linear {
		return err
	}
	strict := operation == smtArithOpLT || operation == smtArithOpGT
	if operation == smtArithOpGE || operation == smtArithOpGT {
		formLeft, formRight = formRight, formLeft
	}
	formRight.scale(big.NewRat(-1, 1))
	formLeft.add(&formRight)
	objectiveIndex, found := t.atomByBool[t.recordConflictBound.Var()]
	if !found || objectiveIndex < 0 || objectiveIndex >= len(t.atoms) || t.atoms[objectiveIndex].native == nil {
		return ast.ErrASTDomain
	}
	objective, err := t.Enode(t.atoms[objectiveIndex].native.variable)
	if err != nil {
		return err
	}
	objectiveForm, linear, err := t.linearize(objective.Owner())
	if err != nil || !linear {
		return err
	}
	factor, err := farkas.NormalizeFactor()
	if err != nil {
		return err
	}
	watchCoefficient.Mul(watchCoefficient, factor)
	objectiveForm.scale(watchCoefficient)
	formLeft.add(&objectiveForm)
	if len(formLeft.terms) != 0 || watchCoefficient.Sign() == 0 {
		return nil
	}
	formLeft.infinity.Quo(&formLeft.infinity, watchCoefficient)
	formLeft.constant.Quo(&formLeft.constant, watchCoefficient)
	formLeft.infinitesimal.Quo(&formLeft.infinitesimal, watchCoefficient)
	var candidate simplex.Infinitesimal
	candidate.Infinity.Set(&formLeft.infinity)
	candidate.Main.Set(&formLeft.constant)
	candidate.Epsilon.Set(&formLeft.infinitesimal)
	if strict {
		column := t.simplexVariables[t.atoms[objectiveIndex].native.variable]
		if t.integer || t.integerColumns[column] {
			candidate.Main.Sub(&candidate.Main, big.NewRat(1, 1))
		} else {
			candidate.Epsilon.Sub(&candidate.Epsilon, big.NewRat(1, 1))
		}
	}
	if candidate.Infinity.Sign() != 0 {
		return nil
	}
	if !t.conflictUpperFinite || candidate.Cmp(&t.conflictUpper) > 0 {
		t.conflictUpper = candidate
		t.conflictUpperFinite = true
	}
	return nil
}
