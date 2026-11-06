package tactic

// Source: src/math/subpaving/tactic/expr2subpaving.cpp:173-178.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// A leading multiplication coefficient is read directly by process_mul.
// It is not a recursive process visit and must not charge the AST resource
// limit or cooperate as a separate expression.
func copyExprCoefficient(qm *numeral.RationalManager, text string, n, d *numeral.Int) (err error) {
	coefficient := numeral.NewRational()
	defer func() { err = errors.Join(err, qm.Del(&coefficient)) }()
	if err = qm.SetString(&coefficient, text); err != nil {
		return err
	}
	if err = qm.Numerator(&coefficient, n); err != nil {
		return err
	}
	return qm.Denominator(&coefficient, d)
}
