package apalache

import "context"

// RecordingSolverContext records active solver operations as a differential
// log while delegating execution to the configured production context.
type RecordingSolverContext struct {
	parent *SMTLog
	config SolverConfig
	solver *SolverContext
	levels [][]SMTLogRecord
}

func NewRecordingSolverContext(parent *SMTLog, config SolverConfig) (*RecordingSolverContext, error) {
	solver, err := CreateSolverContext(config)
	if err != nil {
		return nil, err
	}
	if err = parent.Replay(solver); err != nil {
		_ = solver.Dispose()
		return nil, err
	}
	return &RecordingSolverContext{
		parent: parent, config: config, solver: solver, levels: [][]SMTLogRecord{nil},
	}, nil
}

func (context *RecordingSolverContext) Config() SolverConfig   { return context.config }
func (context *RecordingSolverContext) Solver() *SolverContext { return context.solver }
func (context *RecordingSolverContext) Level() int             { return context.solver.Level() }
func (context *RecordingSolverContext) Metrics() SolverContextMetrics {
	return context.solver.Metrics()
}
func (context *RecordingSolverContext) SetSMTListener(listener SMTListener) {
	context.solver.SetSMTListener(listener)
}

func (context *RecordingSolverContext) append(record SMTLogRecord) {
	level := len(context.levels) - 1
	context.levels[level] = append(context.levels[level], record)
}

func (context *RecordingSolverContext) DeclareCell(cell ArenaCell) error {
	if err := context.solver.DeclareCell(cell); err != nil {
		return err
	}
	context.append(DeclareCellRecord{Cell: cell})
	return nil
}

func (context *RecordingSolverContext) DeclareMembership(set, element ArenaCell) error {
	before := context.solver.Metrics().NConstants
	if err := context.solver.DeclareMembership(set, element); err != nil {
		return err
	}
	if context.solver.Metrics().NConstants != before {
		context.append(DeclareMembershipRecord{Set: set, Element: element})
	}
	return nil
}

func (context *RecordingSolverContext) Assert(expression Expr) error {
	if err := context.solver.Assert(expression); err != nil {
		return err
	}
	context.append(AssertGroundExpressionRecord{Expression: expression})
	return nil
}

func (context *RecordingSolverContext) Evaluate(expression Expr) (Expr, error) {
	return context.solver.Evaluate(expression)
}

func (context *RecordingSolverContext) Check(ctx context.Context) (SMTStatus, error) {
	return context.solver.Check(ctx)
}

func (context *RecordingSolverContext) CheckWithTimeout(ctx context.Context, seconds int) (SMTStatus, error) {
	return context.solver.CheckWithTimeout(ctx, seconds)
}

func (context *RecordingSolverContext) CheckConsistency(arena *PureArena) error {
	return context.solver.CheckConsistency(arena)
}

func (context *RecordingSolverContext) Log(message string) { context.solver.Log(message) }
func (context *RecordingSolverContext) Logs() []string     { return context.solver.Logs() }

func (context *RecordingSolverContext) Push() error {
	if err := context.solver.Push(); err != nil {
		return err
	}
	context.levels = append(context.levels, nil)
	return nil
}

func (context *RecordingSolverContext) Pop(count ...int) error {
	n := 1
	if len(count) > 1 {
		return context.solver.Pop(count...)
	}
	if len(count) == 1 {
		n = count[0]
	}
	if err := context.solver.Pop(n); err != nil {
		return err
	}
	context.levels = context.levels[:len(context.levels)-n]
	return nil
}

func (context *RecordingSolverContext) ExtractLog() *SMTLog {
	count := 0
	for _, level := range context.levels {
		count += len(level)
	}
	records := make([]SMTLogRecord, 0, count)
	for _, level := range context.levels {
		records = append(records, level...)
	}
	return NewSMTLog(context.parent, records)
}

func (context *RecordingSolverContext) Dispose() error { return context.solver.Dispose() }

var _ SMTLogReplayTarget = (*RecordingSolverContext)(nil)
var _ GroundSolverContext = (*RecordingSolverContext)(nil)
