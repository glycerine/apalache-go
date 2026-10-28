package apalache

import (
	"context"
	"fmt"
	"sort"
)

// EncodedTransition is the symbolic result of rewriting one transition.
// Trigger must hold when the transition is selected, and Binding contains
// the primed values produced by its assignments.
type EncodedTransition struct {
	Trigger ArenaCell
	Binding Binding
}

// TransitionExecutor incrementally builds a bounded symbolic execution.
// Rewriting is solver-free; Check encodes the accumulated arena, definitions,
// and selected path constraints into a fresh g3 solver context.
type TransitionExecutor struct {
	constants         map[string]struct{}
	variables         map[string]struct{}
	rewriter          *StratifiedRewriter
	scope             RewriterScope
	asserted          []Expr
	prepared          map[int]EncodedTransition
	states            []Binding
	step              int
	picked            bool
	solverUIntOptions map[string]uint32
	solverTextOptions map[string]string
}

func NewTransitionExecutor(constants, variables []string) *TransitionExecutor {
	return newTransitionExecutorWithSolverOptions(constants, variables, nil, nil, false)
}

func newTransitionExecutorWithSolverOptions(
	constants, variables []string, solverUIntOptions map[string]uint32, solverTextOptions map[string]string,
	shortCircuit bool,
) *TransitionExecutor {
	return &TransitionExecutor{
		constants:         stringSet(constants),
		variables:         stringSet(variables),
		rewriter:          NewStratifiedRewriterWithOptions(StratifiedRewriterOptions{ShortCircuit: shortCircuit}),
		scope:             InitialRewriterScope(),
		prepared:          make(map[int]EncodedTransition),
		solverUIntOptions: solverUIntOptions,
		solverTextOptions: solverTextOptions,
	}
}

func stringSet(names []string) map[string]struct{} {
	result := make(map[string]struct{}, len(names))
	for _, name := range names {
		result[name] = struct{}{}
	}
	return result
}

func (executor *TransitionExecutor) Step() int { return executor.step }

func (executor *TransitionExecutor) Binding() Binding { return executor.scope.Binding }

func (executor *TransitionExecutor) StateBindings() []Binding {
	return append([]Binding(nil), executor.states...)
}

func (executor *TransitionExecutor) PreparedTransitionNumbers() []int {
	result := make([]int, 0, len(executor.prepared))
	for number := range executor.prepared {
		result = append(result, number)
	}
	sort.Ints(result)
	return result
}

// InitializeConstants applies a normalized constant initializer. It may only
// be called before any state transition has been prepared.
func (executor *TransitionExecutor) InitializeConstants(expression Expr) error {
	if executor.step != 0 || len(executor.prepared) != 0 || executor.picked {
		return fmt.Errorf("constants can only be initialized before the initial transition")
	}
	if err := requireBooleanExpression(expression, "constant initializer"); err != nil {
		return err
	}
	scope, trigger, err := executor.rewriter.Rewrite(expression, executor.scope)
	if err != nil {
		return fmt.Errorf("rewrite constant initializer: %w", err)
	}
	shifted := scope.Binding.Shift(map[string]struct{}{})
	if missing := missingNames(executor.constants, shifted); len(missing) != 0 {
		return fmt.Errorf("constants are not initialized: %v", missing)
	}
	if extras := unexpectedNames(executor.constants, shifted); len(extras) != 0 {
		return fmt.Errorf("constant initializer assigned non-constants: %v", extras)
	}
	executor.scope = RewriterScope{Arena: scope.Arena, Binding: shifted}
	executor.asserted = append(executor.asserted, trigger.NameExpr())
	return nil
}

