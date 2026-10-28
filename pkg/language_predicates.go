package apalache

import (
	"fmt"
	"reflect"
)

type LanguageFailure struct {
	ID      UID
	Message string
}

type PredicateResult struct{ Failures []LanguageFailure }

func PredicateOK() PredicateResult { return PredicateResult{} }
func PredicateFail(id UID, message string) PredicateResult {
	return PredicateResult{Failures: []LanguageFailure{{ID: id, Message: message}}}
}
func (result PredicateResult) OK() bool { return len(result.Failures) == 0 }
func (result PredicateResult) And(other PredicateResult) PredicateResult {
	failures := make([]LanguageFailure, 0, len(result.Failures)+len(other.Failures))
	failures = append(failures, result.Failures...)
	failures = append(failures, other.Failures...)
	return PredicateResult{Failures: failures}
}

type LanguagePredicate interface {
	CheckExpression(Expr) PredicateResult
	CheckModule(*Module) PredicateResult
}

// NamedLanguagePredicate supplies the Scala class name used in watchdog
// diagnostics. Predicates without it fall back to their concrete Go name.
type NamedLanguagePredicate interface {
	LanguagePredicateName() string
}

type LanguagePredicateError struct {
	Subject       string
	PredicateName string
	Failures      []LanguageFailure
}

func (err *LanguagePredicateError) Error() string {
	if err.PredicateName == "" {
		return fmt.Sprintf("Some %s do not fit in the expected fragment", err.Subject)
	}
	return fmt.Sprintf("Some %s do not fit in the fragment %s", err.Subject, err.PredicateName)
}

// Unwrap exposes the canonical Scala LanguagePredError while retaining the
// older Go watch-error fields used by callers.
func (err *LanguagePredicateError) Unwrap() error {
	return NewLanguagePredError(err.Error(), err.Failures)
}

func WatchExpression(predicate LanguagePredicate, expression Expr) error {
	result := predicate.CheckExpression(expression)
	if result.OK() {
		return nil
	}
	return &LanguagePredicateError{
		Subject:       "expressions",
		PredicateName: languagePredicateName(predicate),
		Failures:      append([]LanguageFailure(nil), result.Failures...),
	}
}

func WatchModule(predicate LanguagePredicate, module *Module) error {
	result := predicate.CheckModule(module)
	if result.OK() {
		return nil
	}
	return &LanguagePredicateError{
		Subject:       "declarations",
		PredicateName: languagePredicateName(predicate),
		Failures:      append([]LanguageFailure(nil), result.Failures...),
	}
}

func languagePredicateName(predicate LanguagePredicate) string {
	if named, ok := predicate.(NamedLanguagePredicate); ok {
		return named.LanguagePredicateName()
	}
	typ := reflect.TypeOf(predicate)
	if typ == nil {
		return "<nil>"
	}
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ.Name()
}

type FlatLanguagePredicate struct{}

func (FlatLanguagePredicate) LanguagePredicateName() string { return "FlatLanguagePred" }

func (FlatLanguagePredicate) CheckExpression(expression Expr) PredicateResult {
	return checkFlatInContext(make(map[string]struct{}), expression)
}

func (predicate FlatLanguagePredicate) CheckModule(module *Module) PredicateResult {
	result := PredicateOK()
	for _, declaration := range module.OperDeclarations() {
		result = result.And(predicate.CheckExpression(declaration.Body()))
	}
	return result
}

func checkFlatInContext(context map[string]struct{}, expression Expr) PredicateResult {
	switch typed := expression.(type) {
	case *LetInExpr:
		definitionsResult := PredicateOK()
		definitionContext := cloneMap(context)
		for _, declaration := range typed.Declarations() {
			definitionsResult = definitionsResult.And(checkFlatInContext(definitionContext, declaration.Body()))
			definitionContext[declaration.Name()] = struct{}{}
		}
		bodyContext := cloneMap(context)
		for _, declaration := range typed.Declarations() {
			bodyContext[declaration.Name()] = struct{}{}
		}
		return definitionsResult.And(checkFlatInContext(bodyContext, typed.Body()))
	case *OperExpr:
		arguments := typed.Arguments()
		if typed.Operator().Name() == "OPER_APP" && len(arguments) > 0 {
			if name, ok := arguments[0].(*NameExpr); ok {
				if _, declared := context[name.Name()]; !declared {
					return PredicateFail(typed.ID(), fmt.Sprintf("undeclared operator %s [FlatLanguagePredicate]", name.Name()))
				}
				if len(arguments) > 1 {
					return PredicateFail(typed.ID(), fmt.Sprintf("non-nullary operator %s [FlatLanguagePredicate]", name.Name()))
				}
				return PredicateOK()
			}
		}
		result := PredicateOK()
		for _, argument := range arguments {
			result = result.And(checkFlatInContext(context, argument))
		}
		return result
	default:
		return PredicateOK()
	}
}

// ReTLALanguagePredicate accepts the restricted expression language consumed
// by the transition disassembler and VMT backend. User operators must already
// be inlined, except for nullary operators introduced by a surrounding LET-IN.
type ReTLALanguagePredicate struct{}

