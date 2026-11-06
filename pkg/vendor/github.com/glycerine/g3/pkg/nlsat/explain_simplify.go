package nlsat

// Source: src/nlsat/nlsat_explain.cpp:998-1298.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type explanationEqualityInfo struct {
	equality                  polynomial.Polynomial
	variable, degree          uint32
	leading                   polynomial.Polynomial
	leadingSign               int
	leadingConstant           bool
	addLeading, addInequality bool
}

func (n *explanationAssumptions) selectCoreEquality(core *scopedLiteralVector, max uint32) (polynomial.Polynomial, error) {
	var selected polynomial.Polynomial
	minimum := uint32(math.MaxUint32)
	for _, literal := range core.result {
		if literal.Sign() {
			continue
		}
		atom, err := n.search.store.literalAtom(literal)
		if err != nil {
			return polynomial.Polynomial{}, err
		}
		inequality, ok := atom.(*InequalityAtom)
		if !ok || inequality.kind != AtomEQ || len(inequality.factors) != 1 || inequality.factors[0].Even {
			continue
		}
		degree, err := n.search.store.atoms.ineq.core.Degree(inequality.factors[0].Polynomial, max)
		if err != nil {
			return polynomial.Polynomial{}, err
		}
		if degree < minimum {
			selected, minimum = inequality.factors[0].Polynomial, degree
			if degree == 1 {
				break
			}
		}
	}
	return selected, nil
}

func (n *explanationAssumptions) rewriteEqualityLiteral(literal sat.Literal, info *explanationEqualityInfo, max uint32) (result sat.Literal, modified bool, err error) {
	atom, err := n.search.store.literalAtom(literal)
	if err != nil {
		return result, false, err
	}
	inequality, ok := atom.(*InequalityAtom)
	if !ok {
		return literal, false, nil
	}
	if len(inequality.factors) == 1 {
		equal, equalErr := n.search.store.atoms.ineq.core.Equal(inequality.factors[0].Polynomial, info.equality)
		if equalErr != nil {
			return result, false, equalErr
		}
		if equal {
			return literal, false, nil
		}
	}
	core := n.search.store.atoms.ineq.core
	factors, err := polynomial.NewPolynomialRefVector(core)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, factors.Close()) }()
	remainder, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, remainder.Close()) }()
	even := make([]bool, 0, len(inequality.factors))
	atomSign := 1
	for _, factor := range inequality.factors {
		degree, degreeErr := core.Degree(factor.Polynomial, info.variable)
		if degreeErr != nil {
			return result, false, degreeErr
		}
		if degree < info.degree {
			if err = factors.Push(factor.Polynomial); err != nil {
				return result, false, err
			}
			even = append(even, factor.Even)
			continue
		}
		modified = true
		var d uint32
		if err = core.PseudoRemainder(factor.Polynomial, info.equality, info.variable, &d, remainder); err != nil {
			return result, false, err
		}
		value, valueErr := remainder.Get()
		if valueErr != nil {
			return result, false, valueErr
		}
		if d%2 == 1 && !factor.Even && info.leadingSign < 0 {
			atomSign = -atomSign
		}
		constant, constantErr := core.IsConstant(value)
		if constantErr != nil {
			return result, false, constantErr
		}
		if constant {
			sign, signErr := n.sign(value)
			if signErr != nil {
				return result, false, signErr
			}
			if sign == 0 {
				atomValue := inequality.kind == AtomEQ
				if !info.leadingConstant {
					info.addLeading = true
				}
				return explanationTruthLiteral(atomValue != literal.Sign()), true, nil
			}
			if !info.leadingConstant {
				info.addLeading = true
				if d%2 == 1 && !factor.Even && inequality.kind != AtomEQ {
					info.addInequality = true
				}
			}
			if sign < 0 && !factor.Even {
				atomSign = -atomSign
			}
			continue
		}
		if err = factors.Push(value); err != nil {
			return result, false, err
		}
		even = append(even, factor.Even)
		if !info.leadingConstant {
			info.addLeading = true
			if d%2 == 1 && !factor.Even && inequality.kind != AtomEQ {
				info.addInequality = true
			}
		}
	}
	if !modified {
		return literal, false, nil
	}
	kind := inequality.kind
	if atomSign < 0 {
		kind, err = kind.Flip()
		if err != nil {
			return result, false, err
		}
	}
	count, err := factors.Size()
	if err != nil {
		return result, false, err
	}
	values := make([]polynomial.Polynomial, count)
	for i := range values {
		values[i], err = factors.Get(uint32(i))
		if err != nil {
			return result, false, err
		}
	}
	result, err = n.search.store.atoms.mkInequalityLiteral(kind, values, even)
	if err != nil {
		return result, false, err
	}
	if literal.Sign() {
		result = result.Negated()
	}
	if result == TrueLiteral || result == FalseLiteral {
		return result, true, nil
	}
	resultMax, err := n.search.store.maxVar([]sat.Literal{result})
	if err != nil {
		return result, false, err
	}
	if resultMax < max {
		value, valueErr := n.search.valueCore(result)
		if valueErr != nil {
			return result, false, valueErr
		}
		if value == lbool.False {
			if err = n.addLiteral(result); err != nil {
				return result, false, err
			}
		}
		return TrueLiteral, true, nil
	}
	result, err = n.normalizeLiteral(result, max)
	return result, true, err
}