// PrepareTransition rewrites one normalized transition against the current
// state. It returns false for a transition that does not assign every state
// variable, matching the disabled-partial-transition behavior in Scala.
func (executor *TransitionExecutor) PrepareTransition(number int, expression Expr) (bool, error) {
	if executor.picked {
		return false, fmt.Errorf("advance to the next state before preparing another transition")
	}
	if _, exists := executor.prepared[number]; exists {
		return false, fmt.Errorf("transition %d is already prepared", number)
	}
	if err := requireBooleanExpression(expression, fmt.Sprintf("transition %d", number)); err != nil {
		return false, err
	}
	base := RewriterScope{Arena: executor.scope.Arena, Binding: executor.scope.Binding.ForgetPrimed()}
	scope, trigger, err := executor.rewriter.Rewrite(expression, base)
	if err != nil {
		return false, fmt.Errorf("rewrite transition %d: %w", number, err)
	}
	// Keep arena growth from every alternative but do not leak its primed
	// assignments into the next alternative.
	executor.scope = RewriterScope{Arena: scope.Arena, Binding: base.Binding}
	assigned := make(map[string]struct{})
	for name := range scope.Binding.Map() {
		if len(name) > 0 && name[len(name)-1] == '\'' {
			assigned[name[:len(name)-1]] = struct{}{}
		}
	}
	if len(missingNameSet(executor.variables, assigned)) != 0 ||
		len(unexpectedNameSet(executor.variables, assigned)) != 0 {
		return false, nil
	}
	executor.prepared[number] = EncodedTransition{Trigger: trigger, Binding: scope.Binding}
	return true, nil
}

