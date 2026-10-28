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

type EncodedState struct {
	Binding          Binding
	TransitionNumber int
}

type EncodedExecution struct {
	Arena *PureArena
	Path  []EncodedState
}

type DecodedExecution struct {
	Path []DecodedState
}

type ExecutorControlState uint8

const (
	ExecutorPreparing ExecutorControlState = iota
	ExecutorPicked
)

// TransitionExecutorSnapshot captures a complete rollback point. PureArena
// and Binding are immutable values; mutable slices, maps, and rewriter caches
// are copied so later executor work cannot mutate the snapshot.
type TransitionExecutorSnapshot struct {
	ControlState        ExecutorControlState
	Scope               RewriterScope
	Assertions          []Expr
	PreparedTransitions map[int]EncodedTransition
	States              []Binding
	TransitionNumbers   []int
	TransitionOracles   []ChoiceOracle
	SelectedTransition  int
	SelectedOracle      ChoiceOracle
	Step                int
	Rewriter            StratifiedRewriterSnapshot
}

// TransitionExecutor incrementally builds a bounded symbolic execution.
// Rewriting is solver-free; Check encodes the accumulated arena, definitions,
// and selected path constraints into a fresh g3 solver context.
type TransitionExecutor struct {
	constants          map[string]struct{}
	variables          map[string]struct{}
	rewriter           *StratifiedRewriter
	scope              RewriterScope
	asserted           []Expr
	prepared           map[int]EncodedTransition
	states             []Binding
	transitionNumbers  []int
	transitionOracles  []ChoiceOracle
	selectedTransition int
	selectedOracle     ChoiceOracle
	step               int
	picked             bool
	solverUIntOptions  map[string]uint32
	solverTextOptions  map[string]string
	smtEncoding        SMTEncoding
}

func NewTransitionExecutor(constants, variables []string) *TransitionExecutor {
	return newTransitionExecutorWithSolverOptions(constants, variables, nil, nil, false)
}

func newTransitionExecutorWithSolverOptions(
	constants, variables []string, solverUIntOptions map[string]uint32, solverTextOptions map[string]string,
	shortCircuit bool,
) *TransitionExecutor {
	return newTransitionExecutorWithEncodingAndSolverOptions(
		constants, variables, solverUIntOptions, solverTextOptions, shortCircuit, SMTEncodingOOPSLA19)
}

