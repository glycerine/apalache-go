package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:217-219,231,297,307.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func forwardAssertionEnabled() (bool, error) {
	return diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
}

func forwardAssertionFailure(line int32, condition string) error {
	return diagnostic.DebugAssertionFailure(diagnostic.BorrowProcessDebugContext(), "src/tactic/ufbv/ufbv_rewriter.cpp", line, condition)
}