func missingNames(expected map[string]struct{}, actual Binding) []string {
	missing := make([]string, 0)
	for name := range expected {
		if !actual.Contains(name) {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

func missingNameSet(expected, actual map[string]struct{}) []string {
	missing := make([]string, 0)
	for name := range expected {
		if _, found := actual[name]; !found {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

func unexpectedNames(expected map[string]struct{}, actual Binding) []string {
	actualSet := make(map[string]struct{})
	for name := range actual.Map() {
		actualSet[name] = struct{}{}
	}
	return unexpectedNameSet(expected, actualSet)
}

func unexpectedNameSet(expected, actual map[string]struct{}) []string {
	unexpected := make([]string, 0)
	for name := range actual {
		if _, found := expected[name]; !found {
			unexpected = append(unexpected, name)
		}
	}
	sort.Strings(unexpected)
	return unexpected
}

// AssumeTransition selects a prepared branch and adds its trigger to the path.
func (executor *TransitionExecutor) AssumeTransition(number int) error {
	if executor.picked {
		return fmt.Errorf("a transition has already been selected at step %d", executor.step)
	}
	transition, found := executor.prepared[number]
	if !found {
		return fmt.Errorf("transition %d has not been prepared", number)
	}
	executor.asserted = append(executor.asserted, transition.Trigger.NameExpr())
	executor.scope.Binding = transition.Binding
	executor.picked = true
	return nil
}

// AssertState rewrites and asserts a Boolean state or action expression.
func (executor *TransitionExecutor) AssertState(expression Expr) error {
	if err := requireBooleanExpression(expression, "state assertion"); err != nil {
		return err
	}
	scope, assertion, err := executor.rewriter.Rewrite(expression, executor.scope)
	if err != nil {
		return fmt.Errorf("rewrite state assertion: %w", err)
	}
	executor.scope = scope
	executor.asserted = append(executor.asserted, assertion.NameExpr())
	return nil
}

// AssertNextState rewrites a state predicate against the selected transition's
// primed binding while retaining the current action binding.
func (executor *TransitionExecutor) AssertNextState(expression Expr) error {
	if !executor.picked {
		return fmt.Errorf("select a transition before asserting its target state")
	}
	if err := requireBooleanExpression(expression, "next-state assertion"); err != nil {
		return err
	}
	target := RewriterScope{Arena: executor.scope.Arena, Binding: executor.scope.Binding.Shift(executor.constants)}
	target, assertion, err := executor.rewriter.Rewrite(expression, target)
	if err != nil {
		return fmt.Errorf("rewrite next-state assertion: %w", err)
	}
	executor.scope.Arena = target.Arena
	executor.asserted = append(executor.asserted, assertion.NameExpr())
	return nil
}

func requireBooleanExpression(expression Expr, description string) error {
	if expression == nil {
		return fmt.Errorf("%s is nil", description)
	}
	typ, typed := expression.Tag().Type()
	if !typed || !EqualTypes(typ, BoolT1) {
		return fmt.Errorf("%s must have Boolean type", description)
	}
	return nil
}

// NextState commits the selected primed binding as the next unprimed state.
func (executor *TransitionExecutor) NextState() error {
	if !executor.picked {
		return fmt.Errorf("select a transition before advancing from step %d", executor.step)
	}
	executor.scope.Binding = executor.scope.Binding.Shift(executor.constants)
	executor.states = append(executor.states, executor.scope.Binding)
	executor.prepared = make(map[int]EncodedTransition)
	executor.picked = false
	executor.step++
	return nil
}

// Check tests the complete accumulated symbolic path for satisfiability.
func (executor *TransitionExecutor) Check(ctx context.Context) (SMTStatus, error) {
	return executor.check(ctx, nil)
}

// CheckState tests whether expression can hold on the current path without
// permanently adding it as a path constraint. Definitions introduced while
// rewriting the expression remain reusable in subsequent checks.
func (executor *TransitionExecutor) CheckState(ctx context.Context, expression Expr) (SMTStatus, error) {
	if err := requireBooleanExpression(expression, "state expression"); err != nil {
		return SMTUnknown, err
	}
	scope, assertion, err := executor.rewriter.Rewrite(expression, executor.scope)
	if err != nil {
		return SMTUnknown, fmt.Errorf("rewrite state expression: %w", err)
	}
	executor.scope = scope
	return executor.check(ctx, []Expr{assertion.NameExpr()})
}

// Evaluate rewrites and decodes an expression in one model of the current
// symbolic path. Rewriter definitions are retained for subsequent queries.
func (executor *TransitionExecutor) Evaluate(ctx context.Context, expression Expr) (Expr, SMTStatus, error) {
	if expression == nil {
		return nil, SMTUnknown, fmt.Errorf("evaluation expression is nil")
	}
	scope, cell, err := executor.rewriter.Rewrite(expression, executor.scope)
	if err != nil {
		return nil, SMTUnknown, fmt.Errorf("rewrite evaluation expression: %w", err)
	}
	executor.scope = scope
	solver, encoder, status, err := executor.solve(ctx, nil)
	if solver != nil {
		defer solver.Close()
	}
	if err != nil || status != SMTSatisfiable {
		return nil, status, err
	}
	decoder, err := NewSymbolicDecoder(encoder, executor.rewriter)
	if err != nil {
		return nil, SMTUnknown, err
	}
	value, err := decoder.DecodeCell(executor.scope.Arena, cell)
	if err != nil {
		return nil, SMTUnknown, fmt.Errorf("decode evaluation expression: %w", err)
	}
	return value, status, nil
}

func (executor *TransitionExecutor) check(ctx context.Context, temporary []Expr) (SMTStatus, error) {
	solver, _, status, err := executor.solve(ctx, temporary)
	if solver != nil {
		_ = solver.Close()
	}
	return status, err
}

// CheckAndDecode tests a state expression and, when satisfiable, decodes every
// committed state using the same constrained model.
func (executor *TransitionExecutor) CheckAndDecode(ctx context.Context, expression Expr) (SMTStatus, []DecodedState, error) {
	return executor.checkAndDecode(ctx, expression, executor.states)
}

func (executor *TransitionExecutor) checkAndDecodeDistinctState(
	ctx context.Context, expression, view Expr, excluded [][]Expr,
) (SMTStatus, []DecodedState, []Expr, error) {
	return executor.checkAndDecodeDistinctView(ctx, expression, executor.states, view, excluded)
}

// CheckTraceAndDecode instantiates a one-argument trace predicate with the
// sequence of records represented by the committed symbolic state bindings.
func (executor *TransitionExecutor) CheckTraceAndDecode(
	ctx context.Context,
	parameter string,
	expression Expr,
	historyType SeqType,
	variables []*VarDecl,
) (SMTStatus, []DecodedState, error) {
	if len(executor.states) == 0 {
		return SMTUnknown, nil, fmt.Errorf("cannot check a trace predicate before the initial state")
	}
	instantiated, err := instantiateTraceExpression(parameter, expression, historyType, variables, executor.states)
	if err != nil {
		return SMTUnknown, nil, err
	}
	return executor.checkAndDecode(ctx, instantiated, executor.states)
}

func instantiateTraceExpression(
	parameter string, expression Expr, historyType SeqType, variables []*VarDecl, bindings []Binding,
) (Expr, error) {
	ordered := append([]*VarDecl(nil), variables...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].Name() < ordered[right].Name() })
	records := make([]Expr, len(bindings))
	for stateIndex, binding := range bindings {
		arguments := make([]Expr, 0, len(ordered)*2)
		for _, variable := range ordered {
			cell, found := binding.Get(variable.Name())
			if !found {
				return nil, fmt.Errorf("state %d has no binding for %s", stateIndex, variable.Name())
			}
			arguments = append(arguments,
				NewValueExpr(StringValue(variable.Name()), Typed(StrT1)), cell.NameExpr())
		}
		records[stateIndex] = MustOperExpr(Op("RECORD"), Typed(historyType.Element()), arguments...)
	}
	history := MustOperExpr(Op("TUPLE"), Typed(historyType), records...)
	return replaceNames(expression, map[string]Expr{parameter: history}, nil), nil
}

func (executor *TransitionExecutor) checkTraceAndDecodeDistinctView(
	ctx context.Context, parameter string, expression Expr, historyType SeqType, variables []*VarDecl,
	view Expr, excluded [][]Expr,
) (SMTStatus, []DecodedState, []Expr, error) {
	instantiated, err := instantiateTraceExpression(parameter, expression, historyType, variables, executor.states)
	if err != nil {
		return SMTUnknown, nil, nil, err
	}
	return executor.checkAndDecodeDistinctView(ctx, instantiated, executor.states, view, excluded)
}

// CheckPendingAndDecode evaluates an action expression before NextState and
// decodes the selected transition's target as the final state in the trace.
func (executor *TransitionExecutor) CheckPendingAndDecode(ctx context.Context, expression Expr) (SMTStatus, []DecodedState, error) {
	if !executor.picked {
		return SMTUnknown, nil, fmt.Errorf("select a transition before decoding a pending action")
	}
	bindings := append([]Binding(nil), executor.states...)
	bindings = append(bindings, executor.scope.Binding.Shift(executor.constants))
	return executor.checkAndDecode(ctx, expression, bindings)
}

func (executor *TransitionExecutor) checkPendingAndDecodeDistinctView(
	ctx context.Context, expression, view Expr, excluded [][]Expr,
) (SMTStatus, []DecodedState, []Expr, error) {
	if !executor.picked {
		return SMTUnknown, nil, nil, fmt.Errorf("select a transition before decoding a pending action")
	}
	bindings := append([]Binding(nil), executor.states...)
	bindings = append(bindings, executor.scope.Binding.Shift(executor.constants))
	return executor.checkAndDecodeDistinctView(ctx, expression, bindings, view, excluded)
}

func (executor *TransitionExecutor) checkAndDecode(
	ctx context.Context,
	expression Expr,
	bindings []Binding,
) (SMTStatus, []DecodedState, error) {
	if err := requireBooleanExpression(expression, "state expression"); err != nil {
		return SMTUnknown, nil, err
	}
	scope, assertion, err := executor.rewriter.Rewrite(expression, executor.scope)
	if err != nil {
		return SMTUnknown, nil, fmt.Errorf("rewrite state expression: %w", err)
	}
	executor.scope = scope
	solver, encoder, status, err := executor.solve(ctx, []Expr{assertion.NameExpr()})
	if solver != nil {
		defer solver.Close()
	}
	if err != nil || status != SMTSatisfiable {
		return status, nil, err
	}
	decoder, err := NewSymbolicDecoder(encoder, executor.rewriter)
	if err != nil {
		return SMTUnknown, nil, err
	}
	states := make([]DecodedState, len(bindings))
	for index, binding := range bindings {
		values, decodeErr := decoder.DecodeBinding(executor.scope.Arena, binding)
		if decodeErr != nil {
			return SMTUnknown, nil, fmt.Errorf("decode state %d: %w", index, decodeErr)
		}
		states[index] = DecodedState{Values: values}
	}
	return status, states, nil
}

func (executor *TransitionExecutor) checkAndDecodeDistinctView(
	ctx context.Context,
	expression Expr,
	bindings []Binding,
	view Expr,
	excluded [][]Expr,
) (SMTStatus, []DecodedState, []Expr, error) {
	if view == nil {
		status, states, err := executor.checkAndDecode(ctx, expression, bindings)
		return status, states, nil, err
	}
	if err := requireBooleanExpression(expression, "state expression"); err != nil {
		return SMTUnknown, nil, nil, err
	}
	scope, assertion, err := executor.rewriter.Rewrite(expression, executor.scope)
	if err != nil {
		return SMTUnknown, nil, nil, fmt.Errorf("rewrite state expression: %w", err)
	}
	executor.scope = scope
	viewCells := make([]ArenaCell, len(bindings))
	for index, binding := range bindings {
		viewScope, cell, rewriteErr := executor.rewriter.Rewrite(view,
			RewriterScope{Arena: executor.scope.Arena, Binding: binding})
		if rewriteErr != nil {
			return SMTUnknown, nil, nil, fmt.Errorf("rewrite state view at depth %d: %w", index, rewriteErr)
		}
		executor.scope.Arena = viewScope.Arena
		viewCells[index] = cell
	}
	temporary := []Expr{assertion.NameExpr()}
	for _, prior := range excluded {
		limit := len(prior)
		if limit > len(viewCells) {
			continue
		}
		differences := make([]Expr, limit)
		for index := 0; index < limit; index++ {
			equal := MustOperExpr(Op("EQ"), Typed(BoolT1), viewCells[index].NameExpr(), prior[index])
			differences[index] = MustOperExpr(Op("NOT"), Typed(BoolT1), equal)
		}
		if len(differences) == 1 {
			temporary = append(temporary, differences[0])
		} else if len(differences) > 1 {
			temporary = append(temporary, MustOperExpr(Op("OR"), Typed(BoolT1), differences...))
		}
	}
	solver, encoder, status, err := executor.solve(ctx, temporary)
	if solver != nil {
		defer solver.Close()
	}
	if err != nil || status != SMTSatisfiable {
		return status, nil, nil, err
	}
	decoder, err := NewSymbolicDecoder(encoder, executor.rewriter)
	if err != nil {
		return SMTUnknown, nil, nil, err
	}
	states := make([]DecodedState, len(bindings))
	views := make([]Expr, len(viewCells))
	for index, binding := range bindings {
		values, decodeErr := decoder.DecodeBinding(executor.scope.Arena, binding)
		if decodeErr != nil {
			return SMTUnknown, nil, nil, fmt.Errorf("decode state %d: %w", index, decodeErr)
		}
		states[index] = DecodedState{Values: values}
		views[index], decodeErr = decoder.DecodeCell(executor.scope.Arena, viewCells[index])
		if decodeErr != nil {
			return SMTUnknown, nil, nil, fmt.Errorf("decode state view %d: %w", index, decodeErr)
		}
	}
	return status, states, views, nil
}

type DecodedState struct {
	Values map[string]Expr
}

func (executor *TransitionExecutor) solve(ctx context.Context, temporary []Expr) (*G3SolverContext, *G3GroundEncoder, SMTStatus, error) {
	solver, err := newG3SolverContextWithOptions("QF_AUFNIA", executor.solverUIntOptions, executor.solverTextOptions)
	if err != nil {
		return nil, nil, SMTUnknown, err
	}
	encoder, err := EncodeRewriterScope(solver, executor.scope, executor.rewriter.Constraints())
	if err != nil {
		_ = solver.Close()
		return nil, nil, SMTUnknown, err
	}
	if err = encoder.AssertAll(executor.asserted); err != nil {
		_ = solver.Close()
		return nil, nil, SMTUnknown, fmt.Errorf("assert symbolic path: %w", err)
	}
	if err = encoder.AssertAll(temporary); err != nil {
		_ = solver.Close()
		return nil, nil, SMTUnknown, fmt.Errorf("assert temporary state expression: %w", err)
	}
	status, err := solver.Check(ctx)
	if err != nil {
		_ = solver.Close()
		return nil, nil, status, err
	}
	return solver, encoder, status, nil
}
