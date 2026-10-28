package apalache

import "fmt"

type TraceePassOptions struct {
	Predicates  DerivedPredicates
	Trace       TraceeStateSeq
	Expressions []string
	Tracker     TransformationTracker
	WriteOutput func(passName string, module *Module) error
}

// TraceeEncodingPass replaces the configured behavior with transitions that
// evaluate selected expressions along a concrete input trace.
type TraceeEncodingPass struct {
	options    TraceePassOptions
	predicates DerivedPredicates
}

func NewTraceeEncodingPass(options TraceePassOptions) *TraceeEncodingPass {
	trace := make(TraceeStateSeq, len(options.Trace))
	for index, state := range options.Trace {
		trace[index] = cloneTraceState(state)
	}
	options.Trace = trace
	seenExpressions := make(map[string]struct{}, len(options.Expressions))
	expressions := make([]string, 0, len(options.Expressions))
	for _, name := range options.Expressions {
		if _, duplicate := seenExpressions[name]; !duplicate {
			seenExpressions[name] = struct{}{}
			expressions = append(expressions, name)
		}
	}
	options.Expressions = expressions
	options.Predicates = cloneDerivedPredicates(options.Predicates)
	return &TraceeEncodingPass{options: options, predicates: options.Predicates}
}

func cloneDerivedPredicates(predicates DerivedPredicates) DerivedPredicates {
	predicates.Invariants = append([]string(nil), predicates.Invariants...)
	predicates.StateConstraints = append([]string(nil), predicates.StateConstraints...)
	predicates.ActionConstraints = append([]string(nil), predicates.ActionConstraints...)
	predicates.TemporalProperties = append([]string(nil), predicates.TemporalProperties...)
	predicates.Persistent = append([]string(nil), predicates.Persistent...)
	return predicates
}

func (*TraceeEncodingPass) Name() string { return "TraceePass" }
func (*TraceeEncodingPass) Dependencies() []ModuleProperty {
	return []ModuleProperty{PropertyConfigured}
}
func (*TraceeEncodingPass) Transformations() []ModuleProperty { return nil }

func (pass *TraceeEncodingPass) Predicates() DerivedPredicates {
	return cloneDerivedPredicates(pass.predicates)
}

func (pass *TraceeEncodingPass) Execute(module *Module) (*Module, error) {
	if pass == nil {
		return nil, fmt.Errorf("cannot execute a nil trace-evaluation pass")
	}
	if module == nil {
		return nil, fmt.Errorf("trace evaluation requires a module")
	}
	if !module.HasProperty(PropertyConfigured) {
		return nil, fmt.Errorf("trace evaluation requires a configured module")
	}
	if len(pass.options.Trace) == 0 {
		failure, err := NewPassFailure(pass.Name(), "The provided trace is empty.", ExitCodeError)
		if err != nil {
			return nil, err
		}
		return nil, failure
	}
	if len(pass.options.Expressions) == 0 {
		failure, err := NewPassFailure(pass.Name(), "The provided list of expressions is empty.", ExitCodeError)
		if err != nil {
			return nil, err
		}
		return nil, failure
	}
	tracker := pass.options.Tracker
	if tracker == nil {
		tracker = IdleTracker{}
	}
	expressions := make(map[string]Expr, len(pass.options.Expressions))
	for _, name := range pass.options.Expressions {
		body, found := OperatorBody(module, name)
		if !found {
			return nil, &ConfigurationError{Message: fmt.Sprintf("Operator %s undefined in module %s.", name, module.Name())}
		}
		expressions[name] = body
	}

	var constantInit *OperDecl
	if name := pass.options.Predicates.ConstantInit; name != "" {
		body, found := OperatorBody(module, name)
		if !found {
			return nil, &ConfigurationError{Message: fmt.Sprintf("Operator %s undefined in module %s.", name, module.Name())}
		}
		introduction := NewAssignmentOperatorIntroduction(func(UID) bool { return true }, tracker)
		constantInit = ExpressionToOperatorDefinition(NormalizedConstInit, introduction.Expression(body))
	}

	variables := make([]Decl, 0, len(pass.options.Expressions))
	for _, name := range pass.options.Expressions {
		variables = append(variables, NewVarDecl(name, expressions[name].Tag()))
	}
	constructor := NewDrivenTransitionConstructor(tracker, expressions)
	transitions := make([]Expr, len(pass.options.Trace))
	for index, state := range pass.options.Trace {
		transition, err := constructor.MakeTransition(state)
		if err != nil {
			return nil, fmt.Errorf("construct trace transition %d: %w", index, err)
		}
		transitions[index] = transition
	}
	initDeclaration := ExpressionToOperatorDefinition(NormalizedInitPrefix, transitions[0])
	nextDeclarations := ExpressionsToOperatorDefinitions(NormalizedNextPrefix, transitions[1:])

	dropped := make(map[string]struct{})
	for _, name := range []string{pass.options.Predicates.Init, pass.options.Predicates.Next,
		pass.options.Predicates.ConstantInit} {
		if name != "" {
			dropped[name] = struct{}{}
		}
	}
	for _, name := range pass.options.Predicates.Invariants {
		dropped[name] = struct{}{}
	}
	for _, name := range pass.options.Expressions {
		dropped[name] = struct{}{}
	}

	declarations := variables
	for _, declaration := range module.ConstDeclarations() {
		declarations = append(declarations, declaration)
	}
	for _, declaration := range module.AssumeDeclarations() {
		declarations = append(declarations, declaration)
	}
	for _, declaration := range module.OperDeclarations() {
		if _, remove := dropped[declaration.Name()]; !remove {
			declarations = append(declarations, declaration)
		}
	}
	declarations = append(declarations, initDeclaration)
	for _, declaration := range nextDeclarations {
		declarations = append(declarations, declaration)
	}
	if constantInit != nil {
		declarations = append(declarations, constantInit)
	}
	result := NewModule(module.Name(), declarations)
	result.copyMetadataFrom(module)

	pass.predicates = pass.options.Predicates
	pass.predicates.Persistent = append([]string(nil), pass.options.Predicates.Persistent...)
	pass.predicates.Persistent = append(pass.predicates.Persistent, initDeclaration.Name())
	for _, declaration := range nextDeclarations {
		pass.predicates.Persistent = append(pass.predicates.Persistent, declaration.Name())
	}
	if pass.options.WriteOutput != nil {
		if err := pass.options.WriteOutput(pass.Name(), result); err != nil {
			return nil, fmt.Errorf("write trace-evaluation output: %w", err)
		}
	}
	return result, nil
}

var _ ModulePass = (*TraceeEncodingPass)(nil)
