package smtcore

// Source: src/smt/theory_arith_core.h row bound propagation and fixed-value
// equality propagation. Only unconditional replay bounds justify equalities.

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Derive intervals for each variable in a small canonical row. Keeping only
// newly fixed original columns avoids an infinite sequence of fractional
// improvements when no available bound atom requires the intermediate bounds.
func (t *SMTLinearArithmeticTheory) propagateFixedSharedValues() (bool, error) {
	var shared []uint32
	for variable, column := range t.simplexVariables {
		node, err := t.Enode(SMTTheoryVar(variable))
		if err != nil {
			return false, err
		}
		isShared, err := t.sharedArithmeticNode(node)
		if err != nil {
			return false, err
		}
		if isShared {
			shared = append(shared, column)
		}
	}
	if len(shared) < 2 {
		return false, nil
	}
	solver, status, err := t.buildSimplex()
	if err != nil || status == lbool.Undef {
		return false, err
	}
	if status == lbool.False {
		return true, t.derivedBoundConflict(solver)
	}
	alreadyFixed := make(map[uint32]bool)
	for _, column := range t.simplexVariables {
		_, alreadyFixed[column] = smtFixedSimplexColumn(solver, column)
	}
	if conflict, err := t.propagateArithmeticRowBounds(solver); err != nil {
		return false, err
	} else if conflict {
		return true, t.derivedBoundConflict(solver)
	}
	status, err = solver.MakeFeasible()
	if err != nil || status == lbool.Undef {
		return false, err
	}
	if status == lbool.False {
		return true, t.derivedBoundConflict(solver)
	}
	progress := false
	for _, column := range t.simplexVariables {
		value, fixed := smtFixedSimplexColumn(solver, column)
		if !fixed || alreadyFixed[column] {
			continue
		}
		bound := simplex.RationalInfinitesimal(value)
		t.derivedBounds = append(t.derivedBounds,
			smtArithmeticReplayBound{column: column, lower: true, value: bound.Clone(), explanation: smtArithmeticCopyBoundExplanation(solver, column, true)},
			smtArithmeticReplayBound{column: column, lower: false, value: bound.Clone(), explanation: smtArithmeticCopyBoundExplanation(solver, column, false)})
		progress = true
	}
	if progress {
		t.solver = solver
		t.state.theoryProgress++
	}
	for i, first := range shared {
		firstValue, fixed := smtFixedSimplexColumn(solver, first)
		if !fixed {
			continue
		}
		for _, second := range shared[i+1:] {
			secondValue, fixed := smtFixedSimplexColumn(solver, second)
			if !fixed || firstValue.Cmp(secondValue) != 0 {
				continue
			}
			merged, err := t.propagateFixedEquality(solver, first, second)
			if err != nil || merged {
				return true, err
			}
		}
	}
	return progress, nil
}

