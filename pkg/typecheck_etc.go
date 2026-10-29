package apalache

import (
	"errors"
	"fmt"
	"strings"
)

var errTypeInferenceUnwind = errors.New("type inference failed")

type EtcTypeChecker struct {
	pool           *TypeVarPool
	inferPolytypes bool
	listener       TypeCheckerListener
}

func NewEtcTypeChecker(pool *TypeVarPool, inferPolytypes bool) *EtcTypeChecker {
	if pool == nil {
		panic("NewEtcTypeChecker requires a type variable pool")
	}
	return &EtcTypeChecker{pool: pool, inferPolytypes: inferPolytypes, listener: IdleTypeCheckerListener{}}
}

// Compute infers the expression type. A type error is reported to listener and
// returned as ok=false; err is reserved for malformed ETC or internal errors.
func (checker *EtcTypeChecker) Compute(listener TypeCheckerListener, context TypeContext, expression EtcExpr) (typ Type, ok bool, err error) {
	if expression == nil {
		return nil, false, fmt.Errorf("cannot type-check a nil ETC expression")
	}
	if listener == nil {
		listener = IdleTypeCheckerListener{}
	}
	checker.listener = listener
	solver := NewConstraintSolver(checker.pool, nil)
	rootType, err := checker.computeRec(context, solver, expression)
	if errors.Is(err, errTypeInferenceUnwind) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	solution, solved, err := solver.Solve()
	if err != nil {
		return nil, false, err
	}
	if !solved {
		checker.onTypeError(expression.SourceRef(), "Error when computing the type of the top expression")
		return nil, false, nil
	}
	exactType, err := solution.ApplyRecursive(rootType)
	if err != nil {
		return nil, false, err
	}
	checker.onTypeFound(expression.SourceRef(), exactType)
	return exactType, true, nil
}

