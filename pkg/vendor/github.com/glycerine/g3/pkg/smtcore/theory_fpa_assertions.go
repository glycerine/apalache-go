package smtcore

// Source: retained SASSERT sites in src/smt/theory_fpa.cpp.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func fpaSourceAssert(line int32, condition string, evaluate func() (bool, error)) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		return err
	}
	ok, err := evaluate()
	if err != nil || ok {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, "src/smt/theory_fpa.cpp", line, condition)
}

func fpaSourceAssertBool(line int32, condition string, value bool) error {
	return fpaSourceAssert(line, condition, func() (bool, error) { return value, nil })
}
