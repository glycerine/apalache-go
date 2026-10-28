package apalache

import (
	"fmt"
	"sort"
)

type ScopeError struct{ Message string }

func (err *ScopeError) Error() string { return err.Message }

type scopeContext struct {
	free map[string]Type
	used map[string]struct{}
}

func newScopeContext() *scopeContext {
	return &scopeContext{free: make(map[string]Type), used: make(map[string]struct{})}
}

// ExprInstruction is a delayed, composable scoped-builder computation.
type ExprInstruction struct {
	run func(*scopeContext) (Expr, error)
}

func (instruction ExprInstruction) Build() (Expr, error) {
	if instruction.run == nil {
		return nil, fmt.Errorf("empty builder instruction")
	}
	return instruction.run(newScopeContext())
}

type ScopedBuilder struct{ typed *TypedBuilder }

func NewScopedBuilder() *ScopedBuilder { return &ScopedBuilder{typed: NewTypedBuilder()} }

func (builder *ScopedBuilder) Unchecked(expr Expr) ExprInstruction {
	return ExprInstruction{run: func(*scopeContext) (Expr, error) {
		if expr == nil {
			return nil, fmt.Errorf("unchecked expression is nil")
		}
		return expr, nil
	}}
}

func (builder *ScopedBuilder) Name(name string, typ Type) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		finalType := typ
		if expected, exists := context.free[name]; exists {
			unified, ok, err := CommonSupertype(expected, typ)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, &ScopeError{Message: fmt.Sprintf("name %s with type %v constructed where expected type is %v", name, typ, expected)}
			}
			finalType = unified
		}
		context.free[name] = finalType
		context.used[name] = struct{}{}
		return builder.typed.Name(name, finalType), nil
	}}
}

func (builder *ScopedBuilder) NameWithInferredType(name string) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		typ, exists := context.free[name]
		if !exists {
			return nil, &ScopeError{Message: fmt.Sprintf("cannot infer type of name %s: it is not in scope", name)}
		}
		return builder.typed.Name(name, typ), nil
	}}
}

func (builder *ScopedBuilder) Int64(value int64) ExprInstruction {
	return builder.Unchecked(builder.typed.Int64(value))
}
func (builder *ScopedBuilder) Bool(value bool) ExprInstruction {
	return builder.Unchecked(builder.typed.Bool(value))
}
func (builder *ScopedBuilder) String(value string) ExprInstruction {
	return ExprInstruction{run: func(*scopeContext) (Expr, error) { return builder.typed.String(value) }}
}
func (builder *ScopedBuilder) EmptySet(element Type) ExprInstruction {
	return builder.Unchecked(builder.typed.EmptySet(element))
}
func (builder *ScopedBuilder) EmptySequence(element Type) ExprInstruction {
	return builder.Unchecked(builder.typed.EmptySequence(element))
}
func (builder *ScopedBuilder) IsFiniteSet(set ExprInstruction) ExprInstruction {
	return builder.Apply(Op("FiniteSets!IsFiniteSet"), set)
}
func (builder *ScopedBuilder) Cardinality(set ExprInstruction) ExprInstruction {
	return builder.Apply(Op("FiniteSets!Cardinality"), set)
}

func (builder *ScopedBuilder) Apply(operator Operator, arguments ...ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, arguments)
		if err != nil {
			return nil, err
		}
		return builder.typed.Apply(operator, built...)
	}}
}

func (builder *ScopedBuilder) Tuple(arguments ...ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, arguments)
		if err != nil {
			return nil, err
		}
		return builder.typed.Tuple(built...)
	}}
}

func (builder *ScopedBuilder) Sequence(arguments ...ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		built, err := runInstructions(context, arguments)
		if err != nil {
			return nil, err
		}
		return builder.typed.Sequence(built...)
	}}
}

// BindBounded constructs an operator whose arguments are (bound variable,
// set, body), while enforcing type-consistent use and absence of shadowing.
func (builder *ScopedBuilder) BindBounded(operator Operator, variable, set, body ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		usedBefore := cloneNameSet(context.used)
		boundBefore := boundNames(context)
		setExpr, err := set.run(context)
		if err != nil {
			return nil, err
		}
		usedAfterSet := cloneNameSet(context.used)
		bodyExpr, err := body.run(context)
		if err != nil {
			return nil, err
		}
		boundAfterBody := boundNames(context)
		variableExpr, err := variable.run(context)
		if err != nil {
			return nil, err
		}
		names, err := markBound(context, variableExpr)
		if err != nil {
			return nil, err
		}
		usedInSet := differenceNameSets(usedAfterSet, usedBefore)
		newlyBoundInBody := differenceNameSets(boundAfterBody, boundBefore)
		if shadowed := intersectionNameSets(names, unionNameSets(usedInSet, newlyBoundInBody)); len(shadowed) > 0 {
			return nil, &ScopeError{Message: fmt.Sprintf("variable shadowing: %v", shadowed)}
		}
		return builder.typed.Apply(operator, variableExpr, setExpr, bodyExpr)
	}}
}

