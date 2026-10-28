package apalache

import "fmt"

// DesugarModule applies the production desugarer to every declaration body.
func DesugarModule(module *Module, names *UniqueNameGenerator, tracker TransformationTracker) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot desugar a nil module")
	}
	if !module.HasProperty(PropertyTypeChecked) {
		return nil, fmt.Errorf("desugaring requires a typechecked module")
	}
	stateVariables := make([]string, len(module.VarDeclarations()))
	for index, declaration := range module.VarDeclarations() {
		stateVariables[index] = declaration.Name()
	}
	operatorDefinitions := make(map[string]Expr)
	for _, declaration := range module.OperDeclarations() {
		if len(declaration.FormalParams()) == 0 {
			operatorDefinitions[declaration.Name()] = declaration.Body()
		}
	}
	desugarer := NewDesugarer(names, stateVariables, tracker, operatorDefinitions)
	result, err := transformModuleExpressionsWithError(module, "desugaring", nil, desugarer.Expression)
	if err != nil {
		return nil, err
	}
	result.AddProperty(PropertyDesugared)
	return result, nil
}

type SafetyPipelineOptions struct {
	Specification SpecificationConfiguration
	Names         *UniqueNameGenerator
	Renamer       *IncrementalRenamer
	Tracker       TransformationTracker
	Sources       *SourceStore
}

type SafetyPipelineResult struct {
	Module     *Module
	Predicates DerivedPredicates
}

// PrepareSafetyTransitions executes the ported Scala checker front half
// through temporal encoding and optimized symbolic transitions. VC generation
// remains a separate stage because the bounded checker evaluates invariants
// directly against each symbolic state or action.
func PrepareSafetyTransitions(module *Module, options SafetyPipelineOptions) (SafetyPipelineResult, error) {
	tracker := options.Tracker
	if tracker == nil {
		tracker = IdleTracker{}
	}
	names := options.Names
	if names == nil {
		names = NewUniqueNameGenerator()
	}
	renamer := options.Renamer
	if renamer == nil {
		renamer = NewIncrementalRenamer(tracker)
	}
	configuration := options.Specification
	configuration.Tracker = tracker
	configured, err := ConfigureModule(module, configuration)
	if err != nil {
		return SafetyPipelineResult{}, fmt.Errorf("configuration: %w", err)
	}
	desugared, err := DesugarModule(configured.Module, names, tracker)
	if err != nil {
		return SafetyPipelineResult{}, err
	}
	inlineOptions := InlinePassOptions{
		Init: configured.Predicates.Init, Next: configured.Predicates.Next,
		ConstantInit: configured.Predicates.ConstantInit, Relevant: configured.Predicates.OperatorNames(),
		Tracker: tracker, Renamer: renamer,
	}
	inlined, err := InlineConfiguredModule(desugared, inlineOptions)
	if err != nil {
		return SafetyPipelineResult{}, fmt.Errorf("first inline pass: %w", err)
	}
	inlined, configured.Predicates, err = EncodeTemporalProperties(
		inlined, configured.Predicates, names, renamer, tracker,
	)
	if err != nil {
		return SafetyPipelineResult{}, fmt.Errorf("temporal encoding: %w", err)
	}
	inlineOptions.Relevant = configured.Predicates.OperatorNames()
	inlined, err = InlineConfiguredModule(inlined, inlineOptions)
	if err != nil {
		return SafetyPipelineResult{}, fmt.Errorf("second inline pass: %w", err)
	}
	primed, err := PrimeInitializers(inlined, PrimingOptions{
		Init: configured.Predicates.Init, ConstantInit: configured.Predicates.ConstantInit, Tracker: tracker,
	})
	if err != nil {
		return SafetyPipelineResult{}, err
	}
	preprocessed, err := PreprocessModule(primed, PreprocessOptions{Names: names, Renamer: renamer, Tracker: tracker, ConstantInitName: configured.Predicates.ConstantInit})
	if err != nil {
		return SafetyPipelineResult{}, err
	}
	persistent := make(map[string]struct{}, len(configured.Predicates.Persistent))
	for _, name := range configured.Predicates.Persistent {
		persistent[name] = struct{}{}
	}
	if configured.Predicates.View != "" {
		persistent[configured.Predicates.View] = struct{}{}
	}
	for _, name := range append(append([]string(nil), configured.Predicates.StateConstraints...), configured.Predicates.ActionConstraints...) {
		persistent[name] = struct{}{}
	}
	transitions, err := FindTransitions(preprocessed, TransitionPassOptions{
		Init: configured.Predicates.Init, Next: configured.Predicates.Next,
		ConstantInit: configured.Predicates.ConstantInit, Invariants: configured.Predicates.Invariants,
		Persistent: persistent, Tracker: tracker, Sources: options.Sources, Renamer: renamer,
	})
	if err != nil {
		return SafetyPipelineResult{}, err
	}
	optimized, err := OptimizeModule(transitions, PreprocessOptions{Names: names, Renamer: renamer, Tracker: tracker})
	if err != nil {
		return SafetyPipelineResult{}, err
	}
	analyzed, err := MarkSkolemizableModule(optimized, tracker)
	if err != nil {
		return SafetyPipelineResult{}, err
	}
	analyzed, err = MarkSetExpansionsModule(analyzed, tracker)
	if err != nil {
		return SafetyPipelineResult{}, err
	}
	return SafetyPipelineResult{Module: analyzed, Predicates: configured.Predicates}, nil
}