func (checker *EtcTypeChecker) computeRec(context TypeContext, solver *ConstraintSolver, expression EtcExpr) (Type, error) {
	switch typed := expression.(type) {
	case *EtcConst:
		fresh := checker.pool.Fresh()
		clause := &EqClause{Unknown: fresh, Term: typed.Polytype()}
		clause.OnTypeFound = func(found Type) { checker.onTypeFound(expression.SourceRef(), found) }
		clause.OnTypeError = func(*Substitution, []Type) { checker.onTypeError(expression.SourceRef(), "Unresolved type") }
		if err := solver.Add(clause); err != nil {
			return nil, err
		}
		return fresh, nil

	case *EtcTypeDecl:
		quantified := typed.DeclaredType().UsedNames()
		scheme, err := NewTypeScheme(typed.DeclaredType(), quantified...)
		if err != nil {
			return nil, err
		}
		extended := context.Bind(typed.Name(), scheme)
		if len(quantified) == 0 {
			fresh := checker.pool.Fresh()
			watch := &EqClause{Unknown: fresh, Term: typed.DeclaredType(), OnTypeFound: func(found Type) {
				checker.onTypeFound(expression.SourceRef(), found)
			}}
			if err = solver.Add(watch); err != nil {
				return nil, err
			}
		}
		return checker.computeRec(extended, solver, typed.Scoped())

	case *EtcName:
		scheme, err := context.Lookup(typed.Name())
		if err != nil {
			checker.onTypeError(expression.SourceRef(), fmt.Sprintf("No annotation found for %s. Make sure that you've put one in front of %s.", typed.Name(), typed.Name()))
			return nil, errTypeInferenceUnwind
		}
		if len(scheme.Principal().UsedNames()) == 0 {
			checker.onTypeFound(expression.SourceRef(), scheme.Principal())
			return scheme.Principal(), nil
		}
		return checker.computeRec(context, solver, NewEtcConst(expression.SourceRef(), scheme.Principal()))

	case *EtcApp:
		return checker.computeApplication(context, solver, typed)

	case *EtcAppByName:
		scheme, err := context.Lookup(typed.Name().Name())
		if err != nil {
			checker.onTypeError(expression.SourceRef(), fmt.Sprintf("The operator %s is used before it is defined.", typed.Name()))
			return nil, errTypeInferenceUnwind
		}
		instantiated, err := checker.instantiate(scheme)
		if err != nil {
			return nil, err
		}
		fresh := checker.pool.Fresh()
		if err = solver.Add(&EqClause{Unknown: fresh, Term: instantiated, OnTypeFound: func(found Type) {
			checker.onTypeFound(typed.Name().SourceRef(), found)
		}}); err != nil {
			return nil, err
		}
		application := NewEtcApp(expression.SourceRef(), []Type{instantiated}, typed.Arguments()...)
		application.SetTypeErrorExplanation(func(expected, actual []Type) (string, bool) {
			return typed.Explain(expected, actual)
		})
		return checker.computeApplication(context, solver, application)

	case *EtcAbs:
		extended, err := checker.translateBindings(context, solver, typed.Bindings())
		if err != nil {
			return nil, err
		}
		bodyType, err := checker.computeRec(extended, solver, typed.Body())
		if err != nil {
			return nil, err
		}
		parameterTypes := make([]Type, len(typed.Bindings()))
		for i, binding := range typed.Bindings() {
			scheme, lookupErr := extended.Lookup(binding.Name.Name())
			if lookupErr != nil {
				return nil, lookupErr
			}
			parameterTypes[i] = scheme.Principal()
		}
		operatorType, _ := NewOperatorType(parameterTypes, bodyType)
		lambdaVar := checker.pool.Fresh()
		clause := &EqClause{Unknown: lambdaVar, Term: operatorType,
			OnTypeFound: func(found Type) { checker.onTypeFound(expression.SourceRef(), found) },
			OnTypeError: func(_ *Substitution, types []Type) {
				message := "Type error in parameters"
				if len(types) > 0 {
					message += ": " + types[0].String()
				}
				checker.onTypeError(expression.SourceRef(), message)
			}}
		if err = solver.Add(clause); err != nil {
			return nil, err
		}
		return operatorType, nil

	case *EtcLet:
		abstraction, ok := typed.Bound().(*EtcAbs)
		if !ok {
			return nil, fmt.Errorf("ill-formed ETC let %q: bound expression is %T, expected abstraction", typed.Name(), typed.Bound())
		}
		return checker.computeLet(context, solver, typed, abstraction)

	default:
		return nil, fmt.Errorf("unsupported ETC expression %T", expression)
	}
}