// Source theory_arith_eq.h fixed_var_eh pushes four endpoint justifications
// in this order, using zero coefficients for its equality proof rule.
func (t *SMTLinearArithmeticTheory) propagateFixedEquality(solver *simplex.Solver, first, second uint32) (progress bool, result error) {
	if !t.canPropagateArithmeticEqualities() {
		return false, nil
	}
	firstLower, firstHasLower := solver.Lower(first)
	firstUpper, firstHasUpper := solver.Upper(first)
	secondLower, secondHasLower := solver.Lower(second)
	secondUpper, secondHasUpper := solver.Upper(second)
	firstFixed := firstHasLower && firstHasUpper && firstLower.Cmp(&firstUpper) == 0
	secondFixed := secondHasLower && secondHasUpper && secondLower.Cmp(&secondUpper) == 0
	if err := arithmeticEqualityAssertion(40, "is_fixed(v)", firstFixed && secondFixed); err != nil {
		return false, err
	}
	if !firstFixed || !secondFixed {
		return false, nil
	}
	if err := t.emitArithmeticEquality("arith_bug", "fixed_var_eh", 43, func() bool { return !firstLower.IsRational() }, func(out io.Writer) error {
		if err := t.displayArithmeticEqualityVariable(out, first); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return false, err
	}
	if err := arithmeticEqualityAssertion(44, "lower_bound(v).is_rational()", firstLower.IsRational() && secondLower.IsRational()); err != nil {
		return false, err
	}
	if !firstLower.IsRational() || !secondLower.IsRational() || firstLower.Main.Cmp(&secondLower.Main) != 0 {
		return false, nil
	}
	firstValue := &firstLower.Main
	if err := t.emitArithmeticEquality("arith_eq", "fixed_var_eh", 47, func() bool { return true }, func(out io.Writer) error {
		if err := t.displayArithmeticEqualityVariable(out, first); err != nil {
			return err
		}
		return smtWriteTheoryText(out, " = "+firstValue.RatString()+"\n")
	}); err != nil {
		return false, err
	}
	proofs, err := t.state.manager.ProofsEnabled()
	if err != nil {
		return false, err
	}
	uses := []simplex.ArithmeticBoundUse{
		{Column: first, Lower: true},
		{Column: second, Lower: false},
		{Column: second, Lower: true},
		{Column: first, Lower: false},
	}
	antecedents, known, err := smtArithmeticExplainBounds(solver, uses, proofs, t.equalityBounds)
	if err != nil {
		return false, err
	}
	if err = t.emitArithmeticEquality("arith_eq", "fixed_var_eh", 68, func() bool { return true }, func(out io.Writer) error {
		if err := smtWriteTheoryText(out, fmt.Sprintf("propagate eq: v%d = v%d\n", first, second)); err != nil {
			return err
		}
		if err := t.displayArithmeticEqualityVariable(out, first); err != nil {
			return err
		}
		if err := smtWriteTheoryText(out, "\n"); err != nil {
			return err
		}
		if err := t.displayArithmeticEqualityVariable(out, second); err != nil {
			return err
		}
		return smtWriteTheoryText(out, "\n")
	}); err != nil {
		return false, err
	}
	if !known {
		// Logical replay may contain an endpoint whose explanation has not
		// yet been ported. Preserve the existing conservative justification.
		antecedents, err := t.allArithmeticAntecedents()
		if err != nil {
			return false, err
		}
		return t.propagateEqualityWithParameters(t.state, first, second, antecedents.literals, nil, antecedents.equalities)
	}
	parameters, err := antecedents.parameters(t.state.manager, "eq-propagate")
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(t.state.manager, parameters)) }()
	return t.propagateEqualityWithParameters(t.state, first, second, antecedents.literals, parameters, antecedents.equalities)
}

