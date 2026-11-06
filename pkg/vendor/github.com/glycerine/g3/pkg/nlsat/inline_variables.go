package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2300-2378.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type inlineVariablePatch struct {
	variable               uint32
	numerator, denominator *polynomial.PolynomialRef
}

func (s *Solver) inlineVariableSigns() ([]polynomial.PolynomialSign, error) {
	signs := make([]polynomial.PolynomialSign, len(s.clauses.vars.isInt))
	for _, clause := range s.clauses.originals {
		if clause.Size() != 1 {
			continue
		}
		literal := clause.Literals()[0]
		atom, err := s.clauses.literalAtom(literal)
		if err != nil {
			return nil, err
		}
		ineq, ok := atom.(*InequalityAtom)
		if !ok || len(ineq.factors) != 1 || ineq.factors[0].Even {
			continue
		}
		var variable uint32
		matched, err := s.config.Polynomial.IsVariable(ineq.factors[0].Polynomial, &variable)
		if err != nil {
			return nil, err
		}
		if matched {
			if uint64(variable) >= uint64(len(signs)) || literal.Sign() {
				continue
			}
			switch ineq.kind {
			case AtomGT:
				signs[variable] = polynomial.PolynomialSignPositive
			case AtomLT:
				signs[variable] = polynomial.PolynomialSignNegative
			}
			continue
		}
		var numeral unumeral.Int
		matched, numeralSign, cause := s.inlineVariableNumeral(ineq.factors[0].Polynomial, &variable, &numeral)
		if cause != nil {
			return nil, cause
		}
		if !matched || uint64(variable) >= uint64(len(signs)) {
			continue
		}
		switch {
		case !literal.Sign() && ineq.kind == AtomGT && numeralSign >= 0:
			signs[variable] = polynomial.PolynomialSignPositive
		case !literal.Sign() && ineq.kind == AtomLT && numeralSign <= 0:
			signs[variable] = polynomial.PolynomialSignNegative
		case literal.Sign() && ineq.kind == AtomGT && numeralSign > 0:
			signs[variable] = polynomial.PolynomialSignNegative
		case literal.Sign() && ineq.kind == AtomLT && numeralSign < 0:
			signs[variable] = polynomial.PolynomialSignPositive
		}
	}
	return signs, nil
}

func (s *Solver) inlineVariableNumeral(p polynomial.Polynomial, variable *uint32, numeral *unumeral.Int) (matched bool, sign int32, err error) {
	core, err := s.config.Polynomial.ArithmeticCore()
	if err != nil {
		return false, 0, err
	}
	integers, err := core.IntegerManager()
	if err != nil {
		return false, 0, err
	}
	defer func() {
		err = errors.Join(err, integers.Del(numeral), s.config.Polynomial.CleanupVariableNumeral())
	}()
	matched, err = s.config.Polynomial.IsVariableNumeral(p, variable, numeral)
	if err != nil || !matched {
		return matched, 0, err
	}
	sign, err = integers.Sign(numeral)
	return matched, sign, err
}