func (checker *EtcTypeChecker) computeApplication(context TypeContext, solver *ConstraintSolver, application *EtcApp) (Type, error) {
	arguments := application.Arguments()
	argumentTypes := make([]Type, len(arguments))
	for i, argument := range arguments {
		typ, err := checker.computeRec(context, solver, argument)
		if err != nil {
			return nil, err
		}
		argumentTypes[i] = typ
	}
	resultVar := checker.pool.Fresh()
	operatorVar := checker.pool.Fresh()
	expectedApplication, _ := NewOperatorType(argumentTypes, resultVar)
	applicationClause := &EqClause{Unknown: operatorVar, Term: expectedApplication}
	applicationClause.OnTypeFound = func(found Type) {
		if operator, ok := found.(OperatorType); ok {
			checker.onTypeFound(application.SourceRef(), operator.Result())
		}
	}
	applicationClause.OnTypeError = func(solution *Substitution, _ []Type) {
		evaluatedArgs := applyTypes(solution, argumentTypes)
		evaluatedSignature, _ := solution.ApplyRecursive(operatorVar)
		message := fmt.Sprintf("An operator with the signature %s cannot be applied to the provided %s of type %s",
			evaluatedSignature, pluralArguments(len(argumentTypes)), joinTypeList(evaluatedArgs, " and "))
		if specific, found := application.Explain([]Type{evaluatedSignature}, evaluatedArgs); found {
			message = specific
		}
		checker.onTypeError(application.SourceRef(), message)
	}
	if err := solver.Add(applicationClause); err != nil {
		return nil, err
	}

	operatorTypes := application.OperatorTypes()
	if len(operatorTypes) == 1 {
		signature := operatorTypes[0]
		clause := &EqClause{Unknown: operatorVar, Term: signature}
		clause.OnTypeError = func(solution *Substitution, signatures []Type) {
			evaluatedArgs := applyTypes(solution, argumentTypes)
			evaluatedSignature := signature
			if len(signatures) > 0 {
				evaluatedSignature = signatures[0]
			}
			message := fmt.Sprintf("An operator with the signature %s cannot be applied to the provided %s of type %s",
				evaluatedSignature, pluralArguments(len(argumentTypes)), joinTypeList(evaluatedArgs, " and "))
			if specific, found := application.Explain([]Type{evaluatedSignature}, evaluatedArgs); found {
				message = specific
			}
			checker.onTypeError(application.SourceRef(), message)
		}
		if err := solver.Add(clause); err != nil {
			return nil, err
		}
	} else {
		alternatives := make([]*EqClause, len(operatorTypes))
		for i, signature := range operatorTypes {
			alternatives[i] = &EqClause{Unknown: operatorVar, Term: signature}
		}
		clause := &OrClause{Clauses: alternatives}
		clause.OnTypeError = func(solution *Substitution, signatures []Type) {
			evaluatedArgs := applyTypes(solution, argumentTypes)
			message := fmt.Sprintf("No matching signature for %s %s", pluralArguments(len(argumentTypes)), joinTypeList(evaluatedArgs, " and "))
			if len(signatures) > 0 {
				message = fmt.Sprintf("Annotation required. Found %d matching operator signatures %s for %s %s",
					len(signatures), joinTypeList(signatures, " or "), pluralArguments(len(argumentTypes)), joinTypeList(evaluatedArgs, " and "))
			}
			if specific, found := application.Explain(signatures, evaluatedArgs); found {
				message = specific
			}
			checker.onTypeError(application.SourceRef(), message)
		}
		if err := solver.Add(clause); err != nil {
			return nil, err
		}
	}
	return resultVar, nil
}

