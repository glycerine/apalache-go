package apalache

import (
	"fmt"
	"strings"
)

type SpecificationConfiguration struct {
	Behavior           BehaviorSpec
	ConstantInit       string
	Invariants         []string
	StateConstraints   []string
	ActionConstraints  []string
	TemporalProperties []string
	View               string
	Persistent         []string
	TLCConfig          *TLCConfig
	ContextName        string
	Tracker            TransformationTracker
}

type DerivedPredicates struct {
	Init               string
	Next               string
	ConstantInit       string
	Invariants         []string
	StateConstraints   []string
	ActionConstraints  []string
	TemporalProperties []string
	View               string
	Persistent         []string
}

func (predicates DerivedPredicates) OperatorNames() []string {
	result := []string{predicates.Init, predicates.Next}
	for _, name := range []string{predicates.ConstantInit, predicates.View} {
		if name != "" {
			result = append(result, name)
		}
	}
	result = append(result, predicates.Invariants...)
	result = append(result, predicates.StateConstraints...)
	result = append(result, predicates.ActionConstraints...)
	result = append(result, predicates.TemporalProperties...)
	result = append(result, predicates.Persistent...)
	return result
}

type ConfigurationResult struct {
	Module     *Module
	Predicates DerivedPredicates
}

type ConfigurationError struct{ Message string }

func (err *ConfigurationError) Error() string { return err.Message }

