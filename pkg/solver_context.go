package apalache

import (
	"context"
	"fmt"
	"io"
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
	SMTLib2() (string, error)
	Log(string)
	Logs() []string
	Push() error
	Pop(...int) error
	Dispose() error
}

// SolverContext exposes the arena-cell/TLA-expression solver boundary used by
// rewriting and exploration while delegating SMT terms to pure-Go g3.
type SolverContext struct {
	config          SolverConfig
	solver          *G3SolverContext
	encoder         *G3GroundEncoder
	level           int
	metrics         SolverContextMetrics
	listener        SMTListener
	logs            []string
	memberships     map[[2]int]struct{}
	membershipStack []map[[2]int]struct{}
	maxCellID       int
	maxCellStack    []int
}

func CreateSolverContext(config SolverConfig) (*SolverContext, error) {
	solver, err := NewG3SolverContextWithConfig(config)
	if err != nil {
		return nil, err
	}
	encoder, err := NewG3GroundEncoderWithEncoding(solver, config.SMTEncoding)
	if err != nil {
		_ = solver.Close()
		return nil, err
	}
	return &SolverContext{
		config: config, solver: solver, encoder: encoder, listener: IdleSMTListener{},
		memberships: make(map[[2]int]struct{}), maxCellID: -1,
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
	if cell.ID <= solver.maxCellID {
		return fmt.Errorf("declaring cell %d while cell %d has already been declared: damaged arena", cell.ID, solver.maxCellID)
	}
	if err := solver.encoder.DeclareCell(cell); err != nil {
		return err
	}
	solver.maxCellID = cell.ID
	solver.metrics = solver.metrics.AddCells(1).AddConstants(1)
	solver.listener.OnIntroduceCell(int64(cell.ID))
	solver.listener.OnIntroduceSMTConstant(cell.String())
	if (cell.ID == 0 || cell.ID == 1) && EqualTypes(cell.Type.TLAType(), BoolT1) {
		truth := cell.ID == 1
		constraint := MustOperExpr(Op("EQ"), Typed(BoolT1), cell.NameExpr(), boolExpr(truth))
		if err := solver.encoder.Assert(constraint); err != nil {
			return fmt.Errorf("constrain canonical Boolean cell %s: %w", cell, err)
		}
		solver.metrics = solver.metrics.AddSMTExpressions(1)
	}
	return nil
}

func (solver *SolverContext) DeclareMembership(set, element ArenaCell) error {
	if solver.config.SMTEncoding == SMTEncodingArrays {
		return nil
	}
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

func (solver *SolverContext) WriteSMTLib2(out io.Writer) error {
	if solver == nil || solver.solver == nil {
		return fmt.Errorf("solver context is not initialized")
	}
	return solver.solver.WriteSMTLib2(out)
}

func (solver *SolverContext) SMTLib2() (string, error) {
	if solver == nil || solver.solver == nil {
		return "", fmt.Errorf("solver context is not initialized")
	}
	return solver.solver.SMTLib2()
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
	solver.encoder.Push()
	solver.membershipStack = append(solver.membershipStack, cloneMap(solver.memberships))
	solver.maxCellStack = append(solver.maxCellStack, solver.maxCellID)
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
	if err := solver.encoder.Pop(n); err != nil {
		return err
	}
	if n > 0 {
		solver.memberships = solver.membershipStack[len(solver.membershipStack)-n]
		solver.membershipStack = solver.membershipStack[:len(solver.membershipStack)-n]
		solver.maxCellID = solver.maxCellStack[len(solver.maxCellStack)-n]
		solver.maxCellStack = solver.maxCellStack[:len(solver.maxCellStack)-n]
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