func (checker *EtcTypeChecker) computeLet(context TypeContext, outerSolver *ConstraintSolver, let *EtcLet, abstraction *EtcAbs) (Type, error) {
	approximation, consistent, err := outerSolver.SolvePartially()
	if err != nil {
		return nil, err
	}
	if !consistent {
		return nil, errTypeInferenceUnwind
	}
	definitionSolver := NewConstraintSolver(checker.pool, nil)
	scheme, annotated := context.types[let.Name()]
	if annotated {
		if _, ok := scheme.Principal().(OperatorType); !ok {
			operator, makeErr := NewOperatorType(nil, scheme.Principal())
			if makeErr != nil {
				return nil, makeErr
			}
			scheme, _ = NewTypeScheme(operator, scheme.Quantified()...)
		}
	} else {
		arguments := make([]Type, len(abstraction.Bindings()))
		quantified := make([]int, 0, len(arguments)+1)
		for i := range arguments {
			variable := checker.pool.Fresh()
			arguments[i] = variable
			quantified = append(quantified, variable.Number())
		}
		result := checker.pool.Fresh()
		quantified = append(quantified, result.Number())
		operator, _ := NewOperatorType(arguments, result)
		scheme, _ = NewTypeScheme(operator, quantified...)
	}

	refined := context.Types()
	refined[let.Name()] = scheme
	for name, current := range refined {
		principal, applyErr := approximation.ApplyRecursive(current.Principal())
		if applyErr != nil {
			return nil, applyErr
		}
		refined[name], _ = NewTypeScheme(principal, current.Quantified()...)
	}
	preContext := NewTypeContext(refined)
	extended, err := checker.translateBindings(preContext, definitionSolver, abstraction.Bindings())
	if err != nil {
		return nil, err
	}
	operatorScheme, ok := scheme.Principal().(OperatorType)
	if !ok {
		return nil, fmt.Errorf("let scheme is not an operator type: %T", scheme.Principal())
	}
	if len(operatorScheme.Arguments()) != len(abstraction.Bindings()) {
		checker.onTypeError(abstraction.SourceRef(), fmt.Sprintf("Expected %d parameters in %s, found %d", len(operatorScheme.Arguments()), let.Name(), len(abstraction.Bindings())))
		return nil, errTypeInferenceUnwind
	}
	for i, binding := range abstraction.Bindings() {
		boundScheme, lookupErr := extended.Lookup(binding.Name.Name())
		if lookupErr != nil {
			return nil, lookupErr
		}
		parameter, ok := boundScheme.Principal().(VarType)
		if !ok {
			return nil, fmt.Errorf("unexpected binder type %T", boundScheme.Principal())
		}
		annotation := operatorScheme.Arguments()[i]
		if err = definitionSolver.Add(&EqClause{Unknown: parameter, Term: annotation, OnTypeError: func(_ *Substitution, types []Type) {
			message := fmt.Sprintf("Mismatch in parameter %s", binding.Name.Name())
			if len(types) > 0 {
				message += ". Found: " + types[0].String()
			}
			checker.onTypeError(binding.Name.SourceRef(), message)
		}}); err != nil {
			return nil, err
		}
	}

	reportDefinitionError := func(solution *Substitution, types []Type) {
		resolved := applyTypes(solution, types)
		expected, _ := solution.ApplyRecursive(scheme.Principal())
		checker.onTypeError(abstraction.SourceRef(), fmt.Sprintf("Expected %s in %s. Found: %s",
			formatDiagnosticType(expected), let.Name(), joinDiagnosticTypeList(resolved, " and ")))
	}
	operatorVar := checker.pool.Fresh()
	if err = definitionSolver.Add(&EqClause{Unknown: operatorVar, Term: scheme.Principal(), OnTypeError: reportDefinitionError}); err != nil {
		return nil, err
	}
	bodyType, err := checker.computeRec(extended, definitionSolver, abstraction.Body())
	if err != nil {
		return nil, err
	}
	parameterTypes := make([]Type, len(abstraction.Bindings()))
	for i, binding := range abstraction.Bindings() {
		boundScheme, _ := extended.Lookup(binding.Name.Name())
		parameterTypes[i] = boundScheme.Principal()
	}
	definitionType, _ := NewOperatorType(parameterTypes, bodyType)
	if err = definitionSolver.Add(&EqClause{Unknown: operatorVar, Term: definitionType, OnTypeError: reportDefinitionError}); err != nil {
		return nil, err
	}
	definitionSolution, solved, err := definitionSolver.Solve()
	if err != nil {
		return nil, err
	}
	if !solved {
		checker.onTypeError(let.SourceRef(), fmt.Sprintf("Error when computing the type of %s", let.Name()))
		return nil, errTypeInferenceUnwind
	}
	principalType, err := definitionSolution.ApplyRecursive(definitionType)
	if err != nil {
		return nil, err
	}
	freeVariables := make([]int, 0)
	for _, number := range principalType.UsedNames() {
		if outerSolver.IsFreeVar(number) {
			freeVariables = append(freeVariables, number)
		}
	}
	if !checker.inferPolytypes && len(freeVariables) > 0 {
		checker.onTypeError(let.SourceRef(), fmt.Sprintf("Operator %s has a parameterized type, while polymorphism is disabled: %s", let.Name(), principalType))
		return nil, errTypeInferenceUnwind
	}
	if annotated {
		_, unified, unifies, unifyErr := NewTypeUnifier(checker.pool).Unify(EmptySubstitution(), principalType, scheme.Principal())
		if unifyErr != nil {
			return nil, unifyErr
		}
		if !unifies {
			return nil, fmt.Errorf("contradiction in type solver: %s and %s should be unifiable", principalType, scheme.Principal())
		}
		if len(unified.UsedNames()) < len(scheme.Principal().UsedNames()) {
			checker.onTypeWarning(let.SourceRef(), fmt.Sprintf("%s's type annotation %s is too general, inferred: %s", let.Name(), scheme.Principal(), principalType))
		}
	}
	checker.onTypeFound(abstraction.SourceRef(), principalType)
	principalScheme, err := NewTypeScheme(principalType, freeVariables...)
	if err != nil {
		return nil, err
	}
	return checker.computeRec(context.Bind(let.Name(), principalScheme), outerSolver, let.Body())
}