// BindUnbounded constructs an operator whose arguments are (bound variable,
// body). A free occurrence of the variable in the body is permitted and is
// made bound; an independently bound occurrence is rejected as shadowing.
func (builder *ScopedBuilder) BindUnbounded(operator Operator, variable, body ExprInstruction) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		boundBefore := boundNames(context)
		bodyExpr, err := body.run(context)
		if err != nil {
			return nil, err
		}
		boundAfterBody := boundNames(context)
		variableExpr, err := variable.run(context)
		if err != nil {
			return nil, err
		}
		names, err := markBound(context, variableExpr)
		if err != nil {
			return nil, err
		}
		if shadowed := intersectionNameSets(names, differenceNameSets(boundAfterBody, boundBefore)); len(shadowed) > 0 {
			return nil, &ScopeError{Message: fmt.Sprintf("variable shadowing: %v", shadowed)}
		}
		return builder.typed.Apply(operator, variableExpr, bodyExpr)
	}}
}

type BoundDomain struct {
	Variable ExprInstruction
	Set      ExprInstruction
}

// BindVariadic constructs SET_MAP or FUN_CTOR style applications:
// body, variable1, set1, ... variableN, setN.
func (builder *ScopedBuilder) BindVariadic(operator Operator, body ExprInstruction, domains ...BoundDomain) ExprInstruction {
	return ExprInstruction{run: func(context *scopeContext) (Expr, error) {
		if len(domains) == 0 {
			return nil, fmt.Errorf("variadic binder requires at least one domain")
		}
		boundBefore := boundNames(context)
		bodyExpr, err := body.run(context)
		if err != nil {
			return nil, err
		}
		boundAfterBody := boundNames(context)
		arguments := []Expr{bodyExpr}
		variables := make([]Expr, 0, len(domains))
		for _, domain := range domains {
			usedBefore := cloneNameSet(context.used)
			setExpr, runErr := domain.Set.run(context)
			if runErr != nil {
				return nil, runErr
			}
			usedAfterSet := cloneNameSet(context.used)
			variableExpr, runErr := domain.Variable.run(context)
			if runErr != nil {
				return nil, runErr
			}
			names, namesErr := boundVariableNames(variableExpr)
			if namesErr != nil {
				return nil, namesErr
			}
			usedInScope := unionNameSets(differenceNameSets(usedAfterSet, usedBefore), differenceNameSets(boundAfterBody, boundBefore))
			if shadowed := intersectionNameSets(names, usedInScope); len(shadowed) > 0 {
				return nil, &ScopeError{Message: fmt.Sprintf("variable shadowing: %v", shadowed)}
			}
			variables = append(variables, variableExpr)
			arguments = append(arguments, variableExpr, setExpr)
		}
		seen := make(map[string]struct{})
		for _, variable := range variables {
			names, _ := boundVariableNames(variable)
			for name := range names {
				if _, duplicate := seen[name]; duplicate {
					return nil, &ScopeError{Message: fmt.Sprintf("duplicate bound variable %s", name)}
				}
				seen[name] = struct{}{}
			}
			if _, err := markBound(context, variable); err != nil {
				return nil, err
			}
		}
		return builder.typed.Apply(operator, arguments...)
	}}
}

func runInstructions(context *scopeContext, instructions []ExprInstruction) ([]Expr, error) {
	result := make([]Expr, len(instructions))
	for index, instruction := range instructions {
		if instruction.run == nil {
			return nil, fmt.Errorf("empty builder instruction at index %d", index)
		}
		var err error
		result[index], err = instruction.run(context)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func boundVariableNames(expr Expr) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	switch typed := expr.(type) {
	case *NameExpr:
		result[typed.Name()] = struct{}{}
	case *OperExpr:
		if typed.Operator().Name() != "TUPLE" {
			return nil, fmt.Errorf("bound variable must be a name or tuple of names")
		}
		for _, argument := range typed.Arguments() {
			name, ok := argument.(*NameExpr)
			if !ok {
				return nil, fmt.Errorf("bound tuple contains a non-name expression")
			}
			if _, duplicate := result[name.Name()]; duplicate {
				return nil, fmt.Errorf("bound tuple contains duplicate name %s", name.Name())
			}
			result[name.Name()] = struct{}{}
		}
	default:
		return nil, fmt.Errorf("bound variable must be a name or tuple of names")
	}
	return result, nil
}

func markBound(context *scopeContext, expr Expr) (map[string]struct{}, error) {
	names, err := boundVariableNames(expr)
	if err != nil {
		return nil, err
	}
	for name := range names {
		delete(context.free, name)
	}
	return names, nil
}

func boundNames(context *scopeContext) map[string]struct{} {
	result := make(map[string]struct{})
	for name := range context.used {
		if _, free := context.free[name]; !free {
			result[name] = struct{}{}
		}
	}
	return result
}

func cloneNameSet(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source))
	for name := range source {
		result[name] = struct{}{}
	}
	return result
}
func differenceNameSets(left, right map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{})
	for name := range left {
		if _, exists := right[name]; !exists {
			result[name] = struct{}{}
		}
	}
	return result
}
func unionNameSets(left, right map[string]struct{}) map[string]struct{} {
	result := cloneNameSet(left)
	for name := range right {
		result[name] = struct{}{}
	}
	return result
}
func intersectionNameSets(left, right map[string]struct{}) []string {
	result := make([]string, 0)
	for name := range left {
		if _, exists := right[name]; exists {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}
