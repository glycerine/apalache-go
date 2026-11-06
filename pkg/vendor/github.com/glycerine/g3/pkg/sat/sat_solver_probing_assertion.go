package sat

// Source: src/sat/sat_probing.cpp:61-62,112-113,199; src/util/debug.h.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func satProbingSourceAssertion(ok bool, line int32, expression string) error {
	if ok {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	if !d.HasAssertionActions() {
		return ErrSATAssertion
	}
	return diagnostic.DebugAssertionFailure(d, "src/sat/sat_probing.cpp", line, expression)
}

func (s *satSolverProbing) assertQHead(line int32) error {
	if !satStorageSourceDebug {
		return nil
	}
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	v := s.propagation().variables
	n, err := v.trail.Size(v.trailRef)
	if err != nil {
		return err
	}
	return satProbingSourceAssertion(v.qhead == n, line, "s.m_qhead == s.m_trail.size()")
}

func (s *satSolverProbing) assertUndef(l Literal, line int32, expression string) error {
	if !satStorageSourceDebug {
		return nil
	}
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled {
		return err
	}
	a, err := s.propagation().variables.valueVar(l.variable())
	if err != nil {
		return err
	}
	return satProbingSourceAssertion(a == lbool.Undef, line, expression)
}

func (s *satSolverProbing) assertReady(l Literal, qheadLine int32, undefExpression string) error {
	if err := s.assertQHead(qheadLine); err != nil {
		return err
	}
	return s.assertUndef(l, qheadLine+1, undefExpression)
}
