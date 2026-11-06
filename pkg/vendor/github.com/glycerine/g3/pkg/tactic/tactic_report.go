package tactic

// Source: src/tactic/tactic.cpp:26-61; src/tactic/tactic.h:108.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/stopwatch"
	"github.com/glycerine/g3/pkg/util/text"
)

const tacticVerbosityLevel = 10

// TacticReport borrows its goal and process services until Close. Source
// construction erases the whole body when the process verbosity is below 10.
type TacticReport struct {
	goal        *Goal
	runtime     *memory.Runtime
	watch       stopwatch.Stopwatch
	id          string
	startMemory float64
	closed      bool
}

func NewTacticReport(id string, goal *Goal) (*TacticReport, error) {
	return newTacticReport(id, goal, nil)
}

// ReportTacticProgress emits positive counters at tactic verbosity level.
// The source suppresses zero values before checking the verbose stream.
func ReportTacticProgress(id string, value uint32) error {
	if value == 0 {
		return nil
	}
	enabled, err := diagnostic.ProcessVerboseEnabled(tacticVerbosityLevel)
	if err != nil || !enabled {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err := stream.AppendCString("("); err != nil {
		return err
	}
	if err := stream.AppendCString(id); err != nil {
		return err
	}
	if err := stream.AppendCString(" "); err != nil {
		return err
	}
	if err := stream.AppendUint32(value); err != nil {
		return err
	}
	if err := stream.AppendCString(")\n"); err != nil {
		return err
	}
	return stream.Flush()
}

func newTacticReport(id string, goal *Goal, clock func() (uint64, error)) (*TacticReport, error) {
	enabled, err := diagnostic.ProcessVerboseEnabled(tacticVerbosityLevel)
	if err != nil || !enabled {
		return nil, err
	}
	if goal == nil {
		return nil, ast.ErrASTHandle
	}
	if err := goal.open(); err != nil {
		return nil, err
	}
	numerals := goal.manager.NumeralService()
	if numerals == nil || numerals.RuntimeOwner() == nil {
		return nil, ast.ErrASTHandle
	}
	runtime := numerals.RuntimeOwner()
	bytes, err := runtime.AllocationSize()
	if err != nil {
		return nil, err
	}
	r := &TacticReport{goal: goal, runtime: runtime, watch: stopwatch.New(clock), id: id, startMemory: float64(bytes) / float64(1024*1024)}
	if err := r.watch.Start(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *TacticReport) Close() error {
	if r == nil || r.closed {
		return nil
	}
	if err := r.watch.Stop(); err != nil {
		return err
	}
	r.closed = true
	endBytes, err := r.runtime.AllocationSize()
	if err != nil {
		return err
	}
	count, err := r.goal.NumExprs()
	if err != nil {
		return err
	}
	seconds, err := r.watch.Seconds()
	if err != nil {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	for _, part := range []string{"(", r.id, " :num-exprs "} {
		if err := stream.AppendCString(part); err != nil {
			return err
		}
	}
	if err := stream.AppendUint32(count); err != nil {
		return err
	}
	if err := stream.AppendCString(" :num-asts "); err != nil {
		return err
	}
	if err := stream.AppendUint32(r.goal.manager.Stats().LiveNodes); err != nil {
		return err
	}
	if err := stream.AppendCString(" :time "); err != nil {
		return err
	}
	if err := stream.SetFloatMode(text.NumericFixedFloat); err != nil {
		return err
	}
	if err := stream.SetPrecision(2); err != nil {
		return err
	}
	if err := stream.AppendFloat64(seconds); err != nil {
		return err
	}
	if err := stream.AppendCString(" :before-memory "); err != nil {
		return err
	}
	if err := stream.AppendFloat64(r.startMemory); err != nil {
		return err
	}
	if err := stream.AppendCString(" :after-memory "); err != nil {
		return err
	}
	if err := stream.AppendFloat64(float64(endBytes) / float64(1024*1024)); err != nil {
		return err
	}
	if err := stream.AppendCString(")\n"); err != nil {
		return err
	}
	return stream.Flush()
}
