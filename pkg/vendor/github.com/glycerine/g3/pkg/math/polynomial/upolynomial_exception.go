package polynomial

// Source: upolynomial.h:40-43; upolynomial.cpp:159;
// upolynomial_factorization.cpp:39,44.

import "github.com/glycerine/g3/pkg/util/z3exception"

// UPolynomialException preserves the source default_exception base. The Go
// sentinel classification stays available to existing errors.Is consumers.
type UPolynomialException struct {
	z3exception.DefaultException
	canceled, domain bool
}

func (e UPolynomialException) Unwrap() error { return e.DefaultException }
func (e UPolynomialException) Is(target error) bool {
	return (e.canceled && target == ErrUPolynomialCanceled) || (e.domain && target == ErrUPolynomialDomain)
}

// Immutable value boxed once: cancellation stays allocation-free in warm
// arithmetic paths. errors.As returns a copy, not a mutable shared pointer.
var upolynomialCancellationError error = UPolynomialException{
	DefaultException: z3exception.NewDefaultException("canceled"), canceled: true,
}

func newUPolynomialPrimeException() UPolynomialException {
	return UPolynomialException{DefaultException: z3exception.NewDefaultException("The prime number attempted in factorization is too big!"), domain: true}
}
