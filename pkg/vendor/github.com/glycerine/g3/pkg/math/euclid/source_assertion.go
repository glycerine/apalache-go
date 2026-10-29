package euclid

// Source: src/math/euclid/euclidean_solver.cpp retained SASSERT sites.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func euclidSourceAssertion(line int32, condition string, holds bool) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, e := diagnostic.SourceAssertionsEnabled(d)
	if e != nil || !enabled || holds {
		return e
	}
	return diagnostic.DebugAssertionFailure(d, "src/math/euclid/euclidean_solver.cpp", line, condition)
}