func newTransitionExecutorWithEncodingAndSolverOptions(
	constants, variables []string, solverUIntOptions map[string]uint32, solverTextOptions map[string]string,
	shortCircuit bool, smtEncoding SMTEncoding,
) *TransitionExecutor {
	return &TransitionExecutor{
		constants: stringSet(constants),
		variables: stringSet(variables),
		rewriter: NewStratifiedRewriterWithOptions(StratifiedRewriterOptions{
			ShortCircuit: shortCircuit, SMTEncoding: smtEncoding,
		}),
		scope:             InitialRewriterScope(),
		prepared:          make(map[int]EncodedTransition),
		solverUIntOptions: solverUIntOptions,
		solverTextOptions: solverTextOptions,
		smtEncoding:       smtEncoding,
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

func (executor *TransitionExecutor) StateTransitionNumbers() []int {
	return append([]int(nil), executor.transitionNumbers...)
}

func (executor *TransitionExecutor) EncodedExecution() EncodedExecution {
	path := make([]EncodedState, len(executor.states))
	for index, binding := range executor.states {
		path[index] = EncodedState{Binding: binding, TransitionNumber: executor.transitionNumbers[index]}
	}
	return EncodedExecution{Arena: executor.scope.Arena, Path: path}
}

func (executor *TransitionExecutor) PreparedTransitionNumbers() []int {
	result := make([]int, 0, len(executor.prepared))
	for number := range executor.prepared {
		result = append(result, number)
	}
	sort.Ints(result)
	return result
}

// MayChangeAssertion implements the syntactic transition filter used by the
// Scala executor. At the initial step every assertion is reconsidered. Later,
// an assertion may change exactly when it mentions a variable whose primed
// binding is absent from, or differs from, its unprimed binding.
func (executor *TransitionExecutor) MayChangeAssertion(
	transitionNumber int, _ InvariantKind, _ int, assertion Expr,
) (bool, error) {
	transition, found := executor.prepared[transitionNumber]
	if !found {
		return false, fmt.Errorf("transition %d has not been prepared", transitionNumber)
	}
	if assertion == nil {
		return false, fmt.Errorf("assertion cannot be nil")
	}
	used, err := FindUsedNames(assertion)
	if err != nil {
		return false, fmt.Errorf("find names used by assertion: %w", err)
	}
	if executor.step == 0 {
		return true, nil
	}
	changed := make(map[string]struct{})
	for name, primedCell := range transition.Binding.Map() {
		if len(name) == 0 || name[len(name)-1] != '\'' {
			continue
		}
		unprimed := name[:len(name)-1]
		currentCell, currentFound := transition.Binding.Get(unprimed)
		if !currentFound || !currentCell.Equal(primedCell) {
			changed[name] = struct{}{}
		}
	}
	for name := range used {
		if len(name) == 0 || name[len(name)-1] != '\'' {
			name += "'"
		}
		if _, found := changed[name]; found {
			return true, nil
		}
	}
	return false, nil
}

func (executor *TransitionExecutor) ControlState() ExecutorControlState {
	if executor.picked {
		return ExecutorPicked
	}
	return ExecutorPreparing
}

func cloneEncodedTransitions(source map[int]EncodedTransition) map[int]EncodedTransition {
	result := make(map[int]EncodedTransition, len(source))
	for number, transition := range source {
		result[number] = transition
	}
	return result
}

func (executor *TransitionExecutor) Snapshot() TransitionExecutorSnapshot {
	return TransitionExecutorSnapshot{
		ControlState: executor.ControlState(), Scope: executor.scope,
		Assertions:          append([]Expr(nil), executor.asserted...),
		PreparedTransitions: cloneEncodedTransitions(executor.prepared),
		States:              append([]Binding(nil), executor.states...),
		TransitionNumbers:   append([]int(nil), executor.transitionNumbers...),
		TransitionOracles:   append([]ChoiceOracle(nil), executor.transitionOracles...),
		SelectedTransition:  executor.selectedTransition, Step: executor.step,
		SelectedOracle: executor.selectedOracle,
		Rewriter:       executor.rewriter.Snapshot(),
	}
}

func (executor *TransitionExecutor) Recover(snapshot TransitionExecutorSnapshot) {
	if err := executor.RecoverChecked(snapshot); err != nil {
		panic(err)
	}
}

func (executor *TransitionExecutor) RecoverChecked(snapshot TransitionExecutorSnapshot) error {
	if snapshot.Scope.Arena == nil {
		return fmt.Errorf("transition executor snapshot has no arena")
	}
	if snapshot.ControlState != ExecutorPreparing && snapshot.ControlState != ExecutorPicked {
		return fmt.Errorf("transition executor snapshot has invalid control state %d", snapshot.ControlState)
	}
	if len(snapshot.TransitionNumbers) != len(snapshot.States) {
		return fmt.Errorf("transition executor snapshot has %d states but %d transition numbers",
			len(snapshot.States), len(snapshot.TransitionNumbers))
	}
	if len(snapshot.TransitionOracles) != 0 && len(snapshot.TransitionOracles) != len(snapshot.States) {
		return fmt.Errorf("transition executor snapshot has %d states but %d transition oracles",
			len(snapshot.States), len(snapshot.TransitionOracles))
	}
	executor.scope = snapshot.Scope
	executor.asserted = append([]Expr(nil), snapshot.Assertions...)
	executor.prepared = cloneEncodedTransitions(snapshot.PreparedTransitions)
	executor.states = append([]Binding(nil), snapshot.States...)
	executor.transitionNumbers = append([]int(nil), snapshot.TransitionNumbers...)
	executor.transitionOracles = append([]ChoiceOracle(nil), snapshot.TransitionOracles...)
	executor.selectedTransition = snapshot.SelectedTransition
	executor.selectedOracle = snapshot.SelectedOracle
	executor.step = snapshot.Step
	executor.picked = snapshot.ControlState == ExecutorPicked
	executor.rewriter.Recover(snapshot.Rewriter)
	return nil
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
	executor.selectedTransition = number
	executor.selectedOracle = &MockOracle{Fixed: number}
	executor.picked = true
	return nil
}

// PickTransition symbolically selects one prepared transition. A single
// transition uses a fixed oracle; multiple transitions share one oracle across
// the trigger and every primed variable, preserving their correlation.
func (executor *TransitionExecutor) PickTransition() (ChoiceOracle, error) {
	if executor.picked {
		return nil, fmt.Errorf("a transition has already been selected at step %d", executor.step)
	}
	numbers := executor.PreparedTransitionNumbers()
	if len(numbers) == 0 {
		return nil, fmt.Errorf("unable to pick transitions from an empty set")
	}
	if len(numbers) == 1 {
		if err := executor.AssumeTransition(numbers[0]); err != nil {
			return nil, err
		}
		return executor.selectedOracle, nil
	}

	scope, oracle, constraints, err := NewOracleFactory(executor.rewriter.literals).NewDefault(executor.scope, len(numbers))
	if err != nil {
		return nil, err
	}
	executor.scope = scope
	for _, constraint := range constraints {
		executor.rewriter.assert(constraint)
	}
	triggers := make([]Expr, len(numbers))
	for index, number := range numbers {
		triggers[index] = executor.prepared[number].Trigger.NameExpr()
	}
	enabledChoice, err := oracle.Cases(executor.scope, triggers, nil)
	if err != nil {
		return nil, err
	}
	executor.asserted = append(executor.asserted, enabledChoice)

	first := executor.prepared[numbers[0]].Binding
	primed := make([]string, 0)
	for name := range first.Map() {
		if len(name) > 0 && name[len(name)-1] == '\'' {
			primed = append(primed, name)
		}
	}
	sort.Strings(primed)
	final := make(map[string]ArenaCell, len(primed)+len(executor.constants))
	picker := NewCherryPicker(executor.rewriter)
	for _, name := range primed {
		candidates := make([]ArenaCell, len(numbers))
		for index, number := range numbers {
			cell, found := executor.prepared[number].Binding.Get(name)
			if !found {
				return nil, fmt.Errorf("prepared transition %d has no binding for %s", number, name)
			}
			candidates[index] = cell
		}
		executor.scope, final[name], err = picker.Pick(executor.scope, oracle, candidates)
		if err != nil {
			return nil, fmt.Errorf("pick transition value for %s: %w", name, err)
		}
	}
	for name := range executor.constants {
		if cell, found := executor.scope.Binding.Get(name); found {
			final[name] = cell
		}
	}
	sparse, err := NewSparseOracle(oracle, numbers)
	if err != nil {
		return nil, err
	}
	executor.scope.Binding = NewBinding(final)
	executor.selectedTransition = -1
	executor.selectedOracle = sparse
	executor.picked = true
	return sparse, nil
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
	executor.transitionNumbers = append(executor.transitionNumbers, executor.selectedTransition)
	executor.transitionOracles = append(executor.transitionOracles, executor.selectedOracle)
	executor.prepared = make(map[int]EncodedTransition)
	executor.picked = false
	executor.selectedTransition = 0
	executor.selectedOracle = nil
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

func (executor *TransitionExecutor) CheckDecodedExecution(
	ctx context.Context, expression Expr,
) (SMTStatus, DecodedExecution, error) {
	status, states, err := executor.CheckAndDecode(ctx, expression)
	return status, DecodedExecution{Path: states}, err
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
		transitionNumber, decodeErr := executor.decodedTransitionNumber(encoder, index, len(bindings))
		if decodeErr != nil {
			return SMTUnknown, nil, fmt.Errorf("decode transition at state %d: %w", index, decodeErr)
		}
		states[index] = DecodedState{Values: values, TransitionNumber: transitionNumber}
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
		transitionNumber, decodeErr := executor.decodedTransitionNumber(encoder, index, len(bindings))
		if decodeErr != nil {
			return SMTUnknown, nil, nil, fmt.Errorf("decode transition at state %d: %w", index, decodeErr)
		}
		states[index] = DecodedState{Values: values, TransitionNumber: transitionNumber}
		views[index], decodeErr = decoder.DecodeCell(executor.scope.Arena, viewCells[index])
		if decodeErr != nil {
			return SMTUnknown, nil, nil, fmt.Errorf("decode state view %d: %w", index, decodeErr)
		}
	}
	return status, states, views, nil
}

type DecodedState struct {
	Values           map[string]Expr
	TransitionNumber int
}

func (executor *TransitionExecutor) decodedTransitionNumber(
	encoder *G3GroundEncoder, index, stateCount int,
) (int, error) {
	var oracle ChoiceOracle
	if index < len(executor.transitionOracles) {
		oracle = executor.transitionOracles[index]
	} else if executor.picked && stateCount == len(executor.transitionNumbers)+1 && index == len(executor.transitionNumbers) {
		oracle = executor.selectedOracle
	}
	if decoder, ok := oracle.(interface {
		Decode(*G3GroundEncoder) (int, error)
	}); ok {
		return decoder.Decode(encoder)
	}
	if index < len(executor.transitionNumbers) {
		return executor.transitionNumbers[index], nil
	}
	if executor.picked && index == len(executor.transitionNumbers) {
		return executor.selectedTransition, nil
	}
	return 0, nil
}

func (executor *TransitionExecutor) solve(ctx context.Context, temporary []Expr) (*G3SolverContext, *G3GroundEncoder, SMTStatus, error) {
	solver, err := newG3SolverContextWithOptions("QF_AUFNIA", executor.solverUIntOptions, executor.solverTextOptions)
	if err != nil {
		return nil, nil, SMTUnknown, err
	}
	encoder, err := EncodeRewriterScopeWithEncoding(
		solver, executor.scope, executor.rewriter.Constraints(), executor.smtEncoding)
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
