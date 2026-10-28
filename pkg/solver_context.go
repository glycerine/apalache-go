package apalache

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type GroundSolverContext interface {
	Config() SolverConfig
	Level() int
	Metrics() SolverContextMetrics
	SetSMTListener(SMTListener)
	DeclareCell(ArenaCell) error
	DeclareMembership(ArenaCell, ArenaCell) error
	Assert(Expr) error
	Evaluate(Expr) (Expr, error)
	Check(context.Context) (SMTStatus, error)
	CheckWithTimeout(context.Context, int) (SMTStatus, error)
	CheckConsistency(*PureArena) error
	Log(string)
	Logs() []string
	Push() error
	Pop(...int) error
	Dispose() error
}

// SolverContext exposes the arena-cell/TLA-expression solver boundary used by
// rewriting and exploration while delegating SMT terms to pure-Go g3.
type SolverContext struct {
	config      SolverConfig
	solver      *G3SolverContext
	encoder     *G3GroundEncoder
	level       int
	metrics     SolverContextMetrics
	listener    SMTListener
	logs        []string
	memberships map[[2]int]struct{}
}

func CreateSolverContext(config SolverConfig) (*SolverContext, error) {
	solver, err := NewG3SolverContextWithConfig(config)
	if err != nil {
		return nil, err
	}
	encoder, err := NewG3GroundEncoder(solver)
	if err != nil {
		_ = solver.Close()
		return nil, err
	}
	return &SolverContext{
		config: config, solver: solver, encoder: encoder, listener: IdleSMTListener{},
		memberships: make(map[[2]int]struct{}),
	}, nil
}

func (solver *SolverContext) Config() SolverConfig          { return solver.config }
func (solver *SolverContext) Encoder() *G3GroundEncoder     { return solver.encoder }
func (solver *SolverContext) Level() int                    { return solver.level }
func (solver *SolverContext) Metrics() SolverContextMetrics { return solver.metrics }

func (solver *SolverContext) SetSMTListener(listener SMTListener) {
	if listener == nil {
		listener = IdleSMTListener{}
	}
	solver.listener = listener
}

func (solver *SolverContext) DeclareCell(cell ArenaCell) error {
	if err := solver.encoder.DeclareCell(cell); err != nil {
		return err
	}
	solver.metrics = solver.metrics.AddCells(1).AddConstants(1)
	solver.listener.OnIntroduceCell(int64(cell.ID))
	solver.listener.OnIntroduceSMTConstant(cell.String())
	return nil
}

func (solver *SolverContext) DeclareMembership(set, element ArenaCell) error {
	key := [2]int{set.ID, element.ID}
	if _, declared := solver.memberships[key]; declared {
		return nil
	}
	if err := solver.encoder.DeclareMembership(set, element); err != nil {
		return err
	}
	solver.memberships[key] = struct{}{}
	solver.metrics = solver.metrics.AddConstants(1)
	solver.listener.OnIntroduceSMTConstant(MembershipPredicateName(set, element))
	return nil
}

func (solver *SolverContext) Assert(expression Expr) error {
	if err := solver.encoder.Assert(expression); err != nil {
		return err
	}
	nodes := int64(groundExpressionSize(expression))
	solver.metrics = solver.metrics.AddSMTExpressions(nodes)
	solver.listener.OnSMTAssert(expression, nodes)
	return nil
}

func groundExpressionSize(expression Expr) int {
	if expression == nil {
		return 0
	}
	size := 1
	switch typed := expression.(type) {
	case *OperExpr:
		for _, argument := range typed.Arguments() {
			size += groundExpressionSize(argument)
		}
	case *LetInExpr:
		size += groundExpressionSize(typed.Body())
		for _, declaration := range typed.Declarations() {
			size += groundExpressionSize(declaration.Body())
		}
	}
	return size
}

func (solver *SolverContext) Evaluate(expression Expr) (Expr, error) {
	typ, typed := expression.Tag().Type()
	if !typed {
		return nil, fmt.Errorf("cannot evaluate an untyped ground expression")
	}
	switch {
	case EqualTypes(typ, BoolT1):
		value, err := solver.encoder.EvaluateBool(expression)
		if err != nil {
			return nil, err
		}
		return NewValueExpr(BoolValue(value), Typed(BoolT1)), nil
	case EqualTypes(typ, IntT1):
		value, err := solver.encoder.EvaluateInt(expression)
		if err != nil {
			return nil, err
		}
		return NewValueExpr(NewIntValue(value), Typed(IntT1)), nil
	default:
		return nil, fmt.Errorf("ground evaluation supports Bool and Int, found %s", typ)
	}
}

func (solver *SolverContext) Check(ctx context.Context) (SMTStatus, error) {
	return solver.solver.Check(ctx)
}

func (solver *SolverContext) CheckWithTimeout(ctx context.Context, timeoutSeconds int) (SMTStatus, error) {
	if timeoutSeconds > 0 {
		ctx = WithSMTQueryTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	}
	return solver.Check(ctx)
}

func (solver *SolverContext) CheckConsistency(arena *PureArena) error {
	if arena == nil {
		return fmt.Errorf("solver consistency check requires an arena")
	}
	declared := solver.encoder.DeclaredCellNames()
	actual := make([]string, 0, len(arena.CellMap()))
	for name := range arena.CellMap() {
		actual = append(actual, name)
	}
	sort.Strings(actual)
	if len(declared) != len(actual) {
		return fmt.Errorf("solver has %d declared cells, arena has %d", len(declared), len(actual))
	}
	for index := range actual {
		if declared[index] != actual[index] {
			return fmt.Errorf("solver cell %s does not match arena cell %s", declared[index], actual[index])
		}
	}
	return nil
}

func (solver *SolverContext) Log(message string) {
	if solver.config.Debug {
		solver.logs = append(solver.logs, message)
	}
}

func (solver *SolverContext) Logs() []string {
	return append([]string(nil), solver.logs...)
}

func (solver *SolverContext) Push() error {
	if err := solver.solver.Push(); err != nil {
		return err
	}
	solver.level++
	return nil
}

func (solver *SolverContext) Pop(count ...int) error {
	n := 1
	if len(count) > 1 {
		return fmt.Errorf("solver pop accepts at most one count")
	}
	if len(count) == 1 {
		n = count[0]
	}
	if n < 0 || n > solver.level {
		return fmt.Errorf("cannot pop %d levels from solver context at level %d", n, solver.level)
	}
	for range n {
		if err := solver.solver.Pop(); err != nil {
			return err
		}
	}
	solver.level -= n
	return nil
}

func (solver *SolverContext) Dispose() error {
	if solver == nil || solver.solver == nil {
		return nil
	}
	solver.level = 0
	return solver.solver.Close()
}

var _ SMTLogReplayTarget = (*SolverContext)(nil)
var _ GroundSolverContext = (*SolverContext)(nil)
