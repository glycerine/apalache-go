package apalache

import "fmt"

type PrimingOptions struct {
	Init         string
	ConstantInit string
	Tracker      TransformationTracker
}

// PrimeInitializers appends InitPrimed and optional CInitPrimed definitions
// whose copied bodies refer to primed state variables and constants.
func PrimeInitializers(module *Module, options PrimingOptions) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot prime initializers in a nil module")
	}
	if !module.HasProperty(PropertyConfigured) {
		return nil, fmt.Errorf("priming requires a configured module")
	}
	if options.Init == "" {
		return nil, fmt.Errorf("priming requires an Init operator name")
	}
	tracker := options.Tracker
	if tracker == nil {
		tracker = IdleTracker{}
	}
	operators := OperatorBodyMap(module.OperDeclarations()...)
	copyAndPrime := func(name string, selected map[string]struct{}) (*OperDecl, error) {
		declaration, found := operators[name]
		if !found {
			return nil, fmt.Errorf("initializer operator %s was not found", name)
		}
		body := NewDeepCopier(tracker).Expression(declaration.Body())
		body = DecorateWithPrime(tracker, selected)(body)
		return ExpressionToOperatorDefinition(name+"Primed", body), nil
	}

	declarations := module.Declarations()
	if options.ConstantInit != "" {
		constants := make(map[string]struct{}, len(module.ConstDeclarations()))
		for _, declaration := range module.ConstDeclarations() {
			constants[declaration.Name()] = struct{}{}
		}
		primed, err := copyAndPrime(options.ConstantInit, constants)
		if err != nil {
			return nil, err
		}
		declarations = append(declarations, primed)
	}
	variables := make(map[string]struct{}, len(module.VarDeclarations()))
	for _, declaration := range module.VarDeclarations() {
		variables[declaration.Name()] = struct{}{}
	}
	primedInit, err := copyAndPrime(options.Init, variables)
	if err != nil {
		return nil, err
	}
	declarations = append(declarations, primedInit)
	result := NewModule(module.Name(), declarations)
	result.properties = cloneMap(module.properties)
	result.AddProperty(PropertyPrimed)
	return result, nil
}
