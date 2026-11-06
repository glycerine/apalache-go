package smtcore

// Source: src/smt/smt_case_split_queue.cpp retained SASSERT gates.

import "github.com/glycerine/g3/pkg/util/diagnostic"

func smtCaseSplitAssert(line int32, condition string, test func() bool) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		return err
	}
	if !test() {
		return diagnostic.DebugAssertionFailure(d, "src/smt/smt_case_split_queue.cpp", line, condition)
	}
	return nil
}

func (s *SMTInternalizerState) assertCaseSplitAdmissionVar(variable int32) error {
	var line int32
	switch s.caseSplitStrategy {
	case 3:
		line = 343
	case 4:
		line = 542
	case 5:
		line = 775
	default:
		return nil
	}
	return smtCaseSplitAssert(line, "var != null_bool_var", func() bool { return variable != SMTNullBoolVar })
}

func (s *SMTInternalizerState) assertCaseSplitAdmissionOr(isOr bool) error {
	var line int32
	switch s.caseSplitStrategy {
	case 3:
		line = 354
	case 5:
		line = 786
	default:
		return nil
	}
	return smtCaseSplitAssert(line, "is_or", func() bool { return isOr })
}

func (s *SMTInternalizerState) assertCaseSplitPopCount(count uint32) error {
	var line int32
	switch s.caseSplitStrategy {
	case 3:
		line = 391
	case 4:
		line = 585
	case 5:
		line = 831
	default:
		return nil
	}
	return smtCaseSplitAssert(line, "num_scopes <= m_scopes.size()", func() bool { return uint64(count) <= uint64(len(s.scopes)) })
}

func (q *SMTActivityCaseSplitQueue) candidateAssertionLines() (int32, int32, int32) {
	switch q.strategy {
	case 4:
		return 603, 610, 625
	case 5:
		return 867, 874, 892
	default:
		return 411, 418, 436
	}
}
