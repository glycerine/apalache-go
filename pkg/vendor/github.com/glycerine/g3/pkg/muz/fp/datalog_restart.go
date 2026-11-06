package fp

// Source: rel_context.cpp:133-227. A restart restores the query's original
// rules/predicates while keeping the partial original relations and completed
// strata saved by the termination program. This path covers finite compiled
// saturation; explanation and other relational backends have separate drivers.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type datalogRestartRound struct {
	timeout     uint32
	restarting  bool
	interrupted bool
	statistics  statistics.Statistics
}

// Compilation is outside the restart timer. Termination must run with that
// timer disabled so it can publish the partial relation registers. Source
// execution_context::reset preserves its counters across these rounds.
func performDatalogRestartRound(execution, termination *rel.InstructionBlock, context *rel.ExecutionContext, round *datalogRestartRound) (complete, terminated bool, resultErr error) {
	if round != nil && round.timeout != 0 {
		if err := context.SetTimeLimit(round.timeout); err != nil {
			return false, false, err
		}
	}
	complete, err := execution.Perform(context)
	err = errors.Join(err, context.ResetTimeLimit())
	if err != nil {
		return complete, false, err
	}
	terminated, err = termination.Perform(context)
	if err != nil {
		return complete, terminated, err
	}
	if err = execution.ProcessAllCosts(); err != nil {
		return complete, terminated, err
	}
	if round != nil {
		err = context.CollectStatistics(&round.statistics)
	}
	return complete, terminated, err
}

func (e *PropositionalEngine) datalogInitialRestartTimeout() (timeout uint32, resultErr error) {
	if e.params == nil || e.globals == nil {
		return 0, nil
	}
	view, err := e.fixedpointParamView()
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, view.Close()) }()
	return view.UInt("datalog.initial_restart_timeout", 0)
}

func nextDatalogRestartTimeout(current, initial uint32) uint32 {
	next := uint64(current) * uint64(initial)
	if next > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(next)
}

func (e *PropositionalEngine) queryFiniteDatalogWithRestarts(declarations []ast.ASTHandle) (lbool.LBool, error) {
	initial, err := e.datalogInitialRestartTimeout()
	if err != nil {
		return lbool.Undef, err
	}
	round := datalogRestartRound{timeout: initial}
	conversionBase := len(e.datalogConversions)
	for {
		status, err := e.queryFiniteDatalogRound(declarations, &round)
		if err != nil || !round.interrupted {
			return status, err
		}
		// Per-round deferred owners have restored the source rule/predicate
		// scope. Its model conversions must also retire before rerunning the
		// transforms; a refused release stays attached to the engine for retry.
		if err = closeHornModelConversions(e.datalogConversions[conversionBase:]); err != nil {
			return lbool.Undef, err
		}
		e.datalogConversions = e.datalogConversions[:conversionBase]
		round.timeout = nextDatalogRestartTimeout(round.timeout, initial)
		round.restarting = true
		round.interrupted = false
	}
}
