package nlsat

// Source: src/nlsat/nlsat_evaluator.cpp:387,425,467,605. Each numerical call
// constructs a real polynomial_ref temporary for the full call expression.

import "github.com/glycerine/g3/pkg/math/polynomial"

func (e *Evaluator) beginPolynomial(p polynomial.Polynomial) error {
	if e.callRef != nil {
		return polynomial.ErrPolynomialPending
	}
	var err error
	e.callRef, err = polynomial.NewPolynomialRef(e.core)
	if err != nil {
		return err
	}
	return e.callRef.Assign(p)
}
func (e *Evaluator) finishPolynomial(err *error) {
	// A stopped younger numerical scope protects this accepted temporary.
	// Explicit evaluator Cleanup retires it after the callee is acknowledged.
	if *err != nil {
		return
	}
	*err = e.callRef.Close()
	if *err == nil {
		e.callRef = nil
	}
}
func (e *Evaluator) isolateRoots(p polynomial.Polynomial, x uint32, roots *polynomial.AlgebraicVector) (err error) {
	if err = e.beginPolynomial(p); err != nil {
		return err
	}
	defer e.finishPolynomial(&err)
	return e.am.IsolateMultivariateRoots(e.pm, p, UndefVarAssignment{Assignment: e.assignment, Variable: x}, roots)
}
func (e *Evaluator) isolateRootsWithSigns(p polynomial.Polynomial, x uint32) (err error) {
	if err = e.beginPolynomial(p); err != nil {
		return err
	}
	defer e.finishPolynomial(&err)
	return e.am.IsolateMultivariateRootsWithSigns(e.pm, p, UndefVarAssignment{Assignment: e.assignment, Variable: x}, e.addRoots, &e.addSigns)
}
