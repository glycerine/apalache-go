package sat

// Source: src/sat/sat_solver.cpp:3753-3768; src/sat/sat_types.h:289-293.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// The source mem_stat reads the process allocation owner before emitting text.
// This specialization binds that read to the solver's actual shared Runtime.
// Global verbose dispatch and external stream flushing remain separate bindings.

func (s *Solver) displayMemoryStat(out *utext.NumericTextStream) error {
	if e := s.open(); e != nil {
		return e
	}
	return appendSATMemoryStat(out, s.runtime.memory)
}

// The source mem_stat can be emitted while a solver operation is unwinding.
// Its process allocation owner remains available even when solver.open refuses
// interrupted operations, so report destructors use this concrete formatter.
func appendSATMemoryStat(out *utext.NumericTextStream, memory *umemory.Runtime) error {
	bytes, e := memory.AllocationSize()
	if e != nil {
		return e
	}
	mem := float64(bytes) / float64(1024*1024)
	if e = out.AppendCString(" :memory "); e != nil {
		return e
	}
	if e = out.SetFloatMode(utext.NumericFixedFloat); e != nil {
		return e
	}
	if e = out.SetPrecision(2); e != nil {
		return e
	}
	return out.AppendFloat64(mem)
}
func (s *Solver) displayCompactStat(out *utext.NumericTextStream) error {
	if e := s.open(); e != nil {
		return e
	}
	return s.appendCompactStat(out)
}

// Source report destructors read the already constructed solver's counters
// while a failed owner operation may still hold an interrupted continuation.
func (s *Solver) appendCompactStat(out *utext.NumericTextStream) error {
	if len(s.Lifecycle.Original) != 0 {
		if e := out.AppendCString(" :clauses "); e != nil {
			return e
		}
		if e := out.AppendUint32(uint32(len(s.Lifecycle.Original))); e != nil {
			return e
		}
	}
	if len(s.Lifecycle.learned) != 0 {
		if e := out.AppendCString(" :learned "); e != nil {
			return e
		}
		if e := out.AppendUint32(uint32(len(s.Lifecycle.learned)) - s.gc.numFrozen); e != nil {
			return e
		}
		if s.gc.numFrozen > 0 {
			if e := out.AppendCString(" :frozen "); e != nil {
				return e
			}
			if e := out.AppendUint32(s.gc.numFrozen); e != nil {
				return e
			}
		}
	}
	if e := out.AppendCString(" :gc-clause "); e != nil {
		return e
	}
	if e := out.AppendUint32(s.gc.gcClauseStat); e != nil {
		return e
	}
	return appendSATMemoryStat(out, s.runtime.memory)
}
