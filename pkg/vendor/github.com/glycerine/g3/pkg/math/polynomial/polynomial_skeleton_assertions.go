package polynomial

// Source: src/math/polynomial/polynomial.cpp:3020,3049,3051;
// src/util/vector.h:309; src/util/scoped_ptr_vector.h:33.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func polynomialSkeletonSourceAssert(file string, line int32, expression string, predicate func() (bool, error)) error {
	if !polynomialSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled {
		return e
	}
	ok, e := predicate()
	if e != nil || ok {
		return e
	}
	if e = diagnostic.DebugAssertionFailure(d, file, line, expression); e != nil {
		return errors.Join(ErrPolynomialAssertion, e)
	}
	return nil
}

func polynomialSkeletonScalarAssert(file string, line int32, expression string, condition bool) error {
	return polynomialSkeletonSourceAssert(file, line, expression, func() (bool, error) { return condition, nil })
}

func polynomialSkeletonVectorIndex(i uint32, n int) error {
	if e := polynomialSkeletonScalarAssert("src/util/vector.h", 309, "idx < size()", uint64(i) < uint64(n)); e != nil {
		return e
	}
	return skeletonNativeIndex(i, n)
}