// ConfigureModule collects specification predicates and applies TLC constant
// and definition overrides, following ConfigurationPassImpl.
func ConfigureModule(module *Module, options SpecificationConfiguration) (ConfigurationResult, error) {
	if module == nil {
		return ConfigurationResult{}, &ConfigurationError{Message: "cannot configure a nil module"}
	}
	if !module.HasProperty(PropertyTypeChecked) {
		return ConfigurationResult{}, &ConfigurationError{Message: "configuration requires a typechecked module"}
	}
	if err := WatchModule(NonrecursiveLanguagePredicate{}, module); err != nil {
		return ConfigurationResult{}, &ConfigurationError{Message: err.Error()}
	}
	tracker := options.Tracker
	if tracker == nil {
		tracker = IdleTracker{}
	}
	predicates := DerivedPredicates{
		ConstantInit:       options.ConstantInit,
		Invariants:         append([]string(nil), options.Invariants...),
		StateConstraints:   append([]string(nil), options.StateConstraints...),
		ActionConstraints:  append([]string(nil), options.ActionConstraints...),
		TemporalProperties: append([]string(nil), options.TemporalProperties...),
		View:               options.View,
		Persistent:         append([]string(nil), options.Persistent...),
	}
	overrides := []Decl(nil)
	if options.TLCConfig == nil {
		behavior, ok := options.Behavior.(InitNextSpec)
		if !ok {
			return ConfigurationResult{}, &ConfigurationError{Message: "INIT and NEXT must be configured when no TLC configuration is provided"}
		}
		predicates.Init, predicates.Next = behavior.Init, behavior.Next
	} else {
		imported, err := ImportTLCConfig(module, *options.TLCConfig)
		if err != nil {
			return ConfigurationResult{}, err
		}
		for _, declaration := range imported.Declarations()[len(module.Declarations()):] {
			if strings.HasPrefix(declaration.Name(), OverridePrefix) {
				overrides = append(overrides, declaration)
			}
		}
		behavior := options.Behavior
		if behavior == nil {
			behavior = options.TLCConfig.Behavior
		}
		switch typed := behavior.(type) {
		case InitNextSpec:
			predicates.Init, predicates.Next = typed.Init, typed.Next
		case TemporalSpec:
			context := options.ContextName
			if context == "" {
				context = "TLC configuration"
			}
			predicates.Init, predicates.Next, err = extractInitNextFromSpecification(module, context, typed.Name)
			if err != nil {
				return ConfigurationResult{}, err
			}
		default:
			predicates.Init, predicates.Next = "Init", "Next"
		}
		if len(predicates.Invariants) == 0 {
			predicates.Invariants = append([]string(nil), options.TLCConfig.Invariants...)
		}
		if len(predicates.StateConstraints) == 0 {
			predicates.StateConstraints = append([]string(nil), options.TLCConfig.StateConstraints...)
		}
		if len(predicates.ActionConstraints) == 0 {
			predicates.ActionConstraints = append([]string(nil), options.TLCConfig.ActionConstraints...)
		}
		if len(predicates.TemporalProperties) == 0 {
			predicates.TemporalProperties = append([]string(nil), options.TLCConfig.TemporalProperties...)
		}
		predicates.Persistent = nil
	}

	constantOverrideTypes := make(map[string]TypeTag)
	for _, declaration := range module.ConstDeclarations() {
		constantOverrideTypes[OverridePrefix+declaration.Name()] = declaration.Tag()
	}
	constantOverrides := make([]*OperDecl, 0)
	otherOverrides := make([]Decl, 0)
	for _, declaration := range overrides {
		if constantType, constant := constantOverrideTypes[declaration.Name()]; constant {
			override, ok := declaration.(*OperDecl)
			if !ok || !override.Body().Tag().Equal(constantType) {
				return ConfigurationResult{}, &ConfigurationError{Message: fmt.Sprintf("constant %s has type %s, while its TLC value has type %s", strings.TrimPrefix(declaration.Name(), OverridePrefix), constantType, declarationBodyTag(declaration))}
			}
			constantOverrides = append(constantOverrides, override)
		} else {
			otherOverrides = append(otherOverrides, declaration)
		}
	}
	declarations := module.Declarations()
	if len(constantOverrides) != 0 {
		if predicates.ConstantInit == "" {
			predicates.ConstantInit = "CInit"
		}
		declarations = extendConstantInit(declarations, predicates.ConstantInit, constantOverrides)
	}
	declarations = append(declarations, otherOverrides...)
	current := NewModule(module.Name(), declarations)
	current.properties = cloneMap(module.properties)
	checks := []struct {
		role  string
		names []string
	}{
		{"the initialization predicate", []string{predicates.Init}},
		{"the transition predicate", []string{predicates.Next}},
		{"the constant initialization predicate", compactNames(predicates.ConstantInit)},
		{"an invariant", predicates.Invariants},
		{"a state constraint", predicates.StateConstraints},
		{"an action constraint", predicates.ActionConstraints},
		{"a temporal property", predicates.TemporalProperties},
	}
	for _, check := range checks {
		for _, name := range check.names {
			if _, found := OperatorBody(current, name); !found {
				return ConfigurationResult{}, &ConfigurationError{Message: fmt.Sprintf("operator %s not found (used as %s)", name, check.role)}
			}
		}
	}
	configured, err := RewriteOverrides(current, tracker)
	if err != nil {
		return ConfigurationResult{}, err
	}
	configured.AddProperty(PropertyConfigured)
	return ConfigurationResult{Module: configured, Predicates: predicates}, nil
}

func declarationBodyTag(declaration Decl) TypeTag {
	if operator, ok := declaration.(*OperDecl); ok {
		return operator.Body().Tag()
	}
	return declaration.Tag()
}

func extendConstantInit(declarations []Decl, name string, overrides []*OperDecl) []Decl {
	conjuncts := make([]Expr, 0, len(overrides)+1)
	for _, declaration := range declarations {
		if operator, ok := declaration.(*OperDecl); ok && operator.Name() == name {
			conjuncts = append(conjuncts, operator.Body())
			break
		}
	}
	for _, override := range overrides {
		constantName := strings.TrimPrefix(override.Name(), OverridePrefix)
		conjuncts = append(conjuncts, MustOperExpr(Op("EQ"), Typed(BoolT1),
			NewNameExpr(constantName, override.Body().Tag()), override.Body()))
	}
	newInitializer := NewOperDecl(name, nil, MustOperExpr(Op("AND"), Typed(BoolT1), conjuncts...), Typed(mustOperator(nil, BoolT1)))
	result := make([]Decl, 0, len(declarations)+1)
	for _, declaration := range declarations {
		if declaration.Name() != name {
			result = append(result, declaration)
		}
	}
	return append(result, newInitializer)
}

