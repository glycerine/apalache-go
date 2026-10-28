package apalache

import (
	"context"
	"fmt"
	"strings"
)

// ValidateTraceBehavior reports whether concrete states form a behavior of a
// module produced by PrepareSafetyTransitions. It checks all normalized Init
// and Next alternatives, so distinct nondeterministic witnesses can be
// validated semantically instead of compared byte-for-byte.
func ValidateTraceBehavior(ctx context.Context, module *Module, states []TraceState) (bool, error) {
	if module == nil {
		return false, fmt.Errorf("cannot validate a trace without a module")
	}
	if !module.HasProperty(PropertyTransitionsFound) {
		return false, fmt.Errorf("trace validation requires normalized transitions")
	}
	if len(states) == 0 {
		return false, fmt.Errorf("cannot validate an empty trace")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	statePredicates := make([]Expr, len(states))
	for index, state := range states {
		predicate, err := concreteTraceStatePredicate(module, state)
		if err != nil {
			return false, fmt.Errorf("trace state %d: %w", index, err)
		}
		statePredicates[index] = predicate
	}
	initTransitions := TransitionsFromModule(module, NormalizedInitPrefix)
	nextTransitions := TransitionsFromModule(module, NormalizedNextPrefix)
	if len(initTransitions) == 0 {
		return false, fmt.Errorf("module has no normalized initial transitions")
	}
	path := make([]int, max(0, len(states)-1))
	var search func(int) (bool, error)
	search = func(depth int) (bool, error) {
		if depth == len(path) {
			for initNumber := range initTransitions {
				valid, err := validateConcreteTracePath(ctx, module, statePredicates, initNumber, path)
				if err != nil || valid {
					return valid, err
				}
			}
			return false, nil
		}
		for nextNumber := range nextTransitions {
			path[depth] = nextNumber
			valid, err := search(depth + 1)
			if err != nil || valid {
				return valid, err
			}
		}
		return false, nil
	}
	return search(0)
}

func concreteTraceStatePredicate(module *Module, state TraceState) (Expr, error) {
	variables := make(map[string]*VarDecl)
	for _, declaration := range module.VarDeclarations() {
		variables[declaration.Name()] = declaration
	}
	constants := stringSet(declarationNameSlice(module.ConstDeclarations()))
	arguments := make([]Expr, 0, len(variables))
	for _, name := range sortedStringKeys(variables) {
		declaration := variables[name]
		if strings.HasPrefix(name, "__") {
			continue
		}
		value, found := state[name]
		if !found {
			return nil, fmt.Errorf("missing variable %s", name)
		}
		if value == nil {
			return nil, fmt.Errorf("variable %s has a nil trace value", name)
		}
		variableType, typed := declaration.Tag().Type()
		valueType, valueTyped := value.Tag().Type()
		if !typed || !valueTyped || !EqualTypes(variableType, valueType) {
			return nil, fmt.Errorf("variable %s has trace type %v, expected %v", name, valueType, variableType)
		}
		arguments = append(arguments, MustOperExpr(Op("EQ"), Typed(BoolT1),
			NewNameExpr(name, Typed(variableType)), value))
	}
	for name := range state {
		if _, variable := variables[name]; !variable {
			if _, constant := constants[name]; !constant && !strings.HasPrefix(name, "__") {
				return nil, fmt.Errorf("unknown trace field %s", name)
			}
		}
	}
	if len(arguments) == 0 {
		return NewValueExpr(BoolValue(true), Typed(BoolT1)), nil
	}
	return MustOperExpr(Op("AND"), Typed(BoolT1), arguments...), nil
}

func validateConcreteTracePath(
	ctx context.Context, module *Module, states []Expr, initNumber int, nextNumbers []int,
) (bool, error) {
	executor := NewTransitionExecutor(
		declarationNameSlice(module.ConstDeclarations()), declarationNameSlice(module.VarDeclarations()))
	if constantInit, found := OperatorBody(module, NormalizedConstInit); found {
		if err := executor.InitializeConstants(constantInit); err != nil {
			return false, err
		}
	}
	for _, assumption := range module.AssumeDeclarations() {
		if err := executor.AssertState(assumption.Body()); err != nil {
			return false, err
		}
	}
	apply := func(number int, transition, state Expr) (bool, error) {
		complete, err := executor.PrepareTransition(number, transition)
		if err != nil || !complete {
			return false, err
		}
		if err = executor.AssumeTransition(number); err != nil {
			return false, err
		}
		if err = executor.AssertNextState(state); err != nil {
			return false, err
		}
		if err = executor.NextState(); err != nil {
			return false, err
		}
		return true, nil
	}
	initTransitions := TransitionsFromModule(module, NormalizedInitPrefix)
	complete, err := apply(initNumber, initTransitions[initNumber], states[0])
	if err != nil || !complete {
		return false, err
	}
	nextTransitions := TransitionsFromModule(module, NormalizedNextPrefix)
	for index, number := range nextNumbers {
		complete, err = apply(number, nextTransitions[number], states[index+1])
		if err != nil || !complete {
			return false, err
		}
	}
	status, err := executor.Check(ctx)
	if err != nil {
		return false, err
	}
	return status == SMTSatisfiable, nil
}
