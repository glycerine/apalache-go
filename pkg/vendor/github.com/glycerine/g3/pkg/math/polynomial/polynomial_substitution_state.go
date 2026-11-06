package polynomial

// Source: src/math/polynomial/polynomial.cpp:6237-6425,3185; src/math/polynomial/polynomial.h:942-966.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// This is the actually used cold virtual-map surface. The source manager()
// virtual is not called by substitution; array wrappers likewise do not bind
// a fabricated rational manager to the source UNREACHABLE implementation.

type PolynomialRationalSubstitution interface {
	Contains(uint32) (bool, error)
	Rational(uint32) (*unumeral.Rational, error)
}
type polynomialSubstitutionArray struct {
	positions *PolynomialVariablePositions
	values    []unumeral.Rational
}

// Distinct actual persistent m_var_max_degree, m_var_pos and m_found_vars.
// Embedded cold metadata is included in the reflected core charge. The source
// imp temporary itself is core.tmp1, never a monomial-manager scratch slot.
type polynomialSubstitutionState struct {
	max                     PolynomialMaxDegrees
	positions               PolynomialVariablePositions
	found                   polynomialFoundVariables
	array                   polynomialSubstitutionArray
	scope                   polynomialSubstitutionScope
	orphans                 []Monomial
	orphanDrain             uint32
	generatedReceipt        Monomial
	borrowedReceipt         uint64
	generatedCalleeReceipt  uint64
	bodyCause, cleanupCause error
}
type polynomialSubstitutionScope struct {
	traceInsertion                                   unumeral.NumeralInsertionReceipt
	traceMonomial                                    Monomial
	invocation                                       uint64
	powerNumeric                                     polynomialSubstitutionPowerNumeric
	algebraicCaller                                  *algebraicSignSubstitutionScope
	som                                              polynomialSOMNormalizeInvocationReceipt
	sourceCause, ownCause                            error
	ownStage                                         uint8
	ownStarted, ownResetStopped                      bool
	somInvocation                                    uint64
	active, executing                                bool
	kind                                             uint8
	newA, tmp                                        unumeral.Int
	newAConstructed, tmpConstructed                  bool
	modularPowerPending                              bool
	maxConstructed, posConstructed                   bool
	xs                                               PolynomialVariablesView
	xsCount                                          uint32
	single                                           bool
	singleX                                          uint32
	p1, q1, product                                  polynomialReference
	p1Constructed, q1Constructed, productConstructed bool
	monomial                                         polynomialDivisionReference
	monomialConstructed                              bool
	stage                                            uint8
	resetStopped                                     bool
	returned                                         Polynomial
	bodyCause, cleanupCause                          error
}