func extractInitNextFromSpecification(module *Module, context, name string) (string, string, error) {
	declaration, found := declarationNamed(module, name)
	operator, ok := declaration.(*OperDecl)
	if !found || !ok || len(operator.FormalParams()) != 0 {
		return "", "", &ConfigurationError{Message: fmt.Sprintf("%s: expected %s to be in the canonical form Init /\\ [][Next]_vars /\\ ...", context, name)}
	}
	flattened := flattenConjunction(operator.Body())
	and, ok := flattened.(*OperExpr)
	if !ok || and.Operator().Name() != "AND" {
		return "", "", &ConfigurationError{Message: fmt.Sprintf("%s: expected %s to be in the canonical form Init /\\ [][Next]_vars /\\ ...", context, name)}
	}
	nonFairness := make([]Expr, 0, len(and.Arguments()))
	for _, argument := range and.Arguments() {
		if fairness, ok := argument.(*OperExpr); ok && (fairness.Operator().Name() == "WEAK_FAIRNESS" || fairness.Operator().Name() == "STRONG_FAIRNESS") {
			continue
		}
		nonFairness = append(nonFairness, argument)
	}
	if len(nonFairness) != 2 {
		return "", "", &ConfigurationError{Message: fmt.Sprintf("%s: expected %s to be in the canonical form Init /\\ [][Next]_vars /\\ ...", context, name)}
	}
	if init, initOK := nullaryApplicationName(nonFairness[0]); initOK {
		if next, nextOK := boxedStutteringApplicationName(nonFairness[1]); nextOK {
			return init, next, nil
		}
	}
	if next, nextOK := boxedStutteringApplicationName(nonFairness[0]); nextOK {
		if init, initOK := nullaryApplicationName(nonFairness[1]); initOK {
			return init, next, nil
		}
	}
	return "", "", &ConfigurationError{Message: fmt.Sprintf("%s: expected %s to be in the canonical form Init /\\ [][Next]_vars /\\ ...", context, name)}
}

func flattenConjunction(expression Expr) Expr {
	operation, ok := expression.(*OperExpr)
	if !ok || operation.Operator().Name() != "AND" {
		return expression
	}
	arguments := make([]Expr, 0)
	for _, argument := range operation.Arguments() {
		flattened := flattenConjunction(argument)
		if nested, ok := flattened.(*OperExpr); ok && nested.Operator().Name() == "AND" {
			arguments = append(arguments, nested.Arguments()...)
		} else {
			arguments = append(arguments, flattened)
		}
	}
	return MustOperExpr(Op("AND"), operation.Tag(), arguments...)
}

func nullaryApplicationName(expression Expr) (string, bool) {
	application, ok := expression.(*OperExpr)
	if !ok || application.Operator().Name() != "OPER_APP" || len(application.Arguments()) != 1 {
		return "", false
	}
	name, ok := application.Arguments()[0].(*NameExpr)
	if !ok {
		return "", false
	}
	return name.Name(), true
}

func boxedStutteringApplicationName(expression Expr) (string, bool) {
	box, ok := expression.(*OperExpr)
	if !ok || box.Operator().Name() != "GLOBALLY" {
		return "", false
	}
	stutter, ok := box.Arguments()[0].(*OperExpr)
	if !ok || stutter.Operator().Name() != "STUTTER" {
		return "", false
	}
	return nullaryApplicationName(stutter.Arguments()[0])
}

func compactNames(name string) []string {
	if name == "" {
		return nil
	}
	return []string{name}
}
