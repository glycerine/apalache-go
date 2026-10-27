package apalache

import "fmt"

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

type LanguagePredicateError struct {
	Subject  string
	Failures []LanguageFailure
}

func (err *LanguagePredicateError) Error() string {
	return fmt.Sprintf("some %s do not fit the expected language fragment", err.Subject)
}

func WatchExpression(predicate LanguagePredicate, expression Expr) error {
	result := predicate.CheckExpression(expression)
	if result.OK() {
		return nil
	}
	return &LanguagePredicateError{Subject: "expressions", Failures: result.Failures}
}

func WatchModule(predicate LanguagePredicate, module *Module) error {
	result := predicate.CheckModule(module)
	if result.OK() {
		return nil
	}
	return &LanguagePredicateError{Subject: "declarations", Failures: result.Failures}
}

type FlatLanguagePredicate struct{}

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

type MonotypeLanguagePredicate struct{}

// TypedLanguagePredicate accepts polymorphic tags but rejects every untyped
// expression or declaration. It is used when loading already-typechecked IR,
// whose imported operators may legitimately retain type variables.
type TypedLanguagePredicate struct{}

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
	return PredicateFail(id, fmt.Sprintf("expected a non-polymorphic type, found: %s; you may need to annotate an empty collection", typ))
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