func (s *Solver) inlineCandidate(c *Clause, signs []polynomial.PolynomialSign) (uint32, *polynomial.PolynomialRef, *polynomial.PolynomialRef, bool, error) {
	if c.Size() != 1 {
		return 0, nil, nil, false, nil
	}
	literal := c.Literals()[0]
	if literal.Sign() {
		return 0, nil, nil, false, nil
	}
	atom, err := s.clauses.literalAtom(literal)
	if err != nil {
		return 0, nil, nil, false, err
	}
	ineq, ok := atom.(*InequalityAtom)
	if !ok || ineq.kind != AtomEQ || len(ineq.factors) != 1 || ineq.factors[0].Even {
		return 0, nil, nil, false, nil
	}
	core, err := s.config.Polynomial.ArithmeticCore()
	if err != nil {
		return 0, nil, nil, false, err
	}
	p0 := ineq.factors[0].Polynomial
	maximum, err := core.MaxVariable(p0)
	if err != nil {
		return 0, nil, nil, false, err
	}
	if maximum == NullVariable || uint64(maximum) >= uint64(len(s.clauses.vars.isInt)) {
		return 0, nil, nil, false, nil
	}
	for variable := uint32(0); variable <= maximum; variable++ {
		if s.clauses.vars.isInt[variable] {
			continue
		}
		degree, cause := core.Degree(p0, variable)
		if cause != nil {
			return 0, nil, nil, false, cause
		}
		if degree != 1 {
			continue
		}
		reduct, cause := polynomial.NewPolynomialRef(core)
		if cause != nil {
			return 0, nil, nil, false, cause
		}
		coefficient, cause := polynomial.NewPolynomialRef(core)
		if cause != nil {
			return 0, nil, nil, false, errors.Join(cause, reduct.Close())
		}
		var call polynomial.PolynomialCoefficientCall
		cause = core.CoeffWithReductInto(p0, variable, 1, coefficient, reduct, &call)
		if cause != nil {
			return 0, nil, nil, false, errors.Join(cause, call.CleanupRetained(), coefficient.Close(), reduct.Close())
		}
		coefficientValue, cause := coefficient.Get()
		if cause != nil {
			return 0, nil, nil, false, errors.Join(cause, coefficient.Close(), reduct.Close())
		}
		sign, cause := s.config.Polynomial.Sign(coefficientValue, signs)
		if cause != nil || sign == polynomial.PolynomialSignUnknown {
			if closeErr := errors.Join(coefficient.Close(), reduct.Close()); cause != nil || closeErr != nil {
				return 0, nil, nil, false, errors.Join(cause, closeErr)
			}
			continue
		}
		if sign == polynomial.PolynomialSignNegative {
			negated, negCause := core.Neg(coefficientValue)
			if negCause == nil {
				negCause = coefficient.Assign(negated)
			}
			if negCause != nil {
				return 0, nil, nil, false, errors.Join(negCause, coefficient.Close(), reduct.Close())
			}
			return variable, coefficient, reduct, true, nil
		}
		reductValue, cause := reduct.Get()
		if cause != nil {
			return 0, nil, nil, false, errors.Join(cause, coefficient.Close(), reduct.Close())
		}
		numerator, cause := polynomial.NewPolynomialRef(core)
		if cause == nil {
			var negated polynomial.Polynomial
			negated, cause = core.Neg(reductValue)
			if cause == nil {
				cause = numerator.Assign(negated)
			}
		}
		if cause != nil {
			if numerator != nil {
				cause = errors.Join(cause, numerator.Close())
			}
			return 0, nil, nil, false, errors.Join(cause, coefficient.Close(), reduct.Close())
		}
		if cause = reduct.Close(); cause != nil {
			return 0, nil, nil, false, errors.Join(cause, numerator.Close(), coefficient.Close())
		}
		return variable, coefficient, numerator, true, nil
	}
	return 0, nil, nil, false, nil
}

func (s *Solver) rememberInlinePatch(variable uint32, numerator, denominator *polynomial.PolynomialRef) {
	s.patches = append(s.patches, inlineVariablePatch{variable: variable, numerator: numerator, denominator: denominator})
}

func (s *Solver) substituteInlineVariable(variable uint32, denominator, numerator polynomial.Polynomial) error {
	core, err := s.config.Polynomial.ArithmeticCore()
	if err != nil {
		return err
	}
	replacements := make(map[sat.BoolVar]sat.Literal)
	numAtoms := len(s.atoms.bools.atoms)
	for j := 0; j < numAtoms; j++ {
		old, ok := s.atoms.bools.atoms[j].(*InequalityAtom)
		if !ok || old == nil {
			continue
		}
		factors := make([]polynomial.Polynomial, 0, len(old.factors))
		even := make([]bool, 0, len(old.factors))
		changed := false
		vanished := false
		refs := make([]*polynomial.PolynomialRef, 0, len(old.factors))
		for _, factor := range old.factors {
			degree, cause := core.Degree(factor.Polynomial, variable)
			if cause != nil {
				err = cause
				break
			}
			if degree == 0 {
				factors = append(factors, factor.Polynomial)
				even = append(even, factor.Even)
				continue
			}
			out, cause := polynomial.NewPolynomialRef(core)
			if cause != nil {
				err = cause
				break
			}
			refs = append(refs, out)
			if cause = core.SubstitutePolynomialFraction(factor.Polynomial, variable, numerator, denominator, out); cause != nil {
				err = cause
				break
			}
			value, cause := out.Get()
			if cause != nil {
				err = cause
				break
			}
			zero, cause := core.IsZero(value)
			if cause != nil {
				err = cause
				break
			}
			if zero {
				factors = factors[:0]
				even = even[:0]
				changed = true
				vanished = true
				break
			}
			factors = append(factors, value)
			even = append(even, factor.Even)
			changed = changed || value != factor.Polynomial
		}
		if err == nil && changed {
			var literal sat.Literal
			var cause error
			if vanished {
				literal = FalseLiteral
				if old.kind == AtomEQ {
					literal = TrueLiteral
				}
			} else if len(factors) == 0 {
				literal, cause = s.atoms.mkInequalityLiteral(old.kind, factors, even)
			} else {
				literal, cause = s.MkInequality(old.kind, factors, even, false)
			}
			if cause != nil {
				err = cause
			} else if literal.Variable() != old.boolVar {
				replacements[old.boolVar] = literal
			}
		}
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, refs[i].Close())
		}
		if err != nil {
			return err
		}
	}
	return s.rewriteInlineClauses(replacements)
}