func (ReTLALanguagePredicate) LanguagePredicateName() string { return "ReTLALanguagePred" }

var reTLAOperators = map[string]struct{}{
	"Apalache!:=":     {},
	"Apalache!Skolem": {},
	"AND":             {},
	"EQUIV":           {},
	"EXISTS3":         {},
	"FORALL3":         {},
	"IMPLIES":         {},
	"NOT":             {},
	"OR":              {},
	"IF_THEN_ELSE":    {},
	"FUN_CTOR":        {},
	"EQ":              {},
	"NE":              {},
}

func (ReTLALanguagePredicate) CheckExpression(expression Expr) PredicateResult {
	return checkReTLAInContext(make(map[string]struct{}), expression)
}

func (predicate ReTLALanguagePredicate) CheckModule(module *Module) PredicateResult {
	result := PredicateOK()
	for _, declaration := range module.OperDeclarations() {
		result = result.And(predicate.CheckExpression(declaration.Body()))
	}
	return result
}

func checkReTLAArguments(context map[string]struct{}, arguments []Expr) PredicateResult {
	result := PredicateOK()
	for _, argument := range arguments {
		result = result.And(checkReTLAInContext(context, argument))
	}
	return result
}

func checkReTLAInContext(context map[string]struct{}, expression Expr) PredicateResult {
	switch typed := expression.(type) {
	case *ValueExpr:
		switch value := typed.Value().(type) {
		case BoolValue, IntValue, StringValue:
			return PredicateOK()
		case PredefinedSet:
			if value == BoolSet || value == IntSet || value == NatSet {
				return PredicateOK()
			}
		}
		return PredicateFail(typed.ID(), fmt.Sprint(typed.Value()))
	case *NameExpr:
		return PredicateOK()
	case *LetInExpr:
		definitionsResult := PredicateOK()
		definitionContext := cloneMap(context)
		for _, declaration := range typed.Declarations() {
			definitionsResult = definitionsResult.And(checkReTLAInContext(definitionContext, declaration.Body()))
			definitionContext[declaration.Name()] = struct{}{}
		}
		bodyContext := cloneMap(context)
		for _, declaration := range typed.Declarations() {
			bodyContext[declaration.Name()] = struct{}{}
		}
		return definitionsResult.And(checkReTLAInContext(bodyContext, typed.Body()))
	case *OperExpr:
		name := typed.Operator().Name()
		arguments := typed.Arguments()
		if name == "PRIME" && len(arguments) == 1 {
			if _, ok := arguments[0].(*NameExpr); ok {
				return PredicateOK()
			}
			return PredicateFail(typed.ID(), name)
		}
		if _, allowed := reTLAOperators[name]; allowed {
			return checkReTLAArguments(context, arguments)
		}
		if name == "FUN_APP" && len(arguments) == 2 {
			result := checkReTLAInContext(context, arguments[0])
			if tuple, ok := arguments[1].(*OperExpr); ok && tuple.Operator().Name() == "TUPLE" {
				return result.And(checkReTLAArguments(context, tuple.Arguments()))
			}
			return result.And(checkReTLAInContext(context, arguments[1]))
		}
		if name == "EXCEPT" && len(arguments) == 3 {
			result := checkReTLAInContext(context, arguments[0]).And(checkReTLAInContext(context, arguments[2]))
			key, ok := arguments[1].(*OperExpr)
			if !ok || key.Operator().Name() != "TUPLE" {
				return result.And(PredicateFail(arguments[1].ID(), "invalid EXCEPT key"))
			}
			keyArguments := key.Arguments()
			if len(keyArguments) != 1 {
				return result.And(PredicateFail(key.ID(), "invalid EXCEPT key"))
			}
			if nested, ok := keyArguments[0].(*OperExpr); ok && nested.Operator().Name() == "TUPLE" {
				return result.And(checkReTLAArguments(context, nested.Arguments()))
			}
			return result.And(checkReTLAInContext(context, keyArguments[0]))
		}
		if name == "OPER_APP" && len(arguments) > 0 {
			if operator, ok := arguments[0].(*NameExpr); ok {
				if _, declared := context[operator.Name()]; !declared {
					return PredicateFail(typed.ID(), fmt.Sprintf("undeclared operator %s [ReTLALanguagePred]", operator.Name()))
				}
				if len(arguments) > 1 {
					return PredicateFail(typed.ID(), fmt.Sprintf("non-nullary operator %s [ReTLALanguagePred]", operator.Name()))
				}
				return PredicateOK()
			}
		}
		return PredicateFail(typed.ID(), name)
	default:
		return PredicateFail(expression.ID(), fmt.Sprintf("%T", expression))
	}
}

type MonotypeLanguagePredicate struct{}

func (MonotypeLanguagePredicate) LanguagePredicateName() string { return "MonotypeLanguagePred" }

