package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:795-1037.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (c *simpleSumChecker) evaluateSum(form *simpleSumForm, domains []simplePowerDomain) (changed, possible bool, err error) {
	am := c.solver.config.Algebraic
	sign := simpleEQ
	for i := range form.terms {
		term := &form.terms[i]
		if uint64(term.variable) >= uint64(len(domains)) {
			return false, true, nil
		}
		current, cause := domains[term.variable].termSign(am, term)
		if cause != nil {
			return false, false, cause
		}
		sign = mergeSimpleSumSign(sign, current)
		if sign == simpleNone {
			return false, true, nil
		}
	}
	cSign := 0
	neg, cause := am.IsNeg(&form.constant)
	if cause != nil {
		return false, false, cause
	}
	if neg {
		cSign = -1
	} else {
		pos, cause := am.IsPos(&form.constant)
		if cause != nil {
			return false, false, cause
		}
		if pos {
			cSign = 1
		}
	}
	actionable, conflict, relation := simpleSumConsistency(sign, cSign, form.sign, len(form.terms))
	if conflict {
		return false, false, nil
	}
	if !actionable {
		return false, true, nil
	}
	for i := range form.terms {
		term := &form.terms[i]
		step, ok, cause := domains[term.variable].applySumTerm(am, term, &form.constant, relation)
		if cause != nil {
			return changed, false, cause
		}
		changed = changed || step
		if !ok {
			return changed, false, nil
		}
	}
	return changed, true, nil
}
func cloneSimplePowerDomains(am *polynomial.AlgebraicManager, src []simplePowerDomain) (dst []simplePowerDomain, err error) {
	for i := range src {
		copy, cause := src[i].clone(am)
		if cause != nil {
			for j := len(dst) - 1; j >= 0; j-- {
				err = errors.Join(err, dst[j].close(am))
			}
			return nil, errors.Join(cause, err)
		}
		dst = append(dst, copy)
	}
	return
}
func closeSimplePowerDomains(am *polynomial.AlgebraicManager, domains []simplePowerDomain) (err error) {
	for i := len(domains) - 1; i >= 0; i-- {
		err = errors.Join(err, domains[i].close(am))
	}
	return
}