func (n *explanationAssumptions) simplifyCoreWithEquality(core *scopedLiteralVector, equality polynomial.Polynomial, max uint32) (modified bool, err error) {
	polynomials := n.search.store.atoms.ineq.core
	variable, err := polynomials.MaxVariable(equality)
	if err != nil {
		return false, err
	}
	degree, err := polynomials.Degree(equality, variable)
	if err != nil {
		return false, err
	}
	leadingRef, err := polynomial.NewPolynomialRef(polynomials)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, leadingRef.Close()) }()
	leading, err := polynomials.Coeff(equality, variable, degree)
	if err != nil {
		return false, err
	}
	if err = leadingRef.Assign(leading); err != nil {
		return false, err
	}
	leading, err = leadingRef.Get()
	if err != nil {
		return false, err
	}
	leadingSign, err := n.sign(leading)
	if err != nil {
		return false, err
	}
	leadingConstant, err := polynomials.IsConstant(leading)
	if err != nil {
		return false, err
	}
	info := explanationEqualityInfo{equality: equality, variable: variable, degree: degree, leading: leading, leadingSign: leadingSign, leadingConstant: leadingConstant}
	j := uint32(0)
	for _, literal := range append([]sat.Literal(nil), core.result...) {
		newLiteral, changed, rewriteErr := n.rewriteEqualityLiteral(literal, &info, max)
		if rewriteErr != nil {
			return false, rewriteErr
		}
		modified = modified || changed
		if newLiteral == TrueLiteral {
			continue
		}
		if newLiteral == FalseLiteral {
			j = 0
			break
		}
		if err = core.set(j, newLiteral); err != nil {
			return false, err
		}
		j++
	}
	if err = core.shrink(j); err != nil {
		return false, err
	}
	if info.addLeading {
		if info.addInequality {
			kind := AtomGT
			if info.leadingSign < 0 {
				kind = AtomLT
			}
			err = n.addSimpleAssumption(kind, info.leading, false)
		} else {
			err = n.addSimpleAssumption(AtomEQ, info.leading, true)
		}
	}
	return modified, err
}

func (n *explanationAssumptions) simplifyCoreEqualities(core *scopedLiteralVector, max uint32) error {
	for len(core.result) != 0 {
		equality, err := n.selectCoreEquality(core, max)
		if err != nil {
			return err
		}
		if equality == (polynomial.Polynomial{}) {
			break
		}
		modified, err := n.simplifyCoreWithEquality(core, equality, max)
		if err != nil || !modified {
			return err
		}
	}
	for len(core.result) != 0 {
		equality, err := n.selectLowerStageEquality(core, max)
		if err != nil || equality == nil {
			return err
		}
		modified, err := n.simplifyCoreWithEquality(core, equality.factors[0].Polynomial, max)
		if err != nil {
			return err
		}
		if err = atomAssert("nlsat_explain.cpp", 1333, "simplify(C, eq_p, max)", modified); err != nil {
			return err
		}
		if !modified {
			return ErrSearchPending
		}
		if err = n.addLiteral(sat.MakeLiteral(equality.boolVar, true)); err != nil {
			return err
		}
	}
	return nil
}

func (n *explanationAssumptions) selectLowerStageEquality(core *scopedLiteralVector, max uint32) (*InequalityAtom, error) {
	polynomials := n.search.store.atoms.ineq.core
	for _, literal := range core.result {
		atom, err := n.search.store.literalAtom(literal)
		if err != nil {
			return nil, err
		}
		inequality, ok := atom.(*InequalityAtom)
		if !ok {
			continue
		}
		for _, factor := range inequality.factors {
			for variable := uint32(0); variable < max && uint64(variable) < uint64(len(n.search.store.vars.equalities)); variable++ {
				candidate, ok := n.search.store.vars.equalities[variable].(*InequalityAtom)
				if !ok || candidate == nil || len(candidate.factors) != 1 || candidate.factors[0].Even {
					continue
				}
				equality := candidate.factors[0].Polynomial
				degree, err := polynomials.Degree(equality, variable)
				if err != nil {
					return nil, err
				}
				constant, err := polynomials.NonzeroConstantCoefficient(equality, variable, degree)
				if err != nil {
					return nil, err
				}
				if !constant {
					continue
				}
				factorDegree, err := polynomials.Degree(factor.Polynomial, variable)
				if err != nil {
					return nil, err
				}
				if factorDegree >= degree {
					return candidate, nil
				}
			}
		}
	}
	return nil, nil
}
