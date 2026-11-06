package smtcore

// Source: src/smt/smt_statistics.h:26-54; src/smt/smt_statistics.cpp:24-26;
// src/smt/smt_context.h:70.

// SMTContextStatistics is the actual first declared context member. It is a
// pointer-free inline value; no separate owner, allocator ticket or mirrored
// producer counters are introduced. Real context consumers update these exact
// unsigned fields when their source action is eventually ported.
type SMTContextStatistics struct {
	NumPropagations    uint32
	NumBinPropagations uint32
	NumConflicts       uint32
	NumSatConflicts    uint32
	NumDecisions       uint32
	NumAddEq           uint32
	NumRestarts        uint32
	NumFinalChecks     uint32
	NumMkBoolVar       uint32
	NumDelBoolVar      uint32
	NumMkEnode         uint32
	NumDelEnode        uint32
	NumMkClause        uint32
	NumDelClause       uint32
	NumMkBinClause     uint32
	NumMkLits          uint32
	NumDynAck          uint32
	NumDelDynAck       uint32
	NumInterfaceEqs    uint32
	MaxGeneration      uint32
	NumMinimizedLits   uint32
	NumChecks          uint32
}

func NewSMTContextStatistics() SMTContextStatistics {
	var s SMTContextStatistics
	s.Reset()
	return s
}

// Source memset zeroes the entire scalar object, including every trailing
// member. Ordinary value assignment performs that exact zero publication.
func (s *SMTContextStatistics) Reset() { *s = SMTContextStatistics{} }
