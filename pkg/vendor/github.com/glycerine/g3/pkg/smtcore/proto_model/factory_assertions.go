package protomodel

// Source: SASSERT sites in src/smt/proto_model/value_factory.h/.cpp.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func simpleFactoryAssert(line int32, condition string, evaluate func() bool) error {
	return valueFactoryAssert("src/smt/proto_model/value_factory.h", line, condition, evaluate)
}

func valueFactoryAssert(file string, line int32, condition string, evaluate func() bool) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil {
		return err
	}
	if enabled && !evaluate() {
		return diagnostic.DebugAssertionFailure(d, file, line, condition)
	}
	return nil
}
