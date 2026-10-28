package apalache

import "fmt"

// PreprocessOptions supplies the state shared by the production preprocessing
// sequence. A zero value creates deterministic default generators and trackers.
type PreprocessOptions struct {
	Names            *UniqueNameGenerator
	Renamer          *IncrementalRenamer
	Tracker          TransformationTracker
	ConstantInitName string
}

// PreprocessModule runs the same ordered transformations as Scala's
// PreproPassImpl. The input is expected to have completed inlining.
func PreprocessModule(module *Module, options PreprocessOptions) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot preprocess a nil module")
	}
	if !module.HasProperty(PropertyInlined) {
		return nil, fmt.Errorf("preprocessing requires an inlined module")
	}
	tracker, names, renamer := preprocessingState(options)
	stateVariables := make(map[string]struct{}, len(module.VarDeclarations()))
	variableNames := make([]string, 0, len(module.VarDeclarations()))
	for _, declaration := range module.VarDeclarations() {
		stateVariables[declaration.Name()] = struct{}{}
		variableNames = append(variableNames, declaration.Name())
	}
	constantInitName := options.ConstantInitName
	if constantInitName == "" {
		constantInitName = "CInit"
	}
	withoutPrimedConstantInit := func(declaration Decl) bool {
		operator, ok := declaration.(*OperDecl)
		return !ok || operator.Name() != constantInitName+"Primed"
	}

	current, err := transformModuleExpressionsWithError(module, "prime propagation", withoutPrimedConstantInit,
		func(expression Expr) (Expr, error) { return PropagatePrimes(tracker, stateVariables)(expression), nil })
	if err != nil {
		return nil, err
	}
	desugarer := NewDesugarer(names, variableNames, tracker, nil)
	current, err = transformModuleExpressionsWithError(current, "desugaring", nil, desugarer.Expression)
	if err != nil {
		return nil, err
	}
	current, err = renamer.RenameModule(current)
	if err != nil {
		return nil, fmt.Errorf("unique renaming: %w", err)
	}
	current, err = transformModuleExpressionsWithError(current, "normalization", nil,
		func(expression Expr) (Expr, error) { return NormalizeNNF(expression, tracker) })
	if err != nil {
		return nil, err
	}
	current, err = transformModuleExpressionsWithError(current, "early set membership simplification", nil,
		func(expression Expr) (Expr, error) { return SimplifySetMembership(expression, names, tracker, true) })
	if err != nil {
		return nil, err
	}
	keramelizer := NewKeramelizer(names, tracker)
	current, err = transformModuleExpressionsWithError(current, "KerA lowering", nil, keramelizer.Expression)
	if err != nil {
		return nil, err
	}
	current, err = renamer.RenameModule(current)
	if err != nil {
		return nil, fmt.Errorf("post-preprocessing unique renaming: %w", err)
	}
	if err := WatchModule(FlatLanguagePredicate{}, current); err != nil {
		return nil, fmt.Errorf("preprocessing language check: %w", err)
	}
	current.AddProperty(PropertyPreprocessed)
	return current, nil
}

