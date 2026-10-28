package apalache

import (
	"fmt"
	"sort"
	"strings"
)

const (
	NormalizedInitPrefix           = "Init$"
	NormalizedNextPrefix           = "Next$"
	NormalizedConstInit            = "CInit$0"
	NormalizedVCInvariantPrefix    = "VCInv$"
	NormalizedVCNotInvariantPrefix = "VCNotInv$"
	NormalizedVCActionPrefix       = "VCActionInv$"
	NormalizedVCNotActionPrefix    = "VCNotActionInv$"
	NormalizedVCTracePrefix        = "VCTraceInv$"
	NormalizedVCNotTracePrefix     = "VCNotTraceInv$"
	NormalizedVCTemporalPrefix     = "VCTemporal$"
	NormalizedVCView               = "VCView$0"
)

// NormalizedStandardOptionNames returns the checker options that identify the
// critical pieces of a specification, in the order used by Scala's
// NormalizedNames.STANDARD_OPTION_NAMES.
func NormalizedStandardOptionNames() []string {
	return []string{ConfigInit, ConfigCInit, ConfigNext, ConfigInvariant, ConfigTemporal}
}

func IsVerificationCondition(declaration Decl) bool {
	if _, ok := declaration.(*OperDecl); !ok {
		return false
	}
	for _, prefix := range []string{
		NormalizedVCInvariantPrefix, NormalizedVCNotInvariantPrefix,
		NormalizedVCActionPrefix, NormalizedVCNotActionPrefix,
		NormalizedVCTracePrefix, NormalizedVCNotTracePrefix, NormalizedVCView,
	} {
		if strings.HasPrefix(declaration.Name(), prefix) {
			return true
		}
	}
	return false
}

func IsTemporalVerificationCondition(declaration Decl) bool {
	operator, ok := declaration.(*OperDecl)
	return ok && len(operator.FormalParams()) == 0 && strings.HasPrefix(operator.Name(), NormalizedVCTemporalPrefix)
}

func IsNormalizedConstantInit(declaration Decl) bool {
	operator, ok := declaration.(*OperDecl)
	return ok && len(operator.FormalParams()) == 0 && operator.Name() == NormalizedConstInit
}

func IsNormalizedInit(declaration Decl) bool {
	operator, ok := declaration.(*OperDecl)
	return ok && len(operator.FormalParams()) == 0 && strings.HasPrefix(operator.Name(), NormalizedInitPrefix)
}

func IsNormalizedNext(declaration *OperDecl) bool {
	return declaration != nil && len(declaration.FormalParams()) == 0 && strings.HasPrefix(declaration.Name(), NormalizedNextPrefix)
}

type TransitionPassOptions struct {
	Init         string
	Next         string
	ConstantInit string
	Invariants   []string
	Persistent   map[string]struct{}
	Tracker      TransformationTracker
	Sources      *SourceStore
	Renamer      *IncrementalRenamer
}

