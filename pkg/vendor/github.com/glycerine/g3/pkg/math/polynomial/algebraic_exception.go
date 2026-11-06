package polynomial

// Source: src/math/polynomial/algebraic_numbers.h:41-44.

import "github.com/glycerine/g3/pkg/util/z3exception"

// AlgebraicException preserves the algebraic exception class and its
// default_exception base used by retained AST callers.
type AlgebraicException struct {
	z3exception.DefaultException
	canceled bool
}

func (e AlgebraicException) Unwrap() error { return e.DefaultException }
func newAlgebraicException(message string) AlgebraicException {
	return AlgebraicException{DefaultException: z3exception.NewDefaultException(message)}
}

// Is keeps the port's shared cancellation classification without replacing the
// source algebraic_exception/default_exception inheritance or message.
func (e AlgebraicException) Is(target error) bool {
	return e.canceled && target == ErrUPolynomialCanceled
}
func newAlgebraicCancellation() AlgebraicException {
	return AlgebraicException{DefaultException: z3exception.NewDefaultException("canceled"), canceled: true}
}