func (s *Solver) rewriteInlineClauses(replacements map[sat.BoolVar]sat.Literal) error {
	if len(replacements) == 0 {
		return nil
	}
	clauses := append([]*Clause(nil), s.clauses.originals...)
	for _, clause := range clauses {
		changed, tautology := false, false
		literals := make([]sat.Literal, 0, len(clause.Literals()))
		seen := make(map[sat.Literal]bool)
		for _, literal := range clause.Literals() {
			if replacement, ok := replacements[literal.Variable()]; ok {
				changed = true
				if literal.Sign() {
					replacement = replacement.Negated()
				}
				literal = replacement
			}
			if literal == TrueLiteral || seen[literal.Negated()] {
				tautology = true
				break
			}
			if literal == FalseLiteral || seen[literal] {
				continue
			}
			seen[literal] = true
			literals = append(literals, literal)
		}
		if !changed {
			continue
		}
		if !tautology {
			if len(literals) == 0 {
				literals = append(literals, FalseLiteral)
			}
			if _, err := s.clauses.mkClause(literals, clause.IsLearned(), clause.Assumptions()); err != nil {
				return err
			}
		}
		if err := s.clauses.deleteClause(clause); err != nil {
			return err
		}
	}
	return nil
}

func (s *Solver) inlineVariables() error {
	if len(s.clauses.learned) != 0 {
		return nil
	}
	signs, err := s.inlineVariableSigns()
	if err != nil {
		return err
	}
	for {
		changed := false
		for _, clause := range append([]*Clause(nil), s.clauses.originals...) {
			variable, denominatorRef, numeratorRef, ok, err := s.inlineCandidate(clause, signs)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			denominator, err := denominatorRef.Get()
			if err != nil {
				return errors.Join(err, numeratorRef.Close(), denominatorRef.Close())
			}
			numerator, err := numeratorRef.Get()
			if err != nil {
				return errors.Join(err, numeratorRef.Close(), denominatorRef.Close())
			}
			s.rememberInlinePatch(variable, numeratorRef, denominatorRef)
			if err = s.clauses.deleteClause(clause); err != nil {
				return err
			}
			if err = s.substituteInlineVariable(variable, denominator, numerator); err != nil {
				return err
			}
			changed = true
			break
		}
		if !changed {
			return nil
		}
	}
}

func (s *Solver) fixInlineVariablePatches() (err error) {
	for i := len(s.patches) - 1; i >= 0; i-- {
		patch := &s.patches[i]
		numerator, cause := patch.numerator.Get()
		if cause != nil {
			return cause
		}
		denominator, cause := patch.denominator.Get()
		if cause != nil {
			return cause
		}
		var nv, dv, value polynomial.Algebraic
		if cause = s.config.Polynomial.EvaluateAlgebraic(numerator, s.config.Algebraic.AlgebraicRootCore, s.assignment, &nv); cause == nil {
			cause = s.config.Polynomial.EvaluateAlgebraic(denominator, s.config.Algebraic.AlgebraicRootCore, s.assignment, &dv)
		}
		if cause == nil {
			cause = s.config.Algebraic.Div(&nv, &dv, &value)
		}
		if cause == nil {
			cause = s.assignment.SetCore(patch.variable, &value)
		}
		err = errors.Join(cause, s.config.Algebraic.Del(&value), s.config.Algebraic.Del(&dv), s.config.Algebraic.Del(&nv))
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Solver) closeInlineVariablePatches() (err error) {
	for i := len(s.patches) - 1; i >= 0; i-- {
		err = errors.Join(err, s.patches[i].denominator.Close(), s.patches[i].numerator.Close())
		if err != nil {
			return err
		}
	}
	s.patches = nil
	return nil
}