// FindTransitions ports TransitionPassImpl: it extracts Init/Next (and
// optional constant-init) transitions, retains verification declarations,
// and discards preprocessing-only operators.
func FindTransitions(module *Module, options TransitionPassOptions) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot find transitions in a nil module")
	}
	if !module.HasProperty(PropertyPrimed) || !module.HasProperty(PropertyPreprocessed) {
		return nil, fmt.Errorf("transition finding requires a primed and preprocessed module")
	}
	if options.Init == "" || options.Next == "" {
		return nil, fmt.Errorf("transition finding requires Init and Next operator names")
	}
	tracker := options.Tracker
	if tracker == nil {
		tracker = IdleTracker{}
	}
	renamer := options.Renamer
	if renamer == nil {
		renamer = NewIncrementalRenamer(tracker)
	}
	operators := OperatorBodyMap(module.OperDeclarations()...)
	extractor := NewSMTFreeSymbolicTransitionExtractor(tracker, options.Sources)
	stateVariables := make(map[string]struct{}, len(module.VarDeclarations()))
	userStateVariables := make(map[string]struct{}, len(module.VarDeclarations()))
	for _, declaration := range module.VarDeclarations() {
		stateVariables[declaration.Name()] = struct{}{}
		if !declaration.IsInternal() {
			userStateVariables[declaration.Name()] = struct{}{}
		}
	}
	extract := func(variables map[string]struct{}, inputName, outputPrefix string, expandChoices bool) ([]*OperDecl, error) {
		declaration, found := operators[inputName]
		if !found {
			return nil, fmt.Errorf("operator %s was not found", inputName)
		}
		action := declaration.Body()
		if expandChoices {
			action = expandFiniteQuantifiers(action, userStateVariables, tracker)
		}
		transitions, err := extractor.Extract(variables, action, operators)
		if err != nil {
			return nil, fmt.Errorf("extracting %s: %w", inputName, err)
		}
		sort.Slice(transitions, func(left, right int) bool {
			return transitions[left].Expression.ID().Compare(transitions[right].Expression.ID()) < 0
		})
		expressions := make([]Expr, len(transitions))
		for index, transition := range transitions {
			expressions[index] = transition.Expression
		}
		return ExpressionsToOperatorDefinitions(outputPrefix, expressions), nil
	}

	initDeclarations, err := extract(stateVariables, options.Init+"Primed", NormalizedInitPrefix, false)
	if err != nil {
		return nil, err
	}
	nextDeclarations, err := extract(stateVariables, options.Next, NormalizedNextPrefix, true)
	if err != nil {
		return nil, err
	}

	constantInitDeclarations := []*OperDecl(nil)
	if options.ConstantInit != "" {
		constants := make(map[string]struct{}, len(module.ConstDeclarations()))
		for _, declaration := range module.ConstDeclarations() {
			constants[declaration.Name()] = struct{}{}
		}
		parts, extractionErr := extract(constants, options.ConstantInit+"Primed", "", false)
		if extractionErr != nil {
			return nil, extractionErr
		}
		bodies := make([]Expr, len(parts))
		for index, declaration := range parts {
			bodies[index] = declaration.Body()
		}
		constantInitDeclarations = append(constantInitDeclarations,
			ExpressionToOperatorDefinition(NormalizedConstInit, MustOperExpr(Op("AND"), Typed(BoolT1), bodies...)))
	}

	invariants := make([]Decl, 0, len(options.Invariants))
	for _, name := range options.Invariants {
		declaration, found := declarationNamed(module, name)
		if !found {
			return nil, fmt.Errorf("invariant declaration %s was not found", name)
		}
		invariants = append(invariants, declaration)
	}
	verificationConditions := make([]Decl, 0)
	for _, declaration := range module.Declarations() {
		if IsVerificationCondition(declaration) {
			verificationConditions = append(verificationConditions, declaration)
		}
	}
	if len(verificationConditions) == 0 {
		verificationConditions = invariants
	}
	persistent := make([]Decl, 0)
	for _, declaration := range module.Declarations() {
		if _, keep := options.Persistent[declaration.Name()]; keep {
			persistent = append(persistent, declaration)
		}
	}

	declarations := make([]Decl, 0)
	for _, declaration := range module.ConstDeclarations() {
		declarations = append(declarations, declaration)
	}
	for _, declaration := range module.VarDeclarations() {
		declarations = append(declarations, declaration)
	}
	for _, declaration := range module.AssumeDeclarations() {
		declarations = append(declarations, declaration)
	}
	for _, group := range [][]*OperDecl{constantInitDeclarations, initDeclarations, nextDeclarations} {
		for _, declaration := range group {
			declarations = append(declarations, declaration)
		}
	}
	declarations = append(declarations, verificationConditions...)
	declarations = append(declarations, persistent...)
	result, err := renamer.RenameModule(NewModule(module.Name(), declarations))
	if err != nil {
		return nil, fmt.Errorf("renaming extracted transitions: %w", err)
	}
	result.copyMetadataFrom(module)
	result.AddProperty(PropertyTransitionsFound)
	return result, nil
}

func declarationNamed(module *Module, name string) (Decl, bool) {
	for _, declaration := range module.Declarations() {
		if declaration.Name() == name {
			return declaration, true
		}
	}
	return nil, false
}