func (checker *EtcTypeChecker) translateBindings(context TypeContext, solver *ConstraintSolver, bindings []EtcBinding) (TypeContext, error) {
	domainTypes := make([]Type, len(bindings))
	for i, binding := range bindings {
		typ, err := checker.computeRec(context, solver, binding.Domain)
		if err != nil {
			return TypeContext{}, err
		}
		domainTypes[i] = typ
	}
	setVariables := checker.pool.FreshN(len(bindings))
	elementVariables := checker.pool.FreshN(len(bindings))
	for i, binding := range bindings {
		setType, _ := NewSetType(elementVariables[i])
		clause := &EqClause{Unknown: setVariables[i], Term: setType,
			OnTypeFound: func(found Type) { checker.onTypeFound(binding.Domain.SourceRef(), found) },
			OnTypeError: func(_ *Substitution, types []Type) {
				message := "Expected a set"
				if len(types) > 0 {
					message += ". Found: " + types[0].String()
				}
				checker.onTypeError(binding.Domain.SourceRef(), message)
			}}
		if err := solver.Add(clause); err != nil {
			return TypeContext{}, err
		}
		if err := solver.Add(&EqClause{Unknown: setVariables[i], Term: domainTypes[i], OnTypeError: clause.OnTypeError}); err != nil {
			return TypeContext{}, err
		}
		if err := solver.Add(&EqClause{Unknown: elementVariables[i], Term: elementVariables[i], OnTypeFound: func(found Type) {
			checker.onTypeFound(binding.Name.SourceRef(), found)
		}}); err != nil {
			return TypeContext{}, err
		}
	}
	result := context
	for i, binding := range bindings {
		scheme, _ := NewTypeScheme(elementVariables[i])
		result = result.Bind(binding.Name.Name(), scheme)
	}
	return result, nil
}

func (checker *EtcTypeChecker) instantiate(scheme TypeScheme) (Type, error) {
	entries := make([]SubstitutionEntry, len(scheme.Quantified()))
	for i, variable := range scheme.Quantified() {
		entries[i] = SubstitutionEntry{Class: NewEqClass(variable), Type: checker.pool.Fresh()}
	}
	substitution, err := NewSubstitution(entries...)
	if err != nil {
		return nil, err
	}
	return substitution.ApplyRecursive(scheme.Principal())
}

func (checker *EtcTypeChecker) onTypeFound(reference EtcRef, typ Type) {
	if exact, ok := reference.(ExactRef); ok {
		checker.listener.OnTypeFound(exact, typ)
	}
}
func (checker *EtcTypeChecker) onTypeError(reference EtcRef, message string) {
	checker.listener.OnTypeError(reference, message)
}
func (checker *EtcTypeChecker) onTypeWarning(reference EtcRef, message string) {
	checker.listener.OnTypeWarning(reference, message)
}

func applyTypes(substitution *Substitution, types []Type) []Type {
	result := make([]Type, len(types))
	for i, typ := range types {
		result[i] = typ
		if substituted, err := substitution.ApplyRecursive(typ); err == nil {
			result[i] = substituted
		}
	}
	return result
}
func joinTypeList(types []Type, separator string) string {
	parts := make([]string, len(types))
	for i, typ := range types {
		parts[i] = typ.String()
	}
	return strings.Join(parts, separator)
}
func pluralArguments(count int) string {
	if count == 1 {
		return "argument"
	}
	return "arguments"
}
