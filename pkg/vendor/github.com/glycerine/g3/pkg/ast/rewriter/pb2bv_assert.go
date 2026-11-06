package rewriter

import "github.com/glycerine/g3/pkg/util/diagnostic"

func pb2BVSourceAssert(line int32, condition string, evaluate func() bool) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		return err
	}
	if evaluate() {
		return nil
	}
	return diagnostic.DebugAssertionFailure(d, "src/ast/rewriter/pb2bv_rewriter.cpp", line, condition)
}
