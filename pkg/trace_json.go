package apalache

import (
	"fmt"
	"sort"
	"strings"
)

// MarshalDecodedTraceApalacheJSON emits the legacy Apalache-IR trace shape
// written by Scala's JsonCounterexampleWriter.
func MarshalDecodedTraceApalacheJSON(module *Module, states []DecodedState, violation Expr) ([]byte, error) {
	traceModule, err := decodedTraceModule(module, states, violation)
	if err != nil {
		return nil, err
	}
	return MarshalSingleModuleIRJSON(traceModule, IRJSONOptions{})
}

// FormatDecodedTraceTLA emits the parseable counterexample module shape used
// by Scala's TlaCounterexampleWriter, without nondeterministic timestamps.
func FormatDecodedTraceTLA(module *Module, states []DecodedState, violation Expr) (string, error) {
	traceModule, err := decodedTraceModule(module, states, violation)
	if err != nil {
		return "", err
	}
	return FormatTLAModule(traceModule, TLAFormatOptions{ExtendedModules: []string{module.Name()}})
}

// FormatDecodedTraceTLC emits the TLC console-message trace shape used by
// Scala's TlcCounterexampleWriter. It deliberately omits wall-clock text so
// repeated standalone runs are byte-for-byte reproducible.
func FormatDecodedTraceTLC(module *Module, states []DecodedState) (string, error) {
	if module == nil {
		return "", fmt.Errorf("cannot encode decoded trace for nil module")
	}
	if len(states) == 0 {
		return "", fmt.Errorf("expected at least one decoded state, found none")
	}
	constantNames := make(map[string]struct{}, len(module.ConstDeclarations()))
	for _, declaration := range module.ConstDeclarations() {
		constantNames[declaration.Name()] = struct{}{}
	}
	variableNames := make(map[string]struct{}, len(module.VarDeclarations()))
	internalNames := make(map[string]struct{})
	for _, declaration := range module.VarDeclarations() {
		if declaration.IsInternal() {
			internalNames[declaration.Name()] = struct{}{}
		} else {
			variableNames[declaration.Name()] = struct{}{}
		}
	}
	// A Trace is a self-contained witness. Scala's writer preserves bindings
	// carried by the witness even when its lightweight root-module fixture does
	// not repeat the declarations. Do the same without exposing known constants
	// or generated internal variables as state variables.
	for _, state := range states {
		for name := range state.Values {
			if _, constant := constantNames[name]; constant {
				continue
			}
			if _, internal := internalNames[name]; internal {
				continue
			}
			variableNames[name] = struct{}{}
		}
	}
	var output strings.Builder
	output.WriteString("@!@!@STARTMSG 2262:0 @!@!@\nCreated by Apalache\n@!@!@ENDMSG 2262 @!@!@\n")
	output.WriteString("@!@!@STARTMSG 2110:1 @!@!@\nInvariant is violated.\n@!@!@ENDMSG 2110 @!@!@\n")
	output.WriteString("@!@!@STARTMSG 2121:1 @!@!@\nThe behavior up to this point is:\n@!@!@ENDMSG 2121 @!@!@\n")
	for index, state := range states {
		label := "<Next>"
		if index == 0 {
			label = "<Initial predicate>"
		}
		fmt.Fprintf(&output, "@!@!@STARTMSG 2217:4 @!@!@\n%d: %s\n", index+1, label)
		names := make([]string, 0, len(variableNames))
		for name := range variableNames {
			if _, found := state.Values[name]; found {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		if len(names) == 0 {
			output.WriteString("TRUE\n")
		} else {
			prefix := ""
			if len(names) > 1 {
				prefix = "/\\ "
			}
			for _, name := range names {
				value, err := FormatTLAExpression(state.Values[name])
				if err != nil {
					return "", fmt.Errorf("format state %d variable %s: %w", index, name, err)
				}
				fmt.Fprintf(&output, "%s%s = %s\n", prefix, ParseableTLAName(name), value)
			}
		}
		output.WriteString("\n@!@!@ENDMSG 2217 @!@!@\n")
	}
	return output.String(), nil
}

func decodedTraceModule(module *Module, states []DecodedState, violation Expr) (*Module, error) {
	if module == nil {
		return nil, fmt.Errorf("cannot encode decoded trace for nil module")
	}
	if len(states) == 0 {
		return nil, fmt.Errorf("expected at least one decoded state, found none")
	}
	if violation == nil {
		violation = NewValueExpr(BoolValue(true), Typed(BoolT1))
	}
	constantNames := make(map[string]struct{}, len(module.ConstDeclarations()))
	for _, declaration := range module.ConstDeclarations() {
		constantNames[declaration.Name()] = struct{}{}
	}
	variableNames := make(map[string]struct{}, len(module.VarDeclarations()))
	internalNames := make(map[string]struct{})
	for _, declaration := range module.VarDeclarations() {
		if declaration.IsInternal() {
			internalNames[declaration.Name()] = struct{}{}
		} else {
			variableNames[declaration.Name()] = struct{}{}
		}
	}
	// Preserve bindings supplied by self-contained traces even when a minimal
	// root module does not repeat their declarations (as in Scala's fixtures).
	for _, state := range states {
		for name := range state.Values {
			if _, constant := constantNames[name]; constant {
				continue
			}
			if _, internal := internalNames[name]; internal {
				continue
			}
			variableNames[name] = struct{}{}
		}
	}
	declarations := make([]Decl, 0, len(states)+2)
	declarations = append(declarations, traceStateDeclaration("ConstInit", states[0].Values, constantNames))
	for index, state := range states {
		declarations = append(declarations,
			traceStateDeclaration(fmt.Sprintf("State%d", index), state.Values, variableNames))
	}
	declarations = append(declarations,
		NewOperDecl("InvariantViolation", nil, violation, Typed(mustOperator(nil, BoolT1))))
	return NewModule("counterexample", declarations), nil
}

func traceStateDeclaration(name string, values map[string]Expr, included map[string]struct{}) *OperDecl {
	names := make([]string, 0, len(included))
	for variable := range included {
		if _, found := values[variable]; found {
			names = append(names, variable)
		}
	}
	sort.Strings(names)
	assignments := make([]Expr, 0, len(names))
	for _, variable := range names {
		value := values[variable]
		assignments = append(assignments, MustOperExpr(Op("EQ"), Typed(BoolT1),
			NewNameExpr(variable, value.Tag()), value))
	}
	var body Expr
	switch len(assignments) {
	case 0:
		body = NewValueExpr(BoolValue(true), Typed(BoolT1))
	default:
		body = MustOperExpr(Op("AND"), Typed(BoolT1), assignments...)
	}
	return NewOperDecl(name, nil, body, Typed(mustOperator(nil, BoolT1)))
}

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
	if len(declarations) < 2 {
		return nil, fmt.Errorf("Apalache JSON trace requires constant-init and invariant declarations")
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
	if value, ok := expression.(*ValueExpr); ok {
		if boolean, isBoolean := value.Value().(BoolValue); isBoolean && boolean.Bool() {
			return TraceState{}, nil
		}
	}
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
