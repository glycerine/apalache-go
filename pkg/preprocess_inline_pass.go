package apalache

import "fmt"

type InlinePassOptions struct {
	Init         string
	Next         string
	ConstantInit string
	Relevant     []string
	Tracker      TransformationTracker
	Renamer      *IncrementalRenamer
}

// InlineConfiguredModule ports InlinePassImpl: it removes irrelevant
// operators while expanding their uses, then substitutes primitive constant
// values learned from CInit everywhere except CInit and CInitPrimed.
func InlineConfiguredModule(module *Module, options InlinePassOptions) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot inline a nil module")
	}
	if options.Init == "" || options.Next == "" {
		return nil, fmt.Errorf("inlining requires Init and Next operator names")
	}
	constantInit := options.ConstantInit
	if constantInit == "" {
		if _, found := OperatorBody(module, "CInit"); found {
			constantInit = "CInit"
		}
	}
	tracker := options.Tracker
	if tracker == nil {
		tracker = IdleTracker{}
	}
	renamer := options.Renamer
	if renamer == nil {
		renamer = NewIncrementalRenamer(tracker)
	}
	relevant := make(map[string]struct{}, len(options.Relevant)+5)
	for _, name := range options.Relevant {
		relevant[name] = struct{}{}
	}
	for _, name := range []string{options.Init, options.Next, options.Init + "Primed"} {
		relevant[name] = struct{}{}
	}
	if constantInit != "" {
		relevant[constantInit] = struct{}{}
		relevant[constantInit+"Primed"] = struct{}{}
	}

	renamed, err := renamer.RenameModule(module)
	if err != nil {
		return nil, fmt.Errorf("pre-inline unique renaming: %w", err)
	}
	inliner := NewInliner(tracker, renamer, true, KeepOnlyInlinedDeclarations(relevant))
	inlined, err := inliner.Module(renamed)
	if err != nil {
		return nil, fmt.Errorf("operator inlining: %w", err)
	}
	for _, name := range []string{options.Init, options.Next} {
		if _, found := OperatorBody(inlined, name); !found {
			return nil, fmt.Errorf("configured operator %s was not found after inlining", name)
		}
	}
	if constantInit == "" {
		result := NewModule(module.Name(), inlined.Declarations())
		result.copyMetadataFrom(module)
		result.AddProperty(PropertyInlined)
		return result, nil
	}
	cinitBody, found := OperatorBody(inlined, constantInit)
	if !found {
		return nil, fmt.Errorf("constant initializer operator %s was not found after inlining", constantInit)
	}
	constantNames := make([]string, len(module.ConstDeclarations()))
	for index, declaration := range module.ConstDeclarations() {
		constantNames[index] = declaration.Name()
	}
	constantInliner := NewTLAConstantInliner(tracker, constantNames)
	constantValues, err := constantInliner.BuildConstantMap(nil, cinitBody)
	if err != nil {
		return nil, fmt.Errorf("reading primitive constant initializations: %w", err)
	}
	declarations := inlined.Declarations()
	for index, declaration := range declarations {
		operator, ok := declaration.(*OperDecl)
		if !ok || operator.Name() == constantInit || operator.Name() == constantInit+"Primed" {
			continue
		}
		copy := operator.copyWithTag(operator.Tag())
		copy.SetBody(constantInliner.Replace(operator.Body(), constantValues))
		declarations[index] = tracker.TrackDeclaration(func(Decl) Decl { return copy })(operator)
	}
	result := NewModule(module.Name(), declarations)
	result.copyMetadataFrom(module)
	result.AddProperty(PropertyInlined)
	return result, nil
}