// TypedLanguagePredicate accepts polymorphic tags but rejects every untyped
// expression or declaration. It is used when loading already-typechecked IR,
// whose imported operators may legitimately retain type variables.
type TypedLanguagePredicate struct{}

func (TypedLanguagePredicate) LanguagePredicateName() string { return "TypedLanguagePred" }

func checkTyped(id UID, tag TypeTag) PredicateResult {
	if _, typed := tag.Type(); typed {
		return PredicateOK()
	}
	return PredicateFail(id, "expected a typed expression or declaration")
}

func (TypedLanguagePredicate) CheckExpression(expression Expr) PredicateResult {
	result := checkTyped(expression.ID(), expression.Tag())
	switch typed := expression.(type) {
	case *OperExpr:
		for _, argument := range typed.Arguments() {
			result = result.And((TypedLanguagePredicate{}).CheckExpression(argument))
		}
	case *LetInExpr:
		for _, declaration := range typed.Declarations() {
			result = result.And(checkTyped(declaration.ID(), declaration.Tag()))
			result = result.And((TypedLanguagePredicate{}).CheckExpression(declaration.Body()))
		}
		result = result.And((TypedLanguagePredicate{}).CheckExpression(typed.Body()))
	}
	return result
}

func (predicate TypedLanguagePredicate) CheckModule(module *Module) PredicateResult {
	result := PredicateOK()
	for _, declaration := range module.Declarations() {
		result = result.And(checkTyped(declaration.ID(), declaration.Tag()))
		switch typed := declaration.(type) {
		case *OperDecl:
			result = result.And(predicate.CheckExpression(typed.Body()))
		case *AssumeDecl:
			result = result.And(predicate.CheckExpression(typed.Body()))
		case *TheoremDecl:
			result = result.And(predicate.CheckExpression(typed.Body()))
		}
	}
	return result
}

func checkMonotype(id UID, tag TypeTag) PredicateResult {
	typ, typed := tag.Type()
	if !typed {
		return PredicateFail(id, "expected a typed, non-polymorphic expression")
	}
	if len(typ.UsedNames()) == 0 {
		return PredicateOK()
	}
	return PredicateFail(id, fmt.Sprintf("Expected a non-polymorphic type, found: %s\nYou may need to annotate an empty collection.", typ))
}

func (predicate MonotypeLanguagePredicate) CheckExpression(expression Expr) PredicateResult {
	switch typed := expression.(type) {
	case *OperExpr:
		result := checkMonotype(typed.ID(), typed.Tag())
		for _, argument := range typed.Arguments() {
			result = result.And(predicate.CheckExpression(argument))
		}
		return result
	case *LetInExpr:
		result := checkMonotype(typed.Body().ID(), typed.Body().Tag())
		for _, declaration := range typed.Declarations() {
			result = checkMonotype(declaration.ID(), declaration.Tag()).And(predicate.CheckExpression(declaration.Body())).And(result)
		}
		return result
	default:
		return checkMonotype(expression.ID(), expression.Tag())
	}
}

func (predicate MonotypeLanguagePredicate) CheckModule(module *Module) PredicateResult {
	result := PredicateOK()
	for _, declaration := range module.OperDeclarations() {
		result = predicate.CheckExpression(declaration.Body()).And(checkMonotype(declaration.ID(), declaration.Tag())).And(result)
	}
	return result
}

type NonrecursiveLanguagePredicate struct{}

func (NonrecursiveLanguagePredicate) LanguagePredicateName() string {
	return "NonrecursiveLanguagePred"
}

func (predicate NonrecursiveLanguagePredicate) checkDeclaration(declaration *OperDecl) PredicateResult {
	result := PredicateOK()
	if declaration.IsRecursive() {
		parameters := ""
		for index := range declaration.FormalParams() {
			if index > 0 {
				parameters += ","
			}
			parameters += "_"
		}
		result = PredicateFail(declaration.ID(), fmt.Sprintf("RECURSIVE %s(%s). Apalache does not support recursive operators.", declaration.Name(), parameters))
	}
	return result.And(predicate.CheckExpression(declaration.Body()))
}

func (predicate NonrecursiveLanguagePredicate) CheckExpression(expression Expr) PredicateResult {
	switch typed := expression.(type) {
	case *LetInExpr:
		result := predicate.CheckExpression(typed.Body())
		for _, declaration := range typed.Declarations() {
			result = result.And(predicate.checkDeclaration(declaration))
		}
		return result
	case *OperExpr:
		result := PredicateOK()
		if typed.Operator().Name() == "FUN_REC_CTOR" {
			result = PredicateFail(typed.ID(), "Apalache does not support recursive functions.")
		}
		for _, argument := range typed.Arguments() {
			result = result.And(predicate.CheckExpression(argument))
		}
		return result
	default:
		return PredicateOK()
	}
}

func (predicate NonrecursiveLanguagePredicate) CheckModule(module *Module) PredicateResult {
	result := PredicateOK()
	for _, declaration := range module.OperDeclarations() {
		result = result.And(predicate.checkDeclaration(declaration))
	}
	return result
}