// Derive bounds through canonical equality and comparison rows. The caller
// owns replay and records only bounds supported by unconditional assertions.
func (t *SMTLinearArithmeticTheory) propagateArithmeticRowBounds(solver *simplex.Solver) (bool, error) {
	maxLemmaSize := t.maxLemmaSize
	if maxLemmaSize == 0 {
		maxLemmaSize = 128
	}
	for column := uint32(0); column < solver.NumVariables(); column++ {
		row, basic := solver.BasicRow(column)
		if !basic {
			continue
		}
		entries, err := solver.RowEntries(row)
		if err != nil {
			return false, err
		}
		if uint64(len(entries)) >= uint64(maxLemmaSize) {
			continue
		}
		if t.skipRowsWithBigCoefficients && smtArithmeticRowHasBigCoefficient(entries) {
			continue
		}
		for _, target := range entries {
			for _, lower := range []bool{true, false} {
				var bound simplex.Infinitesimal
				var uses []simplex.ArithmeticBoundUse
				finite := true
				for _, other := range entries {
					if other.Variable == target.Variable {
						continue
					}
					coefficient := new(big.Rat).Neg(new(big.Rat).Quo(other.Coefficient, target.Coefficient))
					useLower := lower == (coefficient.Sign() > 0)
					endpoint, exists := solver.Upper(other.Variable)
					if useLower {
						endpoint, exists = solver.Lower(other.Variable)
					}
					if !exists {
						finite = false
						break
					}
					// Linear row arithmetic operates on inf_numeral, preserving the
					// coefficient and sign of epsilon. Rational interval projection can
					// strengthen a bound such as 5-epsilon to 5 and invent an equality.
					bound.Infinity.Add(&bound.Infinity, new(big.Rat).Mul(coefficient, &endpoint.Infinity))
					bound.Main.Add(&bound.Main, new(big.Rat).Mul(coefficient, &endpoint.Main))
					bound.Epsilon.Add(&bound.Epsilon, new(big.Rat).Mul(coefficient, &endpoint.Epsilon))
					uses = append(uses, simplex.ArithmeticBoundUse{Column: other.Variable, Lower: useLower, Coefficient: *coefficient})
				}
				if !finite {
					continue
				}
				if t.integer || t.integerColumns[target.Variable] {
					integer := floorSMTRational(&bound.Main)
					if lower && (!bound.Main.IsInt() || bound.Epsilon.Sign() > 0) {
						integer.Add(integer, big.NewInt(1))
					} else if !lower && bound.Main.IsInt() && bound.Epsilon.Sign() < 0 {
						integer.Sub(integer, big.NewInt(1))
					}
					bound.Infinity.SetInt64(0)
					bound.Main.SetInt(integer)
					bound.Epsilon.SetInt64(0)
				}
				explanation := smtArithmeticWeightedBoundExplanation(solver, uses)
				if _, err := smtTightenReplaySimplexBound(solver, target.Variable, lower, bound, explanation); err != nil {
					t.recordArithmeticBoundConflict(solver, target.Variable, lower, explanation)
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// Source propagate_bounds turns an implied row endpoint into the corresponding
// unassigned Boolean arithmetic atom. The replay tableau owns endpoint values;
// the context justification owns copied logical premises and proof parameters.
func (t *SMTLinearArithmeticTheory) propagateArithmeticBoundAtoms() error {
	if t.boundPropagation == 0 || t.conflictCount >= t.propagationThreshold {
		return nil
	}
	// Source row propagation computes implications without installing derived
	// numeric bounds into the candidate tableau. Use a separate logical replay:
	// only a published Boolean consequence may affect the next live rebuild.
	solver, status, err := t.buildSimplex()
	if err != nil {
		return err
	}
	if status == lbool.False {
		return t.derivedBoundConflict(solver)
	}
	if status == lbool.Undef {
		return nil
	}
	bases := t.arithmeticAtomBases()
	rowsByColumn, err := t.indexArithmeticPropagationRows(solver)
	if err != nil {
		return err
	}
	proofs, err := t.state.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	for index := range t.atoms {
		atom := &t.atoms[index]
		if !atom.supported || atom.decided {
			continue
		}
		positive := NewSMTLiteral(atom.variable, false)
		assignment, err := t.state.Assignment(positive)
		if err != nil {
			return err
		}
		if assignment != lbool.Undef {
			continue
		}
		base := bases[index]
		for _, truth := range []bool{true, false} {
			lower, threshold := smtLinearAtomBound(atom, truth, t.integralAtom(atom))
			endpoint, explanation, found, err := t.impliedArithmeticRowEndpoint(solver, rowsByColumn[base], base, lower)
			if err != nil {
				return err
			}
			if !found || lower && endpoint.Cmp(&threshold) < 0 || !lower && endpoint.Cmp(&threshold) > 0 {
				continue
			}
			if t.strongerLemmas && len(explanation.uses) != 0 {
				explanation = t.relaxArithmeticRowExplanation(solver, explanation.uses, endpoint, threshold, lower, bases)
			}
			var antecedents smtArithmeticAntecedents
			for _, dependency := range explanation.dependencies {
				if err = smtArithmeticPushDependency(&antecedents, dependency, &dependency.Coefficient, proofs, t.equalityBounds); err != nil {
					return err
				}
			}
			literal := NewSMTLiteral(atom.variable, !truth)
			if err = t.assignArithmeticBoundLiteral(literal, &antecedents); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

// indexArithmeticPropagationRows preserves the source row scan while sharing
// it across every candidate atom. Without this reverse index, a large set of
// independent arithmetic atoms rescans the complete simplex matrix twice per
// atom even though almost every row cannot mention the candidate column.
func (t *SMTLinearArithmeticTheory) indexArithmeticPropagationRows(solver *simplex.Solver) (map[uint32][][]simplex.RowEntry, error) {
	result := make(map[uint32][][]simplex.RowEntry)
	maxLemmaSize := t.maxLemmaSize
	if maxLemmaSize == 0 {
		maxLemmaSize = 128
	}
	for basic := uint32(0); basic < solver.NumVariables(); basic++ {
		row, exists := solver.BasicRow(basic)
		if !exists {
			continue
		}
		entries, err := solver.RowEntries(row)
		if err != nil {
			return nil, err
		}
		if uint64(len(entries)) >= uint64(maxLemmaSize) || t.skipRowsWithBigCoefficients && smtArithmeticRowHasBigCoefficient(entries) {
			continue
		}
		for _, entry := range entries {
			result[entry.Variable] = append(result[entry.Variable], entries)
		}
	}
	return result, nil
}

func (t *SMTLinearArithmeticTheory) arithmeticAtomBases() []uint32 {
	bases := make([]uint32, len(t.atoms))
	coefficientBases := make(map[string]uint32)
	for index := range t.atoms {
		atom := &t.atoms[index]
		if !atom.supported {
			continue
		}
		key := smtArithmeticCoefficientKey(&atom.form)
		if base, found := coefficientBases[key]; found {
			bases[index] = base
			continue
		}
		base := t.nextSimplex + uint32(index)
		bases[index] = base
		coefficientBases[key] = base
	}
	return bases
}

// impliedArithmeticRowEndpoint computes the strongest currently justified
// endpoint for one column without installing it. This is the source
// imply_bound_for_monomial operation: integer normalization happens only when
// comparing against an integer atom threshold, never by changing the tableau.
func (t *SMTLinearArithmeticTheory) impliedArithmeticRowEndpoint(solver *simplex.Solver, rows [][]simplex.RowEntry, column uint32, lower bool) (simplex.Infinitesimal, *smtArithmeticBoundExplanation, bool, error) {
	var best simplex.Infinitesimal
	var explanation *smtArithmeticBoundExplanation
	found := false
	if lower {
		if endpoint, exists := solver.Lower(column); exists {
			if reason := smtArithmeticCopyBoundExplanation(solver, column, true); reason != nil {
				best, explanation, found = endpoint, reason, true
			}
		}
	} else if endpoint, exists := solver.Upper(column); exists {
		if reason := smtArithmeticCopyBoundExplanation(solver, column, false); reason != nil {
			best, explanation, found = endpoint, reason, true
		}
	}
	for _, entries := range rows {
		for _, target := range entries {
			if target.Variable != column {
				continue
			}
			var candidate simplex.Infinitesimal
			var uses []simplex.ArithmeticBoundUse
			finite := true
			for _, other := range entries {
				if other.Variable == column {
					continue
				}
				coefficient := new(big.Rat).Neg(new(big.Rat).Quo(other.Coefficient, target.Coefficient))
				useLower := lower == (coefficient.Sign() > 0)
				endpoint, endpointFound := solver.Upper(other.Variable)
				if useLower {
					endpoint, endpointFound = solver.Lower(other.Variable)
				}
				if !endpointFound {
					finite = false
					break
				}
				candidate.Infinity.Add(&candidate.Infinity, new(big.Rat).Mul(coefficient, &endpoint.Infinity))
				candidate.Main.Add(&candidate.Main, new(big.Rat).Mul(coefficient, &endpoint.Main))
				candidate.Epsilon.Add(&candidate.Epsilon, new(big.Rat).Mul(coefficient, &endpoint.Epsilon))
				uses = append(uses, simplex.ArithmeticBoundUse{Column: other.Variable, Lower: useLower, Coefficient: *coefficient})
			}
			if !finite {
				continue
			}
			reason := smtArithmeticWeightedBoundExplanation(solver, uses)
			if reason == nil {
				continue
			}
			if !found || lower && candidate.Cmp(&best) > 0 || !lower && candidate.Cmp(&best) < 0 {
				reason.uses = append(reason.uses[:0], uses...)
				best, explanation, found = candidate, reason, true
			}
		}
	}
	return best, explanation, found, nil
}

// Source rational::is_big is true when either normalized mpq component no
// longer fits the inline signed-int representation used by mpz.
func smtArithmeticRowHasBigCoefficient(entries []simplex.RowEntry) bool {
	const minSmall, maxSmall = int64(-(1<<31 - 1)), int64(1<<31 - 1)
	for _, entry := range entries {
		numerator, denominator := entry.Coefficient.Num(), entry.Coefficient.Denom()
		if !numerator.IsInt64() || !denominator.IsInt64() {
			return true
		}
		n, d := numerator.Int64(), denominator.Int64()
		if n < minSmall || n > maxSmall || d > maxSmall {
			return true
		}
	}
	return false
}

type smtArithmeticAssignedBoundCandidate struct {
	value      simplex.Infinitesimal
	dependency simplex.BoundDependency
}

// relaxArithmeticRowExplanation is source explain_bound with relax_bounds:
// spend the implication slack from left to right, replacing each strongest
// endpoint by the weakest assigned atom that remains sufficient.
func (t *SMTLinearArithmeticTheory) relaxArithmeticRowExplanation(solver *simplex.Solver, uses []simplex.ArithmeticBoundUse, endpoint, threshold simplex.Infinitesimal, lower bool, bases []uint32) *smtArithmeticBoundExplanation {
	delta := endpoint.Clone()
	if lower {
		delta.Sub(&threshold)
	} else {
		delta = threshold.Clone()
		delta.Sub(&endpoint)
	}
	result := &smtArithmeticBoundExplanation{uses: append([]simplex.ArithmeticBoundUse(nil), uses...)}
	positions := make(map[[2]uint32]int)
	push := func(dependency simplex.BoundDependency, coefficient *big.Rat) {
		weighted := new(big.Rat).Mul(&dependency.Coefficient, coefficient)
		key := smtArithmeticDependencyKey(dependency)
		if position, found := positions[key]; found {
			result.dependencies[position].Coefficient.Add(&result.dependencies[position].Coefficient, weighted)
			return
		}
		positions[key] = len(result.dependencies)
		result.dependencies = append(result.dependencies, simplex.BoundDependency{Literal: dependency.Literal, Equality: dependency.Equality, Coefficient: *weighted})
	}
	for _, use := range uses {
		current, found := solver.Upper(use.Column)
		if use.Lower {
			current, found = solver.Lower(use.Column)
		}
		if !found {
			return smtArithmeticWeightedBoundExplanation(solver, uses)
		}
		absolute := new(big.Rat).Abs(new(big.Rat).Set(&use.Coefficient))
		var selected *smtArithmeticAssignedBoundCandidate
		var selectedCost simplex.Infinitesimal
		for _, candidate := range t.assignedArithmeticBoundCandidates(use.Column, use.Lower, bases) {
			distance := current.Clone()
			if use.Lower {
				if candidate.value.Cmp(&current) >= 0 {
					continue
				}
				distance.Sub(&candidate.value)
			} else {
				if candidate.value.Cmp(&current) <= 0 {
					continue
				}
				distance = candidate.value.Clone()
				distance.Sub(&current)
			}
			distance.Mul(absolute)
			if distance.Cmp(&delta) > 0 || selected != nil && (use.Lower && candidate.value.Cmp(&selected.value) >= 0 || !use.Lower && candidate.value.Cmp(&selected.value) <= 0) {
				continue
			}
			copy := candidate
			selected, selectedCost = &copy, distance
		}
		if selected != nil {
			push(selected.dependency, &use.Coefficient)
			delta.Sub(&selectedCost)
			continue
		}
		dependencies, known := solver.BoundDependencies(use.Column, use.Lower)
		if !known {
			return smtArithmeticWeightedBoundExplanation(solver, uses)
		}
		for _, dependency := range dependencies {
			push(dependency, &use.Coefficient)
		}
	}
	return result
}

func (t *SMTLinearArithmeticTheory) assignedArithmeticBoundCandidates(column uint32, lower bool, bases []uint32) []smtArithmeticAssignedBoundCandidate {
	result := make([]smtArithmeticAssignedBoundCandidate, 0)
	for _, assertion := range t.assertions {
		atom := &t.atoms[assertion.atom]
		if !atom.supported {
			continue
		}
		boundLower, value := smtLinearAtomBound(atom, assertion.truth, t.integralAtom(atom))
		literal := smtLinearAssertionLiteral(atom, assertion)
		if bases[assertion.atom] == column && boundLower == lower {
			result = append(result, smtArithmeticAssignedBoundCandidate{value: value, dependency: simplex.BoundDependency{Literal: literal.Index(), Coefficient: *big.NewRat(1, 1)}})
		}
		if len(atom.form.terms) != 1 {
			continue
		}
		for variable, coefficient := range atom.form.terms {
			if variable != column || coefficient.Sign() == 0 || lower != (boundLower == (coefficient.Sign() > 0)) {
				continue
			}
			direct := value.Clone()
			direct.Quo(coefficient)
			result = append(result, smtArithmeticAssignedBoundCandidate{value: direct, dependency: simplex.BoundDependency{Literal: literal.Index(), Coefficient: *new(big.Rat).Inv(coefficient)}})
		}
	}
	return result
}

func (t *SMTLinearArithmeticTheory) assignArithmeticBoundLiteral(literal SMTLiteral, antecedents *smtArithmeticAntecedents) (result error) {
	if antecedents == nil {
		return ast.ErrASTDomain
	}
	m := t.state.manager
	parameters, err := antecedents.parameters(m, "assign-bounds")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, smtArithmeticReleaseParameters(m, parameters)) }()
	if uint64(len(antecedents.literals)) < uint64(t.smallLemmaSize) && len(antecedents.equalities) == 0 {
		clause := make([]SMTLiteral, 0, len(antecedents.literals)+1)
		for _, premise := range antecedents.literals {
			clause = append(clause, premise.Negated())
		}
		clause = append(clause, literal)
		var justification *SMTTheoryLemmaJustification
		proofs, err := m.ProofsEnabled()
		if err != nil {
			return err
		}
		if proofs {
			justification, err = NewSMTTheoryLemmaJustification(m, t.FamilyID(), clause, t.state.boolExpr, parameters)
			if err != nil {
				return err
			}
		}
		var owner SMTJustification
		if justification != nil {
			owner = justification
		}
		beforeClauses, beforeOwners := len(t.state.clauses), len(t.state.ownedJustifications)
		err = t.state.addClauseKindWithJustificationAndDeleteHandler(clause, false, SMTClauseAuxLemma, ast.ASTHandle{}, owner, nil)
		if justification != nil && len(t.state.clauses) == beforeClauses && len(t.state.ownedJustifications) == beforeOwners {
			err = errors.Join(err, justification.Release(m))
		}
		if err != nil {
			return err
		}
	} else {
		owner, err := NewSMTExtTheoryPropagationJustification(m, t.FamilyID(), antecedents.literals, antecedents.equalities, literal, parameters)
		if err != nil {
			return err
		}
		justification, err := NewSMTBooleanJustification(owner)
		if err != nil {
			return errors.Join(err, owner.Release(m))
		}
		if err = t.state.AssignLiteral(literal, justification); err != nil {
			return errors.Join(err, owner.Release(m))
		}
		t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	}
	t.statistics.boundPropagations++
	return nil
}
