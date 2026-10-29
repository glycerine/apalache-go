package sat

// Source: src/sat/sat_solver.cpp:2773-2796,2859-3017;
// src/sat/tactic/sat_tactic.cpp:61-66,186-189,200-206.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func CollectSolverParamDescrs(d *uparams.ParamDescrs) error { return collectSolverParamDescrs(d) }
func (s *Solver) CollectStatistics(st *statistics.Statistics) error {
	keys := satSolverStatisticsKeys{
		main:       [15]statistics.Key{"mk bool var", "mk binary clause", "mk ternary clause", "mk clause", "gc clause", "del clause", "conflicts", "propagations", "decisions", "binary propagations", "ternary propagations", "restarts", "minimized lits", "dyn subsumption resolution", "blocked correction sets"},
		cleaner:    [2]statistics.Key{"elim clauses", "elim literals"},
		simplifier: [5]statistics.Key{"subsumed", "subsumption resolution", "elim literals", "elim bool vars", "elim blocked clauses"},
		scc:        "elim bool vars", asymm: "elim literals", probing: "probing assigned",
	}
	return s.collectStatistics(st, keys)
}
func (s *Solver) DisplayStatus(out *utext.NumericTextStream) error { return s.displayStatus(out) }
func (s *Solver) DisplayDIMACS(out *utext.NumericTextStream) error { return s.displayDIMACS(out) }
func (s *Solver) BindDIMACS(out *utext.NumericTextStream) error    { return s.bindDIMACS(out) }

// StoppedCleanupCause reports the immutable, already observed source
// destructor guard. Ordinary Close never suppresses or acknowledges it.
func (s *Solver) StoppedCleanupCause() error { return s.closeSourceCause }

func (s *Solver) DIMACSDisplayEnabled() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	state, err := s.config.State()
	return state.DimacsDisplay, err
}

func (s *Solver) AssertTacticInvariant() error {
	if err := s.open(); err != nil {
		return err
	}
	if !satStorageSourceDebug {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := d.ConditionalAssertionEnabled("sat_solver")
	if err != nil || !enabled {
		return err
	}
	return s.Lifecycle.checkInvariantWithActions(true)
}