// PreprocessReTLAModule runs the restricted preprocessing sequence used by
// Scala's ReTLAPreproPassImpl before VMT generation.
func PreprocessReTLAModule(module *Module, options PreprocessOptions) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot preprocess a nil ReTLA module")
	}
	if !module.HasProperty(PropertyInlined) {
		return nil, fmt.Errorf("ReTLA preprocessing requires an inlined module")
	}
	tracker, _, renamer := preprocessingState(options)
	current, err := transformModuleExpressionsWithError(module, "UNCHANGED unrolling", nil,
		func(expression Expr) (Expr, error) { return UnrollSimpleUnchanged(tracker)(expression), nil })
	if err != nil {
		return nil, err
	}
	stateVariables := make(map[string]struct{}, len(module.VarDeclarations()))
	for _, declaration := range module.VarDeclarations() {
		stateVariables[declaration.Name()] = struct{}{}
	}
	constantInitName := options.ConstantInitName
	if constantInitName == "" {
		constantInitName = "CInit"
	}
	include := func(declaration Decl) bool {
		operator, ok := declaration.(*OperDecl)
		return !ok || operator.Name() != constantInitName+"Primed"
	}
	current, err = transformModuleExpressionsWithError(current, "prime propagation", include,
		func(expression Expr) (Expr, error) { return PropagatePrimes(tracker, stateVariables)(expression), nil })
	if err != nil {
		return nil, err
	}
	current, err = renamer.RenameModule(current)
	if err != nil {
		return nil, fmt.Errorf("unique renaming: %w", err)
	}
	current, err = transformModuleExpressionsWithError(current, "normalization", nil,
		func(expression Expr) (Expr, error) { return NormalizeNNF(expression, tracker) })
	if err != nil {
		return nil, err
	}
	if err = WatchModule(ReTLALanguagePredicate{}, current); err != nil {
		return nil, fmt.Errorf("ReTLA preprocessing language check: %w", err)
	}
	current.AddProperty(PropertyPreprocessed)
	return current, nil
}

// OptimizeModule runs the same expression sequence as Scala's OptPassImpl.
func OptimizeModule(module *Module, options PreprocessOptions) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot optimize a nil module")
	}
	if !module.HasProperty(PropertyPreprocessed) {
		return nil, fmt.Errorf("optimization requires a preprocessed module")
	}
	tracker, names, _ := preprocessingState(options)
	current, err := transformModuleExpressionsWithError(module, "constant simplification", nil,
		func(expression Expr) (Expr, error) { return SimplifyConstants(expression, tracker) })
	if err != nil {
		return nil, err
	}
	optimizer := NewExprOptimizer(names, tracker)
	current, err = transformModuleExpressionsWithError(current, "expression optimization", nil, optimizer.Expression)
	if err != nil {
		return nil, err
	}
	current, err = transformModuleExpressionsWithError(current, "set membership simplification", nil,
		func(expression Expr) (Expr, error) { return SimplifySetMembership(expression, names, tracker, false) })
	if err != nil {
		return nil, err
	}
	current, err = transformModuleExpressionsWithError(current, "final constant simplification", nil,
		func(expression Expr) (Expr, error) { return SimplifyConstants(expression, tracker) })
	if err != nil {
		return nil, err
	}
	current.AddProperty(PropertyOptimized)
	return current, nil
}

func preprocessingState(options PreprocessOptions) (TransformationTracker, *UniqueNameGenerator, *IncrementalRenamer) {
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
	return tracker, names, renamer
}

func transformModuleExpressionsWithError(
	module *Module,
	stage string,
	include func(Decl) bool,
	transform func(Expr) (Expr, error),
) (*Module, error) {
	declarations := module.Declarations()
	for index, declaration := range declarations {
		if include != nil && !include(declaration) {
			continue
		}
		var body Expr
		switch typed := declaration.(type) {
		case *OperDecl:
			body = typed.Body()
		case *AssumeDecl:
			body = typed.Body()
		case *TheoremDecl:
			body = typed.Body()
		default:
			continue
		}
		transformed, err := transform(body)
		if err != nil {
			return nil, fmt.Errorf("%s in declaration %s: %w", stage, declaration.Name(), err)
		}
		switch typed := declaration.(type) {
		case *OperDecl:
			copy := typed.copyWithTag(typed.Tag())
			copy.SetBody(transformed)
			declarations[index] = copy
		case *AssumeDecl:
			if name, named := typed.DefinedName(); named {
				declarations[index] = NewNamedAssumeDecl(name, transformed, typed.Tag())
			} else {
				declarations[index] = NewUnnamedAssumeDecl(transformed, typed.Tag())
			}
		case *TheoremDecl:
			declarations[index] = NewTheoremDecl(typed.Name(), transformed, typed.Tag())
		}
	}
	result := NewModule(module.Name(), declarations)
	result.copyMetadataFrom(module)
	return result, nil
}
