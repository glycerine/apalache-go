package apalache

import "fmt"

// UnmarshalApalacheJSONTrace decodes the legacy Apalache-IR trace shape used
// by the Scala trace evaluator. The first operator is the synthetic constant
// initializer, the last is the synthetic invariant, and every operator between
// them contains one state as either an equality or a conjunction of equalities.
func UnmarshalApalacheJSONTrace(data []byte) ([]TraceState, error) {
	module, err := UnmarshalSingleModuleIRJSON(data, NewSourceStore())
	if err != nil {
		return nil, fmt.Errorf("cannot decode Apalache JSON trace: %w", err)
	}
	declarations := module.Declarations()
	if len(declarations) < 3 {
		return nil, fmt.Errorf("Apalache JSON trace requires constant-init, state, and invariant declarations")
	}
	states := make([]TraceState, len(declarations)-2)
	for index, declaration := range declarations[1 : len(declarations)-1] {
		operator, ok := declaration.(*OperDecl)
		if !ok || len(operator.FormalParams()) != 0 {
			return nil, fmt.Errorf("Apalache JSON trace state %d is not a nullary operator", index)
		}
		state, stateErr := apalacheJSONTraceState(operator.Body())
		if stateErr != nil {
			return nil, fmt.Errorf("Apalache JSON trace state %d: %w", index, stateErr)
		}
		states[index] = state
	}
	return states, nil
}

func apalacheJSONTraceState(expression Expr) (TraceState, error) {
	assignments := []Expr{expression}
	if conjunction, ok := expression.(*OperExpr); ok && conjunction.Operator().Name() == "AND" {
		assignments = conjunction.Arguments()
	}
	state := make(TraceState, len(assignments))
	for _, assignment := range assignments {
		equality, ok := assignment.(*OperExpr)
		if !ok || equality.Operator().Name() != "EQ" || len(equality.Arguments()) != 2 {
			return nil, fmt.Errorf("expected a variable equality, found %T", assignment)
		}
		arguments := equality.Arguments()
		name, ok := arguments[0].(*NameExpr)
		if !ok {
			return nil, fmt.Errorf("equality left side is not a variable name")
		}
		if _, duplicate := state[name.Name()]; duplicate {
			return nil, fmt.Errorf("variable %s is assigned more than once", name.Name())
		}
		state[name.Name()] = arguments[1]
	}
	return state, nil
}
