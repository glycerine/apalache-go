package apalache

import (
	"context"
	"fmt"
)

// TraceEvaluation contains the synthetic output module and the value of each
// requested nullary operator in every input state.
type TraceEvaluation struct {
	Module *Module
	States []TraceState
}

// EvaluateTrace evaluates named nullary operators from module over concrete
// trace states. The module must be typechecked and the states must bind every
// user variable referenced by the requested operators.
func EvaluateTrace(ctx context.Context, module *Module, states []TraceState, names []string) (TraceEvaluation, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if module == nil {
		return TraceEvaluation{}, fmt.Errorf("cannot evaluate a trace without a module")
	}
	if !module.HasProperty(PropertyTypeChecked) {
		return TraceEvaluation{}, fmt.Errorf("trace evaluation requires a typechecked module")
	}
	if len(states) == 0 {
		return TraceEvaluation{}, fmt.Errorf("cannot evaluate an empty trace")
	}
	if len(names) == 0 {
		return TraceEvaluation{}, fmt.Errorf("cannot evaluate an empty expression list")
	}

	expressions, outputModule, err := prepareTraceExpressions(module, names)
	if err != nil {
		return TraceEvaluation{}, err
	}
	result := make([]TraceState, len(states))
	for index, state := range states {
		if err := ctx.Err(); err != nil {
			return TraceEvaluation{}, err
		}
		values, evalErr := evaluateTraceState(ctx, state, expressions)
		if evalErr != nil {
			return TraceEvaluation{}, fmt.Errorf("state %d: %w", index, evalErr)
		}
		result[index] = values
	}
	return TraceEvaluation{Module: outputModule, States: result}, nil
}

func prepareTraceExpressions(module *Module, names []string) (map[string]Expr, *Module, error) {
	requested := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			return nil, nil, fmt.Errorf("trace expression name cannot be empty")
		}
		if _, duplicate := requested[name]; duplicate {
			return nil, nil, fmt.Errorf("trace expression %s is repeated", name)
		}
		requested[name] = struct{}{}
		declaration, found := declarationNamed(module, name)
		operator, ok := declaration.(*OperDecl)
		if !found || !ok {
			return nil, nil, fmt.Errorf("operator %s is undefined in module %s", name, module.Name())
		}
		if len(operator.FormalParams()) != 0 {
			return nil, nil, fmt.Errorf("trace expression %s requires %d arguments", name, len(operator.FormalParams()))
		}
	}

	tracker := TransformationTracker(IdleTracker{})
	namesGenerator := NewUniqueNameGenerator()
	renamer := NewIncrementalRenamer(tracker)
	desugared, err := DesugarModule(module, namesGenerator, tracker)
	if err != nil {
		return nil, nil, fmt.Errorf("desugar trace expressions: %w", err)
	}
	inlined, err := NewInliner(tracker, renamer, true, KeepOnlyInlinedDeclarations(requested)).Module(desugared)
	if err != nil {
		return nil, nil, fmt.Errorf("inline trace expressions: %w", err)
	}
	inlined.AddProperty(PropertyInlined)
	prepared, err := PreprocessModule(inlined, PreprocessOptions{Names: namesGenerator, Renamer: renamer, Tracker: tracker})
	if err != nil {
		return nil, nil, fmt.Errorf("preprocess trace expressions: %w", err)
	}
	prepared, err = OptimizeModule(prepared, PreprocessOptions{Names: namesGenerator, Renamer: renamer, Tracker: tracker})
	if err != nil {
		return nil, nil, fmt.Errorf("optimize trace expressions: %w", err)
	}
	prepared, err = MarkSkolemizableModule(prepared, tracker)
	if err != nil {
		return nil, nil, fmt.Errorf("analyze trace expressions: %w", err)
	}
	prepared, err = MarkSetExpansionsModule(prepared, tracker)
	if err != nil {
		return nil, nil, fmt.Errorf("analyze trace expansions: %w", err)
	}

	expressions := make(map[string]Expr, len(names))
	declarations := make([]Decl, 0, len(names))
	for _, name := range names {
		body, found := OperatorBody(prepared, name)
		if !found {
			return nil, nil, fmt.Errorf("prepared trace expression %s is missing", name)
		}
		typ, typed := body.Tag().Type()
		if !typed {
			return nil, nil, fmt.Errorf("trace expression %s is untyped", name)
		}
		expressions[name] = body
		declarations = append(declarations, NewVarDecl(name, Typed(typ)))
	}
	return expressions, NewModule(module.Name()+"TraceEvaluation", declarations), nil
}

func evaluateTraceState(ctx context.Context, state TraceState, expressions map[string]Expr) (TraceState, error) {
	rewriter := NewStratifiedRewriter()
	scope := InitialRewriterScope()
	for _, name := range sortedStringKeys(state) {
		value := state[name]
		var cell ArenaCell
		var err error
		scope, cell, err = rewriter.Rewrite(value, scope)
		if err != nil {
			return nil, fmt.Errorf("bind %s: %w", name, err)
		}
		scope.Binding = scope.Binding.With(name, cell)
	}
	cells := make(map[string]ArenaCell, len(expressions))
	for _, name := range sortedStringKeys(expressions) {
		expression := expressions[name]
		var err error
		scope, cells[name], err = rewriter.Rewrite(expression, scope)
		if err != nil {
			return nil, fmt.Errorf("evaluate %s: %w", name, err)
		}
	}
	solver, err := NewG3SolverContext()
	if err != nil {
		return nil, err
	}
	defer solver.Close()
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		return nil, err
	}
	status, err := solver.Check(ctx)
	if err != nil {
		return nil, err
	}
	if status != SMTSatisfiable {
		return nil, fmt.Errorf("expression evaluation returned %v", status)
	}
	decoder, err := NewSymbolicDecoder(encoder, rewriter)
	if err != nil {
		return nil, err
	}
	result := make(TraceState, len(cells))
	for _, name := range sortedStringKeys(cells) {
		cell := cells[name]
		result[name], err = decoder.DecodeCell(scope.Arena, cell)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", name, err)
		}
	}
	return result, nil
}
