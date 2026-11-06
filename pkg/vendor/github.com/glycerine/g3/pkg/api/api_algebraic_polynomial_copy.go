package api

// C++ numeral pointers may be read by another numerical manager. Go integer
// and polynomial handles carry owner identity, so the live API bridge makes
// an owning copy into the algebraic manager's domain before evaluation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (p *apiAlgebraicPolynomial) copyForAlgebraic(algebraic *polynomial.AlgebraicManager) (manager *polynomial.MultivariatePolynomialManager, value polynomial.Polynomial, ref *polynomial.PolynomialRef, resultErr error) {
	manager, err := algebraic.PolynomialManager()
	if err != nil {
		return nil, value, nil, err
	}
	core, err := manager.ArithmeticCore()
	if err != nil {
		return nil, value, nil, err
	}
	ref, err = polynomial.NewPolynomialRef(core)
	if err != nil {
		return nil, value, nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, manager.CleanupConversion(), ref.Close())
			ref = nil
		}
	}()
	value, err = polynomial.ConvertPolynomial(p.manager, p.value, manager, ^uint32(0), 0)
	if err != nil {
		return manager, value, ref, err
	}
	if err = ref.Assign(value); err != nil {
		return manager, value, ref, err
	}
	return manager, value, ref, nil
}
